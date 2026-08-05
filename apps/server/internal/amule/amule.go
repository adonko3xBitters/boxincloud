/*
Package amule pilote un démon aMule par son protocole External Connections.

boxincloud ne réimplémente aucun protocole pair-à-pair : le hash eD2k, l'AICH,
les fichiers partiels, le protocole serveur, le client-à-client, les crédits et
Kademlia sont l'affaire d'amuled, qui les parle depuis vingt ans. Ce paquet ne
fait que commander ce démon et traduire son état. Voir
docs/adr/004-amuled-plutot-que-reimplementation.md.

# La frontière

Aucun type du protocole EC ne franchit ce paquet. Les handlers HTTP, le contrat
OpenAPI et l'interface ne connaissent que les types déclarés ici — un `State`,
un `Daemon`, un `Status`. C'est ce qui permettra de changer de version d'aMule
sans toucher au reste. Voir docs/adr/006-frontiere-ec-etanche.md.

# Ce que fait ce paquet

Il porte la configuration du démon, ouvre une session EC, scrute son état à
cadence adaptative, en dérive des événements, lui transmet des commandes, et
publie dans la bibliothèque ce qu'il a fini de télécharger.
*/
package amule

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
)

/*
State décrit la situation du module, d'un seul coup d'œil.

L'énumération est déclarée ENTIÈRE, y compris les valeurs qu'une instance donnée
ne produira jamais. Ajouter une valeur à une énumération de réponse est une
rupture pour un client strict ; les déclarer toutes d'emblée l'évite.
*/
type State string

const (
	// StateDisabled — BOXINCLOUD_ED2K_ENABLED vaut false. C'est le défaut livré.
	StateDisabled State = "disabled"

	// StateUnconfigured — le module est actif, mais aucun démon n'est déclaré.
	StateUnconfigured State = "unconfigured"

	// StateDisconnected — un démon est déclaré, aucune session EC n'est ouverte.
	StateDisconnected State = "disconnected"

	// StateConnecting — une session EC est en cours d'établissement. Produit à
	// partir de l'étape 1.
	StateConnecting State = "connecting"

	// StateConnected — session EC ouverte, le démon répond. Étape 1.
	StateConnected State = "connected"
)

// Daemon décrit le démon déclaré, tel qu'on peut l'exposer.
//
// Le mot de passe EC n'y figure pas, et ne peut donc pas fuir par une réponse
// d'API. Il n'existe que sous forme scellée en base et en clair le temps d'une
// connexion — même discipline que les identifiants des backends de stockage.
type Daemon struct {
	Host  string
	Port  int
	Label string

	// LastState et LastDetail portent le dernier état constaté, persisté.
	//
	// Après un redémarrage, l'interface doit pouvoir dire « la dernière
	// connexion a échoué, voici pourquoi » plutôt qu'afficher un état neutre
	// qui laisserait croire que rien n'a jamais été tenté.
	LastState  string
	LastDetail string
	LastSeenAt *time.Time
}

// Status est ce que l'interface affiche en tête du module.
type Status struct {
	// Enabled reflète la configuration de déploiement, pas l'état de connexion.
	Enabled bool

	State  State
	Detail string

	// Daemon vaut nil tant qu'aucun démon n'est déclaré.
	Daemon *Daemon

	// IncomingDir est le répertoire d'arrivée, tel que ce serveur le voit.
	// Affiché parce qu'un montage manquant est la première cause de « le
	// téléchargement est fini mais rien n'arrive dans la bibliothèque ».
	IncomingDir string
}

// Erreurs du module.
var (
	// ErrDisabled — l'opération demande un module actif.
	ErrDisabled = errors.New("amule : module eD2k désactivé (BOXINCLOUD_ED2K_ENABLED)")

	// ErrNotConfigured — aucun démon n'est déclaré.
	ErrNotConfigured = errors.New("amule : aucun démon déclaré")
)

/*
RefusedError : le démon a compris la commande et l'a rejetée.

À distinguer d'une panne. Le démon va bien, il est joignable, il a lu la
requête — et il dit non, pour une raison qui lui appartient : Kad désactivé dans
ses préférences, adresse dans une plage qu'il écarte, fichier déjà en file.

`Detail` sont SES mots, en anglais, conservés tels quels. C'est le seul endroit
du projet où un texte non traduit atteint l'écran, et c'est délibéré : personne
d'autre que le démon ne sait pourquoi il a refusé, et une phrase de notre cru
— « la commande a échoué » — ferait perdre la seule information utile. L'interface
l'encadre en français et attribue la citation.
*/
type RefusedError struct {
	Detail string
}

func (e RefusedError) Error() string {
	if e.Detail == "" {
		return "amule : le démon a refusé la commande"
	}
	return "amule : le démon a refusé la commande : " + e.Detail
}

/*
translateEC ramène une erreur du codec vers le domaine.

L'ADR-006 veut qu'aucun type de `ec` ne franchisse `amule.Service`. Un
`ec.RefusedError` remonté tel quel le ferait — les handlers devraient importer
`ec` pour le reconnaître, et la frontière serait percée par le chemin d'erreur,
qui est celui qu'on surveille le moins.
*/
func translateEC(err error) error {
	var refused ec.RefusedError
	if errors.As(err, &refused) {
		return RefusedError{Detail: refused.Detail}
	}
	return err
}

/*
ValidationError porte les erreurs de saisie, champ par champ.

Une erreur globale obligerait l'interface à afficher un message au-dessus du
formulaire au lieu de désigner le champ fautif — ce qui, sur un formulaire de
quatre champs dont un port et un mot de passe, fait chercher.
*/
type ValidationError struct {
	Fields map[string]string
}

func (e ValidationError) Error() string {
	if len(e.Fields) == 0 {
		return "amule : configuration invalide"
	}

	// Ordre stable : ce message finit dans les journaux, et un ordre qui change
	// d'une exécution à l'autre rend deux traces incomparables.
	names := make([]string, 0, len(e.Fields))
	for name := range e.Fields {
		names = append(names, name)
	}
	sort.Strings(names)

	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s : %s", name, e.Fields[name]))
	}
	return "amule : configuration invalide — " + strings.Join(parts, ", ")
}
