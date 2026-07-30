package reader

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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

func (r *PostgresRepository) GetComic(ctx context.Context, id uuid.UUID) (Comic, error) {
	row, err := r.q.GetComic(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Comic{}, ErrNotFound
	}
	if err != nil {
		return Comic{}, err
	}
	if row.DeletedAt.Valid {
		// L'objet a disparu du backend : la ligne existe encore pour préserver
		// la progression de lecture, mais il n'y a plus rien à servir.
		return Comic{}, ErrNotFound
	}

	return Comic{
		ID:        row.ID,
		LibraryID: row.LibraryID,
		ObjectKey: row.ObjectKey,
		Format:    string(row.Format),
		State:     string(row.State),
		PageCount: row.PageCount,
		CoverPage: row.CoverPage,
	}, nil
}

// GetPage lit les coordonnées d'accès aléatoire d'une page.
//
// ★ La requête du chemin chaud : un seul aller-retour en base, puis un seul
// ReadRange sur le backend.
func (r *PostgresRepository) GetPage(ctx context.Context, comicID uuid.UUID, index int32) (Page, error) {
	row, err := r.q.GetComicPage(ctx, sqlc.GetComicPageParams{ComicID: comicID, Index: index})
	if errors.Is(err, pgx.ErrNoRows) {
		return Page{}, ErrNotFound
	}
	if err != nil {
		return Page{}, err
	}
	return pageFromRow(row), nil
}

func (r *PostgresRepository) ListPages(ctx context.Context, comicID uuid.UUID) ([]Page, error) {
	rows, err := r.q.ListComicPages(ctx, comicID)
	if err != nil {
		return nil, err
	}

	out := make([]Page, 0, len(rows))
	for _, row := range rows {
		out = append(out, pageFromRow(row))
	}
	return out, nil
}

func pageFromRow(row sqlc.ComicPage) Page {
	p := Page{
		Index:     row.Index,
		EntryName: row.EntryName,
		Width:     row.Width,
		Height:    row.Height,
		IsDouble:  row.IsDouble,
	}
	if row.DataOffset != nil {
		p.DataOffset = *row.DataOffset
	}
	if row.DataSize != nil {
		p.DataSize = *row.DataSize
	}
	if row.Size != nil {
		p.Size = *row.Size
	}
	if row.Compression != nil {
		p.Compression = *row.Compression
	}
	return p
}
