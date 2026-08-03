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

## M1 — Stockage et indexation ✅ *(~2 semaines)*

**But :** le cœur cloud-native. C'est le jalon qui définit le projet — il vient avant toute interface.

- `storage.Provider` + implémentations `s3` (minio-go) et `local`.
- `storage_backends` et `libraries` en base, secrets chiffrés, test de connexion.
- Module `archive` : **index ZIP par requêtes Range** — l'élément technique le plus critique du produit.
- Jobs River : `ScanLibrary` → `IngestComic` → `BuildPageIndex` → `ExtractMetadata` → `GenerateCover`.
- Parsing `ComicInfo.xml` + analyse du nom de fichier en repli.
- `imaging` avec libvips : vignettes en trois tailles, AVIF/WebP.
- Tests d'intégration testcontainers contre un vrai MinIO.

**Sortie :** `boxincloudctl scan` indexe un bucket de 500 CBZ ; les couvertures sont générées ; une page arbitraire s'extrait en une requête Range vérifiée. **Aucune UI encore.**

**Atteint.** Mesures relevées sur MinIO, 5 albums et 274 pages :

| Opération | Coût |
|---|---|
| Servir une page d'une archive de 2,2 Mo | **1 requête Range, 1,61 %** de l'archive |
| Indexer un album de 62 pages | 65 requêtes, **0,42 %** de l'archive |
| Scan complet, 5 albums / 274 pages | 1,57 s |
| Rescan sans changement | 22 ms, aucun doublon |

Écart rattrapé depuis : le moteur d'imagerie produit les quatre formats, et
toujours sans cgo — les encodeurs WebP et AVIF passent par WebAssembly plutôt
que par libvips. Voir l'architecture pour la mesure qui décide lequel est servi
où : l'AVIF gagne sur les vignettes et perd sur les pages.

**Le CBR et le PDF sont désormais traités.** L'hydratation, prévue dès M1 et
laissée en suspens jusqu'ici, convertit ces formats en CBZ dans le cache dérivé
à l'indexation. Après quoi ils suivent exactement le chemin d'un CBZ — un
ReadRange par page, sans branchement. Le CB7 et l'EPUB restent détectés et
refusés nommément.

---

## M2 — API et lecture ✅ *(~2 semaines)*

**But :** l'API complète de lecture, prouvée sans interface.

- Authentification : inscription du premier admin, connexion, JWT + refresh, appareils.
- Endpoints catalogue : bibliothèques, séries, comics, recherche, filtres, tri, pagination par curseur.
- Endpoints lecture : `manifest`, `pages/{n}` avec négociation de format, `prefetch`.
- Cache dérivé + éviction LRU.
- Progression de lecture + endpoints `/sync`.
- OpenAPI complète, clients TS et Dart générés.

**Sortie :** un parcours complet — connexion, navigation, lecture de 20 pages, progression sauvegardée — validé par des tests d'API. La documentation OpenAPI est publiable.

**Atteint.** 24 endpoints, contrat OpenAPI complet, verrouillé par un test qui
valide chaque réponse du serveur réel contre le contrat publié — 28 sous-tests,
erreurs et réponses binaires comprises.

Points notables : rotation des refresh tokens avec révocation de chaîne sur
réutilisation détectée ; pagination par curseur ; recherche insensible aux
accents et tolérante aux fautes de frappe ; résolution de conflit de
progression « la page la plus avancée gagne », portée par le SQL ; ETag et
requêtes conditionnelles sur les images.

Écart assumé : le filtrage par classification d'âge est implémenté dans les
requêtes mais le `Viewer` ne porte pas encore `MaxAgeRating` — le relire en base
à chaque requête coûterait un aller-retour. Résolu par un cache utilisateur en
M7, quand les profils restreints seront réellement exposés.

---

## M3 — Web : bibliothèque ✅ *(~2 semaines)*

**But :** la première chose que verront les gens. Le soin visuel commence ici.

