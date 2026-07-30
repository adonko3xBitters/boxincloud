<!--
Merci pour cette contribution.

Le titre de la PR doit suivre les Conventional Commits :
  feat(reader): ajoute le mode double page
  fix(storage): gère les réponses 206 partielles de Backblaze B2
-->

## Pourquoi

<!-- Le problème résolu ou le besoin couvert. Le diff dit déjà le « quoi » ;
     c'est le « pourquoi » qui manque au relecteur. -->

Closes #

## Ce qui change

<!-- Les points saillants. Inutile de paraphraser le diff. -->

-

## Vérification

<!-- Comment avez-vous vérifié que ça fonctionne ? -->

- [ ] Tests unitaires ajoutés ou mis à jour
- [ ] Tests d'intégration si le stockage ou la base sont touchés
- [ ] Vérifié manuellement — décrire comment :

## Captures

<!-- Obligatoire pour tout changement visible. Une capture avant/après, ou un
     court enregistrement pour une interaction. -->

## Points de contrôle

- [ ] `make lint` et `make test` passent
- [ ] Aucun accès direct au stockage — tout passe par `storage.Provider`
- [ ] Aucun module métier n'en importe un autre
- [ ] Le contrat OpenAPI a été mis à jour **avant** l'implémentation, le cas échéant
- [ ] `make generate` lancé et le code généré committé
- [ ] Documentation mise à jour si le comportement change
- [ ] Migration réversible (`-- +goose Down` fonctionnel), le cas échéant

## Ruptures de compatibilité

<!-- Rien à signaler ? Écrivez « Aucune ». Sinon, décrivez l'impact et la
     migration nécessaire. Attention particulière à l'API : une version
     ancienne de l'app mobile peut rester installée longtemps. -->

Aucune.
