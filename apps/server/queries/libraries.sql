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
        WHERE comics.library_id = $1
          AND comics.deleted_at IS NULL
          AND comics.excluded_at IS NULL
    )
WHERE libraries.id = $1;

-- Recompte les albums d'une bibliothèque.
--
-- Le compteur est une colonne stockée, et il ne l'était rafraîchi qu'en fin de
-- parcours. Supprimer un album le laissait donc figé : la barre latérale
-- annonçait vingt-et-un albums devant une grille vide, jusqu'au prochain scan.
--
-- Les deux dates comptent. `deleted_at` marque un objet disparu du backend,
-- `excluded_at` un album retiré du catalogue par l'utilisateur. Le premier
-- filtre était seul, si bien qu'un retrait sans suppression de fichier ne
-- décrémentait rien — même après un scan.
-- name: RefreshLibraryCount :exec
UPDATE libraries
SET comic_count = (
        SELECT count(*) FROM comics
        WHERE comics.library_id = $1
          AND comics.deleted_at IS NULL
          AND comics.excluded_at IS NULL
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

-- ─── Administration ──────────────────────────────────────────────────────────

-- name: UpdateLibrary :one
UPDATE libraries
SET name        = coalesce(sqlc.narg('name'), name),
    root_prefix = coalesce(sqlc.narg('root_prefix'), root_prefix)
WHERE id = $1
RETURNING *;

-- Combien de bibliothèques s'appuient sur ce backend ?
--
-- Sert à refuser sa suppression tant qu'il en porte : effacer un backend
-- emporterait ses bibliothèques par cascade, et avec elles la progression de
-- lecture de tout le monde.
-- name: CountLibrariesUsingBackend :one
SELECT count(*) FROM libraries WHERE storage_backend_id = $1;
