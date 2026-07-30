-- +goose Up

-- Deux verrous indépendants, parce qu'ils répondent à deux besoins distincts.
--
-- `read_only` protège d'une fausse manœuvre : le dossier reste parfaitement
-- visible, mais on ne peut plus le renommer, le déplacer, y déposer un fichier
-- ni en supprimer un. C'est le verrou d'une collection qu'on a fini de ranger.
--
-- `access_code_hash` masque : le dossier et tout son contenu disparaissent des
-- listages tant que le code n'a pas été saisi. C'est le verrou d'un serveur
-- partagé, où tout le monde n'a pas à voir toute la bibliothèque.
--
-- Ils se cumulent librement : un dossier peut être masqué sans être protégé, et
-- réciproquement.
ALTER TABLE folders
    ADD COLUMN read_only        boolean NOT NULL DEFAULT false,
    ADD COLUMN access_code_hash text;

COMMENT ON COLUMN folders.read_only IS
    'Interdit renommage, déplacement, dépôt et suppression. Ne masque rien.';
COMMENT ON COLUMN folders.access_code_hash IS
    'argon2id. Non nul : le dossier est masqué tant qu''il n''est pas déverrouillé.';

-- Le déverrouillage est enregistré par compte, avec une échéance.
--
-- Il n'est pas porté par le jeton d'accès : celui-ci est autoporteur, donc ni
-- révocable ni modifiable une fois émis. Un déverrouillage doit pouvoir être
-- retiré tout de suite — c'est la moitié de l'intérêt d'un code.
--
-- La clé étrangère sur le dossier suffit à nettoyer : supprimer un dossier
-- emporte les déverrouillages qui le visaient.
CREATE TABLE folder_unlocks (
    user_id    uuid NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    folder_id  uuid NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, folder_id)
);

CREATE INDEX folder_unlocks_expiry ON folder_unlocks (expires_at);

-- +goose Down

DROP TABLE IF EXISTS folder_unlocks;
ALTER TABLE folders
    DROP COLUMN IF EXISTS read_only,
    DROP COLUMN IF EXISTS access_code_hash;
