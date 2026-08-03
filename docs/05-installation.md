# Installation

Trois chemins, du plus court au plus contrôlé. Tous mènent au même endroit :
une instance qui démarre, un premier compte à créer, une bibliothèque à
connecter.

---

## Le chemin court : Docker Compose

Rien à éditer avant de démarrer, et c'est délibéré. Le fichier embarque
PostgreSQL et MinIO, crée le bucket, applique les migrations et sert
l'interface.

Comptez cinq minutes, dont quatre d'attente de téléchargement.

### 1. Récupérer le fichier compose

```sh
mkdir boxincloud && cd boxincloud
curl -O https://raw.githubusercontent.com/adonko3xBitters/boxincloud/main/deploy/compose/docker-compose.yml
```

Un répertoire dédié : `docker compose` nomme le projet d'après lui, et vos
volumes en dépendent.

Optionnellement, le fichier d'exemple qui documente toutes les variables — le
compose démarre sans, il ne sert qu'à savoir ce qui est réglable :

```sh
curl -O https://raw.githubusercontent.com/adonko3xBitters/boxincloud/main/deploy/compose/.env.example
```

### 2. Démarrer

```sh
docker compose up -d
```

Quatre conteneurs se lancent. Le troisième, `minio-init`, crée le bucket puis
s'arrête — **son état `Exited (0)` est le résultat attendu**, pas une panne.

### 3. Vérifier avant d'ouvrir le navigateur

```sh
docker compose ps
```

Attendu : `boxincloud`, `postgres` et `minio` en `Up`, ce dernier `(healthy)`.
Si `boxincloud` redémarre en boucle, la cause est dans ses journaux :

```sh
docker compose logs boxincloud
```

Puis, quand le serveur répond :

```sh
curl http://localhost:8080/readyz
```

`/readyz` interroge la base ; `/healthz`, à côté, ne prouve que la présence du
processus. C'est le premier des deux qui répond à « puis-je m'en servir ».

### 4. Créer le compte administrateur

Ouvrez <http://localhost:8080>. L'assistant demande un identifiant et un mot de
passe : ce premier compte est administrateur, les suivants ne le seront pas.

Mot de passe oublié, plus tard :

```sh
docker compose exec boxincloud boxincloudctl user set-password votre-compte
```

### 5. Connecter le stockage

L'étape qui se rate le plus souvent, parce que les quatre valeurs sont des
défauts et non des évidences. Recopiez-les à l'identique :

| Champ | Valeur | Remarque |
| --- | --- | --- |
| Type | `S3 / MinIO` | |
| Endpoint | `http://minio:9000` | nom du service, **pas** `localhost` |
| Clé d'accès | `boxincloud` | |
| Clé secrète | `boxincloud` | **pas** `BOXINCLOUD_SECRET_KEY` |
| Bucket | `comics` | et non `boxincloud` |
| SSL | non | |

Deux pièges, et ce sont exactement ceux qui se déclenchent :

**`localhost` ne désigne pas votre machine ici.** C'est le serveur qui joint
MinIO, depuis l'intérieur du réseau du compose, où `localhost` est son propre
conteneur. `minio` est le nom du service, et c'est ce nom qu'il faut. Il reste
`http://minio:9000` même si vous avez changé `MINIO_PORT` : cette variable ne
déplace que le port publié sur l'hôte.

**La clé secrète du stockage n'est pas `BOXINCLOUD_SECRET_KEY`.** Cette
dernière chiffre vos identifiants en base ; elle n'ouvre pas MinIO. Coller les
64 caractères hexadécimaux à la place de `boxincloud` donne une erreur de
signature qui ne ressemble pas à une faute de frappe, alors que c'en est une.

Si vous avez renseigné un `.env`, les valeurs à saisir sont vos
`MINIO_ROOT_USER`, `MINIO_ROOT_PASSWORD` et `MINIO_BUCKET`.

### 6. Créer une bibliothèque et lancer le scan

Choisissez le backend qui vient d'être connecté, donnez un préfixe (`/` pour la
racine du bucket), puis démarrez le scan.

