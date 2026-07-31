/*
Package discovery interroge des catalogues extérieurs et agrège leurs réponses.

# Ce que ce paquet fédère, et ce qu'il ne fédérera pas

Il ne connaît qu'un protocole : **OPDS**, le format de catalogue ouvert que
servent Komga, Kavita, Calibre-Web, Standard Ebooks, Project Gutenberg, une
autre instance boxincloud, et la plupart des bibliothèques numériques publiques.

Ce choix est une frontière, pas une étape. Fédérer de l'OPDS, c'est interroger
des catalogues auxquels l'utilisateur a déjà accès : soit publics, soit ouverts
par des identifiants qu'il fournit lui-même. La légitimité de l'accès est
établie avant que boxincloud n'entre en jeu, et le protocole est public.

Le paquet ne fournit donc pas — et ne fournira pas — de registre de sources
librement configurable pointant vers des bibliothèques clandestines. Un tel
mécanisme n'est pas neutre du fait d'être configurable : concevoir l'outil pour
que le contenu vienne d'ailleurs est précisément ce qui fonde la responsabilité
de celui qui l'a conçu. Ici, l'adresse d'un catalogue est saisie par un
administrateur pour un service dont il a les clés, ce qui n'est pas la même
chose qu'un annuaire fourni avec le produit.

# Ce qu'une source doit savoir faire

L'interface est volontairement étroite, et calquée sur `storage.Provider`, qui a
fait ses preuves : ajouter un protocole de catalogue ne doit toucher qu'un
fichier.
*/
package discovery

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrSourceNotFound signale un catalogue absent de la configuration.
	ErrSourceNotFound = errors.New("discovery : catalogue introuvable")
	// ErrInvalidSource signale une configuration de catalogue inutilisable.
	ErrInvalidSource = errors.New("discovery : configuration de catalogue invalide")
	// ErrNoSearch signale un catalogue qui n'expose pas de recherche.
	ErrNoSearch = errors.New("discovery : ce catalogue n'expose pas de recherche")
)

// Kind identifie le protocole d'un catalogue.
//
// Une seule valeur aujourd'hui. Le type existe quand même : il oblige la base,
// le contrat et l'interface à réserver la place, ce qui coûte moins cher
// maintenant qu'une migration plus tard.
type Kind string

const KindOPDS Kind = "opds"

// Source est un catalogue configuré.
type Source struct {
	ID      uuid.UUID
	Name    string
	URL     string
	Kind    Kind
	Enabled bool
	// Username est vide quand le catalogue est public. Le mot de passe n'est
	// pas ici : il ne sort de la base que chiffré, et seulement pour construire
	// une requête.
	Username string
	// LastError garde le dernier échec rencontré, pour que l'administration
	// puisse montrer un catalogue en panne sans qu'il faille lire les journaux.
	LastError   string
	LastCheckAt *time.Time
	CreatedAt   time.Time
}

// Query décrit une recherche.
type Query struct {
	Text string
	// Limit borne le nombre de résultats retenus PAR catalogue. Un catalogue
	// bavard ne doit pas noyer les autres dans la page agrégée.
	Limit int
}

// Link est un lien porté par un résultat.
type Link struct {
	Href string `json:"href"`
	Type string `json:"type,omitempty"`
	Rel  string `json:"rel,omitempty"`
}

/*
Result est une entrée de catalogue, normalisée.

Les champs sont ceux que l'OPDS garantit en pratique, pas ceux qu'il autorise.
Beaucoup de catalogues n'en remplissent qu'une moitié ; l'interface doit donc
rester lisible avec un titre et rien d'autre.
*/
type Result struct {
	SourceID   uuid.UUID `json:"sourceId"`
	SourceName string    `json:"sourceName"`

	Title     string   `json:"title"`
	Authors   []string `json:"authors,omitempty"`
	Series    string   `json:"series,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	Language  string   `json:"language,omitempty"`
	Published string   `json:"published,omitempty"`
	CoverURL  string   `json:"coverUrl,omitempty"`

	// Acquisitions sont les liens de téléchargement annoncés par le catalogue.
	Acquisitions []Link `json:"acquisitions,omitempty"`
	// PageURL renvoie vers la fiche, quand le catalogue en publie une.
	PageURL string `json:"pageUrl,omitempty"`

	/*
		InLibrary marque ce que l'instance possède déjà.

		C'est le résultat le plus utile d'une recherche fédérée, et celui qu'on
		oublie de rendre : quand on interroge trois catalogues dont deux sont
		ses propres instances, l'issue la plus fréquente est « tu l'as déjà ».
		Sans ce marqueur, l'utilisateur retélécharge ce qu'il possède.
	*/
	InLibrary bool `json:"inLibrary"`
}

// SourceStatus rapporte ce qu'un catalogue a répondu.
//
// Une recherche fédérée réussit partiellement par nature : un catalogue lent ou
// éteint ne doit pas faire échouer les autres. L'interface a donc besoin de
// distinguer « aucun résultat » de « pas de réponse », qui ne se disent pas de
// la même façon à l'utilisateur.
type SourceStatus struct {
	SourceID uuid.UUID `json:"sourceId"`
	Name     string    `json:"name"`
	Count    int       `json:"count"`
	// Error est un code stable, pas une phrase : l'interface le traduit.
	Error string `json:"error,omitempty"`
	// Elapsed sert au diagnostic d'un catalogue qui traîne.
	ElapsedMs int64 `json:"elapsedMs"`
}

// SearchResult agrège la réponse de tous les catalogues interrogés.
type SearchResult struct {
	Results []Result       `json:"results"`
	Sources []SourceStatus `json:"sources"`
}

/*
Fetched est un contenu ouvert chez un catalogue.

Le corps n'est pas lu : il est passé tel quel au dépôt, pour qu'une intégrale
de plusieurs centaines de méga-octets traverse le serveur sans jamais y tenir
en entier.
*/
type Fetched struct {
	Body io.ReadCloser
	// Size vaut -1 quand le catalogue répond en flux fragmenté, ce qui est
	// fréquent : le backend bascule alors sur un envoi en plusieurs parties.
	Size int64
	// Filename est celui que le catalogue déclare, vide s'il n'en déclare pas.
	Filename    string
	ContentType string
}

// Client interroge un catalogue d'un protocole donné.
//
// Déclarée au point d'usage : le service ne connaît que ces trois méthodes, ce
// qui permet de le tester entièrement sans réseau.
type Client interface {
	Search(ctx context.Context, source Source, password string, q Query) ([]Result, error)
	// Probe vérifie qu'un catalogue répond et expose une recherche, au moment
	// où l'administrateur l'ajoute plutôt qu'à la première requête.
	Probe(ctx context.Context, source Source, password string) error
	// Open ouvre un lien d'acquisition chez le catalogue. À l'appelant de
	// fermer le corps.
	Open(ctx context.Context, source Source, password, href string) (Fetched, error)
}
