package scraper

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

/*
Le gabarit, sa validation et ses défauts.

Ce qu'on vérifie ici n'est pas que le YAML se décode — `yaml.v3` sait le faire.
C'est que les gabarits FAUTIFS soient refusés au chargement, avec un message qui
dise quoi corriger.

C'est le cœur du marché passé en abandonnant le compilateur : puisqu'un gabarit
n'est plus vérifié par lui, il doit l'être ici, et complètement. Un gabarit qui
se charge mais ne rend rien est le pire des échecs — il ressemble à une
recherche infructueuse.
*/

func loadReference(t *testing.T) *Compiled {
	t.Helper()

	raw, err := os.ReadFile("testdata/comicshelf.yaml")
	if err != nil {
		t.Fatalf("gabarit de référence illisible : %v", err)
	}
	compiled, err := Parse(raw)
	if err != nil {
		t.Fatalf("gabarit de référence refusé : %v", err)
	}
	return compiled
}

func TestParseReference(t *testing.T) {
	compiled := loadReference(t)

	if compiled.ID != "comicshelf" {
		t.Errorf("id = %q, attendu comicshelf", compiled.ID)
	}
	if got := compiled.Timeout(); got != 5*time.Second {
		t.Errorf("timeout = %v, attendu 5s", got)
	}
	if len(compiled.Mirrors) != 2 {
		t.Errorf("miroirs = %d, attendu 2", len(compiled.Mirrors))
	}

	// Les hôtes déclarés bornent ce qu'un import a le droit de joindre : s'ils
	// se perdaient à la compilation, tout téléchargement serait refusé.
	if !compiled.AllowsHost("comicshelf.example") ||
		!compiled.AllowsHost("mirror.comicshelf.example") {
		t.Errorf("hôtes = %v, les deux miroirs sont attendus", compiled.Hosts())
	}
	if compiled.AllowsHost("ailleurs.example") {
		t.Error("un hôte non déclaré est accepté")
	}
}

// Les défauts doivent s'appliquer AVANT la validation, sans quoi un gabarit
// minimal — celui qu'on écrit en premier — serait refusé pour des bornes qu'il
// n'a pas à connaître.
func TestParseAppliesDefaults(t *testing.T) {
	compiled := mustParse(t, `
id: minimal
name: Minimal
mirrors: [https://minimal.example]
search:
  path: /q
  query: {s: "{terms}"}
results:
  select: "div.row"
  fields:
    title: {select: "a"}
`)

	if compiled.Search.Method != "GET" {
		t.Errorf("method = %q, attendu GET", compiled.Search.Method)
	}
	if compiled.Limits.Timeout.Std() != defaultTimeout {
		t.Errorf("timeout = %v, attendu %v", compiled.Limits.Timeout.Std(), defaultTimeout)
	}
	if compiled.Limits.MaxResults != defaultMaxResults {
		t.Errorf("maxResults = %d, attendu %d", compiled.Limits.MaxResults, defaultMaxResults)
	}
	if compiled.Search.Probe == "" {
		t.Error("le terme d'essai devrait avoir un défaut")
	}
}

