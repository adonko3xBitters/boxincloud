# Gabarits d'opérateur

Ce répertoire n'est **pas** embarqué dans le binaire. Il montre à quoi ressemble
un gabarit qui fonctionne, et sert de point de départ aux vôtres.

## L'essayer

```bash
BOXINCLOUD_SCRAPER_TEMPLATES_DIR=deploy/scraper-templates
```

Redémarrez l'instance. Dans **Configuration → Catalogues fédérés → Ajouter**, le
menu « Type de catalogue » propose désormais « Standard Ebooks » à côté de
« Flux OPDS ». L'adresse devient facultative : le gabarit déclare déjà la sienne.

## Pourquoi Standard Ebooks

Parce qu'on peut ouvrir le site dans son navigateur et comparer avec le gabarit,
ligne à ligne. C'est ce qui fait un bon exemple.

Le site publie bien un flux OPDS, mais il est **fermé** — `/feeds/opds` répond
401, réservé aux souscripteurs. Le gabarit n'est donc pas un exercice de style :
pour un lecteur sans compte, c'est la seule porte ouverte.

Il n'est pourtant pas livré dans le binaire. Un gabarit embarqué engage le
projet à le maintenir et à répondre de ce qu'il ramène ; le catalogue de
Standard Ebooks est assez bien servi par ailleurs pour ne pas le justifier.

## Ce qu'il démontre

- l'ancrage sur la **ligne** de résultat plutôt que sur la page ;
- un sélecteur qui distingue deux nœuds portant le même attribut — le titre et
  l'auteur, tous deux en `property="schema:name"` ;
- la résolution des adresses relatives, pour les fiches et les couvertures ;
- une étape `detail` : le lien de téléchargement n'existe que sur la fiche ;
- `onlyIfMissing`, qui ramène le coût à zéro le jour où la liste suffirait ;
- un `mediaType` déclaré par le gabarit, puisque le HTML ne l'annonce jamais.

Les sélecteurs sont vérifiés à chaque `go test` contre deux pages réelles
enregistrées dans `apps/server/internal/discovery/scraper/testdata/`. Un exemple
faux serait pire qu'une absence d'exemple : il apprendrait une syntaxe qui ne
marche pas.

## Écrire le vôtre

Le format est documenté dans [`docs/06-gabarits-scraper.md`](../../docs/06-gabarits-scraper.md).

Un rappel qui n'est pas une formalité : le critère d'admission vaut aussi ici. La
diffusion des œuvres doit être autorisée — domaine public, licence libre,
autorisation de l'auteur, ou accès que vous fournissez avec vos identifiants. Ce
répertoire est sous votre responsabilité, pas sous celle du projet.
