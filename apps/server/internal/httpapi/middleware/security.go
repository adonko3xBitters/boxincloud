package middleware

import (
	"net/http"
	"strings"
)

/*
En-têtes de sécurité.

Une instance auto-hébergée est souvent exposée sans reverse-proxy configuré
finement : ces en-têtes sont donc la seule défense en place, et pas une couche
de plus au-dessus d'une autre.

La politique de contenu mérite un mot. `script-src 'self'` sans `unsafe-inline`
serait plus strict, mais Next.js en export statique injecte un script en ligne
pour son amorçage : l'interdire produirait une page blanche. On garde donc
`'unsafe-inline'` pour les scripts, et on ferme ce qui compte vraiment ici —
`object-src 'none'` (pas de Flash ni de plugin), `base-uri 'self'` (impossible
de détourner les URL relatives), `frame-ancestors 'none'` (pas d'inclusion dans
un cadre tiers, donc pas de clickjacking).

`connect-src` autorise n'importe quelle origine : une instance peut être servie
sous un nom et pointer son API ailleurs, et la restreindre casserait ce montage
sans rien empêcher qu'un attaquant capable d'injecter du script ne puisse déjà
faire autrement.
*/

const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' 'unsafe-inline'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; " +
	"font-src 'self' data:; " +
	"connect-src *; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

/*
SecurityHeaders pose les en-têtes de défense sur toutes les réponses.

`Strict-Transport-Security` n'y est PAS, volontairement. Beaucoup d'instances
auto-hébergées tournent en HTTP simple sur un réseau local ; poser un HSTS
depuis une réponse HTTP est sans effet, mais le poser depuis une instance
accessible en HTTPS puis en HTTP verrouillerait le navigateur sur une adresse
devenue injoignable — une panne dont on ne sort qu'en purgeant l'état du
navigateur. C'est au reverse-proxy, qui sait s'il termine TLS, de le décider.
*/
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")

		// Aucune de ces fonctions n'est utilisée : les refuser d'avance évite
		// qu'une dépendance future les demande sans qu'on s'en aperçoive.
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")

		// La politique de contenu ne s'applique qu'aux documents. La poser sur
		// une image ou une réponse JSON ne protégerait rien et alourdirait
		// chaque réponse d'un en-tête de trois cents octets — sur une grille de
		// soixante vignettes, cela finit par se voir.
		if isDocument(r) {
			h.Set("Content-Security-Policy", contentSecurityPolicy)
		}

		next.ServeHTTP(w, r)
	})
}

// isDocument devine si la réponse sera une page.
//
// Sur l'en-tête `Accept` plutôt que sur le chemin : l'application web est
// servie depuis la racine et depuis des routes quelconques, et énumérer ces
// dernières obligerait à tenir la liste à jour à chaque page ajoutée.
func isDocument(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}
