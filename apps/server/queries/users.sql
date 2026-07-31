-- name: CreateUser :one
INSERT INTO users (id, username, email, password_hash, role, display_name)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1 AND deleted_at IS NULL;

-- name: ListUsers :many
SELECT * FROM users WHERE deleted_at IS NULL ORDER BY username;

-- Sert à l'assistant de première installation : tant qu'il n'y a personne,
-- l'inscription du premier administrateur est ouverte.
-- name: CountUsers :one
SELECT count(*) FROM users WHERE deleted_at IS NULL;

-- name: SetUserPassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: TouchUserLogin :exec
UPDATE users SET last_login_at = now() WHERE id = $1;

-- name: UpdateUserProfile :one
UPDATE users
SET display_name = coalesce(sqlc.narg('display_name'), display_name),
    email        = coalesce(sqlc.narg('email'), email),
    preferences  = coalesce(sqlc.narg('preferences'), preferences)
WHERE id = $1
RETURNING *;

-- name: SetUserRole :exec
UPDATE users SET role = $2 WHERE id = $1;

-- name: SoftDeleteUser :exec
UPDATE users SET deleted_at = now() WHERE id = $1;

-- ─── Appareils ───────────────────────────────────────────────────────────────

-- name: UpsertDevice :one
INSERT INTO devices (id, user_id, name, platform, app_version)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO UPDATE
SET name         = EXCLUDED.name,
    app_version  = EXCLUDED.app_version,
    last_seen_at = now()
RETURNING *;

-- name: GetDevice :one
SELECT * FROM devices WHERE id = $1;

-- name: ListDevicesByUser :many
SELECT * FROM devices WHERE user_id = $1 ORDER BY last_seen_at DESC;

-- name: TouchDevice :exec
UPDATE devices SET last_seen_at = now() WHERE id = $1;

-- name: DeleteDevice :exec
DELETE FROM devices WHERE id = $1 AND user_id = $2;

-- Cet appareil existe-t-il encore pour ce compte ?
--
-- Interrogée à chaque requête portant un jeton d'appareil, derrière un cache de
-- quelques secondes. Sans elle, révoquer un téléphone perdu ne l'empêcherait de
-- rien pendant la durée de vie du jeton d'accès — un quart d'heure de lecture
-- offert à qui l'a ramassé.
-- name: DeviceExists :one
SELECT EXISTS (
    SELECT 1 FROM devices WHERE id = $1 AND user_id = $2
);

-- name: RevokeDeviceSessions :execrows
UPDATE sessions SET revoked_at = now()
WHERE user_id = $1 AND device_id = $2 AND revoked_at IS NULL;

-- ─── Sessions ────────────────────────────────────────────────────────────────

-- name: CreateSession :one
INSERT INTO sessions (id, user_id, device_id, token_hash, parent_id, expires_at, user_agent, ip)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions WHERE token_hash = $1;

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;

-- Détection de réutilisation : si un jeton déjà tourné est présenté, c'est
-- qu'il a été volé. On révoque alors toute la chaîne de rotation, pas
-- seulement le jeton présenté.
-- name: RevokeSessionChain :execrows
WITH RECURSIVE chain AS (
    SELECT s.id FROM sessions s WHERE s.id = $1
    UNION
    SELECT s.id FROM sessions s JOIN chain c ON s.parent_id = c.id
)
UPDATE sessions
SET revoked_at = now()
WHERE sessions.id IN (SELECT chain.id FROM chain)
  AND sessions.revoked_at IS NULL;

-- name: RevokeAllUserSessions :execrows
UPDATE sessions SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at < now() - interval '30 days';

-- ─── Accès aux bibliothèques ─────────────────────────────────────────────────

-- Bibliothèques visibles par un utilisateur.
--
-- Règle : une bibliothèque sans aucune restriction est visible de tous ; dès
-- qu'une restriction existe, seuls les utilisateurs listés y ont accès. Les
-- administrateurs voient tout.
-- name: ListVisibleLibraries :many
SELECT l.* FROM libraries l
WHERE $2::boolean = true
   OR NOT EXISTS (SELECT 1 FROM library_access a WHERE a.library_id = l.id)
   OR EXISTS (SELECT 1 FROM library_access a WHERE a.library_id = l.id AND a.user_id = $1)
ORDER BY l.name;

-- name: CanAccessLibrary :one
SELECT
    $2::boolean
    OR NOT EXISTS (SELECT 1 FROM library_access a WHERE a.library_id = $3)
    OR EXISTS (SELECT 1 FROM library_access a WHERE a.library_id = $3 AND a.user_id = $1)
    AS allowed;

-- name: GrantLibraryAccess :exec
INSERT INTO library_access (library_id, user_id, can_write)
VALUES ($1, $2, $3)
ON CONFLICT (library_id, user_id) DO UPDATE SET can_write = EXCLUDED.can_write;

-- name: RevokeLibraryAccess :exec
DELETE FROM library_access WHERE library_id = $1 AND user_id = $2;

-- name: ListLibraryAccess :many
SELECT * FROM library_access WHERE library_id = $1;

-- ─── Administration des comptes ──────────────────────────────────────────────

-- name: SetUserRestriction :one
UPDATE users
SET restricted     = $2,
    max_age_rating = $3
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SetUserRoleReturning :one
UPDATE users SET role = $2 WHERE id = $1 AND deleted_at IS NULL RETURNING *;

-- Compte les administrateurs encore actifs.
--
-- Sert à empêcher la suppression ou la rétrogradation du dernier d'entre eux :
-- une instance sans administrateur ne peut plus être administrée du tout, et
-- rien dans l'interface ne permettrait d'en refaire un.
-- name: CountAdmins :one
SELECT count(*) FROM users WHERE role = 'admin' AND deleted_at IS NULL;

-- name: ListAccessByUser :many
SELECT * FROM library_access WHERE user_id = $1;
