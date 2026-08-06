# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Langue du projet

Tout est écrit en français : documentation, commentaires de code, messages de
commit, chaînes d'interface (l'anglais est une traduction, pas la source).
Écrivez en français dans le dépôt, y compris les commentaires que vous ajoutez.

## Commandes

Toujours depuis la racine — c'est ce qui charge `.env` et rend les chemins justes.
`docs/07-commandes.md` est le catalogue complet ; voici l'essentiel.

```bash
make env                # crée .env avec une BOXINCLOUD_SECRET_KEY générée
make deps               # outils Go : sqlc, goose, oapi-codegen, air, golangci-lint
make dev-deps           # PostgreSQL :5432 et MinIO :9000 (console :9001)
make run                # API sur :8080, sans outil supplémentaire
make dev-server         # idem avec rechargement à chaud (exige air)
make dev-web            # interface sur :3000, pointée vers l'API du .env
make ctl ARGS="user list"   # boxincloudctl avec .env chargé
```

**Qualité**

```bash
make test               # unitaires : go test ./... -race -short
make test-integration   # testcontainers : PostgreSQL et MinIO réels
make lint               # go vet + golangci-lint
```

Un seul test Go : `cd apps/server && go test ./internal/archive -run TestZipIndex -v`.
Les tests d'intégration se reconnaissent au nom (`-run Integration`) et exigent Docker.
Certains tests d'archive demandent `p7zip-full` installé.

**Génération** — jamais éditer la sortie à la main (voir plus bas)

```bash
make generate           # api + sql + tokens
make generate-api       # api/openapi.yaml → Go, TypeScript, Dart
make generate-sql       # apps/server/queries/*.sql → sqlc
make generate-tokens    # packages/design-tokens/tokens.json → CSS + Dart
make generate-check     # échoue si le généré est obsolète (ce que fait la CI)
cd apps/mobile && dart run build_runner build   # code Drift, hors make generate
```

**Base**

```bash
make migrate-new name=add_reading_lists   # puis éditer apps/server/migrations/NNNNN_*.sql
make migrate-up / migrate-down / migrate-status
```

**Web** (`cd apps/web`) — la CI lance tout ceci ; ce sont des contrôles maison qui
attrapent des défauts qu'aucun linter ne voit, lisez l'en-tête du script concerné
avant de le contourner :

```bash
npm run typecheck && npm run lint && npm run build
npm run check:css        # classes Tailwind référençant une var CSS avec la mauvaise syntaxe
npm run check:overlays   # calques qui se ferment sur un stopPropagation inopérant
npm run check:nested-buttons  # un élément cliquable dans un autre, même à travers un composant
npm run check:qr         # un code QR produit se relit vraiment
npm run check:i18n       # couverture de traduction — échoue si le reste-à-faire augmente
npm run check:contrast   # WCAG AA sur les paires de jetons réellement affichées
```

**Mobile** (`cd apps/mobile`) : `flutter analyze`, `flutter test`, `flutter run`.

**Build**

```bash
make build       # bin/boxincloud et bin/boxincloudctl
make build-web   # export Next.js → apps/server/web/dist (embarqué)
make build-all   # web puis binaire
make docker WITH_MOBILE=0   # image sans la chaîne Android (obligatoire sur arm64)
```

## Architecture

Monolithe modulaire Go, un seul binaire qui embarque l'export statique Next.js
(`apps/server/web/dist`) et l'APK Android (`apps/server/mobile/dist`). Deux
conteneurs en production : PostgreSQL et lui. Pas de Redis — River fait la file
de jobs sur PostgreSQL. Détail et justifications dans `docs/01-architecture.md`.

```
api/openapi.yaml          contrat, source de vérité des trois clients
apps/server/internal/
  httpapi/                routeur Chi, handlers, middleware, gen/ (oapi-codegen)
  accounts auth catalog folders library progress reader   modules métier
  indexer ingest archive imaging cache                    traitement
  storage/                Provider + implémentations s3, local
  platform/               db, jobs (River), sqlc, crypto, logging, netguard
  app/                    wiring.go — le seul endroit qui câble les modules
apps/web/src/             Next.js export statique, TanStack Query, Tailwind 4
apps/mobile/lib/          Flutter, Riverpod, Drift, offline-first
packages/design-tokens/   tokens.json → CSS (web) + Dart (mobile)
```

