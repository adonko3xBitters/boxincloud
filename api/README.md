# Contrat d'API

`openapi.yaml` est la **source de vérité** de l'API boxincloud.

Le serveur Go, le client TypeScript du web et le client Dart de l'application
Flutter en sont générés. Aucun de ces trois n'est écrit à la main.

## Modifier l'API

```bash
# 1. éditer api/openapi.yaml
# 2. régénérer
make generate-api
# 3. implémenter côté serveur, puis consommer côté clients
```

L'ordre compte. Écrire le handler d'abord et documenter ensuite conduit
mécaniquement à un contrat qui décrit le serveur au lieu de le contraindre — et
à des clients qui divergent.

## Ce que la génération produit

| Cible | Sortie | Outil |
|---|---|---|
| Serveur Go | `apps/server/internal/httpapi/gen/` | `oapi-codegen` |
| Web | `apps/web/src/lib/api/` | `openapi-typescript` |
| Mobile | `apps/mobile/lib/core/api/` | `openapi-generator` (dart-dio) |

Ces répertoires sont versionnés mais **jamais édités à la main**. La CI vérifie
qu'ils correspondent au contrat.

## Règles de compatibilité

`/api/v1` ne change qu'en cas de rupture. Sont considérés **rétrocompatibles** et
n'exigent pas de nouvelle version :

- ajouter un endpoint ;
- ajouter un champ **optionnel** à une requête ;
- ajouter un champ à une réponse ;
- ajouter une valeur à une énumération **de réponse** — à condition que les
  clients traitent l'inconnu par un cas par défaut.

Sont des **ruptures**, et exigent `/api/v2` :

- supprimer ou renommer un champ ;
- rendre obligatoire un champ qui ne l'était pas ;
- changer le type d'un champ ;
- modifier le sens d'une valeur existante.

Le mobile impose une prudence particulière : une version ancienne de
l'application peut rester installée longtemps après la mise à jour du serveur.
Une rupture ne casse pas seulement un navigateur qu'il suffit de rafraîchir.

## Conventions

- `operationId` en `camelCase`, verbe en tête : `getVersion`, `listComics`,
  `updateReadingProgress`. C'est lui qui nomme les fonctions générées.
- Chaque endpoint porte un `tags` — il structure la documentation et regroupe
  les méthodes des clients générés.
- Chaque réponse d'erreur référence `#/components/responses/*` plutôt que de
  redéfinir le schéma.
- Les descriptions sont écrites pour quelqu'un qui ne connaît pas le code.
