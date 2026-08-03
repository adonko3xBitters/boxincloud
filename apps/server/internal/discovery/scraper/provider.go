package scraper

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/adonko3xBitters/boxincloud/server/internal/discovery"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/netguard"
)

/*
Client de sites pilotés par gabarit.

Implémente `discovery.Client`, la même interface que `OPDSClient` — et c'est
tout l'intérêt : le service de recherche fédérée, l'import asynchrone, la
déduplication et le marquage « déjà dans votre bibliothèque » fonctionnent sans
une ligne de plus. Un site lu au gabarit est un catalogue comme un autre à
partir du moment où il rend des `Result`.

# Comment une source désigne son gabarit

Par son GENRE : `kind = "scraper:digitalcomicmuseum"`. La colonne existe déjà,
elle est déjà la clé de routage, et son commentaire de migration anticipait
exactement cet usage — « un protocole de catalogue de plus coûte moins cher ici
qu'en migration plus tard ».

L'`url` de la source, elle, porte la base ACTIVE. C'est ce qui répond au besoin
« un miroir a changé, je ne veux pas recompiler » : l'administrateur modifie
l'adresse dans la configuration de la source, et elle passe devant celles du
gabarit.
*/

// Client interroge les sites d'un catalogue de gabarits.
type Client struct {
	catalog *Catalog
	fetch   *fetcher
	log     *slog.Logger
}

var (
	_ discovery.Client        = (*Client)(nil)
	_ discovery.OriginChecker = (*Client)(nil)
)

func New(catalog *Catalog, deps Deps) *Client {
	deps = deps.withDefaults()
	return &Client{
		catalog: catalog,
		fetch:   newFetcher(deps),
		log:     deps.Log,
	}
}

/*
Kinds décrit les genres de source que ce client sait traiter.

Sert au câblage : le service enregistre le client pour chacun, et garde la
description pour que l'administration puisse les proposer. Ni le service ni
l'API n'ont ainsi à connaître la convention de nommage des gabarits — elle ne
sort jamais de ce paquet.
*/
func (c *Client) Kinds() []discovery.KindInfo {
	templates := c.catalog.List()
	out := make([]discovery.KindInfo, 0, len(templates)+1)

	// `web` d'abord, et toujours : il ne dépend d'aucun gabarit chargé. C'est
	// lui qui rend le moteur atteignable sur une instance qui n'en a aucun.
	out = append(out, discovery.KindInfo{Kind: discovery.KindWeb, Name: "Site web"})

	for _, template := range templates {
		out = append(out, discovery.KindInfo{
			Kind:     discovery.ScraperKind(template.ID),
			ID:       template.ID,
			Name:     template.Name,
			Homepage: template.Homepage,
			License:  template.License,
			// Copiés : la tranche du gabarit ne doit pas pouvoir être modifiée
			// par un appelant, elle sert à borner ce qu'un import peut joindre.
			Mirrors: append([]string(nil), template.Mirrors...),
		})
	}
	return out
}

/*
Search interroge le site et rend ses résultats, normalisés.

Le budget global est posé ICI, en tête, et couvre tout ce qui suit — requête de
recherche, analyse, suivi des fiches. C'est la seule borne qui protège
réellement la page : les délais unitaires, eux, s'additionnent.
*/
func (c *Client) Search(
	ctx context.Context, source discovery.Source, _ string, q discovery.Query,
) ([]discovery.Result, error) {
	terms := strings.TrimSpace(q.Text)
	if terms == "" {
		return nil, nil
	}

	template, err := c.templateFor(source)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, template.Budget())
	defer cancel()

	limit := q.Limit
	if limit <= 0 || limit > template.Limits.MaxResults {
		limit = template.Limits.MaxResults
	}

	got, err := c.fetch.attempt(ctx, template, basesFor(template, source),
		searchRequest(template, terms, limit))
	if err != nil {
		return nil, err
	}

	doc, err := parseHTML(got.body)
	if err != nil {
		return nil, err
	}

	results := extractResults(doc, template, got.url, limit)
	c.followDetails(ctx, template, results)

	for i := range results {
		results[i].SourceID = source.ID
		results[i].SourceName = source.Name
	}
	return results, nil
}

