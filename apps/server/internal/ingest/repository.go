package ingest

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/adonko3xBitters/boxincloud/server/internal/indexer"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sqlc"
)

// PostgresManage implémente ManageRepository sur les requêtes générées.
type PostgresManage struct {
	q *sqlc.Queries
}

var _ ManageRepository = (*PostgresManage)(nil)

func NewPostgresManage(q *sqlc.Queries) *PostgresManage {
	return &PostgresManage{q: q}
}

// GetComic lit un album, y compris exclu.
//
// Volontairement sans filtre sur `excluded_at` : la suppression et le
// déplacement doivent pouvoir travailler sur un album déjà retiré du catalogue,
// ne serait-ce que pour effacer son fichier après coup.
func (r *PostgresManage) GetComic(ctx context.Context, id uuid.UUID) (indexer.Comic, error) {
	row, err := r.q.GetComic(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return indexer.Comic{}, ErrComicNotFound
	}
	if err != nil {
		return indexer.Comic{}, err
	}

	return indexer.Comic{
		ID:        row.ID,
		LibraryID: row.LibraryID,
		ObjectKey: row.ObjectKey,
		FileSize:  row.FileSize,
		Format:    string(row.Format),
		State:     string(row.State),
	}, nil
}

func (r *PostgresManage) ExcludeComic(ctx context.Context, id uuid.UUID) error {
	return r.q.ExcludeComic(ctx, id)
}

func (r *PostgresManage) PurgeComic(ctx context.Context, id uuid.UUID) error {
	return r.q.PurgeComic(ctx, id)
}

func (r *PostgresManage) MoveComic(ctx context.Context, id uuid.UUID, objectKey, folderPath string) error {
	return r.q.MoveComic(ctx, sqlc.MoveComicParams{
		ID:         id,
		ObjectKey:  objectKey,
		FolderPath: folderPath,
	})
}
