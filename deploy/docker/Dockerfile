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