Règle de dépendance : `http → métier → platform`. `internal/app/wiring.go`
construit un `Core` partagé par le serveur et le CLI — les deux binaires font
les mêmes choses avec le même câblage.

### Trois règles non négociables

Elles viennent de `CONTRIBUTING.md` et une PR qui les enfreint est refusée.

1. **Aucun accès direct au stockage.** Tout passe par `internal/storage.Provider`.
   Pas de `os.Open`, pas d'appel de SDK cloud, pas d'hypothèse de chemin de
   fichier ailleurs que dans une implémentation de `Provider`. C'est ce qui rend
   le multi-backend possible ; une seule violation et la promesse du projet tombe.
2. **Un module métier ne dépend pas d'un autre module métier.** `catalog`
   n'importe pas `library` : il déclare chez lui l'interface minimale dont il a
   besoin, et `cmd/` câble.
3. **Le contrat avant le code.** Une évolution d'API commence par
   `api/openapi.yaml`, puis `make generate`, puis l'implémentation. Jamais
   l'inverse, sinon le web et le mobile divergent.

### Code généré, versionné, jamais édité

```
apps/server/internal/httpapi/gen/     oapi-codegen
apps/server/internal/platform/sqlc/   sqlc, depuis apps/server/queries/*.sql
apps/web/src/lib/api/schema.d.ts      openapi-typescript
apps/mobile/lib/core/api/             tools/generate-dart-models.mjs (générateur maison)
apps/web/src/styles/tokens.css        packages/design-tokens/build.mjs
apps/mobile/lib/**/*.g.dart           build_runner (Drift)
```

Tout cela est commité pour qu'un contributeur compile sans la chaîne de
génération. La CI échoue si c'est obsolète : lancez `make generate` et committez.

### Ce qui est particulier au produit

- **Le chemin chaud est `ReadRange`.** Servir une page d'un CBZ, c'est une seule
  requête HTTP Range sur l'archive distante, grâce à l'index ZIP persisté en base
  (`comic_pages`) à l'indexation. CBR, CB7, PDF et EPUB, incapables d'accès
  aléatoire, sont convertis une fois à l'indexation — après quoi ils coûtent le
  prix d'un CBZ. Toute optimisation qui ajoute un aller-retour ici se paie sur
  chaque page tournée.
- **Pas de SQL dans le Go.** Le SQL vit dans `apps/server/queries/`, sqlc en tire
  du Go typé.
- **Le stockage ne se teste pas avec des mocks.** Les tests d'intégration tournent
  contre un vrai MinIO via testcontainers : un mock ne reproduit ni les requêtes
  Range, ni les codes d'erreur, ni la sémantique des ETag — c'est-à-dire
  exactement ce qui nous intéresse. Fixtures dans `apps/server/testdata/`.
- **Autorisation par `requireAdmin(w, r)`** en tête de handler, pas par groupe de
  routes (routes admin et ouvertes s'entremêlent). Le filet est
  `internal/httpapi/contract_authz_test.go` : toute route réservée y est
  appelée avec un compte ordinaire (403 attendu) *et* avec un admin. Ajouter une
  route réservée sans l'y inscrire, c'est perdre la garantie.
- **Négociation d'image stricte.** Pages en WebP/JPEG, couvertures en
  AVIF/WebP/JPEG ; seule une mention explicite dans `Accept` compte, un joker
  reçoit du JPEG. Les réponses portent `Vary: Accept` (304 compris) et l'ETag
  inclut le format — sans quoi un proxy sert l'AVIF du navigateur à
  l'application Android.
- **Migrations embarquées** et appliquées au démarrage ; toujours écrire un bloc
  `-- +goose Down` fonctionnel, et préférer l'ajout de colonne nullable au
  renommage brutal.

## Commits

[Conventional Commits](https://www.conventionalcommits.org) avec portée, en
français : `feat(reader): ajoute le mode double page`,
`fix(storage): gère les 206 partiels de Backblaze B2`. Types : `feat`, `fix`,
`docs`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`. `release-please` en
dérive les versions et `CHANGELOG.md`.

Une PR = un sujet, `main` reste déployable, la PR décrit le **pourquoi**. Un
choix structurant se documente dans `docs/adr/`.
