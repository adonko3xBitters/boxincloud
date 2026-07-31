# Arborescence du monorepo

Monorepo unique. Trois artefacts (serveur Go, web, mobile Flutter) partagent un contrat OpenAPI ; les séparer imposerait de synchroniser trois dépôts à chaque évolution d'API — coût permanent, bénéfice nul à cette échelle.

Pas d'outil de monorepo (Nx, Turborepo, Bazel) : chaque écosystème garde ses outils natifs, et un `Makefile` à la racine orchestre. Un contributeur Flutter n'a pas à installer Node pour travailler.

```
boxincloud/
├── README.md                     # pitch, captures, quickstart en 3 lignes
├── LICENSE                       # AGPL-3.0
├── CONTRIBUTING.md
├── CODE_OF_CONDUCT.md
├── SECURITY.md
├── CHANGELOG.md                  # généré par release-please
├── Makefile                      # dev · build · test · lint · generate
├── docker-compose.yml            # démo/production : postgres + minio + boxincloud
├── docker-compose.dev.yml        # dépendances seules, pour développer hors conteneur
├── .env.example
│
├── api/
│   ├── openapi.yaml              # ★ SOURCE DE VÉRITÉ du contrat
│   ├── components/               # schémas réutilisables, découpés par domaine
│   └── README.md                 # règles de versionnement et de compatibilité
│
├── apps/
│   ├── server/                   # ─── Go ───────────────────────────────
│   │   ├── go.mod
│   │   ├── cmd/
│   │   │   ├── boxincloud/main.go        # binaire unique (api + workers)
│   │   │   └── boxincloudctl/main.go     # CLI admin : createuser, scan, migrate
│   │   ├── internal/
│   │   │   ├── config/                   # chargement + validation env
│   │   │   ├── httpapi/
│   │   │   │   ├── router.go
│   │   │   │   ├── middleware/           # auth, log, cors, ratelimit, recover
│   │   │   │   ├── gen/                  # ← généré par oapi-codegen (non édité)
│   │   │   │   └── handlers/             # un fichier par ressource
│   │   │   ├── auth/                     # argon2id, JWT, sessions, devices
│   │   │   ├── library/                  # bibliothèques, backends, accès
│   │   │   ├── catalog/                  # series, comics, tags, people, collections
│   │   │   ├── reader/                   # service de pages, manifeste, prefetch
│   │   │   ├── progress/                 # progression + protocole de sync
│   │   │   ├── indexer/                  # jobs de scan et d'ingestion
│   │   │   ├── storage/                  # ★ Provider + s3/ local/ webdav/
│   │   │   ├── archive/                  # cbz/ cbr/ pdf/ — index et extraction
│   │   │   ├── imaging/                  # vips/ pure/ — vignettes, transcodage
│   │   │   ├── cache/                    # cache dérivé + éviction LRU
│   │   │   └── platform/
│   │   │       ├── db/                   # pool pgx, transactions
│   │   │       ├── sqlc/                 # ← généré (non édité)
│   │   │       ├── jobs/                 # client River, enregistrement des workers
│   │   │       ├── events/               # bus interne (in-process)
│   │   │       ├── crypto/               # chiffrement des secrets de backend
│   │   │       └── logging/
│   │   ├── migrations/                   # goose, .sql, embarquées
│   │   ├── queries/                      # .sql sources de sqlc
│   │   ├── testdata/                     # CBZ/CBR/PDF minimaux, libres de droits
│   │   ├── web/embed.go                  # embarque le bundle web dans le binaire
│   │   ├── sqlc.yaml
│   │   └── Dockerfile
│   │
│   ├── web/                      # ─── Next.js (export statique) ─────────
│   │   ├── package.json
│   │   ├── next.config.ts                # output: 'export'
│   │   ├── src/
│   │   │   ├── app/
│   │   │   │   ├── (auth)/               # login, première installation
│   │   │   │   ├── (app)/
│   │   │   │   │   ├── library/[id]/
│   │   │   │   │   ├── series/[id]/
│   │   │   │   │   ├── comics/[id]/
│   │   │   │   │   ├── collections/
│   │   │   │   │   └── settings/
│   │   │   │   └── read/[comicId]/       # lecteur plein écran, hors chrome applicatif
│   │   │   ├── components/
│   │   │   │   ├── ui/                   # shadcn/ui
│   │   │   │   ├── library/              # grille virtualisée, cartes, filtres
│   │   │   │   └── reader/               # ★ le composant le plus soigné du projet
│   │   │   ├── lib/
│   │   │   │   ├── api/                  # ← client généré (non édité)
│   │   │   │   ├── queries/              # hooks TanStack Query
│   │   │   │   └── stores/               # Zustand — état du lecteur
│   │   │   └── styles/
│   │   ├── e2e/                          # Playwright
│   │   └── public/
│   │
│   └── mobile/                   # ─── Flutter ──────────────────────────
│       ├── pubspec.yaml
│       ├── lib/
│       │   ├── main.dart
│       │   ├── app/                      # router, thème, cycle de vie
│       │   ├── core/
│       │   │   ├── api/                  # ← client généré (non édité)
│       │   │   ├── db/                   # schéma Drift, DAOs
│       │   │   ├── sync/                 # file d'opérations, réconciliation
│       │   │   └── download/             # background_downloader, éviction
│       │   ├── features/
│       │   │   ├── auth/
│       │   │   ├── library/
│       │   │   ├── series/
│       │   │   ├── reader/               # ★ pendant mobile du lecteur web
│       │   │   ├── downloads/
│       │   │   └── settings/
│       │   └── shared/                   # widgets, design tokens partagés
│       ├── integration_test/
│       ├── android/
│       └── ios/
│
├── packages/
│   └── design-tokens/            # couleurs, espacements, rayons, durées
│       ├── tokens.json           # ★ source unique
│       └── build.mjs             # → CSS vars (web) + Dart consts (mobile)
│
├── deploy/
│   ├── docker/
│   │   ├── Dockerfile            # multi-étages : web → go → runtime slim
│   │   └── entrypoint.sh
│   ├── compose/                  # variantes : minimal, avec MinIO, avec Traefik
│   ├── helm/                     # post-V1
│   └── unraid/                   # template — canal d'adoption self-hosted majeur
│
├── docs/
│   ├── 01-architecture.md
│   ├── 02-data-model.md
│   ├── 03-repo-structure.md
│   ├── 04-roadmap.md
│   ├── adr/                      # décisions ultérieures, une par fichier
│   ├── contributing/             # guides par domaine : backend, web, mobile
│   ├── deployment/               # installation, migration depuis Komga/Kavita
│   └── site/                     # documentation publique (VitePress), post-V1
│
├── tools/
│   ├── generate-api.sh           # openapi.yaml → Go + TS + Dart
│   ├── seed.go                   # jeu de données de démonstration
│   └── testfixtures/             # génération des archives de test
│
└── .github/
    ├── workflows/                # ci-server · ci-web · ci-mobile · release · docker
    ├── ISSUE_TEMPLATE/           # bug · feature · support
    ├── PULL_REQUEST_TEMPLATE.md
    ├── dependabot.yml
    └── FUNDING.yml               # ★ Buy Me a Coffee + GitHub Sponsors
```

