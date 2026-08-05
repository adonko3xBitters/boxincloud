-- Module eD2k/Kad — connexion au démon aMule.
--
-- La ligne est unique par construction (CHECK (id) dans la migration), ce qui
-- permet à toutes ces requêtes de se passer de clé : il n'y a rien à désigner.

-- name: GetEd2kDaemon :one
SELECT * FROM ed2k_daemon WHERE id;

-- name: UpsertEd2kDaemon :one
INSERT INTO ed2k_daemon (id, host, port, password_enc, label)
VALUES (true, $1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE
    SET host         = EXCLUDED.host,
        port         = EXCLUDED.port,
        password_enc = EXCLUDED.password_enc,
        label        = EXCLUDED.label
RETURNING *;

-- Écrit le dernier état constaté sans toucher aux identifiants.
--
-- Séparée de l'upsert délibérément : la boucle de scrutation met cet état à
-- jour, et elle n'a aucune raison de pouvoir réécrire un mot de passe.
-- name: SetEd2kDaemonState :exec
UPDATE ed2k_daemon
SET last_state   = sqlc.arg(state),
    last_detail  = sqlc.arg(detail),
    -- `seen` n'est vrai que si le démon a effectivement répondu : un échec ne
    -- doit pas rafraîchir la date de dernier contact, sinon l'interface
    -- affiche « vu à l'instant » pour un démon injoignable depuis une heure.
    last_seen_at = CASE WHEN sqlc.arg(seen)::boolean THEN now() ELSE last_seen_at END
WHERE id;

-- name: DeleteEd2kDaemon :exec
DELETE FROM ed2k_daemon WHERE id;
