package catalog

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sqlc"
)

// PostgresTools implémente ToolsRepository.
type PostgresTools struct {
	q *sqlc.Queries
}

var _ ToolsRepository = (*PostgresTools)(nil)

func NewPostgresTools(q *sqlc.Queries) *PostgresTools {
	return &PostgresTools{q: q}
}

// ─── Dossiers ────────────────────────────────────────────────────────────────

func (r *PostgresTools) ListFolders(ctx context.Context, libraryIDs []uuid.UUID) (map[string]int, error) {
	rows, err := r.q.ListFolders(ctx, libraryIDs)
	if err != nil {
		return nil, err
	}

	out := make(map[string]int, len(rows))
	for _, row := range rows {
		out[row.FolderPath] = int(row.ComicCount)
	}
	return out, nil
}

// ─── Favoris ─────────────────────────────────────────────────────────────────

func (r *PostgresTools) SetFavorite(ctx context.Context, userID, comicID uuid.UUID, favorite bool) error {
	if favorite {
		return r.q.SetFavorite(ctx, sqlc.SetFavoriteParams{UserID: userID, ComicID: comicID})
	}
	return r.q.UnsetFavorite(ctx, sqlc.UnsetFavoriteParams{UserID: userID, ComicID: comicID})
}

func (r *PostgresTools) ListFavorites(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	return r.q.ListFavoriteIDs(ctx, userID)
}

// ─── Notes ───────────────────────────────────────────────────────────────────

func (r *PostgresTools) SetRating(ctx context.Context, userID, comicID uuid.UUID, rating int16) error {
	return r.q.SetRating(ctx, sqlc.SetRatingParams{
		UserID: userID, ComicID: comicID, Rating: rating,
	})
}

func (r *PostgresTools) ClearRating(ctx context.Context, userID, comicID uuid.UUID) error {
	return r.q.ClearRating(ctx, sqlc.ClearRatingParams{UserID: userID, ComicID: comicID})
}

func (r *PostgresTools) ListRatings(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]int16, error) {
	rows, err := r.q.ListRatings(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := make(map[uuid.UUID]int16, len(rows))
	for _, row := range rows {
		out[row.ComicID] = row.Rating
	}
	return out, nil
}

// ─── Édition ─────────────────────────────────────────────────────────────────

func (r *PostgresTools) EditComic(ctx context.Context, id uuid.UUID, edit ComicEdit) (Comic, error) {
	row, err := r.q.EditComic(ctx, sqlc.EditComicParams{
		ID:          id,
		Title:       edit.Title,
		Number:      edit.Number,
		Summary:     edit.Summary,
		Language:    edit.Language,
		NewlyLocked: edit.LockedFields(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Comic{}, ErrNotFound
	}
	if err != nil {
		return Comic{}, err
	}
	return comicFromRow(row), nil
}

// ─── Actions en lot ──────────────────────────────────────────────────────────

func (r *PostgresTools) BulkMarkRead(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) (int64, error) {
	return r.q.BulkMarkRead(ctx, sqlc.BulkMarkReadParams{UserID: userID, Column2: ids})
}

func (r *PostgresTools) BulkMarkUnread(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) (int64, error) {
	return r.q.BulkMarkUnread(ctx, sqlc.BulkMarkUnreadParams{UserID: userID, Column2: ids})
}

func (r *PostgresTools) BulkSetFavorite(ctx context.Context, userID uuid.UUID, ids []uuid.UUID, favorite bool) (int64, error) {
	if favorite {
		return r.q.BulkSetFavorite(ctx, sqlc.BulkSetFavoriteParams{UserID: userID, Column2: ids})
	}
	return r.q.BulkUnsetFavorite(ctx, sqlc.BulkUnsetFavoriteParams{UserID: userID, Column2: ids})
}
