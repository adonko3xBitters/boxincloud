package scraper

import "testing"

/*
robots.txt.

Deux règles du format sont contre-intuitives, et ce sont exactement celles qu'on
implémente de travers : ce n'est pas l'ORDRE d'écriture qui décide mais la
LONGUEUR du motif, et le groupe qui nous nomme REMPLACE le groupe générique au
lieu de s'y ajouter.

Se tromper sur la première fait ignorer un `Allow` d'exception, donc refuser des
pages ouvertes. Se tromper sur la seconde fait cumuler des règles que le site a
explicitement remplacées — c'est-à-dire désobéir en croyant obéir.
*/

func TestRobotsLongestPatternWins(t *testing.T) {
	rules := parseRobots(`
User-agent: *
Disallow: /
Allow: /public/
Disallow: /public/prive/
`)

	cases := map[string]bool{
		"/":                   false,
		"/interne/x":          false,
		"/public/album":       true,
		"/public/prive/album": false,
		"/publicité":          false, // préfixe partiel : /public/ ne couvre pas
	}

	for path, want := range cases {
		if got := rules.allows(path); got != want {
			t.Errorf("allows(%q) = %v, attendu %v", path, got, want)
		}
	}
}

// Le groupe qui nous nomme REMPLACE le générique. Les cumuler ferait obéir à
// des règles que le site a écrites pour les autres.
func TestRobotsNamedGroupReplacesWildcard(t *testing.T) {
	rules := parseRobots(`
User-agent: *
Disallow: /

User-agent: boxincloud
Disallow: /admin/
`)

	if !rules.allows("/catalogue/42") {
		t.Error("le groupe générique s'applique encore alors qu'un groupe nous nomme")
	}
	if rules.allows("/admin/config") {
		t.Error("/admin/ devrait rester interdit")
	}
}

// Plusieurs `User-agent` consécutifs partagent le bloc qui suit. Un groupe ne se
// referme que sur le `User-agent` qui suit une directive.
func TestRobotsSharedGroupHeaders(t *testing.T) {
	rules := parseRobots(`
User-agent: GPTBot
User-agent: boxincloud
Disallow: /prive/

User-agent: *
Disallow: /
`)

	if rules.allows("/prive/x") {
		t.Error("la règle du groupe partagé n'est pas appliquée")
	}
	if !rules.allows("/public/x") {
		t.Error("le groupe générique a été cumulé au nôtre")
	}
}

// `Disallow:` vide est la façon canonique de déclarer un groupe permissif. Le
// traiter comme un préfixe vide interdirait le site entier.
func TestRobotsEmptyDisallowAllowsEverything(t *testing.T) {
	rules := parseRobots("User-agent: *\nDisallow:\n")

	if !rules.allows("/n/importe/quoi") {
		t.Error("un Disallow vide devrait tout autoriser")
	}
}

func TestRobotsWildcardsAndAnchor(t *testing.T) {
	rules := parseRobots(`
User-agent: *
Disallow: /*.pdf$
Disallow: /tmp/*/brouillon
`)

	cases := map[string]bool{
		"/livres/x.pdf":       false,
		"/livres/x.pdf.cbz":   true, // l'ancre exige la fin du chemin
		"/tmp/2024/brouillon": false,
		"/tmp/brouillon":      true, // le joker exige un segment intermédiaire
	}

	for path, want := range cases {
		if got := rules.allows(path); got != want {
			t.Errorf("allows(%q) = %v, attendu %v", path, got, want)
		}
	}
}

// Un fichier vide, absent ou fait de commentaires n'interdit rien. C'est la
// lecture de la spécification, et l'inverse ferait d'une panne réseau un refus
// définitif de la source.
func TestRobotsSilenceAllows(t *testing.T) {
	for _, body := range []string{"", "# rien à déclarer\n", "Sitemap: /sitemap.xml\n"} {
		if !parseRobots(body).allows("/quoi/que/ce/soit") {
			t.Errorf("un robots.txt sans règle devrait autoriser (%q)", body)
		}
	}
}
