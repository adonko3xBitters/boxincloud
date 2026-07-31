package discovery

import (
	"context"
	"fmt"
	"strings"
)

/*
Google Books.

Le fonds le mieux renseigné des trois sur les résumés, les éditeurs et les
paginations, et le seul à couvrir correctement la bande dessinée récente. C'est
ce qui justifie de l'inclure malgré ce qui suit.

# Ce qu'il coûte, et pourquoi il est facultatif

Sans clé d'API, le service fonctionne mais son quota est partagé par adresse IP
et s'épuise vite : sur une instance qui enrichit une bibliothèque entière, les
premières centaines de requêtes passent, les suivantes reçoivent 429.

Avec une clé, le quota est propre à l'instance et généreux. Mais obtenir une clé
demande un compte Google, un projet dans leur console, et l'acceptation de leurs
conditions — trois choses qu'un utilisateur auto-hébergé n'a aucune raison de
vouloir, et qu'on ne peut pas lui imposer.

D'où : **le fournisseur n'est enregistré que si une clé est configurée.** Ce
n'est pas de la prudence excessive. Un fournisseur qui échoue une fois sur deux
est pire qu'un fournisseur absent : il fait douter de la fonctionnalité entière
au lieu de manquer proprement, et l'utilisateur n'a aucun moyen de comprendre
pourquoi son enrichissement rend un album sur trois.

Sans clé, Open Library et Internet Archive couvrent l'essentiel.
*/

const googleBooksURL = "https://www.googleapis.com/books/v1"

// GoogleBooks interroge l'API Google Books.
type GoogleBooks struct {
	metadataSource
	apiKey string
}

var _ DescriptionProvider = (*GoogleBooks)(nil)

// NewGoogleBooks construit le fournisseur. La clé est obligatoire — voir plus
// haut pourquoi un fournisseur sans clé vaut moins qu'aucun fournisseur.
func NewGoogleBooks(apiKey string, deps MetadataDeps) *GoogleBooks {
	return &GoogleBooks{
		metadataSource: newMetadataSource(
			"googlebooks", "Google Books", googleBooksURL, deps),
		apiKey: apiKey,
	}
}

type googleBooksResponse struct {
	Items []struct {
		VolumeInfo struct {
			Title               string   `json:"title"`
			Subtitle            string   `json:"subtitle"`
			Authors             []string `json:"authors"`
			Publisher           string   `json:"publisher"`
			PublishedDate       string   `json:"publishedDate"`
			Description         string   `json:"description"`
			PageCount           int      `json:"pageCount"`
			Categories          []string `json:"categories"`
			Language            string   `json:"language"`
			InfoLink            string   `json:"infoLink"`
			IndustryIdentifiers []struct {
				Type       string `json:"type"`
				Identifier string `json:"identifier"`
			} `json:"industryIdentifiers"`
			ImageLinks struct {
				Thumbnail      string `json:"thumbnail"`
				SmallThumbnail string `json:"smallThumbnail"`
			} `json:"imageLinks"`
		} `json:"volumeInfo"`
	} `json:"items"`
}

func (g *GoogleBooks) Describe(ctx context.Context, w Work) ([]Description, error) {
	terms := googleBooksQuery(w)
	if terms == "" {
		return nil, nil
	}

	target := fmt.Sprintf("%s/volumes?q=%s&maxResults=%d&printType=books",
		g.baseURL, escape(terms), metadataLimit)
	if g.apiKey != "" {
		target += "&key=" + escape(g.apiKey)
	}

	var payload googleBooksResponse
	if err := g.fetchJSON(ctx, target, &payload); err != nil {
		return nil, err
	}

	candidates := make([]Description, 0, len(payload.Items))
	for _, item := range payload.Items {
		info := item.VolumeInfo

		title := info.Title
		if info.Subtitle != "" {
			title += " : " + info.Subtitle
		}

		description := Description{
			ProviderKind: g.kind,
			ProviderName: g.name,
			Title:        title,
			Authors:      info.Authors,
			Summary:      info.Description,
			Publisher:    info.Publisher,
			Published:    info.PublishedDate,
			PageCount:    info.PageCount,
			Language:     info.Language,
			Subjects:     info.Categories,
			PageURL:      info.InfoLink,
			CoverURL:     googleBooksCover(info.ImageLinks.Thumbnail, info.ImageLinks.SmallThumbnail),
		}

		// ISBN-13 de préférence : c'est la forme moderne, et la seule que les
		// catalogues récents publient.
		for _, identifier := range info.IndustryIdentifiers {
			if identifier.Type == "ISBN_13" {
				description.ISBN = identifier.Identifier
				break
			}
			if identifier.Type == "ISBN_10" && description.ISBN == "" {
				description.ISBN = identifier.Identifier
			}
		}

		candidates = append(candidates, description)
	}

	return scored(w, candidates), nil
}

/*
googleBooksQuery compose une requête dans la syntaxe de Google Books.

Elle accepte des qualificatifs — `intitle:`, `inauthor:`, `isbn:` — nettement
plus précis qu'une recherche libre. Les utiliser divise par plusieurs le bruit
sur un titre commun, ce qui est exactement ce qu'on cherche pour un
rapprochement.
*/
func googleBooksQuery(w Work) string {
	if w.ISBN != "" {
		return "isbn:" + normalizeISBN(w.ISBN)
	}

	var parts []string
	if title := strings.TrimSpace(w.Title); title != "" {
		parts = append(parts, `intitle:"`+title+`"`)
	}
	if len(w.Authors) > 0 && strings.TrimSpace(w.Authors[0]) != "" {
		parts = append(parts, `inauthor:"`+strings.TrimSpace(w.Authors[0])+`"`)
	}
	return strings.Join(parts, " ")
}

/*
googleBooksCover choisit une vignette et la sert en HTTPS.

L'API rend encore des liens en `http://` pour certains volumes. Les laisser tels
quels ferait bloquer l'image par le navigateur sur une instance servie en HTTPS
— contenu mixte — et la couverture manquerait sans que rien ne l'explique.
*/
func googleBooksCover(thumbnail, small string) string {
	cover := thumbnail
	if cover == "" {
		cover = small
	}
	return strings.Replace(cover, "http://", "https://", 1)
}
