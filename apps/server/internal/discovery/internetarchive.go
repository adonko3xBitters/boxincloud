package discovery

import (
	"context"
	"fmt"
	"strings"
)

/*
Internet Archive.

Le seul des trois à héberger réellement des œuvres, et pas seulement à les
décrire. C'est aussi le seul dont les fiches portent des scans du domaine public
qu'on peut ensuite rapatrier — mais par son flux OPDS, pas par ici : ce
fournisseur ne fait que décrire.

# Ce que sa recherche a de particulier

L'API `advancedsearch` interroge TOUT l'Archive : des films, des concerts, des
sauvegardes de sites web. Chercher un titre d'album sans restreindre le type de
média rend surtout du bruit — une captation de conférence portant le même mot.

D'où `mediatype:texts` dans chaque requête. Ce n'est pas un filtre de confort :
sans lui, la confiance calculée devrait faire le tri toute seule sur des
candidats qui n'ont rien à voir, et elle n'a pas les moyens de distinguer un
livre d'un enregistrement sonore.

Le second piège est la forme des champs. L'Archive écrit indifféremment
`"creator": "Moebius"` ou `"creator": ["Moebius", "Jodorowsky"]`, selon que le
document en porte un ou plusieurs. Les deux formes se rencontrent dans une même
réponse, ce qui interdit de déclarer un type fixe.
*/

const internetArchiveURL = "https://archive.org"

// InternetArchive interroge archive.org.
type InternetArchive struct {
	metadataSource
}

var _ DescriptionProvider = (*InternetArchive)(nil)

func NewInternetArchive(deps MetadataDeps) *InternetArchive {
	return &InternetArchive{
		metadataSource: newMetadataSource(
			"internetarchive", "Internet Archive", internetArchiveURL, deps),
	}
}

/*
internetArchiveResponse laisse les champs en `any`.

Volontairement : l'Archive écrit une valeur seule ou un tableau selon le
document, et parfois les deux formes dans une même réponse. Déclarer `[]string`
ferait échouer le décodage sur la moitié des résultats, et déclarer `string`
sur l'autre moitié.
*/
type internetArchiveResponse struct {
	Response struct {
		Docs []struct {
			Identifier  string `json:"identifier"`
			Title       any    `json:"title"`
			Creator     any    `json:"creator"`
			Description any    `json:"description"`
			Publisher   any    `json:"publisher"`
			Language    any    `json:"language"`
			Subject     any    `json:"subject"`
			Year        any    `json:"year"`
			Date        any    `json:"date"`
		} `json:"docs"`
	} `json:"response"`
}

func (a *InternetArchive) Describe(ctx context.Context, w Work) ([]Description, error) {
	terms := strings.TrimSpace(queryFor(w))
	if terms == "" {
		return nil, nil
	}

	// Les guillemets gardent l'expression entière : sans eux, l'Archive
	// chercherait chaque mot séparément et rendrait tout document contenant
	// « le » ou « la ».
	query := fmt.Sprintf(`("%s") AND mediatype:(texts)`, strings.ReplaceAll(terms, `"`, ""))

	target := fmt.Sprintf(
		"%s/advancedsearch.php?q=%s&rows=%d&page=1&output=json"+
			"&fl%%5B%%5D=identifier&fl%%5B%%5D=title&fl%%5B%%5D=creator"+
			"&fl%%5B%%5D=description&fl%%5B%%5D=publisher&fl%%5B%%5D=language"+
			"&fl%%5B%%5D=subject&fl%%5B%%5D=year&fl%%5B%%5D=date",
		a.baseURL, escape(query), metadataLimit)

	var payload internetArchiveResponse
	if err := a.fetchJSON(ctx, target, &payload); err != nil {
		return nil, err
	}

	docs := payload.Response.Docs
	candidates := make([]Description, 0, len(docs))

	for _, doc := range docs {
		title := firstString(doc.Title)
		if title == "" {
			continue
		}

		description := Description{
			ProviderKind: a.kind,
			ProviderName: a.name,
			Title:        title,
			Authors:      allStrings(doc.Creator),
			Summary:      firstString(doc.Description),
			Publisher:    firstString(doc.Publisher),
			Language:     internetArchiveLanguage(firstString(doc.Language)),
			Subjects:     allStrings(doc.Subject),
			Published:    internetArchiveYear(doc.Year, doc.Date),
		}

		if doc.Identifier != "" {
			description.CoverURL = internetArchiveURL + "/services/img/" + doc.Identifier
			description.PageURL = internetArchiveURL + "/details/" + doc.Identifier
		}

		candidates = append(candidates, description)
	}

	return scored(w, candidates), nil
}

/*
internetArchiveYear retient une année exploitable.

`year` est propre quand il existe ; `date` est un horodatage ISO complet dont
seules les quatre premières lettres nous intéressent. Prendre `date` en entier
ferait échouer la comparaison avec l'année d'une œuvre, qui est toujours écrite
sur quatre chiffres.
*/
func internetArchiveYear(year, date any) string {
	if value := firstString(year); value != "" {
		return value
	}
	if value := firstString(date); len(value) >= 4 {
		return value[:4]
	}
	return ""
}

// internetArchiveLanguage ramène au code ISO 639-1 utilisé partout ailleurs.
//
// L'Archive écrit tantôt le code MARC, tantôt le nom complet en anglais. Les
// deux formes sont traitées ; l'inconnu ressort tel quel, une valeur
// inexploitable valant mieux qu'une valeur fausse.
func internetArchiveLanguage(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "fre", "fra", "french", "français":
		return "fr"
	case "eng", "english":
		return "en"
	case "spa", "spanish":
		return "es"
	case "ger", "deu", "german":
		return "de"
	case "ita", "italian":
		return "it"
	case "jpn", "japanese":
		return "ja"
	case "dut", "nld", "dutch":
		return "nl"
	case "por", "portuguese":
		return "pt"
	default:
		return value
	}
}
