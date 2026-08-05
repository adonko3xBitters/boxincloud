<div align="center">

# boxincloud

**Votre bibliothèque de BD, comics et mangas — sur votre stockage objet.**

Un serveur de bibliothèque auto-hébergé, moderne et ergonomique, conçu dès le départ
pour **MinIO, S3, Backblaze B2, Cloudflare R2 et Wasabi** — pas pour un disque local
qu'on essaie ensuite de faire ressembler à du cloud.

[![Licence: AGPL v3](https://img.shields.io/badge/licence-AGPL--3.0-blue.svg)](LICENSE)
[![Statut](https://img.shields.io/badge/statut-alpha-orange.svg)](docs/04-roadmap.md)

</div>

---

> **⚠️ Alpha — v0.1.0.** Le serveur, l'interface web et l'application Android
> fonctionnent de bout en bout : indexation, lecture, comptes, partage, hors ligne,
> OPDS. Attendez-vous à des **changements de schéma** d'une version à l'autre :
> sauvegardez votre base avant de mettre à jour, et n'y confiez pas une bibliothèque
> dont vous n'avez pas de copie. Ce qui change est dans le
> [journal](CHANGELOG.md) ; ce qui vient ensuite, dans la
> [feuille de route](docs/04-roadmap.md).

## Pourquoi boxincloud

Il existe d'excellents serveurs de comics auto-hébergés. Ils supposent tous un **système de
fichiers local** : ils scannent un répertoire. Brancher du stockage objet dessus impose un
montage FUSE, avec les performances et la fragilité qui vont avec.

boxincloud prend le problème dans l'autre sens.

- **Stockage objet natif.** Une page se sert par une **requête HTTP Range unique** sur
  l'archive distante. Pas de téléchargement complet, pas de montage, pas de copie locale.
  Le CBR, le CB7, le PDF et l'EPUB, qui ne permettent pas l'accès aléatoire, sont
  convertis une seule fois à l'indexation — après quoi ils coûtent exactement ce que
  coûte un CBZ.
- **Plusieurs backends simultanément.** Une bibliothèque sur votre MinIO, une autre sur
  Backblaze B2, une troisième sur un disque local. Dans la même instance.
- **Une UX qui compte.** Un lecteur fluide et une bibliothèque agréable sont l'objectif,
  pas un habillage posé sur une API.
- **Le mobile comme client de premier rang.** Une application Flutter avec lecture hors
  ligne réelle et synchronisation de la progression — pas une page web emballée.
- **Deux conteneurs.** PostgreSQL et un binaire unique. Pas de Redis, pas de runtime Node.
- **Vos lecteurs préférés continuent de marcher.** L'instance publie son catalogue en
  OPDS : Panels, Chunky, KyBook ou Thorium s'y branchent directement. Le stockage objet
  est un choix d'infrastructure, il ne vous enferme pas dans nos clients.

## À quoi ça ressemble

<!--
  À COMPLÉTER avant l'annonce publique.

  Quatre visuels, dans cet ordre — c'est celui dans lequel un inconnu se fait une
  opinion, et les trois premières secondes décident du reste :

    1. docs/media/bibliotheque.png — la grille, remplie, sur un thème sombre.
       C'est la capture que les gens comparent à Komga et Kavita.
    2. docs/media/lecteur.gif      — le lecteur en défilement continu, 3 à 5 s.
       C'est le seul visuel qui prouve la fluidité ; une capture fixe ne le peut pas.
    3. docs/media/stockage.png     — l'écran des backends, avec un MinIO et un B2
       côte à côte. C'est LE visuel du positionnement : personne d'autre ne peut
       le montrer.
    4. docs/media/mobile.png       — l'application Android, lecture hors ligne.

  Sans ces images, l'annonce sur r/selfhosted perd l'essentiel de son effet : le
  public y juge sur des captures avant de lire une ligne.
-->

> Captures et démonstration du lecteur à venir avant l'annonce publique.

## Démarrage rapide

```bash
curl -O https://raw.githubusercontent.com/adonko3xBitters/boxincloud/main/deploy/compose/docker-compose.yml
docker compose up -d
```

Puis ouvrez `http://localhost:8080` — l'assistant de première installation vous guide
pour créer le compte administrateur, connecter un backend de stockage et lancer un scan.

Rien à éditer avant de démarrer : PostgreSQL et MinIO sont inclus, le bucket est créé,
les migrations s'appliquent. Les valeurs par défaut conviennent à un essai sur une
machine personnelle et **pas** à une instance exposée — la section
[« Avant d'exposer »](docs/05-installation.md#avant-dexposer-sur-internet) dit lesquelles
changer, et pourquoi.

**Unraid, TrueNAS, Synology, sources** → [guide d'installation](docs/05-installation.md).

### Sur téléphone

Depuis l'interface web : menu du compte → **Application mobile**. Un code QR mène à une
page servie par votre instance, qui propose l'APK Android et rappelle l'adresse du
serveur.

**L'application est servie par votre serveur**, pas par un magasin ni par un tiers :
elle est embarquée dans l'image, donc rien ne sort de votre réseau et sa version
correspond exactement à celle de votre instance. iOS n'existe pas encore.

## Ce qui marche aujourd'hui

| | |
|---|---|
| **Indexation** | CBZ en accès direct · CBR, CB7, PDF et EPUB convertis à l'indexation · `ComicInfo.xml` et analyse du nom de fichier · détection des séries |
| **Lecture web** | page simple, double page, défilement continu · ajustements · sens manga · clavier complet · progression synchronisée |
| **Bibliothèque** | grille virtualisée · recherche insensible aux accents et tolérante aux fautes · dossiers, séries, listes de lecture |
| **Gestion** | téléversement par glisser-déposer · dossiers avec verrouillage par code · liens de partage publics · édition des métadonnées |
| **Comptes** | rôles · bibliothèques restreintes · profils enfants filtrés par classification d'âge · révocation d'appareil |
| **Android** | lecture en ligne et hors ligne · téléchargement d'un album ou d'une série · budget disque · réconciliation au retour du réseau |
| **OPDS** | votre instance **publie** son catalogue en OPDS 1.2 et 2.0 — lisible depuis Panels, Chunky, KyBook, Thorium |
| **eD2k / Kad** | client complet adossé au démon aMule — file, recherche, serveurs, Kad, partagés. Un téléchargement terminé peut devenir un album. **Désactivé par défaut**, voir [le guide](docs/06-ed2k-kad.md) |

Ce qui n'y est pas encore : la recherche dans des catalogues extérieurs, iOS et
les applications de bureau. Voir la
[feuille de route](docs/04-roadmap.md#après-la-v010).

## Documentation

| Document | Contenu |
|---|---|
| [Architecture](docs/01-architecture.md) | Principes, décisions techniques, modules |
| [Modèle de données](docs/02-data-model.md) | Schéma PostgreSQL et justifications |
| [Structure du dépôt](docs/03-repo-structure.md) | Arborescence et conventions |
| [Feuille de route](docs/04-roadmap.md) | Jalons jusqu'à la v0.1.0 et au-delà |
| [Installation](docs/05-installation.md) | Compose, Unraid, TrueNAS, Synology, sources, migration |
| [Module eD2k / Kad](docs/06-ed2k-kad.md) | Client eD2k et Kad adossé au démon aMule, et son pont vers la bibliothèque |
| [Journal des modifications](CHANGELOG.md) | Ce qui change d'une version à l'autre |
| [Contribuer](CONTRIBUTING.md) | Installer l'environnement, conventions, workflow |

## Stack

**Serveur** Go · Chi · PostgreSQL · [River](https://riverqueue.com) · sqlc · goose
**Web** Next.js (export statique, embarqué dans le binaire) · React · TypeScript · Tailwind
**Mobile** Flutter · Riverpod · Drift
**Contrat** OpenAPI 3.1 — clients Go, TypeScript et Dart générés depuis une source unique

Les décisions structurantes et leurs justifications sont dans
[docs/01-architecture.md](docs/01-architecture.md#2-décisions-darchitecture-adr-condensés).

## Contribuer

Les contributions sont bienvenues. Le projet est pensé pour être contribuable : architecture
documentée, conventions explicites, environnement de développement en une commande.

```bash
git clone https://github.com/adonko3xBitters/boxincloud.git
cd boxincloud
make dev
```

Lisez [CONTRIBUTING.md](CONTRIBUTING.md) avant votre première pull request.

## Soutenir le projet

boxincloud est gratuit et le restera. Si le projet vous est utile, vous pouvez offrir
un café — c'est ce qui finance le temps passé dessus.

<a href="https://ko-fi.com/adonko3xbitters"><img src="https://img.shields.io/badge/Ko--fi-offrir%20un%20café-FF5E5B?style=for-the-badge&logo=ko-fi&logoColor=white" alt="Soutenir sur Ko-fi"></a>

## Licence

[AGPL-3.0](LICENSE) — vous pouvez l'utiliser, le modifier et le redistribuer librement.
Si vous le proposez comme service en réseau, vous devez en publier les sources modifiées.
