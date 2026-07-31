package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

/*
Les trois bases de métadonnées, contre de vraies réponses HTTP.

Les charges utiles sont recopiées de la forme réelle de chaque API, réduite à
ce qu'on en lit. Ce que ces tests vérifient n'est pas qu'on sait décoder du
JSON — c'est ce qui casse en vrai : les champs que l'Archive écrit tantôt en
chaîne tantôt en tableau, les codes de langue à trois lettres qu'Open Library
publie là où le reste du projet en attend deux, les vignettes que Google sert
encore en clair.
*/

func testDeps() MetadataDeps {
	return MetadataDeps{
		HTTP:     &http.Client{Timeout: 2 * time.Second},
		Throttle: NewThrottle(),
		Memo:     NewMemo(time.Minute, 50),
	}
}

// ─── Open Library ────────────────────────────────────────────────────────────

func TestOpenLibraryDescribe(t *testing.T) {
	var seen *url.URL

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "numFound": 2,
  "docs": [
    {
      "key": "/works/OL1W",
      "title": "L'Incal",
      "subtitle": "L'intégrale",
      "author_name": ["Alejandro Jodorowsky", "Moebius"],
      "first_publish_year": 1981,
      "publisher": ["Les Humanoïdes Associés"],
      "isbn": ["9782731618457"],
      "language": ["fre"],
      "subject": ["Bandes dessinées", "Science-fiction"],
      "number_of_pages_median": 320,
      "cover_i": 12345
    },
    {
      "key": "/works/OL2W",
      "title": "Un titre sans rapport",
      "author_name": ["Quelqu'un d'autre"],
      "language": ["eng"]
    }
  ]
}`))
	}))
	defer server.Close()

	provider := NewOpenLibrary(testDeps())
	provider.baseURL = server.URL

	got, err := provider.Describe(context.Background(), Work{
		Title:   "L'Incal",
		Authors: []string{"Moebius"},
	})
	if err != nil {
		t.Fatalf("describe : %v", err)
	}

	/*
		Le paramètre `fields` doit être envoyé. Sans lui, une réponse de dix
		documents dépasse le méga-octet dont on n'utilise qu'un centième — de la
		bande passante prise à un service financé par des dons, pour rien.
	*/
	if !strings.Contains(seen.RawQuery, "fields=") {
		t.Errorf("requête sans restriction de champs : %s", seen.RawQuery)
	}

	if len(got) == 0 {
		t.Fatal("aucun candidat")
	}

	best := got[0]
	if best.Title != "L'Incal : L'intégrale" {
		t.Errorf("titre = %q, le sous-titre devrait être joint", best.Title)
	}
	if best.Language != "fr" {
		t.Errorf("langue = %q : « fre » doit être ramené à l'ISO 639-1", best.Language)
	}
	if best.Published != "1981" || best.PageCount != 320 {
		t.Errorf("date/pages = %q/%d", best.Published, best.PageCount)
	}
	if best.ISBN != "9782731618457" {
		t.Errorf("isbn = %q", best.ISBN)
	}
	if !strings.HasPrefix(best.CoverURL, "https://covers.openlibrary.org/") {
		t.Errorf("couverture = %q", best.CoverURL)
	}
	if best.PageURL != "https://openlibrary.org/works/OL1W" {
		t.Errorf("fiche = %q", best.PageURL)
	}
	if best.ProviderKind != "openlibrary" {
		t.Errorf("provenance = %q", best.ProviderKind)
	}

	// Le second document ne partage ni titre ni auteur : le laisser passer
	// encombrerait l'écran de correction de propositions absurdes.
	for _, candidate := range got {
		if candidate.Title == "Un titre sans rapport" {
			t.Error("un candidat sans aucun point commun a été retenu")
		}
	}
}

// TestOpenLibraryISBNShortCircuits : un ISBN connu vaut tous les autres indices.
func TestOpenLibraryISBNShortCircuits(t *testing.T) {
	var seen *url.URL

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"docs":[{"key":"/works/OL1W","title":"Peu importe",
		  "isbn":["978-2-7316-1845-7"]}]}`))
	}))
	defer server.Close()

	provider := NewOpenLibrary(testDeps())
	provider.baseURL = server.URL

	got, err := provider.Describe(context.Background(), Work{
		Title: "Un titre complètement différent",
		ISBN:  "9782731618457",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Les termes envoyés sont l'ISBN seul : y joindre un titre approximatif ne
	// ferait que du bruit.
	if !strings.Contains(seen.RawQuery, "9782731618457") {
		t.Errorf("la requête n'utilise pas l'ISBN : %s", seen.RawQuery)
	}
	if len(got) != 1 || got[0].Confidence != 1 {
		t.Fatalf("candidats = %+v, attendu une certitude sur ISBN identique", got)
	}
}

// ─── Google Books ────────────────────────────────────────────────────────────

func TestGoogleBooksDescribe(t *testing.T) {
	var seen *url.URL

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "items": [
    {
      "volumeInfo": {
        "title": "Le Garage hermétique",
        "authors": ["Moebius"],
        "publisher": "Les Humanoïdes Associés",
        "publishedDate": "1979",
        "description": "Les aventures de Jerry Cornelius.",
        "pageCount": 128,
        "categories": ["Comics & Graphic Novels"],
        "language": "fr",
        "infoLink": "https://books.google.com/books?id=abc",
        "industryIdentifiers": [
          {"type": "ISBN_10", "identifier": "2731600000"},
          {"type": "ISBN_13", "identifier": "9782731600001"}
        ],
        "imageLinks": {
          "smallThumbnail": "http://books.google.com/small.jpg",
          "thumbnail": "http://books.google.com/thumb.jpg"
        }
      }
    }
  ]
}`))
	}))
	defer server.Close()

	provider := NewGoogleBooks("clé-de-test", testDeps())
	provider.baseURL = server.URL

	got, err := provider.Describe(context.Background(), Work{
		Title:   "Le Garage hermétique",
		Authors: []string{"Moebius"},
	})
	if err != nil {
		t.Fatalf("describe : %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("%d candidats, attendu 1", len(got))
	}

	// Les qualificatifs divisent le bruit sur un titre commun, ce qui est
	// exactement ce qu'on cherche pour un rapprochement.
	query := seen.Query().Get("q")
	if !strings.Contains(query, "intitle:") || !strings.Contains(query, "inauthor:") {
		t.Errorf("requête sans qualificatifs : %q", query)
	}
	if seen.Query().Get("key") != "clé-de-test" {
		t.Error("la clé d'API n'est pas transmise")
	}

	best := got[0]
	if best.ISBN != "9782731600001" {
		t.Errorf("isbn = %q, l'ISBN-13 doit primer", best.ISBN)
	}
	/*
		L'API rend encore des liens en clair pour certains volumes. Les laisser
		tels quels ferait bloquer l'image par le navigateur sur une instance
		servie en HTTPS — contenu mixte — et la couverture manquerait sans que
		rien ne l'explique.
	*/
	if !strings.HasPrefix(best.CoverURL, "https://") {
		t.Errorf("couverture = %q, attendue en HTTPS", best.CoverURL)
	}
	if best.Summary == "" || best.PageCount != 128 {
		t.Errorf("fiche incomplète : %+v", best)
	}
}

// ─── Internet Archive ────────────────────────────────────────────────────────

func TestInternetArchiveDescribe(t *testing.T) {
	var seen *url.URL

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL
		w.Header().Set("Content-Type", "application/json")
		/*
			`creator` est ici un tableau sur le premier document et une chaîne
			sur le second, et `description` l'inverse. Les deux formes se
			rencontrent dans une même réponse réelle : c'est précisément ce que
			ce test existe pour couvrir.
		*/
		_, _ = w.Write([]byte(`{
  "response": {
    "numFound": 2,
    "docs": [
      {
        "identifier": "incal-integrale",
        "title": "L'Incal",
        "creator": ["Jodorowsky", "Moebius"],
        "description": "Édition intégrale.",
        "language": "French",
        "subject": ["bande dessinée"],
        "year": "1981"
      },
      {
        "identifier": "incal-autre",
        "title": "L'Incal noir",
        "creator": "Moebius",
        "description": ["Premier tome."],
        "language": "fre",
        "date": "1981-03-01T00:00:00Z"
      }
    ]
  }
}`))
	}))
	defer server.Close()

	provider := NewInternetArchive(testDeps())
	provider.baseURL = server.URL

	got, err := provider.Describe(context.Background(), Work{
		Title:   "L'Incal",
		Authors: []string{"Moebius"},
	})
	if err != nil {
		t.Fatalf("describe : %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("%d candidats, attendu 2 : %+v", len(got), got)
	}

	/*
		Sans restriction de type de média, l'Archive rend des films et des
		concerts portant le même mot. La confiance n'a pas les moyens de
		distinguer un livre d'un enregistrement sonore.
	*/
	if !strings.Contains(seen.Query().Get("q"), "mediatype:(texts)") {
		t.Errorf("requête sans restriction de média : %q", seen.Query().Get("q"))
	}

	byTitle := map[string]Description{}
	for _, candidate := range got {
		byTitle[candidate.Title] = candidate
	}

	first := byTitle["L'Incal"]
	if len(first.Authors) != 2 {
		t.Errorf("auteurs en tableau mal lus : %v", first.Authors)
	}
	if first.Summary != "Édition intégrale." {
		t.Errorf("résumé en chaîne mal lu : %q", first.Summary)
	}
	if first.Language != "fr" {
		t.Errorf("langue = %q : « French » doit être ramené à fr", first.Language)
	}
	if first.PageURL != "https://archive.org/details/incal-integrale" {
		t.Errorf("fiche = %q", first.PageURL)
	}

	second := byTitle["L'Incal noir"]
	if len(second.Authors) != 1 || second.Authors[0] != "Moebius" {
		t.Errorf("auteur en chaîne mal lu : %v", second.Authors)
	}
	if second.Summary != "Premier tome." {
		t.Errorf("résumé en tableau mal lu : %q", second.Summary)
	}
	// `date` est un horodatage complet : le garder entier ferait échouer la
	// comparaison à une année sur quatre chiffres.
	if second.Published != "1981" {
		t.Errorf("date = %q, attendue réduite à l'année", second.Published)
	}
}

// ─── Communs ─────────────────────────────────────────────────────────────────

/*
TestMetadataUsesCache est le test qui rend la politesse mesurable.

Une base publique interrogée deux fois pour la même œuvre est une requête de
trop. Sur un enrichissement de série, où trente tomes partagent un auteur, la
différence n'est pas anecdotique.
*/
func TestMetadataUsesCache(t *testing.T) {
	calls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"docs":[{"key":"/works/OL1W","title":"L'Incal"}]}`))
	}))
	defer server.Close()

	provider := NewOpenLibrary(testDeps())
	provider.baseURL = server.URL

	work := Work{Title: "L'Incal"}
	for i := 0; i < 4; i++ {
		if _, err := provider.Describe(context.Background(), work); err != nil {
			t.Fatal(err)
		}
	}

	if calls != 1 {
		t.Errorf("%d appels pour la même œuvre, attendu 1", calls)
	}
}

