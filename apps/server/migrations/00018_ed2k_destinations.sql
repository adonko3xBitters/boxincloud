-- +goose Up

-- ─── Destinations : ce que devient un fichier terminé ────────────────────────
--
-- La décision fondatrice du module : un téléchargement terminé va soit rester
-- où le démon l'a mis, soit entrer dans une bibliothèque boxincloud. Et c'est
-- la CATÉGORIE qui tranche, fichier par fichier — « Linux » reste sur disque,
-- « BD » devient un album.
--
-- Sans cela, le module ne serait qu'un aMule avec une autre façade. Ce qu'il
-- apporte à ce projet-ci, c'est précisément ce pont.

CREATE TABLE ed2k_destinations (
    -- La catégorie telle que le démon la numérote. Zéro est sa catégorie par
    -- défaut, celle de tout fichier ajouté sans précision.
    --
    -- On s'aligne sur SA numérotation plutôt que d'en tenir une nôtre : c'est
    -- lui qui range les fichiers, et deux numérotations qui glissent l'une par
    -- rapport à l'autre publieraient un jour un film dans la bibliothèque de
    -- BD sans que rien ne le signale.
    category integer PRIMARY KEY CHECK (category >= 0),

    -- Nom d'affichage, côté boxincloud. Le démon a le sien ; le nôtre existe
    -- pour que l'interface reste lisible même quand la configuration du démon
    -- n'est pas accessible.
    label text NOT NULL,

    -- Bibliothèque de destination. NULL signifie « laisser sur disque », ce qui
    -- est le comportement par défaut et le seul qui vaille pour ce qui n'est
    -- pas une bande dessinée.
    --
    -- ON DELETE CASCADE : supprimer une bibliothèque doit emporter la règle qui
    -- la vise. La garder produirait une destination fantôme dont chaque
    -- publication échouerait, sans que personne comprenne pourquoi.
    library_id uuid REFERENCES libraries(id) ON DELETE CASCADE,

    -- Dossier dans la bibliothèque, relatif à son préfixe. Chaîne vide pour la
    -- racine.
    folder text NOT NULL DEFAULT '',

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER ed2k_destinations_updated_at
    BEFORE UPDATE ON ed2k_destinations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ─── Publications : ce qui a déjà été publié ─────────────────────────────────
--
-- Le journal de ce que le pont a fait, et la garantie qu'il ne le refera pas.
--
-- Il existe pour une raison précise, écrite dans l'ADR-005 : les événements
-- sont DÉRIVÉS de la comparaison de deux instantanés, et un téléchargement qui
-- démarre et se termine entre deux tours n'en produit aucun. La détection ne
-- peut donc pas reposer sur l'événement — elle repose sur l'état, et cette
-- table est ce qui distingue « déjà publié » de « à publier ».

CREATE TABLE ed2k_publications (
    -- L'empreinte eD2k identifie le fichier sur le réseau, indépendamment de
    -- son nom et du numéro interne du démon, qui changent tous les deux.
    hash text PRIMARY KEY,

    name       text NOT NULL,
    size       bigint NOT NULL,
    category   integer NOT NULL,

    -- Renseignés quand la publication a réussi.
    library_id uuid REFERENCES libraries(id) ON DELETE SET NULL,
    comic_id   uuid,

    -- État de la publication : pending, published, skipped, error.
    --
    -- `skipped` n'est pas un échec : c'est le cas nominal d'une catégorie qui
    -- laisse ses fichiers sur disque. L'inscrire évite de reconsidérer le même
    -- fichier à chaque tour de scrutation.
    status text NOT NULL,
    detail text,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ed2k_publications_pending ON ed2k_publications (status)
    WHERE status = 'pending';

CREATE TRIGGER ed2k_publications_updated_at
    BEFORE UPDATE ON ed2k_publications
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down

DROP TABLE IF EXISTS ed2k_publications;
DROP TABLE IF EXISTS ed2k_destinations;
