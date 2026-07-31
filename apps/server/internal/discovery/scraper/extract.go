package scraper

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/adonko3xBitters/boxincloud/server/internal/discovery"
)

/*
Application d'un gabarit à un document.

Ce fichier ne fait AUCUNE requête et ne connaît pas le réseau. C'est ce qui rend
la partie fragile — celle qui casse quand un site change de gabarit — testable
avec une page enregistrée sur disque, sans serveur ni attente.

# Ce qui est fait ici et pas ailleurs

Trois choses, toutes du même ordre : rendre exploitable ce que le HTML n'a pas
promis d'être.

**Les adresses sont résolues.** Une page écrit `href="/dl/1234.cbz"` ; un lien
de téléchargement doit être absolu, sans quoi l'import ne saura pas où aller.

**Les lignes sans titre sont écartées**, comme dans le client OPDS et pour la
même raison : une ligne sans titre est une ligne d'en-tête ou de mise en page
que le sélecteur a attrapée au passage, pas un album.

**Rien n'est inventé.** Un champ absent reste vide. La tentation serait de
déduire — l'auteur depuis le titre, l'année depuis l'adresse — et c'est
exactement ainsi qu'on remplit une bibliothèque de métadonnées fausses mais
plausibles, qui ne se corrigent plus parce qu'elles ne se voient pas.
*/

// extractResults tire les lignes de résultat d'une page.
//
// `pageURL` est l'adresse qui a servi la page : c'est contre elle que les
// adresses relatives sont résolues, et non contre la base du gabarit. Une
// recherche redirigée vers un autre chemin casserait sinon tous ses liens.
func extractResults(
	doc *goquery.Document, template *Compiled, pageURL string, limit int,
) []discovery.Result {
	if limit <= 0 || limit > template.Limits.MaxResults {
		limit = template.Limits.MaxResults
	}

	rows := doc.FindMatcher(template.row)
	results := make([]discovery.Result, 0, rows.Length())

	rows.EachWithBreak(func(_ int, row *goquery.Selection) bool {
		result := discovery.Result{}
		for name, field := range template.fields {
			assign(&result, name, field, row, pageURL)
		}

		if strings.TrimSpace(result.Title) != "" {
			results = append(results, result)
		}
		return len(results) < limit
	})

	return results
}

/*
applyDetail complète un résultat depuis sa fiche.

N'écrase jamais ce que la liste a déjà donné, exactement comme `discovery.Enrich`
n'écrase pas un catalogue par une base de métadonnées. La règle a la même
justification : la page de liste et la fiche viennent du même site, mais c'est la
liste qui a été écrite pour être lue en série, et ses valeurs sont les plus
régulières. La fiche apporte ce qui manque — le lien de téléchargement, le
résumé — pas une seconde version de ce qu'on a déjà.

La fiche est un document ENTIER, pas une ligne : les sélecteurs de `detail` sont
donc appliqués à la racine.
*/
func applyDetail(
	result *discovery.Result, doc *goquery.Document, template *Compiled, pageURL string,
) {
	root := doc.Selection
	for name, field := range template.detail {
		assign(result, name, field, root, pageURL)
	}
}

/*
assign pose une valeur extraite dans le résultat.

Le `switch` sur le nom du champ est la seule chose qui relie le vocabulaire des
gabarits — du texte YAML — aux champs de `discovery.Result`. Le concentrer ici
plutôt que de le disperser rend visible d'un coup d'œil ce qu'un gabarit peut
remplir, et la liste fermée de `knownFields` garantit qu'aucun cas n'y manque en
silence.

Une valeur déjà remplie n'est jamais écrasée : c'est ce qui rend le suivi de
fiche non destructif sans que l'appelant ait à s'en soucier.
*/
func assign(
	result *discovery.Result,
	name string,
	field compiledField,
	scope *goquery.Selection,
	pageURL string,
) {
	values := extract(field, scope)
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
		/*
			Les liens de téléchargement s'AJOUTENT plutôt que de remplacer.

			Un site sert souvent le même album en plusieurs formats — un CBZ et
			un PDF, ou deux qualités de scan — et l'utilisateur doit pouvoir
			choisir. C'est aussi ce que fait le client OPDS d'une entrée qui
			porte plusieurs acquisitions.
		*/
		for _, value := range values {
			href := resolve(pageURL, value)
			if href == "" || hasAcquisition(result.Acquisitions, href) {
				continue
			}
			result.Acquisitions = append(result.Acquisitions, discovery.Link{
				Href: href,
				// Le type vient du gabarit : le HTML ne l'annonce pas, et le
				// deviner de l'extension serait faux dès qu'un site sert ses
				// fichiers derrière `/download.php?id=42`.
				Type: field.MediaType,
				Rel:  "http://opds-spec.org/acquisition",
			})
		}
	}
}

