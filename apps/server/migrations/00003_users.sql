-- +goose Up
-- Utilisateurs, sessions, appareils et progression de lecture.
--
-- Multi-utilisateur dès la V1 : c'est le scénario self-hosted dominant — une
-- famille, des progressions indépendantes, éventuellement des bibliothèques
-- filtrées pour les enfants.

CREATE TYPE user_role AS ENUM ('admin', 'user');

CREATE TABLE users (
    id             uuid PRIMARY KEY,
    username       citext NOT NULL UNIQUE,
    email          citext UNIQUE,
    password_hash  text NOT NULL,              -- argon2id
    role           user_role NOT NULL DEFAULT 'user',
    display_name   text,
    avatar_key     text,                       -- clé dans le backend de cache

    -- Profil restreint : base du « profil enfant ». Le filtrage effectif
    -- arrive en M7, mais la colonne existe dès maintenant pour éviter une
    -- migration de données ultérieure.
    restricted     boolean NOT NULL DEFAULT false,
    max_age_rating smallint,                   -- NULL = aucune limite

    preferences    jsonb NOT NULL DEFAULT '{}',-- thème, mode de lecture, langue
    last_login_at  timestamptz,

    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz
);

CREATE TRIGGER users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ─── Appareils ───────────────────────────────────────────────────────────────

CREATE TYPE device_platform AS ENUM ('web', 'android', 'ios', 'desktop', 'unknown');

CREATE TABLE devices (
    id           uuid PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         text NOT NULL,
    platform     device_platform NOT NULL DEFAULT 'unknown',
    app_version  text,
    push_token   text,                          -- notifications, post-V1
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX devices_by_user ON devices (user_id);

-- ─── Sessions ────────────────────────────────────────────────────────────────

-- Refresh tokens, avec rotation et détection de réutilisation.
--
-- Le jeton n'est jamais stocké en clair : seul son SHA-256 l'est. Une fuite de
-- la base ne donne donc pas de session utilisable.
--
-- parent_id chaîne les rotations : si un jeton déjà tourné est présenté, c'est
-- qu'il a été volé — on révoque alors toute la chaîne.
CREATE TABLE sessions (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id  uuid REFERENCES devices(id) ON DELETE SET NULL,
    token_hash bytea NOT NULL UNIQUE,
    parent_id  uuid REFERENCES sessions(id) ON DELETE SET NULL,

    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    user_agent text,
    ip         inet,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_active  ON sessions (user_id) WHERE revoked_at IS NULL;
CREATE INDEX sessions_expiry  ON sessions (expires_at);

CREATE TABLE api_keys (
    id           uuid PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         text NOT NULL,
    key_hash     bytea NOT NULL UNIQUE,
    last_used_at timestamptz,
    expires_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX api_keys_by_user ON api_keys (user_id);

-- ─── Accès aux bibliothèques ─────────────────────────────────────────────────

-- Une bibliothèque sans aucune ligne ici est visible de tous. Dès qu'une ligne
-- existe, elle devient restreinte à cette liste, plus les administrateurs.
--
-- Ce choix évite d'avoir à créer des autorisations pour chaque utilisateur dès
-- l'installation : par défaut tout est partagé, on restreint au besoin.
CREATE TABLE library_access (
    library_id uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    can_write  boolean NOT NULL DEFAULT false,
    PRIMARY KEY (library_id, user_id)
);

CREATE INDEX library_access_by_user ON library_access (user_id);

-- ─── Progression de lecture ──────────────────────────────────────────────────

CREATE TYPE read_status AS ENUM ('unread', 'in_progress', 'read');

CREATE TABLE reading_progress (
    user_id     uuid NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    comic_id    uuid NOT NULL REFERENCES comics(id) ON DELETE CASCADE,

    page        integer NOT NULL DEFAULT 0,
    -- Copie du nombre de pages : « page 42 sur 58 » doit rester cohérent même
    -- si l'album est remplacé par une version au découpage différent.
    page_count  integer NOT NULL DEFAULT 0,
    status      read_status NOT NULL DEFAULT 'unread',
    read_count  integer NOT NULL DEFAULT 0,

    -- Incrémenté à chaque écriture : sert de garde à la synchronisation.
    version     bigint NOT NULL DEFAULT 1,
    device_id   uuid REFERENCES devices(id) ON DELETE SET NULL,

    started_at  timestamptz,
    finished_at timestamptz,
    updated_at  timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, comic_id)
);

-- Curseur de la synchronisation delta : GET /sync?since=…
CREATE INDEX reading_progress_sync ON reading_progress (user_id, updated_at DESC);

CREATE TABLE favorites (
    user_id    uuid NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    comic_id   uuid NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, comic_id)
);

CREATE INDEX favorites_by_user ON favorites (user_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS favorites;
DROP TABLE IF EXISTS reading_progress;
DROP TYPE  IF EXISTS read_status;
DROP TABLE IF EXISTS library_access;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS devices;
DROP TYPE  IF EXISTS device_platform;
DROP TABLE IF EXISTS users;
DROP TYPE  IF EXISTS user_role;
