# Journal des modifications

Le format suit [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/), et le
projet applique le [versionnage sémantique](https://semver.org/lang/fr/).

Ce journal s'adresse à quelqu'un qui **installe** ou **met à jour** une
instance. Il dit ce qui change pour lui, pas ce qui a changé dans le code — pour
cela, l'historique git et les documents d'architecture sont plus précis.

## [Non publié]

## [0.1.0] — 2026-07-31

Première version publique. Le serveur, l'interface web et l'application Android
fonctionnent de bout en bout.

**Considérez cette version comme une alpha.** Le schéma de base de données
bougera encore, et une mise à jour pourra demander une intervention. N'y confiez
pas une bibliothèque dont vous n'avez pas de copie.

### Ce qui distingue boxincloud

**Le stockage objet est natif, pas adapté.** Une page se sert par une **requête
HTTP Range unique** sur l'archive distante : ni téléchargement complet, ni
montage FUSE, ni copie locale. Mesuré sur MinIO, servir une page d'une archive
de 2,2 Mo coûte **1,61 %** de l'archive, et indexer un album de 62 pages en
coûte **0,42 %**.

Les formats qui ne permettent pas l'accès aléatoire — CBR, CB7, PDF, EPUB — sont
convertis **une seule fois** à l'indexation. Après quoi ils coûtent exactement
ce que coûte un CBZ.

**Plusieurs backends dans la même instance.** Une bibliothèque sur MinIO, une
autre sur Backblaze B2, une troisième sur un disque local.

### Ajouté

- **Stockage** — MinIO, S3, Backblaze B2, Cloudflare R2, Wasabi et disque local.
  Identifiants chiffrés en AES-256-GCM, essai de connexion à la saisie.
- **Indexation** — `ComicInfo.xml` puis analyse du nom de fichier en repli,
  détection des séries, vignettes en WebP et AVIF.
- **Lecture web** — page simple, double page, défilement continu, sens manga,
  zoom au pincement, clavier complet, progression synchronisée.
- **Bibliothèque** — grille virtualisée, recherche insensible aux accents et
  tolérante aux fautes, dossiers, séries, listes de lecture.
- **Gestion** — téléversement par glisser-déposer, dossiers verrouillés par
  code, liens de partage publics, édition des métadonnées.
- **Comptes** — rôles, bibliothèques restreintes, profils enfants filtrés par
  classification d'âge, révocation d'appareil.
- **Android** — lecture en ligne et hors ligne, téléchargement d'un album ou
  d'une série, budget disque, réconciliation au retour du réseau.
  L'APK est **servi par votre instance**, pas par un magasin : il est embarqué
  dans l'image, donc rien ne sort de votre réseau et sa version correspond
  exactement à celle du serveur.
- **OPDS** — votre instance **publie** son catalogue en OPDS 1.2 et 2.0. Panels,
  Chunky, KyBook et Thorium s'y branchent directement.
- **Découvrir** — recherche fédérée sur des catalogues OPDS tiers, import direct
  vers un backend de stockage, enrichissement des métadonnées depuis Open
  Library, Internet Archive et Google Books.
- **Moteur de gabarits** — les sites du domaine public sans API ni flux OPDS se
  lisent à partir de gabarits YAML déclaratifs : sélecteurs CSS, miroirs de
  repli, débit sortant, `robots.txt` respecté.
  **Aucun gabarit n'est livré dans cette version** — les deux sites visés ne sont
  pas exploitables aujourd'hui, c'est expliqué dans
  [`docs/06-gabarits-scraper.md`](docs/06-gabarits-scraper.md). Le moteur sert
  aux gabarits que vous déclarez vous-même via
  `BOXINCLOUD_SCRAPER_TEMPLATES_DIR`.
- **Déploiement** — image multi-architecture amd64 et arm64, `docker-compose.yml`
  qui démarre sans édition préalable, modèle Unraid, notes TrueNAS et Synology.
- **Interface en français et en anglais**, avec passe d'accessibilité — clavier,
  contrastes, lecteurs d'écran.

### Sécurité

- **Garde SSRF** sur toute adresse jointe par le serveur — backends de stockage
  et catalogues fédérés. Les adresses de lien-local, dont les services de
  métadonnées d'instance des fournisseurs de nuage, sont refusées. Le contrôle
  s'applique aussi après redirection.
- **Import confiné au catalogue déclaré** : une adresse téléchargée doit
  appartenir à une source enregistrée par un administrateur, sans quoi la route
  serait un relais anonyme doublé d'un sondeur de réseau interne.
- Limitation de débit entrante, en-têtes de sécurité, secrets chiffrés au repos.
- Politique de divulgation dans [`SECURITY.md`](SECURITY.md).

### Limites connues

- **iOS n'existe pas** : ni application, ni TestFlight.
- **Les mises à jour de schéma peuvent casser.** Sauvegardez votre base avant de
  changer de version tant que la 1.0 n'est pas là.
- Les valeurs par défaut du `docker-compose.yml` conviennent à un essai sur une
  machine personnelle, **pas** à une instance exposée sur Internet. Voir
  [« Avant d'exposer »](docs/05-installation.md#avant-dexposer-sur-internet).
- Le moteur de gabarits est livré **sans gabarit** (voir plus haut).

### Périmètre des sources de la fonction « Découvrir »

Le registre livré est une **liste fermée, embarquée dans le binaire**. Le critère
d'admission est explicite : la diffusion des œuvres doit être autorisée —
domaine public, licence libre, autorisation de l'auteur, ou accès fourni par
l'utilisateur avec ses propres identifiants.

Deux portes restent ouvertes, et toutes deux s'ouvrent par un geste explicite
d'administration plutôt que par un défaut du produit : la fédération OPDS, où
vous désignez un catalogue dont vous avez les clés, et le répertoire de gabarits
d'opérateur, désactivé par défaut, où vous décrivez un site dont vous répondez.

[Non publié]: https://github.com/adonko3xBitters/boxincloud/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/adonko3xBitters/boxincloud/releases/tag/v0.1.0
