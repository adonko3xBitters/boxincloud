# Feuille de route jusqu'à la v0.1.0 publique

Huit jalons. Chacun se termine par un livrable **démontrable** — pas par « la couche X est écrite ». C'est ce qui permet de corriger le tir tôt et, à partir de M2, de montrer le projet.

Les durées supposent un développeur principal à temps partiel soutenu. Elles sont indicatives : l'ordre et les critères de sortie comptent davantage.

---

## M0 — Fondations *(~1 semaine)*

**But :** un dépôt dans lequel on peut contribuer, et un binaire qui démarre.

- Monorepo initialisé, licence AGPL-3.0, `README` avec le pitch et le positionnement.
- Serveur Go : `cmd/boxincloud`, configuration, `slog`, `/healthz`.
- PostgreSQL + goose + sqlc + River câblés, migration initiale vide.
- `openapi.yaml` minimal + `make generate` fonctionnel de bout en bout.
- `docker-compose.dev.yml`, `Makefile`, CI (lint + tests + build).
- `CONTRIBUTING.md`, templates d'issues et de PR, `FUNDING.yml` avec Buy Me a Coffee.

**Sortie :** `make dev` démarre ; la CI est verte ; un contributeur peut cloner et lancer en moins de 5 minutes.

---

## M1 — Stockage et indexation *(~2 semaines)*

**But :** le cœur cloud-native. C'est le jalon qui définit le projet — il vient avant toute interface.

- `storage.Provider` + implémentations `s3` (minio-go) et `local`.
- `storage_backends` et `libraries` en base, secrets chiffrés, test de connexion.
- Module `archive` : **index ZIP par requêtes Range** — l'élément technique le plus critique du produit.
- Jobs River : `ScanLibrary` → `IngestComic` → `BuildPageIndex` → `ExtractMetadata` → `GenerateCover`.
- Parsing `ComicInfo.xml` + analyse du nom de fichier en repli.
- `imaging` avec libvips : vignettes en trois tailles, AVIF/WebP.
- Tests d'intégration testcontainers contre un vrai MinIO.

**Sortie :** `boxincloudctl scan` indexe un bucket de 500 CBZ ; les couvertures sont générées ; une page arbitraire s'extrait en une requête Range vérifiée. **Aucune UI encore.**

---

## M2 — API et lecture *(~2 semaines)*

**But :** l'API complète de lecture, prouvée sans interface.

- Authentification : inscription du premier admin, connexion, JWT + refresh, appareils.
- Endpoints catalogue : bibliothèques, séries, comics, recherche, filtres, tri, pagination par curseur.
- Endpoints lecture : `manifest`, `pages/{n}` avec négociation de format, `prefetch`.
- Cache dérivé + éviction LRU.
- Progression de lecture + endpoints `/sync`.
- OpenAPI complète, clients TS et Dart générés.

**Sortie :** un parcours complet — connexion, navigation, lecture de 20 pages, progression sauvegardée — validé par des tests d'API. La documentation OpenAPI est publiable.

---

## M3 — Web : bibliothèque *(~2 semaines)*

**But :** la première chose que verront les gens. Le soin visuel commence ici.

