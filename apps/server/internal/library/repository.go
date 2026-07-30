package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sqlc"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
)

// PostgresRepository implémente Repository sur les requêtes générées par sqlc.
//
// Toute la traduction entre les types de la base et ceux du domaine est
// confinée ici : le service ne connaît ni pgtype, ni jsonb.
type PostgresRepository struct {
	q *sqlc.Queries
}

var _ Repository = (*PostgresRepository)(nil)

func NewPostgresRepository(q *sqlc.Queries) *PostgresRepository {
	return &PostgresRepository{q: q}
}

// ─── Backends ────────────────────────────────────────────────────────────────

func (r *PostgresRepository) CreateBackend(ctx context.Context, b Backend, secretsEnc []byte) (Backend, error) {
	config, err := json.Marshal(b.Config)
	if err != nil {
		return Backend{}, fmt.Errorf("library : sérialisation de la configuration : %w", err)
	}

	row, err := r.q.CreateStorageBackend(ctx, sqlc.CreateStorageBackendParams{
		ID:         b.ID,
		Name:       b.Name,
		Kind:       sqlc.StorageKind(b.Kind),
		Config:     config,
		SecretsEnc: secretsEnc,
		IsDefault:  b.IsDefault,
		ReadOnly:   b.ReadOnly,
	})
	if err != nil {
		return Backend{}, err
	}
	return backendFromRow(row)
}

func (r *PostgresRepository) GetBackend(ctx context.Context, id uuid.UUID) (Backend, []byte, error) {
	row, err := r.q.GetStorageBackend(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Backend{}, nil, fmt.Errorf("%w : %s", ErrBackendNotFound, id)
	}
	if err != nil {
		return Backend{}, nil, err
	}
	b, err := backendFromRow(row)
	return b, row.SecretsEnc, err
}

func (r *PostgresRepository) GetBackendByName(ctx context.Context, name string) (Backend, []byte, error) {
	row, err := r.q.GetStorageBackendByName(ctx, name)
	if errors.Is(err, pgx.ErrNoRows) {
		return Backend{}, nil, fmt.Errorf("%w : %q", ErrBackendNotFound, name)
	}
	if err != nil {
		return Backend{}, nil, err
	}
	b, err := backendFromRow(row)
	return b, row.SecretsEnc, err
}

func (r *PostgresRepository) ListBackends(ctx context.Context) ([]Backend, error) {
	rows, err := r.q.ListStorageBackends(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]Backend, 0, len(rows))
	for _, row := range rows {
		b, err := backendFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

func (r *PostgresRepository) SetBackendStatus(ctx context.Context, id uuid.UUID, status, detail string) error {
	var d *string
	if detail != "" {
		d = &detail
	}
	return r.q.SetStorageBackendStatus(ctx, sqlc.SetStorageBackendStatusParams{
		ID:           id,
		Status:       sqlc.StorageStatus(status),
		StatusDetail: d,
	})
}

func backendFromRow(row sqlc.StorageBackend) (Backend, error) {
	config := map[string]string{}
	if len(row.Config) > 0 {
		if err := json.Unmarshal(row.Config, &config); err != nil {
			return Backend{}, fmt.Errorf("library : configuration illisible pour %q : %w", row.Name, err)
		}
	}

	return Backend{
		ID:        row.ID,
		Name:      row.Name,
		Kind:      storage.Kind(row.Kind),
		Config:    config,
		IsDefault: row.IsDefault,
		ReadOnly:  row.ReadOnly,
		Status:    string(row.Status),
	}, nil
}

// ─── Bibliothèques ───────────────────────────────────────────────────────────

func (r *PostgresRepository) CreateLibrary(ctx context.Context, l Library) (Library, error) {
	row, err := r.q.CreateLibrary(ctx, sqlc.CreateLibraryParams{
		ID:               l.ID,
		StorageBackendID: l.BackendID,
		Name:             l.Name,
		Kind:             sqlc.LibraryKind(l.Kind),
		RootPrefix:       l.RootPrefix,
		ScanOptions:      []byte("{}"),
	})
	if err != nil {
		return Library{}, err
	}
	return libraryFromRow(row), nil
}

func (r *PostgresRepository) GetLibrary(ctx context.Context, id uuid.UUID) (Library, error) {
	row, err := r.q.GetLibrary(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Library{}, fmt.Errorf("%w : %s", ErrLibraryNotFound, id)
	}
	if err != nil {
		return Library{}, err
	}
	return libraryFromRow(row), nil
}

func (r *PostgresRepository) GetLibraryByName(ctx context.Context, name string) (Library, error) {
	row, err := r.q.GetLibraryByName(ctx, name)
	if errors.Is(err, pgx.ErrNoRows) {
		return Library{}, fmt.Errorf("%w : %q", ErrLibraryNotFound, name)
	}
	if err != nil {
		return Library{}, err
	}
	return libraryFromRow(row), nil
}

func (r *PostgresRepository) ListLibraries(ctx context.Context) ([]Library, error) {
	rows, err := r.q.ListLibraries(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]Library, 0, len(rows))
	for _, row := range rows {
		out = append(out, libraryFromRow(row))
	}
	return out, nil
}

