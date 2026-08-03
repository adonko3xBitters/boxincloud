# Journal des modifications

Le format suit [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/), et le
projet applique le [versionnage sémantique](https://semver.org/lang/fr/).

Ce journal s'adresse à quelqu'un qui **installe** ou **met à jour** une
instance. Il dit ce qui change pour lui, pas ce qui a changé dans le code — pour
cela, l'historique git et les documents d'architecture sont plus précis.

## [Non publié]

### Corrigé

- **L'application Android s'installait sous le logo de Flutter.** Le projet a
  une icône — celle du site — mais elle n'était branchée nulle part côté
  mobile : les cinq PNG livrés par `flutter create` n'avaient jamais été
  remplacés.

  Elles sont désormais produites depuis `apps/web/public/icon.svg`, source
  unique du web et du mobile, avec une **icône adaptative** — fond et motif en
  deux couches, comme Android 8 et suivants l'attendent pour appliquer leurs
  masques sans rogner le dessin.

- **Le numéro de version de l'APK ne dépend plus d'un geste manuel.** Il est
  dérivé du tag git au moment de la construction. Le versionCode Android était
  resté à 1 pendant toute la 0.1.0 : tant qu'il ne croît pas, le système ne
  reconnaît pas une installation comme plus récente que la précédente.

## [0.1.1] — 2026-08-03

Correctifs. **Si vous utilisez l'application Android, cette mise à jour est
nécessaire** : celle de la 0.1.0 ne fonctionne pas du tout.

### Corrigé

- **L'application Android ne pouvait joindre aucun serveur.** L'APK de la
  version 0.1.0 ne déclarait pas la permission `INTERNET` : il s'installait,
  se lançait, affichait l'écran de connexion, puis échouait sur « serveur
  injoignable » quelle que soit l'adresse saisie.

  Le gabarit Flutter ne déclare cette permission que dans les manifestes
  `debug` et `profile`. Une compilation en release n'en hérite pas, et c'est
  la release qui est embarquée dans l'image. La CI construisait l'APK de
  debug — la seule variante où le défaut ne pouvait pas apparaître. Elle
  construit désormais la release et inspecte l'artefact produit.

  Les serveurs en `http://` sont également joignables de nouveau : depuis
  Android 9, le trafic en clair doit être déclaré, et une instance
  auto-hébergée derrière une IP nue ou sur un réseau local n'a souvent pas de
  certificat. Le chiffrement reste recommandé, et l'écran de connexion
  continue de traiter `https` comme la valeur par défaut.

  **Mettez à jour l'application** : l'APK de la 0.1.0 est inutilisable.

- **Le `.env.example` ne documentait pas l'installation par Docker.** Sur les
  douze variables lues par `deploy/compose/docker-compose.yml`, huit
  n'apparaissaient nulle part — dont `MINIO_BUCKET` et `MINIO_ROOT_PASSWORD`,
  c'est-à-dire deux des quatre valeurs à saisir pour connecter le stockage.

  Le `.env.example` de la racine configure un serveur lancé depuis les sources
  et n'a presque aucune variable en commun avec le compose ; rien ne le disait,
  et les deux fichiers portent le même nom. Il existe désormais un
  `deploy/compose/.env.example` aligné sur le compose, variable pour variable,
  et chaque fichier renvoie à l'autre.

- **Le guide d'installation devient un pas-à-pas.** Six étapes numérotées avec
  leurs commandes Docker et leurs vérifications, les valeurs exactes du
  formulaire de stockage, et un tableau de dépannage qui associe un symptôme à
  sa cause. Les deux pièges qui se déclenchent le plus — `localhost` au lieu de
  `minio`, et `BOXINCLOUD_SECRET_KEY` prise pour la clé secrète du stockage —
  y sont nommés à l'endroit où on les rencontre.

- **Un stockage injoignable répondait 500.** Clé fausse, port fermé, `https://`
  sur un service en clair : autant d'erreurs de saisie que le formulaire
  annonçait par « une erreur inattendue est survenue », sans rien dire de plus.
  Le serveur connaissait pourtant la cause et l'écrivait dans ses journaux.

  Ces échecs sont désormais des erreurs de validation, et le diagnostic du
  service distant est joint au message. « The request signature we calculated
  does not match » et « connection refused » ne se corrigent pas de la même
  façon, et l'un des deux se règle en dix secondes quand on le lit.

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
- **Déploiement** — image multi-architecture amd64 et arm64, `docker-compose.yml`
  qui démarre sans édition préalable, modèle Unraid, notes TrueNAS et Synology.
- **Interface en français et en anglais**, avec passe d'accessibilité — clavier,
  contrastes, lecteurs d'écran.

### Sécurité

- **Garde SSRF** sur toute adresse jointe par le serveur — l'adresse d'un backend
  de stockage est saisie depuis l'interface et jointe par le serveur lui-même.
  Les adresses de lien-local, dont les services de métadonnées d'instance des
  fournisseurs de nuage, sont refusées. Le contrôle s'applique aussi après
  redirection.
- Limitation de débit entrante, en-têtes de sécurité, secrets chiffrés au repos.
- Politique de divulgation dans [`SECURITY.md`](SECURITY.md).

### Limites connues

- **iOS n'existe pas** : ni application, ni TestFlight.
- **Les mises à jour de schéma peuvent casser.** Sauvegardez votre base avant de
  changer de version tant que la 1.0 n'est pas là.
- Les valeurs par défaut du `docker-compose.yml` conviennent à un essai sur une
  machine personnelle, **pas** à une instance exposée sur Internet. Voir
  [« Avant d'exposer »](docs/05-installation.md#avant-dexposer-sur-internet).

### Retirée avant publication : la recherche fédérée

Elle a existé et fonctionné : interrogation de catalogues OPDS distants, import
vers un backend de stockage, lecture de sites sans API à partir de gabarits
déclaratifs, enrichissement des métadonnées depuis des bases publiques.

Elle est retirée du projet avant sa première version. Le raccourci
`Cmd/Ctrl + Maj + F` reste et annonce une fonctionnalité à venir : le supprimer
aurait fait croire à une régression silencieuse.

À ne pas confondre avec le **serveur OPDS**, qui reste et figure ci-dessus :
publier son propre catalogue et aller chercher ailleurs sont deux choses
différentes.

[Non publié]: https://github.com/adonko3xBitters/boxincloud/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/adonko3xBitters/boxincloud/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/adonko3xBitters/boxincloud/releases/tag/v0.1.0
