-- +goose Up

-- ─── Module eD2k/Kad : connexion au démon aMule ──────────────────────────────
--
-- Le module ne réimplémente aucun protocole pair-à-pair : il pilote un démon
-- aMule par son protocole External Connections. Voir
-- docs/adr/004-amuled-plutot-que-reimplementation.md.
--
-- Cette migration ne crée QUE ce que l'étape 0 utilise réellement. Les tables
-- d'annotation — catégories, historique, audit, préférences, droits — arrivent
-- avec les étapes qui les lisent. La migration 00016 a laissé la leçon écrite :
-- un schéma qui porte les traces d'une fonctionnalité absente coûte à chaque
-- lecture, parce que quelqu'un finit par se demander ce qui le remplit.

-- Une table dédiée plutôt qu'une clé dans `settings`, pour une seule raison :
-- le mot de passe.
--
-- `settings.value` est du jsonb, lu en bloc par ListSettings. Y glisser un
-- secret, fût-il chiffré, c'est accepter qu'il traverse un jour une réponse
-- d'API par le chemin le plus banal — quelqu'un ajoute un écran « paramètres
-- avancés » qui liste les réglages, et le secret part avec. Une colonne bytea
-- dans sa propre table rend ce chemin inexistant.
CREATE TABLE ed2k_daemon (
    -- Ligne unique. Le CHECK est ce qui la rend unique : une instance pilote un
    -- démon, et deux lignes signifieraient deux moteurs pour une interface.
    id boolean PRIMARY KEY DEFAULT true CHECK (id),

    -- Adresse du démon, telle qu'elle est joignable DEPUIS CE SERVEUR.
    -- « amuled » sur un réseau Docker interne, ou l'IP du NAS où aMule tourne
    -- déjà. Contrôlée par netguard avant enregistrement : c'est une adresse
    -- saisie depuis l'interface qui devient une connexion émise par le serveur.
    host text NOT NULL,
    port integer NOT NULL CHECK (port > 0 AND port < 65536),

    -- Mot de passe EC, chiffré en AES-256-GCM avec BOXINCLOUD_SECRET_KEY.
    -- Ne sort jamais de la base, pas même pour un administrateur — même
    -- discipline que storage_backends.secrets_enc.
    password_enc bytea NOT NULL,

    -- Nom donné par l'administrateur, pour distinguer deux instances dans les
    -- journaux. Purement cosmétique.
    label text,

    -- Dernier état constaté, écrit par la couche d'intégration.
    --
    -- Persisté plutôt que gardé en mémoire pour une raison précise : après un
    -- redémarrage, l'interface doit pouvoir dire « la dernière connexion a
    -- échoué, voici pourquoi » au lieu d'afficher un état neutre qui laisse
    -- croire que rien n'a jamais été tenté.
    last_state  text,
    last_detail text,
    last_seen_at timestamptz,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER ed2k_daemon_updated_at
    BEFORE UPDATE ON ed2k_daemon
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down

DROP TABLE IF EXISTS ed2k_daemon;
