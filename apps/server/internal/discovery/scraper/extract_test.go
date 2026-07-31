package scraper

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/adonko3xBitters/boxincloud/server/internal/discovery"
)

/*
L'extraction, sur du HTML tel qu'on en rencontre.

La page d'essai est volontairement imparfaite : indentation dans les cellules,
adresses relatives de trois formes différentes, une ligne d'en-tête que le
sélecteur attrape au passage, un lien `javascript:`. Ce sont les défauts réels
des sites visés, et chacun a déjà cassé un scraper quelque part.

Aucun réseau ici : c'est ce qui permet de vérifier la partie fragile — celle qui
casse quand un site change de mise en page — en quelques millisecondes.
*/

const searchPage = `<!doctype html>
<html><body>
<ul class="results">

  <li class="issue header">
    <h3><span>Résultats</span></h3>
  </li>

  <li class="issue">
    <h3><a href="/issue/42">
        Fantastic
        Comics 12
    </a></h3>
    <span class="author">Fletcher Hanks</span>
    <span class="author">Basil Wolverton</span>
    <span class="series">Fantastic Comics</span>
    <span class="meta">Publié en 1940 par Fox Features</span>
    <img class="cover" src="../covers/42.jpg">
  </li>

  <li class="issue">
    <h3><a href="https://cdn.comicshelf.example/issue/43">Planet Comics 5</a></h3>
    <span class="meta">sans année</span>
    <img class="cover" src="javascript:void(0)">
    <a class="download" href="//mirror.comicshelf.example/dl/43.cbz">CBZ</a>
  </li>

</ul>
</body></html>`

func parsePage(t *testing.T, body string) *goquery.Document {
	t.Helper()

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		t.Fatalf("page illisible : %v", err)
	}
	return doc
}

func TestExtractResults(t *testing.T) {
	template := loadReference(t)
	doc := parsePage(t, searchPage)

	results := extractResults(doc, template, "https://comicshelf.example/search?q=x", 10)

	// La ligne d'en-tête n'a pas de titre exploitable : elle doit disparaître,
	// sans quoi la liste s'ouvrirait sur une entrée morte.
	if len(results) != 2 {
		t.Fatalf("résultats = %d, attendu 2 :\n%+v", len(results), results)
	}

	first := results[0]

	// Le titre est réparti sur trois lignes dans la source. Sans repliement des
	// blancs, il finirait tel quel dans le nom du fichier importé.
	if first.Title != "Fantastic Comics 12" {
		t.Errorf("titre = %q", first.Title)
	}
	if len(first.Authors) != 2 || first.Authors[1] != "Basil Wolverton" {
		t.Errorf("auteurs = %v, les deux sont attendus", first.Authors)
	}
	if first.Series != "Fantastic Comics" {
		t.Errorf("série = %q", first.Series)
	}
	// L'année est noyée dans une phrase : c'est le rôle de `regex`.
	if first.Published != "1940" {
		t.Errorf("année = %q, attendu 1940", first.Published)
	}
	// `../covers/42.jpg` est relatif au CHEMIN de la page, pas à la racine.
	if first.CoverURL != "https://comicshelf.example/covers/42.jpg" {
		t.Errorf("couverture = %q", first.CoverURL)
	}
	if first.PageURL != "https://comicshelf.example/issue/42" {
		t.Errorf("fiche = %q", first.PageURL)
	}

	second := results[1]

	// Une adresse déjà absolue traverse sans être recollée à la base.
	if second.PageURL != "https://cdn.comicshelf.example/issue/43" {
		t.Errorf("fiche absolue = %q", second.PageURL)
	}
	// `javascript:` n'est ni une couverture ni un téléchargement.
	if second.CoverURL != "" {
		t.Errorf("couverture = %q, un javascript: devrait être écarté", second.CoverURL)
	}
	// Une année qui ne correspond pas à l'expression rend le VIDE, pas la
	// phrase entière : un gabarit qui déclare une extraction et ne l'obtient
	// pas a trouvé autre chose que ce qu'il croyait.
	if second.Published != "" {
		t.Errorf("année = %q, attendu vide", second.Published)
	}
}

