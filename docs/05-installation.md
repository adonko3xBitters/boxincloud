# Installation

Trois chemins, du plus court au plus contrôlé. Tous mènent au même endroit :
une instance qui démarre, un premier compte à créer, une bibliothèque à
connecter.

---

## Le chemin court : Docker Compose

```sh
curl -O https://raw.githubusercontent.com/adonko3xBitters/boxincloud/main/deploy/compose/docker-compose.yml
docker compose up -d
```

Puis <http://localhost:8080>.

Rien à éditer avant de démarrer, et c'est délibéré. Le fichier embarque
PostgreSQL et MinIO, crée le bucket, applique les migrations et sert
l'interface. L'assistant de première installation prend le relais : créer
l'administrateur, connecter le stockage, créer une bibliothèque, lancer le
scan.

Les identifiants MinIO à saisir dans l'assistant :

| Champ | Valeur |
| --- | --- |
| Endpoint | `minio:9000` |
| Clé d'accès | `boxincloud` |
| Clé secrète | `boxincloud` |
| Bucket | `comics` |
| SSL | non |

### Avant d'exposer sur Internet

Les valeurs par défaut permettent d'essayer en cinq minutes. Elles ne
protègent rien. Trois choses à changer, dans cet ordre d'importance :

**`BOXINCLOUD_SECRET_KEY`.** Elle chiffre les identifiants de vos backends de
stockage en base. Sa valeur par défaut est écrite en clair dans le fichier
compose — donc publique. En produire une vraie :

```sh
openssl rand -hex 32
```

Sa perte rend les backends illisibles : **sauvegardez-la ailleurs que dans la
sauvegarde de la base**, sans quoi les deux disparaîtront ensemble.

**`POSTGRES_PASSWORD` et `MINIO_ROOT_PASSWORD`.** Mêmes raisons, même
publicité.

**Le HTTPS.** boxincloud ne termine pas TLS lui-même : c'est le travail d'un
reverse-proxy, qui le fait mieux et que vous avez probablement déjà. Caddy,
Traefik ou nginx conviennent. Renseignez ensuite `BOXINCLOUD_PUBLIC_URL` avec
l'adresse publique — les liens de partage et le code QR d'installation mobile
en dépendent.

Le tout dans un fichier `.env` à côté du compose :

```sh
BOXINCLOUD_SECRET_KEY=…
POSTGRES_PASSWORD=…
MINIO_ROOT_PASSWORD=…
BOXINCLOUD_PUBLIC_URL=https://bd.exemple.fr
```

---

## Unraid

Le template est dans [`deploy/unraid/boxincloud.xml`](../deploy/unraid/boxincloud.xml).

Unraid ne connaît pas Compose : chaque service y est un conteneur autonome.
Le template décrit donc boxincloud seul et suppose une base PostgreSQL déjà
installée — ce qui est la situation habituelle, le plugin officiel servant
souvent plusieurs applications à la fois.

1. Installer PostgreSQL depuis les Community Applications, créer une base
   `boxincloud`.
2. Ajouter le template : **Docker → Add Container → Template**, coller l'URL
   du fichier XML.
3. Renseigner `DATABASE_URL` et `BOXINCLOUD_SECRET_KEY`.
4. Monter votre share d'albums sur `/comics` si vous préférez le stockage
   local au stockage objet.

---

## TrueNAS SCALE

Pas d'application officielle. Deux voies :

**Docker Compose via une VM ou un jail** — le plus simple, et identique au
chemin court ci-dessus.

**Custom App (Kubernetes)** — dans **Apps → Discover → Custom App** :

| Champ | Valeur |
| --- | --- |
| Image | `ghcr.io/adonko3xbitters/boxincloud` |
| Tag | `latest` |
| Port du conteneur | `8080` |
| Variables | `DATABASE_URL`, `BOXINCLOUD_SECRET_KEY` |
| Volume | hostPath vers vos albums, monté sur `/comics` |
| Volume | hostPath pour le cache, monté sur `/var/lib/boxincloud` |

La base PostgreSQL s'installe depuis le catalogue TrueNAS ; relevez l'adresse
du service dans l'onglet **Application Info** une fois déployée.

---

## Synology

DSM 7 avec Container Manager gère Compose directement.

1. **Container Manager → Projet → Créer**.
2. Chemin : un dossier dans `/docker/boxincloud`.
3. Source : coller le contenu de `deploy/compose/docker-compose.yml`.
4. Créer un fichier `.env` à côté avec au minimum `BOXINCLOUD_SECRET_KEY`.

Sur les modèles ARM récents (DS223, DS224+ et suivants), l'image arm64 est
tirée automatiquement. Les modèles en ARMv7 ne sont pas gérés : la plateforme
n'est plus une cible de compilation Go raisonnable, et prétendre le contraire
donnerait un conteneur qui ne démarre pas.

---

## À partir des sources

Pour contribuer, ou pour ne rien exécuter qu'on n'ait compilé soi-même.

```sh
git clone https://github.com/adonko3xBitters/boxincloud.git
cd boxincloud
make deps          # sqlc, goose, oapi-codegen
cp .env.example .env
# renseigner DATABASE_URL et BOXINCLOUD_SECRET_KEY
docker compose -f docker-compose.dev.yml up -d   # PostgreSQL + MinIO
make dev
```

`make build` produit un binaire unique avec l'interface web embarquée.
`make help` liste le reste.

---

## Migrer depuis Komga ou Kavita

Il n'y a rien à migrer, et c'est une bonne nouvelle.

boxincloud n'impose aucune arborescence : il lit les fichiers là où ils sont,
et déduit séries et numéros de tome des noms de fichiers et des `ComicInfo.xml`
déjà présents dans vos archives — les mêmes que Komga et Kavita utilisent.
Pointer une bibliothèque sur votre répertoire existant suffit.

Ce qui ne se transporte pas :

**La progression de lecture.** Elle vit dans la base de l'application
précédente, dans un schéma qui lui est propre. Un importateur est envisageable ;
il n'existe pas.

**Les métadonnées corrigées à la main dans Komga ou Kavita.** Si elles ont été
écrites dans les `ComicInfo.xml`, elles suivent. Si elles sont restées en base,
non.

Rien n'oblige à choisir tout de suite : les deux applications peuvent lire le
même répertoire en même temps, en lecture seule. C'est même la façon
recommandée d'essayer.
