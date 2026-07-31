package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/auth"
)

/*
Authentification Basic, pour les lecteurs OPDS.

Elle n'existe que pour eux. Les lecteurs de bande dessinée tiers — Chunky,
Panels, KyBook, Moon+ Reader — n'implémentent pas d'échange de jetons : ils
envoient un couple identifiant/mot de passe sur CHAQUE requête, et c'est la
seule authentification que la spécification OPDS mentionne. Sans elle, aucun de
ces lecteurs ne peut se connecter, ce qui vide le catalogue OPDS de son intérêt.

Elle reste confinée aux routes `/opds`. L'API du projet garde ses jetons, dont
la révocation par appareil et l'expiration courte n'ont pas d'équivalent ici.

# Le cache, et pourquoi il n'est pas optionnel

argon2id coûte volontairement cher — c'est ce qui protège les mots de passe. Un
lecteur qui affiche une page de vingt couvertures fait vingt et une requêtes, et
donc vingt et une vérifications complètes. À une centaine de millisecondes de
processeur chacune, une simple consultation de catalogue occupe deux secondes de
calcul, et deux lecteurs suffisent à saturer une machine modeste.

Le résultat est donc mémorisé une minute. La fenêtre est courte pour que la
révocation d'un compte prenne effet vite, et suffisante pour couvrir la rafale
d'une page.

Le mot de passe n'est jamais gardé, même haché avec son propre sel : la clé du
cache est un HMAC-SHA256 calculé avec un secret tiré au démarrage et qui ne
quitte pas le processus. Un vidage mémoire ne rend donc rien de réutilisable,
et la clé change à chaque redémarrage.
*/

const (
	// basicTTL borne la validité d'un couple mémorisé.
	basicTTL = time.Minute
	// basicMaxEntries borne le cache. Sans plafond, un attaquant créerait une
	// entrée par tentative et ferait grossir la mémoire indéfiniment.
	basicMaxEntries = 512
)

// BasicVerifier est ce que le middleware demande au service d'authentification.
type BasicVerifier interface {
	VerifyCredentials(ctx context.Context, username, password string) (auth.User, error)
}

type basicEntry struct {
	claims  auth.Claims
	expires time.Time
}

// BasicCache mémorise les vérifications récentes.
type BasicCache struct {
	mu      sync.Mutex
	entries map[string]basicEntry
	secret  []byte
	now     func() time.Time
}

func NewBasicCache() *BasicCache {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		// Le générateur du système est indisponible : refuser de démarrer vaut
		// mieux que de mémoriser des couples sous une clé prévisible.
		panic("middleware : générateur aléatoire indisponible : " + err.Error())
	}
	return &BasicCache{
		entries: make(map[string]basicEntry, basicMaxEntries),
		secret:  secret,
		now:     time.Now,
	}
}

func (c *BasicCache) key(username, password string) string {
	mac := hmac.New(sha256.New, c.secret)
	// Le séparateur empêche qu'un identifiant se terminant par le début d'un
	// mot de passe produise la même empreinte qu'un autre couple.
	mac.Write([]byte(username))
	mac.Write([]byte{0})
	mac.Write([]byte(password))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *BasicCache) get(username, password string) (auth.Claims, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[c.key(username, password)]
	if !ok || c.now().After(entry.expires) {
		return auth.Claims{}, false
	}
	return entry.claims, true
}

func (c *BasicCache) put(username, password string, claims auth.Claims) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Éviction grossière : au plafond, on vide. Une éviction fine ne vaudrait
	// pas son code ici — les entrées durent une minute, et le cas qui remplit
	// le cache est une tentative de force brute, qu'on n'a aucune raison de
	// servir efficacement.
	if len(c.entries) >= basicMaxEntries {
		c.entries = make(map[string]basicEntry, basicMaxEntries)
	}

	c.entries[c.key(username, password)] = basicEntry{
		claims:  claims,
		expires: c.now().Add(basicTTL),
	}
}

/*
BasicAuth authentifie sur un couple identifiant/mot de passe.

Les claims posées sont les mêmes que celles d'`Authenticate`, à l'appareil près :
un lecteur OPDS n'en déclare aucun. Tout ce qui suit — la construction du
lecteur, le filtrage par bibliothèque et par classification d'âge — fonctionne
donc à l'identique, sans rien savoir de la façon dont l'authentification a eu
lieu.

Le `WWW-Authenticate` sur le refus n'est pas décoratif : c'est lui qui déclenche
la demande d'identifiants dans un lecteur tiers. Sans cet en-tête, l'utilisateur
voit un catalogue vide sans jamais qu'on lui demande de se connecter.
*/
func BasicAuth(verifier BasicVerifier, cache *BasicCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if !ok || username == "" {
				challenge(w)
				return
			}

			if claims, cached := cache.get(username, password); cached {
				next.ServeHTTP(w, r.WithContext(
					context.WithValue(r.Context(), claimsKey{}, claims)))
				return
			}

			user, err := verifier.VerifyCredentials(r.Context(), username, password)
			if err != nil {
				if errors.Is(err, auth.ErrAccountDisabled) ||
					errors.Is(err, auth.ErrInvalidCredentials) {
					challenge(w)
					return
				}
				// Une panne de base n'est pas un défaut d'identifiants : la
				// confondre ferait redemander son mot de passe à quelqu'un dont
				// il est correct.
				http.Error(w, "service unavailable", http.StatusServiceUnavailable)
				return
			}

			claims := auth.Claims{
				UserID:   user.ID,
				Username: user.Username,
				Role:     user.Role,
			}
			cache.put(username, password, claims)

			next.ServeHTTP(w, r.WithContext(
				context.WithValue(r.Context(), claimsKey{}, claims)))
		})
	}
}

func challenge(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="boxincloud", charset="UTF-8"`)
	http.Error(w, "authentication required", http.StatusUnauthorized)
}
