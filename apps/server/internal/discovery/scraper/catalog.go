package scraper

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adonko3xBitters/boxincloud/server/internal/discovery"
)

/*
Chargement des gabarits.

# Deux origines, deux statuts

Les gabarits EMBARQUÉS voyagent dans le binaire. Ce sont ceux que le projet
livre, revus comme du code, et soumis au critère d'admission de la feuille de
route. Un gabarit embarqué fautif empêche le démarrage : il est passé par une
revue, donc son échec est un défaut de livraison, pas une erreur d'exploitation.

Les gabarits d'OPÉRATEUR viennent d'un répertoire, désactivé par défaut. Ce sont
ceux qu'un administrateur écrit pour son propre besoin — l'intranet de sa
médiathèque, un site local. Un gabarit d'opérateur fautif est IGNORÉ avec un
message : il n'a été revu par personne, et faire tomber une instance entière
pour un fichier déposé à la main serait disproportionné.

La différence de traitement est le seul endroit où la distinction se voit dans
le code, et elle est délibérée.

# Pourquoi un catalogue plutôt que des variables globales

Le catalogue est construit au câblage et passé au client, comme le registre des
bases de métadonnées. Un état global rendrait les tests dépendants de leur ordre
d'exécution — le défaut classique des paquets de plugins.
*/

// Le répertoire entier, et non `templates/*.yaml` : ce motif exige au moins une
// correspondance à la compilation, ce qui rendrait impossible de livrer le
// moteur sans gabarit. Or c'est exactement la situation aujourd'hui — voir
// templates/README.md.
//
//go:embed templates
var embedded embed.FS

// Catalog rassemble les gabarits chargés.
//
// Construit une fois au démarrage puis lu seulement : aucune synchronisation
// n'est nécessaire, et en ajouter suggérerait à tort qu'on peut y écrire à
// chaud.
type Catalog struct {
	byID map[string]*Compiled
}

// LoadEmbedded charge les gabarits livrés avec le binaire.
//
// Une erreur ici est un défaut de livraison, pas de configuration : elle doit
// remonter jusqu'à l'échec du démarrage.
func LoadEmbedded() (*Catalog, error) {
	catalog := &Catalog{byID: map[string]*Compiled{}}

	entries, err := fs.Glob(embedded, "templates/*.yaml")
	if err != nil {
		return nil, fmt.Errorf("scraper : lecture des gabarits embarqués : %w", err)
	}
	sort.Strings(entries)

	for _, name := range entries {
		raw, err := embedded.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("scraper : %s : %w", name, err)
		}
		compiled, err := Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("scraper : %s : %w", name, err)
		}
		if _, clash := catalog.byID[compiled.ID]; clash {
			return nil, fmt.Errorf("scraper : %s : identifiant %q déjà pris",
				name, compiled.ID)
		}
		catalog.byID[compiled.ID] = compiled
	}
	return catalog, nil
}

/*
LoadDir ajoute les gabarits d'un répertoire d'opérateur.

Un répertoire absent n'est pas une erreur : c'est le cas normal, celui d'une
instance qui n'a rien à ajouter.

Un gabarit d'opérateur ne peut PAS remplacer un gabarit embarqué. Autoriser
l'écrasement ferait d'un fichier déposé dans un volume le moyen de redéfinir
silencieusement une source livrée par le projet, ce qui n'est pas la même chose
qu'en ajouter une.
*/
func (c *Catalog) LoadDir(dir string, log *slog.Logger) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		log.Info("répertoire de gabarits absent", slog.String("dir", dir))
		return nil
	}
	if err != nil {
		return fmt.Errorf("scraper : lecture de %s : %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())

		raw, err := os.ReadFile(path) //nolint:gosec // chemin fourni par l'opérateur, délibérément
		if err != nil {
			log.Warn("gabarit illisible", slog.String("file", path), slog.Any("err", err))
			continue
		}
		compiled, err := Parse(raw)
		if err != nil {
			log.Warn("gabarit refusé", slog.String("file", path), slog.Any("err", err))
			continue
		}
		if _, clash := c.byID[compiled.ID]; clash {
			log.Warn("gabarit ignoré : identifiant déjà pris",
				slog.String("file", path), slog.String("id", compiled.ID))
			continue
		}

		c.byID[compiled.ID] = compiled
		log.Info("gabarit d'opérateur chargé",
			slog.String("id", compiled.ID), slog.String("file", path))
	}
	return nil
}

// Get rend un gabarit par son identifiant.
func (c *Catalog) Get(id string) (*Compiled, bool) {
	compiled, ok := c.byID[id]
	return compiled, ok
}

// List rend les gabarits chargés, triés par identifiant.
//
// Sert à l'administration : c'est cette liste que l'écran de configuration
// propose quand on ajoute une source de ce genre.
func (c *Catalog) List() []*Compiled {
	out := make([]*Compiled, 0, len(c.byID))
	for _, compiled := range c.byID {
		out = append(out, compiled)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

/*
ApplyRates déclare le débit sortant de chaque hôte auprès du limiteur.

Un compartiment par HÔTE, pas par gabarit. Deux miroirs d'un même site sont deux
machines, souvent chez deux hébergeurs : les compter ensemble ferait attendre
l'un à cause de l'autre, et l'intérêt d'un miroir est précisément de ne pas
partager le sort du premier.

À l'inverse, deux gabarits qui partageraient un hôte — un site et son
sous-domaine d'images — doivent bien partager un compartiment, puisque c'est la
même machine qui encaisse.
*/
func (c *Catalog) ApplyRates(throttle *discovery.Throttle) {
	if throttle == nil {
		return
	}
	for host, rate := range c.rates() {
		throttle.SetRate(bucketFor(host), rate)
	}
}

// rates calcule le débit retenu pour chaque hôte.
//
// Séparée d'`ApplyRates` pour être vérifiable : le limiteur n'expose pas ce
// qu'on lui a déclaré, et tester la composition à travers lui demanderait
// d'attendre réellement le temps qu'on mesure.
func (c *Catalog) rates() map[string]discovery.Rate {
	// Le débit le plus prudent l'emporte quand deux gabarits se partagent un
	// hôte : c'est la seule composition qui ne trahisse aucun des deux.
	strictest := map[string]discovery.Rate{}

	for _, compiled := range c.List() {
		rate := discovery.Rate{
			Every: compiled.Rate.Every.Std(),
			Burst: compiled.Rate.Burst,
		}
		for _, host := range compiled.Hosts() {
			current, seen := strictest[host]
			if !seen || rate.Every > current.Every {
				strictest[host] = rate
			}
		}
	}
	return strictest
}

// bucketFor nomme le compartiment de limitation d'un hôte.
//
// Préfixé pour ne pas entrer en collision avec les compartiments par genre du
// paquet parent — « opds », « openlibrary » — qui vivent dans le même limiteur.
func bucketFor(host string) string { return "scraper@" + strings.ToLower(host) }
