package scraper

import (
	"os"
	"strings"
	"testing"
)

/*
Le gabarit JSON, contre une vraie réponse.

`testdata/gutendex-search.json` a été ENREGISTRÉ depuis l'API. Comme pour les
pages HTML, cela rend la partie fragile vérifiable sans réseau — et cela a la
même limite, qu'il vaut mieux nommer : ce test dit que les chemins lisent CETTE
réponse, pas que l'API n'a pas changé depuis. C'est `Probe` qui le dira.
*/

func loadArchiveJSON(t *testing.T) *Compiled {
	t.Helper()

	raw, err := os.ReadFile("../../../../../deploy/scraper-templates/internetarchive.yaml")
	if err != nil {
		t.Fatalf("gabarit illisible : %v", err)
	}
	compiled, err := Parse(raw)
	if err != nil {
		t.Fatalf("gabarit refusé : %v", err)
	}
	return compiled
}

func TestJSONTemplateReadsTheRealAPI(t *testing.T) {
	template := loadArchiveJSON(t)

	if template.Format != FormatJSON {
		t.Fatalf("format = %q", template.Format)
	}

	body, err := os.ReadFile("testdata/internetarchive-search.json")
	if err != nil {
		t.Fatal(err)
	}

	results := extractJSON(body, template,
		"https://archive.org/advancedsearch.php", 25)

	if len(results) < 5 {
		t.Fatalf("résultats = %d", len(results))
	}

	first := results[0]

	if first.Title == "" {
		t.Error("titre vide")
	}

	// `creator` arrive en chaîne ou en tableau selon le document, parfois les
	// deux formes dans une même réponse. Les deux doivent donner une liste.
	if len(first.Authors) == 0 {
		t.Errorf("auteurs = %v", first.Authors)
	}

	/*
		`year` est un ENTIER dans la réponse.

		Une extraction qui n'accepterait que les chaînes le perdrait sans rien
		signaler — la date disparaîtrait de la fiche, et personne ne saurait
		pourquoi.
	*/
	if first.Published == "" {
		t.Error("année perdue : un entier doit être converti, pas écarté")
	}

	// Cette API ne rend pas de lien de téléchargement : c'est documenté dans le
	// gabarit, et ce test fige la limite pour qu'elle ne surprenne personne.
	if len(first.Acquisitions) != 0 {
		t.Errorf("acquisitions inattendues : %+v", first.Acquisitions)
	}
}

// Un chemin vide désigne la racine : certaines API rendent un tableau nu, et
// exiger un chemin obligerait à en inventer un.
func TestJSONAcceptsARootArray(t *testing.T) {
	template := mustParse(t, `
id: nu
name: Tableau nu
format: json
mirrors: [https://x.example]
search: {path: /s, query: {q: "{terms}"}}
results:
  fields:
    title: {select: nom}`)

	results := extractJSON([]byte(`[{"nom":"Un"},{"nom":"Deux"}]`), template, "https://x.example", 10)

	if len(results) != 2 || results[0].Title != "Un" {
		t.Fatalf("résultats = %+v", results)
	}
}

/*
Les en-têtes de provenance sont refusés par la VALIDATION.

C'est le seul endroit où ce refus tient : une règle qu'on peut contourner en
écrivant deux lignes de YAML n'est pas une règle.
*/
func TestForgeableHeadersAreRefused(t *testing.T) {
	for _, header := range []string{"User-Agent", "Referer", "Origin", "Host", "ORIGIN"} {
		t.Run(header, func(t *testing.T) {
			_, err := Parse([]byte(`
id: x
name: X
mirrors: [https://x.example]
search:
  path: /s
  query: {q: "{terms}"}
  headers:
    ` + header + `: "peu importe"
results: {select: div, fields: {title: {select: a}}}`))

			if err == nil {
				t.Fatal("en-tête accepté")
			}
			if !strings.Contains(err.Error(), "refusé") {
				t.Errorf("message = %v", err)
			}
		})
	}
}

// Un en-tête d'authentification légitime passe, lui — c'est tout l'intérêt.
func TestAuthHeaderIsAllowed(t *testing.T) {
	compiled := mustParse(t, `
id: avecle
name: Avec clé
format: json
mirrors: [https://api.example]
search:
  path: /s
  query: {q: "{terms}"}
  authHeader: Authorization
results:
  select: items
  fields:
    title: {select: title}`)

	if compiled.Search.AuthHeader != "Authorization" {
		t.Errorf("authHeader = %q", compiled.Search.AuthHeader)
	}
}

// …sauf s'il sert à forger une provenance sous un autre nom.
func TestAuthHeaderCannotForgeOrigin(t *testing.T) {
	_, err := Parse([]byte(`
id: x
name: X
format: json
mirrors: [https://x.example]
search: {path: /s, query: {q: "{terms}"}, authHeader: Referer}
results: {select: items, fields: {title: {select: t}}}`))

	if err == nil || !strings.Contains(err.Error(), "refusé") {
		t.Fatalf("erreur = %v", err)
	}
}
