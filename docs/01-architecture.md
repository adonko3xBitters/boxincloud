# Architecture

> Statut : proposition initiale, v0. Document vivant.

## 1. Principes directeurs

1. **Cloud-native d'abord.** Aucune partie du serveur ne suppose un système de fichiers local. Le filesystem est *un* backend de stockage parmi d'autres, pas le modèle de référence.
2. **API-first.** Le contrat OpenAPI est la source de vérité. Le web et le mobile sont deux clients de premier rang du même contrat ; aucun endpoint privé réservé au web.
3. **Monolithe modulaire.** Un seul binaire, des modules à frontières explicites. Pas de micro-services avant d'avoir une raison mesurée de découper.
4. **Déploiement trivial.** L'objectif est `docker compose up` qui fonctionne du premier coup, sans étape manuelle.
5. **Lisible par un contributeur.** Entre deux solutions équivalentes, on prend celle qu'un nouveau venu comprend sans documentation.

---

## 2. Décisions d'architecture (ADR condensés)

Trois écarts par rapport à la stack que tu as esquissée. Chacun est justifié par tes propres critères.

### ADR-001 — Routeur HTTP : **Chi**, pas Fiber

Fiber est bâti sur `fasthttp`, qui **n'implémente pas** `net/http`. Conséquences concrètes :

- Incompatible avec l'écosystème de middlewares standard (OpenTelemetry, `oauth2`, la majorité des libs d'auth).
- Le streaming et les requêtes `Range` — le cœur du produit — se font contre le grain de `fasthttp`, dont le modèle de réutilisation de buffers piège régulièrement sur les réponses longues.
- Un contributeur Go connaît `net/http` ; il ne connaît pas forcément l'API Fiber.

Chi est un routeur `net/http` pur, ~1000 lignes, sans dépendance. On garde la performance (le goulot est l'I/O réseau et le décodage d'images, pas le routage) et on gagne la compatibilité. *Alternative acceptable : `net/http` seul, dont le routeur sait depuis Go 1.22 gérer méthodes et wildcards.*

### ADR-002 — File de jobs : **River (sur PostgreSQL)**, pas Redis

Redis/Valkey ajoute un service à déployer, superviser et sauvegarder, pour un besoin — indexation, vignettes, scans — de quelques dizaines de jobs par minute au plus.

