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
}

// Repository est tout ce dont l'indexeur a besoin de la persistance.
//
// Déclarée au point d'usage : l'indexeur ne dépend ni du paquet de données
// généré, ni du client de jobs. C'est ce qui permet de le tester sans
// PostgreSQL, et de remplacer la mise en file par une exécution directe.
type Repository interface {
	// Comics
	UpsertComic(ctx context.Context, p UpsertComicParams) (Comic, bool, error)
	GetComic(ctx context.Context, id uuid.UUID) (Comic, error)
	SetComicState(ctx context.Context, id uuid.UUID, state, detail string) error
	SetComicIndexed(ctx context.Context, id uuid.UUID, pageCount int) error
	ApplyMetadata(ctx context.Context, id uuid.UUID, m Metadata, seriesID *uuid.UUID, metadataJSON []byte) error
	SetCoverPlaceholder(ctx context.Context, id uuid.UUID, dataURI string) error
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