// Le sélecteur de ligne borne la portée des champs. Sans cet ancrage, les
// auteurs de la première ligne se retrouveraient sur la seconde, qui n'en a
// pas.
func TestExtractKeepsFieldsInsideTheirRow(t *testing.T) {
	template := loadReference(t)
	doc := parsePage(t, searchPage)

	results := extractResults(doc, template, "https://comicshelf.example/search", 10)

	if len(results[1].Authors) != 0 {
		t.Errorf("auteurs de la seconde ligne = %v, attendu aucun", results[1].Authors)
	}
}

func TestExtractRespectsLimit(t *testing.T) {
	template := loadReference(t)
	doc := parsePage(t, searchPage)

	if results := extractResults(doc, template, "https://comicshelf.example/", 1); len(results) != 1 {
		t.Errorf("résultats = %d, attendu 1", len(results))
	}
}

/*
La fiche complète sans écraser.

C'est la règle de `discovery.Enrich`, appliquée ici : la page de liste a été
écrite pour être lue en série, ses valeurs sont les plus régulières. La fiche
apporte ce qui manque, pas une seconde version de ce qu'on a déjà.
*/
func TestApplyDetailDoesNotOverwrite(t *testing.T) {
	template := loadReference(t)
	doc := parsePage(t, `<html><body>
	  <a class="download" href="/dl/42.cbz">Télécharger</a>
	  <div class="synopsis">Un résumé venu de la fiche.</div>
	  </body></html>`)

	result := extractResults(parsePage(t, searchPage), template,
		"https://comicshelf.example/search", 10)[0]
	result.Summary = "Un résumé venu de la liste."

	applyDetail(&result, doc, template, "https://comicshelf.example/issue/42")

	if result.Summary != "Un résumé venu de la liste." {
		t.Errorf("résumé = %q, celui de la liste devait être gardé", result.Summary)
	}
	if len(result.Acquisitions) != 1 {
		t.Fatalf("acquisitions = %v, attendu une", result.Acquisitions)
	}
	if result.Acquisitions[0].Href != "https://comicshelf.example/dl/42.cbz" {
		t.Errorf("lien = %q", result.Acquisitions[0].Href)
	}
	// Le type vient du gabarit : le HTML ne l'annonce pas.
	if result.Acquisitions[0].Type != "application/vnd.comicbook+zip" {
		t.Errorf("type = %q", result.Acquisitions[0].Type)
	}
}

// Un site sert souvent le même album en plusieurs formats. Les liens
// s'ajoutent, sans doublon, pour que l'utilisateur choisisse.
func TestApplyDetailAccumulatesAcquisitions(t *testing.T) {
	template := mustParse(t, `
id: multi
name: Multi
mirrors: [https://multi.example]
search: {path: "/q/{terms}"}
results:
  select: "li"
  fields:
    title: {select: "h3"}
detail:
  from: pageUrl
  fields:
    acquisition: {select: "a.download", from: attr, attr: href, all: true}`)

	doc := parsePage(t, `<html><body>
	  <a class="download" href="/dl/42.cbz">CBZ</a>
	  <a class="download" href="/dl/42.pdf">PDF</a>
	  <a class="download" href="/dl/42.cbz">CBZ (miroir)</a>
	  </body></html>`)

	result := discoveryResult()
	applyDetail(&result, doc, template, "https://multi.example/issue/42")

	if len(result.Acquisitions) != 2 {
		t.Fatalf("acquisitions = %d, attendu 2 (le doublon écarté) : %+v",
			len(result.Acquisitions), result.Acquisitions)
	}
}

// Un sélecteur peut désigner la ligne elle-même, et pas seulement ses
// descendants. `Find` seul ne le verrait pas.
func TestExtractMatchesTheRowItself(t *testing.T) {
	template := mustParse(t, `
id: plate
name: Plate
mirrors: [https://plate.example]
search: {path: "/q/{terms}"}
results:
  select: "a.issue"
  fields:
    title: {select: "a.issue"}
    pageUrl: {select: "a.issue", from: attr, attr: href}`)

	doc := parsePage(t, `<html><body>
	  <a class="issue" href="/i/1">Wonder Comics 1</a>
	  </body></html>`)

	results := extractResults(doc, template, "https://plate.example/search", 10)

	if len(results) != 1 || results[0].Title != "Wonder Comics 1" {
		t.Fatalf("résultats = %+v", results)
	}
	if results[0].PageURL != "https://plate.example/i/1" {
		t.Errorf("fiche = %q", results[0].PageURL)
	}
}

func discoveryResult() discovery.Result { return discovery.Result{Title: "Sans lien"} }
