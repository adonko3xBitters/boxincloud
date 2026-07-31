package indexer

import (
	"context"

	"github.com/google/uuid"
)

// Comic est la vue que l'indexeur a d'un album.
type Comic struct {
	ID            uuid.UUID
	LibraryID     uuid.UUID
	ObjectKey     string
	FileSize      int64
	Format        string
	State         string
	NeedsIndexing bool
}

// Page est une ligne de comic_pages avant persistance.
type Page struct {
	Index       int
	EntryName   string
	DataOffset  int64
	DataSize    int64
	Size        int64
	Compression int16
	Width       int
	Height      int
	IsDouble    bool
}

// ScanRun identifie une exécution de scan.
type ScanRun struct {
	ID uuid.UUID
}

// UpsertComicParams décrit un objet découvert lors d'un scan.
type UpsertComicParams struct {
	LibraryID uuid.UUID
	ObjectKey string
	FileSize  int64
	FileETag  string
	Format    string
	Title     string
	// FolderPath est le dossier contenant l'album, relatif au préfixe de la
	// bibliothèque. Vide à la racine.
	FolderPath string
}

// Repository est tout ce dont l'indexeur a besoin de la persistance.
//
// Déclarée au point d'usage : l'indexeur ne dépend ni du paquet de données
// généré, ni du client de jobs. C'est ce qui permet de le tester sans
// PostgreSQL, et de remplacer la mise en file par une exécution directe.
// FolderObserver réconcilie l'arborescence avec ce qu'un parcours a trouvé.
//
// Déclarée ici plutôt qu'importée : le paquet folders dépend déjà des
// bibliothèques et du stockage, et l'importer créerait un cycle.
type FolderObserver interface {
	Observe(ctx context.Context, libraryID uuid.UUID, paths []string) error
}

type Repository interface {
	// Comics
	UpsertComic(ctx context.Context, p UpsertComicParams) (Comic, bool, error)
	GetComic(ctx context.Context, id uuid.UUID) (Comic, error)
	SetComicState(ctx context.Context, id uuid.UUID, state, detail string) error
	SetComicIndexed(ctx context.Context, id uuid.UUID, pageCount int) error

	// SetComicHydrated enregistre la clé de l'archive normalisée, dans le cache
	// dérivé, d'un album qui n'a pas d'accès aléatoire natif.
	SetComicHydrated(ctx context.Context, id uuid.UUID, key string) error
	ApplyMetadata(ctx context.Context, id uuid.UUID, m Metadata, seriesID *uuid.UUID, metadataJSON []byte) error
	SetCoverPlaceholder(ctx context.Context, id uuid.UUID, dataURI string) error
	SetFolder(ctx context.Context, id uuid.UUID, folder string) error
	MarkMissingDeleted(ctx context.Context, libraryID uuid.UUID, seenKeys []string) (int64, error)

	// Pages — remplacement atomique : une réindexation ne doit jamais laisser
	// un mélange d'anciennes et de nouvelles pages.
	ReplaceComicPages(ctx context.Context, comicID uuid.UUID, pages []Page) error

	// Séries
	UpsertSeries(ctx context.Context, libraryID uuid.UUID, name, sortName string) (uuid.UUID, error)
	RefreshSeriesCounts(ctx context.Context, libraryID uuid.UUID) error

	// Bibliothèques et scans
	SetLibraryScanResult(ctx context.Context, libraryID uuid.UUID, status string) error
	StartScanRun(ctx context.Context, libraryID uuid.UUID) (ScanRun, error)
	FinishScanRun(ctx context.Context, runID uuid.UUID, status string, stats ScanStats, detail string) error

	// Mise en file du job d'indexation.
	EnqueueIndexComic(ctx context.Context, comicID uuid.UUID) error
}
