package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/catalog"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/middleware"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
)

// Catalog expose la consultation de la bibliothèque.
type Catalog struct {
	svc *catalog.Service
}

func NewCatalog(svc *catalog.Service) *Catalog { return &Catalog{svc: svc} }

// ─── Représentations ─────────────────────────────────────────────────────────

type libraryDTO struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	ComicCount int32     `json:"comicCount"`
}

type comicDTO struct {
	ID         uuid.UUID  `json:"id"`
	LibraryID  uuid.UUID  `json:"libraryId"`
	SeriesID   *uuid.UUID `json:"seriesId,omitempty"`
	SeriesName string     `json:"seriesName,omitempty"`
	Title      string     `json:"title"`
	Number     string     `json:"number,omitempty"`
	Volume     *int16     `json:"volume,omitempty"`
	Summary    string     `json:"summary,omitempty"`
	Format     string     `json:"format"`
	PageCount  int32      `json:"pageCount"`
	State      string     `json:"state"`
	AgeRating  *int16     `json:"ageRating,omitempty"`
	Language   string     `json:"language,omitempty"`
	FileSize   int64      `json:"fileSize"`
	ReleasedAt *string    `json:"releasedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`

	// Chemin de la couverture, relatif à l'API. Le client y ajoute la largeur
	// voulue plutôt que de composer l'URL lui-même.
	CoverPath string `json:"coverPath"`
}

func toComicDTO(c catalog.Comic) comicDTO {
	dto := comicDTO{
		ID:         c.ID,
		LibraryID:  c.LibraryID,
		SeriesID:   c.SeriesID,
		SeriesName: c.SeriesName,
		Title:      c.Title,
		Number:     c.Number,
		Volume:     c.Volume,
		Summary:    c.Summary,
		Format:     c.Format,
		PageCount:  c.PageCount,
		State:      c.State,
		AgeRating:  c.AgeRating,
		Language:   c.Language,
		FileSize:   c.FileSize,
		CreatedAt:  c.CreatedAt,
		CoverPath:  "/api/v1/comics/" + c.ID.String() + "/cover",
	}
	if c.ReleasedAt != nil {
		s := c.ReleasedAt.Format("2006-01-02")
		dto.ReleasedAt = &s
	}
	return dto
}

func toComicDTOs(comics []catalog.Comic) []comicDTO {
	out := make([]comicDTO, 0, len(comics))
	for _, c := range comics {
		out = append(out, toComicDTO(c))
	}
	return out
}

type seriesDTO struct {
	ID           uuid.UUID  `json:"id"`
	LibraryID    uuid.UUID  `json:"libraryId"`
	Name         string     `json:"name"`
	Description  string     `json:"description,omitempty"`
	Publisher    string     `json:"publisher,omitempty"`
	ComicCount   int32      `json:"comicCount"`
	CoverComicID *uuid.UUID `json:"coverComicId,omitempty"`
	CoverPath    string     `json:"coverPath,omitempty"`
}

func toSeriesDTO(s catalog.Series) seriesDTO {
	dto := seriesDTO{
		ID:           s.ID,
		LibraryID:    s.LibraryID,
		Name:         s.Name,
		Description:  s.Description,
		Publisher:    s.Publisher,
		ComicCount:   s.ComicCount,
		CoverComicID: s.CoverComicID,
	}
	if s.CoverComicID != nil {
		dto.CoverPath = "/api/v1/comics/" + s.CoverComicID.String() + "/cover"
	}
	return dto
}

func toSeriesDTOs(series []catalog.Series) []seriesDTO {
	out := make([]seriesDTO, 0, len(series))
	for _, s := range series {
		out = append(out, toSeriesDTO(s))
	}
	return out
}

// pageDTO enveloppe une page de résultats.
//
// nextCursor absent signifie « dernière page » — le client n'a pas à comparer
// le nombre d'éléments à la taille demandée pour le déduire.
type pageDTO[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// ─── Handlers ────────────────────────────────────────────────────────────────

func (h *Catalog) ListLibraries(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	libs, err := h.svc.ListLibraries(r.Context(), v)
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}

	out := make([]libraryDTO, 0, len(libs))
	for _, l := range libs {
		out = append(out, libraryDTO{ID: l.ID, Name: l.Name, Kind: l.Kind, ComicCount: l.ComicCount})
	}
	writeJSON(w, http.StatusOK, map[string]any{"libraries": out})
}

