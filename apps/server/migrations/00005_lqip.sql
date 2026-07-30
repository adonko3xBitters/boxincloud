-- +goose Up
-- Placeholder de chargement des couvertures (LQIP).
--
-- Une miniature de quelques dizaines d'octets, encodée en base64 dans la
-- réponse JSON, affichée floutée pendant que la vraie couverture arrive.
--
-- Le squelette animé actuel réserve déjà la place — il n'y a donc pas de
-- décalage — mais une grille de squelettes gris reste terne. Un aperçu coloré
-- donne l'impression que la page est déjà là, ce qui est exactement l'effet
-- recherché sur l'écran le plus regardé de l'application.
--
-- Pourquoi en base et pas dans le cache : la valeur tient en ~200 octets et
-- doit arriver AVEC la liste d'albums, en une seule requête. La chercher
-- ailleurs annulerait tout le bénéfice.

ALTER TABLE comics ADD COLUMN cover_placeholder text;

COMMENT ON COLUMN comics.cover_placeholder IS
    'Data-URI d''une miniature JPEG de ~16px, affichée floutée pendant le chargement.';

-- +goose Down
ALTER TABLE comics DROP COLUMN IF EXISTS cover_placeholder;
