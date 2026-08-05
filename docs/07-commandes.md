# Commandes

Toutes les commandes du dépôt, avec ce qu'elles font et quand s'en servir.

Deux règles valent pour tout ce qui suit.

**Lancez `make` depuis la racine du dépôt.** Les chemins sont relatifs à elle, et
c'est aussi ce qui permet à `make` de charger `.env`.

**Les binaires ne lisent pas `.env`.** La configuration vient de
l'environnement — en production c'est Docker ou systemd qui la fournit. En
développement, `make run` et `make ctl` le chargent pour vous. Appeler
`./bin/boxincloud` directement demande d'exporter les variables soi-même.

> `source .env` n'est pas une solution fiable : le shell coupe les valeurs
> contenant un espace, ce qui arrive dès que le dépôt vit dans un répertoire
> comme `Mes projets`. `make` et `docker compose` les lisent correctement.

---

## Démarrer, la première fois

```bash
make env        # crée .env depuis .env.example, avec une clé secrète générée
make dev-deps   # PostgreSQL et MinIO dans Docker
make run        # l'API sur :8080
make dev-web    # l'interface sur :3000, dans un autre terminal
```

Ouvrez **http://localhost:3000**. L'assistant crée le compte administrateur,
connecte un backend et lance un premier scan.

`make run` ne demande que Go. `make dev-server` fait la même chose avec
rechargement à chaud, mais exige `air` — donc `make deps` au préalable.

---

## `make` — orchestration

### Environnement

| Commande | Effet |
|---|---|
| `make help` | Liste les cibles disponibles. C'est le défaut. |
| `make env` | Crée `.env` depuis `.env.example` et génère `BOXINCLOUD_SECRET_KEY`. Ne fait rien si `.env` existe. |
| `make deps` | Installe les outils Go : `sqlc`, `goose`, `oapi-codegen`, `air`, `golangci-lint`. Nécessaire seulement pour générer, migrer ou linter. |

Les outils vont dans `$(go env GOPATH)/bin`. Le Makefile l'ajoute au `PATH`
tout seul ; pour les appeler depuis votre shell, ajoutez à `~/.zshrc` :

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Développement

| Commande | Effet |
|---|---|
| `make dev-deps` | Démarre PostgreSQL (`:5432`) et MinIO (`:9000`, console `:9001`). |
| `make dev-deps-down` | Les arrête, en gardant les données. |
| `make dev-reset` | **Détruit les volumes** et redémarre à vide. Irréversible. |
| `make run` | Démarre l'API. Aucun outil à installer. |
| `make dev-server` | Idem, avec rechargement à chaud. Demande `air`. |
| `make dev` | `dev-deps` puis `dev-server`. |
| `make dev-web` | L'interface sur `:3000`, avec rechargement à chaud. |
| `make dev-mobile` | L'application Flutter sur un appareil connecté. |
| `make ctl ARGS="…"` | Lance `boxincloudctl` avec `.env` chargé. Voir plus bas. |

### Génération

Le code généré est **versionné**. La CI échoue s'il n'est pas à jour : lancez
`make generate` et committez le résultat.

| Commande | Effet |
|---|---|
| `make generate` | Tout régénérer. |
| `make generate-api` | `api/openapi.yaml` → serveur Go, client TypeScript, modèles Dart. |
| `make generate-sql` | `apps/server/queries/*.sql` → Go typé, via sqlc. |
| `make generate-tokens` | `tokens.json` → variables CSS et constantes Dart. |
| `make generate-check` | Régénère puis échoue si quelque chose a changé. Utilisé en CI. |

### Base de données

| Commande | Effet |
|---|---|
| `make migrate-up` | Applique les migrations en attente. |
| `make migrate-down` | **Annule la dernière migration.** Peut perdre des données. |
| `make migrate-status` | Affiche l'état des migrations. |
| `make migrate-new name=ma_migration` | Crée une paire de fichiers de migration. |

Le serveur applique les migrations au démarrage quand
`BOXINCLOUD_DB_AUTO_MIGRATE` est vrai, ce qui est le défaut.

### Qualité

