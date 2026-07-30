package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/catalog"
	"github.com/adonko3xBitters/boxincloud/server/internal/folders"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
	"github.com/adonko3xBitters/boxincloud/server/internal/library"
)

/*
Folders expose l'arborescence et sa gestion.

Les dossiers ne se déduisent plus seulement des clés d'objet : ils existent en
base, ce qui permet d'en créer un vide, de le renommer, et bientôt d'y attacher
un verrou ou un partage.
*/
type Folders struct {
	svc     *folders.Service
	catalog *catalog.Service
}

func NewFolders(svc *folders.Service, catalogSvc *catalog.Service) *Folders {
	return &Folders{svc: svc, catalog: catalogSvc}
}

type folderNodeDTO struct {
	ID         string `json:"id"`
	LibraryID  string `json:"libraryId"`
	Path       string `json:"path"`
	Name       string `json:"name"`
	Depth      int    `json:"depth"`
	ComicCount int    `json:"comicCount"`
	Explicit   bool   `json:"explicit"`
}

func toFolderDTO(f folders.Folder) folderNodeDTO {
	return folderNodeDTO{
		ID:         f.ID.String(),
		LibraryID:  f.LibraryID.String(),
		Path:       f.Path,
		Name:       f.Name,
		Depth:      f.Depth,
		ComicCount: f.ComicCount,
		Explicit:   f.Explicit,
	}
}

/*
List retourne l'arborescence, à plat, parents avant enfants.

Le client bâtit l'arbre en une passe, sans tri ni récursion. Les compteurs sont
cumulés : un dossier affiche le total de sa branche, ce qu'attend quelqu'un qui
replie un nœud.
*/
func (h *Folders) List(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	libraryIDs, err := h.visibleLibraries(r, v)
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}

	tree, err := h.svc.Tree(r.Context(), libraryIDs)
	if err != nil {
		writeInternal(w, r, err)
		return
	}

	out := make([]folderNodeDTO, 0, len(tree))
	for _, f := range tree {
		out = append(out, toFolderDTO(f))
	}
	writeJSON(w, http.StatusOK, map[string]any{"folders": out})
}

type createFolderRequest struct {
	LibraryID string `json:"libraryId"`
	Path      string `json:"path"`
}

/*
Create inscrit un dossier, ancêtres compris.

Rien n'est écrit dans le backend : un magasin d'objets n'a pas de répertoires.
Le dossier existe donc d'abord dans boxincloud, et prendra corps au premier
fichier déposé.
*/
func (h *Folders) Create(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	var req createFolderRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	libraryID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"libraryId": "must be a UUID"}))
		return
	}
	if !h.mayWrite(w, r, v, libraryID) {
		return
	}

	folder, err := h.svc.Create(r.Context(), libraryID, req.Path)
	if err != nil {
		writeFolderError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toFolderDTO(folder))
}

type relocateFolderRequest struct {
	LibraryID string `json:"libraryId"`
	Path      string `json:"path"`
	NewPath   string `json:"newPath"`
}

/*
Relocate renomme ou déplace une branche.

Les deux gestes sont le même en dessous : le dossier d'un album découle de la clé
de son objet, si bien que renommer un dossier renomme chacun des objets qu'il
contient. C'est pourquoi cette route dispose d'un délai plus large que les
autres.
*/
func (h *Folders) Relocate(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	var req relocateFolderRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	libraryID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"libraryId": "must be a UUID"}))
		return
	}
	if !h.mayWrite(w, r, v, libraryID) {
		return
	}

	folder, err := h.svc.Relocate(r.Context(), libraryID, req.Path, req.NewPath)
	if err != nil {
		writeFolderError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toFolderDTO(folder))
}

