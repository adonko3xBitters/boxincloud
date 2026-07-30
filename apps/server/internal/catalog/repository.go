package catalog

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sqlc"
)

// PostgresRepository implémente Repository sur les requêtes générées.
type PostgresRepository struct {
	q *sqlc.Queries
}

var _ Repository = (*PostgresRepository)(nil)

func NewPostgresRepository(q *sqlc.Queries) *PostgresRepository {
	return &PostgresRepository{q: q}
}

// ─── Bibliothèques ───────────────────────────────────────────────────────────

func (r *PostgresRepository) ListVisibleLibraries(ctx context.Context, v Viewer) ([]Library, error) {
	rows, err := r.q.ListVisibleLibraries(ctx, sqlc.ListVisibleLibrariesParams{
		UserID:  v.UserID,
		Column2: v.IsAdmin,
	})
	if err != nil {
		return nil, err
	}

	out := make([]Library, 0, len(rows))
	for _, row := range rows {
		out = append(out, Library{
			ID:         row.ID,
			Name:       row.Name,
			Kind:       string(row.Kind),
			ComicCount: row.ComicCount,
		})
	}
	return out, nil
}

func (r *PostgresRepository) CanAccessLibrary(ctx context.Context, v Viewer, libraryID uuid.UUID) (bool, error) {
	allowed, err := r.q.CanAccessLibrary(ctx, sqlc.CanAccessLibraryParams{
		UserID:    v.UserID,
		Column2:   v.IsAdmin,
		LibraryID: libraryID,
	})
	if err != nil {
		return false, err
	}
	// L'expression SQL peut rendre NULL si la bibliothèque n'existe pas.
	// En cas de doute, on refuse : un accès n'est accordé que sur un oui franc.
	return allowed != nil && *allowed, nil
}

// ─── Albums ──────────────────────────────────────────────────────────────────

func (r *PostgresRepository) ListComics(ctx context.Context, p ListComicsParams) ([]Comic, error) {
	params := sqlc.ListComicsPageParams{
		UserID:        p.UserID,
		LibraryIds:    p.LibraryIDs,
		State:         p.State,
		ReadStatus:    p.ReadStatus,
		Folder:        p.Folder,
		FavoritesOnly: p.FavoritesOnly,
		Sort:          string(p.Sort),
		MaxAgeRating:  p.MaxAgeRating,
		PageSize:      p.Limit,
	}
	if p.SeriesID != nil {
		params.SeriesID = uuid.NullUUID{UUID: *p.SeriesID, Valid: true}
	}
	if c := p.Cursor; c != nil {
		params.CursorID = uuid.NullUUID{UUID: c.ID, Valid: true}

		// Seul le champ correspondant au tri est renseigné : les autres restent
		// NULL, ce qui neutralise leur branche dans la clause de curseur.
		switch c.Sort {
		case Sort("title"):
			params.CursorTitle = &c.Title
		case Sort("released"):
			if c.ReleasedAt != nil {
				params.CursorReleased = pgtype.Date{Time: *c.ReleasedAt, Valid: true}
			}
		default:
			params.CursorCreatedAt = pgtype.Timestamptz{Time: c.CreatedAt, Valid: true}
		}
	}

	rows, err := r.q.ListComicsPage(ctx, params)
	if err != nil {
		return nil, err
	}

	out := make([]Comic, 0, len(rows))
	for _, row := range rows {
		out = append(out, comicWithSeries(row.Comic, row.SeriesName))
	}
	return out, nil
}

func (r *PostgresRepository) SearchComics(ctx context.Context, p SearchParams) ([]Comic, error) {
	rows, err := r.q.SearchComics(ctx, sqlc.SearchComicsParams{
		LibraryIds:   p.LibraryIDs,
		Query:        p.Query,
		MaxAgeRating: p.MaxAgeRating,
		PageSize:     p.Limit,
	})
	if err != nil {
		return nil, err
	}

	out := make([]Comic, 0, len(rows))
	for _, row := range rows {
		out = append(out, comicWithSeries(row.Comic, row.SeriesName))
	}
	return out, nil
}

func (r *PostgresRepository) GetComic(ctx context.Context, id uuid.UUID) (Comic, error) {
	row, err := r.q.GetComicDetail(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Comic{}, ErrNotFound
	}
	if err != nil {
		return Comic{}, err
	}

	return comicWithSeries(row.Comic, row.SeriesName), nil
}

func (r *PostgresRepository) ListComicsBySeries(ctx context.Context, seriesID uuid.UUID) ([]Comic, error) {
	rows, err := r.q.ListComicsBySeries(ctx, uuid.NullUUID{UUID: seriesID, Valid: true})
	if err != nil {
		return nil, err
	}

	out := make([]Comic, 0, len(rows))
	for _, row := range rows {
		out = append(out, comicWithSeries(row.Comic, row.SeriesName))
	}
	return out, nil
}

func (r *PostgresRepository) ListRecent(ctx context.Context, p ListComicsParams) ([]Comic, error) {
	rows, err := r.q.ListRecentComics(ctx, sqlc.ListRecentComicsParams{
		LibraryIds:   p.LibraryIDs,
		MaxAgeRating: p.MaxAgeRating,
		PageSize:     p.Limit,
	})
	if err != nil {
		return nil, err
	}

	out := make([]Comic, 0, len(rows))
	for _, row := range rows {
		out = append(out, comicWithSeries(row.Comic, row.SeriesName))
	}
	return out, nil
}

