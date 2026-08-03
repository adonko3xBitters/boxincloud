# Gabarits de scraping

Certains sites du domaine public n'ont ni API REST, ni flux OPDS, ni export
d'aucune sorte. Ils publient des scans depuis vingt ans, dans du HTML écrit à la
main. Le client OPDS ne peut rien pour eux.

`internal/discovery/scraper` lit ces sites-là, piloté par des **gabarits
déclaratifs** : le code sait parcourir un document, les fichiers YAML savent où
regarder.

---

## Ce que ce mécanisme ne change pas

La liste des sites reste fermée et soumise au critère d'admission de
[la feuille de route](04-roadmap.md) : la diffusion des œuvres doit être
autorisée — domaine public, licence libre, autorisation de l'auteur, ou accès
fourni par l'utilisateur avec ses propres identifiants.

Un moteur de parsing configurable ne déplace pas cette frontière. Il évite
d'écrire du Go pour chaque site qui la respecte, rien de plus. Les gabarits
livrés sont embarqués dans le binaire et revus comme du code ; un opérateur peut
en charger d'autres depuis `BOXINCLOUD_SCRAPER_TEMPLATES_DIR`, sous sa
responsabilité, et cette porte est fermée par défaut.

---

## Pourquoi du déclaratif

Un site change de mise en page deux ou trois fois par an, sans prévenir. Si
chaque site est un fichier Go, chacun de ces changements est un correctif, une
revue, une version, une image Docker et une mise à jour chez tous les
utilisateurs — pour deux caractères dans un sélecteur CSS.

En déclaratif, c'est un fichier de données. Il se corrige sans connaître Go, se
teste contre une page enregistrée, et se remplace sur une instance sans attendre
la prochaine version.

Le prix est réel : **le gabarit n'est plus vérifié par le compilateur.** C'est
pourquoi la validation au chargement est aussi sévère, et pourquoi une clé
inconnue est une erreur plutôt qu'un silence. Un gabarit qui se charge mais ne
rend rien serait le pire des échecs — il ressemble à une recherche
infructueuse.

### Pourquoi CSS plutôt que XPath

XPath est plus puissant : il sait remonter au parent. Cette puissance sert
surtout à écrire des chemins positionnels — `div[3]/table/tr[2]` — qui sont
exactement ceux qui cassent au premier remaniement.

CSS l'emporte parce que c'est la langue dans laquelle les sélecteurs se lisent
déjà : on ouvre les outils de développement de son navigateur, on copie le
sélecteur qu'ils donnent, c'est du CSS. Ne pas savoir remonter pousse d'ailleurs
à ancrer les gabarits sur la **ligne** de résultat, ce qui les rend plus
robustes.

---

## Un gabarit, de bout en bout

```yaml
id: comicshelf                    # devient le genre de source `scraper:comicshelf`
name: Comic Shelf
homepage: https://comicshelf.example
license: Domaine public

# Essayés dans l'ordre. L'URL enregistrée pour la source passe devant.
mirrors:
  - https://comicshelf.example
  - https://mirror.comicshelf.example

rate:                             # débit sortant, PAR HÔTE
  every: 1s
  burst: 2

limits:
  timeout: 5s                     # une requête
  budget: 20s                     # la recherche entière, fiches comprises
  maxBytes: 2097152
  maxResults: 25
  followDetail: 5

search:
  method: GET                     # ou POST, pour les formulaires anciens
  path: /search
  query:
    q: "{terms}"
    per_page: "{limit}"
  probe: comic                    # terme d'essai — doit toujours rendre quelque chose

results:
  select: "ul.results > li.issue" # le sélecteur de LIGNE
  fields:
    title:      {select: "h3 a"}
    authors:    {select: "span.author", all: true}
    series:     {select: "span.series"}
    published:  {select: "span.meta", regex: '(\d{4})'}
    coverUrl:   {select: "img.cover", from: attr, attr: src}
    pageUrl:    {select: "h3 a", from: attr, attr: href}

detail:                           # facultatif : une requête par fiche
  from: pageUrl
  onlyIfMissing: [acquisition]    # ne suivre que les lignes sans lien
  fields:
    acquisition: {select: "a.download", from: attr, attr: href,
                  mediaType: application/vnd.comicbook+zip}
    summary:     {select: "div.synopsis"}
```

Ce gabarit vit dans
`apps/server/internal/discovery/scraper/testdata/comicshelf.yaml`, et la suite
de tests l'exécute de bout en bout contre un serveur d'essai — il ne peut donc
pas dériver de ce que le moteur sait faire.

---

## Le sélecteur de ligne

`results.select` désigne **la ligne, pas la page**. C'est la décision qui rend
un gabarit robuste.

