<div align="center">

# boxincloud

**Votre bibliothèque de BD, comics et mangas — sur votre stockage objet.**

Un serveur de bibliothèque auto-hébergé, moderne et ergonomique, conçu dès le départ
pour **MinIO, S3, Backblaze B2, Cloudflare R2 et Wasabi** — pas pour un disque local
qu'on essaie ensuite de faire ressembler à du cloud.

[![Licence: AGPL v3](https://img.shields.io/badge/licence-AGPL--3.0-blue.svg)](LICENSE)
[![Statut](https://img.shields.io/badge/statut-pré--alpha-orange.svg)](docs/04-roadmap.md)

</div>

---

> **⚠️ Pré-alpha.** Le projet est en cours de construction initiale. Rien n'est utilisable
> aujourd'hui. Voir la [feuille de route](docs/04-roadmap.md) pour l'état d'avancement.

## Pourquoi boxincloud

Il existe d'excellents serveurs de comics auto-hébergés. Ils supposent tous un **système de
fichiers local** : ils scannent un répertoire. Brancher du stockage objet dessus impose un
montage FUSE, avec les performances et la fragilité qui vont avec.

boxincloud prend le problème dans l'autre sens.

- **Stockage objet natif.** Une page se sert par une **requête HTTP Range unique** sur
  l'archive distante. Pas de téléchargement complet, pas de montage, pas de copie locale.
- **Plusieurs backends simultanément.** Une bibliothèque sur votre MinIO, une autre sur
  Backblaze B2, une troisième sur un disque local. Dans la même instance.
- **Une UX qui compte.** Un lecteur fluide et une bibliothèque agréable sont l'objectif,
  pas un habillage posé sur une API.
- **Le mobile comme client de premier rang.** Une application Flutter avec lecture hors
  ligne réelle et synchronisation de la progression — pas une page web emballée.
- **Deux conteneurs.** PostgreSQL et un binaire unique. Pas de Redis, pas de runtime Node.

## Démarrage rapide

> Disponible à la v0.1.0. En attendant, voir [le guide de développement](CONTRIBUTING.md).

```bash
curl -O https://raw.githubusercontent.com/adonko3xBitters/boxincloud/main/docker-compose.yml
docker compose up -d
```

Puis ouvrez `http://localhost:8080` — l'assistant de première installation vous guide
pour créer le compte administrateur, connecter un backend de stockage et lancer un scan.

## Documentation

| Document | Contenu |
|---|---|
| [Architecture](docs/01-architecture.md) | Principes, décisions techniques, modules |
| [Modèle de données](docs/02-data-model.md) | Schéma PostgreSQL et justifications |
| [Structure du dépôt](docs/03-repo-structure.md) | Arborescence et conventions |
| [Feuille de route](docs/04-roadmap.md) | Jalons jusqu'à la v0.1.0 et au-delà |
| [Contribuer](CONTRIBUTING.md) | Installer l'environnement, conventions, workflow |

## Stack

**Serveur** Go · Chi · PostgreSQL · [River](https://riverqueue.com) · sqlc · goose · libvips
**Web** Next.js (export statique, embarqué dans le binaire) · React · TypeScript · Tailwind · shadcn/ui
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

<a href="https://www.buymeacoffee.com/"><img src="https://img.shields.io/badge/Buy%20me%20a%20coffee-☕-FFDD00?style=for-the-badge" alt="Buy me a coffee"></a>

## Licence

[AGPL-3.0](LICENSE) — vous pouvez l'utiliser, le modifier et le redistribuer librement.
Si vous le proposez comme service en réseau, vous devez en publier les sources modifiées.
