package discovery

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/netguard"
)

/*
Client OPDS.

Deux versions du format coexistent dans la nature et il faut savoir lire les
deux. OPDS 1.2 est de l'Atom — c'est ce que servent Calibre-Web, Standard
Ebooks, Gutenberg et l'immense majorité des bibliothèques publiques. OPDS 2.0
est du JSON, servi par Komga, Kavita et les implémentations récentes.

La bascule se fait sur le type de contenu de la réponse plutôt que sur une
option de configuration : un administrateur qui ajoute son Komga colle l'URL
qu'il a sous les yeux, il n'a pas à savoir quelle version elle parle.

# Trouver la recherche

Un catalogue OPDS n'a pas d'URL de recherche conventionnelle. Il l'annonce, et
de trois façons selon l'implémentation :

  - un lien `rel="search"` vers un document OpenSearch, qui contient lui-même le
    gabarit — c'est la voie normative en OPDS 1.2, et la plus répandue ;
  - un lien `rel="search"` dont le href EST le gabarit, avec `{searchTerms}` ;
  - en OPDS 2.0, un lien `rel="search"` marqué `templated`, avec la syntaxe
    d'expansion de l'RFC 6570 : `{?query}`.

Les trois sont gérées. Un catalogue qui n'annonce rien retourne ErrNoSearch,
qui se dit à l'utilisateur — c'est une information, pas une panne.
*/

const (
	// maxFeedBytes borne ce qu'on accepte de lire d'un catalogue distant.
	//
	// Un flux OPDS de résultats fait quelques dizaines de kilo-octets. La borne
	// n'existe pas contre les gros catalogues, elle existe contre l'hôte qui
	// répondrait un flux sans fin à un serveur qui lui fait confiance.
	maxFeedBytes = 8 << 20

	// defaultTimeout est le temps laissé à un catalogue pour répondre.
	//
	// Court volontairement : la recherche est fédérée, et un catalogue lent ne
	// doit pas retenir la page entière. Mieux vaut le déclarer injoignable et
	// montrer les autres.
	defaultTimeout = 8 * time.Second

	// defaultLimit borne les résultats retenus par catalogue.
	defaultLimit = 40
)

// Relations OPDS, telles qu'elles apparaissent dans les flux.
const (
	relSearch      = "search"
	relImage       = "http://opds-spec.org/image"
	relThumbnail   = "http://opds-spec.org/image/thumbnail"
	relAcquisition = "http://opds-spec.org/acquisition"
	relAlternate   = "alternate"
)

// OPDSClient interroge un catalogue OPDS 1.2 ou 2.0.
type OPDSClient struct {
	http    *http.Client
	timeout time.Duration
}

var _ Client = (*OPDSClient)(nil)

func NewOPDSClient() *OPDSClient {
	return &OPDSClient{
		http: &http.Client{
			// Les redirections sont suivies, mais pas indéfiniment : une
			// boucle de redirection sur un catalogue distant ne doit pas
			// occuper un worker jusqu'au bout du délai.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("trop de redirections")
				}
				// L'adresse d'arrivée est contrôlée comme celle de départ :
				// sinon une redirection suffirait à contourner le garde-fou.
				return netguard.Check(req.URL.String())
			},
		},
		timeout: defaultTimeout,
	}
}

// Probe vérifie qu'un catalogue répond et expose une recherche.
//
// Appelé quand l'administrateur enregistre le catalogue : une URL fautive se
// signale à la saisie, pas trois semaines plus tard au milieu d'une recherche
// qui ne rend rien.
func (c *OPDSClient) Probe(ctx context.Context, source Source, password string) error {
	_, err := c.searchTemplate(ctx, source, password)
	return err
}

/*
searchEndpoint est un gabarit d'URL et l'adresse contre laquelle le résoudre.

Les deux voyagent ensemble parce que l'ordre des opérations compte : un gabarit
d'expansion de formulaire — `search{?query}` — contient un point d'interrogation
une fois rempli, mais pas avant. Le résoudre d'abord ferait analyser `{?query}`
comme une chaîne de requête, et l'URL ressortirait coupée en deux.

On remplit, puis on résout.
*/
type searchEndpoint struct {
	template string
	base     string
}

