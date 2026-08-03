package scraper

import (
	"errors"
	"os"
	"strings"
	"testing"
)

/*
La description saisie au formulaire, jusqu'au moteur.

Ce que ces tests protègent n'est pas la traduction en elle-même, mais le fait
qu'elle produise EXACTEMENT ce que produit un fichier YAML. Le jour où les deux
chemins divergent, la version « simple » se met à avoir ses propres défauts, et
personne ne le remarque avant qu'un utilisateur ne le signale.
*/

func TestWebSpecCompilesLikeAFile(t *testing.T) {
	compiled, err := WebSpec{
		SearchURL: "https://standardebooks.org/ebooks?query={terms}",
		Row:       "ol.ebooks-list > li",
		Title:     `p:not([class]) span[property="schema:name"]`,
		Author:    `p.author span[property="schema:name"]`,
		Cover:     "div.thumbnail-container img",
		Link:      "p:not([class]) a",
	}.Compile()
	if err != nil {
		t.Fatalf("description refusée : %v", err)
	}

	// L'adresse est découpée par la machine : l'utilisateur colle une URL, il
	// n'a pas à séparer domaine, chemin et paramètre à la main.
	if got := compiled.Mirrors; len(got) != 1 || got[0] != "https://standardebooks.org" {
		t.Errorf("miroir = %v", got)
	}
	if compiled.Search.Path != "/ebooks" {
		t.Errorf("chemin = %q", compiled.Search.Path)
	}
	if compiled.Search.Query["query"] != "{terms}" {
		t.Errorf("paramètre = %q, le marqueur doit survivre à l'analyse d'URL",
			compiled.Search.Query["query"])
	}

	/*
		L'épreuve du feu : ces règles doivent lire la vraie page, exactement
		comme le gabarit sur disque. Même fixture, même attentes.
	*/
	page, err := os.ReadFile("testdata/standardebooks-search.html")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseHTML(page)
	if err != nil {
		t.Fatal(err)
	}

	results := extractResults(doc, compiled,
		"https://standardebooks.org/ebooks?query=verne", 30)

	if len(results) < 5 {
		t.Fatalf("résultats = %d", len(results))
	}
	if results[0].Title != "The Giant Raft" {
		t.Errorf("titre = %q", results[0].Title)
	}
	if len(results[0].Authors) == 0 || results[0].Authors[0] != "Jules Verne" {
		t.Errorf("auteurs = %v", results[0].Authors)
	}
	// Le lien sert de fiche ET d'acquisition : on ne sait pas lequel il est.
	if results[0].PageURL == "" || len(results[0].Acquisitions) == 0 {
		t.Errorf("lien non repris des deux côtés : %+v", results[0])
	}
}

func TestWebSpecRejects(t *testing.T) {
	cases := map[string]WebSpec{
		"sans adresse": {Row: "li", Title: "h3"},
		"sans {terms}": {SearchURL: "https://x.example/s?q=verne", Row: "li", Title: "h3"},
		"sans ligne":   {SearchURL: "https://x.example/s?q={terms}", Title: "h3"},
		"sans titre":   {SearchURL: "https://x.example/s?q={terms}", Row: "li"},
		"pas en http":  {SearchURL: "ftp://x.example/s?q={terms}", Row: "li", Title: "h3"},
		"sélecteur cassé": {
			SearchURL: "https://x.example/s?q={terms}", Row: "li", Title: "h3[[",
		},
	}

	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := spec.Compile(); err == nil {
				t.Fatal("description acceptée alors qu'elle est fautive")
			} else if !errors.Is(err, ErrInvalidTemplate) {
				t.Errorf("erreur = %v", err)
			}
		})
	}
}

// Sans `{terms}`, la même page reviendrait pour toute recherche : des résultats
// s'afficheraient, toujours les mêmes. Le message doit donc nommer le marqueur.
func TestWebSpecExplainsTheMissingMarker(t *testing.T) {
	_, err := WebSpec{
		SearchURL: "https://x.example/s?q=verne", Row: "li", Title: "h3",
	}.Compile()

	if err == nil || !strings.Contains(err.Error(), "{terms}") {
		t.Fatalf("message = %v", err)
	}
}

/*
Une API JSON décrite au formulaire.

Les six champs gardent leur sens, seule leur nature change. Ce test vérifie que
la traduction n'emporte pas d'options propres au HTML — `from: attr` sur un
chemin JSON ferait un gabarit qui se charge et ne remplit rien, la panne la plus
coûteuse à diagnostiquer.
*/
func TestWebSpecCompilesJSON(t *testing.T) {
	compiled, err := WebSpec{
		Format:    FormatJSON,
		SearchURL: "https://archive.org/advancedsearch.php?q={terms}&output=json",
		Row:       "response.docs",
		Title:     "title",
		Author:    "creator",
		Cover:     "cover_url",
		Link:      "identifier",
	}.Compile()
	if err != nil {
		t.Fatalf("description refusée : %v", err)
	}

	if compiled.Format != FormatJSON {
		t.Fatalf("format = %q", compiled.Format)
	}
	for name, field := range compiled.fields {
		if field.From != "" || field.Attr != "" || field.All {
			t.Errorf("%s porte une option HTML : %+v", name, field.FieldSpec)
		}
	}

	body := []byte(`{"response":{"docs":[{"title":"Un titre","creator":"Une autrice"}]}}`)
	results := extractJSON(body, compiled, "https://archive.org/", 10)

	if len(results) != 1 || results[0].Title != "Un titre" {
		t.Fatalf("résultats = %+v", results)
	}
	if len(results[0].Authors) == 0 || results[0].Authors[0] != "Une autrice" {
		t.Errorf("auteurs = %v", results[0].Authors)
	}
}