Extraire chaque champ par un sélecteur global donnerait trois listes parallèles
— titres, liens, couvertures — qu'il faudrait apparier par position, et un
résultat sans couverture décalerait tout le reste d'un cran. En ancrant sur la
ligne, chaque champ est cherché **dans sa ligne**, et un champ absent est
simplement vide.

Une ligne sans titre est écartée : c'est presque toujours un en-tête que le
sélecteur a attrapé au passage.

---

## Format JSON

`format: json` dans le gabarit, et `select` cesse d'être un sélecteur CSS pour
devenir un **chemin** : un point sépare les niveaux, un nombre indexe un
tableau, `#` le parcourt en entier.

```yaml
format: json
results:
  select: response.docs          # le chemin du TABLEAU
  fields:
    title:   {select: title}
    authors: {select: creator}   # chaîne ou tableau — aplati dans les deux cas
    cover:   {select: "formats.image/jpeg"}
```

Un `select` vide à la racine désigne le document lui-même : certaines API
rendent un tableau nu, et exiger un chemin obligerait à en inventer un.

`from`, `attr` et `split` n'ont pas de sens ici et sont refusés à la validation
plutôt qu'ignorés — un gabarit qui se charge sans rien remplir est la panne la
plus coûteuse à diagnostiquer.

Seule l'extraction change. Miroirs, repli, limitation de débit, `robots.txt`,
bornes de temps et de taille, contrôle d'origine à l'import : même code. Deux
moteurs finiraient par avoir deux jeux de bugs, et celui qu'on regarde le moins
accumulerait les siens en silence.

### Ce qui manque encore en JSON

Le **suivi de fiche** n'existe qu'en HTML. Une API qui rend un identifiant sans
lien de téléchargement — Internet Archive, par exemple — donne donc des
résultats consultables mais pas importables.

Les **clés répétées** ne sont pas exprimables : `query` est une carte, une
valeur par clé. Une API qui exige `fl[]=a&fl[]=b` ne peut pas être décrite
telle quelle.

## En-têtes et authentification

`search.authHeader` nomme l'en-tête qui portera le secret de la source. Le
gabarit dit **où** mettre la clé, la source dit **laquelle** : le secret vit
chiffré en base, comme le mot de passe d'un catalogue OPDS, et ne traverse
jamais un fichier — un gabarit est souvent versionné, parfois partagé.

```yaml
search:
  authHeader: Authorization      # ou X-API-Key, selon l'API
```

Une réponse obtenue avec un secret n'est **jamais mise en cache** : deux comptes
peuvent voir deux catalogues différents à la même adresse.

`search.headers` accepte le reste — `Accept`, `Accept-Language`, un en-tête
maison. Quatre sont **refusés à la validation** : `User-Agent`, `Referer`,
`Origin`, `Host`. Ils ne servent qu'à mentir sur qui appelle et d'où, c'est-à-dire
à franchir un contrôle qui vous refuse. Le refus est dans le code : une règle
qu'on peut contourner en écrivant deux lignes de YAML n'est pas une règle.

## Champs reconnus

La liste est fermée, et vérifiée au chargement : une faute de frappe dans un nom
de champ est une erreur, pas une colonne vide découverte six mois plus tard.

| Champ | Devient |
|---|---|
| `title` | **requis** — le titre du résultat |
| `authors` | la liste d'auteurs |
| `series` | la série |
| `summary` | le résumé |
| `language` | la langue |
| `published` | la date ou l'année |
| `coverUrl` | la couverture (adresse résolue) |
| `pageUrl` | la fiche sur le site (adresse résolue) |
| `acquisition` | un lien de téléchargement (adresses résolues, cumulées) |

### Règles d'extraction

| Clé | Effet |
|---|---|
| `select` | sélecteur CSS, relatif à la ligne. **Requis.** |
| `from` | `text` (défaut), `attr` ou `html` |
| `attr` | nom de l'attribut, requis avec `from: attr` |
| `all` | retient tous les nœuds au lieu du premier — plusieurs auteurs |
| `split` | découpe la valeur — `"Hanks, Wolverton"` en deux |
| `regex` | extrait un morceau ; le 1er groupe capturant l'emporte |
| `mediaType` | type MIME du lien, réservé à `acquisition` |

L'ordre est : sélectionner → lire → découper → filtrer → nettoyer. Le découpage
**avant** l'expression rationnelle permet de nettoyer chaque élément d'une liste,
ce qui est le cas utile ; l'inverse ne servirait à rien.

Une valeur qui ne correspond pas au `regex` devient **vide**, pas inchangée : un
gabarit qui déclare une extraction et ne l'obtient pas a trouvé autre chose que
ce qu'il croyait.

Les schémas autres que `http(s)` sont écartés — `javascript:`, `data:`,
`mailto:` se rencontrent dans les pages réelles, et aucun n'est une couverture.

---

## Miroirs et résilience

Un site du domaine public change de domaine, tombe, ou survit dans un miroir.
Trois mécanismes s'empilent :