/*
Chaque cas est un défaut qu'on a de vraies chances de commettre en écrivant un
gabarit à la main, et dont le symptôme SANS validation serait « la source ne
rend rien » — indiscernable d'une recherche infructueuse.

Le message attendu est vérifié, pas seulement l'échec : un refus qui ne dit pas
quoi corriger oblige à relire le format en entier.
*/
func TestParseRejects(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		mustSay string
	}{
		{
			name: "sans {terms} la recherche rendrait toujours la même page",
			yaml: `
id: x
name: X
mirrors: [https://x.example]
search: {path: /q, query: {s: batman}}
results: {select: "div", fields: {title: {select: a}}}`,
			mustSay: "{terms}",
		},
		{
			name: "sans titre, aucun résultat ne survivrait au filtre du service",
			yaml: `
id: x
name: X
mirrors: [https://x.example]
search: {path: "/q/{terms}"}
results: {select: "div", fields: {coverUrl: {select: img, from: attr, attr: src}}}`,
			mustSay: "results.fields.title",
		},
		{
			name: "un sélecteur CSS invalide",
			yaml: `
id: x
name: X
mirrors: [https://x.example]
search: {path: "/q/{terms}"}
results: {select: "div", fields: {title: {select: "a[["}}}`,
			mustSay: "sélecteur invalide",
		},
		{
			name: "une faute de frappe dans un nom de champ",
			yaml: `
id: x
name: X
mirrors: [https://x.example]
search: {path: "/q/{terms}"}
results: {select: "div", fields: {title: {select: a}, coverURL: {select: img}}}`,
			mustSay: "champ inconnu",
		},
		{
			name: "from: attr sans nom d'attribut",
			yaml: `
id: x
name: X
mirrors: [https://x.example]
search: {path: "/q/{terms}"}
results: {select: "div", fields: {title: {select: a, from: attr}}}`,
			mustSay: "attr : requis",
		},
		{
			name: "une base qui n'est pas une adresse http",
			yaml: `
id: x
name: X
mirrors: ["ftp://x.example"]
search: {path: "/q/{terms}"}
results: {select: "div", fields: {title: {select: a}}}`,
			mustSay: "mirrors",
		},
		{
			name: "un budget plus court qu'un délai unitaire",
			yaml: `
id: x
name: X
mirrors: [https://x.example]
limits: {timeout: 30s, budget: 10s}
search: {path: "/q/{terms}"}
results: {select: "div", fields: {title: {select: a}}}`,
			mustSay: "limits.budget",
		},
		{
			name: "une clé que le format ne connaît pas",
			yaml: `
id: x
name: X
mirrors: [https://x.example]
search: {path: "/q/{terms}"}
results: {select: "div", fields: {title: {select: a}}}
selectors: {title: a}`,
			mustSay: "selectors",
		},
		{
			name: "un identifiant qui n'est pas un identifiant",
			yaml: `
id: "Comic Shelf!"
name: X
mirrors: [https://x.example]
search: {path: "/q/{terms}"}
results: {select: "div", fields: {title: {select: a}}}`,
			mustSay: "id",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte(c.yaml))
			if err == nil {
				t.Fatal("gabarit accepté alors qu'il est fautif")
			}
			if !errors.Is(err, ErrInvalidTemplate) {
				t.Errorf("erreur = %v, ErrInvalidTemplate attendue", err)
			}
			if !strings.Contains(err.Error(), c.mustSay) {
				t.Errorf("le message ne mentionne pas %q :\n%v", c.mustSay, err)
			}
		})
	}
}

// Les problèmes sont accumulés : celui qui corrige un gabarit doit les voir
// tous d'un coup, pas les découvrir un par un.
func TestParseReportsEveryProblem(t *testing.T) {
	_, err := Parse([]byte(`
id: x
name: X
mirrors: [https://x.example]
search: {path: /q, query: {s: batman}}
results: {select: "div", fields: {coverUrl: {select: img, from: attr}}}`))

	if err == nil {
		t.Fatal("gabarit accepté")
	}
	for _, want := range []string{"{terms}", "results.fields.title", "attr : requis"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("le message omet %q :\n%v", want, err)
		}
	}
}

// Un débit par hôte, et le plus prudent l'emporte quand deux gabarits se
// partagent une machine : c'est elle qui encaisse, pas le gabarit.
func TestCatalogRatesTakeTheStrictestPerHost(t *testing.T) {
	catalog := &Catalog{byID: map[string]*Compiled{
		"rapide": mustParse(t, `
id: rapide
name: Rapide
mirrors: [https://commun.example, https://propre.example]
rate: {every: 500ms, burst: 4}
search: {path: "/q/{terms}"}
results: {select: div, fields: {title: {select: a}}}`),
		"lent": mustParse(t, `
id: lent
name: Lent
mirrors: [https://commun.example]
rate: {every: 3s, burst: 1}
search: {path: "/q/{terms}"}
results: {select: div, fields: {title: {select: a}}}`),
	}}

	rates := catalog.rates()

	if got := rates["commun.example"].Every; got != 3*time.Second {
		t.Errorf("hôte partagé : %v, attendu 3s (le plus prudent)", got)
	}
	if got := rates["propre.example"].Every; got != 500*time.Millisecond {
		t.Errorf("hôte non partagé : %v, attendu 500ms", got)
	}
}

func mustParse(t *testing.T, yaml string) *Compiled {
	t.Helper()

	compiled, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("gabarit refusé : %v", err)
	}
	return compiled
}