// TestMetadataRefusesNonOK : une base en panne rend une erreur, pas une liste
// vide.
//
// La distinction compte : « aucun candidat » et « la base n'a pas répondu »
// s'affichent différemment, et confondre les deux ferait croire qu'une œuvre
// est introuvable alors que personne n'a cherché.
func TestMetadataRefusesNonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	provider := NewGoogleBooks("", testDeps())
	provider.baseURL = server.URL

	_, err := provider.Describe(context.Background(), Work{Title: "L'Incal"})
	if err == nil {
		t.Fatal("un 429 doit remonter comme une erreur")
	}
	if failureCode(err) == "" {
		t.Error("l'erreur doit porter un code traduisible")
	}
}

// TestMetadataEmptyWork : sans rien à chercher, on n'interroge personne.
func TestMetadataEmptyWork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	for _, provider := range []DescriptionProvider{
		func() DescriptionProvider { p := NewOpenLibrary(testDeps()); p.baseURL = server.URL; return p }(),
		func() DescriptionProvider { p := NewGoogleBooks("", testDeps()); p.baseURL = server.URL; return p }(),
		func() DescriptionProvider { p := NewInternetArchive(testDeps()); p.baseURL = server.URL; return p }(),
	} {
		if _, err := provider.Describe(context.Background(), Work{}); err != nil {
			t.Errorf("%s : %v", provider.Kind(), err)
		}
	}

	if calls != 0 {
		t.Errorf("%d requêtes pour une œuvre vide", calls)
	}
}
