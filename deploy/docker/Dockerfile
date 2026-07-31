# syntax=docker/dockerfile:1

# Image de boxincloud.
#
# Un seul binaire, l'interface web embarquée dedans. Pas de runtime Node à
# faire tourner à côté, pas de reverse-proxy à configurer pour répartir entre
# deux services : conséquence directe de l'export statique de Next.js
# (docs/01-architecture.md, ADR-003).
#
# Trois étapes. Les deux premières construisent, en parallèle quand le
# constructeur le permet ; la troisième n'emporte que le binaire.
#
# Construire depuis la racine du dépôt, pas depuis ce répertoire :
#   docker build -f deploy/docker/Dockerfile -t boxincloud .
#
# ─── WITH_MOBILE ─────────────────────────────────────────────────────────────
#
# `--build-arg WITH_MOBILE=0` construit le serveur SANS l'application Android.
# Le défaut est 1 : la CI et la publication ne changent pas de comportement, et
# l'image officielle embarque toujours l'APK.
#
# Cette porte existe pour une raison précise, découverte en essayant. L'étape
# mobile fait télécharger et exécuter le NDK Android par Gradle ; or le NDK ne
# publie ses binaires hôtes qu'en x86-64 pour Linux. Sur une machine arm64 —
# un Mac Apple Silicon, un serveur Ampere — le conteneur doit les émuler, et
# l'émulation casse :
#
#   rosetta error: failed to open elf at /lib64/ld-linux-x86-64.so.2
#   Process 'cmake' finished with non-zero exit value 133
#
# Sans cette porte, le serveur devenait donc INCONSTRUCTIBLE sur ces machines,
# pour un APK dont il n'a pas besoin pour fonctionner. Le couplage n'est pas
# gratuit et il ne doit pas être imposé à qui n'en veut pas.
#
# Ce qu'on ne perd pas : quand l'étape tourne — c'est-à-dire en CI et à la
# publication — l'application et le serveur restent construits ensemble et
# verrouillés sur la même version. La garantie défendue plus bas tient, elle
# n'est simplement plus une condition pour compiler.
#
# Le côté Go était déjà prêt : `//go:embed all:dist` s'accommode d'un répertoire
# vide, et `mobile.Available()` existe pour que la page de téléchargement
# s'adapte au lieu de proposer un lien mort.
ARG WITH_MOBILE=1

# ─── Interface web ───────────────────────────────────────────────────────────

FROM --platform=$BUILDPLATFORM node:22-alpine AS web

WORKDIR /src

# Les manifestes d'abord : tant qu'ils ne changent pas, la couche
# d'installation est réutilisée, et `npm ci` est de loin l'étape la plus lente.
COPY apps/web/package.json apps/web/package-lock.json ./apps/web/
RUN --mount=type=cache,target=/root/.npm \
    cd apps/web && npm ci

COPY packages ./packages
COPY api ./api
COPY tools ./tools
COPY apps/web ./apps/web

RUN cd apps/web && npm run build

# ─── Application Android ─────────────────────────────────────────────────────

# L'APK est construit ICI, dans la même image que le serveur, et embarqué dans
# le binaire. L'instance le sert donc elle-même : le téléphone qui scanne le
# code QR ne parle jamais à un service tiers, et une installation coupée
# d'Internet fonctionne comme les autres.
#
# Le bénéfice moins visible est plus durable : l'application et le serveur sont
# construits ensemble, donc verrouillés sur la même version. Il n'existe aucun
# couple app/serveur non testé.
#
# L'étape est la plus lourde de la construction — le SDK Flutter et la chaîne
# Android pèsent plusieurs gigaoctets. Elle est jetée à la fin ; seuls les
# soixante méga-octets de l'APK survivent.
FROM --platform=$BUILDPLATFORM ghcr.io/cirruslabs/flutter:stable AS mobile

WORKDIR /src/apps/mobile

# Les manifestes d'abord : tant qu'ils ne changent pas, la résolution des
# dépendances est réutilisée d'une construction à l'autre.
COPY apps/mobile/pubspec.yaml apps/mobile/pubspec.lock ./
RUN flutter pub get

COPY packages ../../packages
COPY api ../../api
COPY tools ../../tools
COPY apps/mobile ./

RUN flutter build apk --release

# ─── L'APK, présent ou absent ────────────────────────────────────────────────

# Deux étapes qui rendent la MÊME chose — un répertoire `/apk` — et dont une
# seule sera construite. C'est le seul moyen d'avoir un `COPY --from` conditionnel :
# BuildKit ne construit que les étapes atteignables, donc avec WITH_MOBILE=0 la
# chaîne Flutter n'est même pas téléchargée.
FROM mobile AS apk-1
RUN mkdir -p /apk && \
    cp build/app/outputs/flutter-apk/app-release.apk /apk/boxincloud.apk

# alpine plutôt qu'une image de plus : elle est déjà tirée pour l'étape finale.
FROM alpine:3.21 AS apk-0
RUN mkdir -p /apk

# `WITH_MOBILE` est déclaré tout en haut, AVANT le premier `FROM` : c'est la
# seule portée depuis laquelle un argument peut être lu dans une ligne `FROM`.
FROM apk-${WITH_MOBILE} AS apk

# ─── Serveur ─────────────────────────────────────────────────────────────────

# Construit sur l'architecture du constructeur et croise-compile vers la cible.
# Compiler dans un émulateur QEMU serait cinq à dix fois plus lent, pour un
# résultat identique : le code est du Go pur, sans cgo.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS server

WORKDIR /src/apps/server

COPY apps/server/go.mod apps/server/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY apps/server ./

# L'interface compilée est embarquée par `//go:embed all:dist` : elle doit être
# en place AVANT la compilation, sinon le binaire sert une page blanche.
COPY --from=web /src/apps/web/out ./web/dist

# L'APK de même : embarqué à la compilation, servi par l'instance.
#
# Le contenu du répertoire, pas le fichier : avec WITH_MOBILE=0 il est vide, et
# `COPY` d'un fichier absent ferait échouer la construction là où on veut
# précisément qu'elle continue. Le `.gitkeep` déjà présent dans `dist/` suffit à
# ce que `//go:embed all:dist` compile.
COPY --from=apk /apk/. ./mobile/dist/

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
        -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
        -o /out/boxincloud ./cmd/boxincloud && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
        -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
        -o /out/boxincloudctl ./cmd/boxincloudctl

# ─── Image finale ────────────────────────────────────────────────────────────

FROM alpine:3.21

# ca-certificates pour joindre un stockage S3 en HTTPS ; tzdata pour que les
# dates affichées suivent le fuseau de l'instance plutôt qu'UTC.
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 -h /var/lib/boxincloud boxincloud

COPY --from=server /out/boxincloud /out/boxincloudctl /usr/local/bin/

# Le cache dérivé et le stockage local par défaut vivent ici. À monter en
# volume : sans cela, chaque redémarrage repart d'un cache vide, et une
# bibliothèque en stockage local disparaîtrait avec le conteneur.
RUN mkdir -p /var/lib/boxincloud && chown boxincloud:boxincloud /var/lib/boxincloud
VOLUME /var/lib/boxincloud

USER boxincloud
WORKDIR /var/lib/boxincloud

ENV BOXINCLOUD_ADDR=:8080 \
    BOXINCLOUD_ENV=production \
    BOXINCLOUD_CACHE_DIR=/var/lib/boxincloud/cache

EXPOSE 8080

# Le point de santé du serveur, pas un simple TCP : une écoute ouverte ne dit
# pas que la base répond, et c'est cette panne-là qu'on veut voir.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["boxincloud"]
CMD ["serve"]
