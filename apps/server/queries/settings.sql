-- Paramètres d'instance.
--
-- Première requête du projet : elle valide la chaîne sqlc de bout en bout.
-- Le schéma métier arrive avec M1.

-- name: GetSetting :one
SELECT key, value, updated_at
FROM settings
WHERE key = $1;

-- name: ListSettings :many
SELECT key, value, updated_at
FROM settings
ORDER BY key;

-- name: UpsertSetting :one
INSERT INTO settings (key, value)
VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE
    SET value = EXCLUDED.value
RETURNING key, value, updated_at;

-- name: DeleteSetting :exec
DELETE FROM settings
WHERE key = $1;
