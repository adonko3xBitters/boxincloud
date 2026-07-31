package discovery

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
)

/*
Le client OPDS, contre de vrais serveurs.

Des serveurs de test plutôt que des fichiers d'exemple : ce qui casse dans un
client de catalogue n'est presque jamais l'analyse du document, c'est le chemin
qui y mène — la découverte du lien de recherche, l'expansion du gabarit, la
résolution des adresses relatives. Un test qui part d'un document déjà chargé
saute précisément la partie fragile.

Les deux serveurs imitent des implémentations réelles : le premier suit la voie
normative d'OPDS 1.2 — flux Atom, lien vers un document OpenSearch, gabarit
`{searchTerms}` — comme Calibre-Web ou Standard Ebooks ; le second sert de
l'OPDS 2.0 en JSON avec un lien de recherche `{?query}`, comme Komga.
*/

// opds1Server imite un catalogue OPDS 1.2 complet.
//
// Les adresses qu'il publie sont relatives, comme dans la nature : c'est la
// forme qui casse un client qui oublierait de les résoudre.
func opds1Server(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/opds", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml;profile=opds-catalog")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Catalogue d'essai</title>
  <link rel="search" type="application/opensearchdescription+xml" href="search.xml"/>
</feed>`))
	})

	mux.HandleFunc("/search.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/opensearchdescription+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<OpenSearchDescription xmlns="http://a9.com/-/spec/opensearch/1.1/">
  <ShortName>Essai</ShortName>
  <Url type="text/html" template="/html?q={searchTerms}"/>
  <Url type="application/atom+xml;profile=opds-catalog"
       template="/opds/find?q={searchTerms}&amp;lang={language?}"/>
</OpenSearchDescription>`))
	})

	mux.HandleFunc("/opds/find", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") == "" {
			t.Errorf("recherche sans terme : %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/atom+xml;profile=opds-catalog")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:dc="http://purl.org/dc/terms/">
  <title>Résultats</title>
  <entry>
    <title>L'Incal</title>
    <author><name>Alejandro Jodorowsky</name></author>
    <author><name>Mœbius</name></author>
    <summary>John Difool et l'Incal lumière.</summary>
    <dc:language>fr</dc:language>
    <dc:issued>1981</dc:issued>
    <link rel="http://opds-spec.org/image" href="/covers/incal.jpg" type="image/jpeg"/>
    <link rel="http://opds-spec.org/acquisition"
          href="/download/incal.cbz" type="application/vnd.comicbook+zip"/>
    <link rel="alternate" type="text/html" href="/livre/incal"/>
  </entry>
  <entry>
    <title>Catégorie sans titre utile</title>
    <link rel="subsection" href="/opds/cat/1"/>
  </entry>
  <entry>
    <link rel="subsection" href="/opds/cat/2"/>
  </entry>
</feed>`))
	})

	return httptest.NewServer(mux)
}

// opds2Server imite un catalogue OPDS 2.0 en JSON.
func opds2Server(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/opds/v2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/opds+json")
		_, _ = w.Write([]byte(`{
  "metadata": {"title": "Komga"},
  "links": [
    {"rel": "self", "href": "/opds/v2", "type": "application/opds+json"},
    {"rel": ["search"], "href": "/opds/v2/search{?query}",
     "type": "application/opds+json", "templated": true}
  ]
}`))
	})

	mux.HandleFunc("/opds/v2/search", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") == "" {
			t.Errorf("recherche sans terme : %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/opds+json")
		_, _ = w.Write([]byte(`{
  "metadata": {"title": "Résultats"},
  "publications": [
    {
      "metadata": {
        "title": "L'Incal",
        "author": [{"name": "Alejandro Jodorowsky"}],
        "language": ["fr"],
        "published": "1981",
        "description": "Édition intégrale.",
        "belongsTo": {"series": [{"name": "L'Incal"}]}
      },
      "links": [
        {"rel": "http://opds-spec.org/acquisition",
         "href": "/api/v1/books/42/file", "type": "application/vnd.comicbook+zip"}
      ],
      "images": [{"href": "/api/v1/books/42/thumbnail", "type": "image/jpeg"}]
    },
    {
      "metadata": {"title": "Le Garage hermétique", "author": "Mœbius"},
      "links": [{"rel": "http://opds-spec.org/acquisition", "href": "/api/v1/books/43/file"}]
    }
  ]
}`))
	})

	return httptest.NewServer(mux)
}

func sourceFor(server *httptest.Server, path, name string) Source {
	return Source{
		ID:      uuid.New(),
		Name:    name,
		URL:     server.URL + path,
		Kind:    KindOPDS,
		Enabled: true,
	}
}

func TestOPDS1Search(t *testing.T) {
	server := opds1Server(t)
	defer server.Close()

	source := sourceFor(server, "/opds", "Catalogue Atom")
	results, err := NewOPDSClient().Search(
		context.Background(), source, "", Query{Text: "incal"})
	if err != nil {
		t.Fatalf("recherche : %v", err)
	}

	// Deux des trois entrées n'ont pas de titre utilisable : ce sont des
	// catégories, et les afficher donnerait des lignes vides.
	if len(results) != 1 {
		t.Fatalf("%d résultats, attendu 1 : %+v", len(results), results)
	}

	got := results[0]
	if got.Title != "L'Incal" {
		t.Errorf("titre = %q", got.Title)
	}
	if len(got.Authors) != 2 || got.Authors[0] != "Alejandro Jodorowsky" {
		t.Errorf("auteurs = %v", got.Authors)
	}
	if got.Language != "fr" || got.Published != "1981" {
		t.Errorf("langue/date = %q/%q", got.Language, got.Published)
	}
	if got.SourceName != "Catalogue Atom" || got.SourceID != source.ID {
		t.Errorf("provenance perdue : %+v", got)
	}

	// Les adresses du flux sont relatives ; celles qu'on rend doivent être
	// absolues, sinon le navigateur les résoudrait contre boxincloud.
	for label, link := range map[string]string{
		"couverture": got.CoverURL,
		"fiche":      got.PageURL,
	} {
		if !strings.HasPrefix(link, server.URL) {
			t.Errorf("%s non résolue : %q", label, link)
		}
	}
	if len(got.Acquisitions) != 1 || !strings.HasPrefix(got.Acquisitions[0].Href, server.URL) {
		t.Errorf("acquisitions = %+v", got.Acquisitions)
	}
}

func TestOPDS2Search(t *testing.T) {
	server := opds2Server(t)
	defer server.Close()

	results, err := NewOPDSClient().Search(
		context.Background(), sourceFor(server, "/opds/v2", "Komga"), "", Query{Text: "incal"})
	if err != nil {
		t.Fatalf("recherche : %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("%d résultats, attendu 2", len(results))
	}

	first := results[0]
	if first.Series != "L'Incal" || first.Language != "fr" {
		t.Errorf("série/langue = %q/%q", first.Series, first.Language)
	}
	if !strings.HasPrefix(first.CoverURL, server.URL) {
		t.Errorf("couverture non résolue : %q", first.CoverURL)
	}

	// L'auteur du second est une chaîne nue, pas un objet : le format autorise
	// les deux, et un catalogue sur deux choisit l'autre.
	if len(results[1].Authors) != 1 || results[1].Authors[0] != "Mœbius" {
		t.Errorf("auteur en forme courte mal lu : %v", results[1].Authors)
	}
}

// TestNoSearchLink couvre le catalogue qui n'expose pas de recherche.
//
// Le cas est courant — un flux OPDS statique, une liste de nouveautés — et
// n'est pas une panne : il doit se dire à l'utilisateur tel quel.
func TestNoSearchLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom">
  <title>Nouveautés</title></feed>`))
	}))
	defer server.Close()

	_, err := NewOPDSClient().Search(
		context.Background(), sourceFor(server, "/", "Statique"), "", Query{Text: "x"})
	if !errors.Is(err, ErrNoSearch) {
		t.Fatalf("err = %v, attendu ErrNoSearch", err)
	}
	if failureCode(err) != "no-search" {
		t.Errorf("code = %q", failureCode(err))
	}
}

