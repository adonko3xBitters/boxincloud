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

Le choix est discutable et il l'assume : ce site publie **aussi** un flux OPDS,
et pour un usage réel c'est la bonne route — un flux structuré ne casse pas
quand la mise en page change. Un gabarit se justifie quand il n'y a pas
d'alternative, ce qui est le cas des sites que le moteur vise vraiment.

C'est la raison pour laquelle il n'est pas livré dans le binaire : les gabarits
embarqués sont réservés aux sites sans autre porte d'entrée.

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
