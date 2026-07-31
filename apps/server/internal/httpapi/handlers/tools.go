package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/catalog"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
	"github.com/adonko3xBitters/boxincloud/server/internal/library"
)

// Tools expose les actions de gestion de bibliothèque.
type Tools struct {
	svc *catalog.Tools
}

func NewTools(svc *catalog.Tools) *Tools { return &Tools{svc: svc} }

// ─── Dossiers ────────────────────────────────────────────────────────────────

type folderDTO struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	Depth      int    `json:"depth"`
	ComicCount int    `json:"comicCount"`
}

// ListFolders retourne l'arborescence, à plat.
//
// Les nœuds sont triés de façon qu'un parent précède toujours ses enfants : le
// client bâtit l'arbre en une passe, sans tri ni récursion.
func (h *Tools) ListFolders(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	var libraryID *uuid.UUID
	if id, ok := uuidParam(w, r, "libraryId"); !ok {
		return
	} else if id != nil {
		libraryID = id
	}

	counts, err := h.svc.FolderCounts(r.Context(), v, libraryID)
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}

	folders := library.BuildFolderTree(counts)
	out := make([]folderDTO, 0, len(folders))
	for _, f := range folders {
		out = append(out, folderDTO{
			Path: f.Path, Name: f.Name, Depth: f.Depth, ComicCount: f.ComicCount,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"folders": out})
}

// ─── Favoris et notes ────────────────────────────────────────────────────────

// UserMarks retourne favoris et notes en une requête.
//
// Groupés délibérément : ce sont des annotations qu'une grille affiche
// ensemble, et deux allers-retours pour peupler la même vue seraient du gâchis.
func (h *Tools) UserMarks(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	favorites, err := h.svc.Favorites(r.Context(), v)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	ratings, err := h.svc.Ratings(r.Context(), v)
	if err != nil {
		writeInternal(w, r, err)
		return
	}

	ids := make([]string, 0, len(favorites))
	for _, id := range favorites {
		ids = append(ids, id.String())
	}
	byComic := make(map[string]int16, len(ratings))
	for id, rating := range ratings {
		byComic[id.String()] = rating
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"favorites": ids,
		"ratings":   byComic,
	})
}

type favoriteRequest struct {
	Favorite bool `json:"favorite"`
}

func (h *Tools) SetFavorite(w http.ResponseWriter, r *http.Request) {
	v, comicID, ok := h.target(w, r)
	if !ok {
		return
	}

	var req favoriteRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := h.svc.SetFavorite(r.Context(), v, comicID, req.Favorite); err != nil {
		writeCatalogError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"favorite": req.Favorite})
}

type ratingRequest struct {
	// Rating de 1 à 5. Zéro retire la note.
	Rating int16 `json:"rating"`
}

func (h *Tools) SetRating(w http.ResponseWriter, r *http.Request) {
	v, comicID, ok := h.target(w, r)
	if !ok {
		return
	}

	var req ratingRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Rating < 0 || req.Rating > 5 {
		problem.Write(w, r, problem.Validation(map[string]string{
			"rating": "must be between 0 and 5 (0 clears the rating)",
		}))
		return
	}

	if err := h.svc.SetRating(r.Context(), v, comicID, req.Rating); err != nil {
		writeCatalogError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int16{"rating": req.Rating})
}

// ─── Édition ─────────────────────────────────────────────────────────────────

type editRequest struct {
	Title    *string `json:"title,omitempty"`
	Number   *string `json:"number,omitempty"`
	Summary  *string `json:"summary,omitempty"`
	Language *string `json:"language,omitempty"`
}

// EditComic applique une correction manuelle.
//
// Les champs édités sont verrouillés : une réindexation ne les écrasera plus.
func (h *Tools) EditComic(w http.ResponseWriter, r *http.Request) {
	v, comicID, ok := h.target(w, r)
	if !ok {
		return
	}

	var req editRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	comic, err := h.svc.EditComic(r.Context(), v, comicID, catalog.ComicEdit{
		Title:    req.Title,
		Number:   req.Number,
		Summary:  req.Summary,
		Language: req.Language,
	})
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toComicDTO(comic))
}

// ─── Actions en lot ──────────────────────────────────────────────────────────

type bulkRequest struct {
	Action string   `json:"action"`
	IDs    []string `json:"ids"`
}

func (h *Tools) Bulk(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	var req bulkRequest
	if !decodeJSONLarge(w, r, &req) {
		return
	}

	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, raw := range req.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			problem.Write(w, r, problem.Validation(map[string]string{
				"ids": "invalid",
			}))
			return
		}
		ids = append(ids, id)
	}

	affected, err := h.svc.Bulk(r.Context(), v, catalog.BulkAction(req.Action), ids)
	switch {
	case errors.Is(err, catalog.ErrTooManyItems):
		problem.Write(w, r, problem.Validation(map[string]string{
			"ids": "at most 1000 items per request",
		}))
		return
	case errors.Is(err, catalog.ErrUnknownAction):
		problem.Write(w, r, problem.Validation(map[string]string{
			"action": "one-of",
		}))
		return
	case err != nil:
		writeCatalogError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]int64{"affected": affected})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (h *Tools) target(w http.ResponseWriter, r *http.Request) (catalog.Viewer, uuid.UUID, bool) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return catalog.Viewer{}, uuid.Nil, false
	}

	id, err := uuid.Parse(chi.URLParam(r, "comicID"))
	if err != nil {
		problem.Write(w, r, problem.BadRequest("invalid comic id"))
		return catalog.Viewer{}, uuid.Nil, false
	}
	return v, id, true
}
