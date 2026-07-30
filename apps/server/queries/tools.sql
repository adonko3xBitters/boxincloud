-- Outils de gestion : dossiers, favoris, notes, actions en lot.

-- Arborescence des dossiers d'une bibliothèque.
--
-- Les chemins distincts avec leur nombre d'albums ; le client en reconstitue
-- l'arbre. Calculer l'arbre en SQL demanderait une récursion pour un résultat
-- que le client sait bâtir en une passe.
-- name: ListFolders :many
SELECT folder_path, count(*)::int AS comic_count
FROM comics
WHERE library_id = ANY(@library_ids::uuid[]) AND deleted_at IS NULL
GROUP BY folder_path
ORDER BY folder_path;

-- name: SetComicFolder :exec
UPDATE comics SET folder_path = $2 WHERE id = $1;

-- ─── Favoris ─────────────────────────────────────────────────────────────────

-- name: SetFavorite :exec
INSERT INTO favorites (user_id, comic_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnsetFavorite :exec
DELETE FROM favorites WHERE user_id = $1 AND comic_id = $2;

-- name: ListFavoriteIDs :many
SELECT comic_id FROM favorites WHERE user_id = $1;

-- ─── Notes ───────────────────────────────────────────────────────────────────

-- name: SetRating :exec
INSERT INTO comic_ratings (user_id, comic_id, rating)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, comic_id) DO UPDATE SET rating = EXCLUDED.rating;

-- name: ClearRating :exec
DELETE FROM comic_ratings WHERE user_id = $1 AND comic_id = $2;

-- name: ListRatings :many
SELECT comic_id, rating FROM comic_ratings WHERE user_id = $1;

-- ─── Édition manuelle ────────────────────────────────────────────────────────

-- Édition d'un album par l'utilisateur.
--
-- Chaque champ renseigné entre dans locked_fields : une réindexation ne doit
-- jamais écraser une saisie manuelle. C'est la contrepartie de l'automatisme —
-- sans elle, corriger un titre serait inutile.
-- name: EditComic :one
UPDATE comics
SET title       = coalesce(sqlc.narg('title'), title),
    number      = coalesce(sqlc.narg('number'), number),
    summary     = coalesce(sqlc.narg('summary'), summary),
    language    = coalesce(sqlc.narg('language'), language),
    locked_fields = (
        SELECT array_agg(DISTINCT f) FROM unnest(
            locked_fields || @newly_locked::text[]
        ) AS f
    )
WHERE id = @id
RETURNING *;

-- ─── Actions en lot ──────────────────────────────────────────────────────────

-- Marque un lot d'albums comme lus.
--
-- page est fixée à la dernière page de chaque album : marquer lu doit donner le
-- même état que lire jusqu'au bout, sans quoi « reprendre la lecture »
-- proposerait un album déjà déclaré terminé.
-- name: BulkMarkRead :execrows
INSERT INTO reading_progress (user_id, comic_id, page, page_count, status, read_count, started_at, finished_at)
SELECT $1, c.id, greatest(c.page_count - 1, 0), c.page_count, 'read', 1, now(), now()
FROM comics c
WHERE c.id = ANY($2::uuid[])
ON CONFLICT (user_id, comic_id) DO UPDATE
SET page = excluded.page,
    page_count = excluded.page_count,
    status = 'read',
    read_count = reading_progress.read_count
        + CASE WHEN reading_progress.status <> 'read' THEN 1 ELSE 0 END,
    finished_at = coalesce(reading_progress.finished_at, now()),
    version = reading_progress.version + 1,
    updated_at = now();

-- name: BulkMarkUnread :execrows
DELETE FROM reading_progress
WHERE user_id = $1 AND comic_id = ANY($2::uuid[]);

-- name: BulkSetFavorite :execrows
INSERT INTO favorites (user_id, comic_id)
SELECT $1, unnest($2::uuid[])
ON CONFLICT DO NOTHING;

-- name: BulkUnsetFavorite :execrows
DELETE FROM favorites WHERE user_id = $1 AND comic_id = ANY($2::uuid[]);

-- ─── Suppression et déplacement ──────────────────────────────────────────────

-- Retire l'album du catalogue sans effacer sa ligne.
--
-- La progression de lecture, les favoris et les notes y sont rattachés : les
-- détruire priverait d'historique quelqu'un qui remettrait le fichier en place.
-- name: ExcludeComic :exec
UPDATE comics
SET deleted_at  = coalesce(deleted_at, now()),
    excluded_at = now()
WHERE id = $1;

-- Efface définitivement la ligne, une fois le fichier supprimé du backend.
-- name: PurgeComic :exec
DELETE FROM comics WHERE id = $1;

-- name: MoveComic :exec
UPDATE comics
SET object_key  = $2,
    folder_path = $3
WHERE id = $1;

-- name: RestoreComic :exec
UPDATE comics
SET deleted_at = NULL, excluded_at = NULL
WHERE id = $1;

-- name: ListExcludedComics :many
SELECT * FROM comics
WHERE library_id = $1 AND excluded_at IS NOT NULL
ORDER BY title;
