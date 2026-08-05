# Module eD2k / Kad

> Un client eD2k et Kad accessible depuis le navigateur, adossé au démon aMule.

Le module pilote un **démon aMule** par son protocole *External Connections*. Il
ne réimplémente aucun protocole pair-à-pair : le hash eD2k, l'AICH, les fichiers
partiels, le protocole serveur, le client-à-client, les crédits et Kademlia sont
l'affaire d'amuled, qui les parle depuis vingt ans.

Ce que boxincloud apporte : une interface moderne, une API propre, et un pont
qui fait d'un téléchargement terminé un album de la bibliothèque.

Les décisions structurantes sont dans [`docs/adr/`](adr/) — quatre ADR, de 004 à
007.

## Activer le module

**Deux gestes, et il en faut deux.**

```bash
docker compose --profile ed2k up -d     # ajoute le démon
BOXINCLOUD_ED2K_ENABLED=true            # dit à boxincloud de le piloter
```

Le profil ajoute le conteneur ; la variable branche le module. L'un sans l'autre
ne sert à rien — et le module désactivé répond quand même : sa page dit son état
au lieu d'échouer.

Puis, dans l'interface : menu du compte → **eD2k / Kad** → **Paramètres**, où se
déclarent l'adresse du démon et son mot de passe.

> **Le mot de passe déclaré ici est celui EN CLAIR.** Dans `amule.conf`,
> `ECPassword` n'est pas le mot de passe mais son empreinte MD5. C'est la
> confusion la plus fréquente, et `boxincloudctl ed2k ping` la nomme
> explicitement quand elle se produit.

### Piloter un aMule déjà installé

Gratuit : le client EC ne fait pas la différence entre le démon du compose et
celui qui tourne depuis trois ans sur le NAS. Il suffit d'en donner l'adresse.
Dans ce cas, ne lancez pas le profil `ed2k`.

Le démon doit avoir, dans son `amule.conf` :

```ini
[ExternalConnect]
AcceptExternalConnections=1
ECPort=4712
ECPassword=<empreinte MD5 du mot de passe>
```

## Ce que le module fait

| Écran | Contenu |
|---|---|
| **Téléchargements** | file, progression, sources par fichier, pause, reprise, priorité, annulation |
| **Recherche** | serveurs eD2k, recherche globale, Kad ; mise en file d'un résultat |
| **Envois** | transferts sortants et file d'attente des pairs |
| **Partagés** | bibliothèque partagée, demandes, servis, octets envoyés |
| **Serveurs** | liste, connexion, déconnexion, ping, utilisateurs, échecs |
| **Statistiques** | débits, plafonds, taille des deux réseaux, état de joignabilité |
| **Bibliothèque** | destinations par catégorie, et ce que le pont a fait |
| **Journal** | le journal d'exploitation du démon |

Le tout en temps réel : un flux Server-Sent Events porte les changements d'état,
et les tableaux se rafraîchissent à leur propre cadence.

## Le pont vers la bibliothèque

C'est ce qui justifie ce module **dans ce projet**. Un fichier terminé peut
rester sur disque, ou entrer dans une bibliothèque boxincloud et devenir un
album indexé, lisible depuis le navigateur et depuis l'application Android.

**La catégorie décide.** Page **Bibliothèque**, une règle par catégorie du
démon :

```
#0  Défaut   → laisser sur disque
#1  BD       → bibliothèque « Comics », dossier « Ajouts »
#2  Linux    → laisser sur disque
```

Une catégorie sans règle laisse ses fichiers où ils sont. C'est le défaut, et le
bon comportement pour tout ce qui n'est pas une bande dessinée.

L'historique des publications figure sur le même écran : ce qui a été publié, ce
qui est resté sur disque, et ce qui a échoué — avec la raison.

## Diagnostic

```bash
make ctl ARGS="ed2k ping"
```

Joint le démon, s'authentifie, mesure l'aller-retour, et dit lequel des quatre
points a lâché : adresse, mot de passe, version du protocole, réseau.

### « Le téléchargement est fini mais rien n'arrive dans la bibliothèque »

Trois causes, dans l'ordre de fréquence :

1. **Le volume n'est pas partagé.** boxincloud lit le répertoire d'arrivée du
   démon ; sans montage commun, il ne voit rien. L'historique des publications
   le dit alors explicitement, en nommant `BOXINCLOUD_ED2K_INCOMING_DIR`.
2. **La catégorie n'a pas de destination.** Le fichier est inscrit comme
   « sur disque », ce qui est le comportement voulu par défaut.
3. **La bibliothèque visée a été supprimée.** La règle disparaît avec elle.

### « Je trouve très peu de sources »

Regardez la page **Statistiques**. Un **LowID** ou un **Kad derrière pare-feu**
signifient que les connexions entrantes n'arrivent pas : le client reste
utilisable mais dépend d'intermédiaires.

La cause est presque toujours l'absence de redirection des ports 4662/tcp et
4672/udp vers la machine. **Ce n'est pas un défaut de boxincloud ni d'aMule** :
c'est une propriété du réseau eD2k, et eMule vit avec depuis vingt ans.

### « Mes résultats de recherche ont disparu »

Le démon ne tient **qu'une recherche à la fois** : en démarrer une seconde
efface la première. Deux personnes qui cherchent en même temps se marchent
dessus. C'est une conséquence du fait qu'amuled est un moteur unique partagé, et
non un choix qu'on pourrait assouplir.

## Ce qu'il faut savoir avant d'activer

**Trois conteneurs au lieu de deux.** La promesse d'installation du projet ne
vaut plus quand le module est actif. Le profil compose limite la portée : elle
reste vraie pour qui n'active rien.

**Des ports entrants.** 4662/tcp et 4672/udp doivent être joignables pour
obtenir un HighID. Le port EC, lui, n'est jamais publié — il donne le pilotage
complet du démon.

**Un moteur unique, partagé.** amuled n'a aucune notion d'utilisateur : une
seule file, un seul jeu de préférences. Le module est réservé aux
administrateurs, et ce qui ressemblerait à du multi-utilisateurs serait une
couche d'autorisation, jamais d'isolation.

**Licence.** amuled est sous GPL-2, boxincloud sous AGPL-3.0. Le dialogue se
fait par socket entre deux processus séparés : c'est de l'agrégation. Le codec
EC de ce projet est écrit d'après la spécification, sans reprendre de code aMule.

## Pour le contributeur

```
apps/server/internal/amule/
  ec/              codec External Connections — paquets, tags, poignée de main
  types.go         les types du domaine : c'est là que le protocole s'arrête
  mapping_*.go     traduction EC → domaine, le SEUL endroit qui connaisse les tags
  poller.go        scrutation à cadence adaptative, instantanés
  events.go        dérivation des événements par comparaison d'instantanés
  commands.go      ce qui agit sur le démon
  search.go        recherche
  bridge.go        le pont vers la bibliothèque
```

Les tests d'intégration tournent contre un **vrai amuled** en conteneur, démarré
hors ligne en une seconde et demie (`internal/testsupport/amuletest`). C'est la
même doctrine que pour le stockage : un protocole ne se teste pas avec des
doublures, qui ne feraient que confirmer nos propres suppositions.
