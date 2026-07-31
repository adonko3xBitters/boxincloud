package scraper

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/discovery"
)

/*
Le client, contre de vrais serveurs.

Comme pour le client OPDS : ce qui casse dans un client de catalogue n'est
presque jamais l'analyse du document — elle est déjà couverte sans réseau — mais
le CHEMIN qui y mène. Composition de l'URL, choix du miroir, repli, suivi des
fiches, contrôle d'origine.

Les serveurs d'essai imitent donc les comportements qu'on rencontre vraiment :
un miroir en panne, un site qui refuse fermement, un `robots.txt` qui ferme une
partie de l'arborescence, une fiche qui porte le lien que la liste n'a pas.
*/

// shelfServer imite un site lisible au gabarit de référence.
func shelfServer(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()

	var hits atomic.Int64
	mux := http.NewServeMux()

	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /interne/\n"))
	})

	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Query().Get("q") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(searchPage))
	})

	// Les fiches : c'est là que ce site publie ses liens de téléchargement.
	mux.HandleFunc("/issue/", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		id := strings.TrimPrefix(r.URL.Path, "/issue/")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>
			<div class="synopsis">Résumé de ` + id + `.</div>
			<a class="download" href="/dl/` + id + `.cbz">Télécharger</a>
			</body></html>`))
	})

	mux.HandleFunc("/dl/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.comicbook+zip")
		w.Header().Set("Content-Disposition", `attachment; filename="album.cbz"`)
		_, _ = w.Write([]byte("PK\x03\x04 contenu"))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, &hits
}

/*
clientFor construit un client dont le gabarit vise les serveurs donnés.

Les miroirs d'un gabarit sont écrits en dur dans son YAML ; les réécrire ici est
la seule façon de le confronter à un vrai serveur. Les hôtes autorisés suivent,
sans quoi le contrôle d'origine refuserait tous les téléchargements.
*/
func clientFor(t *testing.T, template *Compiled, mirrors ...string) *Client {
	t.Helper()

	template.Mirrors = mirrors
	template.hosts = map[string]bool{}
	for _, mirror := range mirrors {
		template.hosts[hostOf(mirror)] = true
	}

	catalog := &Catalog{byID: map[string]*Compiled{template.ID: template}}
	return New(catalog, Deps{})
}

func sourceFor(template *Compiled, url string) discovery.Source {
	return discovery.Source{
		ID:      uuid.New(),
		Name:    template.Name,
		Kind:    discovery.ScraperKind(template.ID),
		URL:     url,
		Enabled: true,
	}
}

func TestSearchEndToEnd(t *testing.T) {
	server, _ := shelfServer(t)

	template := loadReference(t)
	client := clientFor(t, template, server.URL)
	source := sourceFor(template, "")

	results, err := client.Search(context.Background(), source, "",
		discovery.Query{Text: "fantastic", Limit: 10})
	if err != nil {
		t.Fatalf("recherche : %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("résultats = %d, attendu 2 : %+v", len(results), results)
	}

	// La provenance est posée par le client : sans elle, l'interface ne sait pas
	// de quel catalogue vient la ligne, ni à qui demander l'import.
	if results[0].SourceID != source.ID || results[0].SourceName != source.Name {
		t.Errorf("provenance = %v / %q", results[0].SourceID, results[0].SourceName)
	}

	// Le lien de téléchargement n'existe que sur la fiche : le trouver ici
	// prouve que le suivi a bien eu lieu, et que son résultat est revenu dans
	// la ligne d'origine.
	if len(results[0].Acquisitions) == 0 {
		t.Fatalf("aucun lien d'acquisition : %+v", results[0])
	}
	if !strings.HasSuffix(results[0].Acquisitions[0].Href, "/dl/42.cbz") {
		t.Errorf("lien = %q", results[0].Acquisitions[0].Href)
	}
	if results[0].Summary == "" {
		t.Error("le résumé de la fiche n'a pas été rapporté")
	}
}

/*
Le repli sur un miroir.

Le premier miroir répond 500, le second sert la page. C'est LE cas pour lequel
la liste de miroirs existe, et la recherche doit aboutir sans que l'utilisateur
apprenne qu'il s'est passé quelque chose.
*/
func TestSearchFallsBackToNextMirror(t *testing.T) {
	broken := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
	t.Cleanup(broken.Close)

	server, _ := shelfServer(t)

	template := loadReference(t)
	client := clientFor(t, template, broken.URL, server.URL)

	results, err := client.Search(context.Background(),
		sourceFor(template, ""), "", discovery.Query{Text: "fantastic"})
	if err != nil {
		t.Fatalf("le repli n'a pas eu lieu : %v", err)
	}
	if len(results) == 0 {
		t.Fatal("aucun résultat après repli")
	}
}

/*
Un refus ferme n'est PAS une panne.

Un 404 dit que le site a compris la demande. Le reposer à chaque miroir ferait N
requêtes pour apprendre N fois la même chose — exactement le comportement qui
fait remarquer un client.
*/
func TestSearchDoesNotRetryPermanentRefusals(t *testing.T) {
	var first, second atomic.Int64

	count := func(counter *atomic.Int64, status int) *httptest.Server {
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/robots.txt" {
					counter.Add(1)
				}
				w.WriteHeader(status)
			}))
		t.Cleanup(server.Close)
		return server
	}

	a := count(&first, http.StatusNotFound)
	b := count(&second, http.StatusOK)

	template := loadReference(t)
	client := clientFor(t, template, a.URL, b.URL)

	_, err := client.Search(context.Background(),
		sourceFor(template, ""), "", discovery.Query{Text: "fantastic"})
	if err == nil {
		t.Fatal("un 404 devrait faire échouer la recherche")
	}
	if got := second.Load(); got != 0 {
		t.Errorf("le second miroir a été sollicité %d fois, aucune attendue", got)
	}
}

/*
L'URL de la source passe devant les miroirs du gabarit.

C'est la réponse au cas « le domaine a changé » : l'administrateur saisit la
nouvelle adresse, et elle l'emporte sans qu'on recompile quoi que ce soit.
*/
func TestSourceURLOverridesMirrors(t *testing.T) {
	server, _ := shelfServer(t)

	broken := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
	t.Cleanup(broken.Close)

	template := loadReference(t)
	client := clientFor(t, template, broken.URL)

	// Le gabarit ne connaît que le miroir en panne ; la source désigne l'autre.
	source := sourceFor(template, server.URL)

	bases := basesFor(template, source)
	if bases[0] != strings.TrimRight(server.URL, "/") {
		t.Fatalf("bases = %v, celle de la source devrait venir en premier", bases)
	}

	if _, err := client.Search(context.Background(), source, "",
		discovery.Query{Text: "fantastic"}); err != nil {
		t.Fatalf("recherche : %v", err)
	}
}

/*
robots.txt arrête le suivi de fiche, sans arrêter la recherche.

Un site qui ferme une partie de son arborescence garde les autres ouvertes. La
liste doit donc revenir, moins complète, plutôt que de disparaître.
*/
func TestRobotsBlocksDetailWithoutBreakingSearch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /issue/\n"))
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(searchPage))
	})
	mux.HandleFunc("/issue/", func(w http.ResponseWriter, _ *http.Request) {
		t.Error("une fiche interdite par robots.txt a été demandée")
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	template := loadReference(t)
	client := New(&Catalog{byID: map[string]*Compiled{template.ID: template}},
		Deps{Memo: discovery.NewMemo(0, 0)})

	template.Mirrors = []string{server.URL}
	template.hosts = map[string]bool{hostOf(server.URL): true}

	results, err := client.Search(context.Background(),
		sourceFor(template, ""), "", discovery.Query{Text: "fantastic"})
	if err != nil {
		t.Fatalf("la recherche a échoué alors que seules les fiches sont fermées : %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("résultats = %d, attendu 2", len(results))
	}
	if len(results[0].Acquisitions) != 0 {
		t.Error("un lien a été rapporté d'une fiche interdite")
	}
}

/*
`onlyIfMissing` ramène le coût à zéro quand la liste suffit déjà.

C'est le garde-fou qui compte : sans lui, un gabarit à fiche impose une requête
par résultat, et c'est ainsi qu'on se fait bloquer.
*/
func TestDetailIsSkippedWhenNothingIsMissing(t *testing.T) {
	server, hits := shelfServer(t)

	// Même site, mais la liste publie déjà le lien de téléchargement.
	template := mustParse(t, `
id: comicshelf
name: Comic Shelf
mirrors: [https://placeholder.example]
search: {path: /search, query: {q: "{terms}"}}
results:
  select: "ul.results > li.issue"
  fields:
    title: {select: "h3 a"}
    pageUrl: {select: "h3 a", from: attr, attr: href}
    acquisition: {select: "a.download", from: attr, attr: href}
detail:
  from: pageUrl
  onlyIfMissing: [acquisition]
  fields:
    summary: {select: "div.synopsis"}`)

	client := clientFor(t, template, server.URL)

	results, err := client.Search(context.Background(),
		sourceFor(template, ""), "", discovery.Query{Text: "fantastic"})
	if err != nil {
		t.Fatalf("recherche : %v", err)
	}

	// Deux lignes : la première n'a pas de lien dans la liste et sera suivie,
	// la seconde en a un et doit être laissée tranquille. Une requête de
	// recherche + une fiche = deux visites.
	if got := hits.Load(); got != 2 {
		t.Errorf("requêtes = %d, attendu 2 (la seconde fiche ne devait pas être suivie)", got)
	}
	if len(results[1].Acquisitions) != 1 {
		t.Errorf("le lien de la liste a été perdu : %+v", results[1])
	}
}

/*
Probe voit ce qu'une recherche vide ne dit pas.

Un site qui refait sa mise en page répond parfaitement et ne rend plus une seule
ligne. Sans ce contrôle, la panne se déguise en « aucun résultat », et personne
ne va chercher plus loin.
*/
func TestProbeDetectsARedesign(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><div id="app">Nouvelle version</div></body></html>`))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	template := loadReference(t)
	client := clientFor(t, template, server.URL)

	err := client.Probe(context.Background(), sourceFor(template, ""), "")
	if err == nil {
		t.Fatal("l'essai réussit alors que le gabarit ne lit plus rien")
	}
	if !errors.Is(err, discovery.ErrInvalidSource) {
		t.Errorf("erreur = %v, ErrInvalidSource attendue", err)
	}
	if !strings.Contains(err.Error(), "mise en page") {
		t.Errorf("le message ne dit pas quoi soupçonner :\n%v", err)
	}
}

