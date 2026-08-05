package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
)

/*
Les routes qui AGISSENT sur le démon.

Toutes répondent 202 et un corps vide. Ce n'est pas de la paresse : amuled
n'accuse que RÉCEPTION, jamais l'effet. Rendre 200 avec un état laisserait
croire que la réponse décrit le résultat, alors qu'elle décrit au mieux
l'intention. Le 202 dit ce qui s'est réellement passé — la commande est partie,
l'état suivant fera foi.

Le client n'a pas à sonder pour autant : la commande réveille la scrutation, et
l'instantané suivant arrive dans la seconde.
*/

// accepted répond « transmis », sans corps.
func accepted(w http.ResponseWriter) {
	w.WriteHeader(http.StatusAccepted)
}

func (h *Ed2k) Act(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var body struct {
		Action string `json:"action"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	err := h.svc.ActOnDownload(r.Context(), chi.URLParam(r, "hash"),
		amule.DownloadAction(body.Action))
	if err != nil {
		writeEd2kCommandError(w, r, err, "action")
		return
	}
	accepted(w)
}

func (h *Ed2k) SetPriority(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var body struct {
		Priority string `json:"priority"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	err := h.svc.SetDownloadPriority(r.Context(), chi.URLParam(r, "hash"),
		amule.Priority(body.Priority))
	if err != nil {
		writeEd2kCommandError(w, r, err, "priority")
		return
	}
	accepted(w)
}

func (h *Ed2k) AddLink(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var body struct {
		Link string `json:"link"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	if err := h.svc.AddLink(r.Context(), body.Link); err != nil {
		writeEd2kCommandError(w, r, err, "link")
		return
	}
	accepted(w)
}

func (h *Ed2k) ConnectServer(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	// Corps facultatif : sans adresse, le démon choisit son serveur. C'est le
	// geste courant, et l'imposer obligerait l'interface à envoyer un objet
	// vide pour dire « peu importe ».
	var body struct {
		IP   string `json:"ip"`
		Port int    `json:"port"`
	}
	if r.ContentLength > 0 && !decodeJSON(w, r, &body) {
		return
	}

	if err := h.svc.ConnectServer(r.Context(), body.IP, body.Port); err != nil {
		writeEd2kCommandError(w, r, err, "server")
		return
	}
	accepted(w)
}

func (h *Ed2k) DisconnectServer(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	if err := h.svc.DisconnectServer(r.Context()); err != nil {
		writeEd2kCommandError(w, r, err, "server")
		return
	}
	accepted(w)
}

func (h *Ed2k) SetKad(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var body struct {
		Running bool `json:"running"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	var err error
	if body.Running {
		err = h.svc.StartKad(r.Context())
	} else {
		err = h.svc.StopKad(r.Context())
	}
	if err != nil {
		writeEd2kCommandError(w, r, err, "kad")
		return
	}
	accepted(w)
}

func (h *Ed2k) ReloadShared(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	if err := h.svc.ReloadSharedFiles(r.Context()); err != nil {
		writeEd2kCommandError(w, r, err, "shared")
		return
	}
	accepted(w)
}

/*
writeEd2kCommandError traduit l'échec d'une commande.

Les refus de SAISIE — empreinte malformée, lien d'un autre protocole — désignent
le champ fautif : sur un formulaire, un message global fait chercher.

Le reste passe par le traducteur commun. Un refus du DÉMON y arrive comme une
erreur ordinaire, et son explication est conservée : c'est elle qui dira d'aller
regarder amule.conf plutôt que le réseau.
*/
func writeEd2kCommandError(w http.ResponseWriter, r *http.Request, err error, field string) {
	switch {
	case errors.Is(err, amule.ErrInvalidHash):
		problem.Write(w, r, problem.Validation(map[string]string{
			"hash": "not a valid eD2k hash (16 bytes, hexadecimal)",
		}))

	case errors.Is(err, amule.ErrInvalidLink):
		problem.Write(w, r, problem.Validation(map[string]string{
			field: err.Error(),
		}))

	default:
		writeEd2kError(w, r, err)
	}
}
