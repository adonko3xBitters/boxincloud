-- Outils de gestion : dossiers, favoris, notes, actions en lot.

-- Arborescence des dossiers d'une bibliothèque.
--
-- Les chemins distincts avec leur nombre d'albums ; le client en reconstitue
-- l'arbre. Calculer l'arbre en SQL demanderait une récursion pour un résultat
-- que le client sait bâtir en une passe.
-- name: ListFolders :many
SELECT folder_path, count(*)::int AS comic_count
FROM comics
WHERE library_id = ANY(@library_ids::uuid[]) AND deleted_at IS NULL
GROUP BY folder_path
ORDER BY folder_path;

-- name: SetComicFolder :exec
UPDATE comics SET folder_path = $2 WHERE id = $1;

-- ─── Favoris ─────────────────────────────────────────────────────────────────

-- name: SetFavorite :exec
INSERT INTO favorites (user_id, comic_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnsetFavorite :exec
DELETE FROM favorites WHERE user_id = $1 AND comic_id = $2;

-- name: ListFavoriteIDs :many
SELECT comic_id FROM favorites WHERE user_id = $1;

-- ─── Notes ───────────────────────────────────────────────────────────────────

-- name: SetRating :exec
INSERT INTO comic_ratings (user_id, comic_id, rating)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, comic_id) DO UPDATE SET rating = EXCLUDED.rating;

-- name: ClearRating :exec
DELETE FROM comic_ratings WHERE user_id = $1 AND comic_id = $2;

-- name: ListRatings :many
SELECT comic_id, rating FROM comic_ratings WHERE user_id = $1;

-- ─── Édition manuelle ────────────────────────────────────────────────────────

-- Édition d'un album par l'utilisateur.
--
-- Chaque champ renseigné entre dans locked_fields : une réindexation ne doit
-- jamais écraser une saisie manuelle. C'est la contrepartie de l'automatisme —
-- sans elle, corriger un titre serait inutile.
-- name: EditComic :one
UPDATE comics
SET title       = coalesce(sqlc.narg('title'), title),
    number      = coalesce(sqlc.narg('number'), number),
    summary     = coalesce(sqlc.narg('summary'), summary),
    language    = coalesce(sqlc.narg('language'), language),
    locked_fields = (
        SELECT array_agg(DISTINCT f) FROM unnest(
            locked_fields || @newly_locked::text[]
        ) AS f
    )
WHERE id = @id
RETURNING *;

-- ─── Actions en lot ──────────────────────────────────────────────────────────

-- Marque un lot d'albums comme lus.
--
-- page est fixée à la dernière page de chaque album : marquer lu doit donner le
-- même état que lire jusqu'au bout, sans quoi « reprendre la lecture »
-- proposerait un album déjà déclaré terminé.
-- name: BulkMarkRead :execrows
INSERT INTO reading_progress (user_id, comic_id, page, page_count, status, read_count, started_at, finished_at)
SELECT $1, c.id, greatest(c.page_count - 1, 0), c.page_count, 'read', 1, now(), now()
FROM comics c
WHERE c.id = ANY($2::uuid[])
ON CONFLICT (user_id, comic_id) DO UPDATE
SET page = excluded.page,
    page_count = excluded.page_count,
    status = 'read',
    read_count = reading_progress.read_count
        + CASE WHEN reading_progress.status <> 'read' THEN 1 ELSE 0 END,
    finished_at = coalesce(reading_progress.finished_at, now()),
    version = reading_progress.version + 1,
    updated_at = now();

-- name: BulkMarkUnread :execrows
DELETE FROM reading_progress
WHERE user_id = $1 AND comic_id = ANY($2::uuid[]);

-- name: BulkSetFavorite :execrows
INSERT INTO favorites (user_id, comic_id)
SELECT $1, unnest($2::uuid[])
ON CONFLICT DO NOTHING;

-- name: BulkUnsetFavorite :execrows
DELETE FROM favorites WHERE user_id = $1 AND comic_id = ANY($2::uuid[]);

-- ─── Suppression et déplacement ──────────────────────────────────────────────

-- Retire l'album du catalogue sans effacer sa ligne.
--
-- La progression de lecture, les favoris et les notes y sont rattachés : les
-- détruire priverait d'historique quelqu'un qui remettrait le fichier en place.
-- name: ExcludeComic :exec
UPDATE comics
SET deleted_at  = coalesce(deleted_at, now()),
    excluded_at = now()
WHERE id = $1;

-- Efface définitivement la ligne, une fois le fichier supprimé du backend.
-- name: PurgeComic :exec
DELETE FROM comics WHERE id = $1;

-- name: MoveComic :exec
UPDATE comics
SET object_key  = $2,
    folder_path = $3
WHERE id = $1;

-- name: RestoreComic :exec
UPDATE comics
SET deleted_at = NULL, excluded_at = NULL
WHERE id = $1;

-- name: ListExcludedComics :many
SELECT * FROM comics
WHERE library_id = $1 AND excluded_at IS NOT NULL
ORDER BY title;

-- ─── Dossiers ────────────────────────────────────────────────────────────────

-- name: ListFoldersByLibraries :many
SELECT * FROM folders
WHERE library_id = ANY(@library_ids::uuid[])
ORDER BY library_id, path;

-- name: GetFolder :one
SELECT * FROM folders WHERE library_id = $1 AND path = $2;

-- name: GetFolderByID :one
SELECT * FROM folders WHERE id = $1;

