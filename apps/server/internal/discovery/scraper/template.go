package scraper

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/andybalholm/cascadia"
	"gopkg.in/yaml.v3"
)

/*
Gabarit de lecture d'un site.

# Pourquoi du déclaratif plutôt que du Go

Un site change de gabarit HTML deux ou trois fois par an, sans prévenir. Si
chaque site est un fichier Go, chacun de ces changements est un correctif, une
revue, une version, une image Docker et une mise à jour chez tous les
utilisateurs — pour deux caractères dans un sélecteur CSS.

En déclaratif, c'est un fichier de données. Il peut être corrigé par quelqu'un
qui ne connaît pas Go, testé contre une page enregistrée, et — pour un opérateur
pressé — remplacé sur son instance sans attendre la prochaine version.

Le prix à payer est réel et il faut le nommer : le gabarit n'est plus vérifié
par le compilateur. C'est pourquoi `Validate` est aussi sévère qu'elle l'est, et
pourquoi elle tourne au CHARGEMENT et non à la recherche. Un gabarit fautif doit
empêcher l'instance de le proposer, pas rendre une liste vide en silence.

# Pourquoi CSS

Trois candidats se présentaient : les expressions rationnelles, XPath, les
sélecteurs CSS.

Les expressions rationnelles sur du HTML sont écartées sans regret — elles
cassent au premier attribut ajouté, et donnent des gabarits illisibles.

XPath est plus puissant : il sait remonter au parent, ce que CSS ne sait pas. Il
demande une dépendance de plus, et cette puissance sert surtout à écrire des
chemins positionnels — `div[3]/table/tr[2]` — qui sont exactement ceux qui
cassent au premier remaniement.

CSS l'emporte parce que c'est la langue dans laquelle les sélecteurs se lisent
déjà : quelqu'un qui veut corriger un gabarit ouvre les outils de développement
de son navigateur, copie le sélecteur qu'ils lui donnent, et c'est du CSS. La
contrainte de ne pas savoir remonter pousse d'ailleurs à ancrer les gabarits sur
la LIGNE de résultat, ce qui les rend plus robustes.
*/

// Duration se lit « 2s », « 500ms », « 1m » dans le YAML.
//
// Un type à part parce que `time.Duration` se décode en nanosecondes entières
// depuis du YAML, ce qui donnerait des gabarits écrits `every: 2000000000`.
type Duration time.Duration

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var raw string
	if err := node.Decode(&raw); err != nil {
		return fmt.Errorf("durée illisible : %w", err)
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("durée illisible (%q) : %w", raw, err)
	}
	*d = Duration(parsed)
	return nil
}

func (d Duration) Std() time.Duration { return time.Duration(d) }

// Template est un gabarit tel qu'il est écrit dans un fichier YAML.
type Template struct {
	// ID identifie le gabarit de façon stable. Il devient le genre de la source
	// (`scraper:digitalcomicmuseum`), la clé du catalogue et le préfixe de
	// cache : le changer revient à créer un autre site.
	ID string `yaml:"id"`
	// Name est le nom affiché quand l'administrateur choisit un gabarit.
	Name string `yaml:"name"`
	// Homepage sert uniquement à l'administration, pour que celui qui active un
	// gabarit puisse aller voir de quel site il s'agit.
	Homepage string `yaml:"homepage"`
	// License documente à quel titre le site est admissible. Purement
	// informatif pour le moteur, décisif pour la revue d'un gabarit proposé.
	License string `yaml:"license"`

	/*
		Mirrors sont les bases essayées, dans l'ordre.

		La première est le défaut. L'URL enregistrée pour la source l'emporte sur
		toutes : c'est ce qui permet de suivre un changement de domaine sans
		recompiler.
	*/
	Mirrors []string `yaml:"mirrors"`

	/*
		IgnoreRobots désactive la lecture de robots.txt pour CE site.

		Faux par défaut, et il faut que ça le reste : ce fichier est la frontière
		que le site a publiée, et la respecter est ce qui distingue un client
		toléré d'un client bloqué par adresse — au détriment de tous les
		utilisateurs de l'instance, pas seulement de celui qui a insisté.

		L'option existe parce que l'administrateur a souvent autorité sur la
		cible : l'intranet qu'il opère, un site partenaire, ou un `Disallow:
		/search` posé contre les moissonneurs et non contre une requête qu'un
		humain vient de taper.

		Ce qu'elle ne change PAS : l'agent reste `boxincloud`, avec l'adresse du
		dépôt. Passer outre un avis consultatif en restant identifiable n'est pas
		se faire passer pour quelqu'un d'autre — le site peut toujours refuser,
		et son refus sera alors sans ambiguïté.

		Elle est journalisée à chaque activation. Une dérogation silencieuse
		serait pire que pas de dérogation du tout.
	*/
	IgnoreRobots bool `yaml:"ignoreRobots"`

	Rate    RateSpec    `yaml:"rate"`
	Limits  LimitsSpec  `yaml:"limits"`
	Search  SearchSpec  `yaml:"search"`
	Results ResultsSpec `yaml:"results"`
	// Detail est facultatif : beaucoup de sites ne publient le lien de
	// téléchargement que sur la fiche, pas dans la liste.
	Detail *DetailSpec `yaml:"detail"`
}

