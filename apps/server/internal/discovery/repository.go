package discovery

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sqlc"
)

// PostgresRepository implémente Repository sur les requêtes générées par sqlc.
//
// Comme ailleurs, toute la traduction entre les types de la base et ceux du
// domaine est confinée ici : le service ne connaît pas pgtype.
type PostgresRepository struct {
	q *sqlc.Queries
}

var _ Repository = (*PostgresRepository)(nil)

func NewPostgresRepository(q *sqlc.Queries) *PostgresRepository {
	return &PostgresRepository{q: q}
}

func (r *PostgresRepository) ListSources(ctx context.Context) ([]Source, error) {
	rows, err := r.q.ListDiscoverySources(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovery : liste des catalogues : %w", err)
	}

	out := make([]Source, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSource(row))
	}
	return out, nil
}

func (r *PostgresRepository) GetSource(ctx context.Context, id uuid.UUID) (Source, error) {
	row, err := r.q.GetDiscoverySource(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Source{}, ErrSourceNotFound
	}
	if err != nil {
		return Source{}, fmt.Errorf("discovery : lecture du catalogue : %w", err)
	}
	return toSource(row), nil
}

func (r *PostgresRepository) SourceSecret(ctx context.Context, id uuid.UUID) ([]byte, error) {
	secret, err := r.q.GetDiscoverySourceSecret(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSourceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("discovery : lecture des identifiants : %w", err)
	}
	return secret, nil
}

func (r *PostgresRepository) CreateSource(
	ctx context.Context, s Source, secret []byte,
) (Source, error) {
	row, err := r.q.CreateDiscoverySource(ctx, sqlc.CreateDiscoverySourceParams{
		ID:        uuid.New(),
		Name:      s.Name,
		URL:       s.URL,
		Kind:      string(s.Kind),
		Enabled:   s.Enabled,
		Username:  s.Username,
		SecretEnc: secret,
		Template:  s.Template,
	})
	if err != nil {
		return Source{}, fmt.Errorf("discovery : création du catalogue : %w", err)
	}
	return toSource(row), nil
}