func setString(target *string, value string) {
	if *target == "" {
		*target = strings.TrimSpace(value)
	}
}

func hasAcquisition(links []discovery.Link, href string) bool {
	for _, link := range links {
		if link.Href == href {
			return true
		}
	}
	return false
}

/*
extract applique une règle de champ et rend les valeurs obtenues.

L'ordre des opérations est celui-ci, et il compte :

 1. sélectionner — un nœud, ou tous si `all` ;
 2. lire — texte, attribut ou HTML ;
 3. découper, si `split` ;
 4. filtrer par expression rationnelle, si `regex` ;
 5. nettoyer et écarter le vide.

Le découpage AVANT l'expression rationnelle permet d'écrire `split: ","` puis
`regex` sur chaque morceau, ce qui est le cas utile — nettoyer chaque nom d'une
liste d'auteurs. L'inverse ne servirait à rien.
*/
func extract(field compiledField, scope *goquery.Selection) []string {
	nodes := scope.FindMatcher(field.sel)
	// Le sélecteur peut aussi désigner la ligne elle-même — `select: "a"` sur
	// une ligne qui EST un lien. `Find` ne regarde que les descendants ; sans ce
	// repli, un gabarit parfaitement raisonnable ne rendrait rien.
	if nodes.Length() == 0 {
		nodes = scope.FilterMatcher(field.sel)
	}
	if nodes.Length() == 0 {
		return nil
	}
	if !field.All {
		nodes = nodes.First()
	}

	var out []string
	nodes.Each(func(_ int, node *goquery.Selection) {
		raw, ok := read(field, node)
		if !ok {
			return
		}

		pieces := []string{raw}
		if field.Split != "" {
			pieces = strings.Split(raw, field.Split)
		}

		for _, piece := range pieces {
			if field.regex != nil {
				piece = capture(field.regex, piece)
			}
			if piece = strings.TrimSpace(piece); piece != "" {
				out = append(out, piece)
			}
		}
	})

	return out
}

// read tire la valeur brute d'un nœud.
//
// Le second retour distingue « attribut absent » de « attribut vide ». La
// nuance sert : un `href` absent est une ligne mal sélectionnée, un `href=""`
// est un lien inerte, et les deux méritent d'être écartés — mais seul le
// premier signale un gabarit qui vise à côté.
func read(field compiledField, node *goquery.Selection) (string, bool) {
	switch field.From {
	case "attr":
		return node.Attr(field.Attr)
	case "html":
		raw, err := node.Html()
		if err != nil {
			return "", false
		}
		return raw, true
	default:
		return collapse(node.Text()), true
	}
}

/*
capture applique l'expression rationnelle d'un champ.

Le premier groupe capturant l'emporte quand il y en a un ; sinon, la
correspondance entière. C'est ce qui permet d'écrire `(\d{4})` pour ne garder
que l'année d'une phrase, sans avoir à décrire le reste.

Aucune correspondance rend le VIDE, pas la valeur d'origine — voir FieldSpec.
*/
func capture(re *regexp.Regexp, value string) string {
	matches := re.FindStringSubmatch(value)
	if matches == nil {
		return ""
	}
	if len(matches) > 1 && matches[1] != "" {
		return matches[1]
	}
	return matches[0]
}

/*
collapse ramène le texte d'un nœud à une ligne.

Le HTML mis en forme truffe ses nœuds de retours à la ligne et
d'indentation : `Text()` sur une cellule de tableau rend couramment
`"\n\t\t  Le titre\n\t\t"`. Sans ce repliement, ces blancs se retrouveraient
dans le titre de l'album, puis dans le nom du fichier importé.
*/
func collapse(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

/*
resolve rend absolue une adresse relative à la page qui la porte.

Un href qui reste relatif après cette étape est écarté plutôt que rendu tel
quel : il deviendrait un lien mort dans l'interface, ou pire, une adresse que
l'import tenterait de joindre en la résolvant contre lui-même.

Les schémas autres que http(s) sont refusés — `javascript:`, `data:`, `mailto:`
se rencontrent dans les pages réelles, et aucun n'a de sens comme couverture ou
comme lien de téléchargement.
*/
func resolve(pageURL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}

	target, err := url.Parse(href)
	if err != nil {
		return ""
	}

	if !target.IsAbs() {
		base, err := url.Parse(pageURL)
		if err != nil {
			return ""
		}
		target = base.ResolveReference(target)
	}

	if target.Scheme != "http" && target.Scheme != "https" {
		return ""
	}
	return target.String()
}