// RateSpec est le débit sortant vers CHAQUE hôte du gabarit.
type RateSpec struct {
	// Every est l'intervalle minimal entre deux requêtes vers un même hôte.
	Every Duration `yaml:"every"`
	// Burst est la rafale tolérée avant que l'intervalle ne s'applique.
	Burst int `yaml:"burst"`
}

/*
LimitsSpec borne ce qu'une recherche a le droit de coûter.

Toutes les bornes ont un défaut, et aucune n'est facultative dans les faits : un
gabarit qui n'en déclare aucune hérite de valeurs prudentes plutôt que de
l'infini.
*/
type LimitsSpec struct {
	// Timeout borne UNE requête.
	Timeout Duration `yaml:"timeout"`
	/*
		Budget borne la recherche ENTIÈRE, suivi des fiches compris.

		Distinct du délai par requête, et c'est le point : dix fiches suivies à
		huit secondes font quatre-vingts secondes sans qu'aucun délai unitaire
		ne soit dépassé. C'est cette borne-là qui protège la page.
	*/
	Budget Duration `yaml:"budget"`
	// MaxBytes borne une page lue. Contre l'hôte qui répondrait sans fin, pas
	// contre les grosses pages.
	MaxBytes int64 `yaml:"maxBytes"`
	// MaxResults borne les lignes retenues sur une page de résultats.
	MaxResults int `yaml:"maxResults"`
	// FollowDetail borne le nombre de fiches suivies par recherche.
	FollowDetail int `yaml:"followDetail"`
}

/*
SearchSpec décrit la requête de recherche.

`{terms}` est remplacé par les mots cherchés, échappés pour le contexte —
composant d'URL dans le chemin, valeur de requête ailleurs. `{limit}` est
remplacé par le nombre de résultats demandés, pour les sites qui l'acceptent.
*/
type SearchSpec struct {
	// Method vaut GET ou POST. Défaut GET.
	//
	// POST existe parce que plusieurs moteurs de recherche de sites anciens
	// n'acceptent que lui — le formulaire est en POST, et l'équivalent en GET
	// rend la page d'accueil.
	Method string `yaml:"method"`
	// Path est le chemin, relatif à la base. Peut contenir `{terms}`.
	Path string `yaml:"path"`
	// Query sont les paramètres de la chaîne de requête.
	Query map[string]string `yaml:"query"`
	// Form est le corps `application/x-www-form-urlencoded`, pour un POST.
	Form map[string]string `yaml:"form"`
	/*
		Probe est le terme d'essai, celui que `Probe` envoie pour vérifier que le
		gabarit lit encore le site.

		Il doit rendre des résultats sur CE site, toujours. Un terme trop rare
		ferait échouer l'essai d'un gabarit parfaitement valide, et
		l'administrateur conclurait que la source est cassée alors qu'elle
		fonctionne.
	*/
	Probe string `yaml:"probe"`
	// Headers ajoute des en-têtes. Le User-Agent n'y est pas modifiable : voir
	// fetch.go, où le choix est expliqué.
	Headers map[string]string `yaml:"headers"`
}

