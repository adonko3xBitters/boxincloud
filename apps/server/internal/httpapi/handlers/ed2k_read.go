package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/gen"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
)

/*
Les routes de lecture du module eD2k.

Elles servent TOUTES le même instantané, celui que la scrutation tient à jour.
Aucune ne parle au démon : une requête HTTP ne doit pas déclencher un
aller-retour EC, sans quoi dix onglets ouverts sur la page des serveurs
suffiraient à décupler la charge sur le démon.

L'exception assumée est la liste des sources d'un fichier, qui ne figure pas
dans l'instantané — voir Service.Sources et son commentaire.

# Les types de réponse sont ceux du contrat

Les structures rendues ici viennent du code ENGENDRÉ depuis api/openapi.yaml.
Écrire des structures à la main ferait diverger la réponse du contrat au premier
renommage de champ, et le test de contrat ne le verrait que sur les routes qu'il
appelle. Là, une divergence casse la compilation.
*/

// snapshotOrProblem rend l'instantané courant, ou écrit la réponse d'erreur.
//
// Un instantané nil n'est PAS une erreur de serveur : il signifie que la
// scrutation n'a encore rien collecté, parce que personne n'avait ouvert
// l'interface. On répond 503 avec une explication — l'état arrive dès qu'un
// client s'abonne au flux — plutôt qu'une file vide, qui serait un mensonge.
func (h *Ed2k) snapshotOrProblem(w http.ResponseWriter, r *http.Request) (*amule.Snapshot, bool) {
	if !h.svc.Enabled() {
		problem.Write(w, r, problem.Conflict(
			"the eD2k module is disabled on this instance (BOXINCLOUD_ED2K_ENABLED)"))
		return nil, false
	}

	snapshot := h.svc.Snapshot()
	if snapshot == nil {
		p := problem.ServiceUnavailable(
			"no snapshot yet: the daemon is polled only while a client watches the event stream")
		problem.Write(w, r, p)
		return nil, false
	}
	return snapshot, true
}

func (h *Ed2k) Snapshot(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	snapshot, ok := h.snapshotOrProblem(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, gen.Ed2kSnapshot{
		TakenAt:     snapshot.TakenAt,
		Connection:  apiConnection(snapshot.Connection),
		Stats:       apiStats(snapshot.Stats),
		Downloads:   mapSlice(snapshot.Downloads, apiDownload),
		Uploads:     mapSlice(snapshot.Uploads, apiUpload),
		QueuedPeers: mapSlice(snapshot.QueuedPeers, apiQueuedPeer),
		Servers:     mapSlice(snapshot.Servers, apiServer),
		SharedFiles: mapSlice(snapshot.SharedFiles, apiSharedFile),
	})
}

func (h *Ed2k) Downloads(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	snapshot, ok := h.snapshotOrProblem(w, r)
	if !ok {
		return
	}

	// La date de l'instantané accompagne chaque liste partielle : sans elle,
	// une interface qui compose plusieurs routes ne peut pas savoir si elle
	// mélange deux états successifs.
	writeJSON(w, http.StatusOK, map[string]any{
		"takenAt":   snapshot.TakenAt,
		"downloads": mapSlice(snapshot.Downloads, apiDownload),
	})
}

/*
Sources est la seule route de lecture qui parle au démon.

Les sources ne sont pas dans l'instantané : les collecter coûterait une requête
par fichier à chaque tour de scrutation, pour une information que l'interface
n'affiche que sur le fichier ouvert. Le coût est donc payé par un geste
explicite, pas par la boucle de fond.
*/
func (h *Ed2k) Sources(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	hash := chi.URLParam(r, "hash")
	sources, err := h.svc.Sources(r.Context(), hash)
	if err != nil {
		writeEd2kError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"takenAt": nowUTC(),
		"sources": mapSlice(sources, apiSource),
	})
}

func (h *Ed2k) Uploads(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	snapshot, ok := h.snapshotOrProblem(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"takenAt":     snapshot.TakenAt,
		"uploads":     mapSlice(snapshot.Uploads, apiUpload),
		"queuedPeers": mapSlice(snapshot.QueuedPeers, apiQueuedPeer),
	})
}

func (h *Ed2k) Servers(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	snapshot, ok := h.snapshotOrProblem(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"takenAt":    snapshot.TakenAt,
		"servers":    mapSlice(snapshot.Servers, apiServer),
		"connection": apiConnection(snapshot.Connection),
	})
}

func (h *Ed2k) SharedFiles(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	snapshot, ok := h.snapshotOrProblem(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"takenAt": snapshot.TakenAt,
		"files":   mapSlice(snapshot.SharedFiles, apiSharedFile),
	})
}

func (h *Ed2k) Stats(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	snapshot, ok := h.snapshotOrProblem(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"takenAt":    snapshot.TakenAt,
		"stats":      apiStats(snapshot.Stats),
		"connection": apiConnection(snapshot.Connection),
	})
}
