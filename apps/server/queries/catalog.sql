-- Requêtes de consultation du catalogue.
--
-- Pagination par curseur plutôt que par OFFSET : sur une bibliothèque de
-- plusieurs milliers de titres, OFFSET force PostgreSQL à parcourir puis jeter
-- toutes les lignes précédentes, et une insertion pendant la pagination décale
-- silencieusement les résultats. Le curseur est stable et à coût constant.

-- name: ListComicsPage :many
SELECT * FROM comics
WHERE library_id = ANY(@library_ids::uuid[])
  AND deleted_at IS NULL
  AND (sqlc.narg('series_id')::uuid IS NULL OR series_id = sqlc.narg('series_id')::uuid)
  AND (@state::text = '' OR state::text = @state)
  -- Filtrage par classification d'âge, pour les profils restreints.
  AND (sqlc.narg('max_age_rating')::smallint IS NULL
       OR age_rating IS NULL
       OR age_rating <= sqlc.narg('max_age_rating')::smallint)
  -- Curseur : (created_at, id) est un ordre total, donc sans ex æquo ambigus.
  AND (sqlc.narg('cursor_created_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_created_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @page_size;

-- name: ListComicsBySeries :many
SELECT * FROM comics
WHERE series_id = $1 AND deleted_at IS NULL
ORDER BY number_sort NULLS LAST, title;

-- Recherche plein texte, avec repli sur la similarité trigramme.
--
-- Les deux sont combinés parce qu'ils échouent différemment :
-- websearch_to_tsquery gère les expressions et les mots multiples mais rate
-- les fautes de frappe ; la similarité trigramme rattrape « asterics » →
-- « Astérix » mais ne comprend pas les requêtes à plusieurs mots.
--
-- word_similarity plutôt que similarity : cette dernière compare la requête au
-- titre ENTIER, si bien qu'un titre long fait chuter le score sous le seuil.
-- Mesuré sur « asterics » contre « Astérix le Gaulois » : similarity = 0,27
-- (rejeté), word_similarity = 0,67 (accepté). word_similarity cherche la
-- meilleure sous-séquence de mots, ce qui est le comportement attendu quand on
-- tape un seul mot d'un titre.
--
-- Tout est désaccentué de part et d'autre : personne ne saisit les accents
-- dans un champ de recherche, et c'est rédhibitoire sur de la BD franco-belge.
-- name: SearchComics :many
SELECT *, (
    ts_rank(search_vector, websearch_to_tsquery('simple', immutable_unaccent(@query)))
    + word_similarity(immutable_unaccent(@query), immutable_unaccent(title))
)::real AS rank
FROM comics
WHERE library_id = ANY(@library_ids::uuid[])
  AND deleted_at IS NULL
  AND (sqlc.narg('max_age_rating')::smallint IS NULL
       OR age_rating IS NULL
       OR age_rating <= sqlc.narg('max_age_rating')::smallint)
  AND (search_vector @@ websearch_to_tsquery('simple', immutable_unaccent(@query))
       OR immutable_unaccent(@query) <% immutable_unaccent(title))
ORDER BY rank DESC, title
LIMIT @page_size;

-- name: GetComicDetail :one
SELECT sqlc.embed(comics), sqlc.embed(series)
FROM comics
LEFT JOIN series ON series.id = comics.series_id
WHERE comics.id = $1 AND comics.deleted_at IS NULL;

-- name: ListSeriesPage :many
SELECT * FROM series
WHERE library_id = ANY(@library_ids::uuid[])
  AND (@cursor_sort_name::text = '' OR sort_name > @cursor_sort_name)
ORDER BY sort_name
LIMIT @page_size;

-- name: SearchSeries :many
SELECT *, word_similarity(immutable_unaccent(@query), immutable_unaccent(name))::real AS rank
FROM series
WHERE library_id = ANY(@library_ids::uuid[])
  AND (immutable_unaccent(@query) <% immutable_unaccent(name) OR sort_name LIKE @prefix::text)
ORDER BY rank DESC, sort_name
LIMIT @page_size;

-- Étagère d'accueil : les derniers albums ajoutés.
-- name: ListRecentComics :many
SELECT * FROM comics
WHERE library_id = ANY(@library_ids::uuid[])
  AND deleted_at IS NULL
  AND state = 'ready'
  AND (sqlc.narg('max_age_rating')::smallint IS NULL
       OR age_rating IS NULL
       OR age_rating <= sqlc.narg('max_age_rating')::smallint)
ORDER BY created_at DESC
LIMIT @page_size;

-- Étagère « Suite de la série » : le premier album non lu de chaque série déjà
-- entamée. C'est la suggestion la plus utile d'une page d'accueil de lecteur.
-- name: ListNextInSeries :many
SELECT DISTINCT ON (c.series_id) c.*
FROM comics c
JOIN series s ON s.id = c.series_id
WHERE c.library_id = ANY(@library_ids::uuid[])
  AND c.deleted_at IS NULL
  AND c.state = 'ready'
  -- Album non commencé…
  AND NOT EXISTS (
      SELECT 1 FROM reading_progress p
      WHERE p.comic_id = c.id AND p.user_id = @user_id AND p.status <> 'unread'
  )
  -- …dans une série dont au moins un album a été lu.
  AND EXISTS (
      SELECT 1 FROM reading_progress p
      JOIN comics c2 ON c2.id = p.comic_id
      WHERE c2.series_id = c.series_id AND p.user_id = @user_id AND p.status = 'read'
  )
ORDER BY c.series_id, c.number_sort NULLS LAST, c.title
LIMIT @page_size;
