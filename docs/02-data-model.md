# Modèle de données — v0

PostgreSQL 16+. Identifiants en **UUIDv7** (`uuid`), triés temporellement — pratique pour la pagination par curseur et l'indexation.

Conventions : `snake_case`, `created_at`/`updated_at` en `timestamptz` sur toute table mutable, suppression logique (`deleted_at`) uniquement là où c'est nécessaire.

---

## Vue d'ensemble des relations

```
storage_backends ──< libraries ──< series ──< comics ──< comic_pages
                                      │         │
                       library_access ┘         ├──< reading_progress >── users
                                                ├──< downloads >──────── devices >── users
                                                ├──< comic_tags >─────── tags
                                                ├──< comic_credits >──── people
                                                └──< collection_items >─ collections
```

---

## 1. Identité et accès

```sql
CREATE TYPE user_role AS ENUM ('admin', 'user');

CREATE TABLE users (
    id              uuid PRIMARY KEY,
    username        citext NOT NULL UNIQUE,
    email           citext UNIQUE,
    password_hash   text NOT NULL,              -- argon2id
    role            user_role NOT NULL DEFAULT 'user',
    display_name    text,
    avatar_key      text,                       -- clé dans le backend de cache
    -- Profil restreint : base du "profil enfant"
    restricted      boolean NOT NULL DEFAULT false,
    max_age_rating  smallint,                   -- NULL = aucune limite
    preferences     jsonb NOT NULL DEFAULT '{}',-- thème, mode de lecture par défaut, langue
    last_login_at   timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz
);

CREATE TYPE device_platform AS ENUM ('web', 'android', 'ios', 'desktop', 'unknown');

CREATE TABLE devices (
    id            uuid PRIMARY KEY,
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          text NOT NULL,                -- "Pixel 9 de Nïando"
    platform      device_platform NOT NULL DEFAULT 'unknown',
    app_version   text,
    push_token    text,                         -- notifications, post-V1
    last_seen_at  timestamptz NOT NULL DEFAULT now(),
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON devices (user_id);

-- Refresh tokens : rotation + détection de réutilisation
CREATE TABLE sessions (
    id            uuid PRIMARY KEY,
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id     uuid REFERENCES devices(id) ON DELETE SET NULL,
    token_hash    bytea NOT NULL UNIQUE,        -- SHA-256 du refresh token
    parent_id     uuid REFERENCES sessions(id) ON DELETE SET NULL,
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz,
    user_agent    text,
    ip            inet,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON sessions (user_id) WHERE revoked_at IS NULL;
CREATE INDEX ON sessions (expires_at);

-- Clés d'API (OPDS, scripts, intégrations) — post-V1 mais la table coûte peu
CREATE TABLE api_keys (
    id           uuid PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         text NOT NULL,
    key_hash     bytea NOT NULL UNIQUE,
    last_used_at timestamptz,
    expires_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
```

---

## 2. Stockage et bibliothèques