// ResultsSpec dit où sont les lignes de résultat et ce qu'elles portent.
type ResultsSpec struct {
	/*
		Select désigne la LIGNE, pas la page.

		C'est la décision qui rend un gabarit robuste. Extraire chaque champ par
		un sélecteur global donnerait trois listes parallèles — titres, liens,
		couvertures — qu'il faudrait ensuite apparier par position, et un
		résultat sans couverture décalerait tout le reste d'un cran.

		En ancrant sur la ligne, chaque champ est cherché DANS sa ligne, et un
		champ absent est simplement vide.
	*/
	Select string               `yaml:"select"`
	Fields map[string]FieldSpec `yaml:"fields"`
}

/*
DetailSpec décrit une seconde requête, vers la fiche d'un résultat.

Nécessaire parce que beaucoup de sites ne mettent le lien de téléchargement que
sur la fiche. Coûteux, aussi : c'est une requête PAR résultat, et c'est
exactement le comportement qui fait bloquer un client.

Deux garde-fous, tous deux dans le gabarit plutôt que dans le code : le nombre
de fiches suivies est borné par `limits.followDetail`, et `onlyIfMissing`
permet de ne suivre que les lignes auxquelles il manque quelque chose. Un site
qui publie déjà le lien dans sa liste ne coûte alors aucune requête de plus.
*/
type DetailSpec struct {
	// From est le champ de la ligne qui porte l'adresse de la fiche. Défaut
	// `pageUrl`.
	From string `yaml:"from"`
	// OnlyIfMissing ne suit la fiche que si l'un de ces champs est vide.
	OnlyIfMissing []string             `yaml:"onlyIfMissing"`
	Fields        map[string]FieldSpec `yaml:"fields"`
}

/*
FieldSpec dit comment tirer une valeur d'un fragment de document.

Volontairement pauvre. Chaque possibilité ajoutée ici est une possibilité de se
tromper dans un gabarit, et un gabarit ne se teste pas au compilateur. Les cinq
opérations retenues couvrent ce qu'on rencontre : choisir un nœud, en prendre le
texte ou un attribut, en découper une liste, et en extraire un morceau.
*/
type FieldSpec struct {
	// Select est relatif à la ligne. Vide : le champ n'est pas extrait.
	Select string `yaml:"select"`
	// From vaut `text`, `attr` ou `html`. Défaut `text`.
	From string `yaml:"from"`
	// Attr est le nom de l'attribut quand From vaut `attr`.
	Attr string `yaml:"attr"`
	/*
		All retient TOUS les nœuds correspondants plutôt que le premier.

		Sert aux auteurs, que les sites listent en autant de liens séparés. Sans
		lui, un album à quatre dessinateurs n'en montrerait qu'un.
	*/
	All bool `yaml:"all"`
	// Split découpe la valeur obtenue, pour les sites qui écrivent tous les
	// auteurs dans un seul nœud séparés par des virgules.
	Split string `yaml:"split"`
	/*
		Regex extrait un morceau de la valeur.

		Le premier groupe capturant l'emporte s'il y en a un, sinon la
		correspondance entière. Sert surtout aux champs noyés dans une phrase :
		« Publié en 1948 par Fox » dont on ne veut que l'année.

		Une valeur qui ne correspond pas devient VIDE, pas inchangée. C'est
		délibéré : un gabarit qui déclare une extraction et ne l'obtient pas a
		trouvé autre chose que ce qu'il croyait.
	*/
	Regex string `yaml:"regex"`
	// MediaType n'est lu que pour le champ `acquisition` : il renseigne le type
	// du lien quand le site ne l'annonce pas, ce qui est la règle en HTML.
	MediaType string `yaml:"mediaType"`
}