// TestBasicAuth vérifie que les identifiants du catalogue sont bien envoyés.
func TestBasicAuth(t *testing.T) {
	var seen bool

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "lecteur" || password != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		seen = true
		w.Header().Set("Content-Type", "application/opds+json")
		_, _ = w.Write([]byte(`{"links":[{"rel":"search","href":"/s{?query}","templated":true}]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	source := sourceFor(server, "/", "Privé")
	source.Username = "lecteur"

	if err := NewOPDSClient().Probe(context.Background(), source, "secret"); err != nil {
		t.Fatalf("essai : %v", err)
	}
	if !seen {
		t.Error("le catalogue n'a jamais reçu d'identifiants")
	}
}

// TestRefusesForbiddenAddress vérifie que le garde-fou couvre aussi les
// catalogues.
//
// Une URL de catalogue est saisie depuis l'administration et jointe par le
// serveur : c'est exactement la SSRF que le contrôle des backends arrête.
func TestRefusesForbiddenAddress(t *testing.T) {
	source := Source{
		ID:      uuid.New(),
		Name:    "Métadonnées d'instance",
		URL:     "http://169.254.169.254/latest/meta-data/",
		Kind:    KindOPDS,
		Enabled: true,
	}

	err := NewOPDSClient().Probe(context.Background(), source, "")
	if !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("err = %v, attendu ErrInvalidSource", err)
	}
}

func TestExpandTemplate(t *testing.T) {
	cases := []struct {
		name     string
		template string
		terms    string
		want     string
	}{
		{
			name:     "OpenSearch",
			template: "https://x.fr/find?q={searchTerms}",
			terms:    "l'incal",
			want:     "https://x.fr/find?q=l%27incal",
		},
		{
			name:     "OpenSearch avec variable non remplie",
			template: "https://x.fr/find?q={searchTerms}&lang={language?}",
			terms:    "incal",
			want:     "https://x.fr/find?q=incal&lang=",
		},
		{
			name:     "expansion de formulaire",
			template: "https://x.fr/search{?query}",
			terms:    "incal",
			want:     "https://x.fr/search?query=incal",
		},
		{
			name:     "expansion de formulaire à plusieurs variables",
			template: "https://x.fr/search{?query,author}",
			terms:    "incal",
			want:     "https://x.fr/search?query=incal",
		},
		{
			name:     "expansion après un paramètre existant",
			template: "https://x.fr/search?type=book{?query}",
			terms:    "incal",
			want:     "https://x.fr/search?type=book&query=incal",
		},
		{
			name:     "aucun marqueur : on ajoute le paramètre conventionnel",
			template: "https://x.fr/search",
			terms:    "incal",
			want:     "https://x.fr/search?query=incal",
		},
		{
			name:     "les espaces sont échappés",
			template: "https://x.fr/find?q={searchTerms}",
			terms:    "le garage hermétique",
			want:     "https://x.fr/find?q=" + url.QueryEscape("le garage hermétique"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := expandTemplate(tc.template, tc.terms); got != tc.want {
				t.Errorf("expandTemplate(%q) = %q, attendu %q", tc.template, got, tc.want)
			}
		})
	}
}

func TestNormalizeTitle(t *testing.T) {
	// Deux catalogues n'écrivent presque jamais un titre de la même façon.
	// Toutes ces formes doivent se rejoindre, sans quoi la déduplication ne
	// déduplique rien.
	same := []string{
		"L'Incal",
		"l'incal",
		"L’INCAL",
		"  L'Incal  ",
		"L'Incal !",
	}
	want := normalizeTitle(same[0])
	for _, form := range same[1:] {
		if got := normalizeTitle(form); got != want {
			t.Errorf("normalizeTitle(%q) = %q, attendu %q", form, got, want)
		}
	}

	if normalizeTitle("Épée") != normalizeTitle("epee") {
		t.Error("les accents doivent se replier")
	}
	if normalizeTitle("Tome 1") == normalizeTitle("Tome 2") {
		t.Error("deux tomes distincts ne doivent pas se confondre")
	}
}
