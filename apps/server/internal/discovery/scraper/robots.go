package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

/*
robots.txt.

# Pourquoi c'est ici et pas dans un « à faire plus tard »

Un site publie dans ce fichier la frontière qu'il accepte. La respecter n'est
pas une obligation légale ; c'est la différence pratique entre un client toléré
et un client bloqué par adresse — et un blocage par adresse prive TOUS les
utilisateurs de l'instance, pas seulement celui qui a lancé la recherche de
trop.

Les sites du domaine public dont il est question ici sont tenus par des
bénévoles sur des hébergements mutualisés. Ils bloquent vite, et durablement.

# Ce qui est implémenté, et ce qui ne l'est pas

Le nécessaire : les groupes `User-agent`, `Allow` et `Disallow`, les jokers `*`
et l'ancre `$`, la règle du chemin le plus long, et `Allow` qui l'emporte à
longueur égale.

Pas implémenté, et volontairement : `Crawl-delay`, `Sitemap`, `Host`. Le premier
est déjà couvert, en plus prudent, par le limiteur sortant du gabarit ; les deux
autres ne concernent pas un client qui va chercher une page précise.

Un fichier absent, illisible ou servi en erreur AUTORISE. C'est la lecture
retenue par la spécification, et l'inverse ferait d'une panne réseau passagère
un refus définitif de la source.
*/

// agentToken est le nom sous lequel boxincloud se cherche dans robots.txt.
//
// La casse est ignorée à la comparaison, et il n'y a pas de version. Un site
// qui veut bloquer boxincloud écrit `User-agent: boxincloud` et n'a pas à
// suivre nos numéros de version pour que sa règle continue de s'appliquer.
const agentToken = "boxincloud"

type robots struct {
	fetcher *fetcher
}

func newRobots(f *fetcher) *robots { return &robots{fetcher: f} }

/*
allows dit si le site autorise cette adresse.

Le second retour est l'erreur de LECTURE du fichier, pas un refus. L'appelant
la journalise et poursuit : voir le commentaire d'en-tête sur l'absence qui
autorise.
*/
func (r *robots) allows(ctx context.Context, template *Compiled, target string) (bool, error) {
	parsed, err := url.Parse(target)
	if err != nil {
		// Rendue plutôt qu'avalée, même si le verdict reste « autorisé » : une
		// adresse illisible n'est pas une adresse interdite, mais elle ne doit
		// pas non plus passer sans laisser de trace. L'appelant échouera de
		// toute façon en tentant de la joindre.
		return true, fmt.Errorf("adresse illisible (%s) : %w", target, err)
	}

	rules, err := r.rulesFor(ctx, template, parsed)
	if err != nil {
		return true, err
	}

	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}
	return rules.allows(path), nil
}

// rulesFor lit et mémorise le robots.txt d'un hôte.
//
// Mémorisé par ORIGINE, pas par gabarit : deux gabarits qui partageraient un
// hôte liraient sinon deux fois le même fichier, et un miroir ajouté à un
// gabarit relirait celui des autres.
func (r *robots) rulesFor(
	ctx context.Context, template *Compiled, target *url.URL,
) (*ruleset, error) {
	origin := target.Scheme + "://" + target.Host
	key := "robots\x00" + origin

	if r.fetcher.memo != nil {
		if raw, ok := r.fetcher.memo.Get(key); ok {
			if rules, ok := raw.(*ruleset); ok {
				return rules, nil
			}
		}
	}

	if r.fetcher.throttle != nil {
		if err := r.fetcher.throttle.Wait(ctx, bucketFor(target.Host)); err != nil {
			return permissive, err
		}
	}

	// Appel direct à `do` : passer par `fetch` rappellerait ce contrôle pour
	// aller chercher le fichier qui le décrit, indéfiniment.
	body, _, err := r.fetcher.do(ctx, template, origin+"/robots.txt",
		request{method: http.MethodGet})
	if err != nil {
		// L'échec est mémorisé comme une autorisation, pour ne pas redemander
		// le fichier à chaque page d'une même recherche. Le cache expire, donc
		// un site qui republie son robots.txt sera relu.
		if r.fetcher.memo != nil {
			r.fetcher.memo.Put(key, permissive)
		}
		return permissive, err
	}

	rules := parseRobots(string(body))
	if r.fetcher.memo != nil {
		r.fetcher.memo.Put(key, rules)
	}
	return rules, nil
}

// ruleset est la politique d'un hôte, réduite à ce qui nous concerne.
type ruleset struct{ rules []rule }

