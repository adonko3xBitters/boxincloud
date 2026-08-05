package handlers

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
)

/*
Le pont vers la bibliothèque, et le journal du démon.

Les destinations sont le seul réglage du module qui change ce que boxincloud
FAIT du contenu, plutôt que la façon dont il l'affiche. C'est aussi la seule
partie que quelqu'un règle une fois et oublie — d'où le soin porté aux messages
d'erreur : personne ne se souviendra du format six mois plus tard.
*/

func (h *Ed2k) Destinations(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	destinations, err := h.svc.Destinations(r.Context())
	if err != nil {
		writeEd2kError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"destinations": mapSlice(destinations, apiDestination),
	})
}

func (h *Ed2k) SetDestination(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var body struct {
		Category  int     `json:"category"`
		Label     string  `json:"label"`
		LibraryID *string `json:"libraryId"`
		Folder    string  `json:"folder"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	destination := amule.Destination{
		Category: body.Category,
		Label:    body.Label,
		Folder:   body.Folder,
	}

	// Une bibliothèque absente rétablit le défaut ; une bibliothèque mal formée
	// est une erreur de saisie, et les confondre ferait taire une faute de
	// frappe en la traitant comme un choix.
	if body.LibraryID != nil && *body.LibraryID != "" {
		id, err := uuid.Parse(*body.LibraryID)
		if err != nil {
			problem.Write(w, r, problem.Validation(map[string]string{
				"libraryId": "not a valid identifier",
			}))
			return
		}
		destination.LibraryID = &id
	}

	saved, err := h.svc.SetDestination(r.Context(), destination)
	if err != nil {
		writeEd2kError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, apiDestination(saved))
}

func (h *Ed2k) Publications(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, ok := parsePositive(raw); ok {
			limit = parsed
		}
	}

	publications, err := h.svc.Publications(r.Context(), limit)
	if err != nil {
		writeEd2kError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"publications": mapSlice(publications, apiPublication),
	})
}

func (h *Ed2k) Logs(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	lines, err := h.svc.Logs(r.Context())
	if err != nil {
		writeEd2kError(w, r, err)
		return
	}

	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, line.Text)
	}
	writeJSON(w, http.StatusOK, map[string]any{"lines": out})
}

func (h *Ed2k) ClearLogs(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	if err := h.svc.ClearLogs(r.Context()); err != nil {
		writeEd2kError(w, r, err)
		return
	}
	accepted(w)
}

// ─── Traduction ──────────────────────────────────────────────────────────────

func apiDestination(d amule.Destination) map[string]any {
	out := map[string]any{
		"category": d.Category,
		"label":    d.Label,
		"folder":   d.Folder,
	}
	if d.LibraryID != nil {
		out["libraryId"] = d.LibraryID.String()
	}
	return out
}

func apiPublication(p amule.Publication) map[string]any {
	out := map[string]any{
		"hash":     p.Hash,
		"name":     p.Name,
		"size":     p.Size,
		"category": p.Category,
		"status":   string(p.Status),
	}
	if p.Detail != "" {
		out["detail"] = p.Detail
	}
	if p.LibraryID != nil {
		out["libraryId"] = p.LibraryID.String()
	}
	if p.ComicID != nil {
		out["comicId"] = p.ComicID.String()
	}
	return out
}

// parsePositive lit un entier strictement positif, sans se plaindre.
//
// Une limite illisible retombe sur le défaut plutôt que de faire échouer la
// requête : c'est un confort d'affichage, pas un paramètre dont dépend la
// justesse de la réponse.
func parsePositive(raw string) (int, bool) {
	value := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, false
		}
		value = value*10 + int(r-'0')
		if value > 500 {
			return 500, true
		}
	}
	return value, value > 0
}