```sql
CREATE TYPE storage_kind AS ENUM ('s3', 'local', 'webdav');
CREATE TYPE storage_status AS ENUM ('ok', 'degraded', 'error', 'unchecked');

CREATE TABLE storage_backends (
    id            uuid PRIMARY KEY,
    name          text NOT NULL UNIQUE,         -- "MinIO maison", "Backblaze archives"
    kind          storage_kind NOT NULL,
    -- Configuration NON sensible, lisible par l'API : endpoint, bucket, région, style d'URL
    config        jsonb NOT NULL DEFAULT '{}',
    -- Identifiants chiffrés AES-GCM avec BOXINCLOUD_SECRET_KEY. Jamais exposés.
    secrets_enc   bytea,
    is_default    boolean NOT NULL DEFAULT false,
    read_only     boolean NOT NULL DEFAULT false,
    status        storage_status NOT NULL DEFAULT 'unchecked',
    status_detail text,
    checked_at    timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
-- Un seul backend par défaut
CREATE UNIQUE INDEX ON storage_backends (is_default) WHERE is_default;

CREATE TYPE library_kind AS ENUM ('comic', 'manga', 'book', 'mixed');

CREATE TABLE libraries (
    id                 uuid PRIMARY KEY,
    storage_backend_id uuid NOT NULL REFERENCES storage_backends(id) ON DELETE RESTRICT,
    name               text NOT NULL,
    kind               library_kind NOT NULL DEFAULT 'comic',
    root_prefix        text NOT NULL DEFAULT '',  -- préfixe/bucket path, ex. "collections/bd/"
    -- Options de scan : extensions, exclusions, périodicité, politique d'hydratation
    scan_options       jsonb NOT NULL DEFAULT '{}',
    scan_cron          text,                      -- NULL = manuel uniquement
    last_scan_at       timestamptz,
    last_scan_status   text,
    comic_count        integer NOT NULL DEFAULT 0,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX ON libraries (storage_backend_id, root_prefix);

-- Visibilité : une bibliothèque sans ligne d'accès est visible de tous.
-- Dès qu'une ligne existe, elle devient restreinte à cette liste (+ admins).
CREATE TABLE library_access (
    library_id uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    can_write  boolean NOT NULL DEFAULT false,   -- éditer les métadonnées, déclencher un scan
    PRIMARY KEY (library_id, user_id)
);
```

---

## 3. Catalogue

```sql
CREATE TABLE series (
    id            uuid PRIMARY KEY,
    library_id    uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    name          text NOT NULL,
    sort_name     text NOT NULL,                -- "Astérix" → "asterix" ; gère les articles
    description   text,
    publisher     text,
    status        text,                          -- ongoing | ended | hiatus
    age_rating    smallint,
    year_started  smallint,
    cover_comic_id uuid,                         -- FK ajoutée après comics (cycle)
    comic_count   integer NOT NULL DEFAULT 0,
    metadata      jsonb NOT NULL DEFAULT '{}',
    locked_fields text[] NOT NULL DEFAULT '{}',  -- champs édités à la main, jamais écrasés
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX ON series (library_id, sort_name);

CREATE TYPE comic_format AS ENUM ('cbz', 'cbr', 'cb7', 'pdf', 'epub');
CREATE TYPE comic_state  AS ENUM ('pending', 'indexing', 'ready', 'hydrating', 'error');

CREATE TABLE comics (
    id              uuid PRIMARY KEY,
    library_id      uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    series_id       uuid REFERENCES series(id) ON DELETE SET NULL,

    -- Localisation physique
    object_key      text NOT NULL,              -- clé complète dans le backend
    file_size       bigint NOT NULL,
    file_etag       text,                       -- détection de modification à bas coût
    content_hash    bytea,                      -- SHA-256, calculé en différé (déduplication)
    format          comic_format NOT NULL,

    -- Identité éditoriale
    title           text NOT NULL,
    number          text,                       -- texte : gère "3", "3.5", "HS1", "Tome 2"
    number_sort     numeric(10,3),              -- dérivé pour le tri
    volume          smallint,
    summary         text,
    released_at     date,
    age_rating      smallint,
    language        text,                       -- BCP-47

    -- Pagination
    page_count      integer NOT NULL DEFAULT 0,
    cover_page      integer NOT NULL DEFAULT 0,

    -- Cycle de vie
    state           comic_state NOT NULL DEFAULT 'pending',
    state_detail    text,
    hydrated_at     timestamptz,                -- pages extraites dans le cache (CBR/PDF)
    indexed_at      timestamptz,

    metadata        jsonb NOT NULL DEFAULT '{}', -- ComicInfo.xml brut + champs non modélisés
    locked_fields   text[] NOT NULL DEFAULT '{}',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz                  -- objet disparu du backend, données conservées
);

CREATE UNIQUE INDEX ON comics (library_id, object_key);
CREATE INDEX ON comics (series_id, number_sort NULLS LAST, title);
CREATE INDEX ON comics (library_id, created_at DESC);
CREATE INDEX ON comics (content_hash) WHERE content_hash IS NOT NULL;

ALTER TABLE series ADD CONSTRAINT series_cover_fk
    FOREIGN KEY (cover_comic_id) REFERENCES comics(id) ON DELETE SET NULL;

-- Recherche plein texte
ALTER TABLE comics ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(summary, '')), 'C')
    ) STORED;
CREATE INDEX ON comics USING gin (search_vector);
-- Recherche approximative sur les titres (trigrammes)
CREATE INDEX ON comics USING gin (title gin_trgm_ops);
```

