package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/adonko3xBitters/boxincloud/server/internal/accounts"
	"github.com/adonko3xBitters/boxincloud/server/internal/auth"
	"github.com/adonko3xBitters/boxincloud/server/internal/cache"
	"github.com/adonko3xBitters/boxincloud/server/internal/catalog"
	"github.com/adonko3xBitters/boxincloud/server/internal/config"
	"github.com/adonko3xBitters/boxincloud/server/internal/discovery"
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
	Discovery *discovery.Service
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

	// Même indirection que ci-dessus, pour la même raison : le worker d'import
	// écrit par l'ingestion, et l'ingestion est construite plus bas.
	var deferredIngest *ingest.Service

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

	discoveryService := discovery.NewService(
		discovery.NewPostgresRepository(queries),
		discovery.NewOPDSClient(),
		sealer,
		log,
	)
	discoveryService.SetMetadata(buildMetadataRegistry(cfg, log))

	jobClient, err := jobs.New(pool, cfg.Jobs, log, func(w *river.Workers) {
		discovery.Register(w, discoveryService, depositTo(&deferredIngest))

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
	discoveryService.SetImportQueue(&importQueue{client: jobClient})

	ingestService := ingest.NewService(
		libraries,
		indexerRepo,
		ingest.NewPostgresManage(queries),
		&jobScanner{client: jobClient},
		cacheProvider,
		cfg.Upload.MaxSize,
		log,
	)

	// Supprimer un dossier peut emporter les albums qu'il contient. La règle de
	// suppression — exclusion ou effacement, dans le bon ordre — appartient à
	// l'ingestion : elle est empruntée plutôt que réécrite.
	deferredIngest = ingestService

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
		Discovery: discoveryService,
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

/*
RunImport exécute un import sans passer par la file.

Même intention que `ScanLibrary` côté direct : permettre au harnais de tests et
à la ligne de commande de dérouler le travail sans démarrer de workers. Le
chemin exécuté est exactement celui du worker — c'est ce qui donne au test sa
valeur.
*/
func (c *Core) RunImport(ctx context.Context, importID uuid.UUID) error {
	return c.Discovery.RunImport(ctx, importID, depositTo(&c.Ingest))
}

// ScanLibrary enfile un scan de bibliothèque.
func (c *Core) ScanLibrary(ctx context.Context, libraryID uuid.UUID) error {
	return c.Jobs.Insert(ctx, indexer.ScanLibraryArgs{LibraryID: libraryID})
}

/*
importQueue enfile un import, et rien d'autre.

Comme `jobScanner` : le service de découverte ne connaît que cette opération de
la file, et lui passer le client entier lui donnerait le pouvoir d'enfiler
n'importe quoi.
*/
type importQueue struct {
	client *jobs.Client
}

func (q *importQueue) EnqueueImport(ctx context.Context, importID uuid.UUID) error {
	return q.client.Insert(ctx, discovery.ImportArgs{ImportID: importID})
}

/*
depositTo branche l'import sur l'ingestion.

L'adaptateur vaut mieux qu'une dépendance directe : le paquet `discovery`
décrit ce dont il a besoin, l'ingestion garde ses types, et les règles
d'écriture — borne de taille sur le flux, signature vérifiée avant d'écrire,
refus d'écraser, contrôle du dossier de destination — ne sont pas réécrites une
seconde fois.

C'est aussi ici, et nulle part ailleurs, que les deux vocabulaires d'erreur se
rencontrent : les échecs d'ingestion en ressortent porteurs d'un code stable que
l'interface saura traduire.

Le pointeur est indirect parce que le worker est déclaré avant l'ingestion.
*/
func depositTo(service **ingest.Service) discovery.Deposit {
	return func(ctx context.Context, p discovery.DepositParams) (discovery.Deposited, error) {
		result, err := (*service).Upload(ctx, ingest.UploadParams{
			LibraryID: p.LibraryID,
			Folder:    p.Folder,
			Filename:  p.Filename,
			Size:      p.Size,
			Content:   p.Content,
		})
		if err != nil {
			return discovery.Deposited{}, discovery.ErrDeposit{
				Code:   depositCode(err),
				Detail: err.Error(),
			}
		}
		return discovery.Deposited{
			ComicID:   result.ComicID,
			ObjectKey: result.ObjectKey,
			Title:     result.Title,
			Format:    result.Format,
			Size:      result.Size,
		}, nil
	}
}

// depositCode nomme un échec d'ingestion pour l'interface.
//
// Le code, pas la phrase : le serveur ne devine pas la langue du lecteur, ici
// pas plus qu'ailleurs.
func depositCode(err error) string {
	switch {
	case errors.Is(err, ingest.ErrUnsupportedFormat):
		return "unsupported-format"
	case errors.Is(err, ingest.ErrContentMismatch):
		return "content-mismatch"
	case errors.Is(err, ingest.ErrAlreadyExists):
		return "exists"
	case errors.Is(err, ingest.ErrTooLarge):
		return "too-large"
	default:
		return "deposit-failed"
	}
}

/*
buildMetadataRegistry enregistre les bases de métadonnées disponibles.

Le débit et le cache sont construits ICI et partagés par tous les fournisseurs.
C'est la seule façon qu'ils valent quelque chose : un limiteur par fournisseur
laisserait passer autant de requêtes qu'on a construit d'objets, et un cache par
fournisseur ne mémoriserait rien d'un appel à l'autre.

Aucun fournisseur n'est indispensable. Une instance coupée d'Internet — le cas
d'un serveur familial sur un réseau fermé — voit simplement le rapprochement
rendre une liste vide, sans que rien d'autre en pâtisse.
*/
func buildMetadataRegistry(cfg *config.Config, log *slog.Logger) *discovery.Registry {
	throttle := discovery.NewThrottle()
	throttle.SetRate("openlibrary", discovery.RateOpenLibrary)
	throttle.SetRate("internetarchive", discovery.RateInternetArchive)
	throttle.SetRate("googlebooks", discovery.RateGoogleBooks)
	throttle.SetRate("opds", discovery.RateOPDS)

	// Cinq minutes et cinq cents entrées : de quoi absorber les recherches
	// répétées d'une session sans jamais servir une fiche franchement périmée,
	// pour moins d'un méga-octet.
	deps := discovery.MetadataDeps{
		Throttle: throttle,
		Memo:     discovery.NewMemo(5*time.Minute, 500),
	}

	registry := discovery.NewRegistry()

	// Un registre vide est un cas normal, pas dégradé : le rapprochement rend
	// alors une liste vide et rien d'autre n'en pâtit.
	if !cfg.Discovery.Metadata {
		log.Info("bases de métadonnées désactivées")
		return registry
	}

	registry.Register(discovery.NewOpenLibrary(deps))
	registry.Register(discovery.NewInternetArchive(deps))

	// Voir config.Discovery : sans clé, Google Books échoue une fois sur deux,
	// ce qui vaut moins que son absence.
	if key := cfg.Discovery.GoogleBooksKey; key != "" {
		registry.Register(discovery.NewGoogleBooks(key, deps))
		log.Info("Google Books activé")
	}

	return registry
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

func (s *cacheStore) Stats(ctx context.Context) (cache.Stats, error) {
	row, err := s.q.CacheStats(ctx)
	if err != nil {
		return cache.Stats{}, err
	}

	stats := cache.Stats{Entries: row.Entries, Bytes: row.Bytes, Hits: row.Hits}

	// Un cache vide n'a ni plus ancienne entrée ni dernier accès : les dates
	// restent nulles plutôt que de valoir l'époque Unix, qui se serait affichée
	// comme « 1970 » dans l'interface.
	if !row.OldestAt.Time.IsZero() {
		oldest := row.OldestAt.Time
		stats.OldestAt = &oldest
	}
	if !row.NewestHitAt.Time.IsZero() {
		newest := row.NewestHitAt.Time
		stats.NewestHitAt = &newest
	}
	return stats, nil
}

func (s *cacheStore) PurgeEntries(ctx context.Context) ([]cache.Entry, error) {
	rows, err := s.q.PurgeCacheEntries(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]cache.Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, cache.Entry{Key: row.Key, Size: row.Size})
	}
	return out, nil
}
