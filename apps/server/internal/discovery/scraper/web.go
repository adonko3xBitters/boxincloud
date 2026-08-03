package scraper

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

/*
Sources décrites depuis l'interface.

# Le problème que ce fichier résout

Un gabarit sur disque suppose un accès au serveur. C'est raisonnable pour les
sites que le projet livre — ils sont revus comme du code — et absurde pour
quelqu'un qui administre son instance depuis un navigateur : la fonctionnalité
lui est simplement hors de portée.

`WebSpec` est la même chose, saisie dans un formulaire. Elle est délibérément
plus PAUVRE que `Template` : une URL de recherche, la ligne, quatre champs. Pas
d'expressions rationnelles, pas de miroirs, pas de suivi de fiche, pas de réglage
de débit.

Ce n'est pas une étape vers l'exposition du format complet. Un formulaire à
trente champs ne serait rempli correctement par personne, et chaque possibilité
offerte est une possibilité de se tromper sans que rien ne le signale. Qui a
besoin de plus écrit un fichier — la porte reste ouverte, et elle est
documentée.

# Un seul moteur

`Compile` traduit la saisie en `Template`, exactement celui que produit un
fichier YAML. Tout ce qui suit — extraction, résolution des adresses, robots.txt,
limitation de débit, bornes de temps — est le même code pour les deux origines.

C'est la seule façon d'éviter que la version « simple » dérive de l'autre et
finisse par avoir ses propres bugs.
*/

/*
webProbeTerm est le mot envoyé pour vérifier qu'une source répond.

Le mot d'origine est irrécupérable : l'utilisateur a remplacé « moebius » par
`{terms}` dans son adresse, et il n'en reste rien. Une lettre isolée est le
meilleur repli — c'est le terme qui a le plus de chances de rendre quelque
chose, et un essai qui ne rend rien déclarerait cassée une source qui marche.
*/
const webProbeTerm = "a"

// WebSpec est la description d'un site, telle qu'elle est saisie.
type WebSpec struct {
	/*
		SearchURL est l'adresse de recherche complète, `{terms}` à la place des
		mots cherchés.

		Une seule zone de saisie plutôt que « domaine », « chemin » et
		« paramètre » séparés : personne ne pense une recherche en trois
		morceaux. On copie l'adresse depuis la barre du navigateur après avoir
		cherché sur le site, et on remplace le mot cherché par `{terms}`.
	*/
	SearchURL string `json:"searchUrl"`

	// Row désigne le conteneur d'UN résultat, celui qui se répète.
	Row string `json:"row"`

	Title  string `json:"title"`
	Author string `json:"author,omitempty"`
	Cover  string `json:"cover,omitempty"`

	/*
		Link est le lien porté par la ligne.

		Rendu à la fois comme fiche et comme lien d'acquisition, faute de savoir
		lequel il est. Un site liste rarement des liens de téléchargement
		directs ; il mène le plus souvent à une page. Le proposer aux deux
		usages laisse l'utilisateur ouvrir la fiche, et l'import échouer
		proprement si ce n'est pas un fichier — plutôt que de refuser d'avance
		un lien qui aurait pu marcher.
	*/
	Link string `json:"link,omitempty"`

	// MediaType renseigne le type du lien, que le HTML n'annonce jamais.
	MediaType string `json:"mediaType,omitempty"`

	// IgnoreRobots désactive la lecture de robots.txt pour ce site. Voir
	// Template.IgnoreRobots : même sens, mêmes réserves.
	IgnoreRobots bool `json:"ignoreRobots,omitempty"`

	/*
		Format vaut `html` — le défaut — ou `json`.

		Les six champs gardent leur sens, seule leur nature change : des
		sélecteurs CSS deviennent des chemins JSON. C'est ce qui permet au
		formulaire de ne pas doubler ses champs, et à qui a déjà décrit un site
		de comprendre une API sans rien réapprendre.
	*/
	Format string `json:"format,omitempty"`
}

// ParseWebSpec lit une description saisie, la valide et la compile.
func ParseWebSpec(raw []byte) (*Compiled, error) {
	var spec WebSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("%w : description illisible : %w", ErrInvalidTemplate, err)
	}
	return spec.Compile()
}