func TestProbeSucceedsOnALiveSite(t *testing.T) {
	server, _ := shelfServer(t)

	template := loadReference(t)
	client := clientFor(t, template, server.URL)

	if err := client.Probe(context.Background(), sourceFor(template, ""), ""); err != nil {
		t.Fatalf("essai : %v", err)
	}
}

/*
Le contrôle d'origine élargi, et sa limite.

Élargi : un fichier servi par un miroir déclaré doit être téléchargeable, alors
que la règle par défaut — même hôte que l'URL de la source — le refuserait.

Limité : la liste reste celle du gabarit. Un hôte tiers n'y entre pas, quelle
que soit l'adresse qu'un client présente.
*/
func TestAllowsHostStaysClosed(t *testing.T) {
	template := loadReference(t)
	client := clientFor(t, template,
		"https://comicshelf.example", "https://mirror.comicshelf.example")
	source := sourceFor(template, "https://comicshelf.example")

	cases := map[string]bool{
		"https://comicshelf.example/dl/42.cbz":        true,
		"https://mirror.comicshelf.example/dl/42.cbz": true,
		"https://ailleurs.example/dl/42.cbz":          false,
		"https://comicshelf.example.attaquant.test/x": false,
		"ftp://comicshelf.example/dl/42.cbz":          false,
	}

	for href, want := range cases {
		if got := client.AllowsHost(source, href); got != want {
			t.Errorf("AllowsHost(%q) = %v, attendu %v", href, got, want)
		}
	}
}