### Index des pages — le cœur de l'accès aléatoire

```sql
CREATE TABLE comic_pages (
    comic_id     uuid NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
    index        integer NOT NULL,              -- 0-based
    entry_name   text NOT NULL,                 -- nom de l'entrée dans l'archive
    -- Coordonnées d'accès aléatoire ZIP : évite toute relecture du Central Directory
    data_offset  bigint,                        -- offset des données compressées
    data_size    bigint,                        -- taille compressée
    method       smallint,                      -- 0 = stored, 8 = deflate
    -- Dimensions : permettent au client de réserver la mise en page (zéro décalage visuel)
    width        integer,
    height       integer,
    is_double    boolean NOT NULL DEFAULT false,-- double planche détectée (ratio > 1.2)
    PRIMARY KEY (comic_id, index)
);
```

> Cette table est la raison pour laquelle une page se sert en une seule requête `Range`. Elle est reconstructible : un rescan la régénère.

### Classification

```sql
CREATE TYPE tag_kind AS ENUM ('genre', 'tag', 'publisher', 'imprint');

CREATE TABLE tags (
    id    uuid PRIMARY KEY,
    kind  tag_kind NOT NULL DEFAULT 'tag',
    name  citext NOT NULL,
    UNIQUE (kind, name)
);

CREATE TABLE comic_tags (
    comic_id uuid NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
    tag_id   uuid NOT NULL REFERENCES tags(id)   ON DELETE CASCADE,
    PRIMARY KEY (comic_id, tag_id)
);
CREATE INDEX ON comic_tags (tag_id);

CREATE TABLE people (
    id        uuid PRIMARY KEY,
    name      citext NOT NULL UNIQUE,
    sort_name text NOT NULL
);

CREATE TYPE credit_role AS ENUM
    ('writer','penciller','inker','colorist','letterer','cover_artist','editor','translator','other');

CREATE TABLE comic_credits (
    comic_id  uuid NOT NULL REFERENCES comics(id)  ON DELETE CASCADE,
    person_id uuid NOT NULL REFERENCES people(id)  ON DELETE CASCADE,
    role      credit_role NOT NULL,
    PRIMARY KEY (comic_id, person_id, role)
);
CREATE INDEX ON comic_credits (person_id);
```

### Collections (listes constituées par l'utilisateur)

```sql
CREATE TABLE collections (
    id         uuid PRIMARY KEY,
    owner_id   uuid REFERENCES users(id) ON DELETE CASCADE,  -- NULL = collection serveur
    name       text NOT NULL,
    description text,
    is_public  boolean NOT NULL DEFAULT false,               -- visible des autres comptes
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collection_items (
    collection_id uuid NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    comic_id      uuid NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
    position      integer NOT NULL,
    added_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (collection_id, comic_id)
);
CREATE INDEX ON collection_items (collection_id, position);
```

---

## 4. Lecture et synchronisation

