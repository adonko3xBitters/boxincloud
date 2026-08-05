# ADR-007 — amuled est un processus séparé, et le module est désactivé par défaut

> Statut : accepté. Étape 0 du module eD2k/Kad Center.

## Contexte

boxincloud tient une promesse d'installation simple : PostgreSQL et un binaire,
deux conteneurs, `docker compose up` qui marche du premier coup. Ajouter un
démon pair-à-pair met cette promesse sous tension.

Par ailleurs, un client eD2k n'est pas un composant neutre : il ouvre des ports
entrants, publie l'adresse IP de l'instance sur deux réseaux publics, consomme
de la bande passante en permanence et occupe du disque. Une instance qui sert
une bibliothèque de BD à une famille n'a aucune raison de payer cela.

## Décision

**amuled tourne dans son propre processus**, joint par TCP sur le réseau interne.
Il n'est ni lancé par notre binaire, ni supervisé par lui : boxincloud est un
client de son API, rien de plus.

**Le module est désactivé par défaut.** `BOXINCLOUD_ED2K_ENABLED=false` est la
valeur livrée. Désactivé, le module ne se connecte à rien, n'ouvre aucun port,
ne démarre aucune boucle de scrutation, et sa page annonce clairement son état
au lieu d'échouer.

**Le déploiement se fait par un profil compose.** `docker compose --profile ed2k
up` ajoute le service ; sans le profil, le fichier se comporte comme avant.
Trois conteneurs pour qui en veut, deux pour les autres.

**Le port EC n'est jamais publié.** amuled vit sur un réseau interne, sans
`ports:` pour EC. Seuls 4662/tcp et 4672/udp — ceux dont le protocole a
réellement besoin — sont exposés. Le mot de passe EC est scellé en base avec
`BOXINCLOUD_SECRET_KEY`, comme les identifiants des backends de stockage, et ne
ressort jamais par l'API.

**Une seule instance pilote un démon.** Un verrou consultatif PostgreSQL est pris
au démarrage de la boucle de scrutation : une seconde réplique constate qu'elle
n'a pas le verrou et sert l'interface sans scruter. Sans lui, deux répliques
enverraient des commandes contradictoires au même démon, et dériveraient chacune
des événements en double.

## Conséquences

**La promesse « deux conteneurs » ne vaut plus quand le module est actif.** C'est
à écrire dans le README et le guide d'installation, pas à laisser découvrir. Le
profil compose limite la portée : elle reste vraie pour qui n'active rien.

**Se connecter à un amuled déjà installé est gratuit.** Le client EC ne fait pas
la différence entre un démon de notre compose et celui qui tourne depuis trois
ans sur le NAS. C'est même le premier cas à documenter : le public visé a
souvent déjà un aMule.

**Le cycle de vie du démon ne nous appartient pas.** Il peut être arrêté, mis à
jour ou redémarré sans nous prévenir. La couche d'intégration doit donc traiter
la déconnexion comme un état normal — reconnexion à intervalle croissant, état
visible dans l'interface — et non comme une erreur exceptionnelle.

**Le répertoire d'arrivée est monté en lecture seule côté boxincloud.** C'est
tout ce dont le pont vers la bibliothèque a besoin, et cela rend structurellement
impossible qu'un défaut de notre côté abîme la zone de travail du démon.

## Alternative écartée

**Lancer amuled comme processus fils du binaire boxincloud.** Cela aurait donné
un seul conteneur et une supervision simple, au prix de trois choses
inacceptables : une image qui embarque aMule et ses dépendances pour tout le
monde, un couplage de cycle de vie qui rend impossible de piloter un démon
existant, et une proximité avec du code GPL-2 que le dialogue par socket évite
proprement.
