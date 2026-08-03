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

# La version de l'APK vient du tag, pas du pubspec.
#
# Le nombre après « + » dans pubspec.yaml est le versionCode Android. Il doit
# croître à chaque publication, sans quoi le système ne reconnaît pas une
# installation comme plus récente que la précédente. Laissé à la main, il est
# resté à 1 pendant toute la 0.1.0, sur un APK qui s'annonçait par ailleurs en
# 1.0.0 — une version qui n'a jamais existé.
#
# Le rang choisi est majeur × 10000 + mineur × 100 + correctif : 0.1.1 donne
# 101, 1.0.0 donne 10000. Croissant tant que mineur et correctif restent sous
# 100, ce que la dérivation vérifie plutôt que de le supposer.
#
# Une construction hors tag — `VERSION=dev`, le défaut — reçoit 0.0.0 et le
# code 1. Surtout pas un code élevé : il empêcherait ensuite d'installer une
# vraie version par-dessus.
#
# Les préversions partagent le code de la version finale qu'elles visent :
# v0.2.0-rc1 et v0.2.0 donnent tous deux 20000. Installer la finale par-dessus
# la préversion fonctionne, sans être vu comme une mise à jour.
ARG VERSION=dev
RUN set -eu; \
    name="${VERSION#v}"; \
    name="${name%%-*}"; \
    major="${name%%.*}"; \
    rest="${name#*.}"; \
    minor="${rest%%.*}"; \
    patch="${rest#*.}"; \
    valid=1; \
    case "$name" in *.*.*) ;; *) valid=0 ;; esac; \
    for part in "$major" "$minor" "$patch"; do \
        case "$part" in '' | *[!0-9]*) valid=0 ;; esac; \
    done; \
    if [ "$valid" -eq 1 ] && { [ "$minor" -gt 99 ] || [ "$patch" -gt 99 ]; }; then \
        valid=0; \
    fi; \
    if [ "$valid" -eq 1 ]; then \
        code=$(( major * 10000 + minor * 100 + patch )); \
    else \
        echo "VERSION=$VERSION n'est pas un tag de version : APK en 0.0.0 (code 1)"; \
        name=0.0.0; code=1; \
    fi; \
    echo "APK $name (versionCode $code)"; \
    flutter build apk --release --build-name="$name" --build-number="$code"

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

ARG TARGETOS
ARG TARGETARCH

# Les caches sont NOMMÉS PAR ARCHITECTURE.
#
# Sans `id`, BuildKit donne à tous les montages visant le même chemin un cache
# unique — partagé, ici, entre la construction amd64 et la construction arm64
# qui tournent en parallèle. C'est le défaut qui a produit une image arm64
# inutilisable : le binaire livré réclamait le chargeur glibc
# `/lib/ld-linux-aarch64.so.1`, absent d'alpine, alors que la commande de
# compilation portait bien `CGO_ENABLED=0`.
#
# Un binaire Go sans cgo n'a pas d'interpréteur. Celui-là en avait un : il ne
# venait donc pas de cette compilation, mais du cache d'une autre.
COPY apps/server/go.mod apps/server/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod,id=gomod-$TARGETARCH \
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

ARG VERSION=dev
ARG COMMIT=unknown

# L'interpréteur ELF est FIXÉ, et il faut expliquer pourquoi.
#
# `CGO_ENABLED=0` ne suffit pas ici. L'encodeur AVIF passe par
# `gen2brain/avif`, qui passe par `ebitengine/purego`, lequel charge sa
# bibliothèque à l'exécution avec `//go:cgo_import_dynamic` — un mécanisme qui
# impose un interpréteur ELF SANS utiliser cgo. Le binaire n'est donc jamais
# statique, quoi qu'on demande.
#
# En compilation native, Go écrit le chemin de la libc qu'il voit : musl, dans
# alpine. En compilation CROISÉE il ne peut pas la deviner et écrit le chemin
# conventionnel, celui de la glibc — `/lib/ld-linux-aarch64.so.1`, absent
# d'alpine. C'est exactement ce qui a rendu l'image arm64 de la v0.0.1-rc1
# inutilisable, alors que l'amd64, compilée nativement, fonctionnait.
#
# Bissection qui a mené là : un « hello world » croisé vers arm64 sort statique
# dans cette même image ; le nôtre non. La chaîne d'outils était donc saine, et
# la cause dans nos dépendances.
#
# `-I` dit au lieur quel interpréteur inscrire. Il vaut mieux qu'un binaire
# statique qu'on ne peut pas obtenir, et mieux qu'une image de base glibc qui
# pèserait dix fois plus.
RUN --mount=type=cache,target=/go/pkg/mod,id=gomod-$TARGETARCH \
    --mount=type=cache,target=/root/.cache/go-build,id=gobuild-$TARGETARCH \
    case "$TARGETARCH" in \
        amd64) MUSL=/lib/ld-musl-x86_64.so.1 ;; \
        arm64) MUSL=/lib/ld-musl-aarch64.so.1 ;; \
        *) echo "architecture non prévue : $TARGETARCH"; exit 1 ;; \
    esac && \
    for target in boxincloud boxincloudctl; do \
        CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
        go build -trimpath -tags osusergo,netgo \
            -ldflags "-s -w -I $MUSL -X main.version=${VERSION} -X main.commit=${COMMIT}" \
            -o "/out/$target" "./cmd/$target" || exit 1; \
    done

# Le binaire tournera-t-il dans l'image finale ?
#
# La question n'est pas rhétorique : elle a déjà eu la mauvaise réponse. La
# v0.0.1-rc1 a publié une image arm64 dont le binaire réclamait le chargeur
# glibc `/lib/ld-linux-aarch64.so.1`, absent d'alpine. Elle s'installait sans
# bruit et échouait au premier `exec`, chez l'utilisateur.
#
# Ce qu'on vérifie est donc l'INTERPRÉTEUR, pas la staticité. Ces binaires sont
# légitimement dynamiques et liés à musl — ce qu'alpine fournit. Exiger un
# binaire statique refuserait toutes les constructions saines ; exiger musl
# attrape exactement le défaut rencontré.
RUN apk add --no-cache file >/dev/null && \
    for binary in /out/boxincloud /out/boxincloudctl; do \
        description=$(file "$binary"); \
        case "$description" in \
            *"statically linked"*|*ld-musl*) ;; \
            *) echo "✗ $binary ne tournera pas sur alpine :"; \
               echo "  $description"; \
               exit 1 ;; \
        esac; \
    done && \
    echo "✓ binaires compatibles avec l'image finale"

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
