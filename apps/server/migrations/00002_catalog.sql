-- +goose Up
-- Schéma du catalogue : backends de stockage, bibliothèques, séries, comics et
-- index des pages.
--
-- Le sous-ensemble de M1 : ce qu'il faut pour scanner un bucket et servir une
-- page. Les utilisateurs, la progression de lecture et les collections arrivent
-- avec M2 et M7.

-- ─── Backends de stockage ────────────────────────────────────────────────────

CREATE TYPE storage_kind   AS ENUM ('s3', 'local', 'webdav');
CREATE TYPE storage_status AS ENUM ('ok', 'degraded', 'error', 'unchecked');

CREATE TABLE storage_backends (
    id            uuid PRIMARY KEY,
    name          text NOT NULL UNIQUE,
    kind          storage_kind NOT NULL,

    -- Configuration non sensible, exposée par l'API : endpoint, bucket,
    -- region, use_ssl, path_style, root…
    config        jsonb NOT NULL DEFAULT '{}',

    -- Identifiants chiffrés en AES-256-GCM avec BOXINCLOUD_SECRET_KEY.
    -- Ne sortent jamais de la base, pas même pour un administrateur.
    secrets_enc   bytea,

    is_default    boolean NOT NULL DEFAULT false,
    read_only     boolean NOT NULL DEFAULT false,

    status        storage_status NOT NULL DEFAULT 'unchecked',
    status_detail text,
    checked_at    timestamptz,

    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- Un seul backend par défaut à la fois.
CREATE UNIQUE INDEX storage_backends_single_default
    ON storage_backends (is_default) WHERE is_default;

CREATE TRIGGER storage_backends_updated_at
    BEFORE UPDATE ON storage_backends
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ─── Bibliothèques ───────────────────────────────────────────────────────────

CREATE TYPE library_kind AS ENUM ('comic', 'manga', 'book', 'mixed');

CREATE TABLE libraries (
    id                 uuid PRIMARY KEY,
    storage_backend_id uuid NOT NULL REFERENCES storage_backends(id) ON DELETE RESTRICT,
    name               text NOT NULL,
    kind               library_kind NOT NULL DEFAULT 'comic',

    -- Préfixe dans le backend. Vide = tout le bucket.
    root_prefix        text NOT NULL DEFAULT '',

    scan_options       jsonb NOT NULL DEFAULT '{}',
    scan_cron          text,

    last_scan_at       timestamptz,
    last_scan_status   text,
    comic_count        integer NOT NULL DEFAULT 0,

    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

-- Deux bibliothèques ne peuvent pas indexer le même emplacement.
CREATE UNIQUE INDEX libraries_unique_location
    ON libraries (storage_backend_id, root_prefix);

CREATE TRIGGER libraries_updated_at
    BEFORE UPDATE ON libraries
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ─── Séries ──────────────────────────────────────────────────────────────────

CREATE TABLE series (
    id             uuid PRIMARY KEY,
    library_id     uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,

    name           text NOT NULL,
    -- Nom normalisé pour le tri et l'unicité : minuscules, accents et articles
    -- retirés. « Les Aventures de Tintin » → « aventures de tintin ».
    sort_name      text NOT NULL,

    description    text,
    publisher      text,
    status         text,
    age_rating     smallint,
    year_started   smallint,

    cover_comic_id uuid,   -- FK ajoutée après comics : dépendance circulaire
    comic_count    integer NOT NULL DEFAULT 0,

    metadata       jsonb NOT NULL DEFAULT '{}',
    -- Champs saisis à la main, que l'indexation ne doit jamais écraser.
    locked_fields  text[] NOT NULL DEFAULT '{}',

    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX series_unique_per_library ON series (library_id, sort_name);

CREATE TRIGGER series_updated_at
    BEFORE UPDATE ON series
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ─── Comics ──────────────────────────────────────────────────────────────────

CREATE TYPE comic_format AS ENUM ('cbz', 'cbr', 'cb7', 'pdf', 'epub');
CREATE TYPE comic_state  AS ENUM ('pending', 'indexing', 'ready', 'hydrating', 'error');

CREATE TABLE comics (
    id            uuid PRIMARY KEY,
    library_id    uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    series_id     uuid REFERENCES series(id) ON DELETE SET NULL,

    -- Localisation physique
    object_key    text NOT NULL,
    file_size     bigint NOT NULL,
    -- Détection de modification à bas coût, sans relire l'objet. Forme opaque :
    -- MD5 chez les uns, somme composite multipart chez les autres.
    file_etag     text,
    -- SHA-256 du contenu, calculé en différé pour la déduplication.
    content_hash  bytea,
    format        comic_format NOT NULL,

    -- Identité éditoriale
    title         text NOT NULL,
    -- Texte : les numéros réels sont sales — « 3 », « 3.5 », « HS », « Tome 2 ».
    number        text,
    -- Valeur dérivée pour le tri, NULL quand le numéro n'est pas numérique.
    number_sort   numeric(10,3),
    volume        smallint,
    summary       text,
    released_at   date,
    age_rating    smallint,
    language      text,

    page_count    integer NOT NULL DEFAULT 0,
    cover_page    integer NOT NULL DEFAULT 0,

    -- Cycle de vie
    state         comic_state NOT NULL DEFAULT 'pending',
    state_detail  text,
    hydrated_at   timestamptz,   -- pages extraites vers le cache (CBR, PDF)
    indexed_at    timestamptz,

    metadata      jsonb NOT NULL DEFAULT '{}',
    locked_fields text[] NOT NULL DEFAULT '{}',

    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    -- Objet disparu du backend. On marque plutôt que de supprimer : un backend
    -- momentanément injoignable ne doit pas détruire la progression de lecture
    -- de tous les utilisateurs.
    deleted_at    timestamptz
);

-- Clé naturelle : un objet du backend correspond à au plus un comic. C'est ce
-- qui rend le job d'ingestion idempotent et le rescan sans effet de bord.
CREATE UNIQUE INDEX comics_unique_object ON comics (library_id, object_key);

CREATE INDEX comics_by_series  ON comics (series_id, number_sort NULLS LAST, title);
CREATE INDEX comics_by_recency ON comics (library_id, created_at DESC);
CREATE INDEX comics_by_state   ON comics (state) WHERE state <> 'ready';
CREATE INDEX comics_by_hash    ON comics (content_hash) WHERE content_hash IS NOT NULL;

ALTER TABLE series
    ADD CONSTRAINT series_cover_comic_fk
    FOREIGN KEY (cover_comic_id) REFERENCES comics(id) ON DELETE SET NULL;

-- Recherche plein texte. Configuration « simple » plutôt qu'une langue
-- particulière : une bibliothèque mélange couramment le français et l'anglais,
-- et un stemmer d'une langue dégrade l'autre.
ALTER TABLE comics ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(number, '')), 'B') ||
        setweight(to_tsvector('simple', coalesce(summary, '')), 'C')
    ) STORED;

