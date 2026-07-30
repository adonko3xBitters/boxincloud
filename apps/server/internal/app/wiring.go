package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/adonko3xBitters/boxincloud/server/internal/accounts"
	"github.com/adonko3xBitters/boxincloud/server/internal/auth"
	"github.com/adonko3xBitters/boxincloud/server/internal/cache"
	"github.com/adonko3xBitters/boxincloud/server/internal/catalog"
	"github.com/adonko3xBitters/boxincloud/server/internal/config"
	"github.com/adonko3xBitters/boxincloud/server/internal/folders"
	"github.com/adonko3xBitters/boxincloud/server/internal/imaging"
	"github.com/adonko3xBitters/boxincloud/server/internal/indexer"
	"github.com/adonko3xBitters/boxincloud/server/internal/ingest"
	"github.com/adonko3xBitters/boxincloud/server/internal/library"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/crypto"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/db"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/jobs"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sqlc"
	"github.com/adonko3xBitters/boxincloud/server/internal/progress"
	"github.com/adonko3xBitters/boxincloud/server/internal/reader"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage/local"
)

// Core rassemble les services métier.
//
// Construit une seule fois et partagé par le serveur et le CLI : les deux
// binaires font exactement les mêmes choses, avec le même câblage.
type Core struct {
	Queries   *sqlc.Queries
	Auth      *auth.Service
	Accounts  *accounts.Service
	Catalog   *catalog.Service
	Tools     *catalog.Tools
	Folders   *folders.Service
	Reader    *reader.Service
	Progress  *progress.Service
	Libraries *library.Service
	Cache     *cache.Cache
	Imaging   imaging.Processor
	Indexer   indexer.Repository
	Ingest    *ingest.Service
	Jobs      *jobs.Client
}

// BuildCore assemble les services métier au-dessus d'un pool PostgreSQL.
func BuildCore(ctx context.Context, cfg *config.Config, pool *db.Pool, log *slog.Logger) (*Core, error) {
	queries := sqlc.New(pool)

	sealer, err := crypto.NewSealer(cfg.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("clé de chiffrement inutilisable : %w", err)
	}

	authService := auth.NewService(
		auth.NewPostgresRepository(queries),
		auth.NewTokenIssuer(cfg.Auth.JWTSecret, cfg.Auth.AccessTokenTTL),
		cfg.Auth.RefreshTokenTTL,
		log,
	)

	libraryRepo := library.NewPostgresRepository(queries)
	libraries := library.NewService(libraryRepo, sealer, log)
	libraries.SetAdminRepository(libraryRepo)

	// Le cache dérivé vit sur un provider local. Le placer sur un bucket dédié
	// — pour le partager entre plusieurs instances — ne demandera que de
	// changer ce provider.
	//
	// Le répertoire est créé au besoin : son contenu est entièrement
	// reconstructible, il n'y a aucune raison d'exiger de l'administrateur
	// qu'il le prépare.
	if err := os.MkdirAll(cfg.Cache.Dir, 0o750); err != nil {
		return nil, fmt.Errorf("création du répertoire de cache (%s) : %w", cfg.Cache.Dir, err)
	}

	cacheProvider, err := local.New(local.Options{Root: cfg.Cache.Dir})
	if err != nil {
		return nil, fmt.Errorf("répertoire de cache inutilisable (%s) : %w", cfg.Cache.Dir, err)
	}

	derived := cache.New(cacheProvider, &cacheStore{q: queries}, cfg.Cache.MaxSize, log)

	// Dépendance circulaire assumée : le repository doit pouvoir enfiler des
	// jobs, et le client de jobs doit connaître les workers qui utilisent ce
	// repository. On la casse par une indirection, renseignée juste après.
	enqueuer := &deferredEnqueuer{}
	indexerRepo := indexer.NewPostgresRepository(queries, pool, enqueuer)

	processor := imaging.NewPureGo()
	catalogService := catalog.NewService(catalog.NewPostgresRepository(queries))

	folderRepo := folders.NewPostgresRepository(queries)
	folderService := folders.NewService(folderRepo, libraries, log)
	folderService.SetLockRepository(folderRepo)
	folderService.SetShareRepository(folderRepo)

	// Le catalogue masque ce que les codes d'accès cachent. Le résolveur est
	// injecté plutôt qu'importé : le catalogue est délibérément mince, et
	// dépendre du paquet folders créerait un cycle.
	catalogService.SetLockResolver(folderService.HiddenPaths)

	jobClient, err := jobs.New(pool, cfg.Jobs, log, func(w *river.Workers) {
		indexer.Register(w, indexer.Deps{
			Libraries: libraries,
			Repo:      indexerRepo,
			Cache:     derived,
			Imaging:   processor,
			Log:       log,
			Folders:   folderService,
		})
	})
	if err != nil {
		return nil, err
	}
	enqueuer.client = jobClient

	ingestService := ingest.NewService(
		libraries,
		indexerRepo,
		ingest.NewPostgresManage(queries),
		&jobScanner{client: jobClient},
		cfg.Upload.MaxSize,
		log,
	)

	// Supprimer un dossier peut emporter les albums qu'il contient. La règle de
	// suppression — exclusion ou effacement, dans le bon ordre — appartient à
	// l'ingestion : elle est empruntée plutôt que réécrite.
	folderService.SetComicRemover(ingestService.BulkDelete)
	ingestService.SetFolderRegistrar(folderService.Ensure)
	ingestService.SetWriteGuard(folderService.EnsureWritable)

	return &Core{
		Queries:  queries,
		Auth:     authService,
		Accounts: accounts.NewService(accounts.NewPostgresRepository(queries), authService, log),
		Catalog:  catalogService,
		Tools:    catalog.NewTools(catalog.NewPostgresTools(queries), catalogService),
		Reader: reader.NewService(
			reader.NewPostgresRepository(queries), libraries, derived, processor, log),
		Progress:  progress.NewService(progress.NewPostgresRepository(queries)),
		Libraries: libraries,
		Cache:     derived,
		Imaging:   processor,
		Indexer:   indexerRepo,
		Ingest:    ingestService,
		Folders:   folderService,
		Jobs:      jobClient,
	}, nil
}

