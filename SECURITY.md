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

---

## Mesures en place

Ce que le serveur fait, et ce qu'il ne fait délibérément pas.

### Authentification

Mots de passe en argon2id. Jetons d'accès JWT HS256 de quinze minutes, jetons
de rafraîchissement tournants : réutiliser un jeton déjà échangé révoque toute
la chaîne, ce qui transforme un vol de jeton en déconnexion visible plutôt
qu'en accès silencieux.

Un jeton d'accès est autoporteur, donc valable jusqu'à son expiration même
après une désactivation. Le compte ET l'appareil sont donc relus en base
derrière un cache de quinze secondes : désactiver un compte, rétrograder un
administrateur ou révoquer un téléphone perdu prend effet immédiatement sur
une instance unique, et au pire quinze secondes plus tard.

### Limitation de débit

Cinq tentatives d'affilée sur `/auth`, puis une toutes les douze secondes, par
adresse d'origine. La clé est l'adresse seule et non l'identifiant : compter
par identifiant permettrait à qui connaît un nom d'utilisateur de le
verrouiller depuis n'importe où.

`X-Forwarded-For` est lu sans vérifier qui l'a écrit. C'est un arbitrage :
sans cette lecture, toutes les requêtes derrière un reverse-proxy
partageraient son adresse et un seul mot de passe raté verrouillerait la
maisonnée. **Le serveur doit donc n'écouter que sur l'interface du proxy** —
s'il est joignable directement, la limite se contourne en forgeant l'en-tête.

En mémoire, dans le processus. Plusieurs répliques derrière un répartiteur
diviseraient la limite par leur nombre.

### Adresses de backend

Un backend S3 est une adresse que le serveur va joindre : c'est la définition
d'une SSRF. Sont refusés le lien-local IPv4 et IPv6 — où répondent les services
de métadonnées d'AWS, GCP, Azure, DigitalOcean et Hetzner — ainsi que
`0.0.0.0/8`.

Les adresses privées et la boucle locale sont **acceptées**, et ce n'est pas un
oubli : `minio:9000` et `192.168.1.10` sont les adresses normales d'une
instance auto-hébergée. Les refuser ne sécuriserait rien et casserait le cas
d'usage principal.

Réserve connue : le contrôle porte sur ce qui est saisi, pas sur ce que le nom
résoudra. Un nom de domaine pointant vers `169.254.169.254` passerait. Contre
ce cas, la défense est de refuser au conteneur l'accès au réseau de métadonnées
— c'est une affaire de déploiement, pas de code applicatif.

### En-têtes

`Content-Security-Policy` sur les documents, `X-Frame-Options: DENY`,
`X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin`,
`Permissions-Policy` refusant caméra, micro, géolocalisation et paiement.

`Strict-Transport-Security` est **absent**, volontairement. Beaucoup
d'instances tournent en HTTP simple sur un réseau local ; le poser depuis une
instance accessible dans les deux modes verrouillerait le navigateur sur une
adresse devenue injoignable. C'est au reverse-proxy, qui sait s'il termine TLS,
de le décider.

### Secrets

Les identifiants des backends de stockage sont chiffrés en base (AES-256-GCM)
par `BOXINCLOUD_SECRET_KEY`. **Sa perte rend les backends illisibles** :
sauvegardez-la ailleurs que dans la sauvegarde de la base, sans quoi les deux
disparaîtront ensemble.

Le jeton d'accès est accepté en paramètre d'URL sur deux familles de routes
seulement — les images, qu'une balise `<img>` ne peut pas assortir d'un
en-tête, et l'envoi de progression par `sendBeacon` à la fermeture d'un onglet.
Ailleurs, jamais : un jeton dans une URL fuit par le `Referer`, les journaux de
proxy et l'historique du navigateur.

### Ce qui n'est pas fait

- Pas d'authentification à deux facteurs.
- Pas de journal d'audit des actions d'administration.
- Pas de rotation automatique de `BOXINCLOUD_SECRET_KEY`.
- Le partage public repose sur un jeton de 32 octets non devinable, sans
  limitation de débit propre : un lien révoqué cesse de fonctionner, un lien
  fuité reste valable jusqu'à sa révocation.