func (r *PostgresRepository) UpdateSource(
	ctx context.Context, s Source, secret []byte, replaceSecret bool,
) (Source, error) {
	row, err := r.q.UpdateDiscoverySource(ctx, sqlc.UpdateDiscoverySourceParams{
		ID:            s.ID,
		Name:          s.Name,
		URL:           s.URL,
		Enabled:       s.Enabled,
		Username:      s.Username,
		ReplaceSecret: replaceSecret,
		SecretEnc:     secret,
		// Nul conserve les règles en place : la requête applique un COALESCE.
		// Une source `web` ne peut pas les perdre par distraction.
		Template: s.Template,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Source{}, ErrSourceNotFound
	}
	if err != nil {
		return Source{}, fmt.Errorf("discovery : mise à jour du catalogue : %w", err)
	}
	return toSource(row), nil
}

func (r *PostgresRepository) DeleteSource(ctx context.Context, id uuid.UUID) error {
	if err := r.q.DeleteDiscoverySource(ctx, id); err != nil {
		return fmt.Errorf("discovery : suppression du catalogue : %w", err)
	}
	return nil
}

func (r *PostgresRepository) RecordProbe(ctx context.Context, id uuid.UUID, failure string) error {
	err := r.q.RecordDiscoveryProbe(ctx, sqlc.RecordDiscoveryProbeParams{
		ID:        id,
		LastError: failure,
	})
	if err != nil {
		return fmt.Errorf("discovery : enregistrement de l'état : %w", err)
	}
	return nil
}

func toSource(row sqlc.DiscoverySource) Source {
	source := Source{
		ID:        row.ID,
		Name:      row.Name,
		URL:       row.URL,
		Kind:      Kind(row.Kind),
		Enabled:   row.Enabled,
		Username:  row.Username,
		Template:  row.Template,
		LastError: row.LastError,
	}
	if row.LastCheckedAt.Valid {
		checked := row.LastCheckedAt.Time
		source.LastCheckAt = &checked
	}
	if row.CreatedAt.Valid {
		source.CreatedAt = row.CreatedAt.Time
	} else {
		source.CreatedAt = time.Time{}
	}
	return source
}

// ─── Imports ─────────────────────────────────────────────────────────────────

func (r *PostgresRepository) CreateImport(ctx context.Context, i Import) (Import, error) {
	row, err := r.q.CreateDiscoveryImport(ctx, sqlc.CreateDiscoveryImportParams{
		ID:          uuid.New(),
		SourceID:    nullUUID(i.SourceID),
		SourceName:  i.SourceName,
		Href:        i.Href,
		LibraryID:   i.LibraryID,
		Folder:      i.Folder,
		Title:       i.Title,
		RequestedBy: nullUUID(i.RequestedBy),
	})
	if err != nil {
		return Import{}, fmt.Errorf("discovery : création de l'import : %w", err)
	}
	return toImport(row), nil
}

func (r *PostgresRepository) GetImport(ctx context.Context, id uuid.UUID) (Import, error) {
	row, err := r.q.GetDiscoveryImport(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Import{}, ErrImportNotFound
	}
	if err != nil {
		return Import{}, fmt.Errorf("discovery : lecture de l'import : %w", err)
	}
	return toImport(row), nil
}

// maxImportRows borne ce qu'une lecture d'imports peut rapporter.
//
// Le service applique déjà sa propre borne, plus basse. Celle-ci est ailleurs
// et sert à autre chose : le dépôt est exporté, `limit` descend d'un paramètre
// de requête, et le convertir en int32 sans le borner est exactement ce qu'un
// analyseur statique signale à raison. Borner ici rend le dépôt sûr quel que
// soit l'appelant, plutôt que sûr par convention.
const maxImportRows = 500

func (r *PostgresRepository) ListImports(ctx context.Context, limit int) ([]Import, error) {
	if limit < 0 {
		limit = 0
	}
	if limit > maxImportRows {
		limit = maxImportRows
	}

	rows, err := r.q.ListDiscoveryImports(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("discovery : liste des imports : %w", err)
	}

	out := make([]Import, 0, len(rows))
	for _, row := range rows {
		out = append(out, toImport(row))
	}
	return out, nil
}

func (r *PostgresRepository) StartImport(ctx context.Context, id uuid.UUID) error {
	if err := r.q.StartDiscoveryImport(ctx, id); err != nil {
		return fmt.Errorf("discovery : démarrage de l'import : %w", err)
	}
	return nil
}

func (r *PostgresRepository) FinishImport(
	ctx context.Context, id uuid.UUID, d Deposited,
) error {
	err := r.q.FinishDiscoveryImport(ctx, sqlc.FinishDiscoveryImportParams{
		ID:        id,
		ComicID:   uuid.NullUUID{UUID: d.ComicID, Valid: d.ComicID != uuid.Nil},
		ObjectKey: d.ObjectKey,
		FileSize:  d.Size,
	})
	if err != nil {
		return fmt.Errorf("discovery : clôture de l'import : %w", err)
	}
	return nil
}

func (r *PostgresRepository) FailImport(
	ctx context.Context, id uuid.UUID, code, detail string,
) error {
	err := r.q.FailDiscoveryImport(ctx, sqlc.FailDiscoveryImportParams{
		ID:        id,
		ErrorCode: code,
		// Le diagnostic vient d'un serveur tiers : sa longueur n'est pas sous
		// notre contrôle, et une colonne de texte n'a pas à recevoir la page
		// d'erreur HTML de quelqu'un d'autre.
		ErrorDetail: truncate(detail, 2000),
	})
	if err != nil {
		return fmt.Errorf("discovery : échec de l'import non enregistré : %w", err)
	}
	return nil
}

func toImport(row sqlc.DiscoveryImport) Import {
	out := Import{
		ID:          row.ID,
		SourceName:  row.SourceName,
		Href:        row.Href,
		LibraryID:   row.LibraryID,
		Folder:      row.Folder,
		Title:       row.Title,
		Status:      ImportStatus(row.Status),
		ErrorCode:   row.ErrorCode,
		ErrorDetail: row.ErrorDetail,
		ObjectKey:   row.ObjectKey,
		FileSize:    row.FileSize,
	}
	if row.SourceID.Valid {
		id := row.SourceID.UUID
		out.SourceID = &id
	}
	if row.ComicID.Valid {
		id := row.ComicID.UUID
		out.ComicID = &id
	}
	if row.RequestedBy.Valid {
		id := row.RequestedBy.UUID
		out.RequestedBy = &id
	}
	if row.CreatedAt.Valid {
		out.CreatedAt = row.CreatedAt.Time
	}
	if row.StartedAt.Valid {
		at := row.StartedAt.Time
		out.StartedAt = &at
	}
	if row.FinishedAt.Valid {
		at := row.FinishedAt.Time
		out.FinishedAt = &at
	}
	return out
}

func nullUUID(id *uuid.UUID) uuid.NullUUID {
	if id == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *id, Valid: true}
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

// ─── Enrichissement ──────────────────────────────────────────────────────────

var _ ComicWriter = (*PostgresRepository)(nil)

/*
Enrich complète un album sans jamais écraser.

La garantie est dans le SQL, pas ici : seuls les champs vides et non verrouillés
par une saisie manuelle sont touchés. La placer dans la requête plutôt que dans
ce code la rend impossible à contourner par un appelant distrait.
*/
func (r *PostgresRepository) Enrich(
	ctx context.Context, comicID uuid.UUID, summary, language string,
) error {
	params := sqlc.EnrichComicParams{ID: comicID}
	if summary != "" {
		params.Summary = &summary
	}
	if language != "" {
		params.Language = &language
	}

	if _, err := r.q.EnrichComic(ctx, params); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// L'album a disparu entre l'import et l'enrichissement. Rare, mais
			// pas anormal : rien à corriger, rien à signaler.
			return nil
		}
		return fmt.Errorf("discovery : enrichissement : %w", err)
	}
	return nil
}