- Next.js en export statique embarqué dans le binaire Go.
- Design tokens partagés, thèmes clair et sombre.
- Écran de connexion + **assistant de première installation** (créer l'admin, connecter un backend, créer une bibliothèque, lancer le scan) — c'est la première impression, il mérite un vrai travail de finition.
- Grille virtualisée avec LQIP, recherche instantanée, filtres (série, auteur, genre, statut de lecture), tri.
- Page série et page détail d'un ouvrage.
- Étagères d'accueil : « Reprendre la lecture », « Récemment ajouté », « Prochain dans la série ».
- Responsive desktop et tablette.

**Sortie :** une bibliothèque de 2000 titres se parcourt sans à-coups. Premières captures d'écran présentables.

**Atteint.** Design tokens partagés web/Flutter, assistant d'installation,
accueil à étagères, grille virtualisée par rangées, séries, recherche avec
anti-rebond, filtres par statut de lecture et tri, aperçus de chargement LQIP.
Thème clair/sombre/système sans flash. Un binaire unique de 16 Mo.

Écart assumé : les routes de détail utilisent `?id=` — l'export statique exige
de connaître toutes les routes au build.

---

## M4 — Web : le lecteur ✅ *(~2 semaines)*

**But :** la pièce sur laquelle le projet sera jugé. Ne pas la bâcler.

- Modes page simple, double page (avec détection des doubles planches), défilement continu.
- Zoom au pincement et à la molette, panoramique inertiel, ajustement largeur/hauteur/page.
- Préchargement glissant, libération mémoire hors fenêtre, transitions sans clignotement.
- Clavier complet, gestes tactiles, plein écran, sens de lecture droite-à-gauche (manga).
- Interface qui s'efface, barre de progression, sélecteur de page avec miniatures.
- Sauvegarde de progression avec anti-rebond + persistance à la fermeture d'onglet.
- Réglages persistés par utilisateur.

**Sortie :** lire un album complet est agréable au point qu'on ne pense plus à l'outil. Critère subjectif assumé — c'est le différenciateur du projet.

**Atteint.** Trois modes, ajustements, sens de lecture inversable, clavier
complet, interface qui s'efface, préchargement glissant avec libération
mémoire, progression sauvegardée par anti-rebond et sendBeacon.

Le zoom au pincement et le sélecteur de page à miniatures, d'abord reportés,
sont faits. Le zoom prend deux formes, parce que ce sont deux gestes différents :
en mode page on agrandit une planche et on s'y déplace ; en défilement continu
on élargit la colonne.

La distinction n'est pas cosmétique. Transformer la page en défilement casserait
tout le reste — le conteneur ne connaîtrait plus la vraie hauteur de son contenu,
la barre sauterait, et la détection de la page courante, qui repose sur ce qui
occupe le centre de l'écran, deviendrait fausse. En jouant sur la largeur, le
reste continue de fonctionner sans le savoir.

La largeur de colonne est persistée : quelqu'un qui lit des webtoons la règle une
fois, pas à chaque album.

---

## M5 — Flutter : lecture en ligne — **atteint**

**But :** un client mobile de premier rang, pas un miroir du web.

- [x] Connexion (saisie de l'URL du serveur), stockage sécurisé des jetons, gestion multi-serveurs.
- [x] Schéma Drift, client API généré, chargement de la bibliothèque en cache local.
- [x] Écrans bibliothèque, série, détail — même langage visuel que le web via les tokens partagés.
- [x] Lecteur tactile : `PageView`, zoom, préchargement, plein écran immersif, mode manga.
- [x] Synchronisation de la progression avec file d'opérations hors ligne.
- [x] Recherche : serveur d'abord, repli sur le cache local hors ligne.
- [x] Navigation par séries, en plus des dossiers.

**Sortie :** APK de debug construit et vérifié en intégration continue. Le build
iOS reste à valider : il demande Xcode, absent de la machine de développement.

La recherche interroge le serveur, qui sait faire mieux — trigrammes, tolérance
aux fautes de frappe — et retombe sur le cache local quand le réseau manque. Ce
repli n'est pas un lot de consolation : hors ligne, les albums en cache sont
exactement ceux qu'on peut lire, donc les seuls qu'il vaille la peine de
chercher. Il se contente d'un pliage des accents et d'une correspondance par
sous-chaîne : « asterix » trouve « Astérix », « asterics » ne trouve rien.

Le client Dart est généré par `tools/generate-dart-models.mjs` plutôt que par
openapi-generator, qui exigerait une machine virtuelle Java — dépendance lourde
à imposer à un contributeur. Le générateur échoue franchement sur toute
construction qu'il ne sait pas traduire, plutôt que de produire un modèle
silencieusement faux.

---

## M6 — Flutter : hors ligne — **atteint**

**But :** ce qui distingue une vraie application mobile d'une page web.

- [x] Téléchargement d'un album ou d'une série entière, avec reprise.
- [x] Stockage local en pages transcodées, budget disque configurable.
- [x] Éviction automatique (lu d'abord, puis plus ancien), indicateurs d'occupation.
- [x] Lecture 100 % hors ligne, y compris serveur injoignable.
- [x] Réconciliation au retour du réseau, avec la règle « page la plus avancée gagne ».
- [x] Écran de gestion des téléchargements.

Les pages sont téléchargées une par une, redimensionnées, plutôt que l'archive
d'origine. Ce n'est pas un détail de mise en œuvre : l'application ne sait
décompresser ni un RAR ni un PDF, et embarquer les deux décodeurs pour lire un
album dans un train reviendrait à payer très cher une conversion que le serveur
fait déjà. Le corollaire est heureux — un album de soixante mégaoctets de
planches scannées en pèse une quinzaine à la définition d'un téléphone.

La reprise ne demande aucun protocole. Les pages sont des unités indépendantes
écrites dans l'ordre : le nombre de pages écrites EST le point de reprise, et
une interruption ne coûte au pire qu'une page. Chacune est écrite sous un nom
temporaire puis renommée, ce qui interdit qu'une coupure laisse un fichier
tronqué qu'on prendrait ensuite pour une page valide.

**Réserve :** le téléchargement ne survit pas à la fermeture de l'application.
Un vrai téléchargement d'arrière-plan demande WorkManager côté Android et une
extension côté iOS ; il reprend au retour au premier plan, et l'écran des
téléchargements le dit plutôt que de le laisser croire.

---

## M7 — Multi-utilisateur et administration — **atteint**

**But :** l'usage familial, qui est le scénario self-hosted dominant.

- [x] Gestion des comptes : création, rôles, réinitialisation de mot de passe.
- [x] `library_access` : bibliothèques partagées ou restreintes.
- [x] Profils restreints : filtrage par classification d'âge.
- [x] Console d'administration : backends de stockage, bibliothèques, scans et historique, statistiques de cache, purge.
- [x] Édition manuelle des métadonnées avec verrouillage des champs (`locked_fields`).
- [x] Gestion des appareils et révocation de session.

La révocation d'un appareil a demandé plus qu'une route. Un jeton d'accès est
autoporteur : le supprimer de la base ne l'empêche de rien tant qu'il n'a pas
expiré, et révoquer un téléphone perdu lui laissait donc un quart d'heure de
lecture — et surtout un quart d'heure pour en faire autre chose. La
vérification par appareil rejoint celle du compte, derrière le même cache de
quinze secondes : une requête par appareil actif et par quart de minute, contre
une fenêtre de résidu ramenée de quinze minutes à zéro sur une instance unique.

La purge du cache est présentée sans dramatisation, parce qu'elle n'en mérite
pas : tout s'y régénère depuis les archives d'origine. Un test de contrat le
vérifie plutôt que de l'affirmer — il lit une page, purge, et relit la même
page.

---

## M8 — Préparation de la release *(~2 semaines)*

**But :** ce qui transforme un projet fonctionnel en projet adopté. À ne pas compresser.

- [x] `Dockerfile` multi-étages, images multi-arch amd64/arm64 sur GHCR.
- [x] `docker-compose.yml` unique, avec MinIO inclus, qui fonctionne sans édition préalable.
- [x] Template Unraid, notes pour TrueNAS et Synology.
- [x] Documentation publique : installation, configuration, migration depuis Komga/Kavita.
- [x] README travaillé : positionnement explicite (« stockage objet natif »), OPDS et « Découvrir » annoncés. **Captures et GIF du lecteur restent à produire** — emplacement et cadrage réservés dans le README.
- [x] `CHANGELOG.md`, avec les limites connues écrites franchement.
- [x] Passe d'accessibilité (navigation clavier, contrastes, lecteurs d'écran).
- [x] i18n français et anglais.
- [x] Passe de sécurité : limitation de débit, en-têtes, protection SSRF sur les URL de backend, `SECURITY.md`.
- [x] Tests de charge sur une bibliothèque de 10 000 titres.
- [x] Étiquettes préparées dans `.github/labels.yml`.
- [x] Distribution mobile : **l'APK est embarqué dans l'image et servi par l'instance**.
- [ ] Roadmap publique en GitHub Projects — action sortante, à faire par le mainteneur.
- [ ] TestFlight — demande un compte Apple Developer et Xcode, absents de la machine.

**Sortie : v0.1.0 publiée.** Annonce sur r/selfhosted, r/comicbooks, Lemmy selfhosted, awesome-selfhosted.

### Internationalisation

**382 clés, deux langues.** Le catalogue anglais est typé d'après le français :
ajouter une clé sans la traduire casse la compilation. Une traduction manquante
ne peut donc pas être livrée en silence — ce qui compte, puisque personne ne
relit une interface dans une langue qu'il ne lit pas.

Le français reste le défaut, et pas par commodité : c'est la langue dans
laquelle le projet a été écrit, et une traduction faite après coup est toujours
moins juste que l'original. Le sélecteur est dans le menu du compte, et les
langues s'y affichent dans leur propre langue — « Français », « English » — et
non traduites : quelqu'un qui ne lit pas la langue courante doit reconnaître la
sienne, ce que « Anglais » ne permet pas.

Ce qui ne passe **pas** par le catalogue, et pourquoi :

- Les **dates relatives** viennent d'`Intl.RelativeTimeFormat`. Le navigateur
  connaît déjà les formes, y compris celles qu'on n'aurait pas prévues, et le
  catalogue n'a pas à porter douze entrées pour ce que la plateforme sait faire.
- Les **noms de séries et de dossiers** viennent des données, pas de
  l'interface.
- La **description `<meta>`** de `layout.tsx` est écrite à la construction.
  L'export statique produit un seul HTML servi à tout le monde : elle ne *peut*
  pas suivre la langue du visiteur. Limite du rendu statique, nommée dans le
  contrôle pour qu'on ne la redécouvre pas dans six mois.
- Les **commentaires et les messages de commit** restent en français. C'est la
  langue de travail du projet.

`npm run check:i18n` compte les chaînes encore écrites en dur et **échoue si le
nombre augmente**. À zéro aujourd'hui : une nouvelle chaîne non traduite fait
donc échouer l'intégration continue de celui qui vient de l'écrire, au moment
précis où il a le contexte pour la traduire.

### Charge mesurée

Dix mille albums, cent séries, sur PostgreSQL 17 en conteneur. Le pire de cinq
appels, pas la moyenne — une moyenne noierait exactement la requête lente qu'on
cherche.

| Route | Pire cas |
|---|---|
| `GET /comics?limit=100` | 15,6 ms |
| `GET /comics?sort=title` | 7,0 ms |
| `GET /comics?readStatus=unread` | 9,6 ms |
| `GET /search?q=album` | 17,8 ms |
| `GET /series?limit=100` | 0,9 ms |
| `GET /home` | 6,2 ms |
| Pagination complète, 51 pages | 21,0 ms par page |

La pagination par curseur reste plate du début à la fin : c'est ce que le test
vérifie en allant au bout des 10 002 albums, ce que personne ne fait à la main.
Un `OFFSET` se serait dégradé page après page sans que rien ne le signale.

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

1. ~~**OPDS sortant**~~ — **fait.** L'instance publie son catalogue en OPDS 1.2 sous `/opds`, avec authentification Basic — la seule qu'implémentent Chunky, Panels, KyBook ou Moon+ Reader. Les droits sont ceux de l'API, et un test le vérifie en comparant les deux portes pour un même compte.
2. **Découvrir : métadonnées** — la recherche fédérée et l'import sont faits ; enrichir la collection existante ne l'est pas. Voir la section dédiée ci-dessous.
3. **Import et téléversement** — déposer un fichier depuis le web ou le mobile vers un backend.
4. **OIDC** — Authelia, Authentik, Keycloak. Très demandé par le public self-hosted.
5. **Notifications push** — nouveautés dans une série suivie.
6. **Applications de bureau** — Flutter desktop, réutilise M5/M6.
7. **Recommandations** — d'abord par similarité de métadonnées, sans IA.
8. **Plugins et extensions** — n'ouvrir une API d'extension qu'une fois les frontières internes stabilisées par l'usage.
9. **IA : organisation, recherche sémantique, OCR, traduction** — puissant, mais coûteux et clivant sur ce public. À traiter en module optionnel, désactivé par défaut, jamais dans le chemin critique.

---

## « Découvrir » — retirée avant la v0.1.0

La recherche fédérée a été **retirée du projet**. Elle interrogeait des
catalogues extérieurs, importait vers un backend, lisait des sites sans API à
partir de gabarits déclaratifs, et enrichissait les métadonnées depuis des bases
publiques. Tout cela existait et fonctionnait ; l'historique git le garde.

Ce qui subsiste est la porte : le raccourci `Cmd/Ctrl + Maj + F` ouvre une
feuille qui annonce une fonctionnalité à venir. La supprimer aurait fait croire à
une régression silencieuse — on appuie, rien ne se passe, et rien ne dit si
c'est cassé ou voulu.

À ne pas confondre avec le **serveur OPDS**, qui reste : votre instance publie
son propre catalogue, lisible depuis Panels, Chunky, KyBook ou Thorium. Publier
ce qu'on possède et aller chercher ailleurs sont deux fonctionnalités
distinctes ; seule la seconde est partie.

Le jour où elle reviendra, deux questions se poseront avant la première ligne de
code : quelles sources, et sous quelle responsabilité. Ce sont elles qui
décident de la forme, pas l'inverse.

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
