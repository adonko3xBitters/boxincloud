# ADR-006 — La frontière EC est étanche

> Statut : accepté. Étape 0 du module eD2k/Kad Center.

## Contexte

Le protocole EC a une forme à lui : des opcodes numériques, des tags imbriqués
identifiés par des entiers, huit types de valeurs, des largeurs variables, une
compression zlib optionnelle, et un numéro de version qui change d'une release
d'aMule à l'autre.

Ces détails sont faciles à laisser fuir. Il est tentant de renvoyer au frontend
la structure décodée telle quelle : elle est déjà là, elle contient tout, et
écrire une traduction ressemble à du travail sans valeur.

C'est un piège connu, et il se referme au premier changement de version du
démon : les noms de tags apparaissent alors dans le code de l'interface, dans le
contrat OpenAPI, et jusque dans les préférences enregistrées des utilisateurs.

## Décision

**Aucun type du paquet `ec` ne franchit `amule.Service`.**

```
handlers/ed2k.go        ← ne connaît que les types du domaine
      ▼
amule.Service           ← LA frontière. mapping.go traduit ici, et nulle part ailleurs.
      ▼
amule/ec                ← opcodes, tags, paquets — confinés
      ▼
   amuled
```

Trois règles concrètes :

1. **`mapping.go` est le seul fichier autorisé à importer `ec` et à produire des
   types du domaine.** Un opcode qui apparaît ailleurs est un défaut, pas une
   optimisation.
2. **Le contrat OpenAPI ne cite jamais EC.** Ni un nom de tag, ni un numéro
   d'opcode, ni une valeur d'énumération empruntée au démon. Les états sont ceux
   du domaine — `disabled`, `unconfigured`, `disconnected`, `connecting`,
   `connected` — et c'est le mapping qui les fabrique.
3. **Le frontend ne parle jamais à amuled.** Ni directement, ni par un
   intermédiaire transparent. Il ne connaît que l'API du projet, avec ses jetons,
   ses rôles et son format d'erreur.

## Conséquences

**Une version d'aMule se change sans toucher aux handlers ni au contrat.** C'est
tout l'objet : le jour où un opcode est renommé ou un tag ajouté, un seul fichier
bouge.

**Le coût est réel et assumé** : `mapping.go` est du code de traduction, sans
intelligence, qu'il faut écrire et maintenir. C'est le prix d'une frontière, et
il se paie une fois.

**La version du protocole se négocie et se refuse clairement.** Un décodage
approximatif contre une version inconnue ne plante pas : il produit des champs
muets, ce qui est le pire des diagnostics — l'interface s'affiche, à moitié
vide, et rien ne dit pourquoi. Le client EC compare donc la version annoncée à
celle qu'il sait parler, et refuse explicitement en nommant les deux, dans la
veine de `listenFailure` : dire ce qui ne va pas, et ce qu'il faut faire.

**Le mot de passe EC n'existe pas côté domaine.** Il entre par l'API, il est
scellé, il ressort seulement vers le client EC. Aucun type exposé ne le porte,
et aucune réponse ne peut donc le divulguer par accident — la même discipline
que les identifiants des backends de stockage.