-- name: UpsertFolder :one
INSERT INTO folders (id, library_id, path, name, depth, parent_path, explicit)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (library_id, path) DO UPDATE
-- Un dossier constaté par le parcours ne perd pas son caractère explicite :
-- l'utilisateur l'a voulu, y déposer des fichiers ne défait pas sa décision.
SET explicit = folders.explicit OR EXCLUDED.explicit
RETURNING *;

-- name: DeleteFolderTree :execrows
DELETE FROM folders
WHERE library_id = $1
  AND (path = @path::text OR path LIKE @path::text || '/%');

-- Renomme une branche entière en une passe.
--
-- Les chemins sont des chaînes : déplacer « Tintin » vers « BD/Tintin » revient
-- à réécrire le préfixe de tous les descendants. Un parcours récursif
-- d'identifiants ferait le même travail en n requêtes.
-- name: RenameFolderTree :execrows
UPDATE folders
SET path = @new_path::text || substring(path FROM length(@old_path::text) + 1),
    name = CASE
        WHEN path = @old_path::text THEN @new_name::text
        ELSE name
    END,
    depth = depth + @depth_delta::int,
    parent_path = CASE
        WHEN path = @old_path::text THEN @new_parent::text
        ELSE @new_path::text || substring(parent_path FROM length(@old_path::text) + 1)
    END
WHERE library_id = $1
  AND (path = @old_path::text OR path LIKE @old_path::text || '/%');

-- Supprime les dossiers constatés devenus vides.
--
-- Seuls les non-explicites : un dossier créé à la main survit au fait d'être
-- vide, c'est même souvent la raison de l'avoir créé.
-- name: PruneEmptyFolders :execrows
DELETE FROM folders f
WHERE f.library_id = $1
  AND f.explicit = false
  AND f.path <> ''
  AND NOT EXISTS (
      SELECT 1 FROM comics c
      WHERE c.library_id = f.library_id
        AND c.deleted_at IS NULL
        AND c.excluded_at IS NULL
        AND (c.folder_path = f.path OR c.folder_path LIKE f.path || '/%')
  )
  AND NOT EXISTS (
      SELECT 1 FROM folders child
      WHERE child.library_id = f.library_id
        AND child.explicit = true
        AND child.path LIKE f.path || '/%'
  );

-- Comptes d'albums par dossier exact, sans les descendants.
-- Le cumul se fait ensuite en une passe côté service.
-- name: CountComicsByExactFolder :many
SELECT c.library_id, c.folder_path, count(*)::int AS comic_count
FROM comics c
WHERE c.library_id = ANY(@library_ids::uuid[])
  AND c.deleted_at IS NULL
  AND c.excluded_at IS NULL
GROUP BY c.library_id, c.folder_path;

-- name: ListComicsInFolderTree :many
SELECT id, object_key FROM comics
WHERE library_id = $1
  AND deleted_at IS NULL
  AND (folder_path = @path::text OR folder_path LIKE @path::text || '/%')
ORDER BY object_key;

-- ─── Verrous de dossiers ─────────────────────────────────────────────────────

-- name: SetFolderReadOnly :one
UPDATE folders SET read_only = $3 WHERE library_id = $1 AND path = $2 RETURNING *;

-- name: SetFolderAccessCode :one
UPDATE folders SET access_code_hash = $3 WHERE library_id = $1 AND path = $2 RETURNING *;

-- Dossiers masqués d'une bibliothèque, avec l'échéance du déverrouillage
-- éventuel accordé à ce compte.
--
-- Une seule requête plutôt que deux : la liste des dossiers à code et celle des
-- déverrouillages sont toujours consultées ensemble, et les séparer laisserait
-- une fenêtre où l'une aurait changé sans l'autre.
-- name: ListLockedFolders :many
SELECT f.id, f.library_id, f.path, u.expires_at
FROM folders f
LEFT JOIN folder_unlocks u
       ON u.folder_id = f.id AND u.user_id = @user_id AND u.expires_at > now()
WHERE f.library_id = ANY(@library_ids::uuid[])
  AND f.access_code_hash IS NOT NULL
ORDER BY f.path;

-- name: GetFolderAccessCode :one
SELECT id, access_code_hash FROM folders WHERE library_id = $1 AND path = $2;

-- name: UnlockFolder :exec
INSERT INTO folder_unlocks (user_id, folder_id, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, folder_id) DO UPDATE SET expires_at = EXCLUDED.expires_at;

-- name: LockFolderAgain :exec
DELETE FROM folder_unlocks WHERE user_id = $1 AND folder_id = $2;

-- Retire tous les déverrouillages d'un dossier, quel que soit le compte.
--
-- Appelé quand le code change ou disparaît : un déverrouillage obtenu avec
-- l'ancien code ne doit pas survivre au nouveau.
-- name: RevokeFolderUnlocks :exec
DELETE FROM folder_unlocks WHERE folder_id = $1;

-- Le dossier lui-même ou l'un de ses ancêtres est-il en lecture seule ?
--
-- La protection est héritée : verrouiller « BD » protège tout ce qu'il contient,
-- ce qu'on attend de ce geste. La vérifier ancêtre par ancêtre côté service
-- coûterait une requête par niveau.
-- name: IsFolderTreeReadOnly :one
SELECT EXISTS (
    SELECT 1 FROM folders
    WHERE library_id = $1
      AND read_only = true
      AND (@path::text = path OR @path::text LIKE path || '/%')
) AS locked;