| Commande | Effet |
|---|---|
| `make test` | Tests unitaires, avec détecteur de compétition. Rapides, sans Docker. |
| `make test-integration` | Tests d'intégration : PostgreSQL et MinIO réels via testcontainers. Demande Docker. |
| `make cover` | Rapport de couverture HTML. |
| `make lint` | `go vet` puis `golangci-lint`. |
| `make fmt` | Formate le code Go. |
| `make tidy` | Nettoie `go.mod`. |

### Construction

| Commande | Effet |
|---|---|
| `make build` | `bin/boxincloud` et `bin/boxincloudctl`. |
| `make build-web` | Compile l'interface dans le répertoire embarqué du serveur. |
| `make build-apk` | Compile l'APK Android dans le répertoire embarqué. |
| `make build-all` | `build-web` puis `build`. Le binaire sert alors l'interface. |
| `make docker` | Image Docker complète, APK compris. |
| `make docker-server` | Image serveur **sans** la chaîne Android. |
| `make clean` | Supprime les artefacts de build. |

`make docker-server` — ou `make docker WITH_MOBILE=0` — n'est pas un raccourci de
confort. Le NDK Android ne publie ses binaires hôtes qu'en x86-64 : sur une
machine **arm64**, un Mac Apple Silicon ou un serveur Ampere, l'étape mobile
échoue à l'émulation et l'image entière devient inconstructible. Ces
machines-là doivent utiliser cette cible.

---

## `boxincloudctl` — administration

Toujours via `make ctl ARGS="…"` en développement, pour que `.env` soit chargé.
En production, le binaire est dans l'image : `docker compose exec boxincloud
boxincloudctl …`.

### Diagnostic

| Commande | Effet |
|---|---|
| `ping-db` | Vérifie la connexion à PostgreSQL. |
| `ping-job [message]` | Enfile un job de test et vérifie que la file tourne. |
| `version` | Version, commit, version de Go. |

### Comptes

| Commande | Effet |
|---|---|
| `user list` | Comptes, rôles, dernière connexion. |
| `user set-password <compte>` | Change un mot de passe et révoque les sessions. |

C'est la **porte de secours** d'une instance auto-hébergée. Sans courriel de
récupération et avec un seul administrateur créé à l'installation, un mot de
passe perdu enfermerait dehors définitivement.

```bash
make ctl ARGS="user set-password niando"
```

Le mot de passe est demandé sur l'entrée standard — jamais en argument, qui
finirait dans l'historique du shell et dans la sortie de `ps`. Douze caractères
minimum, aucune autre contrainte. La saisie n'est pas masquée.

Un mot de passe **ne se retrouve pas** : il est haché en argon2id, un sens
unique. On en écrit un nouveau par-dessus.

### Stockage

| Commande | Effet |
|---|---|
| `storage add <nom> <type> [clé=valeur …]` | Enregistre un backend après l'avoir testé. |
| `storage list` | Liste les backends. |
| `storage test <nom>` | Vérifie qu'un backend répond. |

Types et clés :

```
s3     endpoint= bucket= access_key= secret_key=
       [region=] [use_ssl=false] [path_style=true]
