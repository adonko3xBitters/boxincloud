-- name: CreateDiscoverySource :one
INSERT INTO discovery_sources (id, name, url, kind, enabled, username, secret_enc, template)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
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
    -- Les règles d'extraction ne se vident pas : une source `web` sans elles
    -- serait interrogée sans qu'on sache quoi lire, et la contrainte de la base
    -- refuserait la ligne. Ne pas les envoyer conserve celles en place.
    template = COALESCE(sqlc.narg(template)::jsonb, template),
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

-- ─── Imports ─────────────────────────────────────────────────────────────────

-- name: CreateDiscoveryImport :one
INSERT INTO discovery_imports (
    id, source_id, source_name, href, library_id, folder, title, requested_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetDiscoveryImport :one
SELECT * FROM discovery_imports WHERE id = $1;

-- name: ListDiscoveryImports :many
SELECT * FROM discovery_imports
ORDER BY created_at DESC
LIMIT $1;

-- name: StartDiscoveryImport :exec
UPDATE discovery_imports
SET status = 'running', started_at = now()
WHERE id = $1;

-- name: FinishDiscoveryImport :exec
UPDATE discovery_imports
SET status = 'done',
    comic_id = $2,
    object_key = $3,
    file_size = $4,
    error_code = '',
    error_detail = '',
    finished_at = now()
WHERE id = $1;

-- name: FailDiscoveryImport :exec
UPDATE discovery_imports
SET status = 'failed',
    error_code = $2,
    error_detail = $3,
    finished_at = now()
WHERE id = $1;

-- ─── Enrichissement ──────────────────────────────────────────────────────────

-- Complète un album avec une fiche de métadonnées.
--
-- Trois garde-fous inscrits dans la requête plutôt que laissés au code.
--
-- `nullif(champ, '')` : seuls les champs VIDES sont remplis. Une fiche
-- généraliste ne doit pas écraser ce que l'archive elle-même déclarait — le
-- ComicInfo.xml d'un éditeur en sait plus sur son album qu'Open Library.
--
-- `NOT (... = ANY(locked_fields))` : une saisie manuelle est intouchable. C'est
-- la contrepartie de tout automatisme dans ce projet, et l'enrichissement n'y
-- fait pas exception — corriger un titre à la main serait inutile s'il pouvait
-- être défait par une requête vers un service tiers.
--
-- `locked_fields` n'est PAS modifié : l'enrichissement est automatique, et
-- verrouiller ce qu'il pose empêcherait une réindexation de le corriger avec
-- une meilleure source.
-- name: EnrichComic :one
UPDATE comics
SET summary = CASE
        WHEN nullif(summary, '') IS NULL AND NOT ('summary' = ANY(locked_fields))
        THEN coalesce(sqlc.narg('summary'), summary)
        ELSE summary END,
    language = CASE
        WHEN nullif(language, '') IS NULL AND NOT ('language' = ANY(locked_fields))
        THEN coalesce(sqlc.narg('language'), language)
        ELSE language END
WHERE id = @id
RETURNING *;
