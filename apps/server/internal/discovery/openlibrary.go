package discovery

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

/*
Open Library.

Le fonds le plus large et le plus ouvert des trois : données sous licence libre,
API sans clé, sans quota facturé. C'est le fournisseur par défaut, et celui vers
lequel se replier quand les autres ne rendent rien.

Sa faiblesse est connue et il faut la nommer : c'est un catalogue de LIVRES.
La bande dessinée franco-belge y est mal couverte, les séries de comics y sont
souvent regroupées sous une fiche unique, et les mangas y apparaissent sous leur
titre anglais. Pour un lecteur de BD, il complètera un résumé et une couverture
plus souvent qu'il ne donnera le bon tome.

C'est acceptable parce que l'enrichissement ne remplit que les trous, et parce
que la confiance calculée écarte les rapprochements douteux. Ce serait
inacceptable si l'on écrasait des métadonnées avec.
*/

const openLibraryURL = "https://openlibrary.org"

// OpenLibrary interroge openlibrary.org.
type OpenLibrary struct {
	metadataSource
}

var _ DescriptionProvider = (*OpenLibrary)(nil)

func NewOpenLibrary(deps MetadataDeps) *OpenLibrary {
	return &OpenLibrary{
		metadataSource: newMetadataSource(
			"openlibrary", "Open Library", openLibraryURL, deps),
	}
}

// openLibraryResponse ne déclare que les champs demandés.
//
// La liste est explicite dans l'URL — Open Library rend sinon des documents
// énormes, dont la majeure partie ne servirait qu'à être ignorée.
type openLibraryResponse struct {
	Docs []struct {
		Key              string   `json:"key"`
		Title            string   `json:"title"`
		Subtitle         string   `json:"subtitle"`
		AuthorName       []string `json:"author_name"`
		FirstPublishYear int      `json:"first_publish_year"`
		Publisher        []string `json:"publisher"`
		ISBN             []string `json:"isbn"`
		Language         []string `json:"language"`
		Subject          []string `json:"subject"`
		NumberOfPages    int      `json:"number_of_pages_median"`
		CoverID          int      `json:"cover_i"`
	} `json:"docs"`
}

func (o *OpenLibrary) Describe(ctx context.Context, w Work) ([]Description, error) {
	terms := queryFor(w)
	if strings.TrimSpace(terms) == "" {
		return nil, nil
	}

	/*
		Le paramètre `fields` n'est pas une optimisation cosmétique. Sans lui,
		une réponse de dix documents dépasse le méga-octet, dont on n'utilise
		qu'un centième — c'est de la bande passante prise à un service financé
		par des dons, pour rien.

		`isbn` en particulier est demandé alors qu'on ne l'affiche pas : c'est
		lui qui rend un rapprochement certain plutôt que probable.
	*/
	const fields = "key,title,subtitle,author_name,first_publish_year,publisher," +
		"isbn,language,subject,number_of_pages_median,cover_i"

	target := fmt.Sprintf("%s/search.json?q=%s&limit=%d&fields=%s",
		o.baseURL, escape(terms), metadataLimit, fields)

	var payload openLibraryResponse
	if err := o.fetchJSON(ctx, target, &payload); err != nil {
		return nil, err
	}

	candidates := make([]Description, 0, len(payload.Docs))
	for _, doc := range payload.Docs {
		title := doc.Title
		if doc.Subtitle != "" {
			title += " : " + doc.Subtitle
		}

		description := Description{
			ProviderKind: o.kind,
			ProviderName: o.name,
			Title:        title,
			Authors:      doc.AuthorName,
			PageCount:    doc.NumberOfPages,
			Subjects:     doc.Subject,
		}

		if doc.FirstPublishYear > 0 {
			description.Published = strconv.Itoa(doc.FirstPublishYear)
		}
		if len(doc.Publisher) > 0 {
			description.Publisher = doc.Publisher[0]
		}
		if len(doc.ISBN) > 0 {
			description.ISBN = doc.ISBN[0]
		}
		if len(doc.Language) > 0 {
			description.Language = openLibraryLanguage(doc.Language[0])
		}
		if doc.CoverID > 0 {
			// Taille M plutôt que L : c'est une vignette de proposition dans un
			// écran de correction, pas une couverture d'album.
			description.CoverURL = fmt.Sprintf(
				"https://covers.openlibrary.org/b/id/%d-M.jpg", doc.CoverID)
		}
		if doc.Key != "" {
			description.PageURL = openLibraryURL + doc.Key
		}

		candidates = append(candidates, description)
	}

	return scored(w, candidates), nil
}

/*
openLibraryLanguage ramène un code MARC à l'ISO 639-1 attendu ailleurs.

Open Library publie ses langues en MARC à trois lettres — « fre », « eng » —
là où le reste du projet, les flux OPDS compris, utilise deux lettres. Sans
cette conversion, la langue rendue par une fiche ne se comparerait à aucune
autre, et le filtrage par langue écarterait silencieusement les bons candidats.

La table couvre ce qu'une bibliothèque de bande dessinée contient réellement.
Un code inconnu ressort tel quel : mieux vaut une valeur inexploitable qu'une
valeur fausse.
*/
func openLibraryLanguage(marc string) string {
	switch strings.ToLower(marc) {
	case "fre", "fra":
		return "fr"
	case "eng":
		return "en"
	case "spa":
		return "es"
	case "ger", "deu":
		return "de"
	case "ita":
		return "it"
	case "jpn":
		return "ja"
	case "dut", "nld":
		return "nl"
	case "por":
		return "pt"
	default:
		return marc
	}
}
