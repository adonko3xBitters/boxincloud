package amule

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sqlc"
)

// PostgresRepository persiste la configuration du démon.
type PostgresRepository struct {
	q *sqlc.Queries
}

func NewPostgresRepository(q *sqlc.Queries) *PostgresRepository {
	return &PostgresRepository{q: q}
}

var _ Repository = (*PostgresRepository)(nil)

func (r *PostgresRepository) GetDaemon(ctx context.Context) (StoredDaemon, error) {
	row, err := r.q.GetEd2kDaemon(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return StoredDaemon{}, ErrNoDaemonRow
	}
	if err != nil {
		return StoredDaemon{}, err
	}

	stored := StoredDaemon{
		Host:        row.Host,
		Port:        int(row.Port),
		PasswordEnc: row.PasswordEnc,
		Label:       deref(row.Label),
		LastState:   deref(row.LastState),
		LastDetail:  deref(row.LastDetail),
	}
	if row.LastSeenAt.Valid {
		seen := row.LastSeenAt.Time
		stored.LastSeenAt = &seen
	}
	return stored, nil
}

func (r *PostgresRepository) SaveDaemon(ctx context.Context, d StoredDaemon) error {
	_, err := r.q.UpsertEd2kDaemon(ctx, sqlc.UpsertEd2kDaemonParams{
		Host:        d.Host,
		Port:        int32(d.Port), //nolint:gosec // borné à 65535 par validateDaemon et par un CHECK en base
		PasswordEnc: d.PasswordEnc,
		Label:       nilIfEmpty(d.Label),
	})
	return err
}

func (r *PostgresRepository) DeleteDaemon(ctx context.Context) error {
	return r.q.DeleteEd2kDaemon(ctx)
}

func (r *PostgresRepository) SetState(ctx context.Context, state State, detail string, seen bool) error {
	value := string(state)
	return r.q.SetEd2kDaemonState(ctx, sqlc.SetEd2kDaemonStateParams{
		State:  &value,
		Detail: nilIfEmpty(detail),
		Seen:   seen,
	})
}

// nilIfEmpty distingue « pas de valeur » de « chaîne vide ».
//
// La colonne est nullable ; y écrire une chaîne vide ferait afficher un libellé
// vide là où l'interface doit ne rien montrer du tout.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