func (r *PostgresRepository) ListNextInSeries(ctx context.Context, v Viewer, libraryIDs []uuid.UUID, limit int32) ([]Comic, error) {
	rows, err := r.q.ListNextInSeries(ctx, sqlc.ListNextInSeriesParams{
		LibraryIds: libraryIDs,
		UserID:     v.UserID,
		PageSize:   limit,
	})
	if err != nil {
		return nil, err
	}

	out := make([]Comic, 0, len(rows))
	for _, row := range rows {
		out = append(out, comicWithSeries(row.Comic, row.SeriesName))
	}
	return out, nil
}

// ─── Séries ──────────────────────────────────────────────────────────────────

func (r *PostgresRepository) ListSeries(ctx context.Context, p ListSeriesParams) ([]Series, error) {
	rows, err := r.q.ListSeriesPage(ctx, sqlc.ListSeriesPageParams{
		LibraryIds:     p.LibraryIDs,
		CursorSortName: p.AfterSort,
		PageSize:       p.Limit,
	})
	if err != nil {
		return nil, err
	}
	return seriesFromRows(rows), nil
}

func (r *PostgresRepository) SearchSeries(ctx context.Context, p SearchParams) ([]Series, error) {
	rows, err := r.q.SearchSeries(ctx, sqlc.SearchSeriesParams{
		LibraryIds: p.LibraryIDs,
		Query:      p.Query,
		// Recherche par préfixe en complément : taper « ast » doit proposer
		// « Astérix » avant même que la similarité trigramme ne s'active.
		Prefix:   p.Query + "%",
		PageSize: p.Limit,
	})
	if err != nil {
		return nil, err
	}

	out := make([]Series, 0, len(rows))
	for _, row := range rows {
		out = append(out, seriesFromRow(sqlc.Series{
			ID: row.ID, LibraryID: row.LibraryID, Name: row.Name,
			SortName: row.SortName, Description: row.Description,
			Publisher: row.Publisher, ComicCount: row.ComicCount,
			CoverComicID: row.CoverComicID,
		}))
	}
	return out, nil
}

func (r *PostgresRepository) GetSeries(ctx context.Context, id uuid.UUID) (Series, error) {
	row, err := r.q.GetSeries(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Series{}, ErrNotFound
	}
	if err != nil {
		return Series{}, err
	}
	return seriesFromRow(row), nil
}

// ─── Conversions ─────────────────────────────────────────────────────────────

// comicWithSeries assemble un album et le nom de sa série.
//
// Le nom arrive d'un LEFT JOIN ramené à la chaîne vide : un album sans série
// n'est pas une anomalie mais le cas courant des one-shots.
func comicWithSeries(row sqlc.Comic, seriesName string) Comic {
	c := comicFromRow(row)
	c.SeriesName = seriesName
	return c
}

func comicFromRow(row sqlc.Comic) Comic {
	c := Comic{
		ID:        row.ID,
		LibraryID: row.LibraryID,
		Title:     row.Title,
		Volume:    row.Volume,
		Format:    string(row.Format),
		PageCount: row.PageCount,
		State:     string(row.State),
		AgeRating: row.AgeRating,
		FileSize:  row.FileSize,
	}
	if row.SeriesID.Valid {
		id := row.SeriesID.UUID
		c.SeriesID = &id
	}
	if row.Number != nil {
		c.Number = *row.Number
	}
	if row.Summary != nil {
		c.Summary = *row.Summary
	}
	if row.Language != nil {
		c.Language = *row.Language
	}
	if row.CoverPlaceholder != nil {
		c.CoverPlaceholder = *row.CoverPlaceholder
	}
	c.FolderPath = row.FolderPath
	c.FileName = row.ObjectKey
	if idx := strings.LastIndex(row.ObjectKey, "/"); idx >= 0 {
		c.FileName = row.ObjectKey[idx+1:]
	}
	if row.ReleasedAt.Valid {
		t := row.ReleasedAt.Time
		c.ReleasedAt = &t
	}
	if row.CreatedAt.Valid {
		c.CreatedAt = row.CreatedAt.Time
	} else {
		c.CreatedAt = time.Time{}
	}
	return c
}

func seriesFromRows(rows []sqlc.Series) []Series {
	out := make([]Series, 0, len(rows))
	for _, row := range rows {
		out = append(out, seriesFromRow(row))
	}
	return out
}

func seriesFromRow(row sqlc.Series) Series {
	s := Series{
		ID:         row.ID,
		LibraryID:  row.LibraryID,
		Name:       row.Name,
		SortName:   row.SortName,
		ComicCount: row.ComicCount,
	}
	if row.Description != nil {
		s.Description = *row.Description
	}
	if row.Publisher != nil {
		s.Publisher = *row.Publisher
	}
	if row.CoverComicID.Valid {
		id := row.CoverComicID.UUID
		s.CoverComicID = &id
	}
	return s
}
