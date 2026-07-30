# boxincloud — orchestration du monorepo
#
# Chaque écosystème garde ses outils natifs ; ce Makefile ne fait que les appeler.
# Une contribution au serveur ne nécessite ni Node ni Flutter.

SHELL       := /bin/bash
.DEFAULT_GOAL := help

SERVER_DIR  := apps/server
WEB_DIR     := apps/web
MOBILE_DIR  := apps/mobile
BIN_DIR     := bin

GO          ?= go
COMPOSE_DEV := docker compose -f docker-compose.dev.yml

# Chargé pour les cibles qui en ont besoin (migrations, run)
ifneq (,$(wildcard .env))
	include .env
	export
endif

DATABASE_URL ?= postgres://boxincloud:boxincloud@localhost:5432/boxincloud?sslmode=disable

.PHONY: help
help: ## Affiche cette aide
	@echo "boxincloud — cibles disponibles"
	@echo ""
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""

# ─── Environnement ───────────────────────────────────────────────────────────

.PHONY: deps
deps: ## Installe les outils Go (sqlc, goose, oapi-codegen, air, golangci-lint)
	$(GO) install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	$(GO) install github.com/pressly/goose/v3/cmd/goose@latest
	$(GO) install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	$(GO) install github.com/air-verse/air@latest
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@echo "→ Assurez-vous que \$$(go env GOPATH)/bin est dans votre PATH"

.PHONY: env
env: ## Crée .env depuis .env.example avec une clé secrète générée
	@test -f .env && echo "→ .env existe déjà, rien à faire" && exit 0 || true
	@cp .env.example .env
	@key=$$(openssl rand -hex 32); \
	 if sed --version >/dev/null 2>&1; then \
	   sed -i "s|^BOXINCLOUD_SECRET_KEY=.*|BOXINCLOUD_SECRET_KEY=$$key|" .env; \
	 else \
	   sed -i '' "s|^BOXINCLOUD_SECRET_KEY=.*|BOXINCLOUD_SECRET_KEY=$$key|" .env; \
	 fi
	@echo "→ .env créé, BOXINCLOUD_SECRET_KEY générée"

# ─── Développement ───────────────────────────────────────────────────────────

.PHONY: dev-deps
dev-deps: ## Démarre PostgreSQL et MinIO
	$(COMPOSE_DEV) up -d
	@echo "→ PostgreSQL  localhost:5432"
	@echo "→ MinIO API   localhost:9000   console http://localhost:9001  (boxincloud/boxincloud)"

.PHONY: dev-deps-down
dev-deps-down: ## Arrête PostgreSQL et MinIO
	$(COMPOSE_DEV) down

.PHONY: dev-reset
dev-reset: ## Détruit les volumes de développement et redémarre à vide
	$(COMPOSE_DEV) down -v
	$(COMPOSE_DEV) up -d

.PHONY: dev-server
dev-server: ## Démarre l'API avec rechargement à chaud
	cd $(SERVER_DIR) && air

.PHONY: dev-web
dev-web: ## Démarre l'application web
	cd $(WEB_DIR) && npm run dev

.PHONY: dev-mobile
dev-mobile: ## Démarre l'application Flutter
	cd $(MOBILE_DIR) && flutter run

.PHONY: dev
dev: dev-deps ## Démarre les dépendances puis l'API
	@$(MAKE) dev-server

# ─── Génération ──────────────────────────────────────────────────────────────

.PHONY: generate
generate: generate-api generate-sql generate-tokens ## Régénère tout le code généré

.PHONY: generate-api
generate-api: ## OpenAPI → serveur Go + clients TypeScript et Dart
	./tools/generate-api.sh

.PHONY: generate-tokens
generate-tokens: ## tokens.json → variables CSS (web) + constantes Dart (mobile)
	node packages/design-tokens/build.mjs

.PHONY: generate-sql
generate-sql: ## queries/*.sql → Go typé (sqlc)
	cd $(SERVER_DIR) && sqlc generate

.PHONY: generate-check
generate-check: generate ## Échoue si le code généré n'est pas à jour (utilisé en CI)
	@git diff --exit-code || \
		(echo "✗ Code généré obsolète. Lancez 'make generate' et committez." && exit 1)

# ─── Base de données ─────────────────────────────────────────────────────────

GOOSE = goose -dir $(SERVER_DIR)/migrations postgres "$(DATABASE_URL)"

.PHONY: migrate-up
migrate-up: ## Applique les migrations en attente
	$(GOOSE) up

.PHONY: migrate-down
migrate-down: ## Annule la dernière migration
	$(GOOSE) down

.PHONY: migrate-status
migrate-status: ## Affiche l'état des migrations
	$(GOOSE) status

.PHONY: migrate-new
migrate-new: ## Crée une migration — make migrate-new name=add_reading_lists
	@test -n "$(name)" || (echo "✗ Usage : make migrate-new name=ma_migration" && exit 1)
	goose -dir $(SERVER_DIR)/migrations -s create $(name) sql

# ─── Qualité ─────────────────────────────────────────────────────────────────

.PHONY: test
test: ## Tests unitaires (rapides, sans dépendance externe)
	cd $(SERVER_DIR) && $(GO) test ./... -race -short

.PHONY: test-integration
test-integration: ## Tests d'intégration (testcontainers : PostgreSQL + MinIO réels)
	cd $(SERVER_DIR) && $(GO) test ./... -race -run Integration -count=1

.PHONY: cover
cover: ## Rapport de couverture HTML
	cd $(SERVER_DIR) && $(GO) test ./... -short -coverprofile=coverage.out
	cd $(SERVER_DIR) && $(GO) tool cover -html=coverage.out

.PHONY: lint
lint: ## Analyse statique
	cd $(SERVER_DIR) && $(GO) vet ./...
	cd $(SERVER_DIR) && golangci-lint run

.PHONY: fmt
fmt: ## Formate le code
	cd $(SERVER_DIR) && $(GO) fmt ./...

.PHONY: tidy
tidy: ## Nettoie go.mod
	cd $(SERVER_DIR) && $(GO) mod tidy

# ─── Build ───────────────────────────────────────────────────────────────────

VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS   := -s -w \
             -X main.version=$(VERSION) \
             -X main.commit=$(COMMIT)

.PHONY: build
build: ## Compile le binaire du serveur
	mkdir -p $(BIN_DIR)
	cd $(SERVER_DIR) && $(GO) build -ldflags "$(LDFLAGS)" -o ../../$(BIN_DIR)/boxincloud ./cmd/boxincloud
	cd $(SERVER_DIR) && $(GO) build -ldflags "$(LDFLAGS)" -o ../../$(BIN_DIR)/boxincloudctl ./cmd/boxincloudctl
	@echo "→ $(BIN_DIR)/boxincloud $(VERSION)"

.PHONY: build-web
build-web: ## Compile l'application web dans le répertoire embarqué du serveur
	cd $(WEB_DIR) && npm ci && npm run generate:api && npm run build
	rm -rf $(SERVER_DIR)/web/dist
	cp -r $(WEB_DIR)/out $(SERVER_DIR)/web/dist

.PHONY: build-all
build-all: build-web build ## Compile le web puis le binaire complet

.PHONY: docker
docker: ## Construit l'image Docker
	docker build -f deploy/docker/Dockerfile -t boxincloud:$(VERSION) .

.PHONY: clean
clean: ## Supprime les artefacts de build
	rm -rf $(BIN_DIR) $(SERVER_DIR)/coverage.out $(SERVER_DIR)/web/dist
