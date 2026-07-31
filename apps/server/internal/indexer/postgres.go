package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sqlc"
)

// Enqueuer met en file un job d'indexation.
//
// Interface plutôt que dépendance directe au client River : l'indexeur reste
// testable sans base, et un test peut exécuter le job immédiatement plutôt que
// de l'enfiler.
type Enqueuer interface {
	EnqueueIndexComic(ctx context.Context, comicID uuid.UUID) error
}

// PostgresRepository implémente Repository sur les requêtes générées.
type PostgresRepository struct {
	q        *sqlc.Queries
	pool     PgxPool
	enqueuer Enqueuer
}

// PgxPool est le sous-ensemble de pgxpool.Pool nécessaire aux transactions.
type PgxPool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

var _ Repository = (*PostgresRepository)(nil)

func NewPostgresRepository(q *sqlc.Queries, pool PgxPool, enqueuer Enqueuer) *PostgresRepository {
	return &PostgresRepository{q: q, pool: pool, enqueuer: enqueuer}
}

// ─── Comics ──────────────────────────────────────────────────────────────────

func (r *PostgresRepository) UpsertComic(ctx context.Context, p UpsertComicParams) (Comic, bool, error) {
	var etag *string
	if p.FileETag != "" {
		etag = &p.FileETag
	}

	row, err := r.q.UpsertComic(ctx, sqlc.UpsertComicParams{
		ID:        uuid.Must(uuid.NewV7()),
		LibraryID: p.LibraryID,
		ObjectKey: p.ObjectKey,
		FileSize:  p.FileSize,
		FileEtag:  etag,
		Format:    sqlc.ComicFormat(p.Format),
		Title:     p.Title,
	})
	if err != nil {
		return Comic{}, false, err
	}

	return Comic{
		ID:        row.ID,
		LibraryID: row.LibraryID,
		ObjectKey: row.ObjectKey,
		FileSize:  row.FileSize,
		Format:    string(row.Format),
		State:     string(row.State),
		// L'état 'pending' est posé par la requête d'upsert quand l'ETag ou la
		// taille ont changé — c'est donc la base, et elle seule, qui décide de
		// ce qui doit être réindexé.
		NeedsIndexing: row.State == sqlc.ComicStatePending,
	}, row.Inserted, nil
}

// SetFolder enregistre le dossier d'un album.
func (r *PostgresRepository) SetFolder(ctx context.Context, id uuid.UUID, folder string) error {
	return r.q.SetComicFolder(ctx, sqlc.SetComicFolderParams{ID: id, FolderPath: folder})
}

func (r *PostgresRepository) GetComic(ctx context.Context, id uuid.UUID) (Comic, error) {
	row, err := r.q.GetComic(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Comic{}, fmt.Errorf("indexer : comic %s introuvable", id)
	}
	if err != nil {
		return Comic{}, err
	}

	return Comic{
		ID:            row.ID,
		LibraryID:     row.LibraryID,
		ObjectKey:     row.ObjectKey,
		FileSize:      row.FileSize,
		Format:        string(row.Format),
		State:         string(row.State),
		NeedsIndexing: row.State == sqlc.ComicStatePending,
	}, nil
}

func (r *PostgresRepository) SetComicState(ctx context.Context, id uuid.UUID, state, detail string) error {
	var d *string
	if detail != "" {
		d = &detail
	}
	return r.q.SetComicState(ctx, sqlc.SetComicStateParams{
		ID:          id,
		State:       sqlc.ComicState(state),
		StateDetail: d,
	})
}

func (r *PostgresRepository) SetComicHydrated(ctx context.Context, id uuid.UUID, key string) error {
	return r.q.SetComicHydrated(ctx, sqlc.SetComicHydratedParams{ID: id, HydratedKey: &key})
}

func (r *PostgresRepository) SetComicIndexed(ctx context.Context, id uuid.UUID, pageCount int) error {
	return r.q.SetComicIndexed(ctx, sqlc.SetComicIndexedParams{
		ID:        id,
		PageCount: toInt32(pageCount),
	})
}

func (r *PostgresRepository) ApplyMetadata(ctx context.Context, id uuid.UUID, m Metadata, seriesID *uuid.UUID, metadataJSON []byte) error {
	params := sqlc.ApplyComicMetadataParams{
		ID:       id,
		Metadata: metadataJSON,
	}

	if m.Title != "" {
		params.Title = &m.Title
	}
	if m.Number != "" {
		params.Number = &m.Number
	}
	if m.NumberSort != nil {
		params.NumberSort = numericFromFloat(*m.NumberSort)
	}
	if m.Volume != nil {
		params.Volume = m.Volume
	}
	if m.Summary != "" {
		params.Summary = &m.Summary
	}
	if m.Language != "" {
		params.Language = &m.Language
	}
	if m.AgeRating != nil {
		params.AgeRating = m.AgeRating
	}
	if m.Year > 0 {
		params.ReleasedAt = dateFrom(m.Year, m.Month, m.Day)
	}
	if seriesID != nil {
		params.SeriesID = uuid.NullUUID{UUID: *seriesID, Valid: true}
	}

	return r.q.ApplyComicMetadata(ctx, params)
}

func (r *PostgresRepository) SetCoverPlaceholder(ctx context.Context, id uuid.UUID, dataURI string) error {
	var value *string
	if dataURI != "" {
		value = &dataURI
	}
	return r.q.SetComicPlaceholder(ctx, sqlc.SetComicPlaceholderParams{
		ID:               id,
		CoverPlaceholder: value,
	})
}

