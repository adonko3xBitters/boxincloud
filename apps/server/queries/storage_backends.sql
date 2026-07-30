-- name: CreateStorageBackend :one
INSERT INTO storage_backends (id, name, kind, config, secrets_enc, is_default, read_only)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetStorageBackend :one
SELECT * FROM storage_backends WHERE id = $1;

-- name: GetStorageBackendByName :one
SELECT * FROM storage_backends WHERE name = $1;

-- name: GetDefaultStorageBackend :one
SELECT * FROM storage_backends WHERE is_default LIMIT 1;

-- name: ListStorageBackends :many
SELECT * FROM storage_backends ORDER BY is_default DESC, name;

-- name: SetStorageBackendStatus :exec
UPDATE storage_backends
SET status = $2, status_detail = $3, checked_at = now()
WHERE id = $1;

-- Un seul backend par défaut : on retire le drapeau aux autres avant de le
-- poser, l'index unique partiel refuserait sinon la mise à jour.
-- name: ClearDefaultStorageBackend :exec
UPDATE storage_backends SET is_default = false WHERE is_default AND id <> $1;

-- name: SetDefaultStorageBackend :exec
UPDATE storage_backends SET is_default = true WHERE id = $1;

-- name: DeleteStorageBackend :exec
DELETE FROM storage_backends WHERE id = $1;
