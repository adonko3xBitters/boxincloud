-- +goose Up

-- Suppression d'un album depuis l'interface, sans toucher au fichier.
--
-- `deleted_at` seul ne suffit pas : le scan repose sur lui pour signaler les
-- objets disparus du backend, et il le remet à NULL dès qu'il retrouve l'objet.
-- Un album retiré du catalogue mais dont le fichier reste en place
-- réapparaîtrait donc au parcours suivant, sans que rien n'explique pourquoi.
--
-- `excluded_at` porte l'intention de l'utilisateur, que le scan doit respecter.
-- Les deux colonnes disent des choses différentes : l'une constate une absence,
-- l'autre enregistre une décision.
ALTER TABLE comics ADD COLUMN excluded_at timestamptz;

COMMENT ON COLUMN comics.excluded_at IS
    'Retiré du catalogue à la demande. Le scan ne le fait pas réapparaître.';

-- L'index sert la clause que toute requête de listage porte désormais.
CREATE INDEX comics_not_excluded ON comics (library_id)
    WHERE deleted_at IS NULL AND excluded_at IS NULL;

-- +goose Down

DROP INDEX IF EXISTS comics_not_excluded;
ALTER TABLE comics DROP COLUMN IF EXISTS excluded_at;
