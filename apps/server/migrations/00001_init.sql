-- +goose Up
-- Migration initiale : extensions et fonctions partagées.
--
-- Le schéma métier (users, libraries, comics…) arrive avec M1 et M2. Cette
-- migration ne pose que les fondations dont toutes les suivantes dépendent.

-- citext : comparaisons insensibles à la casse pour les noms d'utilisateur,
--          adresses e-mail, tags et noms de personnes.
CREATE EXTENSION IF NOT EXISTS citext;

-- pg_trgm : recherche approximative sur les titres (index GIN trigrammes).
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Met à jour updated_at automatiquement. Déclencheur posé sur chaque table
-- mutable par les migrations suivantes.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- Paramètres d'instance : nom du serveur, état de l'assistant de première
-- installation, préférences globales.
CREATE TABLE settings (
    key        text PRIMARY KEY,
    value      jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER settings_updated_at
    BEFORE UPDATE ON settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS settings;
DROP FUNCTION IF EXISTS set_updated_at();
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS citext;
