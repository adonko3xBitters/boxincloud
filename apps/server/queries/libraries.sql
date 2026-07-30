-- name: CreateLibrary :one
INSERT INTO libraries (id, storage_backend_id, name, kind, root_prefix, scan_options, scan_cron)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetLibrary :one
SELECT * FROM libraries WHERE id = $1;

-- name: GetLibraryByName :one
SELECT * FROM libraries WHERE name = $1;

-- name: ListLibraries :many
SELECT * FROM libraries ORDER BY name;

-- name: ListLibrariesWithBackend :many
SELECT
    sqlc.embed(libraries),
    sqlc.embed(storage_backends)
FROM libraries
JOIN storage_backends ON storage_backends.id = libraries.storage_backend_id
ORDER BY libraries.name;

-- name: SetLibraryScanResult :exec
UPDATE libraries
SET last_scan_at = now(),
    last_scan_status = $2,
    comic_count = (
        SELECT count(*) FROM comics
        WHERE comics.library_id = $1 AND comics.deleted_at IS NULL
    )
WHERE libraries.id = $1;

-- name: DeleteLibrary :exec
DELETE FROM libraries WHERE id = $1;

-- ─── Scans ───────────────────────────────────────────────────────────────────

-- name: StartScanRun :one
INSERT INTO scan_runs (id, library_id)
VALUES ($1, $2)
RETURNING *;

-- name: FinishScanRun :exec
UPDATE scan_runs
SET finished_at = now(),
    status = $2,
    objects_seen = $3,
    added = $4,
    updated = $5,
    removed = $6,
    errors = $7,
    detail = $8
WHERE id = $1;

-- name: GetScanRun :one
SELECT * FROM scan_runs WHERE id = $1;

-- name: ListScanRuns :many
SELECT * FROM scan_runs
WHERE library_id = $1
ORDER BY started_at DESC
LIMIT $2;
