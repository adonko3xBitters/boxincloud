-- +goose Up

-- ─── Catalogues fédérés ──────────────────────────────────────────────────────

-- Catalogues extérieurs que cette instance sait interroger.
--
-- Le protocole est OPDS, et cette table n'a pas vocation à en accueillir un
-- second qui changerait la nature de la chose. Fédérer de l'OPDS, c'est
-- interroger des catalogues auxquels l'utilisateur a déjà accès : soit publics
-- — Standard Ebooks, Gutenberg, une bibliothèque municipale — soit ouverts par
-- des identifiants qu'il fournit lui-même, comme son propre Komga.
--
-- La colonne `kind` n'a donc qu'une valeur aujourd'hui. Elle existe pour que la
-- lecture du schéma dise de quoi il s'agit, et parce qu'un protocole de
-- catalogue de plus coûte moins cher ici qu'en migration plus tard.
CREATE TABLE discovery_sources (
    id       uuid PRIMARY KEY,
    name     text NOT NULL UNIQUE,

    -- Adresse du flux racine. C'est le serveur qui la joindra, d'où le
    -- contrôle appliqué à la saisie (voir internal/platform/netguard).
    url      text NOT NULL,

    kind     text NOT NULL DEFAULT 'opds',
    enabled  boolean NOT NULL DEFAULT true,

    -- Vide quand le catalogue est public, ce qui est le cas le plus fréquent.
    username text NOT NULL DEFAULT '',

    -- Mot de passe chiffré en AES-256-GCM avec BOXINCLOUD_SECRET_KEY, comme les
    -- identifiants de backend. Ne ressort jamais par l'API.
    secret_enc bytea,

    -- Dernier essai de connexion. Sert à montrer un catalogue en panne dans
    -- l'administration sans qu'il faille ouvrir les journaux — un catalogue
    -- tiers qui change d'adresse est le mode de panne le plus courant d'une
    -- fédération, et il est silencieux : la recherche rend simplement moins.
    last_error      text NOT NULL DEFAULT '',
    last_checked_at timestamptz,

    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX discovery_sources_enabled ON discovery_sources (enabled) WHERE enabled;

-- +goose Down

DROP TABLE discovery_sources;