// jobScanner enfile un scan de bibliothèque.
//
// Le service d'ingestion ne connaît que cette opération de la file de jobs :
// lui passer le client entier lui donnerait le pouvoir d'enfiler n'importe quoi.
type jobScanner struct {
	client *jobs.Client
}

func (s *jobScanner) EnqueueScanLibrary(ctx context.Context, libraryID uuid.UUID) error {
	return s.client.Insert(ctx, indexer.ScanLibraryArgs{LibraryID: libraryID})
}

// ScanLibrary enfile un scan de bibliothèque.
func (c *Core) ScanLibrary(ctx context.Context, libraryID uuid.UUID) error {
	return c.Jobs.Insert(ctx, indexer.ScanLibraryArgs{LibraryID: libraryID})
}

// deferredEnqueuer permet de construire le repository avant le client de jobs.
type deferredEnqueuer struct {
	client *jobs.Client
}

func (d *deferredEnqueuer) EnqueueIndexComic(ctx context.Context, comicID uuid.UUID) error {
	if d.client == nil {
		return fmt.Errorf("jobs : client non initialisé")
	}
	return d.client.Insert(ctx, indexer.IndexComicArgs{ComicID: comicID})
}

// cacheStore adapte les requêtes générées à l'interface attendue par le cache.
type cacheStore struct {
	q *sqlc.Queries
}

var _ cache.Store = (*cacheStore)(nil)

func (s *cacheStore) RecordEntry(ctx context.Context, key string, comicID uuid.UUID, size int64) error {
	return s.q.RecordCacheEntry(ctx, sqlc.RecordCacheEntryParams{
		Key:     key,
		ComicID: uuid.NullUUID{UUID: comicID, Valid: comicID != uuid.Nil},
		Size:    size,
	})
}

func (s *cacheStore) TouchEntry(ctx context.Context, key string) (bool, error) {
	rows, err := s.q.TouchCacheEntry(ctx, key)
	return rows > 0, err
}

func (s *cacheStore) TotalSize(ctx context.Context) (int64, error) {
	return s.q.TotalCacheSize(ctx)
}

func (s *cacheStore) ListForEviction(ctx context.Context, limit int32) ([]cache.Entry, error) {
	rows, err := s.q.ListCacheEntriesForEviction(ctx, limit)
	if err != nil {
		return nil, err
	}

	out := make([]cache.Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, cache.Entry{Key: row.Key, Size: row.Size})
	}
	return out, nil
}

func (s *cacheStore) DeleteEntry(ctx context.Context, key string) error {
	return s.q.DeleteCacheEntry(ctx, key)
}
