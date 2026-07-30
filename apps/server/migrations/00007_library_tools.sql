-- +goose Up
-- Outils de gestion de bibliothèque : dossiers, notes, favoris exposés.
--
-- Ce que l'interface dense réclame et que le modèle ne portait pas.

-- ─── Arborescence de dossiers ────────────────────────────────────────────────

-- Le chemin du dossier contenant l'album, relatif au préfixe de la
-- bibliothèque. Dérivé de object_key à l'ingestion.
--
-- Stocké plutôt que calculé à la volée : la colonne est indexée, ce qui rend le
-- filtrage par dossier aussi rapide qu'un filtrage par série. Le calculer dans
-- chaque requête interdirait tout index.
ALTER TABLE comics ADD COLUMN folder_path text NOT NULL DEFAULT '';

CREATE INDEX comics_by_folder ON comics (library_id, folder_path)
    WHERE deleted_at IS NULL;

COMMENT ON COLUMN comics.folder_path IS
    'Dossier contenant l''album, relatif au préfixe de la bibliothèque. Vide à la racine.';

-- Renseigne les albums déjà indexés, sans attendre un rescan.
UPDATE comics c
SET folder_path = (
    SELECT CASE
        -- Retire le préfixe de la bibliothèque, puis le nom de fichier.
        WHEN position('/' in ltrim(substr(c.object_key, length(l.root_prefix) + 1), '/')) = 0
            THEN ''
        ELSE regexp_replace(
            ltrim(substr(c.object_key, length(l.root_prefix) + 1), '/'),
            '/[^/]*$', ''
        )
    END
    FROM libraries l WHERE l.id = c.library_id
);

-- ─── Notes ───────────────────────────────────────────────────────────────────

-- Note personnelle, de 1 à 5. NULL signifie « non noté », ce qui est distinct
-- de « noté zéro » — un utilisateur doit pouvoir retirer sa note.
--
-- Par utilisateur, et non par album : deux personnes d'un même foyer n'ont
-- aucune raison de partager leur avis.
CREATE TABLE comic_ratings (
    user_id    uuid NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    comic_id   uuid NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
    rating     smallint NOT NULL CHECK (rating BETWEEN 1 AND 5),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, comic_id)
);

CREATE INDEX comic_ratings_by_comic ON comic_ratings (comic_id);

CREATE TRIGGER comic_ratings_updated_at
    BEFORE UPDATE ON comic_ratings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS comic_ratings;
DROP INDEX IF EXISTS comics_by_folder;
ALTER TABLE comics DROP COLUMN IF EXISTS folder_path;
