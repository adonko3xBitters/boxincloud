-- +goose Up

-- ─── Partage entre comptes ───────────────────────────────────────────────────

-- Accès à un dossier, accordé compte par compte.
--
-- Le modèle reprend exactement celui des bibliothèques, et c'est délibéré : un
-- dossier SANS aucune autorisation explicite est visible de tous ceux qui voient
-- la bibliothèque ; le premier accès accordé le referme pour tous les autres.
--
-- Deux règles différentes pour deux niveaux de la même arborescence auraient été
-- impossibles à retenir. Celle-ci se dit en une phrase, et l'interface la répète
-- au moment où elle s'applique.
CREATE TABLE folder_access (
    folder_id uuid NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
    user_id   uuid NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    can_write boolean NOT NULL DEFAULT false,
    PRIMARY KEY (folder_id, user_id)
);

CREATE INDEX folder_access_by_user ON folder_access (user_id);

-- ─── Liens publics ───────────────────────────────────────────────────────────

-- Un lien public ouvre un accès SANS compte : qui a l'URL voit le contenu.
--
-- C'est le seul mécanisme de boxincloud qui sorte du périmètre authentifié, d'où
-- trois garde-fous inscrits dans le schéma lui-même plutôt que laissés au code :
--
--   * l'échéance est obligatoire — un lien sans fin finit toujours par circuler
--     plus loin que prévu, et personne ne se souvient de le fermer ;
--   * la portée est exactement un dossier OU un album, jamais une bibliothèque
--     entière ni « tout » ;
--   * seul le HACHAGE du jeton est stocké, comme un mot de passe : une fuite de
--     la base ne doit pas livrer les liens en clair.
CREATE TABLE share_links (
    id         uuid PRIMARY KEY,
    token_hash bytea NOT NULL UNIQUE,
    library_id uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,

    -- Exactement l'un des deux, garanti par la contrainte plus bas.
    folder_path text,
    comic_id    uuid REFERENCES comics(id) ON DELETE CASCADE,

    label      text NOT NULL DEFAULT '',
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    last_used_at timestamptz,
    use_count  bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT share_links_one_target CHECK ((folder_path IS NULL) <> (comic_id IS NULL))
);

CREATE INDEX share_links_by_creator ON share_links (created_by);
CREATE INDEX share_links_expiry     ON share_links (expires_at);

COMMENT ON COLUMN share_links.token_hash IS
    'SHA-256 du jeton. Le jeton en clair n''est montré qu''une fois, à la création.';

-- +goose Down

DROP TABLE IF EXISTS share_links;
DROP TABLE IF EXISTS folder_access;
