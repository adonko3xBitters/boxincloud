-- +goose Up

-- Archive normalisée d'un album sans accès aléatoire.
--
-- Le CBR et le PDF ne permettent pas de servir une page par une requête Range :
-- le RAR solide compresse les fichiers comme un flux continu, et le PDF
-- demanderait un moteur de rendu. Ils sont donc convertis une fois en CBZ,
-- déposé dans le cache dérivé — jamais dans le stockage de l'utilisateur.
--
-- Quand cette colonne est renseignée, les offsets de comic_pages désignent
-- l'archive hydratée et non l'objet d'origine. La lecture retombe alors
-- exactement sur le chemin du CBZ, sans branchement supplémentaire.
--
-- Nullable : un CBZ n'a rien à hydrater, et c'est le cas courant.
ALTER TABLE comics ADD COLUMN hydrated_key text;

COMMENT ON COLUMN comics.hydrated_key IS
    'Clé, dans le cache dérivé, de l''archive CBZ normalisée. NULL pour un album à accès aléatoire natif.';

-- +goose Down
ALTER TABLE comics DROP COLUMN IF EXISTS hydrated_key;
