-- +goose Up

-- ─── Retrait de la recherche fédérée ─────────────────────────────────────────

-- La fonctionnalité est retirée du projet avant sa première version publique.
-- Ces tables ne portent que sa configuration : catalogues déclarés, règles
-- d'extraction, suivi des imports. Rien d'autre ne les lit.
--
-- Elles sont supprimées plutôt que laissées en place. Un schéma qui conserve
-- les traces d'une fonctionnalité absente coûte à chaque lecture : quelqu'un
-- finit par se demander ce qui les remplit, et la réponse — « plus rien » — ne
-- s'écrit nulle part.
--
-- L'ordre compte : les imports référencent les catalogues.
DROP TABLE IF EXISTS discovery_imports;
DROP TABLE IF EXISTS discovery_sources;

-- +goose Down

-- Pas de retour en arrière.
--
-- Recréer des tables vides ne restaurerait pas la fonctionnalité, dont le code
-- n'existe plus. Une migration descendante qui ment sur ce qu'elle rend est
-- pire qu'une migration descendante absente : l'historique git, lui, garde
-- tout.
SELECT 1;
