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
		ID:         f.ID,
		LibraryID:  f.LibraryID,
		Path:       f.Path,
		Name:       f.Name,
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
		LibraryID:  libraryID,
		OldPath:    oldPath,
		NewPath:    newPath,
		NewName:    newName,
		NewParent:  newParent,
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
