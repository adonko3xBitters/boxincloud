package amule

import "time"

/*
Les types du domaine.

C'est ici que s'arrête le protocole. Rien de ce fichier ne mentionne un opcode,
un nom de tag ou une largeur d'entier : les handlers, le contrat OpenAPI et
l'interface ne connaissent que ces structures, et `mapping.go` est le seul
endroit qui traduise. Voir docs/adr/006-frontiere-ec-etanche.md.

Les codes rendus par le démon — état d'un téléchargement, priorité — sont
traduits en chaînes stables plutôt que laissés en entiers. Un entier oblige
chaque client à porter la table, et cette table changerait avec la version
d'aMule sans que rien ne le signale.
*/

// ─── Téléchargements ─────────────────────────────────────────────────────────

// DownloadStatus est l'état d'un téléchargement.
type DownloadStatus string

const (
	DownloadWaiting     DownloadStatus = "waiting"     // en file, aucune source active
	DownloadDownloading DownloadStatus = "downloading" // reçoit des données
	DownloadPaused      DownloadStatus = "paused"      // suspendu par l'utilisateur
	DownloadStopped     DownloadStatus = "stopped"     // arrêté, sources libérées
	DownloadErroneous   DownloadStatus = "erroneous"   // en erreur
	DownloadCompleting  DownloadStatus = "completing"  // assemblage en cours
	DownloadCompleted   DownloadStatus = "completed"   // terminé
	DownloadHashing     DownloadStatus = "hashing"     // vérification des parties
	DownloadAllocating  DownloadStatus = "allocating"  // réservation de l'espace disque
	DownloadUnknown     DownloadStatus = "unknown"     // code non reconnu
)

// Priority est la priorité d'un fichier, en téléchargement comme en partage.
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityNormal   Priority = "normal"
	PriorityHigh     Priority = "high"
	PriorityVeryLow  Priority = "verylow"
	PriorityVeryHigh Priority = "veryhigh"
	PriorityAuto     Priority = "auto" // priorité gérée par le démon
)

// SourceCounts détaille d'où viennent les sources d'un fichier.
//
// Les quatre nombres ne s'additionnent pas : `Transferring` est un
// sous-ensemble de `Total`, et `A4AF` compte des sources qui travaillent sur un
// AUTRE fichier et pourraient basculer sur celui-ci.
type SourceCounts struct {
	Total        int
	NotCurrent   int
	Transferring int
	A4AF         int
}

// Download décrit un fichier en cours de réception.
type Download struct {
	// Hash est l'empreinte eD2k, en hexadécimal minuscule. C'est l'identité du
	// fichier sur le réseau, et notre clé stable : le nom peut changer, le
	// numéro interne du démon aussi d'un redémarrage à l'autre.
	Hash string

	Name string
	Size int64

	// SizeDone est ce qui est acquis et vérifié ; SizeXfer ce qui a transité,
	// corruption comprise. Les deux diffèrent, et c'est l'écart qui explique un
	// téléchargement qui « recule ».
	SizeDone int64
	SizeXfer int64

	Speed int64 // octets par seconde

	Status   DownloadStatus
	Priority Priority
	Category int

	Sources SourceCounts

	// PartCount et AvailableParts décrivent la disponibilité : un fichier dont
	// aucune source ne détient certaines parties ne finira jamais, et c'est la
	// seule façon de le voir avant d'avoir attendu des heures.
	PartCount      int
	AvailableParts int

	// ETA est nil quand elle n'a pas de sens : vitesse nulle, fichier en pause,
	// ou terminé. Rendre « l'infini » obligerait l'interface à le détecter.
	ETA *time.Duration

	LastSeenComplete *time.Time
}

// Source est un pair qui détient tout ou partie d'un fichier.
type Source struct {
	// UserHash identifie le client sur le réseau, indépendamment de son IP.
	UserHash string

	Name     string
	Software string // « eMule », « aMule », « Shareaza »…
	Version  string

	// IP est vide si le pair est en LowID : il n'a pas d'adresse joignable.
	IP   string
	Port int

	// LowID dit que le pair n'accepte pas de connexion entrante. Il passe alors
	// par un serveur, ce qui coûte plus cher et échoue plus souvent.
	LowID bool

	Speed int64 // octets par seconde, sur ce fichier

	// QueueRank est notre rang dans la file d'attente de ce pair. Zéro signifie
	// « pas en file » — soit on transfère, soit il ne nous a pas classés.
	QueueRank int

	// AvailableParts est le nombre de parties que ce pair détient, sur
	// Download.PartCount.
	AvailableParts int

	Downloading bool
}

// ─── Envois ──────────────────────────────────────────────────────────────────

// Upload est un transfert sortant en cours.
type Upload struct {
	UserHash string
	Name     string
	Software string
	Version  string

	IP   string
	Port int

	// FileHash désigne le fichier servi.
	FileHash string
	FileName string

	Speed       int64
	Transferred int64

	// XferUp et XferUpSession distinguent le cumul historique avec ce pair de
	// ce qui a été envoyé depuis qu'il est connecté. C'est la base du système
	// de crédits : ce qu'on a donné à quelqu'un détermine ce qu'il nous rend.
	TotalUploaded   int64
	SessionUploaded int64
}