1. **`mirrors`** — les bases du gabarit, essayées dans l'ordre déclaré ;
2. **l'URL de la source** — saisie par un administrateur, elle passe **devant**
   toutes les autres. C'est la réponse au « le domaine a changé » : on modifie
   l'adresse dans la configuration, sans recompiler ;
3. **le repli automatique**, sur ce qui ressemble à une panne.

Le repli se déclenche sur : connexion refusée, `5xx`, `429`, `408`.

Il ne se déclenche **pas** sur `404` ou `403`. Ce sont des réponses : le site a
compris la demande et l'a refusée. La reposer à chaque miroir ferait N requêtes
pour apprendre N fois la même chose — exactement le comportement qui fait
remarquer un client.

Une interdiction par `robots.txt` arrête tout : les miroirs d'un site publient
la même politique.

---

## Politesse

Trois choses qu'un scraper correct fait, et qu'on oublie.

**Espacer ses requêtes.** Un compartiment de débit **par hôte**, pas par
gabarit : deux miroirs sont deux machines, et les compter ensemble punirait l'un
pour l'autre. Quand deux gabarits partagent un hôte, le débit le plus prudent
l'emporte — c'est cette machine-là qui encaisse.

**Lire `robots.txt`.** Ce n'est pas une obligation légale, c'est la frontière que
le site a publiée. Sont implémentés : les groupes `User-agent`, `Allow`,
`Disallow`, les jokers `*` et l'ancre `$`, la règle du chemin le plus long, et
`Allow` qui l'emporte à longueur égale. Un fichier absent ou illisible
**autorise** — l'inverse ferait d'une panne réseau un refus définitif.

Le groupe qui nomme `boxincloud` **remplace** le groupe `*`, il ne s'y ajoute
pas. Les cumuler ferait obéir à des règles que le site a explicitement
remplacées.

**Renoncer.** `limits.budget` borne la recherche entière, suivi des fiches
compris. Sans elle, dix fiches à huit secondes feraient quatre-vingts secondes
d'attente sans qu'aucun délai unitaire ne soit dépassé.

### Déroger, sans se déguiser

`ignoreRobots: true` — dans un gabarit, ou la case correspondante du formulaire
— désactive la lecture de `robots.txt` **pour cette source**. Faux par défaut.

L'option existe parce que l'administrateur a souvent autorité sur la cible :
l'intranet qu'il opère, un site partenaire, ou un `Disallow: /search` posé
contre les moissonneurs et non contre une requête qu'un humain vient de taper.

Ce qu'elle ne change pas, et c'est le point : **l'agent reste `boxincloud`**.
Passer outre un avis consultatif en restant identifiable n'est pas se faire
passer pour quelqu'un d'autre — le site peut toujours refuser par agent ou par
adresse, et son refus sera alors sans ambiguïté. Chaque requête concernée est
journalisée, parce qu'une dérogation silencieuse serait pire que pas de
dérogation.

Elle ne rend rien licite qui ne l'était pas. Le critère d'admission ne bouge
pas.

### L'agent n'est pas configurable

boxincloud s'annonce, avec l'adresse de son dépôt, et un gabarit ne peut pas le
changer.

Autoriser un agent arbitraire, ce serait livrer le moyen de se faire passer pour
un navigateur — l'outil de contournement d'un blocage, c'est-à-dire l'exact
inverse de ce que ce paquet cherche à faire. Un site qui refuse boxincloud a le
droit de le refuser ; la réponse correcte est de retirer le gabarit ou d'écrire
à son administrateur, pas de se déguiser.

---

## Suivre les fiches, sans se faire bloquer

Beaucoup de sites ne publient le lien de téléchargement que sur la fiche. C'est
une requête **par résultat** — le comportement qui fait bloquer un client.

Trois bornes se cumulent :

- `onlyIfMissing` ne suit que les lignes auxquelles il manque quelque chose. Sur
  un site qui publie déjà tout dans sa liste, le coût tombe à zéro ;
- `limits.followDetail` borne le nombre de fiches ;
- le budget du contexte arrête tout, y compris au milieu.

Le suivi est **séquentiel**. Le paralléliser diviserait l'attente et
multiplierait d'autant la charge imposée au site. Un site associatif mérite
qu'on lui parle une requête à la fois.

Une fiche ne remplace jamais ce que la liste a déjà donné : elle apporte ce qui
manque. La liste a été écrite pour être lue en série, ses valeurs sont les plus
régulières.

---

## Comment une source désigne son gabarit

Par son **genre** : `kind = "scraper:comicshelf"` dans `discovery_sources`. La
colonne existe déjà et est déjà la clé de routage — aucune migration.

Ce n'est pas une économie : deux sites lus au gabarit n'ont rien en commun à
l'exécution, ni adresse, ni règles, ni débit. Ce sont bien deux genres de
catalogue, pas deux configurations d'un même genre.

