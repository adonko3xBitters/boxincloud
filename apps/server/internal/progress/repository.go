package progress

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sqlc"
)

// ErrNotFound signale une progression inexistante.
var ErrNotFound = errors.New("progress : aucune progression enregistrée")

// PostgresRepository implémente Repository sur les requêtes générées.
type PostgresRepository struct {
	q *sqlc.Queries
}

var _ Repository = (*PostgresRepository)(nil)

func NewPostgresRepository(q *sqlc.Queries) *PostgresRepository {
	return &PostgresRepository{q: q}
}

// Upsert applique une progression.
//
// La règle de résolution de conflit — page la plus avancée gagne — vit dans la
// clause ON CONFLICT de la requête, pas ici. C'est ce qui garantit qu'aucun
// chemin de code ne peut la contourner.
func (r *PostgresRepository) Upsert(ctx context.Context, userID uuid.UUID, u Update) (Progress, error) {
	var deviceID uuid.NullUUID
	if u.DeviceID != nil && *u.DeviceID != uuid.Nil {
		deviceID = uuid.NullUUID{UUID: *u.DeviceID, Valid: true}
	}

	row, err := r.q.UpsertReadingProgress(ctx, sqlc.UpsertReadingProgressParams{
		UserID:    userID,
		ComicID:   u.ComicID,
		Page:      u.Page,
		PageCount: u.PageCount,
		Status:    sqlc.ReadStatus(u.Status),
		DeviceID:  deviceID,
	})
	if err != nil {
		return Progress{}, err
	}
	return progressFromRow(row), nil
}

func (r *PostgresRepository) Get(ctx context.Context, userID, comicID uuid.UUID) (Progress, error) {
	row, err := r.q.GetReadingProgress(ctx, sqlc.GetReadingProgressParams{
		UserID:  userID,
		ComicID: comicID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Progress{}, ErrNotFound
	}
	if err != nil {
		return Progress{}, err
	}
	return progressFromRow(row), nil
}

func (r *PostgresRepository) ListByComics(ctx context.Context, userID uuid.UUID, comicIDs []uuid.UUID) ([]Progress, error) {
	rows, err := r.q.ListReadingProgressByComics(ctx, sqlc.ListReadingProgressByComicsParams{
		UserID:  userID,
		Column2: comicIDs,
	})
	if err != nil {
		return nil, err
	}
	return progressFromRows(rows), nil
}

func (r *PostgresRepository) ListSince(ctx context.Context, userID uuid.UUID, since time.Time, limit int32) ([]Progress, error) {
	rows, err := r.q.ListReadingProgressSince(ctx, sqlc.ListReadingProgressSinceParams{
		UserID:    userID,
		UpdatedAt: pgtype.Timestamptz{Time: since, Valid: true},
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	return progressFromRows(rows), nil
}

func (r *PostgresRepository) ListInProgress(ctx context.Context, userID uuid.UUID, limit int32) ([]Progress, error) {
	rows, err := r.q.ListInProgress(ctx, sqlc.ListInProgressParams{
		UserID: userID,
		Limit:  limit,
	})
	if err != nil {
		return nil, err
	}

	out := make([]Progress, 0, len(rows))
	for _, row := range rows {
		out = append(out, progressFromRow(row.ReadingProgress))
	}
	return out, nil
}

func (r *PostgresRepository) Delete(ctx context.Context, userID, comicID uuid.UUID) error {
	return r.q.DeleteReadingProgress(ctx, sqlc.DeleteReadingProgressParams{
		UserID:  userID,
		ComicID: comicID,
	})
}

// ─── Conversions ─────────────────────────────────────────────────────────────

func progressFromRows(rows []sqlc.ReadingProgress) []Progress {
	out := make([]Progress, 0, len(rows))
	for _, row := range rows {
		out = append(out, progressFromRow(row))
	}
	return out
}

func progressFromRow(row sqlc.ReadingProgress) Progress {
	p := Progress{
		ComicID:   row.ComicID,
		Page:      row.Page,
		PageCount: row.PageCount,
		Status:    Status(row.Status),
		ReadCount: row.ReadCount,
		Version:   row.Version,
	}
	if row.DeviceID.Valid {
		id := row.DeviceID.UUID
		p.DeviceID = &id
	}
	if row.StartedAt.Valid {
		t := row.StartedAt.Time
		p.StartedAt = &t
	}
	if row.FinishedAt.Valid {
		t := row.FinishedAt.Time
		p.FinishedAt = &t
	}
	if row.UpdatedAt.Valid {
		p.UpdatedAt = row.UpdatedAt.Time
	}
	return p
}