// QueuedPeer est un pair en attente d'un de nos fichiers.
type QueuedPeer struct {
	UserHash string
	Name     string
	IP       string
	Port     int

	FileHash string

	// Score est la note calculée par le démon : elle mêle ancienneté dans la
	// file et crédits acquis.
	Score int

	// WaitedSince dit depuis quand ce pair patiente.
	WaitedSince *time.Time
}

// ─── Partage ─────────────────────────────────────────────────────────────────

// SharedFile est un fichier que cette instance met à disposition.
type SharedFile struct {
	Hash string
	Name string
	Size int64
	Path string

	Priority Priority

	// Requests, Accepted et Transferred mesurent l'intérêt réel d'un fichier :
	// combien de fois il a été demandé, combien de fois servi, et ce que cela
	// a représenté en octets.
	Requests    int64
	Accepted    int64
	Transferred int64

	// Complete dit si le fichier est entier. Un fichier en cours de
	// téléchargement est partagé lui aussi — c'est le principe du réseau.
	Complete bool
}

// ─── Serveurs eD2k ───────────────────────────────────────────────────────────

// Server est un serveur eD2k connu de l'instance.
type Server struct {
	IP   string
	Port int

	Name        string
	Description string
	Version     string

	// Ping est en millisecondes. Zéro signifie « jamais mesuré ».
	Ping int

	Users    int
	MaxUsers int
	Files    int

	// Failed compte les échecs de connexion consécutifs. Un serveur mort le
	// reste, et ce compteur est ce qui permet de le voir.
	Failed int

	// Static marque un serveur que l'utilisateur a épinglé : il survit aux
	// mises à jour de la liste, contrairement aux autres.
	Static bool

	Priority Priority

	// Connected marque le serveur auquel l'instance est actuellement reliée.
	// Un seul à la fois.
	Connected bool
}

// ─── État de connexion ───────────────────────────────────────────────────────

// IDType distingue un client joignable d'un client qui ne l'est pas.
type IDType string

const (
	// IDHigh — le client accepte les connexions entrantes. Tout fonctionne.
	IDHigh IDType = "high"

	// IDLow — les connexions entrantes n'arrivent pas. Le client reste
	// utilisable mais dépend d'intermédiaires, ce qui limite ses sources.
	IDLow IDType = "low"

	// IDNone — pas connecté, la question ne se pose pas encore.
	IDNone IDType = "none"
)

// Ed2kState décrit le lien avec le réseau serveur.
type Ed2kState struct {
	Connected  bool
	Connecting bool

	// Server est le serveur joint, nil si aucun.
	Server *Server

	// ClientID est l'identifiant attribué par le serveur.
	ClientID uint32

	ID IDType
}

// KadState décrit le lien avec le réseau Kademlia.
type KadState struct {
	// Running dit si le moteur Kad tourne ; Connected s'il a trouvé le réseau.
	// Les deux sont distincts : un Kad démarré peut chercher ses pairs pendant
	// plusieurs minutes.
	Running   bool
	Connected bool

	// Firewalled dit que les connexions entrantes n'arrivent pas. Kad
	// fonctionne alors en dégradé — on peut chercher, mais on est moins
	// trouvable.
	Firewalled bool

	// FirewalledUDP est distinct : Kad utilise UDP pour ses recherches et TCP
	// pour les transferts, et les deux peuvent être bloqués séparément.
	FirewalledUDP bool
}

// Connection rassemble ce que l'interface affiche en tête de page.
type Connection struct {
	Ed2k Ed2kState
	Kad  KadState
}

// ─── Statistiques ────────────────────────────────────────────────────────────

// Stats est l'instantané des compteurs du démon.
type Stats struct {
	UpSpeed   int64 // octets par seconde
	DownSpeed int64

	// UpLimit et DownLimit sont les plafonds configurés. Zéro signifie
	// « aucune limite », ce qui n'est pas la même chose que « débit nul ».
	UpLimit   int64
	DownLimit int64

	UpOverhead   int64
	DownOverhead int64

	// TotalSources est le nombre de sources connues, tous fichiers confondus.
	TotalSources int

	// BannedPeers compte les pairs écartés pour comportement abusif.
	BannedPeers int

	// UploadQueueLength est le nombre de pairs qui attendent un de nos
	// fichiers. C'est la mesure de ce qu'on doit au réseau.
	UploadQueueLength int

	// Ed2kUsers et KadUsers sont les tailles estimées des deux réseaux.
	Ed2kUsers int
	KadUsers  int
	Ed2kFiles int
	KadFiles  int
}

// ─── Instantané ──────────────────────────────────────────────────────────────

/*
Snapshot est l'état complet du démon à un instant.

C'est l'unité de travail de la scrutation : on en prend un, on le compare au
précédent, et la différence produit les événements. Voir
docs/adr/005-temps-reel-sse-evenements-derives.md.

Il est immuable une fois construit. Les lecteurs — handlers HTTP, flux
d'événements — en partagent un pointeur sans copier, ce qui rend gratuit le fait
que vingt onglets lisent le même état.
*/
type Snapshot struct {
	// TakenAt date l'instantané. L'interface en a besoin pour dire depuis
	// combien de temps elle regarde des chiffres qui ne bougent plus.
	TakenAt time.Time

	Connection Connection
	Stats      Stats

	Downloads   []Download
	Uploads     []Upload
	QueuedPeers []QueuedPeer
	Servers     []Server
	SharedFiles []SharedFile
}