func libraryFromRow(row sqlc.Library) Library {
	return Library{
		ID:         row.ID,
		BackendID:  row.StorageBackendID,
		Name:       row.Name,
		Kind:       string(row.Kind),
		RootPrefix: row.RootPrefix,
		ComicCount: row.ComicCount,
	}
}

// ─── Administration ──────────────────────────────────────────────────────────

var _ AdminRepository = (*PostgresRepository)(nil)

func (r *PostgresRepository) UpdateBackend(
	ctx context.Context, id uuid.UUID, p UpdateBackendParams,
) (Backend, error) {
	var config []byte
	if p.Config != nil {
		encoded, err := json.Marshal(p.Config)
		if err != nil {
			return Backend{}, err
		}
		config = encoded
	}

	row, err := r.q.UpdateStorageBackend(ctx, sqlc.UpdateStorageBackendParams{
		ID:         id,
		Name:       p.Name,
		Config:     config,
		SecretsEnc: p.SecretsEnc,
		ReadOnly:   p.ReadOnly,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Backend{}, ErrBackendNotFound
	}
	if err != nil {
		return Backend{}, err
	}

	backend, err := backendFromRow(row)
	if err != nil {
		return Backend{}, err
	}
	return backend, nil
}

func (r *PostgresRepository) DeleteBackend(ctx context.Context, id uuid.UUID) error {
	return r.q.DeleteStorageBackend(ctx, id)
}

func (r *PostgresRepository) CountLibrariesUsing(ctx context.Context, backendID uuid.UUID) (int64, error) {
	return r.q.CountLibrariesUsingBackend(ctx, backendID)
}

func (r *PostgresRepository) SetDefaultBackend(ctx context.Context, id uuid.UUID) error {
	// L'ordre compte : l'index unique partiel refuserait deux défauts
	// simultanés, il faut donc retirer avant de poser.
	if err := r.q.ClearDefaultStorageBackend(ctx, id); err != nil {
		return err
	}
	return r.q.SetDefaultStorageBackend(ctx, id)
}

func (r *PostgresRepository) UpdateLibrary(
	ctx context.Context, id uuid.UUID, name, rootPrefix *string,
) (Library, error) {
	row, err := r.q.UpdateLibrary(ctx, sqlc.UpdateLibraryParams{
		ID:         id,
		Name:       name,
		RootPrefix: rootPrefix,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Library{}, ErrLibraryNotFound
	}
	if err != nil {
		return Library{}, err
	}
	return libraryFromRow(row), nil
}

func (r *PostgresRepository) DeleteLibrary(ctx context.Context, id uuid.UUID) error {
	return r.q.DeleteLibrary(ctx, id)
}

func (r *PostgresRepository) ScanRuns(
	ctx context.Context, libraryID uuid.UUID, limit int32,
) ([]ScanRun, error) {
	rows, err := r.q.ListScanRuns(ctx, sqlc.ListScanRunsParams{
		LibraryID: libraryID,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}

	out := make([]ScanRun, 0, len(rows))
	for _, row := range rows {
		run := ScanRun{
			ID:          row.ID,
			Status:      row.Status,
			ObjectsSeen: int(row.ObjectsSeen),
			Added:       int(row.Added),
			Updated:     int(row.Updated),
			Removed:     int(row.Removed),
			Errors:      int(row.Errors),
		}
		if row.StartedAt.Valid {
			run.StartedAt = row.StartedAt.Time
		}
		if row.FinishedAt.Valid {
			t := row.FinishedAt.Time
			run.FinishedAt = &t
		}
		if len(row.Detail) > 0 {
			run.Detail = string(row.Detail)
		}
		out = append(out, run)
	}
	return out, nil
}
