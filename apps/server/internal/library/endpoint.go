package library

import (
	"errors"
	"fmt"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/netguard"
)

/*
Contrôle des adresses de backend.

Le raisonnement et la liste des adresses refusées vivent dans
`internal/platform/netguard` : le même contrôle s'applique aux catalogues OPDS
distants, qui sont eux aussi des URL saisies depuis l'interface et jointes par
le serveur. Une seule liste, un seul endroit où la corriger.

Ce qui reste ici est la traduction en erreur du domaine, pour que l'appelant
puisse écrire `errors.Is(err, ErrInvalidConfig)` sans rien savoir du réseau.
*/

// ErrForbiddenEndpoint signale une adresse que le serveur refuse de joindre.
type ErrForbiddenEndpoint struct {
	Endpoint string
	Reason   string
}

func (e ErrForbiddenEndpoint) Error() string {
	return fmt.Sprintf("adresse de backend refusée (%s) : %s", e.Endpoint, e.Reason)
}

func (e ErrForbiddenEndpoint) Is(target error) bool {
	return target == ErrInvalidConfig
}

// CheckEndpoint refuse une adresse de backend manifestement illégitime.
func CheckEndpoint(endpoint string) error {
	err := netguard.Check(endpoint)
	if err == nil {
		return nil
	}

	var forbidden netguard.ErrForbidden
	if errors.As(err, &forbidden) {
		return ErrForbiddenEndpoint{Endpoint: forbidden.Endpoint, Reason: forbidden.Reason}
	}
	return err
}
