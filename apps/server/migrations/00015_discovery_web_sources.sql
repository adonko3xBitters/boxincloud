-- +goose Up

-- ─── Sources décrites depuis l'interface ─────────────────────────────────────

-- Un site sans flux OPDS ne se lit qu'en désignant, dans sa page, où se trouvent
-- le titre, l'auteur, la couverture et le lien. Cette description vivait jusqu'ici
-- dans un fichier YAML posé sur le disque de l'instance, ce qui suppose un accès
-- au serveur — et met la fonctionnalité hors de portée de qui administre depuis
-- son navigateur.
--
-- Elle peut désormais être saisie dans le formulaire, et c'est ici qu'elle est
-- rangée.
--
-- Pourquoi une colonne JSON plutôt que six colonnes typées : ces règles sont un
-- ensemble, jamais interrogé champ par champ. Aucune requête ne cherchera « les
-- sources dont le sélecteur de titre vaut h3 ». Les éclater en colonnes
-- imposerait une migration à chaque règle ajoutée — une expression rationnelle,
-- un second lien de téléchargement — pour un schéma que personne ne filtre.
--
-- Le contenu reste modeste et validé avant écriture : une URL de recherche, un
-- sélecteur de ligne, quatre sélecteurs de champ. Le serveur le traduit ensuite
-- dans le même format que les gabarits sur disque, pour qu'il n'existe qu'un
-- seul moteur d'extraction — voir internal/discovery/scraper.
ALTER TABLE discovery_sources
    ADD COLUMN template jsonb;

-- Le genre `web` désigne ces sources-là. Il se distingue de `scraper:<gabarit>`,
-- qui renvoie à un fichier chargé au démarrage : ici la description est DANS la
-- ligne, et supprimer la source suffit à la faire disparaître.
COMMENT ON COLUMN discovery_sources.template IS
    'Règles d''extraction pour kind = web. Nul pour les autres genres.';

-- Une source `web` sans règles serait interrogée sans qu''on sache quoi lire, et
-- des règles sur un catalogue OPDS n''auraient aucun effet — deux incohérences
-- silencieuses que la base peut refuser elle-même.
ALTER TABLE discovery_sources
    ADD CONSTRAINT discovery_sources_template_matches_kind
    CHECK ((kind = 'web') = (template IS NOT NULL));

-- +goose Down

ALTER TABLE discovery_sources
    DROP CONSTRAINT discovery_sources_template_matches_kind;

ALTER TABLE discovery_sources
    DROP COLUMN template;
