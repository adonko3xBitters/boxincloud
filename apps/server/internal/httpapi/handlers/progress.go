package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/catalog"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/middleware"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
	"github.com/adonko3xBitters/boxincloud/server/internal/progress"
)

// Progress expose la progression de lecture et la synchronisation.
type Progress struct {
	svc     *progress.Service
	catalog *catalog.Service
}

func NewProgress(svc *progress.Service, cat *catalog.Service) *Progress {
	return &Progress{svc: svc, catalog: cat}
}

// ─── Représentations ─────────────────────────────────────────────────────────

type progressDTO struct {
	ComicID    uuid.UUID  `json:"comicId"`
	Page       int32      `json:"page"`
	PageCount  int32      `json:"pageCount"`
	Percent    float64    `json:"percent"`
	Status     string     `json:"status"`
	ReadCount  int32      `json:"readCount"`
	Version    int64      `json:"version"`
	DeviceID   *uuid.UUID `json:"deviceId,omitempty"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

func toProgressDTO(p progress.Progress) progressDTO {
	return progressDTO{
		ComicID:    p.ComicID,
		Page:       p.Page,
		PageCount:  p.PageCount,
		Percent:    p.Percent(),
		Status:     string(p.Status),
		ReadCount:  p.ReadCount,
		Version:    p.Version,
		DeviceID:   p.DeviceID,
		StartedAt:  p.StartedAt,
		FinishedAt: p.FinishedAt,
		UpdatedAt:  p.UpdatedAt,
	}
}

func toProgressDTOs(items []progress.Progress) []progressDTO {
	out := make([]progressDTO, 0, len(items))
	for _, p := range items {
		out = append(out, toProgressDTO(p))
	}
	return out
}

// ─── Progression d'un album ──────────────────────────────────────────────────

func (h *Progress) Get(w http.ResponseWriter, r *http.Request) {
	claims, comicID, ok := h.authorized(w, r)
	if !ok {
		return
	}

	p, err := h.svc.Get(r.Context(), claims.UserID, comicID)
	if errors.Is(err, progress.ErrNotFound) {
		// Album jamais ouvert : une progression vide plutôt qu'un 404. Le
		// client affiche « non lu » sans avoir à traiter un cas d'erreur.
		writeJSON(w, http.StatusOK, progressDTO{
			ComicID: comicID,
			Status:  string(progress.StatusUnread),
		})
		return
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toProgressDTO(p))
}

type updateProgressRequest struct {
	Page      int32  `json:"page"`
	PageCount int32  `json:"pageCount"`
	Status    string `json:"status,omitempty"`
	DeviceID  string `json:"deviceId,omitempty"`
}

func (h *Progress) Update(w http.ResponseWriter, r *http.Request) {
	claims, comicID, ok := h.authorized(w, r)
	if !ok {
		return
	}

	var req updateProgressRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	u := progress.Update{
		ComicID:   comicID,
		Page:      req.Page,
		PageCount: req.PageCount,
		Status:    progress.Status(req.Status),
	}
	// Le jeton porte déjà l'appareil : le client n'a pas à le répéter, mais on
	// accepte qu'il le fasse.
	if claims.DeviceID != uuid.Nil {
		id := claims.DeviceID
		u.DeviceID = &id
	}
	if req.DeviceID != "" {
		if id, err := uuid.Parse(req.DeviceID); err == nil {
			u.DeviceID = &id
		}
	}

	p, err := h.svc.Record(r.Context(), claims.UserID, u)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toProgressDTO(p))
}

func (h *Progress) Delete(w http.ResponseWriter, r *http.Request) {
	claims, comicID, ok := h.authorized(w, r)
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), claims.UserID, comicID); err != nil {
		writeInternal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ContinueReading retourne les albums commencés mais non terminés.
func (h *Progress) ContinueReading(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFrom(r.Context())
	if !ok {
		problem.Write(w, r, problem.Unauthorized("authentication required"))
		return
	}

	items, err := h.svc.ContinueReading(r.Context(), claims.UserID, intParam(r, "limit", 20))
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": toProgressDTOs(items)})
}

// ─── Synchronisation ─────────────────────────────────────────────────────────

type syncPullResponse struct {
	Changes []progressDTO `json:"changes"`
	Cursor  string        `json:"cursor"`
	HasMore bool          `json:"hasMore"`
}

// SyncPull retourne les changements serveur depuis le curseur du client.
func (h *Progress) SyncPull(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFrom(r.Context())
	if !ok {
		problem.Write(w, r, problem.Unauthorized("authentication required"))
		return
	}

	res, err := h.svc.Pull(r.Context(), claims.UserID,
		r.URL.Query().Get("since"), intParam(r, "limit", 0))
	if errors.Is(err, progress.ErrInvalidCursor) {
		problem.Write(w, r, problem.Validation(map[string]string{
			"since": "must be an RFC 3339 timestamp returned by a previous sync",
		}))
		return
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, syncPullResponse{
		Changes: toProgressDTOs(res.Changes),
		Cursor:  res.Cursor,
		HasMore: res.HasMore,
	})
}

type syncPushRequest struct {
	Updates []updateItem `json:"updates"`
}

type updateItem struct {
	ComicID   string `json:"comicId"`
	Page      int32  `json:"page"`
	PageCount int32  `json:"pageCount"`
	Status    string `json:"status,omitempty"`
	DeviceID  string `json:"deviceId,omitempty"`
}

// maxSyncBatch borne un lot de synchronisation.
//
// Un client revenant d'une longue période hors ligne découpe son rattrapage
// plutôt que d'envoyer tout d'un coup : la mémoire du serveur ne dépend pas de
// la durée d'absence d'un utilisateur.
const maxSyncBatch = 500

// SyncPush applique un lot de progressions accumulées hors ligne.
//
// Chaque écriture passe par la même règle de résolution que les écritures en
// ligne : rejouer un lot deux fois — ce qui arrive quand la réponse se perd —
// ne peut pas faire régresser la position.
func (h *Progress) SyncPush(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFrom(r.Context())
	if !ok {
		problem.Write(w, r, problem.Unauthorized("authentication required"))
		return
	}

	var req syncPushRequest
	if !decodeJSONLarge(w, r, &req) {
		return
	}
	if len(req.Updates) > maxSyncBatch {
		problem.Write(w, r, problem.Validation(map[string]string{
			"updates": "batch too large — send at most 500 updates per request",
		}))
		return
	}

	updates := make([]progress.Update, 0, len(req.Updates))
	invalid := make(map[string]string)

	for i, item := range req.Updates {
		comicID, err := uuid.Parse(item.ComicID)
		if err != nil {
			invalid["updates["+itoa(i)+"].comicId"] = "invalid"
			continue
		}

		u := progress.Update{
			ComicID:   comicID,
			Page:      item.Page,
			PageCount: item.PageCount,
			Status:    progress.Status(item.Status),
		}
		if item.DeviceID != "" {
			if id, err := uuid.Parse(item.DeviceID); err == nil {
				u.DeviceID = &id
			}
		} else if claims.DeviceID != uuid.Nil {
			id := claims.DeviceID
			u.DeviceID = &id
		}
		updates = append(updates, u)
	}

	if len(invalid) > 0 {
		problem.Write(w, r, problem.Validation(invalid))
		return
	}

	applied, err := h.svc.Push(r.Context(), claims.UserID, updates)
	if err != nil {
		writeInternal(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"applied": toProgressDTOs(applied),
		// Le curseur permet au client d'enchaîner immédiatement sur un pull
		// sans risquer de recevoir ce qu'il vient lui-même d'envoyer.
		"cursor": syncCursorOf(applied),
	})
}

func syncCursorOf(applied []progress.Progress) string {
	if len(applied) == 0 {
		return ""
	}
	latest := applied[0].UpdatedAt
	for _, p := range applied[1:] {
		if p.UpdatedAt.After(latest) {
			latest = p.UpdatedAt
		}
	}
	return latest.UTC().Format(time.RFC3339Nano)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// authorized résout l'album et vérifie que l'utilisateur y a accès.
func (h *Progress) authorized(w http.ResponseWriter, r *http.Request) (claims authClaims, comicID uuid.UUID, ok bool) {
	c, found := middleware.ClaimsFrom(r.Context())
	if !found {
		problem.Write(w, r, problem.Unauthorized("authentication required"))
		return authClaims{}, uuid.Nil, false
	}

	id, err := uuid.Parse(chi.URLParam(r, "comicID"))
	if err != nil {
		problem.Write(w, r, problem.BadRequest("invalid comic id"))
		return authClaims{}, uuid.Nil, false
	}

	v := catalog.Viewer{UserID: c.UserID, IsAdmin: c.Role == "admin"}
	if _, err := h.catalog.GetComic(r.Context(), v, id); err != nil {
		writeCatalogError(w, r, err)
		return authClaims{}, uuid.Nil, false
	}

	return authClaims{UserID: c.UserID, DeviceID: c.DeviceID, Role: c.Role}, id, true
}

// authClaims reprend ce que les handlers de progression utilisent des claims.
type authClaims struct {
	UserID   uuid.UUID
	DeviceID uuid.UUID
	Role     string
}

// maxSyncBody : un lot de 500 progressions tient largement dedans.
const maxSyncBody = 2 << 20

func decodeJSONLarge(w http.ResponseWriter, r *http.Request, dst any) bool {
	return decodeJSONWithLimit(w, r, dst, maxSyncBody)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