Le bucket est vide au premier démarrage — c'est normal, et l'interface le dira.
Déposez des fichiers par glisser-déposer depuis l'interface, ou versez-les
directement dans MinIO via sa console sur <http://localhost:9001>, mêmes
identifiants. Relancez le scan ensuite.

### Quand ça ne marche pas

Le serveur écrit la cause exacte dans ses journaux, y compris ce que le service
distant a répondu mot pour mot. C'est presque toujours la réponse la plus
rapide :

```sh
docker compose logs -f boxincloud
```

| Symptôme | Cause la plus fréquente |
| --- | --- |
| `bind: address already in use` | Le port 8080 est pris. Mettez `BOXINCLOUD_PORT=8081` dans `.env`, puis `docker compose up -d`. |
| Le stockage refuse l'enregistrement | Les valeurs de l'étape 5. Les journaux nomment laquelle. |
| `signature does not match` | Clé d'accès ou clé secrète. Voir les deux pièges ci-dessus. |
| `bucket does not exist` | Bucket saisi ≠ `MINIO_BUCKET`. Le recréer : `docker compose up minio-init`. |
| `connection refused` sur l'endpoint | `localhost` au lieu de `minio`. |
| `boxincloud` redémarre en boucle | Configuration refusée au démarrage : les journaux nomment la variable. |

Repartir de zéro, volumes compris — **cela efface la base et le contenu du
bucket** :

```sh
docker compose down -v && docker compose up -d
```

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

---

## Formats

| Format | Traitement |
| --- | --- |
| **CBZ / ZIP** | Lu directement. Une page = une requête Range sur l'archive distante. |
| **CBR / RAR** | Converti en CBZ à l'indexation, dans le cache. Ensuite identique au CBZ. |
| **CB7 / 7z** | Idem. |
| **PDF** | Idem, par extraction des images de chaque page. |
| **EPUB** | Idem, **dans l'ordre du *spine*** — voir ci-dessous. |

### Pourquoi une conversion

Le RAR ne permettra jamais de servir une page sans lire ce qui la précède : ses
archives « solides » compressent les fichiers comme un flux continu. Et un PDF
demanderait un moteur de rendu écrit en C, incompatible avec le binaire statique
que le projet livre.

Plutôt que d'exclure ces formats ou de prétendre les supporter, boxincloud les
lit **une fois**, à l'indexation, et en dépose une version CBZ dans son cache.
Toute lecture ultérieure suit exactement le chemin d'un CBZ.

**Vos fichiers ne sont jamais touchés.** La version convertie vit dans le cache
dérivé — le seul emplacement dont boxincloud est propriétaire. L'original n'est
ni modifié, ni déplacé, ni dupliqué chez vous.

### Ce que cela coûte

Le cache grossit d'un volume proche de celui des CBR et PDF indexés. Prévoyez-le
si votre bibliothèque en compte beaucoup : `BOXINCLOUD_CACHE_MAX_SIZE` borne les
vignettes et les pages transcodées, **pas** les archives converties — les
évincer rendrait des albums illisibles jusqu'à une réindexation.

Supprimer un album emporte sa version convertie.

### L'EPUB et son piège

Un EPUB **est** un ZIP, et on pourrait croire qu'il relève du chemin direct du
CBZ. C'est faux, et le piège mérite d'être nommé : **l'ordre de lecture d'un
EPUB n'est pas celui de ses noms de fichiers.** Il est défini par le `spine` du
document OPF, et rien n'oblige un éditeur à nommer ses images dans cet ordre —
beaucoup ne le font pas.

L'indexer comme un CBZ donnerait donc un album complet, lisible, et dans le
désordre. C'est la pire des pannes : elle ne ressemble pas à une panne.
L'hydratation suit le spine et renomme les images dans l'ordre qu'il prescrit.

### Deux limites nommées

Un PDF de bande dessinée **native** — du texte et des vecteurs, sans image de
fond — ne donne rien à extraire, et l'album est marqué en erreur. Le cas existe
chez les éditions numériques natives. Le traiter demanderait un moteur de rendu
écrit en C, qu'on a écarté ; le prétendre serait pire que de le refuser.

Un EPUB de **texte** — un roman — se solde de même par un refus explicite. Ce
lecteur sert les bandes dessinées, pas les livres.
