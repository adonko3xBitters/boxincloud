#!/usr/bin/env bash
#
# Vérification automatique après édition d'un fichier.
#
# Branché en PostToolUse sur Write|Edit. Reçoit sur l'entrée standard le JSON du
# hook, en extrait le fichier touché, et lance le contrôle qui correspond à son
# écosystème — rien pour les autres.
#
# # Ce que ce hook n'est PAS
#
# Ce n'est pas la CI. Il ne lance ni les tests, ni les linters complets, ni les
# contrôles maison du web : sur un projet où `go test` prend une minute et où
# `golangci-lint` en prend trente secondes, un contrôle à chaque frappe rendrait
# l'édition insupportable et finirait par être désactivé.
#
# Il attrape la classe d'erreurs qui coûte le plus cher à découvrir tard, et
# elle seule : le code qui ne compile pas, et le formatage qui fera échouer la
# CI sur un détail.
#
# # Il ne bloque jamais
#
# Sortie 0 dans tous les cas. Un contrôle qui interrompt le travail sur un
# fichier volontairement laissé à moitié écrit est un contrôle qu'on désactive.
# Le résultat remonte comme message, pas comme refus.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

payload=$(cat)
file=$(printf '%s' "$payload" | jq -r '.tool_input.file_path // .tool_response.filePath // empty' 2>/dev/null)

[ -n "$file" ] || exit 0
[ -f "$file" ] || exit 0

output=""

case "$file" in
  "$ROOT"/apps/server/*.go)
    # Le code généré n'est pas de notre ressort : sqlc et oapi-codegen le
    # reformatent à leur façon, et le signaler à chaque régénération serait du
    # bruit permanent.
    case "$file" in
      */internal/platform/sqlc/*|*/internal/httpapi/gen/*) exit 0 ;;
    esac

    export PATH="$PATH:$(go env GOPATH)/bin"
    unformatted=$(gofmt -l "$file" 2>&1)
    [ -n "$unformatted" ] && output="gofmt : $unformatted"

    if ! build=$(cd "$ROOT/apps/server" && go build ./... 2>&1); then
      output="${output:+$output$'\n'}$build"
    fi
    ;;

  "$ROOT"/apps/web/src/*.ts|"$ROOT"/apps/web/src/*.tsx)
    # schema.d.ts est engendré depuis le contrat : une erreur dedans se corrige
    # dans api/openapi.yaml, pas ici.
    case "$file" in
      */lib/api/schema.d.ts) exit 0 ;;
    esac

    if ! types=$(cd "$ROOT/apps/web" && npx --no-install tsc --noEmit 2>&1); then
      output=$(printf '%s' "$types" | head -20)
    fi
    ;;

  *)
    exit 0
    ;;
esac

[ -n "$output" ] || exit 0

# Le résultat revient en contexte plutôt qu'en refus : on veut le savoir, pas
# être arrêté.
jq -n --arg o "$output" '{
  systemMessage: ("auto-check :\n" + $o),
  hookSpecificOutput: {
    hookEventName: "PostToolUse",
    additionalContext: ("auto-check a trouvé ceci après votre édition :\n" + $o)
  }
}'
exit 0