- Next.js en export statique embarqué dans le binaire Go.
- Design tokens partagés, thèmes clair et sombre.
- Écran de connexion + **assistant de première installation** (créer l'admin, connecter un backend, créer une bibliothèque, lancer le scan) — c'est la première impression, il mérite un vrai travail de finition.
- Grille virtualisée avec LQIP, recherche instantanée, filtres (série, auteur, genre, statut de lecture), tri.
- Page série et page détail d'un ouvrage.
- Étagères d'accueil : « Reprendre la lecture », « Récemment ajouté », « Prochain dans la série ».
- Responsive desktop et tablette.

**Sortie :** une bibliothèque de 2000 titres se parcourt sans à-coups. Premières captures d'écran présentables.

---

## M4 — Web : le lecteur *(~2 semaines)*

**But :** la pièce sur laquelle le projet sera jugé. Ne pas la bâcler.

- Modes page simple, double page (avec détection des doubles planches), défilement continu.
- Zoom au pincement et à la molette, panoramique inertiel, ajustement largeur/hauteur/page.
- Préchargement glissant, libération mémoire hors fenêtre, transitions sans clignotement.
- Clavier complet, gestes tactiles, plein écran, sens de lecture droite-à-gauche (manga).
- Interface qui s'efface, barre de progression, sélecteur de page avec miniatures.
- Sauvegarde de progression avec anti-rebond + persistance à la fermeture d'onglet.
- Réglages persistés par utilisateur.

**Sortie :** lire un album complet est agréable au point qu'on ne pense plus à l'outil. Critère subjectif assumé — c'est le différenciateur du projet.

---

## M5 — Flutter : lecture en ligne *(~2,5 semaines)*

**But :** un client mobile de premier rang, pas un miroir du web.

- Connexion (saisie de l'URL du serveur), stockage sécurisé des jetons, gestion multi-serveurs.
- Schéma Drift, client API généré, chargement de la bibliothèque en cache local.
- Écrans bibliothèque, série, détail — même langage visuel que le web via les tokens partagés.
- Lecteur tactile : `PageView`, zoom, préchargement, plein écran immersif, mode manga.
- Synchronisation de la progression avec file d'opérations hors ligne.

**Sortie :** APK et build iOS installables, lecture fluide sur téléphone et tablette, progression cohérente entre web et mobile.

---

## M6 — Flutter : hors ligne *(~1,5 semaine)*

**But :** ce qui distingue une vraie application mobile d'une page web.

- Téléchargement d'un album ou d'une série entière, en arrière-plan, avec reprise.
- Stockage local (archive CBZ telle quelle, ou pages transcodées), budget disque configurable.
- Éviction automatique (lu d'abord, puis plus ancien), indicateurs d'occupation.
- Lecture 100 % hors ligne, y compris serveur injoignable.
- Réconciliation au retour du réseau, avec la règle « page la plus avancée gagne ».
- Écran de gestion des téléchargements.

**Sortie :** mode avion pendant un trajet complet : navigation, lecture, progression — puis synchronisation correcte à la reconnexion.

---

## M7 — Multi-utilisateur et administration *(~1,5 semaine)*

**But :** l'usage familial, qui est le scénario self-hosted dominant.

- Gestion des comptes : création, rôles, réinitialisation de mot de passe.
- `library_access` : bibliothèques partagées ou restreintes.
- Profils restreints : filtrage par classification d'âge.
- Console d'administration : backends de stockage (ajout, test, statut), bibliothèques, scans en cours et historique, statistiques de cache, purge.
- Édition manuelle des métadonnées avec verrouillage des champs (`locked_fields`).
- Gestion des appareils et révocation de session.

**Sortie :** une famille de quatre personnes utilise l'instance avec des progressions indépendantes et une bibliothèque enfant filtrée.

---

## M8 — Préparation de la release *(~2 semaines)*

**But :** ce qui transforme un projet fonctionnel en projet adopté. À ne pas compresser.

- `Dockerfile` multi-étages, images multi-arch amd64/arm64 sur GHCR.
- `docker-compose.yml` unique, avec MinIO inclus, qui fonctionne sans édition préalable.
- Template Unraid, notes pour TrueNAS et Synology.
- Documentation publique : installation, configuration, migration depuis Komga/Kavita, FAQ, dépannage.
- README travaillé : captures, GIF du lecteur, positionnement explicite (« stockage objet natif », le point qui vous distingue de Komga et Kavita).
- Passe d'accessibilité (navigation clavier, contrastes, lecteurs d'écran), i18n avec français et anglais.
- Passe de sécurité : dépendances, limitation de débit, en-têtes, protection SSRF sur les URL de backend, `SECURITY.md`.
- Tests de charge sur une bibliothèque de 10 000 titres.
- Roadmap publique en GitHub Projects, étiquettes `good first issue` préparées.
- Distribution des builds mobiles : APK en release GitHub, TestFlight, puis dépôts.

**Sortie : v0.1.0 publiée.** Annonce sur r/selfhosted, r/comicbooks, Lemmy selfhosted, awesome-selfhosted.

---

## Récapitulatif

| Jalon | Objet | Durée | Cumul |
|---|---|---|---|
| M0 | Fondations | 1 sem | 1 |
| M1 | Stockage + indexation | 2 sem | 3 |
| M2 | API + lecture | 2 sem | 5 |
| M3 | Web : bibliothèque | 2 sem | 7 |
| M4 | Web : lecteur | 2 sem | 9 |
| M5 | Flutter : en ligne | 2,5 sem | 11,5 |
| M6 | Flutter : hors ligne | 1,5 sem | 13 |
| M7 | Multi-utilisateur + admin | 1,5 sem | 14,5 |
| M8 | Release | 2 sem | **16,5** |

Environ **quatre mois** à temps partiel soutenu.

---

## Après la v0.1.0

Par ordre de valeur décroissante pour l'adoption :

1. **OPDS** — ouvre l'accès à tous les lecteurs tiers existants ; effort faible, gain immédiat en visibilité.
2. **Métadonnées enrichies** — ComicVine, Metron, MangaUpdates, avec écran de rapprochement manuel.
3. **Import et téléversement** — déposer un fichier depuis le web ou le mobile vers un backend.
4. **OIDC** — Authelia, Authentik, Keycloak. Très demandé par le public self-hosted.
5. **Notifications push** — nouveautés dans une série suivie.
6. **Applications de bureau** — Flutter desktop, réutilise M5/M6.
7. **Recommandations** — d'abord par similarité de métadonnées, sans IA.
8. **Plugins et extensions** — n'ouvrir une API d'extension qu'une fois les frontières internes stabilisées par l'usage.
9. **IA : organisation, recherche sémantique, OCR, traduction** — puissant, mais coûteux et clivant sur ce public. À traiter en module optionnel, désactivé par défaut, jamais dans le chemin critique.

---

## Risques identifiés

| Risque | Portée | Réponse |
|---|---|---|
| Accès aléatoire CBR impossible | Élevée | Hydratation au premier accès, décidée dès M1 — pas une découverte tardive |
| Latence sur stockage distant lointain | Élevée | Cache dérivé agressif, préchargement, URL présignées. À mesurer dès M2 |
| Coût de sortie S3 (B2, R2) | Moyenne | Cache local devant le backend, mesures d'usage exposées dans l'admin |
| Périmètre du lecteur qui s'étend sans fin | Élevée | M4 a une liste fermée ; toute demande supplémentaire va en post-v0.1.0 |
| Deux lecteurs à maintenir (web + Flutter) | Moyenne | Tokens partagés, parité fonctionnelle assumée, pas parité de code |
| « Encore un clone de Komga » | **Élevée** | Le positionnement stockage objet doit être en première ligne du README, pas en note de bas de page |
| Contributions qui n'arrivent pas | Moyenne | `good first issue` dès M8, docs d'architecture publiées tôt, réponses rapides sur les issues |