func TestOpenStreamsTheFile(t *testing.T) {
	server, _ := shelfServer(t)

	template := loadReference(t)
	client := clientFor(t, template, server.URL)
	source := sourceFor(template, server.URL)

	fetched, err := client.Open(context.Background(), source, "", server.URL+"/dl/42.cbz")
	if err != nil {
		t.Fatalf("ouverture : %v", err)
	}
	defer func() { _ = fetched.Body.Close() }()

	// Le nom déclaré par le site est repris : c'est lui qui devient la clé de
	// l'objet, puis ce que l'indexation analyse.
	if fetched.Filename != "album.cbz" {
		t.Errorf("nom = %q", fetched.Filename)
	}
}

func TestOpenRefusesAForeignHost(t *testing.T) {
	server, _ := shelfServer(t)

	template := loadReference(t)
	client := clientFor(t, template, server.URL)

	_, err := client.Open(context.Background(), sourceFor(template, server.URL), "",
		"https://ailleurs.example/dl/42.cbz")
	if !errors.Is(err, discovery.ErrForeignHost) {
		t.Fatalf("erreur = %v, ErrForeignHost attendue", err)
	}
}

// Une source dont le gabarit n'est pas chargé échoue clairement, plutôt que de
// rendre une liste vide qu'on prendrait pour une recherche infructueuse.
func TestUnknownTemplate(t *testing.T) {
	client := New(&Catalog{byID: map[string]*Compiled{}}, Deps{})

	_, err := client.Search(context.Background(),
		discovery.Source{Kind: discovery.ScraperKind("disparu")}, "",
		discovery.Query{Text: "x"})

	if !errors.Is(err, ErrUnknownTemplate) {
		t.Fatalf("erreur = %v, ErrUnknownTemplate attendue", err)
	}
}
