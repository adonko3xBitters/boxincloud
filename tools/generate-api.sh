#!/usr/bin/env bash
#
# Génère les clients et le serveur depuis api/openapi.yaml.
#
# Chaque cible n'est produite que si son application existe et que ses outils
# sont installés : un contributeur backend n'a pas besoin de Node ni de Flutter
# pour lancer `make generate`.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SPEC="$ROOT/api/openapi.yaml"

cyan()  { printf '\033[36m%s\033[0m\n' "$*"; }
warn()  { printf '\033[33m→ %s\033[0m\n' "$*"; }
ok()    { printf '\033[32m✓ %s\033[0m\n' "$*"; }

[[ -f "$SPEC" ]] || { echo "✗ Contrat introuvable : $SPEC" >&2; exit 1; }

# ─── Serveur Go ──────────────────────────────────────────────────────────────

cyan "Serveur Go"

if ! command -v oapi-codegen >/dev/null 2>&1; then
    warn "oapi-codegen absent — 'make deps' pour l'installer. Étape ignorée."
else
    OUT_DIR="$ROOT/apps/server/internal/httpapi/gen"
    mkdir -p "$OUT_DIR"

    # Le chemin de sortie est passé en argument (-o), pas dans le fichier de
    # configuration : oapi-codegen y interpréterait `output` relativement au
    # répertoire courant et non au fichier de config.
    cat > "$OUT_DIR/config.yaml" <<'YAML'
# Généré par tools/generate-api.sh — ne pas éditer.
package: gen
generate:
  chi-server: true
  models: true
  strict-server: true
  embedded-spec: true
YAML

    oapi-codegen --config "$OUT_DIR/config.yaml" -o "$OUT_DIR/api.gen.go" "$SPEC"
    ok "apps/server/internal/httpapi/gen/api.gen.go"
fi

# ─── Client TypeScript ───────────────────────────────────────────────────────

cyan "Client TypeScript"

if [[ ! -d "$ROOT/apps/web" ]] || [[ ! -f "$ROOT/apps/web/package.json" ]]; then
    warn "apps/web pas encore initialisée (M3). Étape ignorée."
elif ! command -v npx >/dev/null 2>&1; then
    warn "npx absent. Étape ignorée."
else
    OUT_DIR="$ROOT/apps/web/src/lib/api"
    mkdir -p "$OUT_DIR"
    npx --yes openapi-typescript "$SPEC" -o "$OUT_DIR/schema.d.ts"
    ok "apps/web/src/lib/api/schema.d.ts"
fi

# ─── Client Dart ─────────────────────────────────────────────────────────────

cyan "Client Dart"

# Générateur maison plutôt qu'openapi-generator : ce dernier exige une machine
# virtuelle Java, dépendance lourde à imposer à quiconque veut contribuer au
# client mobile. Le dépôt écrit déjà son propre générateur pour les tokens de
# design ; celui-ci suit le même principe, avec zéro dépendance nouvelle.
if [[ ! -d "$ROOT/apps/mobile" ]] || [[ ! -f "$ROOT/apps/mobile/pubspec.yaml" ]]; then
    warn "apps/mobile pas encore initialisée. Étape ignorée."
elif ! command -v node >/dev/null 2>&1; then
    warn "node absent. Étape ignorée."
else
    node "$ROOT/tools/generate-dart-models.mjs"
fi

echo
ok "Génération terminée."
