package scraper

import (
	"os"
	"strings"
	"testing"
)

/*
Le gabarit d'exemple, contre de vraies pages.

`deploy/scraper-templates/standardebooks.yaml` est ce qu'on montre à quelqu'un
qui veut écrire son premier gabarit. Un exemple faux est pire qu'une absence
d'exemple : il apprend une syntaxe qui ne marche pas, et celui qui le recopie
cherchera son erreur là où elle n'est pas.

Les deux pages de `testdata/` ont été ENREGISTRÉES depuis le site réel. C'est ce
qui rend ce test possible sans réseau — et c'est aussi sa limite, qu'il vaut
mieux nommer : il vérifie que les sélecteurs lisent CES pages, pas que le site
n'a pas changé depuis. Le jour où il change, c'est `Probe` qui le dira, à
l'endroit prévu pour ça.
*/

func loadExample(t *testing.T) *Compiled {
	t.Helper()

	raw, err := os.ReadFile("../../../../../deploy/scraper-templates/standardebooks.yaml")
	if err != nil {
		t.Fatalf("gabarit d'exemple illisible : %v", err)
	}
	compiled, err := Parse(raw)
	if err != nil {
		t.Fatalf("le gabarit d'exemple est refusé par la validation : %v", err)
	}
	return compiled
}

func TestExampleTemplateReadsRealPages(t *testing.T) {
	template := loadExample(t)

	page, err := os.ReadFile("testdata/standardebooks-search.html")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseHTML(page)
	if err != nil {
		t.Fatal(err)
	}

	results := extractResults(doc, template,
		"https://standardebooks.org/ebooks?query=verne", 30)

	if len(results) < 5 {
		t.Fatalf("résultats = %d, la page en contient une douzaine", len(results))
	}

	first := results[0]

	if first.Title != "The Giant Raft" {
		t.Errorf("titre = %q", first.Title)
	}

	/*
		Le piège du site, et la raison d'être du `:not([class])`.

		Le titre et l'auteur portent tous deux `property="schema:name"`. Un
		sélecteur qui ne distinguerait pas leur paragraphe mettrait « Jules
		Verne » dans le titre une fois sur deux — une erreur silencieuse, qui ne
		se verrait qu'en lisant la liste.
	*/
	if len(first.Authors) == 0 || first.Authors[0] != "Jules Verne" {
		t.Errorf("auteurs = %v", first.Authors)
	}

	// Adresses relatives dans la page : non résolues, elles donneraient des
	// liens morts dans l'interface.
	if !strings.HasPrefix(first.PageURL, "https://standardebooks.org/ebooks/") {
		t.Errorf("fiche = %q", first.PageURL)
	}
	if !strings.HasPrefix(first.CoverURL, "https://standardebooks.org/images/") {
		t.Errorf("couverture = %q", first.CoverURL)
	}

	// Aucune ligne ne porte de lien de téléchargement : c'est ce qui justifie
	// l'étape `detail` du gabarit.
	if len(first.Acquisitions) != 0 {
		t.Errorf("la liste ne devrait porter aucun lien : %+v", first.Acquisitions)
	}
}

func TestExampleTemplateReadsTheDetailPage(t *testing.T) {
	template := loadExample(t)

	page, err := os.ReadFile("testdata/standardebooks-detail.html")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseHTML(page)
	if err != nil {
		t.Fatal(err)
	}

	result := discoveryResult()
	applyDetail(&result, doc, template,
		"https://standardebooks.org/ebooks/jules-verne/the-giant-raft/w-j-gordon")

	/*
		Un seul lien, et c'est le point.

		La fiche propose l'EPUB, l'AZW3, le KEPUB et une variante « advanced ».
		Les prendre tous donnerait quatre lignes pour le même livre, dont trois
		que personne n'a demandées. Le sélecteur vise `a.epub`, et ce test
		échouera si quelqu'un l'élargit sans y penser.
	*/
	if len(result.Acquisitions) != 1 {
		t.Fatalf("acquisitions = %d, attendu 1 : %+v",
			len(result.Acquisitions), result.Acquisitions)
	}

	link := result.Acquisitions[0]
	if !strings.HasSuffix(link.Href, ".epub") ||
		!strings.HasPrefix(link.Href, "https://standardebooks.org/ebooks/") {
		t.Errorf("lien = %q", link.Href)
	}
	// Le type vient du gabarit : le HTML ne l'annonce pas.
	if link.Type != "application/epub+zip" {
		t.Errorf("type = %q", link.Type)
	}

	if result.Summary == "" {
		t.Error("résumé non extrait de la fiche")
	}
}

/*
Le robots.txt du site, tel qu'il est publié.

Il mérite un test parce qu'il contient exactement le piège que le format tend :
un second groupe, nommant six robots de référencement, qui ferme les
téléchargements. Un lecteur pressé — humain ou code — conclurait que les
téléchargements nous sont fermés. Ils ne le sont pas : ce groupe ne nous nomme
pas, et un groupe nommé remplace le générique au lieu de s'y ajouter.

Se tromper ici ferait refuser tous les imports, sans message qui le dise.
*/
func TestExampleSiteRobots(t *testing.T) {
	rules := parseRobots(`Sitemap: https://standardebooks.org/sitemap

# Badly-behaved bots
User-agent: *
Disallow: /honeypot

# SEO crawlers
User-agent: SemrushBot
User-agent: DotBot
User-agent: AhrefsBot

Disallow: /ebooks/*/downloads/*
`)

	if !rules.allows("/ebooks?query=verne") {
		t.Error("la recherche devrait être autorisée")
	}
	if !rules.allows("/ebooks/jules-verne/the-giant-raft/w-j-gordon/downloads/x.epub") {
		t.Error("le téléchargement devrait être autorisé : la règle vise un autre groupe")
	}
	if rules.allows("/honeypot") {
		t.Error("/honeypot est la seule adresse que le groupe générique ferme")
	}
}
