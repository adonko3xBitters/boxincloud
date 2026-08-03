package scraper

import (
	"strings"

	"github.com/tidwall/gjson"

	"github.com/adonko3xBitters/boxincloud/server/internal/discovery"
)

/*
Sources qui répondent en JSON.

# Pourquoi ce n'est pas un second moteur

Beaucoup de catalogues légitimes n'exposent ni OPDS ni page lisible, mais une
API JSON : Gutendex pour Project Gutenberg, AniList, MangaUpdates, ComicVine.
Le moteur ne savait lire que du HTML, ce qui les mettait toutes hors de portée.

Seule l'EXTRACTION change. Miroirs et repli, limitation de débit, robots.txt,
bornes de temps et de taille, résolution des adresses, contrôle d'origine à
l'import, cache : tout cela est le même code. Un `format: json` dans le gabarit,
et c'est le seul embranchement.

C'est le même choix que pour les descriptions saisies au formulaire, et pour la
même raison : deux moteurs finissent par avoir deux jeux de bugs, et celui qu'on
regarde le moins accumule les siens en silence.

# Les chemins remplacent les sélecteurs

`results.select` devient le chemin du TABLEAU de résultats, et chaque champ un
chemin relatif à un élément :

    select: results
    fields:
      title:  { select: title }
      authors:{ select: "authors.#.name" }
      link:   { select: "formats.application/epub+zip" }

La syntaxe est celle de gjson : un point sépare les niveaux, un nombre indexe un
tableau, `#` le parcourt en entier. Écrire un analyseur maison aurait été le
genre de code qui paraît simple et se trompe sur les cas composés — clés
contenant un point, tableaux imbriqués, valeurs absentes.
*/

// extractJSON tire les résultats d'une réponse JSON.
//
// `pageURL` sert à résoudre les adresses relatives, comme en HTML. Les API en
// rendent rarement, mais rien ne l'interdit et une adresse relative non résolue
// donnerait un lien mort.
func extractJSON(
	body []byte, template *Compiled, pageURL string, limit int,
) []discovery.Result {
	if limit <= 0 || limit > template.Limits.MaxResults {
		limit = template.Limits.MaxResults
	}

	parsed := gjson.ParseBytes(body)

	/*
		Un chemin vide désigne la racine.

		Le cas est réel : certaines API rendent directement un tableau, sans
		l'envelopper dans un objet. Exiger un chemin obligerait alors à en
		inventer un.
	*/
	array := parsed
	if path := strings.TrimSpace(template.Results.Select); path != "" {
		array = parsed.Get(path)
	}
	if !array.IsArray() {
		return nil
	}

	results := make([]discovery.Result, 0, limit)

	array.ForEach(func(_, item gjson.Result) bool {
		result := discovery.Result{}
		for name, field := range template.fields {
			assignJSON(&result, name, field, item, pageURL)
		}

		if strings.TrimSpace(result.Title) != "" {
			results = append(results, result)
		}
		return len(results) < limit
	})

	return results
}

/*
assignJSON pose une valeur extraite d'un élément JSON.

Le pendant d'`assign` pour le HTML, et volontairement écrit à côté plutôt que
fusionné avec lui : les deux partagent le vocabulaire des champs, rien de leur
mécanique. Les mêler derrière une abstraction commune aurait donné une fonction
pleine de conditions sur le format, moins lisible que deux fonctions courtes.
*/
func assignJSON(
	result *discovery.Result,
	name string,
	field compiledField,
	item gjson.Result,
	pageURL string,
) {
	found := item.Get(field.Select)
	if !found.Exists() {
		return
	}

	values := jsonValues(found)
	if field.regex != nil {
		filtered := values[:0]
		for _, value := range values {
			if kept := capture(field.regex, value); kept != "" {
				filtered = append(filtered, kept)
			}
		}
		values = filtered
	}
	if len(values) == 0 {
		return
	}

	switch name {
	case fieldTitle:
		setString(&result.Title, values[0])
	case fieldSeries:
		setString(&result.Series, values[0])
	case fieldSummary:
		setString(&result.Summary, values[0])
	case fieldLanguage:
		setString(&result.Language, values[0])
	case fieldPublished:
		setString(&result.Published, values[0])
	case fieldCoverURL:
		setString(&result.CoverURL, resolve(pageURL, values[0]))
	case fieldPageURL:
		setString(&result.PageURL, resolve(pageURL, values[0]))

	case fieldAuthors:
		if len(result.Authors) == 0 {
			result.Authors = values
		}

	case fieldAcquisition:
		for _, value := range values {
			href := resolve(pageURL, value)
			if href == "" || hasAcquisition(result.Acquisitions, href) {
				continue
			}
			result.Acquisitions = append(result.Acquisitions, discovery.Link{
				Href: href,
				Type: field.MediaType,
				Rel:  "http://opds-spec.org/acquisition",
			})
		}
	}
}

/*
jsonValues aplatit ce qu'un chemin a trouvé.

Un chemin rend indifféremment une valeur, un tableau — `authors.#.name` en rend
un — ou un objet. Les trois cas se rencontrent dans une même API, et souvent
dans un même document : Gutendex donne un titre en chaîne et des auteurs en
tableau d'objets.

Les nombres et les booléens sont convertis plutôt qu'écartés : une année de
publication arrive régulièrement en entier, et l'écarter parce qu'elle n'est pas
une chaîne serait perdre une donnée qu'on a sous la main.
*/
func jsonValues(found gjson.Result) []string {
	var out []string

	appendValue := func(value gjson.Result) {
		text := strings.TrimSpace(value.String())
		if text != "" {
			out = append(out, text)
		}
	}

	if found.IsArray() {
		found.ForEach(func(_, value gjson.Result) bool {
			appendValue(value)
			return true
		})
		return out
	}

	appendValue(found)
	return out
}
