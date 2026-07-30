-- Requêtes de consultation du catalogue.
--
-- Pagination par curseur plutôt que par OFFSET : sur une bibliothèque de
-- plusieurs milliers de titres, OFFSET force PostgreSQL à parcourir puis jeter
-- toutes les lignes précédentes, et une insertion pendant la pagination décale
-- silencieusement les résultats. Le curseur est stable et à coût constant.

-- Liste paginée, filtrable et triable.
--
-- Le tri est porté par la requête plutôt que par une concaténation côté Go :
-- une clause ORDER BY construite en chaîne serait une porte d'injection, et le
-- plan d'exécution ne serait plus mis en cache. Les trois ordres possibles sont
-- donc écrits en dur, sélectionnés par un paramètre.
--
-- Le curseur suit l'ordre choisi : (created_at, id) pour le tri par ajout,
-- (title, id) pour le tri alphabétique. Un seul curseur composite ne
-- conviendrait pas aux deux.
-- name: ListComicsPage :many
SELECT c.* FROM comics c
LEFT JOIN reading_progress p
       ON p.comic_id = c.id AND p.user_id = @user_id
WHERE c.library_id = ANY(@library_ids::uuid[])
  AND c.deleted_at IS NULL
  AND (sqlc.narg('series_id')::uuid IS NULL OR c.series_id = sqlc.narg('series_id')::uuid)
  AND (@state::text = '' OR c.state::text = @state)
  -- Filtrage par dossier. Le préfixe permet d'inclure les sous-dossiers, ce
  -- qu'attend un utilisateur qui clique sur un nœud de l'arbre.
  AND (sqlc.narg('folder')::text IS NULL
       OR c.folder_path = sqlc.narg('folder')::text
       OR c.folder_path LIKE sqlc.narg('folder')::text || '/%')
  AND (@favorites_only::boolean = false
       OR EXISTS (SELECT 1 FROM favorites f
                  WHERE f.comic_id = c.id AND f.user_id = @user_id))
  -- Filtrage par classification d'âge, pour les profils restreints.
  AND (sqlc.narg('max_age_rating')::smallint IS NULL
       OR c.age_rating IS NULL
       OR c.age_rating <= sqlc.narg('max_age_rating')::smallint)
  -- Filtrage par statut de lecture. « unread » couvre l'absence de ligne :
  -- un album jamais ouvert n'a pas d'entrée dans reading_progress.
  AND (@read_status::text = ''
       OR (@read_status = 'unread'      AND (p.status IS NULL OR p.status = 'unread'))
       OR (@read_status = 'in_progress' AND p.status = 'in_progress')
       OR (@read_status = 'read'        AND p.status = 'read'))
  -- Curseur, dans l'ordre du tri demandé.
  AND (
    CASE @sort::text
      WHEN 'title' THEN
        sqlc.narg('cursor_title')::text IS NULL
        OR (c.title, c.id) > (sqlc.narg('cursor_title')::text, sqlc.narg('cursor_id')::uuid)
      WHEN 'released' THEN
        sqlc.narg('cursor_released')::date IS NULL
        OR (c.released_at, c.id) < (sqlc.narg('cursor_released')::date, sqlc.narg('cursor_id')::uuid)
      ELSE
        sqlc.narg('cursor_created_at')::timestamptz IS NULL
        OR (c.created_at, c.id) < (sqlc.narg('cursor_created_at')::timestamptz, sqlc.narg('cursor_id')::uuid)
    END
  )
ORDER BY
  CASE WHEN @sort = 'title'    THEN c.title END ASC,
  CASE WHEN @sort = 'released' THEN c.released_at END DESC NULLS LAST,
  CASE WHEN @sort NOT IN ('title', 'released') THEN c.created_at END DESC,
  c.id DESC
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
