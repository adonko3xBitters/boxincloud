-- Progression de lecture et synchronisation.

-- ★ La requête qui porte la règle de résolution de conflit.
--
-- Deux appareils peuvent écrire la même progression hors ligne, puis se
-- synchroniser. La règle retenue : **la page la plus avancée gagne**, sauf
-- remise à zéro explicite (status = 'unread').
--
-- C'est le comportement qu'attend un lecteur : on ne perd jamais sa
-- progression, et lire sur tablette puis reprendre sur téléphone reprend au bon
-- endroit. Un « dernière écriture gagne » ferait régresser la position dès que
-- les horloges des appareils divergent un peu.
-- name: UpsertReadingProgress :one
INSERT INTO reading_progress (
    user_id, comic_id, page, page_count, status, device_id, started_at, updated_at
)
VALUES (
    $1, $2, $3, $4, $5, $6,
    CASE WHEN $3 > 0 THEN now() ELSE NULL END,
    now()
)
ON CONFLICT (user_id, comic_id) DO UPDATE
SET page = CASE
        WHEN EXCLUDED.status = 'unread' THEN EXCLUDED.page
        WHEN EXCLUDED.page > reading_progress.page THEN EXCLUDED.page
        ELSE reading_progress.page
    END,
    page_count = EXCLUDED.page_count,
    status = CASE
        WHEN EXCLUDED.status = 'unread' THEN 'unread'::read_status
        WHEN EXCLUDED.status = 'read' OR reading_progress.status = 'read' THEN 'read'::read_status
        ELSE 'in_progress'::read_status
    END,
    read_count = reading_progress.read_count
        + CASE WHEN EXCLUDED.status = 'read' AND reading_progress.status <> 'read' THEN 1 ELSE 0 END,
    finished_at = CASE
        WHEN EXCLUDED.status = 'read' AND reading_progress.finished_at IS NULL THEN now()
        WHEN EXCLUDED.status = 'unread' THEN NULL
        ELSE reading_progress.finished_at
    END,
    started_at = coalesce(reading_progress.started_at, EXCLUDED.started_at),
    version = reading_progress.version + 1,
    device_id = EXCLUDED.device_id,
    updated_at = now()
RETURNING *;

-- name: GetReadingProgress :one
SELECT * FROM reading_progress WHERE user_id = $1 AND comic_id = $2;

-- name: ListReadingProgressByComics :many
SELECT * FROM reading_progress
WHERE user_id = $1 AND comic_id = ANY($2::uuid[]);

-- Synchronisation delta : tout ce qui a changé depuis le curseur du client.
-- name: ListReadingProgressSince :many
SELECT * FROM reading_progress
WHERE user_id = $1 AND updated_at > $2
ORDER BY updated_at
LIMIT $3;

-- « Reprendre la lecture » : les albums commencés mais non terminés.
-- name: ListInProgress :many
SELECT sqlc.embed(reading_progress), sqlc.embed(comics)
FROM reading_progress
JOIN comics ON comics.id = reading_progress.comic_id
WHERE reading_progress.user_id = $1
  AND reading_progress.status = 'in_progress'
  AND comics.deleted_at IS NULL
ORDER BY reading_progress.updated_at DESC
LIMIT $2;

-- name: DeleteReadingProgress :exec
DELETE FROM reading_progress WHERE user_id = $1 AND comic_id = $2;

-- ─── Favoris ─────────────────────────────────────────────────────────────────

-- name: AddFavorite :exec
INSERT INTO favorites (user_id, comic_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveFavorite :exec
DELETE FROM favorites WHERE user_id = $1 AND comic_id = $2;

-- name: ListFavorites :many
SELECT sqlc.embed(comics)
FROM favorites
JOIN comics ON comics.id = favorites.comic_id
WHERE favorites.user_id = $1 AND comics.deleted_at IS NULL
ORDER BY favorites.created_at DESC
LIMIT $2 OFFSET $3;
