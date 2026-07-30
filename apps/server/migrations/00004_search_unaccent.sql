-- +goose Up
-- Recherche insensible aux accents.
--
-- Sans cela, un lecteur qui tape « asterix » ne trouve pas « Astérix », et
-- « tresor » ne trouve pas « Trésor ». Pour un lecteur de BD franco-belge,
-- c'est rédhibitoire : personne ne saisit les accents dans un champ de
-- recherche.
--
-- Vérifié avant correction, sur des données réelles :
--   websearch_to_tsquery('simple','asterix') → 0 résultat
--   similarity('Astérix le Gaulois', 'asterix') → 0,227 (sous le seuil de 0,3)

CREATE EXTENSION IF NOT EXISTS unaccent;

-- unaccent() est déclarée STABLE et non IMMUTABLE, car son résultat dépend
-- d'un dictionnaire modifiable. PostgreSQL refuse donc de l'utiliser dans une
-- colonne générée ou un index.
--
-- L'enveloppe ci-dessous la déclare IMMUTABLE en désignant explicitement le
-- dictionnaire. C'est la pratique établie, avec une contrepartie à connaître :
-- si le dictionnaire `unaccent` était modifié, les index existants seraient à
-- reconstruire. Il ne l'est jamais en pratique.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION immutable_unaccent(text)
RETURNS text AS $$
    SELECT public.unaccent('public.unaccent'::regdictionary, $1)
$$ LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT;
-- +goose StatementEnd

-- Reconstruction du vecteur de recherche sur du texte désaccentué.
DROP INDEX IF EXISTS comics_search;
ALTER TABLE comics DROP COLUMN IF EXISTS search_vector;

ALTER TABLE comics ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', immutable_unaccent(coalesce(title, ''))), 'A') ||
        setweight(to_tsvector('simple', immutable_unaccent(coalesce(number, ''))), 'B') ||
        setweight(to_tsvector('simple', immutable_unaccent(coalesce(summary, ''))), 'C')
    ) STORED;

CREATE INDEX comics_search ON comics USING gin (search_vector);

-- Index trigramme sur le titre désaccentué, pour la tolérance aux fautes de
-- frappe. L'ancien index sur le titre brut est remplacé : il ne servirait plus,
-- les requêtes portant désormais sur la forme désaccentuée.
DROP INDEX IF EXISTS comics_title_trgm;
CREATE INDEX comics_title_unaccent_trgm
    ON comics USING gin (immutable_unaccent(title) gin_trgm_ops);

-- Même traitement pour les séries : chercher une série est aussi fréquent que
-- chercher un album.
CREATE INDEX series_name_unaccent_trgm
    ON series USING gin (immutable_unaccent(name) gin_trgm_ops);

-- sort_name est déjà normalisé côté application (minuscules, articles retirés),
-- mais pas désaccentué : « Astérix » s'y classait après « Azrael ».
UPDATE series SET sort_name = immutable_unaccent(sort_name);

-- +goose Down
DROP INDEX IF EXISTS series_name_unaccent_trgm;
DROP INDEX IF EXISTS comics_title_unaccent_trgm;
DROP INDEX IF EXISTS comics_search;
ALTER TABLE comics DROP COLUMN IF EXISTS search_vector;

ALTER TABLE comics ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(number, '')), 'B') ||
        setweight(to_tsvector('simple', coalesce(summary, '')), 'C')
    ) STORED;

CREATE INDEX comics_search ON comics USING gin (search_vector);
CREATE INDEX comics_title_trgm ON comics USING gin (title gin_trgm_ops);

DROP FUNCTION IF EXISTS immutable_unaccent(text);
