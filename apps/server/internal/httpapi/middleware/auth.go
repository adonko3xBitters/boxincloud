package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/auth"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
)

type claimsKey struct{}

// Verifier est ce dont le middleware a besoin du service d'authentification.
type Verifier interface {
	VerifyAccessToken(token string) (auth.Claims, error)

	// AccountState confirme que le compte est toujours actif et rend son rôle
	// courant. Un jeton est autoporteur : sans cette vérification, désactiver
	// un compte ou le rétrograder n'aurait d'effet qu'à l'expiration du jeton.
	AccountState(ctx context.Context, userID uuid.UUID) (string, error)
}

// Authenticate exige un jeton d'accès valide, porté par l'en-tête
// Authorization.
//
// Les claims sont attachées au contexte : les handlers y accèdent par
// ClaimsFrom, sans reparser le jeton.
func Authenticate(v Verifier) func(http.Handler) http.Handler {
	return authenticate(v, false)
}

// AuthenticateAllowingQueryToken accepte en plus le jeton dans le paramètre
// `token`.
//
// Réservé aux routes qui ne peuvent pas porter d'en-tête : une image chargée
// par une balise <img>, et l'envoi de progression par sendBeacon à la fermeture
// d'un onglet.
//
// Cette tolérance n'est PAS le défaut, et c'est délibéré : un jeton dans une
// URL fuit par l'en-tête Referer, les journaux de proxy et l'historique du
// navigateur. On l'accepte là où il n'y a pas d'alternative, nulle part
// ailleurs.
func AuthenticateAllowingQueryToken(v Verifier) func(http.Handler) http.Handler {
	return authenticate(v, true)
}

func authenticate(v Verifier, allowQuery bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r, allowQuery)
			if !ok {
				problem.Write(w, r, problem.Unauthorized("missing bearer token"))
				return
			}

			claims, err := v.VerifyAccessToken(token)
			if err != nil {
				// L'expiration est distinguée de l'invalidité : c'est ce qui
				// permet au client de savoir s'il doit rafraîchir son jeton ou
				// redemander une connexion complète.
				if errors.Is(err, auth.ErrTokenExpired) {
					p := problem.Unauthorized("access token expired")
					p.Type = "https://boxincloud.dev/problems/token-expired"
					problem.Write(w, r, p)
					return
				}
				problem.Write(w, r, problem.Unauthorized("invalid access token"))
				return
			}

			// Le rôle porté par le jeton peut avoir vieilli : c'est celui de la
			// base qui fait foi, sans quoi un administrateur rétrogradé
			// resterait administrateur jusqu'à l'expiration.
			role, err := v.AccountState(r.Context(), claims.UserID)
			if err != nil {
				if errors.Is(err, auth.ErrAccountDisabled) {
					problem.Write(w, r, problem.Unauthorized("account disabled"))
					return
				}
				// Une panne de base n'est pas un défaut d'authentification.
				problem.Write(w, r, problem.ServiceUnavailable("account state unavailable"))
				return
			}
			claims.Role = role

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey{}, claims)))
		})
	}
}

// RequireAdmin refuse l'accès aux comptes non administrateurs.
//
// À composer après Authenticate.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFrom(r.Context())
		if !ok {
			problem.Write(w, r, problem.Unauthorized("authentication required"))
			return
		}
		if claims.Role != "admin" {
			problem.Write(w, r, problem.Forbidden("administrator role required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClaimsFrom récupère les claims attachées par Authenticate.
func ClaimsFrom(ctx context.Context) (auth.Claims, bool) {
	claims, ok := ctx.Value(claimsKey{}).(auth.Claims)
	return claims, ok
}

// bearerToken extrait le jeton de l'en-tête Authorization, et éventuellement du
// paramètre `token`.
func bearerToken(r *http.Request, allowQuery bool) (string, bool) {
	header := r.Header.Get("Authorization")
	if header != "" {
		const prefix = "Bearer "
		if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
			return header[len(prefix):], true
		}
		// En-tête présent mais malformé : c'est une erreur du client, pas une
		// occasion de chercher ailleurs.
		return "", false
	}

	if allowQuery {
		if token := r.URL.Query().Get("token"); token != "" {
			return token, true
		}
	}
	return "", false
}
