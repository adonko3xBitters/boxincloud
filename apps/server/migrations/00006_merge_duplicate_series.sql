-- +goose Up
-- Fusionne les séries dupliquées par divergence de normalisation.
--
-- Le défaut : la migration 00004 a désaccentué `sort_name` en base, mais le
-- code Go qui le produit ne désaccentuait pas. Une réindexation créait donc
-- « astérix » à côté de « asterix », l'index d'unicité ne voyant pas le
-- doublon. Résultat visible : deux séries de même nom, dont une vide.
--
-- Le code est corrigé (indexer.SortName désaccentue désormais comme
-- PostgreSQL). Cette migration nettoie ce que la divergence a produit.

-- 1. Réaffecter les albums vers la série survivante — la plus ancienne de
--    chaque groupe, celle qui a le plus de chances d'être référencée ailleurs.
-- +goose StatementBegin
WITH normalized AS (
    SELECT id, library_id, immutable_unaccent(sort_name) AS canonical,
           row_number() OVER (
               PARTITION BY library_id, immutable_unaccent(sort_name)
               ORDER BY created_at
           ) AS rank
    FROM series
),
survivors AS (
    SELECT library_id, canonical, id AS keep_id FROM normalized WHERE rank = 1
),
duplicates AS (
    SELECT n.id AS drop_id, s.keep_id
    FROM normalized n
    JOIN survivors s ON s.library_id = n.library_id AND s.canonical = n.canonical
    WHERE n.rank > 1
)
UPDATE comics c
SET series_id = d.keep_id
FROM duplicates d
WHERE c.series_id = d.drop_id;
-- +goose StatementEnd

-- 2. Supprimer les séries devenues orphelines.
-- +goose StatementBegin
WITH normalized AS (
    SELECT id, library_id, immutable_unaccent(sort_name) AS canonical,
           row_number() OVER (
               PARTITION BY library_id, immutable_unaccent(sort_name)
               ORDER BY created_at
           ) AS rank
    FROM series
)
DELETE FROM series
WHERE id IN (SELECT id FROM normalized WHERE rank > 1);
-- +goose StatementEnd

-- 3. Normaliser les sort_name restants, pour qu'ils correspondent à ce que le
--    code produit désormais.
UPDATE series SET sort_name = immutable_unaccent(sort_name)
WHERE sort_name <> immutable_unaccent(sort_name);

-- 4. Rafraîchir les compteurs, faussés par la réaffectation.
UPDATE series s
SET comic_count = (
        SELECT count(*) FROM comics c
        WHERE c.series_id = s.id AND c.deleted_at IS NULL
    ),
    cover_comic_id = (
        SELECT c.id FROM comics c
        WHERE c.series_id = s.id AND c.deleted_at IS NULL
        ORDER BY c.number_sort NULLS LAST, c.title
        LIMIT 1
    );

-- +goose Down
-- Une fusion ne se défait pas : les identifiants supprimés sont perdus, et
-- rien ne permet de retrouver quel album appartenait à quel doublon. Cette
-- migration est volontairement irréversible.
SELECT 1;