func (r *PostgresRepository) MarkMissingDeleted(ctx context.Context, libraryID uuid.UUID, seenKeys []string) (int64, error) {
	// Une liste vide effacerait toute la bibliothèque. Cela n'arrive que si le
	// backend a répondu sans erreur mais sans aucun objet — bien plus
	// probablement un montage vide qu'une suppression réelle. On s'abstient.
	if len(seenKeys) == 0 {
		return 0, nil
	}
	return r.q.MarkMissingComicsDeleted(ctx, sqlc.MarkMissingComicsDeletedParams{
		LibraryID: libraryID,
		Column2:   seenKeys,
	})
}

// ─── Pages ───────────────────────────────────────────────────────────────────

// ReplaceComicPages remplace l'index de pages d'un album, atomiquement.
//
// La suppression et les insertions sont dans une même transaction : une
// réindexation interrompue ne laisse jamais un album avec un index partiel, qui
// se lirait avec des pages manquantes au milieu.
func (r *PostgresRepository) ReplaceComicPages(ctx context.Context, comicID uuid.UUID, pages []Page) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }() // sans effet après Commit

	q := r.q.WithTx(tx)

	if err := q.DeleteComicPages(ctx, comicID); err != nil {
		return fmt.Errorf("suppression de l'index de pages : %w", err)
	}

	for _, p := range pages {
		if err := q.InsertComicPage(ctx, sqlc.InsertComicPageParams{
			ComicID:     comicID,
			Index:       toInt32(p.Index),
			EntryName:   p.EntryName,
			DataOffset:  &p.DataOffset,
			DataSize:    &p.DataSize,
			Size:        &p.Size,
			Compression: &p.Compression,
			Width:       int32Ptr(p.Width),
			Height:      int32Ptr(p.Height),
			IsDouble:    p.IsDouble,
		}); err != nil {
			return fmt.Errorf("insertion de la page %d : %w", p.Index, err)
		}
	}

	return tx.Commit(ctx)
}

// ─── Séries ──────────────────────────────────────────────────────────────────

func (r *PostgresRepository) UpsertSeries(ctx context.Context, libraryID uuid.UUID, name, sortName string) (uuid.UUID, error) {
	row, err := r.q.UpsertSeries(ctx, sqlc.UpsertSeriesParams{
		ID:        uuid.Must(uuid.NewV7()),
		LibraryID: libraryID,
		Name:      name,
		SortName:  sortName,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return row.ID, nil
}

/*
RefreshSeriesCounts recalcule les compteurs et retire les séries vidées.

L'élagage accompagne le recalcul plutôt que de vivre à part : une série sans
album n'a aucune raison de figurer dans la barre latérale, et un compteur à zéro
qui subsiste ressemble à un défaut d'affichage plutôt qu'à une donnée périmée.
*/
func (r *PostgresRepository) RefreshSeriesCounts(ctx context.Context, libraryID uuid.UUID) error {
	if err := r.q.RefreshSeriesCounts(ctx, libraryID); err != nil {
		return err
	}
	_, err := r.q.PruneEmptySeries(ctx, libraryID)
	return err
}

// ─── Bibliothèques et scans ──────────────────────────────────────────────────

func (r *PostgresRepository) SetLibraryScanResult(ctx context.Context, libraryID uuid.UUID, status string) error {
	return r.q.SetLibraryScanResult(ctx, sqlc.SetLibraryScanResultParams{
		LibraryID:      libraryID,
		LastScanStatus: &status,
	})
}

func (r *PostgresRepository) StartScanRun(ctx context.Context, libraryID uuid.UUID) (ScanRun, error) {
	row, err := r.q.StartScanRun(ctx, sqlc.StartScanRunParams{
		ID:        uuid.Must(uuid.NewV7()),
		LibraryID: libraryID,
	})
	if err != nil {
		return ScanRun{}, err
	}
	return ScanRun{ID: row.ID}, nil
}

func (r *PostgresRepository) FinishScanRun(ctx context.Context, runID uuid.UUID, status string, stats ScanStats, detail string) error {
	detailJSON := []byte("{}")
	if detail != "" {
		if b, err := json.Marshal(map[string]string{"error": detail}); err == nil {
			detailJSON = b
		}
	}

	return r.q.FinishScanRun(ctx, sqlc.FinishScanRunParams{
		ID:          runID,
		Status:      status,
		ObjectsSeen: toInt32(stats.ObjectsSeen),
		Added:       toInt32(stats.Added),
		Updated:     toInt32(stats.Updated),
		Removed:     toInt32(stats.Removed),
		Errors:      toInt32(stats.Errors),
		Detail:      detailJSON,
	})
}

func (r *PostgresRepository) EnqueueIndexComic(ctx context.Context, comicID uuid.UUID) error {
	return r.enqueuer.EnqueueIndexComic(ctx, comicID)
}

// ─── Conversions ─────────────────────────────────────────────────────────────

func int32Ptr(v int) *int32 {
	if v == 0 {
		return nil
	}
	n := toInt32(v)
	return &n
}

// numericFromFloat convertit vers le numeric(10,3) de la colonne number_sort.
//
// On passe par des millièmes entiers plutôt que par un flottant : la colonne a
// trois décimales, et un arrondi explicite vaut mieux qu'une conversion
// implicite dont le résultat dépendrait de la représentation binaire.
func numericFromFloat(f float64) pgtype.Numeric {
	const scale = 3
	milli := int64(f*1000 + 0.5)
	if f < 0 {
		milli = int64(f*1000 - 0.5)
	}
	return pgtype.Numeric{
		Int:   big.NewInt(milli),
		Exp:   -scale,
		Valid: true,
	}
}

func dateFrom(year, month, day int) pgtype.Date {
	if year <= 0 {
		return pgtype.Date{}
	}
	if month < 1 || month > 12 {
		month = 1
	}
	if day < 1 || day > 31 {
		day = 1
	}
	return pgtype.Date{
		Time:  time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
}
