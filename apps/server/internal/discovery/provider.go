package discovery

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

/*
Fournisseurs de recherche.

# Pourquoi cette abstraction, alors que `Client` existe déjà

`Client` est taillée pour un PROTOCOLE : une implémentation d'OPDS, à laquelle on
passe une source configurée. Cela suffisait tant qu'OPDS était le seul.

Les bases de métadonnées ouvertes ne rentrent pas dans ce moule, et pas par
accident : elles n'ont ni adresse à configurer — elles sont uniques et connues
d'avance — ni identifiants de l'utilisateur, ni fichier à télécharger. Les
plier de force dans `Client` obligerait à inventer une `Source` fictive pour
chacune, et à laisser `Open` retourner une erreur pour la moitié des
implémentations.

D'où deux familles, distinguées par ce qu'elles savent faire plutôt que par leur
protocole.

# La distinction qui gouverne tout le reste

**Une base de métadonnées ne produit JAMAIS de ligne dans les résultats de
recherche.**

C'est la décision structurante de ce fichier, et elle mérite d'être défendue.
Open Library sait qu'une œuvre existe ; elle ne sait pas vous la donner. Une
ligne de résultat sans lien d'acquisition est une ligne sur laquelle on ne peut
rien faire — le défaut exact qu'on a corrigé pour les entrées de navigation
OPDS, qui encombraient les résultats en menant à du XML brut.

Les métadonnées servent donc à deux autres choses, plus utiles :

  - **enrichir** un résultat d'acquisition dont le catalogue d'origine n'a rendu
    qu'un titre — beaucoup de petits catalogues OPDS sont dans ce cas ;
  - **rapprocher** un album déjà possédé de sa fiche, pour corriger ce que
    l'analyse d'un nom de fichier n'a pas su déduire.

Deux usages, deux chemins, et aucun des deux ne consiste à allonger la liste.
*/

// Capability dit ce qu'un fournisseur sait faire.
//
// Un champ de bits plutôt que deux interfaces exclusives : rien n'interdit
// qu'un fournisseur sache les deux. Un Internet Archive, par exemple, publie à
// la fois un flux OPDS et une API de métadonnées.
type Capability uint8

const (
	// CanAcquire : le fournisseur mène à un fichier téléchargeable.
	CanAcquire Capability = 1 << iota
	// CanDescribe : le fournisseur décrit une œuvre sans la fournir.
	CanDescribe
)

func (c Capability) Has(want Capability) bool { return c&want != 0 }

// Provider est le socle commun. Il ne cherche rien par lui-même : ce sont les
// deux interfaces ci-dessous qui portent le travail.
type Provider interface {
	// Kind identifie le fournisseur de façon stable : "opds", "openlibrary",
	// "googlebooks". Sert de clé de registre, de préfixe de cache et de
	// compartiment de limitation de débit.
	Kind() string
	// Name est le nom affiché. Pour un catalogue OPDS, celui que
	// l'administrateur a saisi ; pour une base publique, son nom propre.
	Name() string
	Capabilities() Capability
}

// AcquisitionProvider mène à des fichiers.
//
// C'est la seule famille dont les résultats apparaissent dans la liste de
// recherche.
type AcquisitionProvider interface {
	Provider

	Search(ctx context.Context, q Query) ([]Result, error)
	// Open ouvre un lien rendu par Search. À l'appelant de fermer le corps.
	Open(ctx context.Context, href string) (Fetched, error)
}

/*
DescriptionProvider décrit une œuvre sans la fournir.

Ses réponses ne deviennent jamais des lignes de résultat : elles complètent une
ligne existante, ou peuplent l'écran de rapprochement d'un album possédé.
*/
type DescriptionProvider interface {
	Provider

	Describe(ctx context.Context, w Work) ([]Description, error)
}

/*
Work est ce qu'on sait de l'œuvre à décrire.

Tous les champs sont facultatifs, et c'est le point : on interroge une base de
métadonnées précisément parce qu'on ne sait pas grand-chose. Un titre approximatif
tiré d'un nom de fichier est le cas courant, pas le cas dégradé.
*/
type Work struct {
	Title   string
	Authors []string
	// ISBN, quand on l'a, vaut tous les autres champs réunis : c'est le seul
	// identifiant qui rend le rapprochement certain plutôt que probable.
	ISBN string
	// Year borne la recherche quand plusieurs œuvres partagent un titre.
	Year string
	// Language en code ISO 639-1, pour écarter les traductions.
	Language string
}