local  root=
```

Le backend est **joint avant d'être enregistré** : une adresse ou une clé fautive
se signale à la saisie, pas au premier scan.

### Bibliothèques

| Commande | Effet |
|---|---|
| `library add <nom> <backend> [préfixe]` | Crée une bibliothèque sur un backend. |
| `library list` | Liste les bibliothèques. |
| `scan <bibliothèque>` | Enfile un scan. **Le serveur doit tourner** pour le traiter. |
| `scan-now <bibliothèque>` | Scanne immédiatement, sans passer par la file. |

`scan-now` sert quand le serveur ne tourne pas, ou pour observer un scan de bout
en bout sans lire les journaux d'un worker.

### Lecture

| Commande | Effet |
|---|---|
| `page <bibliothèque> <clé> <n> [fichier]` | Extrait la page `n` d'une archive et **compte les requêtes Range**. |

C'est l'outil de mesure du cœur du projet. Il montre ce qu'une page coûte
réellement sur du stockage objet — une requête Range, un pourcentage de
l'archive — plutôt que de le supposer.

### eD2k / Kad

| Commande | Effet |
|---|---|
| `ed2k ping` | Joint le démon aMule déclaré, s'authentifie, et mesure l'aller-retour. |

```
$ make ctl ARGS="ed2k ping"
démon        127.0.0.1:4712
version      3.0.1
protocole    0x0204
poignée      2ms
aller-retour 288µs
```

C'est l'outil de diagnostic du module. Quand l'interface affiche « démon
injoignable », quatre choses peuvent avoir lâché — l'adresse, le mot de passe,
la version du protocole, le réseau — et cette commande dit **laquelle**, depuis
le serveur, là où la connexion part réellement.

Deux pièges qu'elle nomme explicitement plutôt que de les laisser deviner :

- **`ECPassword` dans `amule.conf` n'est pas le mot de passe en clair**, c'est
  son empreinte MD5. Ce qui se déclare dans boxincloud est le mot de passe en
  clair.
- **La version du protocole se compare à l'identique.** amuled n'a pas de
  compatibilité ascendante sur External Connections : une version différente ne
  se contourne pas, elle se met à jour des deux côtés.

Le module doit être actif (`BOXINCLOUD_ED2K_ENABLED=true`) et un démon déclaré
depuis l'interface, page **eD2k / Kad**.

---

## `npm` — interface web

Depuis `apps/web`. La CI les exécute tous.

| Commande | Effet |
|---|---|
| `npm run dev` | Serveur de développement sur `:3000`. |
| `npm run build` | Export statique dans `out/`. |
| `npm run typecheck` | TypeScript, sans émettre. |
| `npm run lint` | ESLint. |
| `npm run check:i18n` | Vérifie que les deux catalogues de langue coïncident. |
| `npm run check:css` | Vérifie la feuille de style. |
| `npm run check:contrast` | Contrastes d'accessibilité. |
| `npm run check:overlays` | Cohérence des calques et de leurs empilements. |
| `npm run check:qr` | Le code QR d'installation mobile. |
| `npm run generate:api` | Régénère `schema.d.ts` depuis le contrat. |
| `npm run generate:tokens` | Régénère les variables CSS. |

---

## Flutter — application mobile

Depuis `apps/mobile`.

| Commande | Effet |
|---|---|
| `flutter pub get` | Résout les dépendances. |
| `flutter analyze` | Analyse statique. |
| `flutter test` | Tests unitaires et de widgets. |
| `flutter run` | Lance sur un appareil connecté. |
| `flutter build apk --release` | APK de production. |
| `dart run build_runner build` | Régénère le code Drift et les sources dérivées. |

---

## Docker — production

Depuis `deploy/compose`.

| Commande | Effet |
|---|---|
| `docker compose up -d` | Démarre l'instance, PostgreSQL et MinIO compris. |
| `docker compose logs -f boxincloud` | Suit les journaux du serveur. |
| `docker compose exec boxincloud boxincloudctl user list` | Administration dans le conteneur. |
| `docker compose down` | Arrête, en gardant les données. |
| `docker compose down -v` | **Détruit les volumes.** Irréversible. |

---

## Quand ça ne démarre pas

**`air: command not found`** — `make deps`, ou utilisez `make run`, qui n'en a
pas besoin.

**`DATABASE_URL est obligatoire`** — le binaire a été lancé directement. Passez
par `make run` ou `make ctl`, qui chargent `.env`.

**`le port 8080 est déjà occupé`** — une autre instance tourne. Le message donne
la commande `lsof` pour l'identifier, et `BOXINCLOUD_ADDR=:8081` pour écouter
ailleurs.

**`répertoire de gabarits introuvable`** — l'avertissement affiche le chemin
**résolu**. Un chemin relatif se résout contre le répertoire de lancement :
préférez un chemin absolu, sans guillemets.

**L'interface s'affiche en anglais** — elle suit la langue du navigateur, le
français n'étant que le repli. Le sélecteur est dans le menu du compte, et le
choix est mémorisé.