---

## Conventions structurantes

**Le code généré n'est jamais édité, et il est versionné.** `httpapi/gen/`, `platform/sqlc/`, `web/src/lib/api/`, `mobile/lib/core/api/` sont produits par `make generate`. On les commite : un contributeur peut compiler sans installer la chaîne de génération. La CI vérifie qu'ils sont à jour (`make generate && git diff --exit-code`).

**Un module ne dépend pas d'un autre module.** `catalog` ne fait pas `import ".../library"`. Si `catalog` a besoin d'une bibliothèque, il définit chez lui l'interface minimale dont il a besoin, et le câblage se fait dans `cmd/`. C'est la règle qui rend possible un découpage ultérieur — et qui empêche le monolithe modulaire de redevenir une boule de boue.

**`internal/` partout.** Rien du serveur n'est importable de l'extérieur. Si un jour une bibliothèque publique est utile (un lecteur de CBZ à accès aléatoire, par exemple, qui n'existe pas encore en Go), elle sera extraite volontairement.

**Les design tokens sont partagés.** Le web et le mobile lisent le même `tokens.json`. C'est peu de code et c'est ce qui évite que les deux applications divergent visuellement en six mois — un risque réel quand l'UX est le différenciateur affiché.

**Commits conventionnels.** `feat:`, `fix:`, `chore:`… avec portée : `feat(reader): ...`. `release-please` en dérive versions et journal des modifications.

**Branches.** `main` toujours déployable. Branches de fonctionnalité, PR obligatoire, CI verte. Pas de branche `develop`.

---

## `Makefile` cible

```make
make dev          # postgres + minio + air (hot reload Go) + next dev + flutter run
make generate     # openapi → clients ; sqlc → Go ; tokens → css/dart
make test         # go test ./... + vitest + flutter test
make test-integration  # testcontainers : postgres + minio réels
make lint         # golangci-lint + eslint + dart analyze
make build        # bundle web → embed → binaire unique
make docker       # image multi-arch
make migrate-new name=xxx
```

---

## Publication mobile

L'interface web propose l'application Android derrière un code QR
(« Application mobile » dans le menu de compte). Le code mène à
`/telecharger`, page publique servie par l'instance elle-même : le téléphone
qui scanne apprend ainsi l'adresse du serveur — celle par laquelle il vient
d'arriver — en même temps qu'il récupère l'application.

L'APK n'est pas servi par l'instance mais par les versions GitHub du projet.
Il pèse une soixantaine de mégaoctets, identiques d'une installation à l'autre :
les embarquer dans chaque image self-hosted les ferait payer à tout le monde
pour un fichier que la plupart ne serviront jamais.

Le lien pointe vers `releases/latest/download/boxincloud-android.apk`, sans
numéro de version. Une instance qui n'a pas été mise à jour depuis six mois
propose donc quand même l'application courante — c'est le protocole qui est
versionné, pas le client. **Ce nom d'artefact est un contrat** entre
`.github/workflows/release-mobile.yml` et `apps/web/src/lib/mobile-app.ts` :
le renommer casserait le bouton de téléchargement de toutes les instances déjà
déployées, sans que rien n'échoue à la construction.

### Secrets requis

La publication est déclenchée par un tag `v*`. Elle échoue franchement si la
clé de signature manque, plutôt que de produire un APK signé avec la clé de
debug : Android identifie une application par sa clé autant que par son
identifiant, et publier une fois avec la clé de debug interdirait toute mise à
jour signée autrement.

| Secret | Contenu |
| --- | --- |
| `ANDROID_KEYSTORE_BASE64` | Le fichier `.jks`, encodé en base64 |
| `ANDROID_KEYSTORE_PASSWORD` | Mot de passe du magasin |
| `ANDROID_KEY_ALIAS` | Alias de la clé |
| `ANDROID_KEY_PASSWORD` | Mot de passe de la clé |

Créer la clé, une fois pour toutes — et la sauvegarder ailleurs qu'ici, sa
perte étant définitive :

```sh
keytool -genkey -v -keystore release.jks -keyalg RSA -keysize 2048 \
        -validity 10000 -alias boxincloud
base64 -i release.jks | pbcopy   # à coller dans ANDROID_KEYSTORE_BASE64
```

En local, aucun secret n'est nécessaire : sans `android/key.properties`, la
release retombe sur la clé de debug et `flutter run --release` fonctionne.