/*
Description est une fiche rendue par une base de métadonnées.

Volontairement distincte de `Result`. Les fusionner en un seul type aurait fait
de `Acquisitions` un champ vide la moitié du temps, et rien n'aurait plus empêché
d'afficher une fiche dans la liste des résultats — ce que ce fichier existe
justement pour rendre impossible.
*/
type Description struct {
	// ProviderKind dit d'où vient la fiche. Deux bases se contredisent
	// régulièrement ; l'écran de rapprochement doit pouvoir le montrer.
	ProviderKind string `json:"providerKind"`
	ProviderName string `json:"providerName"`

	Title     string   `json:"title"`
	Authors   []string `json:"authors,omitempty"`
	Series    string   `json:"series,omitempty"`
	Number    string   `json:"number,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	Language  string   `json:"language,omitempty"`
	Published string   `json:"published,omitempty"`
	Publisher string   `json:"publisher,omitempty"`
	ISBN      string   `json:"isbn,omitempty"`
	PageCount int      `json:"pageCount,omitempty"`
	Subjects  []string `json:"subjects,omitempty"`
	CoverURL  string   `json:"coverUrl,omitempty"`
	// PageURL renvoie vers la fiche d'origine, pour que l'utilisateur puisse
	// vérifier ce qu'on lui propose.
	PageURL string `json:"pageUrl,omitempty"`

	/*
		Confidence est la force du rapprochement, de 0 à 1.

		Elle n'est pas décorative. Un rapprochement de métadonnées est flou par
		nature — deux albums peuvent partager un titre, une base peut proposer
		l'édition anglaise d'une œuvre française — et l'enrichissement ne doit
		s'appliquer qu'au-dessus d'un seuil, jamais « au mieux ».

		Sans elle, la seule politique possible serait « prendre le premier », qui
		écrase tôt ou tard la fiche d'un album par celle d'un homonyme.
	*/
	Confidence float64 `json:"confidence"`
}

/*
Factory construit un fournisseur pour une source configurée.

Nécessaire parce que les deux familles ne se peuplent pas de la même façon. Une
base publique est UNIQUE : une instance, enregistrée au démarrage. Un catalogue
OPDS est configuré N fois par l'administrateur, et il faut donc un fournisseur
par source — avec son adresse et ses identifiants.

C'est cette asymétrie que le registre absorbe, pour que le service n'ait à
connaître ni l'une ni l'autre.
*/
type Factory interface {
	Kind() string
	// For construit le fournisseur d'une source. Le mot de passe est déchiffré
	// par l'appelant : une fabrique n'a pas à connaître le coffre.
	For(source Source, password string) (AcquisitionProvider, error)
}

// ErrUnknownProvider signale un genre absent du registre.
var ErrUnknownProvider = fmt.Errorf("discovery : fournisseur inconnu")

/*
Registry rassemble les fournisseurs disponibles.

Sûr d'emploi concurrent : il est lu à chaque recherche, depuis autant de
goroutines qu'il y a de sources.
*/
type Registry struct {
	mu        sync.RWMutex
	builtin   map[string]Provider
	factories map[string]Factory
}

func NewRegistry() *Registry {
	return &Registry{
		builtin:   map[string]Provider{},
		factories: map[string]Factory{},
	}
}

// Register déclare un fournisseur unique — une base publique.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.builtin[p.Kind()] = p
}

// RegisterFactory déclare une fabrique — un protocole configuré par source.
func (r *Registry) RegisterFactory(f Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[f.Kind()] = f
}

// For construit le fournisseur d'une source configurée.
func (r *Registry) For(source Source, password string) (AcquisitionProvider, error) {
	r.mu.RLock()
	factory, ok := r.factories[string(source.Kind)]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w : %s", ErrUnknownProvider, source.Kind)
	}
	return factory.For(source, password)
}

/*
Describers rend les bases de métadonnées enregistrées, dans un ordre stable.

L'ordre compte : à confiance égale, c'est le premier qui l'emporte, et deux
recherches identiques doivent rendre la même fiche. Le tri par genre suffit à
le garantir sans imposer une notion de priorité qui n'aurait pas de sens.
*/
func (r *Registry) Describers() []DescriptionProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]DescriptionProvider, 0, len(r.builtin))
	for _, provider := range r.builtin {
		if !provider.Capabilities().Has(CanDescribe) {
			continue
		}
		if describer, ok := provider.(DescriptionProvider); ok {
			out = append(out, describer)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Kind() < out[j].Kind() })
	return out
}

/*
Best retient la meilleure fiche parmi celles rendues par plusieurs bases.

`min` est le seuil en dessous duquel on préfère ne rien proposer. Ce n'est pas
de la prudence excessive : au-dessous, la fiche a de bonnes chances d'être celle
d'un homonyme, et écraser les métadonnées d'un album par celles d'un autre est
plus grave que de le laisser incomplet. Une absence se voit et se corrige ; une
fiche fausse mais plausible ne se voit pas.
*/
func Best(descriptions []Description, min float64) (Description, bool) {
	best := Description{}
	found := false

	for _, candidate := range descriptions {
		if candidate.Confidence < min {
			continue
		}
		if !found || candidate.Confidence > best.Confidence {
			best, found = candidate, true
		}
	}
	return best, found
}

/*
Enrich complète un résultat sans jamais écraser ce qu'il porte déjà.

La règle est simple et vaut d'être tenue : le catalogue qui fournit le fichier
en sait plus sur SON édition qu'une base généraliste. Un résumé venu d'Open
Library est mieux que rien ; il n'est pas mieux que celui du catalogue.

L'enrichissement ne remplit donc que les trous.
*/
func Enrich(result *Result, from Description) {
	if result.Summary == "" {
		result.Summary = from.Summary
	}
	if result.CoverURL == "" {
		result.CoverURL = from.CoverURL
	}
	if result.Series == "" {
		result.Series = from.Series
	}
	if result.Language == "" {
		result.Language = from.Language
	}
	if result.Published == "" {
		result.Published = from.Published
	}
	if len(result.Authors) == 0 {
		result.Authors = from.Authors
	}
}

/*
MatchConfidence estime la force d'un rapprochement.

Une heuristique, et elle est nommée comme telle. Le titre normalisé pèse le plus
— c'est le seul champ que toutes les bases remplissent — l'auteur départage les
homonymes, l'année écarte les rééditions.

Un ISBN identique court-circuite tout le reste : c'est le seul cas où le
rapprochement est certain plutôt que probable.
*/
func MatchConfidence(want Work, got Description) float64 {
	if want.ISBN != "" && got.ISBN != "" {
		if normalizeISBN(want.ISBN) == normalizeISBN(got.ISBN) {
			return 1
		}
		// Deux ISBN connus et différents désignent deux éditions distinctes.
		// Le reste des indices ne peut pas rattraper cela.
		return 0
	}

	score := 0.0

	if normalizeTitle(want.Title) == normalizeTitle(got.Title) {
		score += 0.6
	} else if want.Title != "" && got.Title != "" &&
		strings.Contains(normalizeTitle(got.Title), normalizeTitle(want.Title)) {
		// Un titre contenu dans l'autre — « L'Incal » dans « L'Incal, intégrale »
		// — vaut mieux qu'une divergence, mais pas autant qu'une égalité.
		score += 0.35
	}

	if len(want.Authors) > 0 && len(got.Authors) > 0 {
		for _, wanted := range want.Authors {
			for _, candidate := range got.Authors {
				if normalizeTitle(wanted) == normalizeTitle(candidate) {
					score += 0.3
					break
				}
			}
		}
	}

	if want.Year != "" && got.Published != "" &&
		strings.HasPrefix(got.Published, want.Year) {
		score += 0.1
	}

	if score > 1 {
		score = 1
	}
	return score
}

// normalizeISBN retire tirets et espaces, seules variations d'écriture d'un
// ISBN.
func normalizeISBN(isbn string) string {
	var builder strings.Builder
	for _, r := range isbn {
		if r >= '0' && r <= '9' || r == 'X' || r == 'x' {
			builder.WriteRune(r)
		}
	}
	return strings.ToUpper(builder.String())
}
