# ADR-004 — Le module eD2k/Kad s'appuie sur amuled, il ne réimplémente rien

> Statut : accepté. Étape 0 du module eD2k/Kad Center.

## Contexte

boxincloud veut offrir un client eD2k et Kad accessible depuis un navigateur.
Deux voies existaient.

**Réimplémenter les protocoles en Go.** Cela demande le hash eD2k (MD4 sur des
parties de 9 500 Ko), l'AICH (arbre SHA-1 sur blocs de 180 Ko), le format
`.part`/`.part.met` et sa liste de trous, le protocole serveur en TCP et UDP,
le protocole client-à-client, le système de crédits, l'identification sécurisée
RSA, et Kademlia dans la variante d'eMule. Il n'existe aucune implémentation Go
mûre de tout cela : ce serait entièrement à écrire. L'estimation honnête est de
15 000 à 25 000 lignes, dont la moitié ne se valide que contre le réseau réel —
c'est-à-dire contre des pairs qu'on ne contrôle pas, avec des bogues qui se
manifestent en corruption silencieuse de fichiers.

**S'appuyer sur amuled**, le démon d'aMule, qui parle déjà ces protocoles depuis
vingt ans et dont l'interopérabilité est éprouvée par des millions de sessions.

## Décision

Le module n'implémente aucun protocole pair-à-pair. Il pilote **amuled** par son
protocole officiel *External Connections* (EC), et remplace `amuleweb` par une
interface moderne intégrée au projet.

Le seul protocole que nous écrivons est **EC lui-même** : environ 1 500 à 2 500
lignes de codec, testables intégralement hors ligne contre des captures
binaires, et vérifiables de bout en bout contre un vrai amuled en conteneur.

## Conséquences

**Ce que cela achète.** Le moteur est éprouvé ; les fichiers téléchargés sont
corrects parce que c'est aMule qui les vérifie ; les évolutions du réseau eD2k
sont suivies par un projet dont c'est le métier. Nous écrivons ce que nous
savons faire — une API propre et une interface — et pas ce que d'autres font
mieux depuis vingt ans.

**Ce que cela renforce, de façon inattendue : la règle n°1 du projet.**

L'approche par réimplémentation exigeait d'écrire à des offsets arbitraires dans
des fichiers partiels, depuis plusieurs sources en parallèle. `storage.Provider`
n'expose que `Write(key, io.Reader, size)`, et le stockage objet ne sait pas
écrire au milieu d'un objet. Il aurait fallu étendre l'interface centrale du
projet par une capacité `RandomWriter` — c'est-à-dire toucher la pièce que
CONTRIBUTING.md déclare non négociable, pour un module accessoire.

Avec amuled, cette pression disparaît entièrement. Le démon possède ses fichiers
partiels, sur son propre volume ; notre serveur ne voit jamais qu'un fichier
**terminé**, en lecture seule. Cela se dit avec ce qui existe déjà, sans une
ligne de modification :

```go
incoming, err := local.New(local.Options{Root: cfg.Ed2k.IncomingDir, ReadOnly: true})
```

`storage.Provider` reste intact. La règle n°1 reste vraie à la lettre.

**Ce que cela coûte.** Une dépendance externe à déployer, dont le cycle de vie
n'est pas le nôtre. Un protocole de plus à connaître. Et surtout : nous héritons
des limites d'EC, dont la plus structurante fait l'objet de [l'ADR-005](005-temps-reel-sse-evenements-derives.md)
— EC ne pousse aucun événement.

**Licence.** amuled est sous GPL-2, boxincloud sous AGPL-3.0. Le dialogue se
fait par socket entre deux processus séparés : c'est de l'agrégation, pas une
œuvre dérivée. La contrainte pratique tient en une phrase — **ne jamais lier ni
recopier du code aMule**, seulement parler son protocole. Le codec EC est écrit
d'après la spécification.

## Alternatives écartées

**Piloter `amuleweb` ou `amulecmd`.** `amuleweb` est une interface HTML à
scruter, pas une API ; en dépendre reviendrait à faire de l'analyse de page pour
lire un état que EC donne proprement. `amulecmd` lance un processus par
commande, ce qui interdit tout suivi continu.

**Réimplémenter partiellement, en déléguant seulement le transfert.** Il n'y a
pas de découpe : les crédits, les sources et la vérification sont solidaires du
transfert. Une frontière au milieu aurait donné les inconvénients des deux.