[River](https://github.com/riverqueue/river) est une file de jobs Go qui stocke tout dans PostgreSQL : jobs transactionnels (on enfile dans la même transaction que l'écriture métier, donc pas de job orphelin), retries, périodiques, interface web d'inspection.

La stack V1 devient **PostgreSQL + un binaire**. Deux conteneurs. C'est un avantage d'adoption réel sur le self-hosted.

Le cache applicatif (sessions, résultats chauds) reste en mémoire dans le process. Redis redeviendra pertinent le jour où on voudra scaler horizontalement — pas avant.

### ADR-003 — Web : Next.js en **export statique**, embarqué dans le binaire

Next.js en mode SSR impose un runtime Node à côté du serveur Go : deuxième conteneur, deuxième surface de mise à jour, complexité de configuration du reverse-proxy.

Or l'application est entièrement derrière authentification et fortement interactive — le SSR n'apporte ni SEO ni gain de premier rendu significatif.

On configure donc `output: 'export'`, et le bundle est embarqué dans le binaire Go via `embed.FS`. Tu gardes Next.js, React, TypeScript, Tailwind et shadcn/ui — l'expérience de développement est identique. On perd les Server Components et les Route Handlers, dont on n'a pas l'usage ici.

> Point de vigilance : cela fige le choix. Si tu prévois à terme un site vitrine public ou des pages de partage indexables, ils vivront dans un projet séparé, ce qui est de toute façon plus sain.

### Décisions alignées avec ta proposition

| Sujet | Choix | Raison |
|---|---|---|
| Langage serveur | Go 1.23+ | Streaming, concurrence, binaire statique, image Docker ~30 Mo |
| Accès données | **sqlc** | SQL explicite, typé, généré. Un contributeur lit du SQL, pas un DSL. Ent est plus lourd pour un modèle de cette taille |
| Base | PostgreSQL 16+ | Recherche plein texte native, `jsonb` pour les métadonnées, base de River |
| Migrations | **goose** | Fichiers SQL simples, embarqués, appliqués au démarrage |
| Contrat | OpenAPI 3.1, **spec-first** | `oapi-codegen` (Go) + `openapi-typescript` (web) + `openapi-generator` (Dart) depuis une source unique |
| Mobile | Flutter + Riverpod + Drift + Dio | Client de premier rang |
| Licence | **AGPL-3.0** | Cohérent avec Immich, Komga, Jellyfin. Protège contre un fork SaaS propriétaire |

---

## 3. Vue d'ensemble

```
┌──────────────┐   ┌──────────────┐
│   Web (SPA)  │   │Flutter mobile│
└──────┬───────┘   └──────┬───────┘
       │   HTTPS / REST   │
       └────────┬─────────┘
                ▼
┌─────────────────────────────────────────────┐
│              boxincloud (Go)                │
│  ┌───────────────────────────────────────┐  │
│  │  HTTP  auth · rate-limit · OpenAPI    │  │
│  ├───────────────────────────────────────┤  │
│  │ catalog │ reader │ progress │ library │  │  ← modules métier
│  ├───────────────────────────────────────┤  │
│  │ indexer │ archive │ imaging           │  │  ← traitement
│  ├───────────────────────────────────────┤  │
│  │ storage (StorageProvider)             │  │  ← abstraction
│  ├───────────────────────────────────────┤  │
│  │ platform  db · jobs · events · cache  │  │
│  └───────────────────────────────────────┘  │
└──────┬─────────────────────┬────────────────┘
       ▼                     ▼
┌─────────────┐   ┌──────────────────────────┐
│ PostgreSQL  │   │ MinIO · S3 · B2 · R2     │
│ (+ River)   │   │ WebDAV · filesystem      │
└─────────────┘   └──────────────────────────┘
```

**Règle de dépendance :** `http → métier → platform`. Un module métier ne référence jamais un autre module métier directement ; il passe par une interface qu'il définit lui-même, ou par un événement. C'est ce qui rend un découpage ultérieur possible sans réécriture.

---

## 4. Module `storage` — l'abstraction centrale

C'est la première chose à écrire, et rien ne doit la contourner.

```go
package storage

// Provider est l'unique porte d'accès aux octets. Aucun module métier
// n'ouvre de fichier ni ne parle à un SDK cloud directement.
type Provider interface {
    // Identité et santé
    Kind() Kind                 // s3 | webdav | local
    Ping(ctx context.Context) error

    // Parcours
    List(ctx context.Context, prefix string, fn func(ObjectInfo) error) error
    Stat(ctx context.Context, key string) (ObjectInfo, error)

    // Lecture — ReadRange est le point chaud du produit
    Open(ctx context.Context, key string) (io.ReadCloser, error)
    ReadRange(ctx context.Context, key string, off, length int64) (io.ReadCloser, error)

    // Écriture (cache dérivé, imports)
    Write(ctx context.Context, key string, r io.Reader, size int64, mime string) error
    Delete(ctx context.Context, key string) error

    // Optionnel — nil si non supporté
    PresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

type ObjectInfo struct {
    Key      string
    Size     int64
    ModTime  time.Time
    ETag     string
}
```

**Implémentations V1**

| Kind | Lib | `ReadRange` | Presign | Notes |
|---|---|---|---|---|
| `s3` | `minio-go/v7` | natif | oui | Couvre MinIO, AWS S3, Backblaze B2, Wasabi, Cloudflare R2, Garage |
| `local` | `os` | `ReadAt` | non | Chemin de migration depuis Komga/Kavita, et mode démo |
| `webdav` | `studio-b12/gowebdav` | via en-tête `Range` | non | Nextcloud, Synology. Post-V1 si le temps manque |

**Multi-backend.** Un `StorageBackend` est une ligne en base (type + configuration chiffrée). Une `Library` pointe vers un backend et un préfixe. Une instance peut donc servir une bibliothèque sur MinIO local et une autre sur B2 en simultané. Le registre de providers est instancié au démarrage et rafraîchi à chaud quand un backend est modifié.

**Chiffrement des identifiants.** Les clés d'accès S3 sont chiffrées en base (AES-GCM) avec une clé maître fournie par variable d'environnement `BOXINCLOUD_SECRET_KEY`. Jamais renvoyées par l'API, même à un admin.

---

## 5. Module `archive` — lecture de page à accès aléatoire

Le défi central : afficher la page 12 d'un CBZ de 200 Mo sur S3 sans le télécharger.

### CBZ / ZIP — accès aléatoire réel

Le format ZIP place son index (*Central Directory*) en fin de fichier. La séquence :

1. `ReadRange` sur les derniers ~64 Ko → localiser le *End of Central Directory*.
2. `ReadRange` sur le Central Directory → obtenir la liste des entrées, leur offset et leur taille compressée.
3. **Persister cet index en base** (table `comic_pages`) — on ne le relit plus jamais.
4. Pour la page N : un seul `ReadRange` sur l'entrée, décompression `flate` en flux.

Coût pour servir une page : une requête HTTP Range. C'est le chemin nominal, et la raison pour laquelle CBZ est le format recommandé.

### CBR / RAR — stratégie de repli

RAR ne permet pas d'accès aléatoire fiable (archives *solid*, en-têtes chaînés) et `nwaples/rardecode` ne fait que du séquentiel. Stratégie :

> **Hydratation au premier accès.** À la première ouverture, un job télécharge l'archive dans le cache local, extrait toutes les pages en WebP dans le backend de cache, et marque le comic `hydrated`. Les lectures suivantes tapent le cache et sont aussi rapides qu'un CBZ. L'utilisateur voit un état « préparation » de quelques secondes.

Le même mécanisme sert de repli universel pour tout format récalcitrant.

### PDF

`pdfcpu` (Go pur) pour l'extraction des pages et métadonnées. Le rendu vectoriel de qualité demanderait MuPDF via cgo — hors périmètre V1 : on traite d'abord les PDF composés d'images scannées, qui sont la quasi-totalité des BD.

### Cache dérivé

Toute donnée dérivée (pages transcodées, vignettes, couvertures) va dans un **backend de cache** — par défaut un bucket `boxincloud-cache` sur le backend par défaut, ou un volume local. Il est intégralement reconstructible : le supprimer ne perd aucune donnée utilisateur.

---

## 6. Module `imaging`

Interface `imaging.Processor`, avec deux implémentations prévues.

**Go pur, quatre formats en sortie.** Décodage JPEG/PNG/GIF/BMP/TIFF/WebP/AVIF, redimensionnement CatmullRom, sortie JPEG, PNG, WebP et AVIF. Toujours pas de cgo : les encodeurs modernes sont compilés en WebAssembly et exécutés par wazero, ce qui préserve `CGO_ENABLED=0`, le binaire unique et la compilation croisée sans chaîne C.

Prix payé : +16 Mo de binaire, dont ~3 pour le WebP et ~13 pour l'AVIF.

**Le format suit qui paie l'encodage.** C'est la mesure qui a tranché, et pas dans le sens attendu — sur une planche de 1600×2400 et sa vignette de 320 px :

| | page 1600×2400 | vignette 320 px |
|---|---|---|
| JPEG q85 | 846 Ko / 52 ms | 38,9 Ko / 2 ms |
| WebP q80 | 503 Ko / 148 ms | 27,1 Ko / 6 ms |
| AVIF q60 vitesse 8 | 322 Ko / **2,1 s** | 14,6 Ko / 130 ms |
| AVIF q60 vitesse 10 | **533 Ko / 663 ms** | — |

L'AVIF assez rapide pour être encodé pendant qu'un lecteur attend sa page est plus gros *et* presque aussi lent que le WebP : sur ce chemin, il ferait payer pour perdre. L'AVIF qui gagne vraiment coûte deux secondes.

D'où la règle :

- **Pages** — négociation entre **WebP** et **JPEG**. L'AVIF n'est pas proposé, même au client qui l'annonce.
- **Couvertures** — négociation entre **AVIF**, **WebP** et **JPEG**. Les 130 ms sont payées une fois par album et par taille, dans une grille qui est le plus gros transfert de l'application.

Seul le JPEG est produit à l'indexation ; les variantes modernes naissent à la première demande et restent en cache. Encoder trois formats d'avance gonflerait le scan pour produire des variantes que telle instance ne servira jamais — une bibliothèque lue depuis l'application Android n'a aucun usage de l'AVIF, que Flutter ne décode pas.

**Négociation stricte.** Seule une mention explicite dans `Accept` compte : un client qui envoie un joker reçoit du JPEG. « Je n'ai rien à déclarer » n'est pas « je sais tout lire », et les deux se ressemblent surtout quand on a tort. Aucun navigateur n'y perd — ils nomment tous leurs formats.

Les réponses portent `Vary: Accept`, y compris les 304, et l'ETag inclut le format. Sans cela, un proxy servirait l'AVIF du premier lecteur au suivant, qui peut être l'application Android.

**libvips via `govips` reste possible** derrière la même interface, pour les instances qui indexent des dizaines de milliers d'albums : nettement plus rapide et plus économe en mémoire. Elle impose cgo — un prix que la grande majorité des installations n'a aucune raison de payer.
- Vignettes en trois tailles : `sm` 160px, `md` 320px, `lg` 640px (largeur).
- Les dimensions de chaque page sont lues via `image.DecodeConfig` — quelques centaines d'octets d'en-tête au lieu de l'image entière — et persistées dans `comic_pages`.

---

## 7. Module `reader` — service de pages

```
GET /api/v1/comics/{id}/pages/{n}?width=1600&format=auto
```

Chaîne de résolution :

1. Cache dérivé — si la variante existe, redirection 302 vers une URL présignée (le serveur ne relaie pas les octets) ou service direct si le backend ne sait pas présigner.
2. Sinon : extraction via `archive`, transcodage via `imaging`, écriture dans le cache, service.
3. En-têtes `Cache-Control: private, max-age=31536000, immutable` + `ETag` — la variante d'une page est immuable.

**Préchargement.** Le client demande les pages N+1 et N+2 en arrière-plan. Le serveur expose `POST /comics/{id}/prefetch` qui enfile la préparation d'une plage de pages, pour absorber le coût du premier accès.

**Manifeste de lecture.** À l'ouverture, `GET /comics/{id}/manifest` renvoie en une requête la liste des pages avec dimensions et ratios — le client peut ainsi réserver la mise en page et éviter tout décalage visuel.

---

## 8. Module `indexer`

Pipeline événementiel, orchestré par River :

```
ScanLibrary → (par objet nouveau/modifié) IngestComic
                                              ├→ ExtractMetadata   (ComicInfo.xml, nom de fichier)
                                              ├→ BuildPageIndex    (index ZIP → comic_pages)
                                              ├→ GenerateCover     (page 1 → vignettes)
                                              └→ MatchSeries       (rattachement à une série)
```

- **Détection de changement** : clé + taille + ETag. Pas de hash complet au scan (coûteux en I/O distant) ; le hash est calculé de façon différée pour la déduplication.
- **Idempotence** : chaque job est rejouable sans effet de bord. Clé naturelle `(storage_backend_id, object_key)`.
- **Reprise** : un scan interrompu redémarre sans repartir de zéro, via un curseur de pagination persisté.
- **Déclenchement** : manuel, périodique (cron River), et à terme notifications de bucket (webhook MinIO) pour de l'indexation quasi temps réel.

Ordre de priorité des métadonnées, du plus fort au plus faible : saisie manuelle > `ComicInfo.xml` > analyse du nom de fichier > défauts. Un champ édité à la main est marqué et jamais écrasé par un rescan.

---

## 9. Module `progress` — synchronisation

Le mobile doit fonctionner hors ligne et se réconcilier au retour du réseau.

**Modèle** : chaque enregistrement de progression porte un `updated_at` serveur et un `version` (entier incrémental).

**Résolution de conflit** : *last-writer-wins* pondéré par la position — entre deux mises à jour concurrentes, on conserve **la page la plus avancée**, sauf si le client a explicitement remis à zéro (`status = 'unread'`). C'est le comportement attendu par un lecteur : on ne perd jamais sa progression, et lire sur tablette puis sur téléphone reprend au bon endroit.

**Protocole de synchronisation delta**

```
GET  /api/v1/sync?since={cursor}   → changements serveur depuis le curseur
POST /api/v1/sync                  → lot de mutations locales en attente
```

Le client Flutter tient une file d'opérations persistée en SQLite (Drift), la rejoue à la reconnexion, et applique les changements distants. Le curseur est un horodatage logique renvoyé par le serveur.

---

## 10. Module `auth`

- Mots de passe : **argon2id** (paramètres OWASP).
- Sessions : JWT d'accès courte durée (15 min) + refresh token opaque en base, avec rotation et détection de réutilisation.
- Le refresh token est lié à un `device_id` — ce qui donne gratuitement la gestion « appareils connectés » et la révocation unitaire.
- Rôles V1 : `admin`, `user`. Un profil peut être marqué `restricted` (contenu filtré par classification d'âge) — la base du profil enfant, sans en développer l'UI en V1.
- OIDC (Authelia, Authentik, Keycloak) prévu par une interface `auth.Provider` mais non implémenté en V1.

---

## 11. Application web

- **Next.js 15** en export statique, React 19, TypeScript strict, TailwindCSS 4, shadcn/ui.
- **TanStack Query** pour le cache serveur, **Zustand** pour l'état du lecteur (léger, hors React tree — important pour la fluidité).
- Client API **généré** depuis l'OpenAPI : aucun type écrit à la main.

**Bibliothèque** : grille virtualisée (`@tanstack/react-virtual`) — indispensable au-delà de quelques centaines de couvertures. Images en `content-visibility: auto`, vignettes servies en AVIF/WebP avec placeholder flouté (LQIP stocké en base sous forme de data-URI de 32 octets).

**Lecteur** — c'est la pièce où se joue la réputation du projet :

- Modes : page simple, double page (avec détection des doubles planches), défilement continu (mode webtoon).
- Navigation clavier complète, gestes tactiles, zoom au pincement avec panoramique inertiel.
- Préchargement glissant de 3 pages en avant, 1 en arrière ; les pages sortant de la fenêtre sont libérées.
- Aucune bibliothèque de lecteur tierce : `<img>` + `transform` CSS, accéléré GPU. Les libs génériques de visionnage sont systématiquement plus lourdes et moins fluides que 300 lignes ciblées.
- Interface qui s'efface : barres masquées par défaut, révélées au mouvement ou au tap central.

---

## 12. Application Flutter

- **Riverpod** (gestion d'état), **Drift** (SQLite typé), **Dio** (HTTP), **go_router** (navigation).
- Client API généré depuis le même OpenAPI.
- **Architecture offline-first** : l'UI lit *toujours* la base locale ; la couche réseau alimente cette base. L'application ne connaît pas d'écran de chargement dépendant du réseau.
- **Téléchargements** : `background_downloader` pour la poursuite en arrière-plan, avec file, reprise, et budget disque configurable (politique d'éviction : lu en premier, puis plus ancien).
- **Stockage des albums hors ligne** : archive originale si CBZ (compacte), sinon pages transcodées. Chiffrement au repos optionnel.
- **Lecteur** : `PageView` custom avec `precacheImage`, `InteractiveViewer` réglé pour le zoom, et rendu en `RepaintBoundary` isolé.

---

## 13. Transverse

**Observabilité** — `log/slog` en JSON, OpenTelemetry (traces + métriques) désactivé par défaut, `/healthz` et `/readyz`, `/metrics` Prometheus optionnel.

**Configuration** — variables d'environnement en source unique (12-factor), fichier `.env` accepté, validation au démarrage avec message d'erreur explicite. Aucune configuration obligatoire au-delà de `DATABASE_URL` et `BOXINCLOUD_SECRET_KEY` : le reste se fait dans l'assistant de première installation.

**Tests** — tests unitaires sur la logique pure ; **testcontainers** pour PostgreSQL et MinIO sur les tests d'intégration (aucun mock de S3, on teste contre un vrai MinIO) ; jeu de fichiers de test versionné (CBZ/CBR/PDF minimaux, libres de droits) ; tests de bout en bout Playwright sur le web.

**CI/CD** — GitHub Actions : lint (`golangci-lint`, `eslint`, `dart analyze`), tests, build multi-arch (amd64/arm64) et publication sur GHCR, `release-please` pour le versionnement sémantique et le journal des modifications.

**Sécurité** — SSRF : les URL de backend fournies par l'utilisateur sont validées (pas de plage IP privée sans opt-in explicite). Limitation de débit sur l'authentification. En-têtes CSP stricts. Aucun secret dans les logs.