// Champs reconnus dans `results.fields` et `detail.fields`.
//
// Une liste fermée, et vérifiée : une faute de frappe dans un nom de champ
// donnerait sinon un gabarit qui se charge, tourne, et ne remplit jamais la
// colonne qu'on croyait avoir décrite.
const (
	fieldTitle       = "title"
	fieldAuthors     = "authors"
	fieldSeries      = "series"
	fieldSummary     = "summary"
	fieldLanguage    = "language"
	fieldPublished   = "published"
	fieldCoverURL    = "coverUrl"
	fieldPageURL     = "pageUrl"
	fieldAcquisition = "acquisition"
)

var knownFields = map[string]bool{
	fieldTitle: true, fieldAuthors: true, fieldSeries: true,
	fieldSummary: true, fieldLanguage: true, fieldPublished: true,
	fieldCoverURL: true, fieldPageURL: true, fieldAcquisition: true,
}

// Valeurs par défaut des bornes.
//
// Calées sur le même esprit que celles du client OPDS : assez courtes pour
// qu'un site en panne sorte de la recherche fédérée sans la retenir, assez
// larges pour un mutualisé qui répond en trois secondes un jour de charge.
const (
	defaultTimeout      = 8 * time.Second
	defaultBudget       = 25 * time.Second
	defaultMaxBytes     = 4 << 20
	defaultMaxResults   = 40
	defaultFollowDetail = 8
	defaultRateEvery    = 2 * time.Second
	defaultRateBurst    = 2
)

// ─── Compilation ─────────────────────────────────────────────────────────────

/*
Compiled est un gabarit prêt à l'emploi.

Les sélecteurs et les expressions rationnelles y sont compilés UNE FOIS, au
chargement. Les recompiler à chaque recherche coûterait le plus clair du temps
de traitement d'une page, et surtout repousserait la découverte d'un sélecteur
fautif au moment où quelqu'un cherche.
*/
type Compiled struct {
	Template

	row    cascadia.Selector
	fields map[string]compiledField
	detail map[string]compiledField

	// hosts est l'ensemble des hôtes que ce gabarit autorise à joindre. Sert au
	// contrôle d'origine de l'import : un lien de téléchargement servi par un
	// miroir doit rester acceptable.
	hosts map[string]bool
}

type compiledField struct {
	FieldSpec
	sel   cascadia.Selector
	regex *regexp.Regexp
}

func (c *Compiled) Timeout() time.Duration { return c.Limits.Timeout.Std() }
func (c *Compiled) Budget() time.Duration  { return c.Limits.Budget.Std() }

// Hosts rend les hôtes joignables du gabarit, dans un ordre stable.
func (c *Compiled) Hosts() []string {
	out := make([]string, 0, len(c.hosts))
	for host := range c.hosts {
		out = append(out, host)
	}
	sort.Strings(out)
	return out
}

// AllowsHost dit si un hôte appartient au gabarit.
func (c *Compiled) AllowsHost(host string) bool {
	return c.hosts[strings.ToLower(strings.TrimSpace(host))]
}

