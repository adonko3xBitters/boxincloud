-- Destinations : ce que devient un fichier terminé, selon sa catégorie.

-- name: ListEd2kDestinations :many
SELECT * FROM ed2k_destinations ORDER BY category;

-- name: GetEd2kDestination :one
SELECT * FROM ed2k_destinations WHERE category = $1;

-- name: UpsertEd2kDestination :one
INSERT INTO ed2k_destinations (category, label, library_id, folder)
VALUES ($1, $2, $3, $4)
ON CONFLICT (category) DO UPDATE
    SET label      = EXCLUDED.label,
        library_id = EXCLUDED.library_id,
        folder     = EXCLUDED.folder
RETURNING *;

-- name: DeleteEd2kDestination :exec
DELETE FROM ed2k_destinations WHERE category = $1;

-- Publications : le journal du pont, et la garantie de ne rien publier deux fois.

-- name: GetEd2kPublication :one
SELECT * FROM ed2k_publications WHERE hash = $1;

-- Réserve une publication.
--
-- L'insertion échoue si l'empreinte est déjà connue : c'est ce qui rend le pont
-- idempotent sans verrou. Deux tours de scrutation qui verraient le même
-- fichier terminé ne produiront qu'une publication.
-- name: ClaimEd2kPublication :one
INSERT INTO ed2k_publications (hash, name, size, category, status)
VALUES ($1, $2, $3, $4, 'pending')
ON CONFLICT (hash) DO NOTHING
RETURNING *;

-- name: SetEd2kPublicationResult :exec
UPDATE ed2k_publications
SET status     = sqlc.arg(status),
    detail     = sqlc.arg(detail),
    library_id = sqlc.arg(library_id),
    comic_id   = sqlc.arg(comic_id)
WHERE hash = sqlc.arg(hash);

-- name: ListEd2kPublications :many
SELECT * FROM ed2k_publications ORDER BY created_at DESC LIMIT $1;