/*
Probe vérifie qu'un site répond ET que le gabarit le lit encore.

La seconde moitié est celle qui compte. Un site refait sa mise en page : il
répond parfaitement, et le gabarit ne trouve plus une seule ligne. Sans ce
contrôle, la panne se déguise en « aucun résultat » — le mode d'échec le plus
coûteux à diagnostiquer, parce qu'il ressemble à une recherche infructueuse.

Le terme d'essai vient du gabarit (`search.probe`), et il doit être choisi pour
rendre toujours quelque chose sur ce site-là. Un terme sans résultat ferait
échouer l'essai d'un gabarit parfaitement valide, ce qui est le défaut inverse
et pas moins gênant.
*/
func (c *Client) Probe(ctx context.Context, source discovery.Source, _ string) error {
	template, err := c.templateFor(source)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, template.Budget())
	defer cancel()

	got, err := c.fetch.attempt(ctx, template, basesFor(template, source),
		searchRequest(template, template.Search.Probe, template.Limits.MaxResults))
	if err != nil {
		return err
	}

	doc, err := parseHTML(got.body)
	if err != nil {
		return err
	}

	if doc.FindMatcher(template.row).Length() == 0 {
		return fmt.Errorf(
			"%w : le site répond, mais le gabarit %q n'y trouve aucun résultat "+
				"(sélecteur %q) — il a probablement changé de mise en page",
			discovery.ErrInvalidSource, template.ID, template.Results.Select)
	}
	return nil
}

