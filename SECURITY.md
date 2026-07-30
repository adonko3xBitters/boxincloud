# Politique de sécurité

## Versions supportées

Le projet est en pré-alpha. Aucune version n'est encore publiée ; seule la branche `main`
reçoit des correctifs.

| Version | Supportée |
|---|---|
| `main` | ✅ |
| < 0.1.0 | ❌ |

## Signaler une vulnérabilité

**N'ouvrez pas d'issue publique pour une faille de sécurité.**

Utilisez [GitHub Security Advisories](https://github.com/adonko3xBitters/boxincloud/security/advisories/new)
— c'est le canal privé du dépôt.

Merci d'inclure :

- le type de faille et le composant concerné ;
- les étapes de reproduction, avec la configuration utilisée ;
- l'impact estimé ;
- le cas échéant, une preuve de concept.

**Délais visés :** accusé de réception sous 72 heures, première évaluation sous 7 jours,
correctif selon la gravité. Vous serez crédité dans l'avis publié, sauf demande contraire.

## Périmètre

Sont considérés dans le périmètre :

- contournement d'authentification ou d'autorisation ;
- accès à une bibliothèque ou à un contenu sans les droits correspondants ;
- exposition des identifiants de backend de stockage ;
- SSRF via une URL de backend fournie par l'utilisateur ;
- injection SQL, XSS, CSRF ;
- traversée de chemin dans les clés d'objet ou les entrées d'archive.

Hors périmètre :

- les instances délibérément exposées sans authentification ;
- le déni de service par saturation de ressources sur une instance auto-hébergée ;
- les rapports issus d'un scanner automatique sans démonstration d'exploitabilité.

## Notes de durcissement pour les administrateurs

- `BOXINCLOUD_SECRET_KEY` chiffre les identifiants de stockage en base. **Sa perte rend
  les backends inutilisables** ; sa fuite les expose. Sauvegardez-la séparément de la base.
- Placez l'instance derrière HTTPS. Aucun jeton ne doit transiter en clair.
- Les identifiants de backend doivent porter le privilège minimal : accès en lecture au
  seul préfixe concerné suffit pour une bibliothèque.
