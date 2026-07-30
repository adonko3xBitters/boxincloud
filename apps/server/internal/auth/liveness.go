package auth

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

/*
Vérification que le compte est toujours en vie.

Un jeton d'accès est autoporteur : sa signature suffit à l'authentifier, sans
requête en base. C'est ce qui rend l'API rapide, et c'est aussi son défaut —
désactiver un compte ou lui retirer son rôle ne se voit nulle part tant que son
jeton n'a pas expiré. Un compte désactivé continuait donc de lire pendant un
quart d'heure, et un administrateur rétrogradé restait administrateur d'autant.

Le compromis retenu : une lecture en base, mise en cache quelques secondes. Le
coût est d'une requête par compte actif toutes les quinze secondes, et la
fenêtre de résidu tombe de quinze minutes à quinze secondes. Une désactivation
faite sur ce processus purge l'entrée immédiatement, ce qui ramène la fenêtre à
zéro dans le cas courant d'un serveur unique.
*/

// ErrAccountDisabled signale un compte désactivé ou supprimé.
var ErrAccountDisabled = errors.New("auth : compte désactivé")

// livenessTTL borne la durée pendant laquelle un état de compte est réutilisé.
//
// Quinze secondes : assez court pour qu'une désactivation soit ressentie comme
// immédiate, assez long pour qu'une rafale de requêtes — une grille de soixante
// vignettes — ne produise qu'une seule lecture.
const livenessTTL = 15 * time.Second

type livenessEntry struct {
	role      string
	active    bool
	refreshed time.Time
}

type livenessCache struct {
	mu      sync.RWMutex
	entries map[uuid.UUID]livenessEntry
}

func newLivenessCache() *livenessCache {
	return &livenessCache{entries: make(map[uuid.UUID]livenessEntry)}
}

func (c *livenessCache) get(id uuid.UUID, now time.Time) (livenessEntry, bool) {
	c.mu.RLock()
	entry, ok := c.entries[id]
	c.mu.RUnlock()

	if !ok || now.Sub(entry.refreshed) > livenessTTL {
		return livenessEntry{}, false
	}
	return entry, true
}

func (c *livenessCache) put(id uuid.UUID, entry livenessEntry) {
	c.mu.Lock()
	c.entries[id] = entry
	c.mu.Unlock()
}

func (c *livenessCache) forget(id uuid.UUID) {
	c.mu.Lock()
	delete(c.entries, id)
	c.mu.Unlock()
}

/*
AccountState retourne le rôle courant d'un compte, ou une erreur s'il n'est
plus actif.

Le rôle vient de la base, pas du jeton : c'est ce qui permet à une
rétrogradation de prendre effet sans attendre l'expiration.
*/
func (s *Service) AccountState(ctx context.Context, userID uuid.UUID) (string, error) {
	now := time.Now()

	if entry, ok := s.liveness.get(userID, now); ok {
		if !entry.active {
			return "", ErrAccountDisabled
		}
		return entry.role, nil
	}

	user, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			// GetUser filtre les comptes supprimés : leur absence EST la
			// désactivation.
			s.liveness.put(userID, livenessEntry{active: false, refreshed: now})
			return "", ErrAccountDisabled
		}
		// Une panne de base ne doit pas déconnecter tout le monde : on laisse
		// remonter l'erreur, que l'appelant traduira en 500 plutôt qu'en 401.
		return "", err
	}

	s.liveness.put(userID, livenessEntry{role: user.Role, active: true, refreshed: now})
	return user.Role, nil
}

// ForgetAccount purge l'état mis en cache d'un compte.
//
// À appeler après toute modification qui doit prendre effet tout de suite :
// désactivation, changement de rôle. Sans cet appel, le changement attendrait
// l'expiration de l'entrée.
func (s *Service) ForgetAccount(userID uuid.UUID) {
	s.liveness.forget(userID)
}
