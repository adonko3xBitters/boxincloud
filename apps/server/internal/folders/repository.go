package folders

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

func (r *PostgresRepository) List(ctx context.Context, libraryIDs []uuid.UUID) ([]Folder, error) {
	rows, err := r.q.ListFoldersByLibraries(ctx, libraryIDs)
	if err != nil {
		return nil, err
	}

	out := make([]Folder, 0, len(rows))
	for _, row := range rows {
		out = append(out, folderFromRow(row))
	}
	return out, nil
}

func (r *PostgresRepository) Get(ctx context.Context, libraryID uuid.UUID, path string) (Folder, error) {
	row, err := r.q.GetFolder(ctx, sqlc.GetFolderParams{LibraryID: libraryID, Path: path})
	if errors.Is(err, pgx.ErrNoRows) {
		return Folder{}, ErrNotFound
	}
	if err != nil {
		return Folder{}, err
	}
	return folderFromRow(row), nil
}

func (r *PostgresRepository) Upsert(ctx context.Context, f Folder) (Folder, error) {
	var parent *string
	if f.Path != "" {
		p := parentOf(f.Path)
		parent = &p
	}

	row, err := r.q.UpsertFolder(ctx, sqlc.UpsertFolderParams{
		ID:        f.ID,
		LibraryID: f.LibraryID,
		Path:      f.Path,
		Name:      f.Name,
		// #nosec G115 -- `Depth` est le nombre de segments d'un chemin de
		// dossier, calculé par depthOf() sur une clé d'objet dont la longueur
		// est bornée à l'ingestion. Il se compte en dizaines, jamais en
		// milliards ; l'analyseur ne voit que la conversion, pas l'invariant.
		Depth:      int32(f.Depth),
		ParentPath: parent,
		Explicit:   f.Explicit,
	})
	if err != nil {
		return Folder{}, err
	}
	return folderFromRow(row), nil
}

func (r *PostgresRepository) RenameTree(
	ctx context.Context,
	libraryID uuid.UUID,
	oldPath, newPath, newName, newParent string,
	depthDelta int,
) (int64, error) {
	return r.q.RenameFolderTree(ctx, sqlc.RenameFolderTreeParams{
		LibraryID: libraryID,
		OldPath:   oldPath,
		NewPath:   newPath,
		NewName:   newName,
		NewParent: newParent,
		// #nosec G115 -- écart entre deux profondeurs de dossier, négatif quand
		// l'arbre remonte. Même invariant que `Depth` dans Upsert.
		DepthDelta: int32(depthDelta),
	})
}

func (r *PostgresRepository) DeleteTree(ctx context.Context, libraryID uuid.UUID, path string) (int64, error) {
	return r.q.DeleteFolderTree(ctx, sqlc.DeleteFolderTreeParams{LibraryID: libraryID, Path: path})
}

func (r *PostgresRepository) PruneEmpty(ctx context.Context, libraryID uuid.UUID) (int64, error) {
	return r.q.PruneEmptyFolders(ctx, libraryID)
}

func (r *PostgresRepository) CountsByExactFolder(
	ctx context.Context, libraryIDs []uuid.UUID,
) (map[uuid.UUID]map[string]int, error) {
	rows, err := r.q.CountComicsByExactFolder(ctx, libraryIDs)
	if err != nil {
		return nil, err
	}

	out := make(map[uuid.UUID]map[string]int, len(libraryIDs))
	for _, row := range rows {
		if out[row.LibraryID] == nil {
			out[row.LibraryID] = make(map[string]int)
		}
		out[row.LibraryID][row.FolderPath] = int(row.ComicCount)
	}
	return out, nil
}

func (r *PostgresRepository) ComicsInTree(
	ctx context.Context, libraryID uuid.UUID, path string,
) ([]ComicRef, error) {
	rows, err := r.q.ListComicsInFolderTree(ctx, sqlc.ListComicsInFolderTreeParams{
		LibraryID: libraryID,
		Path:      path,
	})
	if err != nil {
		return nil, err
	}

	out := make([]ComicRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, ComicRef{ID: row.ID, ObjectKey: row.ObjectKey})
	}
	return out, nil
}

func (r *PostgresRepository) MoveComic(ctx context.Context, id uuid.UUID, objectKey, folderPath string) error {
	return r.q.MoveComic(ctx, sqlc.MoveComicParams{
		ID:         id,
		ObjectKey:  objectKey,
		FolderPath: folderPath,
	})
}

func folderFromRow(row sqlc.Folder) Folder {
	return Folder{
		ID:        row.ID,
		LibraryID: row.LibraryID,
		Path:      row.Path,
		Name:      row.Name,
		Depth:     int(row.Depth),
		Explicit:  row.Explicit,
		ReadOnly:  row.ReadOnly,
		HasCode:   row.AccessCodeHash != nil,
	}
}