/*
Delete retire un dossier.

Trois degrés, du plus sûr au plus définitif. Sans paramètre, un dossier qui
contient encore des albums est REFUSÉ : supprimer une branche entière ne doit
jamais être un geste distrait. `deleteComics` retire les albums du catalogue en
laissant les fichiers ; `deleteFiles` les efface aussi.
*/
func (h *Folders) Delete(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	libraryID, err := uuid.Parse(chi.URLParam(r, "libraryID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"libraryId": "must be a UUID"}))
		return
	}
	if !h.mayWrite(w, r, v, libraryID) {
		return
	}

	query := r.URL.Query()
	deleteFiles := query.Get("deleteFiles") == "true"

	affected, err := h.svc.Delete(r.Context(), folders.DeleteParams{
		LibraryID: libraryID,
		Path:      query.Get("path"),
		// Effacer les fichiers implique de retirer les albums : exiger les deux
		// drapeaux ferait échouer une demande pourtant sans ambiguïté.
		DeleteComics: query.Get("deleteComics") == "true" || deleteFiles,
		DeleteFiles:  deleteFiles,
	})
	if err != nil {
		if errors.Is(err, folders.ErrNotEmpty) {
			problem.Write(w, r, problem.Problem{
				Status: http.StatusConflict,
				Type:   "https://boxincloud.dev/problems/folder-not-empty",
				Title:  "Folder Not Empty",
				Detail: "this folder still contains comics; confirm with deleteComics",
				Errors: map[string]string{"comicCount": strconv.Itoa(affected)},
			})
			return
		}
		writeFolderError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"removedComics": affected})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// visibleLibraries résout les bibliothèques que le viewer peut consulter.
func (h *Folders) visibleLibraries(r *http.Request, v catalog.Viewer) ([]uuid.UUID, error) {
	list, err := h.catalog.ListLibraries(r.Context(), v)
	if err != nil {
		return nil, err
	}

	only := r.URL.Query().Get("libraryId")

	ids := make([]uuid.UUID, 0, len(list))
	for _, lib := range list {
		if only != "" && lib.ID.String() != only {
			continue
		}
		ids = append(ids, lib.ID)
	}
	return ids, nil
}

// mayWrite vérifie que le viewer peut modifier l'arborescence.
//
// La même règle que la consultation pour l'instant : qui voit une bibliothèque
// peut la ranger. La distinction fine entre lecture et écriture viendra avec le
// partage par dossier.
func (h *Folders) mayWrite(
	w http.ResponseWriter, r *http.Request, v catalog.Viewer, libraryID uuid.UUID,
) bool {
	if v.IsAdmin {
		return true
	}

	allowed, err := h.catalog.CanAccessLibrary(r.Context(), v, libraryID)
	if err != nil {
		writeInternal(w, r, err)
		return false
	}
	if !allowed {
		problem.Write(w, r, problem.NotFound("library not found"))
		return false
	}
	return true
}

func writeFolderError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, folders.ErrNotFound), errors.Is(err, library.ErrLibraryNotFound):
		problem.Write(w, r, problem.NotFound("folder not found"))

	case errors.Is(err, folders.ErrAlreadyExists):
		problem.Write(w, r, problem.Validation(map[string]string{
			"path": "a folder or file already exists at this location",
		}))

	case errors.Is(err, folders.ErrRootImmutable):
		problem.Write(w, r, problem.Validation(map[string]string{
			"path": "the library root cannot be renamed or deleted",
		}))

	case errors.Is(err, folders.ErrIntoItself):
		problem.Write(w, r, problem.Validation(map[string]string{
			"newPath": "a folder cannot be moved inside itself",
		}))

	case errors.Is(err, folders.ErrInvalidName):
		problem.Write(w, r, problem.Validation(map[string]string{"path": "unusable folder name"}))

	case errors.Is(err, folders.ErrTooManyComics):
		problem.Write(w, r, problem.Validation(map[string]string{
			"path": "too many comics in this branch; move them in smaller batches",
		}))

	default:
		writeInternal(w, r, err)
	}
}
