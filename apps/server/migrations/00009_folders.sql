-- +goose Up

-- Les dossiers deviennent des entités.
--
-- Jusqu'ici l'arborescence était DÉDUITE des clés d'objet : elle existait à
-- l'affichage, jamais en base. Cela suffisait pour parcourir, mais interdisait
-- tout ce qu'on veut y attacher — un verrou, un partage, ou simplement un
-- dossier vide créé à l'avance pour y ranger ensuite.
--
-- La déduction ne disparaît pas pour autant : le parcours continue d'observer
-- les clés et réconcilie cette table avec ce qu'il trouve. Les deux sources
-- coexistent parce qu'elles répondent à deux questions différentes — où sont
-- réellement les fichiers, et ce que l'utilisateur a décidé.
CREATE TABLE folders (
    id         uuid PRIMARY KEY,
    library_id uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,

    -- Chemin complet relatif au préfixe de la bibliothèque. Chaîne vide pour la
    -- racine, qui existe comme un dossier ordinaire : cela évite un cas
    -- particulier partout ailleurs.
    path  text NOT NULL,
    name  text NOT NULL,
    depth integer NOT NULL,

    -- Le parent est retrouvé par chemin plutôt que par clé étrangère
    -- auto-référente : renommer une branche entière devient une seule mise à
    -- jour de chaînes, au lieu d'un parcours récursif d'identifiants.
    parent_path text,

    -- Distingue un dossier voulu d'un dossier constaté.
    --
    -- Un dossier créé à la main doit survivre au fait d'être vide — c'est même
    -- souvent la raison de le créer. Un dossier qui n'existait que parce que des
    -- fichiers s'y trouvaient, en revanche, doit disparaître quand ils s'en
    -- vont, sans quoi l'arborescence se remplirait de branches mortes.
    explicit boolean NOT NULL DEFAULT false,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    UNIQUE (library_id, path)
);

CREATE INDEX folders_by_parent ON folders (library_id, parent_path);
CREATE INDEX folders_by_depth  ON folders (library_id, depth);

CREATE TRIGGER folders_updated_at
    BEFORE UPDATE ON folders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Reprise de l'existant.
--
-- Chaque dossier observé dans les clés d'objet est inscrit, ancêtres compris :
-- « Aventure/Corto Maltese » suppose « Aventure », qu'aucun album n'occupe
-- forcément directement.
-- +goose StatementBegin
DO $$
DECLARE
    lib      record;
    observed text;
    segments text[];
    prefix   text;
    i        integer;
BEGIN
    FOR lib IN SELECT id FROM libraries LOOP
        -- La racine existe toujours.
        INSERT INTO folders (id, library_id, path, name, depth, parent_path, explicit)
        VALUES (gen_random_uuid(), lib.id, '', '', 0, NULL, true)
        ON CONFLICT (library_id, path) DO NOTHING;

        FOR observed IN
            SELECT DISTINCT folder_path
            FROM comics
            WHERE library_id = lib.id AND folder_path <> ''
        LOOP
            segments := string_to_array(observed, '/');
            prefix := '';

            FOR i IN 1 .. array_length(segments, 1) LOOP
                prefix := CASE WHEN i = 1 THEN segments[1] ELSE prefix || '/' || segments[i] END;

                INSERT INTO folders (id, library_id, path, name, depth, parent_path, explicit)
                VALUES (
                    gen_random_uuid(),
                    lib.id,
                    prefix,
                    segments[i],
                    i,
                    CASE WHEN i = 1 THEN '' ELSE array_to_string(segments[1:i-1], '/') END,
                    false
                )
                ON CONFLICT (library_id, path) DO NOTHING;
            END LOOP;
        END LOOP;
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down

DROP TABLE IF EXISTS folders;
