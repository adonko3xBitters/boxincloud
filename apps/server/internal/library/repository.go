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
