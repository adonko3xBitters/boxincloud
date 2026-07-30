package accounts

import (
	"context"
	"errors"
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

func (r *PostgresRepository) List(ctx context.Context) ([]Account, error) {
	rows, err := r.q.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]Account, 0, len(rows))
	for _, row := range rows {
		out = append(out, accountFromRow(row))
	}
	return out, nil
}

func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (Account, error) {
	row, err := r.q.GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, err
	}
	return accountFromRow(row), nil
}

func (r *PostgresRepository) CountAdmins(ctx context.Context) (int64, error) {
	return r.q.CountAdmins(ctx)
}

func (r *PostgresRepository) UpdateProfile(
	ctx context.Context, id uuid.UUID, displayName, email *string,
) (Account, error) {
	row, err := r.q.UpdateUserProfile(ctx, sqlc.UpdateUserProfileParams{
		ID:          id,
		DisplayName: displayName,
		Email:       email,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, err
	}
	return accountFromRow(row), nil
}

func (r *PostgresRepository) SetRole(ctx context.Context, id uuid.UUID, role string) (Account, error) {
	row, err := r.q.SetUserRoleReturning(ctx, sqlc.SetUserRoleReturningParams{
		ID:   id,
		Role: sqlc.UserRole(role),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, err
	}
	return accountFromRow(row), nil
}

func (r *PostgresRepository) SetRestriction(
	ctx context.Context, id uuid.UUID, restricted bool, maxAgeRating *int16,
) (Account, error) {
	row, err := r.q.SetUserRestriction(ctx, sqlc.SetUserRestrictionParams{
		ID:           id,
		Restricted:   restricted,
		MaxAgeRating: maxAgeRating,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, err
	}
	return accountFromRow(row), nil
}

func (r *PostgresRepository) SetPassword(ctx context.Context, id uuid.UUID, hash string) error {
	return r.q.SetUserPassword(ctx, sqlc.SetUserPasswordParams{ID: id, PasswordHash: hash})
}

func (r *PostgresRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.q.SoftDeleteUser(ctx, id)
}

func (r *PostgresRepository) GrantsForUser(ctx context.Context, userID uuid.UUID) ([]LibraryGrant, error) {
	rows, err := r.q.ListAccessByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return grantsFromRows(rows), nil
}

func (r *PostgresRepository) GrantsForLibrary(ctx context.Context, libraryID uuid.UUID) ([]LibraryGrant, error) {
	rows, err := r.q.ListLibraryAccess(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	return grantsFromRows(rows), nil
}

func (r *PostgresRepository) Grant(ctx context.Context, libraryID, userID uuid.UUID, canWrite bool) error {
	return r.q.GrantLibraryAccess(ctx, sqlc.GrantLibraryAccessParams{
		LibraryID: libraryID,
		UserID:    userID,
		CanWrite:  canWrite,
	})
}

func (r *PostgresRepository) Revoke(ctx context.Context, libraryID, userID uuid.UUID) error {
	return r.q.RevokeLibraryAccess(ctx, sqlc.RevokeLibraryAccessParams{
		LibraryID: libraryID,
		UserID:    userID,
	})
}

// ─── Conversions ─────────────────────────────────────────────────────────────

func accountFromRow(row sqlc.User) Account {
	a := Account{
		ID:           row.ID,
		Username:     row.Username,
		Role:         string(row.Role),
		Restricted:   row.Restricted,
		MaxAgeRating: row.MaxAgeRating,
		CreatedAt:    formatTime(row.CreatedAt),
	}
	if row.Email != nil {
		a.Email = *row.Email
	}
	if row.DisplayName != nil {
		a.DisplayName = *row.DisplayName
	}
	if row.LastLoginAt.Valid {
		formatted := row.LastLoginAt.Time.UTC().Format(time.RFC3339)
		a.LastLoginAt = &formatted
	}
	return a
}

func grantsFromRows(rows []sqlc.LibraryAccess) []LibraryGrant {
	out := make([]LibraryGrant, 0, len(rows))
	for _, row := range rows {
		out = append(out, LibraryGrant{
			LibraryID: row.LibraryID,
			UserID:    row.UserID,
			CanWrite:  row.CanWrite,
		})
	}
	return out
}

func formatTime(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}