// Search interroge le catalogue et retourne ses résultats, normalisés.
func (c *OPDSClient) Search(
	ctx context.Context, source Source, password string, q Query,
) ([]Result, error) {
	if strings.TrimSpace(q.Text) == "" {
		return nil, nil
	}
	limit := q.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	endpoint, err := c.searchTemplate(ctx, source, password)
	if err != nil {
		return nil, err
	}

	target := resolve(endpoint.base, expandTemplate(endpoint.template, q.Text))

	body, contentType, err := c.fetch(ctx, target, source, password)
	if err != nil {
		return nil, err
	}

	results, err := parseFeed(body, contentType, target)
	if err != nil {
		return nil, err
	}

	for i := range results {
		results[i].SourceID = source.ID
		results[i].SourceName = source.Name
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

/*
searchTemplate découvre l'URL de recherche du catalogue.

Le résultat n'est pas mis en cache, et c'est un choix : la découverte coûte une
requête, la recherche fédérée n'est pas un chemin chaud, et un cache introduirait
la question de son invalidation quand un catalogue change d'adresse. Le jour où
ce coût se voit, le cache se pose ici et nulle part ailleurs.
*/
func (c *OPDSClient) searchTemplate(
	ctx context.Context, source Source, password string,
) (searchEndpoint, error) {
	if err := netguard.Check(source.URL); err != nil {
		return searchEndpoint{}, fmt.Errorf("%w : %w", ErrInvalidSource, err)
	}

	body, contentType, err := c.fetch(ctx, source.URL, source, password)
	if err != nil {
		return searchEndpoint{}, err
	}

	if isJSONFeed(contentType, body) {
		template, err := searchTemplateOPDS2(body)
		return searchEndpoint{template: template, base: source.URL}, err
	}

	href, direct, err := searchLinkOPDS1(body)
	if err != nil {
		return searchEndpoint{}, err
	}
	if direct {
		return searchEndpoint{template: href, base: source.URL}, nil
	}

	// Le lien désigne un document OpenSearch : c'est lui qui porte le gabarit.
	// Celui-ci ne contient pas de marqueur, il peut donc être résolu tout de
	// suite — et il doit l'être, c'est une URL à joindre.
	descriptionURL := resolve(source.URL, href)
	description, _, err := c.fetch(ctx, descriptionURL, source, password)
	if err != nil {
		return searchEndpoint{}, fmt.Errorf("document OpenSearch illisible : %w", err)
	}

	template, err := openSearchTemplate(description)
	return searchEndpoint{template: template, base: descriptionURL}, err
}

// fetch lit une URL du catalogue, en bornant le temps et la taille.
func (c *OPDSClient) fetch(
	ctx context.Context, target string, source Source, password string,
) ([]byte, string, error) {
	if err := netguard.Check(target); err != nil {
		return nil, "", fmt.Errorf("%w : %w", ErrInvalidSource, err)
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", fmt.Errorf("%w : %w", ErrInvalidSource, err)
	}

	// L'ordre annonce la préférence : le JSON d'abord pour les catalogues qui
	// savent les deux, l'Atom ensuite, le reste en dernier recours.
	req.Header.Set("Accept", strings.Join([]string{
		"application/opds+json",
		"application/atom+xml;profile=opds-catalog",
		"application/atom+xml",
		"application/xml;q=0.8",
		"*/*;q=0.5",
	}, ","))
	req.Header.Set("User-Agent", "boxincloud")

	if source.Username != "" {
		req.SetBasicAuth(source.Username, password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("catalogue injoignable : %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("le catalogue a répondu %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes))
	if err != nil {
		return nil, "", fmt.Errorf("lecture du catalogue : %w", err)
	}

	return body, resp.Header.Get("Content-Type"), nil
}

// ─── OPDS 1.2 : Atom ─────────────────────────────────────────────────────────

type atomLink struct {
	Rel   string `xml:"rel,attr"`
	Type  string `xml:"type,attr"`
	Href  string `xml:"href,attr"`
	Title string `xml:"title,attr"`
}

type atomEntry struct {
	Title   string `xml:"title"`
	Summary string `xml:"summary"`
	Content string `xml:"content"`
	Authors []struct {
		Name string `xml:"name"`
	} `xml:"author"`
	Links     []atomLink `xml:"link"`
	Language  string     `xml:"language"`
	Issued    string     `xml:"issued"`
	Published string     `xml:"published"`
}

type atomFeed struct {
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

// searchLinkOPDS1 retourne le lien de recherche, et s'il est déjà un gabarit.
//
// L'adresse est rendue telle qu'écrite, non résolue : quand c'est un gabarit,
// il doit d'abord être rempli.
func searchLinkOPDS1(body []byte) (string, bool, error) {
	var feed atomFeed
	if err := decodeXML(body, &feed); err != nil {
		return "", false, fmt.Errorf("flux OPDS illisible : %w", err)
	}

	for _, link := range feed.Links {
		if link.Rel != relSearch || link.Href == "" {
			continue
		}
		// Un href qui contient déjà le marqueur est le gabarit lui-même :
		// certaines implémentations sautent le document OpenSearch.
		return link.Href, strings.Contains(link.Href, "{searchTerms}"), nil
	}

	return "", false, ErrNoSearch
}

// openSearchDescription est le document que pointe un lien `rel="search"`.
type openSearchDescription struct {
	URLs []struct {
		Type     string `xml:"type,attr"`
		Template string `xml:"template,attr"`
	} `xml:"Url"`
}

// openSearchTemplate extrait le gabarit d'un document OpenSearch.
//
// Un document peut en déclarer plusieurs — un par type de réponse. On retient
// celui qui rend de l'OPDS ; à défaut, le premier qui contient le marqueur,
// parce qu'un catalogue qui n'annonce qu'un type générique reste utilisable.
func openSearchTemplate(body []byte) (string, error) {
	var description openSearchDescription
	if err := decodeXML(body, &description); err != nil {
		return "", fmt.Errorf("document OpenSearch illisible : %w", err)
	}

	var fallback string
	for _, entry := range description.URLs {
		if !strings.Contains(entry.Template, "{searchTerms}") {
			continue
		}
		if strings.Contains(entry.Type, "opds") || strings.Contains(entry.Type, "atom") {
			return entry.Template, nil
		}
		if fallback == "" {
			fallback = entry.Template
		}
	}

	if fallback == "" {
		return "", ErrNoSearch
	}
	return fallback, nil
}

func parseAtom(body []byte, base string) ([]Result, error) {
	var feed atomFeed
	if err := decodeXML(body, &feed); err != nil {
		return nil, fmt.Errorf("flux OPDS illisible : %w", err)
	}

	results := make([]Result, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		result := Result{
			Title:     strings.TrimSpace(entry.Title),
			Summary:   firstNonEmpty(entry.Summary, entry.Content),
			Language:  entry.Language,
			Published: firstNonEmpty(entry.Issued, entry.Published),
		}
		for _, author := range entry.Authors {
			if name := strings.TrimSpace(author.Name); name != "" {
				result.Authors = append(result.Authors, name)
			}
		}

		navigation := false

		for _, link := range entry.Links {
			href := resolve(base, link.Href)
			switch {
			case isNavigation(link):
				navigation = true
			case link.Rel == relImage || link.Rel == relThumbnail:
				// La vignette l'emporte quand les deux existent : c'est une
				// grille de résultats, pas une page de couverture.
				if result.CoverURL == "" || link.Rel == relThumbnail {
					result.CoverURL = href
				}
			case strings.HasPrefix(link.Rel, relAcquisition):
				result.Acquisitions = append(result.Acquisitions,
					Link{Href: href, Type: link.Type, Rel: link.Rel})
			case link.Rel == relAlternate && strings.Contains(link.Type, "html"):
				result.PageURL = href
			}
		}

		/*
			Un flux OPDS mélange deux natures d'entrées : des œuvres, et des
			rubriques qui mènent à d'autres flux. Beaucoup de catalogues en
			renvoient dans leurs résultats de recherche — « voir la série
			complète », « autres tomes ».

			Elles sont écartées. Une rubrique dans une liste de résultats est
			une ligne sur laquelle on ne peut rien faire : pas de couverture,
			pas de téléchargement, et le lien mène à un flux XML que le
			navigateur affichera en texte brut.

			Le critère est celui de la spécification : une entrée qui n'a que
			des liens de navigation n'est pas une acquisition. Une entrée qui
			porte les deux reste, parce qu'elle est bien une œuvre.
		*/
		if result.Title == "" || (navigation && len(result.Acquisitions) == 0) {
			continue
		}
		results = append(results, result)
	}
	return results, nil
}

// isNavigation reconnaît un lien qui mène à un autre flux plutôt qu'à une œuvre.
func isNavigation(link atomLink) bool {
	switch link.Rel {
	case "subsection", "collection", "up", "start", "next", "previous", "first", "last":
		return true
	}
	// Le type dit le reste : un lien vers un catalogue OPDS est une rubrique,
	// quelle que soit la relation sous laquelle le catalogue l'a rangé.
	return strings.Contains(link.Type, "profile=opds-catalog")
}

// ─── OPDS 2.0 : JSON ─────────────────────────────────────────────────────────

type jsonLink struct {
	Href      string `json:"href"`
	Type      string `json:"type"`
	Rel       any    `json:"rel"`
	Templated bool   `json:"templated"`
}

// rels normalise la relation, que le format autorise en chaîne ou en tableau.
func (l jsonLink) rels() []string {
	switch value := l.Rel.(type) {
	case string:
		return []string{value}
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func (l jsonLink) hasRel(want string) bool {
	for _, rel := range l.rels() {
		if rel == want || strings.HasPrefix(rel, want) {
			return true
		}
	}
	return false
}

type jsonFeed struct {
	Links        []jsonLink `json:"links"`
	Publications []struct {
		Metadata struct {
			Title       string `json:"title"`
			Author      any    `json:"author"`
			Language    any    `json:"language"`
			Published   string `json:"published"`
			Modified    string `json:"modified"`
			Description string `json:"description"`
			BelongsTo   struct {
				Series any `json:"series"`
			} `json:"belongsTo"`
		} `json:"metadata"`
		Links  []jsonLink `json:"links"`
		Images []jsonLink `json:"images"`
	} `json:"publications"`
}

func searchTemplateOPDS2(body []byte) (string, error) {
	var feed jsonFeed
	if err := json.Unmarshal(body, &feed); err != nil {
		return "", fmt.Errorf("flux OPDS 2 illisible : %w", err)
	}

	for _, link := range feed.Links {
		if link.hasRel(relSearch) && link.Href != "" {
			return link.Href, nil
		}
	}
	return "", ErrNoSearch
}

func parseOPDS2(body []byte, base string) ([]Result, error) {
	var feed jsonFeed
	if err := json.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("flux OPDS 2 illisible : %w", err)
	}

	results := make([]Result, 0, len(feed.Publications))
	for _, publication := range feed.Publications {
		meta := publication.Metadata
		result := Result{
			Title:     strings.TrimSpace(meta.Title),
			Authors:   flattenNames(meta.Author),
			Series:    firstOf(flattenNames(meta.BelongsTo.Series)),
			Summary:   meta.Description,
			Language:  firstOf(flattenNames(meta.Language)),
			Published: firstNonEmpty(meta.Published, meta.Modified),
		}

		if len(publication.Images) > 0 {
			result.CoverURL = resolve(base, publication.Images[0].Href)
		}
		for _, link := range publication.Links {
			href := resolve(base, link.Href)
			switch {
			case link.hasRel(relAcquisition):
				result.Acquisitions = append(result.Acquisitions,
					Link{Href: href, Type: link.Type, Rel: relAcquisition})
			case link.hasRel(relAlternate) && strings.Contains(link.Type, "html"):
				result.PageURL = href
			}
		}

		if result.Title != "" {
			results = append(results, result)
		}
	}
	return results, nil
}

// ─── Communs ─────────────────────────────────────────────────────────────────

func parseFeed(body []byte, contentType, base string) ([]Result, error) {
	if isJSONFeed(contentType, body) {
		return parseOPDS2(body, base)
	}
	return parseAtom(body, base)
}

// isJSONFeed distingue les deux versions du format.
//
// Le type de contenu d'abord, la première accolade ensuite : un serveur mal
// configuré qui annonce `text/plain` sur son JSON reste lisible, et c'est plus
// fréquent qu'on ne voudrait.
func isJSONFeed(contentType string, body []byte) bool {
	if strings.Contains(contentType, "json") {
		return true
	}
	if strings.Contains(contentType, "xml") {
		return false
	}
	// \ufeff : la marque d'ordre des octets, que certains catalogues
	// laissent devant leur JSON.
	trimmed := strings.TrimLeft(string(body), " \t\r\n\ufeff")
	return strings.HasPrefix(trimmed, "{")
}

// decodeXML lit du XML en tolérant ce que les flux réels contiennent.
//
// Les catalogues déclarent des espaces de noms et des encodages que le décodeur
// strict refuse. Comme on ne lit que des éléments et des attributs simples, la
// tolérance ne coûte rien et évite de rejeter un flux parfaitement lisible.
func decodeXML(body []byte, into any) error {
	decoder := xml.NewDecoder(strings.NewReader(string(body)))
	decoder.Strict = false
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) {
		return input, nil
	}
	return decoder.Decode(into)
}

/*
expandTemplate remplit un gabarit d'URL avec les termes cherchés.

Deux syntaxes, parce que les deux versions d'OPDS n'ont pas fait le même choix :
OpenSearch écrit `{searchTerms}` dans le chemin ou la requête, OPDS 2.0 utilise
l'expansion de formulaire de l'RFC 6570 — `{?query}`, qui produit la paire
`?query=…` en entier, séparateur compris.

Ce n'est pas une implémentation complète de l'RFC 6570, et ça n'a pas à l'être :
les catalogues n'utilisent qu'une poignée de formes, toutes couvertes ici. Une
variable inconnue est retirée plutôt que laissée en place — un gabarit à moitié
rempli ferait une URL invalide, alors qu'un paramètre absent laisse simplement
le catalogue appliquer son défaut.
*/
func expandTemplate(template, terms string) string {
	escaped := url.QueryEscape(terms)

	if strings.Contains(template, "{searchTerms}") {
		return removeUnfilled(strings.ReplaceAll(template, "{searchTerms}", escaped))
	}

	// Expansion de formulaire : `{?query}` ou `{?query,author,title}`.
	// Le premier paramètre reçoit les termes, les autres sont abandonnés.
	if start := strings.Index(template, "{?"); start >= 0 {
		if end := strings.Index(template[start:], "}"); end > 0 {
			names := strings.Split(template[start+2:start+end], ",")
			expanded := ""
			if len(names) > 0 && names[0] != "" {
				separator := "?"
				if strings.Contains(template[:start], "?") {
					separator = "&"
				}
				expanded = separator + names[0] + "=" + escaped
			}
			return removeUnfilled(template[:start] + expanded + template[start+end+1:])
		}
	}

	// Aucun marqueur : le catalogue a annoncé une URL fixe. On y ajoute le
	// paramètre conventionnel plutôt que de renoncer.
	separator := "?"
	if strings.Contains(template, "?") {
		separator = "&"
	}
	return template + separator + "query=" + escaped
}

// removeUnfilled retire les variables de gabarit restées vides.
func removeUnfilled(target string) string {
	for {
		start := strings.Index(target, "{")
		if start < 0 {
			return target
		}
		end := strings.Index(target[start:], "}")
		if end < 0 {
			return target[:start]
		}
		target = target[:start] + target[start+end+1:]
	}
}

// resolve rend absolue une adresse relative au flux qui la porte.
func resolve(base, href string) string {
	if href == "" {
		return ""
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return href
	}
	if parsed.IsAbs() {
		return href
	}
	root, err := url.Parse(base)
	if err != nil {
		return href
	}
	return root.ResolveReference(parsed).String()
}

// flattenNames normalise un champ que le format autorise sous trois formes :
// une chaîne, un objet `{name}`, ou un tableau de l'un ou l'autre.
func flattenNames(value any) []string {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	case map[string]any:
		if name, ok := typed["name"].(string); ok && name != "" {
			return []string{name}
		}
		return nil
	case []any:
		var out []string
		for _, item := range typed {
			out = append(out, flattenNames(item)...)
		}
		return out
	default:
		return nil
	}
}

func firstOf(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
