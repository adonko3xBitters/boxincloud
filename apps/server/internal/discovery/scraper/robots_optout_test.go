package scraper

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adonko3xBitters/boxincloud/server/internal/discovery"
)

/*
La dérogation à robots.txt.

Deux tests, et le premier compte autant que le second : il vérifie que le refus
fonctionne toujours. Une option qui désactive un garde-fou doit prouver que le
garde-fou existe, sans quoi elle ne désactive rien et on ne s'en aperçoit qu'en
lisant les journaux d'un site mécontent.
*/

func robotsClosedServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /\n"))
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(searchPage))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestRobotsRefusesByDefault(t *testing.T) {
	server := robotsClosedServer(t)

	template := loadReference(t)
	client := clientFor(t, template, server.URL)

	_, err := client.Search(context.Background(),
		sourceFor(template, ""), "", discovery.Query{Text: "fantastic"})

	if !errors.Is(err, ErrDisallowed) {
		t.Fatalf("erreur = %v, ErrDisallowed attendue — le garde-fou doit mordre", err)
	}
}

func TestIgnoreRobotsLetsTheSourceThrough(t *testing.T) {
	server := robotsClosedServer(t)

	template := loadReference(t)
	template.IgnoreRobots = true
	client := clientFor(t, template, server.URL)

	results, err := client.Search(context.Background(),
		sourceFor(template, ""), "", discovery.Query{Text: "fantastic"})
	if err != nil {
		t.Fatalf("la dérogation n'a pas pris : %v", err)
	}
	if len(results) == 0 {
		t.Fatal("aucun résultat alors que le site répond")
	}
}

// La dérogation voyage depuis la description saisie au formulaire, sans quoi
// l'option serait réservée aux gabarits sur disque — c'est-à-dire à ceux qui
// ont un accès au serveur.
func TestWebSpecCarriesIgnoreRobots(t *testing.T) {
	compiled, err := WebSpec{
		SearchURL:    "https://x.example/s?q={terms}",
		Row:          "li",
		Title:        "h3",
		IgnoreRobots: true,
	}.Compile()
	if err != nil {
		t.Fatal(err)
	}
	if !compiled.IgnoreRobots {
		t.Error("l'option est perdue à la compilation")
	}
}
