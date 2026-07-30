# Contribuer à boxincloud

Merci de l'intérêt que vous portez au projet. Ce document doit vous permettre d'être
opérationnel en quelques minutes.

## Sommaire

- [Environnement de développement](#environnement-de-développement)
- [Structure du projet](#structure-du-projet)
- [Règles d'architecture](#règles-darchitecture)
- [Code généré](#code-généré)
- [Base de données](#base-de-données)
- [Tests](#tests)
- [Commits et pull requests](#commits-et-pull-requests)
- [Par où commencer](#par-où-commencer)

---

## Environnement de développement

### Prérequis

| Outil | Version | Nécessaire pour |
|---|---|---|
| Go | 1.23+ | serveur |
| Docker + Compose | récent | PostgreSQL, MinIO |
| Node.js | 22+ | application web |
| Flutter | 3.24+ | application mobile |
| `make` | — | orchestration |

Vous n'avez besoin **que** des outils du domaine sur lequel vous travaillez. Une
contribution au serveur ne demande ni Node ni Flutter.

### Mise en route

```bash
git clone https://github.com/adonko3xBitters/boxincloud.git
cd boxincloud
cp .env.example .env
make deps          # installe les outils Go (sqlc, goose, oapi-codegen, air…)
make dev-deps      # démarre PostgreSQL et MinIO en arrière-plan
make dev-server    # démarre l'API avec rechargement à chaud
```

L'API écoute sur `http://localhost:8080`. Vérification :

```bash
curl http://localhost:8080/healthz
```

Services de développement :

| Service | Adresse | Identifiants |
|---|---|---|
| API | http://localhost:8080 | — |
| PostgreSQL | `localhost:5432` | `boxincloud` / `boxincloud` |
| MinIO (S3) | http://localhost:9000 | `boxincloud` / `boxincloud` |
| MinIO (console) | http://localhost:9001 | `boxincloud` / `boxincloud` |

`make dev-deps-down` arrête les services, `make dev-reset` remet la base à zéro.

---

## Structure du projet

Voir [docs/03-repo-structure.md](docs/03-repo-structure.md) pour l'arborescence complète.

```
api/              contrat OpenAPI — source de vérité
apps/server/      serveur Go
apps/web/         application web Next.js
apps/mobile/      application Flutter
packages/         ressources partagées (design tokens)
deploy/           Docker, Compose, templates de déploiement
docs/             documentation d'architecture et guides
tools/            scripts de génération et utilitaires
```

---

## Règles d'architecture

Trois règles non négociables. Une PR qui les enfreint sera refusée, quelle que soit sa
qualité par ailleurs.

### 1. Aucun accès direct au stockage

Tout passe par `internal/storage.Provider`. Aucun module ne fait `os.Open`, n'appelle un
SDK cloud, ni ne suppose l'existence d'un chemin de fichier.

```go
// ✗ Non
f, err := os.Open(comic.Path)

// ✓ Oui
r, err := provider.ReadRange(ctx, comic.ObjectKey, offset, length)
```

C'est ce qui rend le multi-backend possible. Une seule violation et la promesse du projet
tombe.

### 2. Un module métier ne dépend pas d'un autre module métier

`catalog` n'importe pas `library`. Si `catalog` a besoin d'une bibliothèque, il déclare
**chez lui** l'interface minimale dont il a besoin, et le câblage se fait dans `cmd/`.

```go
// internal/catalog/service.go
type LibraryReader interface {
    Get(ctx context.Context, id uuid.UUID) (Library, error)
}
```

C'est ce qui empêche le monolithe modulaire de devenir une boule de boue, et ce qui
rendra un découpage ultérieur possible sans réécriture.

### 3. Le contrat OpenAPI vient avant le code

Une évolution d'API commence par `api/openapi.yaml`, puis `make generate`, puis
l'implémentation. Jamais l'inverse — sinon le web et le mobile divergent.

---

## Code généré

Ces répertoires sont **produits par `make generate`** et ne doivent jamais être édités à
la main :

```
apps/server/internal/httpapi/gen/
apps/server/internal/platform/sqlc/
apps/web/src/lib/api/
apps/mobile/lib/core/api/
```

Ils sont **versionnés** : un contributeur peut compiler sans installer la chaîne de
génération. La CI vérifie qu'ils sont à jour — si votre PR échoue sur `generate-check`,
lancez `make generate` et committez le résultat.

---

## Base de données

### Nouvelle migration

```bash
make migrate-new name=add_reading_lists
# édite apps/server/migrations/NNNNN_add_reading_lists.sql
make migrate-up
```

Les migrations sont **embarquées dans le binaire** et appliquées automatiquement au
démarrage. Elles doivent être :

- **irréversibles en avant seulement** dans les faits, mais toujours écrites avec un
  bloc `-- +goose Down` fonctionnel ;
- **compatibles avec la version précédente** du code lorsque c'est possible (ajout de
  colonne nullable plutôt que renommage brutal) ;
- **testées sur une base non vide**.

### Nouvelle requête

Écrivez le SQL dans `apps/server/queries/`, puis `make generate`. sqlc produit du Go typé.
On n'écrit pas de SQL dans le code Go.

---

## Tests

```bash
make test              # unitaires, rapides, sans dépendance externe
make test-integration  # testcontainers : vrai PostgreSQL + vrai MinIO
make lint
```

**Le stockage ne se teste pas avec des mocks.** Les tests d'intégration tournent contre un
vrai MinIO via testcontainers. Un mock de S3 ne reproduit ni le comportement des requêtes
Range, ni les codes d'erreur, ni la sémantique des ETag — c'est-à-dire exactement ce qui
nous intéresse.

Les fichiers de test (CBZ, CBR, PDF minimaux, libres de droits) sont dans
`apps/server/testdata/` et générés par `tools/testfixtures/`.

---

## Commits et pull requests

### Format des commits

[Conventional Commits](https://www.conventionalcommits.org), avec portée :

```
feat(reader): ajoute le mode double page
fix(storage): gère les réponses 206 partielles de Backblaze B2
docs(architecture): précise la stratégie d'hydratation CBR
chore(deps): monte pgx en 5.7
```

Types : `feat`, `fix`, `docs`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`.
`release-please` en dérive les versions et le journal des modifications.

### Pull requests

- Une PR = un sujet. Les PR fourre-tout sont difficiles à relire et lentes à fusionner.
- `main` doit rester déployable. Pas de branche `develop`.
- CI verte obligatoire.
- Décrivez le **pourquoi**, pas seulement le quoi. Le diff dit déjà le quoi.
- Une capture ou un court enregistrement pour tout changement visible.

### Décisions d'architecture

Un choix structurant se documente dans `docs/adr/` — un fichier par décision, format
court : contexte, décision, conséquences. Ouvrez d'abord une issue de discussion.

---

## Par où commencer

- Les issues étiquetées [`good first issue`](https://github.com/adonko3xBitters/boxincloud/labels/good%20first%20issue)
  sont conçues pour une première contribution.
- Pour une fonctionnalité conséquente, **ouvrez une issue avant d'écrire du code**. Cela
  évite qu'un travail sérieux se heurte à une divergence de direction.
- Les questions sont bienvenues dans les Discussions. Il n'y a pas de question idiote sur
  un projet en pré-alpha.

---

## Code de conduite

Ce projet suit le [Contributor Covenant](CODE_OF_CONDUCT.md). Participer, c'est s'engager
à le respecter.