```sql
CREATE TYPE read_status AS ENUM ('unread', 'in_progress', 'read');

CREATE TABLE reading_progress (
    user_id      uuid NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    comic_id     uuid NOT NULL REFERENCES comics(id)  ON DELETE CASCADE,
    page         integer NOT NULL DEFAULT 0,
    page_count   integer NOT NULL DEFAULT 0,          -- copie : survit à un rescan
    status       read_status NOT NULL DEFAULT 'unread',
    read_count   integer NOT NULL DEFAULT 0,          -- nombre de lectures complètes
    -- Synchronisation
    version      bigint NOT NULL DEFAULT 1,           -- incrémenté à chaque écriture
    device_id    uuid REFERENCES devices(id) ON DELETE SET NULL,  -- dernier écrivain
    started_at   timestamptz,
    finished_at  timestamptz,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, comic_id)
);
-- Curseur de synchronisation delta : GET /sync?since=...
CREATE INDEX ON reading_progress (user_id, updated_at DESC);

CREATE TABLE favorites (
    user_id    uuid NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    comic_id   uuid NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, comic_id)
);

CREATE TYPE download_state AS ENUM ('queued', 'downloading', 'ready', 'failed', 'evicted');

-- Vue serveur des téléchargements hors ligne. La vérité opérationnelle est locale
-- à l'appareil ; cette table sert à l'affichage multi-appareils et à la reprise.
CREATE TABLE downloads (
    id          uuid PRIMARY KEY,
    user_id     uuid NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    device_id   uuid NOT NULL REFERENCES devices(id)  ON DELETE CASCADE,
    comic_id    uuid NOT NULL REFERENCES comics(id)   ON DELETE CASCADE,
    state       download_state NOT NULL DEFAULT 'queued',
    bytes_total bigint,
    bytes_done  bigint NOT NULL DEFAULT 0,
    error       text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (device_id, comic_id)
);
CREATE INDEX ON downloads (user_id, updated_at DESC);
```

---

## 5. Exploitation

```sql
-- Suivi des scans, pour l'UI d'administration
CREATE TABLE scan_runs (
    id            uuid PRIMARY KEY,
    library_id    uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    started_at    timestamptz NOT NULL DEFAULT now(),
    finished_at   timestamptz,
    status        text NOT NULL DEFAULT 'running',  -- running | success | failed | cancelled
    objects_seen  integer NOT NULL DEFAULT 0,
    added         integer NOT NULL DEFAULT 0,
    updated       integer NOT NULL DEFAULT 0,
    removed       integer NOT NULL DEFAULT 0,
    errors        integer NOT NULL DEFAULT 0,
    cursor        text,                             -- reprise après interruption
    detail        jsonb NOT NULL DEFAULT '{}'
);

-- Inventaire du cache dérivé, pour l'éviction et la purge
CREATE TABLE cache_entries (
    key          text PRIMARY KEY,                  -- "page/{comic}/{n}/w1600.avif"
    comic_id     uuid REFERENCES comics(id) ON DELETE CASCADE,
    size         bigint NOT NULL,
    last_hit_at  timestamptz NOT NULL DEFAULT now(),
    hits         integer NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON cache_entries (last_hit_at);        -- éviction LRU

-- Paramètres d'instance (nom, page d'accueil, onboarding effectué…)
CREATE TABLE settings (
    key        text PRIMARY KEY,
    value      jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
```

River crée ses propres tables (`river_job`, `river_leader`…) via ses migrations.

---

## Notes de conception

**Pourquoi `number` en texte + `number_sort` en numérique ?** Les numéros réels sont sales : `3`, `3.5`, `HS`, `Tome 2`, `Annual 1`. On conserve la chaîne d'origine pour l'affichage et on dérive une valeur triable, `NULL` quand c'est impossible.

**Pourquoi `locked_fields` ?** Sans ce mécanisme, chaque rescan écrase le travail de curation manuelle. C'est le premier reproche fait aux gestionnaires de médiathèque. Un champ édité à la main entre dans `locked_fields` et devient intouchable par l'indexeur.

**Pourquoi conserver les `comics` supprimés (`deleted_at`) ?** Un objet peut disparaître temporairement (backend en erreur, renommage, bucket démonté). Effacer la ligne détruirait la progression de lecture de tous les utilisateurs. On marque, on masque, et une tâche de purge nettoie après un délai configurable.

**Pourquoi `page_count` dupliqué dans `reading_progress` ?** Pour que « page 42 sur 58 » reste cohérent même si le comic est remplacé par une version au découpage différent.

**Pas de table `permissions` en V1.** `user_role` + `library_access` couvre le besoin réel. Un système de permissions générique introduit maintenant serait de la complexité spéculative ; la migration vers un modèle plus fin restera simple si le besoin apparaît.