type rule struct {
	pattern string
	allow   bool
}

// permissive est la politique d'un site qui n'en publie pas.
var permissive = &ruleset{}

/*
parseRobots lit un fichier robots.txt.

Deux subtilités du format valent d'être notées, parce qu'elles se trompent
facilement.

La première : les groupes se referment sur la première ligne `User-agent` qui
SUIT une directive. Plusieurs `User-agent` consécutifs partagent le même bloc de
règles, et beaucoup de sites s'en servent.

La seconde : le groupe le plus SPÉCIFIQUE l'emporte, et lui seul. Un site qui
déclare `User-agent: *` puis `User-agent: boxincloud` n'applique à boxincloud
que le second bloc — pas l'union des deux. Les cumuler ferait obéir à des règles
que le site a explicitement remplacées.
*/
func parseRobots(body string) *ruleset {
	var (
		starred, named []rule
		agents         []string
		collecting     bool
	)

	appendRule := func(pattern string, allow bool) {
		for _, agent := range agents {
			switch agent {
			case agentToken:
				named = append(named, rule{pattern: pattern, allow: allow})
			case "*":
				starred = append(starred, rule{pattern: pattern, allow: allow})
			}
		}
	}

	for _, line := range strings.Split(body, "\n") {
		if at := strings.IndexByte(line, '#'); at >= 0 {
			line = line[:at]
		}
		field, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		field = strings.ToLower(strings.TrimSpace(field))
		value = strings.TrimSpace(value)

		switch field {
		case "user-agent":
			if collecting {
				// Une directive a été vue depuis le dernier en-tête : ce
				// `User-agent` ouvre un nouveau groupe.
				agents = nil
				collecting = false
			}
			agents = append(agents, strings.ToLower(value))
		case "allow":
			collecting = true
			if value != "" {
				appendRule(value, true)
			}
		case "disallow":
			collecting = true
			// `Disallow:` vide autorise tout — c'est la façon canonique de
			// déclarer un groupe permissif, et la traiter comme un préfixe vide
			// interdirait le site entier.
			if value != "" {
				appendRule(value, false)
			}
		}
	}

	if len(named) > 0 {
		return &ruleset{rules: named}
	}
	return &ruleset{rules: starred}
}

/*
allows applique la règle du chemin le plus long.

C'est la règle du format, et elle n'est pas intuitive : ce n'est ni l'ordre
d'écriture ni la première correspondance qui décide, mais la LONGUEUR du motif.
Un `Disallow: /` suivi d'un `Allow: /public/` autorise donc `/public/x`, ce qui
est précisément l'usage qu'en font les sites qui n'ouvrent qu'une partie de
leur arborescence.

À longueur égale, `Allow` l'emporte — la spécification tranche ainsi, et c'est
la lecture la moins hostile.
*/
func (r *ruleset) allows(path string) bool {
	best, allowed := -1, true

	for _, candidate := range r.rules {
		if !matchPath(candidate.pattern, path) {
			continue
		}
		length := len(candidate.pattern)
		if length > best || (length == best && candidate.allow) {
			best, allowed = length, candidate.allow
		}
	}
	return allowed
}

/*
matchPath compare un chemin à un motif robots.txt.

Le motif est un préfixe, avec deux extensions devenues universelles : `*`
remplace n'importe quelle suite de caractères, `$` en fin de motif ancre la fin
du chemin.

Écrit à la main plutôt que traduit en expression rationnelle : la traduction
demanderait d'échapper le motif — qui contient déjà des caractères spéciaux tirés
d'URL — et une erreur d'échappement donnerait une règle plus large ou plus
étroite que celle que le site a écrite.
*/
func matchPath(pattern, path string) bool {
	anchored := strings.HasSuffix(pattern, "$")
	if anchored {
		pattern = strings.TrimSuffix(pattern, "$")
	}

	segments := strings.Split(pattern, "*")
	at := 0

	for i, segment := range segments {
		if segment == "" {
			continue
		}
		if i == 0 {
			if !strings.HasPrefix(path[at:], segment) {
				return false
			}
			at += len(segment)
			continue
		}

		found := strings.Index(path[at:], segment)
		if found < 0 {
			return false
		}
		at += found + len(segment)
	}

	if anchored {
		// Le dernier segment doit terminer le chemin. Un motif finissant par
		// `*$` accepte n'importe quelle fin, ce qui revient à ne pas ancrer.
		if last := segments[len(segments)-1]; last != "" {
			return strings.HasSuffix(path, last)
		}
	}
	return true
}
