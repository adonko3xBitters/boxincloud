package amule

import (
	"context"
	"errors"
	"time"
)

// ErrNoDaemonRow signale l'absence de la ligne de configuration.
//
// Traduite par le service en ErrNotConfigured : l'appelant n'a pas à savoir
// qu'il existe une table, ni qu'elle est vide.
var ErrNoDaemonRow = errors.New("amule : ligne de configuration absente")

// StoredDaemon est la configuration telle qu'elle est persistée, mot de passe
// scellé compris.
//
// Ce type ne sort jamais du paquet : le service l'ouvre, s'en sert, et n'expose
// que Daemon.
type StoredDaemon struct {
	Host        string
	Port        int
	PasswordEnc []byte
	Label       string

	LastState  string
	LastDetail string
	LastSeenAt *time.Time
}

// Repository persiste la configuration du démon.
//
// Interface déclarée ici, au point d'usage : le service reste testable sans
// PostgreSQL, et le paquet ne dépend pas de sqlc.
type Repository interface {
	GetDaemon(ctx context.Context) (StoredDaemon, error)
	SaveDaemon(ctx context.Context, d StoredDaemon) error
	DeleteDaemon(ctx context.Context) error

	// SetState écrit le dernier état constaté sans toucher aux identifiants.
	//
	// `seen` n'est vrai que si le démon a effectivement répondu : un échec ne
	// doit pas rafraîchir la date de dernier contact, sans quoi l'interface
	// affiche « vu à l'instant » pour un démon injoignable depuis une heure.
	SetState(ctx context.Context, state State, detail string, seen bool) error

	// ─── Pont vers la bibliothèque ───────────────────────────────────────

	ListDestinations(ctx context.Context) ([]Destination, error)
	GetDestination(ctx context.Context, category int) (Destination, error)
	SaveDestination(ctx context.Context, d Destination) (Destination, error)

	/*
		ClaimPublication réserve le traitement d'un fichier terminé.

		Rend false si l'empreinte est déjà connue. C'est ce qui rend le pont
		idempotent SANS verrou : deux tours de scrutation qui verraient le même
		fichier terminé ne produiront qu'une publication, le second n'obtenant
		pas la réservation.
	*/
	ClaimPublication(ctx context.Context, p Publication) (bool, error)

	SetPublicationResult(ctx context.Context, hash string, result Publication) error
	ListPublications(ctx context.Context, limit int) ([]Publication, error)
}
