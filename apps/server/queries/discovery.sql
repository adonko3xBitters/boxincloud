-- name: CreateDiscoverySource :one
INSERT INTO discovery_sources (id, name, url, kind, enabled, username, secret_enc)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetDiscoverySource :one
SELECT * FROM discovery_sources WHERE id = $1;

-- name: ListDiscoverySources :many
SELECT * FROM discovery_sources ORDER BY name;

-- Le secret est lu seul, par une requête distincte.
--
-- Ce n'est pas une contrainte technique : `GetDiscoverySource` le rapporte déjà.
-- C'est une contrainte de lecture. Un développeur qui écrit un gestionnaire voit
-- qu'il faut une deuxième requête pour obtenir le mot de passe, ce qui rend
-- difficile de le laisser filer dans une réponse par distraction.
-- name: GetDiscoverySourceSecret :one
SELECT secret_enc FROM discovery_sources WHERE id = $1;

-- name: UpdateDiscoverySource :one
UPDATE discovery_sources
SET name     = $2,
    url      = $3,
    enabled  = $4,
    username = $5,
    -- Le mot de passe n'est remplacé que si l'appelant le demande : un
    -- formulaire qui renvoie un champ vide ne doit pas effacer ce qui marche.
    secret_enc = CASE WHEN sqlc.arg(replace_secret)::boolean
                      THEN sqlc.narg(secret_enc)::bytea
                      ELSE secret_enc END
WHERE id = $1
RETURNING *;

-- name: DeleteDiscoverySource :exec
DELETE FROM discovery_sources WHERE id = $1;

-- name: RecordDiscoveryProbe :exec
UPDATE discovery_sources
SET last_error = $2,
    last_checked_at = now()
WHERE id = $1;
