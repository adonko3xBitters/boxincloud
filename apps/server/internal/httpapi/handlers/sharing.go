package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/catalog"
	"github.com/adonko3xBitters/boxincloud/server/internal/folders"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
)

/*
Sharing expose le partage entre comptes et les liens publics.

Les liens publics sont la SEULE porte de boxincloud qui ne demande pas de
compte. Tout ce qui les concerne est donc regroupé ici plutôt que dispersé, pour
que la surface non authentifiée se lise d'un seul tenant.
*/
type Sharing struct {
	folders *folders.Service
	catalog *catalog.Service

	// reader est le handler, pas le service : le partage réutilise ses méthodes
	// de service d'album — mêmes en-têtes de cache, mêmes requêtes
	// conditionnelles — en n'y apportant que sa propre autorisation.
	reader *Reader
}

func NewSharing(f *folders.Service, c *catalog.Service, rd *Reader) *Sharing {
	return &Sharing{folders: f, catalog: c, reader: rd}
}

// ─── Partage entre comptes ───────────────────────────────────────────────────

type folderGrantDTO struct {
	UserID      string `json:"userId"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
	CanWrite    bool   `json:"canWrite"`
}

// ListFolderGrants retourne les comptes autorisés sur un dossier.
func (h *Sharing) ListFolderGrants(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}
	if !v.IsAdmin {
		problem.Write(w, r, problem.Forbidden("administrator role required"))
		return
	}

	libraryID, err := uuid.Parse(chi.URLParam(r, "libraryID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"libraryId": "must be a UUID"}))
		return
	}

	grants, err := h.folders.FolderGrants(r.Context(), libraryID, r.URL.Query().Get("path"))
	if err != nil {
		writeFolderError(w, r, err)
		return
	}

	out := make([]folderGrantDTO, 0, len(grants))
	for _, g := range grants {
		out = append(out, folderGrantDTO{
			UserID:      g.UserID.String(),
			Username:    g.Username,
			DisplayName: g.DisplayName,
			CanWrite:    g.CanWrite,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"grants": out})
}

type folderGrantRequest struct {
	LibraryID string `json:"libraryId"`
	Path      string `json:"path"`
	UserID    string `json:"userId"`
	CanWrite  bool   `json:"canWrite"`
}

/*
GrantFolder ouvre un dossier à un compte.

Le modèle est celui des bibliothèques, et l'interface doit le relayer : un
dossier sans autorisation explicite est visible de tous ceux qui voient la
bibliothèque. Le PREMIER accès accordé le referme pour les autres — le geste
restreint autant qu'il ouvre.
*/
func (h *Sharing) GrantFolder(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}
	if !v.IsAdmin {
		problem.Write(w, r, problem.Forbidden("administrator role required"))
		return
	}

	var req folderGrantRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	libraryID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"libraryId": "must be a UUID"}))
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"userId": "must be a UUID"}))
		return
	}

	if err := h.folders.GrantFolder(r.Context(), libraryID, req.Path, userID, req.CanWrite); err != nil {
		writeFolderError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, folderGrantDTO{UserID: userID.String(), CanWrite: req.CanWrite})
}

// RevokeFolderGrant retire l'accès d'un compte à un dossier.
func (h *Sharing) RevokeFolderGrant(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}
	if !v.IsAdmin {
		problem.Write(w, r, problem.Forbidden("administrator role required"))
		return
	}

	libraryID, err := uuid.Parse(chi.URLParam(r, "libraryID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"libraryId": "must be a UUID"}))
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"userId": "must be a UUID"}))
		return
	}

	if err := h.folders.RevokeFolder(r.Context(), libraryID, r.URL.Query().Get("path"), userID); err != nil {
		writeFolderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Liens publics ───────────────────────────────────────────────────────────

type shareLinkDTO struct {
	ID         string  `json:"id"`
	LibraryID  string  `json:"libraryId"`
	FolderPath *string `json:"folderPath,omitempty"`
	ComicID    *string `json:"comicId,omitempty"`
	Label      string  `json:"label,omitempty"`
	ExpiresAt  string  `json:"expiresAt"`
	CreatedAt  string  `json:"createdAt"`
	LastUsedAt *string `json:"lastUsedAt,omitempty"`
	UseCount   int64   `json:"useCount"`

	// Token n'est renseigné qu'à la création : seul son hachage est conservé.
	Token string `json:"token,omitempty"`
}

func toShareDTO(link folders.ShareLink) shareLinkDTO {
	dto := shareLinkDTO{
		ID:         link.ID.String(),
		LibraryID:  link.LibraryID.String(),
		FolderPath: link.FolderPath,
		Label:      link.Label,
		ExpiresAt:  link.ExpiresAt.UTC().Format(time.RFC3339),
		CreatedAt:  link.CreatedAt.UTC().Format(time.RFC3339),
		UseCount:   link.UseCount,
		Token:      link.Token,
	}
	if link.ComicID != nil {
		id := link.ComicID.String()
		dto.ComicID = &id
	}
	if link.LastUsedAt != nil {
		used := link.LastUsedAt.UTC().Format(time.RFC3339)
		dto.LastUsedAt = &used
	}
	return dto
}

type createShareRequest struct {
	LibraryID  string  `json:"libraryId"`
	FolderPath *string `json:"folderPath"`
	ComicID    *string `json:"comicId"`
	Label      string  `json:"label"`
	ExpiresAt  string  `json:"expiresAt"`
}

/*
CreateShare produit un lien public.

Le jeton n'est retourné qu'ici : seul son hachage est conservé, comme un mot de
passe. Perdre le lien oblige à en créer un autre — un lien relisible en base est
un lien qu'une fuite de base livre en clair.
*/
func (h *Sharing) CreateShare(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}
	if !v.IsAdmin {
		problem.Write(w, r, problem.Forbidden("administrator role required"))
		return
	}

	var req createShareRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	libraryID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"libraryId": "must be a UUID"}))
		return
	}

	expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{
			"expiresAt": "must be an RFC 3339 timestamp",
		}))
		return
	}

	params := folders.ShareParams{
		LibraryID:  libraryID,
		FolderPath: req.FolderPath,
		Label:      req.Label,
		CreatedBy:  v.UserID,
		ExpiresAt:  expiresAt,
	}
	if req.ComicID != nil {
		comicID, err := uuid.Parse(*req.ComicID)
		if err != nil {
			problem.Write(w, r, problem.Validation(map[string]string{"comicId": "must be a UUID"}))
			return
		}
		params.ComicID = &comicID
	}

	link, err := h.folders.CreateShare(r.Context(), params)
	if err != nil {
		writeShareError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toShareDTO(link))
}

// ListShares retourne les liens actifs des bibliothèques visibles.
func (h *Sharing) ListShares(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}
	if !v.IsAdmin {
		problem.Write(w, r, problem.Forbidden("administrator role required"))
		return
	}

	libs, err := h.catalog.ListLibraries(r.Context(), v)
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}

	ids := make([]uuid.UUID, 0, len(libs))
	for _, lib := range libs {
		ids = append(ids, lib.ID)
	}

	links, err := h.folders.ListShares(r.Context(), ids)
	if err != nil {
		writeInternal(w, r, err)
		return
	}

	out := make([]shareLinkDTO, 0, len(links))
	for _, link := range links {
		out = append(out, toShareDTO(link))
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": out})
}

// RevokeShare ferme un lien immédiatement.
func (h *Sharing) RevokeShare(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}
	if !v.IsAdmin {
		problem.Write(w, r, problem.Forbidden("administrator role required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "shareID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"shareId": "must be a UUID"}))
		return
	}

	if err := h.folders.RevokeShare(r.Context(), id); err != nil {
		writeShareError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Accès public ────────────────────────────────────────────────────────────

/*
Ces trois routes sont les seules du serveur à ne demander aucun compte.

Elles ne servent QUE ce que le lien désigne. Aucune ne prend d'identifiant de
bibliothèque, aucune ne liste autre chose que la portée du lien, et l'
appartenance d'un album à cette portée est revérifiée à chaque requête : un lien
de dossier donne accès à ce que le dossier contient MAINTENANT, si bien qu'un
album qui en sort cesse d'être accessible sans qu'il faille penser à révoquer.
*/

type sharedComicDTO struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	SeriesName string `json:"seriesName,omitempty"`
	Number     string `json:"number,omitempty"`
	PageCount  int32  `json:"pageCount"`
	CoverPath  string `json:"coverPath"`
}

// GetShared retourne ce que le lien donne à voir.
func (h *Sharing) GetShared(w http.ResponseWriter, r *http.Request) {
	link, ok := h.resolve(w, r)
	if !ok {
		return
	}

	ids, err := h.folders.SharedComics(r.Context(), link)
	if err != nil {
		writeShareError(w, r, err)
		return
	}

	// La lecture passe par le catalogue en viewer administrateur : le contrôle
	// d'accès a déjà été fait par le lien, et le refaire par compte n'aurait pas
	// de sens — il n'y a pas de compte.
	admin := catalog.Viewer{IsAdmin: true}

	items := make([]sharedComicDTO, 0, len(ids))
	for _, id := range ids {
		comic, err := h.catalog.GetComic(r.Context(), admin, id)
		if err != nil {
			continue
		}
		items = append(items, sharedComicDTO{
			ID:         comic.ID.String(),
			Title:      comic.Title,
			SeriesName: comic.SeriesName,
			Number:     comic.Number,
			PageCount:  comic.PageCount,
			CoverPath:  "/api/v1/share/" + shareToken(r) + "/comics/" + comic.ID.String() + "/cover",
		})
	}

	scope := "comic"
	if link.FolderPath != nil {
		scope = "folder"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"scope":     scope,
		"label":     link.Label,
		"expiresAt": link.ExpiresAt.UTC().Format(time.RFC3339),
		"comics":    items,
	})
}

// SharedManifest retourne le manifeste d'un album partagé.
func (h *Sharing) SharedManifest(w http.ResponseWriter, r *http.Request) {
	comicID, ok := h.resolveComic(w, r)
	if !ok {
		return
	}

	h.reader.ServeManifest(w, r, comicID)
}

// SharedPage sert une page d'un album partagé.
func (h *Sharing) SharedPage(w http.ResponseWriter, r *http.Request) {
	comicID, ok := h.resolveComic(w, r)
	if !ok {
		return
	}
	h.reader.ServePage(w, r, comicID)
}

// SharedCover sert la couverture d'un album partagé.
func (h *Sharing) SharedCover(w http.ResponseWriter, r *http.Request) {
	comicID, ok := h.resolveComic(w, r)
	if !ok {
		return
	}
	h.reader.ServeCover(w, r, comicID)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func shareToken(r *http.Request) string { return chi.URLParam(r, "token") }

// resolve retrouve le lien, ou répond 404.
func (h *Sharing) resolve(w http.ResponseWriter, r *http.Request) (folders.ShareLink, bool) {
	link, err := h.folders.ResolveShare(r.Context(), shareToken(r))
	if err != nil {
		// Révoqué, expiré ou inexistant : la même réponse pour les trois.
		// Distinguer confirmerait à un inconnu qu'un lien a existé.
		problem.Write(w, r, problem.NotFound("share link not found"))
		return folders.ShareLink{}, false
	}
	return link, true
}

// resolveComic vérifie que l'album demandé entre bien dans la portée du lien.
func (h *Sharing) resolveComic(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	link, ok := h.resolve(w, r)
	if !ok {
		return uuid.Nil, false
	}

	comicID, err := uuid.Parse(chi.URLParam(r, "comicID"))
	if err != nil {
		problem.Write(w, r, problem.NotFound("share link not found"))
		return uuid.Nil, false
	}

	covered, err := h.folders.ShareCovers(r.Context(), link, comicID)
	if err != nil || !covered {
		problem.Write(w, r, problem.NotFound("share link not found"))
		return uuid.Nil, false
	}
	return comicID, true
}

func writeShareError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, folders.ErrShareNotFound):
		problem.Write(w, r, problem.NotFound("share link not found"))

	case errors.Is(err, folders.ErrShareOnLockedFolder):
		problem.Write(w, r, problem.Validation(map[string]string{
			"folderPath": "this folder is hidden by an access code; a public link would contradict it",
		}))

	case errors.Is(err, folders.ErrShareExpiryRequired):
		problem.Write(w, r, problem.Validation(map[string]string{
			"expiresAt": "a public link must expire, and in the future",
		}))

	case errors.Is(err, folders.ErrShareExpiryTooFar):
		problem.Write(w, r, problem.Validation(map[string]string{
			"expiresAt": "at most one year from now",
		}))

	default:
		writeFolderError(w, r, err)
	}
}