func (h *Catalog) ListComics(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	q := catalog.ListComicsQuery{
		State:  r.URL.Query().Get("state"),
		Cursor: r.URL.Query().Get("cursor"),
		Limit:  intParam(r, "limit", 0),
	}

	if id, ok := uuidParam(w, r, "libraryId"); !ok {
		return
	} else if id != nil {
		q.LibraryID = id
	}
	if id, ok := uuidParam(w, r, "seriesId"); !ok {
		return
	} else if id != nil {
		q.SeriesID = id
	}

	page, err := h.svc.ListComics(r.Context(), v, q)
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, pageDTO[comicDTO]{
		Items:      toComicDTOs(page.Items),
		NextCursor: page.NextCursor,
	})
}

func (h *Catalog) GetComic(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "comicID"))
	if err != nil {
		problem.Write(w, r, problem.BadRequest("invalid comic id"))
		return
	}

	comic, err := h.svc.GetComic(r.Context(), v, id)
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toComicDTO(comic))
}

func (h *Catalog) ListSeries(w http.ResponseWriter, r *http.Request) {
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

	page, err := h.svc.ListSeries(r.Context(), v, libraryID,
		r.URL.Query().Get("cursor"), intParam(r, "limit", 0))
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, pageDTO[seriesDTO]{
		Items:      toSeriesDTOs(page.Items),
		NextCursor: page.NextCursor,
	})
}

func (h *Catalog) GetSeries(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "seriesID"))
	if err != nil {
		problem.Write(w, r, problem.BadRequest("invalid series id"))
		return
	}

	series, comics, err := h.svc.GetSeries(r.Context(), v, id)
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"series": toSeriesDTO(series),
		"comics": toComicDTOs(comics),
	})
}

func (h *Catalog) Search(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	query := r.URL.Query().Get("q")

	var libraryID *uuid.UUID
	if id, ok := uuidParam(w, r, "libraryId"); !ok {
		return
	} else if id != nil {
		libraryID = id
	}

	res, err := h.svc.Search(r.Context(), v, query, libraryID, intParam(r, "limit", 0))
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"comics": toComicDTOs(res.Comics),
		"series": toSeriesDTOs(res.Series),
	})
}

func (h *Catalog) Home(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	home, err := h.svc.GetHome(r.Context(), v, intParam(r, "limit", 20))
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"recent":       toComicDTOs(home.Recent),
		"nextInSeries": toComicDTOs(home.NextInSeries),
	})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// viewerFrom construit le Viewer à partir des claims du jeton.
//
// MaxAgeRating n'y figure pas encore : la restriction par âge est portée par le
// compte, pas par le jeton, et la relire en base à chaque requête coûterait un
// aller-retour. Elle sera résolue par un cache utilisateur en M7, quand les
// profils restreints seront réellement exposés.
func viewerFrom(w http.ResponseWriter, r *http.Request) (catalog.Viewer, bool) {
	claims, ok := middleware.ClaimsFrom(r.Context())
	if !ok {
		problem.Write(w, r, problem.Unauthorized("authentication required"))
		return catalog.Viewer{}, false
	}
	return catalog.Viewer{
		UserID:  claims.UserID,
		IsAdmin: claims.Role == "admin",
	}, true
}

// uuidParam lit un paramètre de requête optionnel de type UUID.
//
// Trois cas distincts : absent (nil, true), valide (valeur, true), invalide
// (nil, false — la réponse d'erreur est déjà écrite).
func uuidParam(w http.ResponseWriter, r *http.Request, name string) (*uuid.UUID, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, true
	}

	id, err := uuid.Parse(raw)
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{name: "must be a valid UUID"}))
		return nil, false
	}
	return &id, true
}

func intParam(r *http.Request, name string, def int32) int32 {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return def
	}
	return int32(n)
}

func writeCatalogError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, catalog.ErrNotFound):
		problem.Write(w, r, problem.NotFound("resource not found"))
	case errors.Is(err, catalog.ErrForbidden):
		problem.Write(w, r, problem.Forbidden("you do not have access to this library"))
	case errors.Is(err, catalog.ErrBadCursor):
		problem.Write(w, r, problem.Validation(map[string]string{"cursor": "malformed"}))
	default:
		writeInternal(w, r, err)
	}
}