CREATE INDEX comics_search      ON comics USING gin (search_vector);
CREATE INDEX comics_title_trgm  ON comics USING gin (title gin_trgm_ops);

CREATE TRIGGER comics_updated_at
    BEFORE UPDATE ON comics
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ─── Index des pages ─────────────────────────────────────────────────────────

-- ★ La table qui rend le stockage objet viable.
--
-- data_offset et data_size sont les coordonnées d'accès aléatoire extraites de
-- l'archive une seule fois. Servir une page ne demande ensuite qu'un unique
-- ReadRange, sans jamais relire l'index de l'archive.
--
-- Intégralement reconstructible : un rescan la régénère.
CREATE TABLE comic_pages (
    comic_id    uuid NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
    index       integer NOT NULL,           -- 0-based, ordre de lecture
    entry_name  text NOT NULL,              -- nom de l'entrée dans l'archive

    data_offset bigint,                     -- offset des données compressées
    data_size   bigint,                     -- taille compressée
    size        bigint,                     -- taille décompressée
    compression smallint,                   -- 0 = stored, 8 = deflate

    -- Dimensions : permettent au client de réserver la mise en page avant
    -- réception de l'image, donc zéro décalage visuel pendant la lecture.
    width       integer,
    height      integer,
    -- Double planche détectée sur le ratio, pour le mode double page.
    is_double   boolean NOT NULL DEFAULT false,

    PRIMARY KEY (comic_id, index)
);

-- ─── Exploitation ────────────────────────────────────────────────────────────

CREATE TABLE scan_runs (
    id           uuid PRIMARY KEY,
    library_id   uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,

    started_at   timestamptz NOT NULL DEFAULT now(),
    finished_at  timestamptz,
    status       text NOT NULL DEFAULT 'running',  -- running|success|failed|cancelled

    objects_seen integer NOT NULL DEFAULT 0,
    added        integer NOT NULL DEFAULT 0,
    updated      integer NOT NULL DEFAULT 0,
    removed      integer NOT NULL DEFAULT 0,
    errors       integer NOT NULL DEFAULT 0,

    -- Curseur de reprise : un scan interrompu ne repart pas de zéro.
    cursor       text,
    detail       jsonb NOT NULL DEFAULT '{}'
);

CREATE INDEX scan_runs_by_library ON scan_runs (library_id, started_at DESC);

-- Inventaire du cache dérivé, pour l'éviction LRU et la purge.
-- Intégralement reconstructible : le vider ne perd aucune donnée utilisateur.
CREATE TABLE cache_entries (
    key         text PRIMARY KEY,
    comic_id    uuid REFERENCES comics(id) ON DELETE CASCADE,
    size        bigint NOT NULL,
    last_hit_at timestamptz NOT NULL DEFAULT now(),
    hits        integer NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX cache_entries_lru      ON cache_entries (last_hit_at);
CREATE INDEX cache_entries_by_comic ON cache_entries (comic_id);

-- +goose Down
DROP TABLE IF EXISTS cache_entries;
DROP TABLE IF EXISTS scan_runs;
DROP TABLE IF EXISTS comic_pages;
ALTER TABLE IF EXISTS series DROP CONSTRAINT IF EXISTS series_cover_comic_fk;
DROP TABLE IF EXISTS comics;
DROP TABLE IF EXISTS series;
DROP TABLE IF EXISTS libraries;
DROP TABLE IF EXISTS storage_backends;
DROP TYPE IF EXISTS comic_state;
DROP TYPE IF EXISTS comic_format;
DROP TYPE IF EXISTS library_kind;
DROP TYPE IF EXISTS storage_status;
DROP TYPE IF EXISTS storage_kind;
