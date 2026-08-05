package amule

import (
	"context"
	"errors"

	"github.com/google/uuid"
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

// ─── Pont vers la bibliothèque ───────────────────────────────────────────────

func (r *PostgresRepository) ListDestinations(ctx context.Context) ([]Destination, error) {
	rows, err := r.q.ListEd2kDestinations(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]Destination, 0, len(rows))
	for _, row := range rows {
		out = append(out, destinationFromRow(row))
	}
	return out, nil
}

func (r *PostgresRepository) GetDestination(ctx context.Context, category int) (Destination, error) {
	row, err := r.q.GetEd2kDestination(ctx, int32(category)) //nolint:gosec // borné par un CHECK en base
	if errors.Is(err, pgx.ErrNoRows) {
		return Destination{}, ErrNoDestination
	}
	if err != nil {
		return Destination{}, err
	}
	return destinationFromRow(row), nil
}

func (r *PostgresRepository) SaveDestination(ctx context.Context, d Destination) (Destination, error) {
	row, err := r.q.UpsertEd2kDestination(ctx, sqlc.UpsertEd2kDestinationParams{
		Category:  int32(d.Category), //nolint:gosec // validé par le service
		Label:     d.Label,
		LibraryID: nullUUID(d.LibraryID),
		Folder:    d.Folder,
	})
	if err != nil {
		return Destination{}, err
	}
	return destinationFromRow(row), nil
}

func destinationFromRow(row sqlc.Ed2kDestination) Destination {
	d := Destination{
		Category: int(row.Category),
		Label:    row.Label,
		Folder:   row.Folder,
	}
	if row.LibraryID.Valid {
		id := row.LibraryID.UUID
		d.LibraryID = &id
	}
	return d
}

func (r *PostgresRepository) ClaimPublication(ctx context.Context, p Publication) (bool, error) {
	_, err := r.q.ClaimEd2kPublication(ctx, sqlc.ClaimEd2kPublicationParams{
		Hash:     p.Hash,
		Name:     p.Name,
		Size:     p.Size,
		Category: int32(p.Category), //nolint:gosec // vient du démon, borné en pratique
	})

	// Aucune ligne rendue signifie que l'empreinte était déjà connue : la
	// réservation revient à quelqu'un d'autre, et ce n'est pas une erreur.
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *PostgresRepository) SetPublicationResult(
	ctx context.Context, hash string, result Publication,
) error {
	return r.q.SetEd2kPublicationResult(ctx, sqlc.SetEd2kPublicationResultParams{
		Hash:      hash,
		Status:    string(result.Status),
		Detail:    nilIfEmpty(result.Detail),
		LibraryID: nullUUID(result.LibraryID),
		ComicID:   nullUUID(result.ComicID),
	})
}

func (r *PostgresRepository) ListPublications(ctx context.Context, limit int) ([]Publication, error) {
	rows, err := r.q.ListEd2kPublications(ctx, int32(limit)) //nolint:gosec // borné par le service
	if err != nil {
		return nil, err
	}

	out := make([]Publication, 0, len(rows))
	for _, row := range rows {
		p := Publication{
			Hash:     row.Hash,
			Name:     row.Name,
			Size:     row.Size,
			Category: int(row.Category),
			Status:   PublicationStatus(row.Status),
			Detail:   deref(row.Detail),
		}
		if row.LibraryID.Valid {
			id := row.LibraryID.UUID
			p.LibraryID = &id
		}
		if row.ComicID.Valid {
			id := row.ComicID.UUID
			p.ComicID = &id
		}
		out = append(out, p)
	}
	return out, nil
}

// nullUUID traduit un pointeur en UUID nullable.
//
// Le nil a un sens précis ici — « laisser sur disque » pour une destination,
// « pas encore publié » pour une publication — et le confondre avec l'UUID nul
// produirait une clé étrangère qui ne désigne rien.
func nullUUID(id *uuid.UUID) uuid.NullUUID {
	if id == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *id, Valid: true}
}
