-- Ingestion idempotente : la clé naturelle (library_id, object_key) permet de
-- rejouer un scan sans créer de doublon ni perdre les champs verrouillés.
-- name: UpsertComic :one
INSERT INTO comics (
    id, library_id, object_key, file_size, file_etag, format, title, state
)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending')
ON CONFLICT (library_id, object_key) DO UPDATE
SET file_size  = EXCLUDED.file_size,
    file_etag  = EXCLUDED.file_etag,
    -- Un album retiré du catalogue à la demande le reste : le scan constate
    -- une présence, il n'annule pas une décision.
    deleted_at = CASE WHEN comics.excluded_at IS NOT NULL THEN comics.deleted_at ELSE NULL END,
    -- Un objet modifié doit être réindexé ; un objet inchangé garde son état.
    state = CASE
        WHEN comics.file_etag IS DISTINCT FROM EXCLUDED.file_etag
          OR comics.file_size <> EXCLUDED.file_size
        THEN 'pending'::comic_state
        ELSE comics.state
    END
RETURNING *, (xmax = 0) AS inserted;

-- name: GetComic :one
SELECT * FROM comics WHERE id = $1;

-- name: GetComicByObjectKey :one
SELECT * FROM comics WHERE library_id = $1 AND object_key = $2;

-- name: ListComicsByLibrary :many
SELECT * FROM comics
WHERE library_id = $1 AND deleted_at IS NULL
ORDER BY title
LIMIT $2 OFFSET $3;

-- name: CountComicsByLibrary :one
SELECT count(*) FROM comics WHERE library_id = $1 AND deleted_at IS NULL;

-- name: SetComicState :exec
UPDATE comics
SET state = $2, state_detail = $3
WHERE id = $1;

-- name: SetComicIndexed :exec
UPDATE comics
SET state = 'ready',
    state_detail = NULL,
    page_count = $2,
    indexed_at = now()
WHERE id = $1;

-- Applique les métadonnées issues de ComicInfo.xml ou du nom de fichier.
-- Les champs présents dans locked_fields sont préservés : une saisie manuelle
-- ne doit jamais être écrasée par un rescan.
-- name: ApplyComicMetadata :exec
UPDATE comics
SET title       = CASE WHEN 'title'       = ANY(locked_fields) THEN title       ELSE coalesce(sqlc.narg('title'), title) END,
    number      = CASE WHEN 'number'      = ANY(locked_fields) THEN number      ELSE coalesce(sqlc.narg('number'), number) END,
    number_sort = CASE WHEN 'number'      = ANY(locked_fields) THEN number_sort ELSE coalesce(sqlc.narg('number_sort'), number_sort) END,
    volume      = CASE WHEN 'volume'      = ANY(locked_fields) THEN volume      ELSE coalesce(sqlc.narg('volume'), volume) END,
    summary     = CASE WHEN 'summary'     = ANY(locked_fields) THEN summary     ELSE coalesce(sqlc.narg('summary'), summary) END,
    released_at = CASE WHEN 'released_at' = ANY(locked_fields) THEN released_at ELSE coalesce(sqlc.narg('released_at'), released_at) END,
    age_rating  = CASE WHEN 'age_rating'  = ANY(locked_fields) THEN age_rating  ELSE coalesce(sqlc.narg('age_rating'), age_rating) END,
    language    = CASE WHEN 'language'    = ANY(locked_fields) THEN language    ELSE coalesce(sqlc.narg('language'), language) END,
    series_id   = coalesce(sqlc.narg('series_id'), series_id),
    metadata    = $2
WHERE id = $1;

-- Marque comme supprimés les objets absents du dernier scan.
-- On ne supprime pas la ligne : un backend momentanément injoignable ne doit
-- pas détruire la progression de lecture des utilisateurs.
-- name: MarkMissingComicsDeleted :execrows
UPDATE comics
SET deleted_at = now()
WHERE library_id = $1
  AND deleted_at IS NULL
  AND object_key <> ALL($2::text[]);

-- name: DeleteComic :exec
DELETE FROM comics WHERE id = $1;

-- ─── Pages ───────────────────────────────────────────────────────────────────

-- name: DeleteComicPages :exec
DELETE FROM comic_pages WHERE comic_id = $1;

-- name: InsertComicPage :exec
INSERT INTO comic_pages (
    comic_id, index, entry_name, data_offset, data_size, size, compression, width, height, is_double
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListComicPages :many
SELECT * FROM comic_pages
WHERE comic_id = $1
ORDER BY index;

-- ★ La requête du chemin chaud : servir une page.
-- Un seul aller-retour en base, puis un seul ReadRange sur le backend.
-- name: GetComicPage :one
SELECT * FROM comic_pages
WHERE comic_id = $1 AND index = $2;

-- name: CountComicPages :one
SELECT count(*) FROM comic_pages WHERE comic_id = $1;

-- ─── Séries ──────────────────────────────────────────────────────────────────

-- name: UpsertSeries :one
INSERT INTO series (id, library_id, name, sort_name)
VALUES ($1, $2, $3, $4)
ON CONFLICT (library_id, sort_name) DO UPDATE
SET name = EXCLUDED.name
RETURNING *;

-- name: GetSeries :one
SELECT * FROM series WHERE id = $1;

-- name: ListSeriesByLibrary :many
SELECT * FROM series
WHERE library_id = $1
ORDER BY sort_name;

-- name: RefreshSeriesCounts :exec
UPDATE series
SET comic_count = (
        -- Les albums retirés du catalogue à la demande ne comptent pas plus que
        -- ceux disparus du stockage : les deux sont invisibles de l'utilisateur.
        SELECT count(*) FROM comics
        WHERE comics.series_id = series.id
          AND comics.deleted_at IS NULL
          AND comics.excluded_at IS NULL
    ),
    cover_comic_id = coalesce(
        series.cover_comic_id,
        (SELECT comics.id FROM comics
         WHERE comics.series_id = series.id AND comics.deleted_at IS NULL
         ORDER BY comics.number_sort NULLS LAST, comics.title
         LIMIT 1)
    )
WHERE series.library_id = $1;

-- ─── Cache dérivé ────────────────────────────────────────────────────────────

-- name: RecordCacheEntry :exec
INSERT INTO cache_entries (key, comic_id, size)
VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE
SET size = EXCLUDED.size, last_hit_at = now();

-- name: TouchCacheEntry :execrows
UPDATE cache_entries
SET last_hit_at = now(), hits = hits + 1
WHERE key = $1;

-- name: TotalCacheSize :one
SELECT coalesce(sum(size), 0)::bigint FROM cache_entries;

-- name: ListCacheEntriesForEviction :many
SELECT key, size FROM cache_entries
ORDER BY last_hit_at
LIMIT $1;

-- name: DeleteCacheEntry :exec
DELETE FROM cache_entries WHERE key = $1;

-- name: SetComicPlaceholder :exec
UPDATE comics SET cover_placeholder = $2 WHERE id = $1;
