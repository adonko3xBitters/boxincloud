package scraper

import (
	"context"
	"os"
	"testing"

	"github.com/adonko3xBitters/boxincloud/server/internal/discovery"
)

// Épreuve contre l'API vivante. Hors de la suite par défaut : un test qui sort
// sur le réseau est lent, instable et impoli envers un service bénévole.
func TestLiveArchiveJSON(t *testing.T) {
	if os.Getenv("BOXINCLOUD_LIVE") == "" {
		t.Skip("BOXINCLOUD_LIVE non défini")
	}

	template := loadArchiveJSON(t)
	catalog := &Catalog{byID: map[string]*Compiled{template.ID: template}}
	client := New(catalog, Deps{})
	source := sourceFor(template, "")

	if err := client.Probe(context.Background(), source, ""); err != nil {
		t.Fatalf("essai : %v", err)
	}

	results, err := client.Search(context.Background(), source, "",
		discovery.Query{Text: "verne", Limit: 5})
	if err != nil {
		t.Fatalf("recherche : %v", err)
	}
	if len(results) == 0 {
		t.Fatal("aucun résultat")
	}

	t.Logf("%d résultats — premier : %q par %v", len(results), results[0].Title, results[0].Authors)

	// Pas d'assertion sur l'acquisition : cette API rend un identifiant, pas un
	// fichier. La limite est documentée dans le gabarit et figée par le test
	// hors ligne ; l'exiger ici ferait échouer un essai qui a réussi.
	if len(results[0].Acquisitions) > 0 {
		t.Logf("lien : %s", results[0].Acquisitions[0].Href)
	}
}