/*
Parse lit un gabarit YAML, le valide et le compile.

Les trois étapes sont volontairement soudées : un `Template` qui n'aurait pas
traversé la validation n'a aucun usage légitime, et l'exposer inviterait à s'en
servir.

`KnownFields` est activé sur le décodeur. Une clé inconnue est donc une erreur
et non un silence — c'est ce qui transforme `mediatype:` mal orthographié en
message au chargement plutôt qu'en champ vide six mois plus tard.
*/
func Parse(raw []byte) (*Compiled, error) {
	var template Template

	decoder := yaml.NewDecoder(strings.NewReader(string(raw)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&template); err != nil {
		return nil, fmt.Errorf("%w : %w", ErrInvalidTemplate, err)
	}

	template.applyDefaults()
	if err := template.validate(); err != nil {
		return nil, err
	}
	return template.compile()
}

// applyDefaults comble les bornes absentes.
//
// Avant la validation, pas après : la validation doit voir le gabarit tel qu'il
// sera utilisé, sinon elle laisserait passer un budget plus court qu'un délai
// unitaire dès lors que l'un des deux est implicite.
func (t *Template) applyDefaults() {
	if t.Search.Method == "" {
		t.Search.Method = "GET"
	}
	t.Search.Method = strings.ToUpper(t.Search.Method)

	if t.Rate.Every <= 0 {
		t.Rate.Every = Duration(defaultRateEvery)
	}
	if t.Rate.Burst <= 0 {
		t.Rate.Burst = defaultRateBurst
	}
	if t.Limits.Timeout <= 0 {
		t.Limits.Timeout = Duration(defaultTimeout)
	}
	if t.Limits.Budget <= 0 {
		t.Limits.Budget = Duration(defaultBudget)
	}
	if t.Limits.MaxBytes <= 0 {
		t.Limits.MaxBytes = defaultMaxBytes
	}
	if t.Limits.MaxResults <= 0 {
		t.Limits.MaxResults = defaultMaxResults
	}
	if t.Limits.FollowDetail <= 0 {
		t.Limits.FollowDetail = defaultFollowDetail
	}
	if strings.TrimSpace(t.Search.Probe) == "" {
		// « comic » rend quelque chose sur tout site de bande dessinée. Un
		// défaut discutable vaut mieux qu'un essai qui ne vérifie rien.
		t.Search.Probe = "comic"
	}
	if t.Detail != nil && t.Detail.From == "" {
		t.Detail.From = fieldPageURL
	}
}

var templateID = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`)

/*
validate refuse un gabarit inutilisable.

Les erreurs sont ACCUMULÉES plutôt que rendues à la première. Un gabarit se
corrige dans un éditeur de texte, souvent par quelqu'un qui n'a pas l'instance
sous la main : lui rendre les six problèmes d'un coup lui épargne six
allers-retours.
*/
func (t *Template) validate() error {
	var problems []string

	if !templateID.MatchString(t.ID) {
		problems = append(problems,
			"id : minuscules, chiffres et tirets, 1 à 40 caractères")
	}
	if strings.TrimSpace(t.Name) == "" {
		problems = append(problems, "name : requis")
	}

	if len(t.Mirrors) == 0 {
		problems = append(problems, "mirrors : au moins une base est requise")
	}
	for _, mirror := range t.Mirrors {
		if host := hostOf(mirror); host == "" {
			problems = append(problems,
				fmt.Sprintf("mirrors : %q n'est pas une adresse http(s) absolue", mirror))
		}
	}

	switch t.Search.Method {
	case "GET":
		if len(t.Search.Form) > 0 {
			problems = append(problems, "search.form : réservé à la méthode POST")
		}
	case "POST":
	default:
		problems = append(problems, "search.method : GET ou POST")
	}

	// Sans `{terms}` quelque part, la recherche rendrait la même page pour
	// n'importe quoi — panne silencieuse s'il en est.
	if !t.Search.mentionsTerms() {
		problems = append(problems,
			"search : {terms} doit apparaître dans path, query ou form")
	}

	if strings.TrimSpace(t.Results.Select) == "" {
		problems = append(problems, "results.select : requis")
	}
	if _, ok := t.Results.Fields[fieldTitle]; !ok {
		// Le service écarte déjà les résultats sans titre. Un gabarit qui n'en
		// extrait pas ne rendrait donc jamais rien, et l'apprendre au
		// chargement vaut mieux que de le déduire d'une liste vide.
		problems = append(problems, "results.fields.title : requis")
	}

	problems = append(problems, validateFields("results.fields", t.Results.Fields)...)

	if t.Detail != nil {
		if !knownFields[t.Detail.From] {
			problems = append(problems,
				fmt.Sprintf("detail.from : champ inconnu %q", t.Detail.From))
		}
		if len(t.Detail.Fields) == 0 {
			problems = append(problems, "detail.fields : requis dès que detail existe")
		}
		for _, name := range t.Detail.OnlyIfMissing {
			if !knownFields[name] {
				problems = append(problems,
					fmt.Sprintf("detail.onlyIfMissing : champ inconnu %q", name))
			}
		}
		problems = append(problems, validateFields("detail.fields", t.Detail.Fields)...)
	}

	if t.Limits.Budget < t.Limits.Timeout {
		problems = append(problems,
			"limits.budget : ne peut pas être plus court que limits.timeout")
	}

	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("%w (%s) :\n  - %s",
		ErrInvalidTemplate, t.ID, strings.Join(problems, "\n  - "))
}

func validateFields(where string, fields map[string]FieldSpec) []string {
	var problems []string

	// L'ordre d'une map n'est pas stable, et un message d'erreur qui change
	// d'ordre à chaque exécution est insupportable à lire en test comme en
	// production.
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		field := fields[name]
		prefix := where + "." + name

		if !knownFields[name] {
			problems = append(problems, prefix+" : champ inconnu")
			continue
		}
		if strings.TrimSpace(field.Select) == "" {
			problems = append(problems, prefix+".select : requis")
		} else if _, err := cascadia.Compile(field.Select); err != nil {
			problems = append(problems,
				fmt.Sprintf("%s.select : sélecteur invalide (%v)", prefix, err))
		}

		switch field.From {
		case "", "text", "html":
			if field.Attr != "" {
				problems = append(problems, prefix+".attr : réservé à from: attr")
			}
		case "attr":
			if strings.TrimSpace(field.Attr) == "" {
				problems = append(problems, prefix+".attr : requis avec from: attr")
			}
		default:
			problems = append(problems, prefix+".from : text, attr ou html")
		}

		if field.Regex != "" {
			if _, err := regexp.Compile(field.Regex); err != nil {
				problems = append(problems,
					fmt.Sprintf("%s.regex : expression invalide (%v)", prefix, err))
			}
		}
		if field.MediaType != "" && name != fieldAcquisition {
			problems = append(problems, prefix+".mediaType : réservé au champ acquisition")
		}
	}
	return problems
}

func (s SearchSpec) mentionsTerms() bool {
	if strings.Contains(s.Path, "{terms}") {
		return true
	}
	for _, value := range s.Query {
		if strings.Contains(value, "{terms}") {
			return true
		}
	}
	for _, value := range s.Form {
		if strings.Contains(value, "{terms}") {
			return true
		}
	}
	return false
}

func (t *Template) compile() (*Compiled, error) {
	row, err := cascadia.Compile(t.Results.Select)
	if err != nil {
		return nil, fmt.Errorf("%w (%s) : results.select : %w", ErrInvalidTemplate, t.ID, err)
	}

	compiled := &Compiled{
		Template: *t,
		row:      row,
		fields:   map[string]compiledField{},
		detail:   map[string]compiledField{},
		hosts:    map[string]bool{},
	}

	for name, field := range t.Results.Fields {
		if compiled.fields[name], err = compileField(field); err != nil {
			return nil, fmt.Errorf("%w (%s) : results.fields.%s : %w",
				ErrInvalidTemplate, t.ID, name, err)
		}
	}
	if t.Detail != nil {
		for name, field := range t.Detail.Fields {
			if compiled.detail[name], err = compileField(field); err != nil {
				return nil, fmt.Errorf("%w (%s) : detail.fields.%s : %w",
					ErrInvalidTemplate, t.ID, name, err)
			}
		}
	}

	for _, mirror := range t.Mirrors {
		compiled.hosts[hostOf(mirror)] = true
	}
	return compiled, nil
}

func compileField(field FieldSpec) (compiledField, error) {
	out := compiledField{FieldSpec: field}

	var err error
	if out.sel, err = cascadia.Compile(field.Select); err != nil {
		return compiledField{}, err
	}
	if field.Regex != "" {
		if out.regex, err = regexp.Compile(field.Regex); err != nil {
			return compiledField{}, err
		}
	}
	return out, nil
}

/*
hostOf rend l'hôte d'une base, en minuscules, ou le vide si l'adresse est
inutilisable comme base.

Le port fait partie de l'hôte : un site en 8080 et le même en 443 sont deux
services, et les confondre élargirait sans le dire ce qu'un import a le droit de
joindre.
*/
func hostOf(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return strings.ToLower(parsed.Host)
}