/*
Open ouvre un lien de téléchargement.

Ne passe PAS par `fetch` : celle-ci lit la réponse en entier pour l'analyser, ce
qui chargerait une intégrale de trois cents méga-octets en mémoire. Le corps est
rendu tel quel au service, qui le passe au dépôt — même choix que pour
`OPDSClient.Open`, et pour la même raison.

Le délai est celui du contexte, pas celui du gabarit : un téléchargement a le
droit de durer là où une recherche ne l'a pas.
*/
func (c *Client) Open(
	ctx context.Context, source discovery.Source, _ string, href string,
) (discovery.Fetched, error) {
	template, err := c.templateFor(source)
	if err != nil {
		return discovery.Fetched{}, err
	}

	if !c.AllowsHost(source, href) {
		return discovery.Fetched{}, fmt.Errorf("%w : %s", discovery.ErrForeignHost, href)
	}
	if err := netguard.Check(href); err != nil {
		return discovery.Fetched{}, fmt.Errorf("%w : %w", discovery.ErrInvalidSource, err)
	}

	// robots.txt s'applique aussi au téléchargement. Un site qui ouvre ses
	// pages de recherche et ferme son répertoire de fichiers l'a écrit
	// exprès ; le lire pour la liste et l'ignorer pour le fichier reviendrait à
	// ne pas le lire du tout.
	//
	// La dérogation vaut pour les deux, et c'est cohérent : une source dispensée
	// pour chercher mais pas pour télécharger rendrait des résultats sur
	// lesquels rien n'est possible.
	if !template.IgnoreRobots {
		if allowed, err := c.fetch.robots.allows(ctx, template, href); err == nil && !allowed {
			return discovery.Fetched{}, fmt.Errorf("%w : %s", ErrDisallowed, href)
		}
	}

	if c.fetch.throttle != nil {
		if err := c.fetch.throttle.Wait(ctx, bucketFor(hostOf(href))); err != nil {
			return discovery.Fetched{}, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, href, nil)
	if err != nil {
		return discovery.Fetched{}, fmt.Errorf("%w : %w", discovery.ErrInvalidSource, err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")

	resp, err := c.fetch.http.Do(req)
	if err != nil {
		return discovery.Fetched{}, fmt.Errorf("téléchargement impossible : %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return discovery.Fetched{}, fmt.Errorf("le site a répondu %s", resp.Status)
	}

	return discovery.Fetched{
		Body:        resp.Body,
		Size:        resp.ContentLength,
		Filename:    filenameFromDisposition(resp.Header.Get("Content-Disposition")),
		ContentType: resp.Header.Get("Content-Type"),
	}, nil
}

/*
AllowsHost élargit le contrôle d'origine de l'import à tous les miroirs.

Nécessaire, et pas par confort. Le contrôle par défaut exige que l'adresse
téléchargée partage l'hôte de la source ; or un site sert couramment ses pages
depuis `exemple.org` et ses fichiers depuis `dl.exemple.org`, et un miroir sert
les deux depuis ailleurs. La règle par défaut refuserait ces téléchargements.

Ce qui est élargi reste FERMÉ : la liste des hôtes est celle que le gabarit
déclare, plus celui que l'administrateur a saisi. Rien n'y entre à l'exécution,
et une redirection vers un hôte tiers ne l'ouvre pas davantage.
*/
func (c *Client) AllowsHost(source discovery.Source, href string) bool {
	template, err := c.templateFor(source)
	if err != nil {
		return false
	}

	host := hostOf(href)
	if host == "" {
		return false
	}
	return template.AllowsHost(host) || host == hostOf(source.URL)
}

// ─── Composition de la recherche ─────────────────────────────────────────────

/*
basesFor ordonne les bases à essayer.

L'adresse enregistrée pour la source vient EN PREMIER quand il y en a une :
c'est le choix explicite d'un administrateur, et un défaut de gabarit ne doit
pas le contredire. Les miroirs du gabarit suivent, dans leur ordre déclaré,
celui de la source retiré pour ne pas l'essayer deux fois.
*/
func basesFor(template *Compiled, source discovery.Source) []string {
	var bases []string
	seen := map[string]bool{}

	if host := hostOf(source.URL); host != "" {
		bases = append(bases, strings.TrimRight(strings.TrimSpace(source.URL), "/"))
		seen[host] = true
	}
	for _, mirror := range template.Mirrors {
		if host := hostOf(mirror); host != "" && !seen[host] {
			bases = append(bases, strings.TrimRight(mirror, "/"))
			seen[host] = true
		}
	}
	return bases
}

/*
searchRequest remplit le gabarit de requête.

`{terms}` et `{limit}` sont remplacés partout où ils apparaissent. L'échappement
n'est PAS fait ici : les valeurs de la chaîne de requête sont encodées par
`url.Values.Encode`, et celles du chemin par `url.Parse` au moment de composer.
Échapper en plus donnerait un `%2520` — le défaut classique du double
encodage, qui fait chercher littéralement « moebius%20giraud ».
*/
func searchRequest(template *Compiled, terms string, limit int) request {
	replace := strings.NewReplacer(
		"{terms}", terms,
		"{limit}", strconv.Itoa(limit),
	)

	req := request{
		method:  template.Search.Method,
		path:    replace.Replace(template.Search.Path),
		headers: template.Search.Headers,
	}

	if len(template.Search.Query) > 0 {
		req.query = url.Values{}
		for name, value := range template.Search.Query {
			req.query.Set(name, replace.Replace(value))
		}
	}
	if len(template.Search.Form) > 0 {
		req.form = url.Values{}
		for name, value := range template.Search.Form {
			req.form.Set(name, replace.Replace(value))
		}
	}
	return req
}

/*
followDetails complète les résultats depuis leur fiche.

Séquentiel, délibérément. Les faire en parallèle diviserait l'attente par le
nombre de goroutines et multiplierait d'autant la charge imposée au site — le
limiteur les rattraperait, mais en gardant des goroutines en attente pour rien.
Un site associatif mérite qu'on lui parle une requête à la fois.

Trois bornes se cumulent, et il en faut trois :

  - `onlyIfMissing` ne suit que les lignes auxquelles il manque quelque chose,
    ce qui ramène le coût à zéro sur un site qui publie déjà tout dans sa liste ;
  - `followDetail` borne le nombre de fiches ;
  - le budget du contexte arrête tout, y compris au milieu.

Un échec sur une fiche n'interrompt rien : le résultat reste, moins complet.
Perdre vingt résultats parce que la troisième fiche a répondu 500 serait un
mauvais échange.
*/
func (c *Client) followDetails(
	ctx context.Context, template *Compiled, results []discovery.Result,
) {
	if template.Detail == nil {
		return
	}

	followed := 0
	for i := range results {
		if followed >= template.Limits.FollowDetail {
			return
		}
		if ctx.Err() != nil {
			return
		}
		if !needsDetail(template.Detail, results[i]) {
			continue
		}

		href := detailHref(template.Detail, results[i])
		if href == "" || !template.AllowsHost(hostOf(href)) {
			continue
		}

		followed++
		got, err := c.fetch.fetch(ctx, template, href, request{method: http.MethodGet})
		if err != nil {
			c.log.Debug("fiche non suivie",
				slog.String("template", template.ID),
				slog.String("url", href),
				slog.Any("err", err))
			continue
		}

		doc, err := parseHTML(got.body)
		if err != nil {
			continue
		}
		applyDetail(&results[i], doc, template, got.url)
	}
}

// needsDetail dit si une ligne justifie une requête de plus.
//
// Sans `onlyIfMissing`, toutes la justifient : c'est le cas d'un site dont la
// liste ne porte qu'un titre et un lien.
func needsDetail(spec *DetailSpec, result discovery.Result) bool {
	if len(spec.OnlyIfMissing) == 0 {
		return true
	}
	for _, name := range spec.OnlyIfMissing {
		if fieldValue(result, name) == "" {
			return true
		}
	}
	return false
}

func detailHref(spec *DetailSpec, result discovery.Result) string {
	return fieldValue(result, spec.From)
}

// fieldValue relit un champ de résultat par son nom de gabarit.
//
// Le pendant en lecture du `switch` d'`assign`. Rendre une chaîne pour des
// champs qui sont des listes suffit ici : `onlyIfMissing` ne demande que
// « est-ce vide », jamais le contenu.
func fieldValue(result discovery.Result, name string) string {
	switch name {
	case fieldTitle:
		return result.Title
	case fieldSeries:
		return result.Series
	case fieldSummary:
		return result.Summary
	case fieldLanguage:
		return result.Language
	case fieldPublished:
		return result.Published
	case fieldCoverURL:
		return result.CoverURL
	case fieldPageURL:
		return result.PageURL
	case fieldAuthors:
		if len(result.Authors) > 0 {
			return result.Authors[0]
		}
	case fieldAcquisition:
		if len(result.Acquisitions) > 0 {
			return result.Acquisitions[0].Href
		}
	}
	return ""
}

// ─── Divers ──────────────────────────────────────────────────────────────────

/*
templateFor retrouve les règles d'une source, d'où qu'elles viennent.

Deux origines, un seul type en sortie. Une source `web` porte sa description
dans sa propre ligne ; un genre `scraper:<gabarit>` renvoie à un fichier chargé
au démarrage. À partir d'ici, plus rien ne les distingue — c'est ce qui évite
que la version saisie au formulaire dérive de l'autre.

La compilation d'une source `web` a lieu à chaque appel, sans cache. Compiler
cinq sélecteurs coûte quelques microsecondes contre plusieurs centaines de
millisecondes de réseau ; un cache ajouterait sa propre invalidation à traiter
quand l'administrateur modifie la source, pour un gain invisible.
*/
func (c *Client) templateFor(source discovery.Source) (*Compiled, error) {
	if source.Kind == discovery.KindWeb {
		if len(source.Template) == 0 {
			return nil, fmt.Errorf("%w : cette source n'a pas de règles d'extraction",
				ErrUnknownTemplate)
		}
		return ParseWebSpec(source.Template)
	}

	id, ok := source.Kind.ScraperTemplate()
	if !ok {
		return nil, fmt.Errorf("%w : %s n'est pas un genre de gabarit",
			ErrUnknownTemplate, source.Kind)
	}
	template, ok := c.catalog.Get(id)
	if !ok {
		return nil, fmt.Errorf("%w : %s", ErrUnknownTemplate, id)
	}
	return template, nil
}

// parseHTML construit l'arbre du document.
//
// `net/html` ne rend jamais d'erreur d'analyse : il applique les règles de
// récupération du HTML5, celles-là mêmes qu'un navigateur applique aux pages
// mal formées. C'est la bonne propriété ici — les sites visés en servent
// beaucoup, et refuser une page qu'un navigateur affiche sans broncher serait
// difficile à justifier.
func parseHTML(body []byte) (*goquery.Document, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("page illisible : %w", err)
	}
	return doc, nil
}

// filenameFromDisposition lit le nom que le site déclare, s'il en déclare un.
//
// Dupliquée depuis le paquet parent où elle n'est pas exportée. Trois lignes
// recopiées valent mieux qu'un élargissement de sa surface publique pour un
// détail d'en-tête HTTP.
func filenameFromDisposition(disposition string) string {
	if disposition == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(disposition)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(params["filename"])
}