L'`url` de la source porte la **base active**, et peut rester vide : le gabarit
fournit alors ses miroirs.

### Côté API

| Route | Effet |
|---|---|
| `GET /discovery/scraper-templates` | les gabarits chargés — `kind`, `id`, `name`, `homepage`, `license`, `mirrors` |
| `POST /discovery/sources` | accepte `kind` (`opds` par défaut) ; `url` facultative sur un gabarit |
| `PATCH /discovery/sources/{id}` | `url` facultative ; `kind` **non modifiable** |

Toutes réservées aux administrateurs.

Le `kind` d'un gabarit est rendu **composé** (`scraper:comicshelf`) et se recopie
tel quel dans la création. La convention de nommage appartient au serveur ; une
interface qui la reconstruirait par concaténation se casserait le jour où elle
change.

Changer le `kind` d'une source existante n'est pas permis : il identifie le
protocole ou le gabarit, et en changer ferait une autre source. Il faut la
supprimer et la recréer.

Un `kind` explicite mais inconnu de l'instance est **refusé à la saisie** (422).
Sans ce contrôle, la source serait enregistrée puis traitée par le client OPDS,
qui demanderait un flux Atom à une page HTML — un échec tardif dont le message ne
dirait rien de la vraie cause. Le même cas rencontré plus tard — un gabarit
retiré du répertoire d'un opérateur alors que des sources s'en servent — rend le
code d'erreur stable `unknown-kind`.

### Contrôle d'origine

L'import exige normalement que l'adresse téléchargée partage l'hôte de la
source. Cette règle, taillée pour OPDS, est trop étroite ici : un site sert ses
pages depuis un hôte et ses fichiers depuis un autre.

Le client implémente donc `discovery.OriginChecker` et répond à partir de la
liste d'hôtes **déclarée par le gabarit**, plus celui saisi par l'administrateur.
Ce qui est élargi reste fermé : rien n'y entre à l'exécution, et une redirection
vers un hôte tiers ne l'ouvre pas.

---

## Un exemple qui fonctionne

`deploy/scraper-templates/standardebooks.yaml` vise un site réel et admissible.
Pointez `BOXINCLOUD_SCRAPER_TEMPLATES_DIR` dessus, redémarrez, et il apparaît
dans le menu « Type de catalogue ».

Ses sélecteurs sont rejoués à chaque `go test` contre deux pages enregistrées du
site — un exemple faux apprendrait une syntaxe qui ne marche pas, ce qui est
pire que pas d'exemple du tout.

Il démontre le cas qui piège tout le monde : sur ce site, le titre et l'auteur
portent le même attribut `property="schema:name"`, et seul leur paragraphe les
distingue. Un sélecteur naïf met l'auteur dans le titre une fois sur deux, sans
que rien ne le signale.

## Écrire un gabarit

1. Ouvrez la page de résultats du site dans un navigateur, outils de
   développement ouverts.
2. Trouvez le **conteneur d'une ligne** — celui qui se répète. C'est
   `results.select`.
3. Pour chaque champ, sélectionnez le nœud à l'intérieur de cette ligne et
   copiez le sélecteur. Préférez une classe à une position.
4. Choisissez `search.probe` : un terme qui rend **toujours** des résultats sur
   ce site.
5. Déposez le fichier dans `BOXINCLOUD_SCRAPER_TEMPLATES_DIR`, redémarrez, et
   ajoutez la source. L'enregistrement déclenche un essai.

L'essai est la partie qui compte : il vérifie que le site répond **et** que le
gabarit y trouve encore des lignes. Sans lui, un site refait sa mise en page,
répond parfaitement, et la panne se déguise en « aucun résultat » — le mode
d'échec le plus coûteux à diagnostiquer.

---

## Ce qui est livré aujourd'hui

**Aucun gabarit**, et la raison est documentée dans
`apps/server/internal/discovery/scraper/templates/README.md`. Les deux sites
visés par la feuille de route ont été vérifiés en juillet 2026 :

- **Digital Comic Museum** répond `403` à toute requête, y compris sa page
  d'accueil, dès lors que le client n'est pas un navigateur — un défi Cloudflare.
  Son `robots.txt` nous autorise pourtant (`User-agent: *`, `Allow: /`) : le
  blocage vient de l'infrastructure, pas d'une politique du site. Le contourner
  demanderait de se déguiser en navigateur ; c'est refusé.
- **Comic Book Plus** est joignable et son `robots.txt` est raisonnable, mais sa
  recherche est un Google Custom Search côté navigateur : il n'existe aucun point
  d'entrée à interroger sur le serveur.

Le moteur est donc en place et éprouvé, et attend un site admissible qui soit
lisible. En attendant, il sert aux gabarits d'opérateur.