// ─── Verrous ─────────────────────────────────────────────────────────────────

var _ LockRepository = (*PostgresRepository)(nil)

func (r *PostgresRepository) SetReadOnly(
	ctx context.Context, libraryID uuid.UUID, path string, readOnly bool,
) (Folder, error) {
	row, err := r.q.SetFolderReadOnly(ctx, sqlc.SetFolderReadOnlyParams{
		LibraryID: libraryID,
		Path:      path,
		ReadOnly:  readOnly,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Folder{}, ErrNotFound
	}
	if err != nil {
		return Folder{}, err
	}
	return folderFromRow(row), nil
}

func (r *PostgresRepository) SetAccessCode(
	ctx context.Context, libraryID uuid.UUID, path string, hash *string,
) (Folder, error) {
	row, err := r.q.SetFolderAccessCode(ctx, sqlc.SetFolderAccessCodeParams{
		LibraryID:      libraryID,
		Path:           path,
		AccessCodeHash: hash,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Folder{}, ErrNotFound
	}
	if err != nil {
		return Folder{}, err
	}
	return folderFromRow(row), nil
}

func (r *PostgresRepository) AccessCode(
	ctx context.Context, libraryID uuid.UUID, path string,
) (uuid.UUID, *string, error) {
	row, err := r.q.GetFolderAccessCode(ctx, sqlc.GetFolderAccessCodeParams{
		LibraryID: libraryID,
		Path:      path,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, nil, err
	}
	return row.ID, row.AccessCodeHash, nil
}

func (r *PostgresRepository) LockedFolders(
	ctx context.Context, userID uuid.UUID, libraryIDs []uuid.UUID,
) ([]LockedFolder, error) {
	rows, err := r.q.ListLockedFolders(ctx, sqlc.ListLockedFoldersParams{
		UserID:     userID,
		LibraryIds: libraryIDs,
	})
	if err != nil {
		return nil, err
	}

	out := make([]LockedFolder, 0, len(rows))
	for _, row := range rows {
		folder := LockedFolder{ID: row.ID, LibraryID: row.LibraryID, Path: row.Path}
		if row.ExpiresAt.Valid {
			until := row.ExpiresAt.Time
			folder.UnlockedUntil = &until
		}
		out = append(out, folder)
	}
	return out, nil
}

func (r *PostgresRepository) Unlock(
	ctx context.Context, userID, folderID uuid.UUID, until time.Time,
) error {
	return r.q.UnlockFolder(ctx, sqlc.UnlockFolderParams{
		UserID:    userID,
		FolderID:  folderID,
		ExpiresAt: pgtype.Timestamptz{Time: until, Valid: true},
	})
}

func (r *PostgresRepository) Relock(ctx context.Context, userID, folderID uuid.UUID) error {
	return r.q.LockFolderAgain(ctx, sqlc.LockFolderAgainParams{
		UserID:   userID,
		FolderID: folderID,
	})
}

func (r *PostgresRepository) RevokeUnlocks(ctx context.Context, folderID uuid.UUID) error {
	return r.q.RevokeFolderUnlocks(ctx, folderID)
}

func (r *PostgresRepository) TreeReadOnly(
	ctx context.Context, libraryID uuid.UUID, path string,
) (bool, error) {
	return r.q.IsFolderTreeReadOnly(ctx, sqlc.IsFolderTreeReadOnlyParams{
		LibraryID: libraryID,
		Path:      path,
	})
}

// ─── Partage ─────────────────────────────────────────────────────────────────

var _ ShareRepository = (*PostgresRepository)(nil)

func (r *PostgresRepository) GrantFolder(ctx context.Context, folderID, userID uuid.UUID, canWrite bool) error {
	return r.q.GrantFolderAccess(ctx, sqlc.GrantFolderAccessParams{
		FolderID: folderID,
		UserID:   userID,
		CanWrite: canWrite,
	})
}

func (r *PostgresRepository) RevokeFolder(ctx context.Context, folderID, userID uuid.UUID) error {
	return r.q.RevokeFolderAccess(ctx, sqlc.RevokeFolderAccessParams{
		FolderID: folderID,
		UserID:   userID,
	})
}

func (r *PostgresRepository) FolderGrants(ctx context.Context, folderID uuid.UUID) ([]FolderGrant, error) {
	rows, err := r.q.ListFolderAccess(ctx, folderID)
	if err != nil {
		return nil, err
	}

	out := make([]FolderGrant, 0, len(rows))
	for _, row := range rows {
		grant := FolderGrant{
			FolderID: row.FolderID,
			UserID:   row.UserID,
			Username: row.Username,
			CanWrite: row.CanWrite,
		}
		if row.DisplayName != nil {
			grant.DisplayName = *row.DisplayName
		}
		out = append(out, grant)
	}
	return out, nil
}

func (r *PostgresRepository) RestrictedFolders(
	ctx context.Context, userID uuid.UUID, libraryIDs []uuid.UUID,
) ([]string, error) {
	return r.q.ListRestrictedFolders(ctx, sqlc.ListRestrictedFoldersParams{
		LibraryIds: libraryIDs,
		UserID:     userID,
	})
}

func (r *PostgresRepository) CanWriteFolder(
	ctx context.Context, userID, libraryID uuid.UUID, path string,
) (bool, error) {
	return r.q.CanWriteFolder(ctx, sqlc.CanWriteFolderParams{
		LibraryID: libraryID,
		UserID:    userID,
		Path:      path,
	})
}

func (r *PostgresRepository) CreateShare(
	ctx context.Context, link ShareLink, tokenHash []byte,
) (ShareLink, error) {
	var comicID uuid.NullUUID
	if link.ComicID != nil {
		comicID = uuid.NullUUID{UUID: *link.ComicID, Valid: true}
	}

	row, err := r.q.CreateShareLink(ctx, sqlc.CreateShareLinkParams{
		ID:         link.ID,
		TokenHash:  tokenHash,
		LibraryID:  link.LibraryID,
		FolderPath: link.FolderPath,
		ComicID:    comicID,
		Label:      link.Label,
		CreatedBy:  link.CreatedBy,
		ExpiresAt:  pgtype.Timestamptz{Time: link.ExpiresAt, Valid: true},
	})
	if err != nil {
		return ShareLink{}, err
	}
	return shareFromRow(row), nil
}

func (r *PostgresRepository) ShareByHash(ctx context.Context, tokenHash []byte) (ShareLink, error) {
	row, err := r.q.GetShareLinkByHash(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ShareLink{}, ErrShareNotFound
	}
	if err != nil {
		return ShareLink{}, err
	}
	return shareFromRow(row), nil
}

func (r *PostgresRepository) ListShares(ctx context.Context, libraryIDs []uuid.UUID) ([]ShareLink, error) {
	rows, err := r.q.ListShareLinks(ctx, libraryIDs)
	if err != nil {
		return nil, err
	}

	out := make([]ShareLink, 0, len(rows))
	for _, row := range rows {
		out = append(out, shareFromRow(row))
	}
	return out, nil
}

func (r *PostgresRepository) RevokeShare(ctx context.Context, id uuid.UUID) (int64, error) {
	return r.q.RevokeShareLink(ctx, id)
}

func (r *PostgresRepository) TouchShare(ctx context.Context, id uuid.UUID) error {
	return r.q.TouchShareLink(ctx, id)
}

func (r *PostgresRepository) TreeHasAccessCode(
	ctx context.Context, libraryID uuid.UUID, path string,
) (bool, error) {
	return r.q.TreeHasAccessCode(ctx, sqlc.TreeHasAccessCodeParams{
		LibraryID: libraryID,
		Path:      path,
	})
}

func (r *PostgresRepository) ComicFolder(
	ctx context.Context, comicID uuid.UUID,
) (uuid.UUID, string, error) {
	row, err := r.q.GetComic(ctx, comicID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, "", ErrNotFound
	}
	if err != nil {
		return uuid.Nil, "", err
	}
	return row.LibraryID, row.FolderPath, nil
}

func (r *PostgresRepository) ComicInScope(
	ctx context.Context, comicID, libraryID uuid.UUID, path string,
) (bool, error) {
	return r.q.ComicInSharedFolder(ctx, sqlc.ComicInSharedFolderParams{
		ID:        comicID,
		LibraryID: libraryID,
		Path:      path,
	})
}

func (r *PostgresRepository) ComicsInScope(
	ctx context.Context, libraryID uuid.UUID, path string,
) ([]uuid.UUID, error) {
	rows, err := r.q.ListSharedComics(ctx, sqlc.ListSharedComicsParams{
		LibraryID: libraryID,
		Path:      path,
	})
	if err != nil {
		return nil, err
	}

	out := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Comic.ID)
	}
	return out, nil
}

func shareFromRow(row sqlc.ShareLink) ShareLink {
	link := ShareLink{
		ID:        row.ID,
		LibraryID: row.LibraryID,
		Label:     row.Label,
		CreatedBy: row.CreatedBy,
		UseCount:  row.UseCount,
	}
	link.FolderPath = row.FolderPath
	if row.ComicID.Valid {
		id := row.ComicID.UUID
		link.ComicID = &id
	}
	if row.ExpiresAt.Valid {
		link.ExpiresAt = row.ExpiresAt.Time
	}
	if row.CreatedAt.Valid {
		link.CreatedAt = row.CreatedAt.Time
	}
	if row.LastUsedAt.Valid {
		t := row.LastUsedAt.Time
		link.LastUsedAt = &t
	}
	return link
}
