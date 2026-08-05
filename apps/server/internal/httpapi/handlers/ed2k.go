package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
)

/*
Ed2k expose le module eD2k/Kad.

Réservé à l'administration, en totalité. Le module engage la bande passante, les
ports et l'adresse IP de l'instance ; et à partir de l'étape 2, la table des
sources exposera des adresses IP de tiers. Les niveaux d'accès plus fins —
consultation, pilotage — arrivent à l'étape 6, quand il y aura quelque chose à
consulter.

Le protocole External Connections n'apparaît nulle part ici : ce handler ne
connaît que les types du domaine. Voir docs/adr/006-frontiere-ec-etanche.md.
*/
type Ed2k struct {
	svc *amule.Service
}

func NewEd2k(svc *amule.Service) *Ed2k {
	return &Ed2k{svc: svc}
}

// ─── Réponses ────────────────────────────────────────────────────────────────
//
// Écrites à la main plutôt que reprises du code généré : oapi-codegen produit
// des types de requête, et les handlers du projet composent leur réponse. Les
// noms de champs suivent le contrat, que le test de contrat vérifie.

type ed2kDaemonResponse struct {
	Host       string  `json:"host"`
	Port       int     `json:"port"`
	Label      string  `json:"label,omitempty"`
	LastState  string  `json:"lastState,omitempty"`
	LastDetail string  `json:"lastDetail,omitempty"`
	LastSeenAt *string `json:"lastSeenAt,omitempty"`
}

type ed2kStatusResponse struct {
	Enabled     bool                `json:"enabled"`
	State       string              `json:"state"`
	Detail      string              `json:"detail"`
	Daemon      *ed2kDaemonResponse `json:"daemon,omitempty"`
	IncomingDir string              `json:"incomingDir"`
}

func daemonResponse(d amule.Daemon) ed2kDaemonResponse {
	out := ed2kDaemonResponse{
		Host:       d.Host,
		Port:       d.Port,
		Label:      d.Label,
		LastState:  d.LastState,
		LastDetail: d.LastDetail,
	}
	if d.LastSeenAt != nil {
		seen := d.LastSeenAt.UTC().Format(time.RFC3339)
		out.LastSeenAt = &seen
	}
	return out
}

func statusResponse(s amule.Status) ed2kStatusResponse {
	out := ed2kStatusResponse{
		Enabled:     s.Enabled,
		State:       string(s.State),
		Detail:      s.Detail,
		IncomingDir: s.IncomingDir,
	}
	if s.Daemon != nil {
		daemon := daemonResponse(*s.Daemon)
		out.Daemon = &daemon
	}
	return out
}

// ─── Handlers ────────────────────────────────────────────────────────────────

/*
Status répond toujours, y compris module éteint ou démon absent.

« Pas de démon déclaré » est un état, et le premier qu'une instance fraîche
connaît. En faire une erreur obligerait l'interface à traiter le cas nominal
comme une panne.
*/
func (h *Ed2k) Status(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	writeJSON(w, http.StatusOK, statusResponse(h.svc.Status(r.Context())))
}

/*
Events tient un flux SSE ouvert.

Le premier message porte l'état complet : sans lui, une interface fraîchement
ouverte resterait vide jusqu'au premier changement, ce qui peut durer
indéfiniment sur une instance au repos.

Le jeton arrive en paramètre d'URL — EventSource ne sait pas porter d'en-tête —
et la route est donc placée dans le groupe déjà prévu pour ce cas.
*/
func (h *Ed2k) Events(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	h.svc.Hub().Serve(w, r, amule.EventStatus, statusResponse(h.svc.Status(r.Context())))
}

func (h *Ed2k) GetDaemon(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	daemon, err := h.svc.Daemon(r.Context())
	if err != nil {
		writeEd2kError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, daemonResponse(daemon))
}

func (h *Ed2k) SetDaemon(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var body struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Password string `json:"password"`
		Label    string `json:"label"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	daemon, err := h.svc.SetDaemon(r.Context(), amule.SetDaemonParams{
		Host:     body.Host,
		Port:     body.Port,
		Password: body.Password,
		Label:    body.Label,
	})
	if err != nil {
		writeEd2kError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, daemonResponse(daemon))
}

func (h *Ed2k) ForgetDaemon(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	if err := h.svc.ForgetDaemon(r.Context()); err != nil {
		writeEd2kError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

/*
writeEd2kError traduit les erreurs du module.

Le module désactivé répond 409 et non 404 : la route existe, elle est documentée
dans le contrat, et elle répondra dès que la configuration changera. Un 404
ferait conclure à une erreur de version.
*/
func writeEd2kError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid amule.ValidationError

	switch {
	case errors.Is(err, amule.ErrDisabled):
		problem.Write(w, r, problem.Conflict(
			"the eD2k module is disabled on this instance (BOXINCLOUD_ED2K_ENABLED)"))

	case errors.Is(err, amule.ErrNotConfigured):
		problem.Write(w, r, problem.NotFound("no aMule daemon has been declared"))

	case errors.As(err, &invalid):
		problem.Write(w, r, problem.Validation(invalid.Fields))

	default:
		writeInternal(w, r, err)
	}
}
