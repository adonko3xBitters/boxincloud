-- +goose Up

-- ─── Imports depuis les catalogues fédérés ───────────────────────────────────

-- Suivi des imports.
--
-- Cette table est la conséquence directe du passage en tâche de fond. Tant que
-- l'import était synchrone, son état tenait dans la réponse HTTP : elle arrivait
-- ou elle échouait, et l'utilisateur avait sa réponse sous les yeux.
--
-- Un import de fond n'a pas cette chance. Il survit à la fermeture de l'onglet,
-- dure parfois plusieurs minutes, et peut échouer longtemps après que celui qui
-- l'a demandé a quitté la page. Sans trace persistée, il devient une action
-- qu'on lance et qui disparaît : on ne sait ni si elle a réussi, ni pourquoi
-- elle a échoué, ni s'il faut la relancer.
--
-- C'est aussi ce qui rend le nouveau comportement acceptable. Rendre 202 au lieu
-- de 201 ne serait pas un progrès si l'on ne rendait rien à interroger ensuite.
--
-- La ligne est écrite AVANT l'enfilement, et les deux ne sont pas dans une même
-- transaction. Le décalage est assumé et traité : si l'enfilement échoue, la
-- ligne est immédiatement marquée en échec. Un import « en attente » qui ne
-- démarre jamais serait le pire des deux mondes — invisible dans les journaux,
-- et indiscernable d'un simple retard pour qui le regarde.
CREATE TABLE discovery_imports (
    id         uuid PRIMARY KEY,

    -- Le catalogue est conservé pour l'affichage, mais son effacement ne doit
    -- pas emporter l'historique : un import réussi reste un fait même si l'on
    -- retire la source ensuite.
    source_id  uuid REFERENCES discovery_sources(id) ON DELETE SET NULL,
    source_name text NOT NULL DEFAULT '',

    href       text NOT NULL,
    library_id uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    folder     text NOT NULL DEFAULT '',
    title      text NOT NULL DEFAULT '',

    -- queued | running | done | failed
    status     text NOT NULL DEFAULT 'queued',

    -- Code STABLE, pas une phrase : l'interface le traduit, comme pour les
    -- problèmes RFC 7807 et les états de catalogue.
    error_code text NOT NULL DEFAULT '',
    -- Diagnostic brut, souvent celui du catalogue distant. Affiché comme un
    -- détail technique sous un titre traduit, jamais comme une phrase à lire.
    error_detail text NOT NULL DEFAULT '',

    -- Renseignés quand l'import aboutit.
    comic_id   uuid REFERENCES comics(id) ON DELETE SET NULL,
    object_key text NOT NULL DEFAULT '',
    file_size  bigint NOT NULL DEFAULT 0,

    -- Qui a demandé. L'import écrit dans une bibliothèque : savoir de qui vient
    -- l'écriture est le minimum quand plusieurs comptes se la partagent.
    requested_by uuid REFERENCES users(id) ON DELETE SET NULL,

    created_at  timestamptz NOT NULL DEFAULT now(),
    started_at  timestamptz,
    finished_at timestamptz
);

CREATE INDEX discovery_imports_recent ON discovery_imports (created_at DESC);

-- Les imports en cours, que l'interface interroge en boucle pendant qu'ils
-- tournent. L'index partiel garde cette requête constante quelle que soit la
-- taille de l'historique.
CREATE INDEX discovery_imports_active ON discovery_imports (created_at DESC)
    WHERE status IN ('queued', 'running');

-- +goose Down

DROP TABLE discovery_imports;
