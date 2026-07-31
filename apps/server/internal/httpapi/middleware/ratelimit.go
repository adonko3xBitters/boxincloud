package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
)

/*
Limitation de débit sur l'authentification.

Un mot de passe se devine si on a le droit d'essayer indéfiniment. argon2id
rend chaque essai coûteux — c'est sa raison d'être — mais coûteux pour le
serveur autant que pour l'attaquant : une rafale de tentatives sur une instance
familiale la met à genoux avant même d'avoir trouvé quoi que ce soit. La
limitation protège donc les deux à la fois, le compte et la machine.

En mémoire, pas en base. Une instance auto-hébergée est un processus unique
dans l'immense majorité des cas, et faire dépendre la connexion d'un aller-
retour en base ajouterait une panne possible à l'endroit le plus sensible.
Plusieurs répliques derrière un répartiteur diviseraient la limite par leur
nombre — une dégradation acceptable, et documentée plutôt que masquée.

Le seau à jetons plutôt qu'un compteur par fenêtre : une fenêtre fixe autorise
le double de la limite à cheval sur sa frontière, ce qui est exactement le
moment que choisirait quelqu'un qui a lu le code.
*/

// Limit décrit une politique de limitation.
type Limit struct {
	// Burst est le nombre de requêtes tolérées d'affilée.
	Burst int
	// Every est le délai de reconstitution d'un jeton.
	Every time.Duration
}

// AuthLimit s'applique aux routes de connexion.
//
// Cinq tentatives d'affilée, puis une toutes les douze secondes. Une faute de
// frappe, un mot de passe oublié, un gestionnaire qui remplit mal : cinq essais
// couvrent l'usage humain. Cinq par minute ne couvrent aucune attaque par
// dictionnaire, qui en demande des milliers.
var AuthLimit = Limit{Burst: 5, Every: 12 * time.Second}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

type limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	limit   Limit
}

func newLimiter(limit Limit) *limiter {
	return &limiter{buckets: make(map[string]*bucket), limit: limit}
}

// allow consomme un jeton et indique si la requête passe.
//
// Retourne aussi le délai avant qu'un jeton soit disponible, pour le
// `Retry-After` : un client qui sait quand réessayer n'a pas besoin de
// tâtonner, et tâtonner est précisément ce qui produit de la charge inutile.
func (l *limiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(l.limit.Burst), lastSeen: now}
		l.buckets[key] = b
	}

	// Reconstitution proportionnelle au temps écoulé, plafonnée au burst.
	elapsed := now.Sub(b.lastSeen)
	b.tokens += elapsed.Seconds() / l.limit.Every.Seconds()
	if b.tokens > float64(l.limit.Burst) {
		b.tokens = float64(l.limit.Burst)
	}
	b.lastSeen = now

	if b.tokens < 1 {
		missing := 1 - b.tokens
		return false, time.Duration(missing * float64(l.limit.Every))
	}

	b.tokens--
	return true, 0
}

// sweep supprime les seaux pleins et inactifs.
//
// Sans lui, la table grandirait d'une entrée par adresse vue et ne
// rétrécirait jamais : une fuite lente, mais une fuite.
func (l *limiter) sweep(now time.Time, idle time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for key, b := range l.buckets {
		if now.Sub(b.lastSeen) > idle {
			delete(l.buckets, key)
		}
	}
}

/*
RateLimit limite les requêtes par adresse d'origine.

La clé est l'adresse IP seule, pas l'adresse et l'identifiant. Compter par
identifiant permettrait à qui connaît un nom d'utilisateur de le verrouiller
depuis n'importe où — une attaque par déni de service sur un compte précis,
bien plus facile à monter qu'une attaque par dictionnaire.
*/
func RateLimit(limit Limit) func(http.Handler) http.Handler {
	l := newLimiter(limit)

	// Un nettoyage périodique plutôt qu'à chaque requête : le balayage prend un
	// verrou global, et le faire sur le chemin d'une connexion ferait payer à
	// l'utilisateur le ménage de tous les autres.
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for now := range ticker.C {
			l.sweep(now, time.Hour)
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, retryAfter := l.allow(clientIP(r), time.Now())
			if !ok {
				seconds := int(retryAfter.Seconds()) + 1
				w.Header().Set("Retry-After", strconv.Itoa(seconds))
				problem.Write(w, r, problem.TooManyRequests(
					"trop de tentatives ; réessayez dans "+strconv.Itoa(seconds)+" s"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

/*
clientIP extrait l'adresse d'origine.

`X-Forwarded-For` n'est lu que sur son PREMIER élément et sans vérifier qui l'a
écrit — parce qu'une instance auto-hébergée est presque toujours derrière un
reverse-proxy qu'elle contrôle. Un client peut donc forger cet en-tête s'il
atteint le serveur directement, et contourner la limite.

Ce n'est pas un oubli mais un arbitrage : sans cette lecture, TOUTES les
requêtes derrière un proxy partageraient l'adresse du proxy, et la limite
verrouillerait la maisonnée entière au premier mot de passe raté. Le déploiement
correct — le serveur n'écoutant que sur l'interface du proxy — rend l'en-tête
digne de confiance, et c'est celui que la documentation décrit.
*/
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if comma := indexByte(forwarded, ','); comma > 0 {
			return trimSpace(forwarded[:comma])
		}
		return trimSpace(forwarded)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
