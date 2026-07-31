// Package problem produit les réponses d'erreur de l'API au format
// RFC 9457 (Problem Details for HTTP APIs).
//
// Un format d'erreur unique et documenté dans l'OpenAPI permet aux clients web
// et Flutter de partager leur traitement des erreurs, au lieu de chacun
// déchiffrer une forme différente selon l'endpoint.
package problem

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// ContentType est le type MIME défini par la RFC 9457.
const ContentType = "application/problem+json"

// Problem décrit une erreur de manière exploitable par un client.
type Problem struct {
	// Type identifie la catégorie d'erreur. Stable : les clients peuvent
	// s'appuyer dessus pour brancher leur comportement.
	Type string `json:"type"`

	// Title résume l'erreur, en anglais et lisible par un développeur.
	Title string `json:"title"`

	// Status reprend le code HTTP.
	Status int `json:"status"`

	// Detail explique ce cas précis. Peut être affiché à l'utilisateur.
	Detail string `json:"detail,omitempty"`

	// Instance porte l'identifiant de requête, pour rapprocher un rapport
	// d'utilisateur des logs serveur.
	Instance string `json:"instance,omitempty"`

	// Errors détaille les champs invalides, pour les erreurs de validation.
	Errors map[string]string `json:"errors,omitempty"`
}

func (p Problem) Error() string { return p.Title }

// Write sérialise le problème dans la réponse.
func Write(w http.ResponseWriter, r *http.Request, p Problem) {
	if p.Instance == "" {
		p.Instance = middleware.GetReqID(r.Context())
	}

	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// ─── Constructeurs ───────────────────────────────────────────────────────────

func Internal() Problem {
	return Problem{
		Type:   "about:blank",
		Title:  "Internal Server Error",
		Status: http.StatusInternalServerError,
		// Volontairement vague : le détail part dans les logs, pas au client.
		Detail: "An unexpected error occurred. If it persists, please report it with the instance identifier.",
	}
}

func NotFound(detail string) Problem {
	return Problem{
		Type:   "https://boxincloud.dev/problems/not-found",
		Title:  "Not Found",
		Status: http.StatusNotFound,
		Detail: detail,
	}
}

func BadRequest(detail string) Problem {
	return Problem{
		Type:   "https://boxincloud.dev/problems/bad-request",
		Title:  "Bad Request",
		Status: http.StatusBadRequest,
		Detail: detail,
	}
}

func Validation(errs map[string]string) Problem {
	return Problem{
		Type:   "https://boxincloud.dev/problems/validation",
		Title:  "Validation Failed",
		Status: http.StatusUnprocessableEntity,
		Detail: "One or more fields are invalid.",
		Errors: errs,
	}
}

func Unauthorized(detail string) Problem {
	return Problem{
		Type:   "https://boxincloud.dev/problems/unauthorized",
		Title:  "Unauthorized",
		Status: http.StatusUnauthorized,
		Detail: detail,
	}
}

func Forbidden(detail string) Problem {
	return Problem{
		Type:   "https://boxincloud.dev/problems/forbidden",
		Title:  "Forbidden",
		Status: http.StatusForbidden,
		Detail: detail,
	}
}

// TooManyRequests signale une limitation de débit.
//
// La réponse porte un `Retry-After` : un client qui sait quand réessayer n'a
// pas besoin de tâtonner, et tâtonner est précisément ce qui produit la charge
// que la limitation cherche à éviter.
func TooManyRequests(detail string) Problem {
	return Problem{
		Type:   "https://boxincloud.dev/problems/too-many-requests",
		Title:  "Too Many Requests",
		Status: http.StatusTooManyRequests,
		Detail: detail,
	}
}

func ServiceUnavailable(detail string) Problem {
	return Problem{
		Type:   "https://boxincloud.dev/problems/service-unavailable",
		Title:  "Service Unavailable",
		Status: http.StatusServiceUnavailable,
		Detail: detail,
	}
}