/*
Compile traduit la saisie en gabarit.

L'URL de recherche est découpée ici, et pas demandée en morceaux à
l'utilisateur : le domaine devient le miroir, le chemin et les paramètres
deviennent la requête. C'est un travail d'analyse que la machine fait mieux, et
qu'un formulaire ferait faire à la main.
*/
func (s WebSpec) Compile() (*Compiled, error) {
	var problems []string

	search := strings.TrimSpace(s.SearchURL)
	if search == "" {
		problems = append(problems, "l'adresse de recherche est requise")
	} else if !strings.Contains(search, "{terms}") {
		// Sans marqueur, la même page reviendrait quelle que soit la recherche.
		// La panne serait silencieuse : des résultats s'afficheraient, toujours
		// les mêmes.
		problems = append(problems,
			"l'adresse doit contenir {terms} à l'endroit des mots cherchés")
	}
	format := strings.ToLower(strings.TrimSpace(s.Format))
	if format == "" {
		format = FormatHTML
	}
	if format != FormatHTML && format != FormatJSON {
		problems = append(problems, "le format doit être html ou json")
	}

	// En JSON, une racine tableau est légitime : certaines API rendent
	// directement la liste, sans objet enveloppant.
	if strings.TrimSpace(s.Row) == "" && format == FormatHTML {
		problems = append(problems, "le sélecteur de résultat est requis")
	}
	if strings.TrimSpace(s.Title) == "" {
		problems = append(problems, "le sélecteur de titre est requis")
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("%w : %s", ErrInvalidTemplate, strings.Join(problems, " ; "))
	}

	/*
		`{terms}` est retiré le temps de l'analyse.

		Une accolade n'est pas un caractère valide dans une URL : `url.Parse`
		l'accepte, mais l'encode ensuite en `%7B`, et le marqueur ne serait plus
		reconnu au moment de le remplacer. On analyse donc une adresse propre,
		puis on remet le marqueur dans les valeurs.

		Le jeton est alphanumérique, et il le faut : une première version
		utilisait des octets NUL, que `url.Parse` refuse comme caractères de
		contrôle — l'adresse entière était alors déclarée illisible.
	*/
	const sentinel = "boxincloudtermsplaceholder"
	parsed, err := url.Parse(strings.ReplaceAll(search, "{terms}", sentinel))
	if err != nil {
		return nil, fmt.Errorf("%w : adresse illisible : %w", ErrInvalidTemplate, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%w : l'adresse doit commencer par http:// ou https://",
			ErrInvalidTemplate)
	}

	query := map[string]string{}
	for name, values := range parsed.Query() {
		if len(values) > 0 {
			query[name] = strings.ReplaceAll(values[0], sentinel, "{terms}")
		}
	}

	template := Template{
		Format:       format,
		IgnoreRobots: s.IgnoreRobots,
		// L'identifiant ne sert qu'au journal et aux compartiments de débit :
		// une source décrite ici est désignée par son genre `web` et son
		// identifiant de ligne, pas par un nom de gabarit.
		ID:      "web",
		Name:    parsed.Host,
		Mirrors: []string{parsed.Scheme + "://" + parsed.Host},
		Search: SearchSpec{
			Method: "GET",
			Path:   strings.ReplaceAll(parsed.EscapedPath(), sentinel, "{terms}"),
			Query:  query,
			Probe:  webProbeTerm,
		},
		Results: ResultsSpec{
			Select: strings.TrimSpace(s.Row),
			Fields: map[string]FieldSpec{
				fieldTitle: {Select: strings.TrimSpace(s.Title)},
			},
		},
	}

	if author := strings.TrimSpace(s.Author); author != "" {
		// `All` ne vaut qu'en HTML : en JSON, un chemin qui traverse un tableau
		// rend déjà plusieurs valeurs, et la validation refuse les options
		// propres au HTML.
		field := FieldSpec{Select: author}
		if format == FormatHTML {
			field.All = true
		}
		template.Results.Fields[fieldAuthors] = field
	}
	/*
		En HTML, une couverture est un `<img>` dont on lit `src`, et un lien un
		`<a>` dont on lit `href`. En JSON, le chemin désigne DÉJÀ la valeur :
		réclamer un attribut n'aurait pas de sens, et la validation le refuse.
	*/
	attr := func(name string) FieldSpec {
		if format == FormatJSON {
			return FieldSpec{}
		}
		return FieldSpec{From: "attr", Attr: name}
	}

	if cover := strings.TrimSpace(s.Cover); cover != "" {
		field := attr("src")
		field.Select = cover
		template.Results.Fields[fieldCoverURL] = field
	}
	if link := strings.TrimSpace(s.Link); link != "" {
		// Le même lien sert de fiche et d'acquisition : voir WebSpec.Link.
		page := attr("href")
		page.Select = link
		template.Results.Fields[fieldPageURL] = page

		acquisition := attr("href")
		acquisition.Select = link
		acquisition.MediaType = strings.TrimSpace(s.MediaType)
		template.Results.Fields[fieldAcquisition] = acquisition
	}

	template.applyDefaults()
	if err := template.validate(); err != nil {
		return nil, err
	}
	return template.compile()
}
