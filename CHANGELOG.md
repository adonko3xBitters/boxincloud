# Journal des modifications

Le format suit [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/), et le
projet applique le [versionnage sémantique](https://semver.org/lang/fr/).

Ce journal s'adresse à quelqu'un qui **installe** ou **met à jour** une
instance. Il dit ce qui change pour lui, pas ce qui a changé dans le code — pour
cela, l'historique git et les documents d'architecture sont plus précis.

## [Non publié]

## [0.1.7] — 2026-08-06

Le module eD2k à l'usage : des tableaux qu'on peut fouiller, une page qui
occupe l'écran, et des commandes dont on voit l'effet.

### Ajouté

- **Filtre et pagination sur tous les tableaux du module.** Une liste de
  serveurs importée fait couramment trois cents entrées ; tout afficher donnait
  une page qui défile sur des mètres, où retrouver une ligne demandait le
  Cmd-F du navigateur — qui ne cherche que ce qui est déjà rendu. Chaque
  tableau cherche ce qui a un sens chez lui : nom et adresse pour un serveur,
  nom de fichier ailleurs.

  Pas de tri par colonne, et c'est délibéré : l'ordre vient du démon et porte
  du sens — la file est dans l'ordre où elle sera servie, les sources dans
  l'ordre où elles ont répondu.

### Corrigé

- **« Se connecter » sur un serveur semblait ne rien faire.** Le démon accuse
  réception avant d'agir : la commande se répond en une milliseconde et met
  plusieurs secondes à aboutir. L'écran ne bougeait pas pendant dix secondes,
  ce qui se lit comme un bouton mort.

- **Le journal faisait défiler la page entière.** Pour lire une ligne du haut,
  il fallait remonter au-dessus de l'en-tête. La carte défile désormais chez
  elle.

### Modifié

- **La page du module occupe la largeur de l'écran.** Ses tableaux ont huit à
  dix colonnes ; les borner laissait deux marges vides et rognait la colonne
  des actions.

- **Les formulaires du module sont plus lisibles.** L'explication remonte en
  tête du formulaire au lieu d'être répétée sous chaque champ — c'est ce qui
  désalignait les champs entre eux. Les champs de saisie de toute l'interface
  gagnent un halo au moment où on les active : une bordure qui change de teinte
  se remarque mal sur quatre champs alignés.

## [0.1.6] — 2026-08-05

La gestion des serveurs eD2k, qui manquait, et trois corrections dont une qui
faisait agir le démon sur le mauvais serveur.

### Ajouté

- **Import d'une liste de serveurs, ajout et retrait à la main.** C'est le
  premier geste sur une instance neuve : sans serveurs, rien ne fonctionne — ni
  connexion, ni recherche, ni source. Écran **Serveurs** → **Importer une
  liste**, avec l'adresse d'un `server.met` publié. C'est le démon qui va la
  chercher, pas votre navigateur.

- **eD2k / Kad devient un onglet**, en haut de page, à côté de
  « Bibliothèque ». Il vivait dans le menu du compte, derrière un avatar. C'est
  un endroit où l'on passe du temps — on cherche, on surveille une file, on
  revient voir où en est un téléchargement — et cela ne se range pas derrière
  deux clics et une icône qui ne l'annonce pas. Réservé aux administrateurs,
  comme avant.

### Corrigé

- **« Se connecter à ce serveur-ci » joignait un autre serveur.** La commande
  désignait sa cible par une chaîne « adresse:port » là où le protocole attend
  six octets. Le démon acceptait sans broncher, ne trouvait aucun serveur à
  cette désignation, et se rabattait sur son comportement par défaut : se
  connecter à n'importe lequel. Rien ne le signalait. Le retrait d'un serveur
  souffrait du même défaut, et le nom d'un serveur ajouté était perdu.

- **Une commande réussie s'affichait en rouge.** Les commandes du module
  répondent « transmis au démon » sans corps ; l'interface tentait de lire ce
  corps vide comme du JSON et signalait une erreur.

- **Le module annonçait qu'il n'était pas implémenté** alors qu'il
  fonctionnait. Un texte d'attente était resté dans l'état du module, au-dessus
  d'une recherche qui marchait.

- **Les messages d'erreur du module passent en français.** Une exception
  assumée : quand le démon aMule refuse une commande, ses mots sont conservés
  tels quels — en anglais, encadrés et attribués. « Kad is disabled in
  preferences » dit exactement quoi faire ; une phrase de notre cru ne dirait
  rien.

### Modifié

- **Le pêle-mêle est la vue d'ouverture de la bibliothèque**, à la place de la
  grille. Il montre une couverture en grand, ce qui l'entoure, et la liste
  complète en dessous — là où la grille seule donne des vignettes, mais ni les
  pages, ni la taille, ni la série.

## [0.1.5] — 2026-08-05

> Cette entrée est écrite après coup : la version a été publiée sans passer par
> ce journal.

### Ajouté

- **Le module eD2k / Kad.** Un client eD2k et Kad accessible depuis le
  navigateur, adossé au démon aMule — boxincloud ne réimplémente aucun
  protocole pair-à-pair, il pilote `amuled` par son protocole officiel
  *External Connections* et remplace `amuleweb`.

  Téléchargements et leurs sources, recherche sur serveur, globale ou Kad,
  envois et file d'attente des pairs, fichiers partagés, serveurs,
  statistiques, journal du démon — le tout en temps réel par un flux
  Server-Sent Events.

  Et un pont vers la bibliothèque : une règle par catégorie du démon décide si
  un téléchargement terminé devient un album indexé, lisible depuis le
  navigateur et l'application Android, ou reste sur disque.

  **Désactivé par défaut.** Il s'active en deux gestes —
  `docker compose --profile ed2k up -d` pour ajouter le démon, et
  `BOXINCLOUD_ED2K_ENABLED=true` pour que boxincloud le pilote. La promesse
  « deux conteneurs » ne vaut plus quand il est actif ; elle reste vraie pour
  qui n'active rien. Voir [`docs/06-ed2k-kad.md`](docs/06-ed2k-kad.md).

## [0.1.4] — 2026-08-03

Deux modes de lecture au lieu d'un, sur mobile. Rien ne change côté serveur ;
mettre à jour l'image sert à obtenir le nouvel APK.

### Ajouté

- **Défilement continu dans le lecteur mobile.** Les planches s'enchaînent sans
  coupure, à la largeur de l'écran. C'est le mode des webtoons, dont les
  planches sont des bandes verticales conçues pour se suivre : les tourner une
  par une coupe le récit au milieu d'une case. Le lecteur web l'avait déjà.

- **Une marge de lecture réglable.** Sur un grand écran tenu à une main, une
  planche à fond perdu pousse le regard jusqu'aux bords, là où sont la paume et
  la courbure de la dalle. Quatre niveaux, de rien à 16 % de chaque côté.

  La marge ne s'applique qu'à l'image : les zones de toucher gardent toute la
  largeur, sans quoi le bord qui tourne la page s'éloignerait du pouce — soit
  l'inverse du but.

- **Un panneau de réglages** réunit le mode, le sens de lecture et la marge,
  comme sur le web. Le sens de lecture, qui était une bascule dans la barre, s'y
  range : il se choisit une fois, alors que les deux autres s'essaient.

### Corrigé

- **Les panneaux du lecteur recouvraient les boutons qui les ouvrent.** Passer
  des vignettes aux réglages demandait de fermer, viser la barre réapparue, et
  rouvrir ; le voyant d'activité de ces boutons n'était jamais visible. La barre
  est désormais au-dessus du panneau ouvert.

## [0.1.3] — 2026-08-03

Le lecteur mobile. Rien ne change côté serveur ; mettre à jour l'image sert à
obtenir le nouvel APK.

### Corrigé

- **Agrandir une planche empêchait de l'explorer.** Une fois zoomé, tirer pour
  voir le reste de la page tournait la page. Les deux gestes sont le même — un
  glissement horizontal — et le défilement des pages l'emportait toujours.

  Le déplacement dans l'image l'emporte désormais tant qu'on est agrandi ;
  tourner la page redevient possible en revenant à l'échelle 1, d'un double tap
  ou d'un pincement.

### Ajouté

- **Les pages en vignettes, dans le lecteur mobile.** Le curseur dit où l'on
  est, il ne dit pas ce qu'il y a : retrouver une planche précise dans un album
  qu'on relit demande de la voir. Le lecteur web l'avait déjà ; le mobile en
  était privé.

- **Les bords de l'écran tournent la page.** Ils ne faisaient rien, et le seul
  moyen d'avancer était de balayer — un geste qui demande le pouce au milieu de
  l'écran et se tient mal à une main. En lecture manga, c'est le bord gauche qui
  avance, comme le sens de lecture.

### Modifié

- **Tirer le curseur de progression n'inonde plus le serveur.** Chaque cran
  écrivait en base et appelait le serveur : parcourir un album de deux cents
  planches déclenchait deux cents requêtes pour une seule intention. Seule la
  position d'arrivée est envoyée.

## [0.1.2] — 2026-08-03

Deux correctifs sur l'empaquetage de l'application Android. Rien ne change côté
serveur ; mettre à jour l'image sert à obtenir le nouvel APK.

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

[Non publié]: https://github.com/adonko3xBitters/boxincloud/compare/v0.1.7...HEAD
[0.1.7]: https://github.com/adonko3xBitters/boxincloud/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/adonko3xBitters/boxincloud/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/adonko3xBitters/boxincloud/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/adonko3xBitters/boxincloud/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/adonko3xBitters/boxincloud/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/adonko3xBitters/boxincloud/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/adonko3xBitters/boxincloud/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/adonko3xBitters/boxincloud/releases/tag/v0.1.0
