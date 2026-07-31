package handlers

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/cache"
	"github.com/adonko3xBitters/boxincloud/server/internal/catalog"
	"github.com/adonko3xBitters/boxincloud/server/internal/folders"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
	"github.com/adonko3xBitters/boxincloud/server/internal/ingest"
	"github.com/adonko3xBitters/boxincloud/server/internal/library"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
)

/*
Admin administre les backends et les bibliothèques, et fait entrer du contenu.

Ces opérations n'existaient qu'en ligne de commande, ce qui revenait à exiger
un accès shell au serveur pour ajouter un album. Une bibliothèque qu'on ne peut
pas remplir depuis son interface n'est pas une bibliothèque.
*/
type Admin struct {
	libraries *library.Service
	catalog   *catalog.Service
	ingest    *ingest.Service
	cache     *cache.Cache
}

func NewAdmin(
	libraries *library.Service,
	catalogSvc *catalog.Service,
	ingestSvc *ingest.Service,
	derived *cache.Cache,
) *Admin {
	return &Admin{libraries: libraries, catalog: catalogSvc, ingest: ingestSvc, cache: derived}
}

// ─── Cache dérivé ────────────────────────────────────────────────────────────

func (h *Admin) CacheStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.cache.Stats(r.Context())
	if err != nil {
		writeInternal(w, r, err)
		return
	}

	out := map[string]any{
		"entries":  stats.Entries,
		"bytes":    stats.Bytes,
		"hits":     stats.Hits,
		"maxBytes": stats.MaxBytes,
	}
	if stats.OldestAt != nil {
		out["oldestAt"] = stats.OldestAt.UTC().Format(time.RFC3339)
	}
	if stats.NewestHitAt != nil {
		out["newestHitAt"] = stats.NewestHitAt.UTC().Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, out)
}

// PurgeCache vide le cache dérivé.
//
// Aucune confirmation côté serveur : tout y est reconstructible, et exiger un
// jeton de confirmation pour une opération sans perte serait de la cérémonie.
// L'interface, elle, demande confirmation — parce qu'une purge coûte du temps
// de régénération, pas parce qu'elle serait dangereuse.
func (h *Admin) PurgeCache(w http.ResponseWriter, r *http.Request) {
	entries, freed, err := h.cache.Purge(r.Context())
	if err != nil {
		writeInternal(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]int64{"entries": entries, "bytes": freed})
}

// ─── Backends ────────────────────────────────────────────────────────────────

type backendDTO struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Kind      string            `json:"kind"`
	Config    map[string]string `json:"config"`
	IsDefault bool              `json:"isDefault"`
	ReadOnly  bool              `json:"readOnly"`
	Status    string            `json:"status"`
}

// toBackendDTO n'expose jamais les secrets.
//
// Ils sont chiffrés en base et ne ressortent pas du service ; ce commentaire
// est là pour que l'ajout d'un champ ici soit un geste conscient.
func toBackendDTO(b library.Backend) backendDTO {
	return backendDTO{
		ID:        b.ID.String(),
		Name:      b.Name,
		Kind:      string(b.Kind),
		Config:    b.Config,
		IsDefault: b.IsDefault,
		ReadOnly:  b.ReadOnly,
		Status:    b.Status,
	}
}

func (h *Admin) ListBackends(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	backends, err := h.libraries.ListBackends(r.Context())
	if err != nil {
		writeInternal(w, r, err)
		return
	}

	out := make([]backendDTO, 0, len(backends))
	for _, b := range backends {
		out = append(out, toBackendDTO(b))
	}
	writeJSON(w, http.StatusOK, map[string]any{"backends": out})
}

type createBackendRequest struct {
	Name      string            `json:"name"`
	Kind      string            `json:"kind"`
	Config    map[string]string `json:"config"`
	Secrets   map[string]string `json:"secrets"`
	IsDefault bool              `json:"isDefault"`
	ReadOnly  bool              `json:"readOnly"`
}

/*
CreateBackend enregistre un backend de stockage.

Le service vérifie qu'il répond avant de l'enregistrer : un backend injoignable
entré en base produirait des scans en échec dont la cause serait à chercher
ailleurs.
*/
func (h *Admin) CreateBackend(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var req createBackendRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Name == "" {
		problem.Write(w, r, problem.Validation(map[string]string{"name": "required"}))
		return
	}

	kind := storage.Kind(req.Kind)
	if kind != storage.KindS3 && kind != storage.KindLocal {
		problem.Write(w, r, problem.Validation(map[string]string{
			"kind": "must be one of s3, local",
		}))
		return
	}

	backend, err := h.libraries.CreateBackend(r.Context(), library.CreateBackendParams{
		Name:      req.Name,
		Kind:      kind,
		Config:    req.Config,
		Secrets:   req.Secrets,
		IsDefault: req.IsDefault,
		ReadOnly:  req.ReadOnly,
	})
	if err != nil {
		writeLibraryError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toBackendDTO(backend))
}

// TestBackend vérifie qu'un backend répond toujours.
func (h *Admin) TestBackend(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "backendID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"backendId": "must be a UUID"}))
		return
	}

	if err := h.libraries.TestBackend(r.Context(), id); err != nil {
		// L'échec du test n'est pas une erreur de la requête : le client a bien
		// demandé un test, et il en reçoit le résultat.
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ─── Bibliothèques ───────────────────────────────────────────────────────────

type libraryAdminDTO struct {
	ID         string `json:"id"`
	BackendID  string `json:"backendId"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	RootPrefix string `json:"rootPrefix"`
	ComicCount int32  `json:"comicCount"`
}

type createLibraryRequest struct {
	Name       string `json:"name"`
	BackendID  string `json:"backendId"`
	Kind       string `json:"kind"`
	RootPrefix string `json:"rootPrefix"`
}

// CreateLibrary enregistre une bibliothèque sur un backend existant.
func (h *Admin) CreateLibrary(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var req createLibraryRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	errs := map[string]string{}
	if req.Name == "" {
		errs["name"] = "required"
	}
	backendID, err := uuid.Parse(req.BackendID)
	if err != nil {
		errs["backendId"] = "must be a UUID"
	}
	if len(errs) > 0 {
		problem.Write(w, r, problem.Validation(errs))
		return
	}

	kind := req.Kind
	if kind == "" {
		kind = "comics"
	}

	lib, err := h.libraries.CreateLibrary(r.Context(), library.CreateLibraryParams{
		Name:       req.Name,
		BackendID:  backendID,
		Kind:       kind,
		RootPrefix: req.RootPrefix,
	})
	if err != nil {
		writeLibraryError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, libraryAdminDTO{
		ID:         lib.ID.String(),
		BackendID:  lib.BackendID.String(),
		Name:       lib.Name,
		Kind:       lib.Kind,
		RootPrefix: lib.RootPrefix,
		ComicCount: lib.ComicCount,
	})
}

/*
Scan demande un parcours complet de la bibliothèque.

La réponse est 202 : le scan est enfilé, pas exécuté. Parcourir des dizaines de
milliers d'objets et en lire l'index prend des minutes — le faire dans la
requête la ferait expirer bien avant la fin.
*/
func (h *Admin) Scan(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "libraryID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"libraryId": "must be a UUID"}))
		return
	}

	if err := h.ingest.Scan(r.Context(), id); err != nil {
		writeLibraryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"queued": true})
}

// ─── Téléversement ───────────────────────────────────────────────────────────

type uploadedDTO struct {
	ComicID   string `json:"comicId"`
	ObjectKey string `json:"objectKey"`
	Title     string `json:"title"`
	Format    string `json:"format"`
	FileSize  int64  `json:"fileSize"`
}

/*
Upload reçoit un fichier et le fait entrer dans la bibliothèque.

Le corps est lu en flux, partie par partie, plutôt que via ParseMultipartForm :
cette dernière écrit sur disque tout ce qui dépasse un seuil, ce qui pour une
intégrale de cinq cents méga-octets signifie écrire le fichier deux fois — une
fois dans un temporaire du serveur, une fois dans le backend.

Les champs sont donc lus dans l'ordre où ils arrivent, et le client doit envoyer
le dossier AVANT le fichier. C'est une contrainte réelle, documentée dans le
contrat, et le prix du flux direct.
*/
func (h *Admin) Upload(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	libraryID, err := uuid.Parse(chi.URLParam(r, "libraryID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"libraryId": "must be a UUID"}))
		return
	}

	// Le téléversement suit la visibilité de la bibliothèque : qui peut la
	// consulter peut l'alimenter. La distinction plus fine entre lecture et
	// écriture viendra avec les rôles.
	if !v.IsAdmin {
		allowed, err := h.catalog.CanAccessLibrary(r.Context(), v, libraryID)
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		if !allowed {
			problem.Write(w, r, problem.NotFound("library not found"))
			return
		}
	}

	reader, err := r.MultipartReader()
	if err != nil {
		problem.Write(w, r, problem.BadRequest("expected a multipart/form-data body"))
		return
	}

	var folder string

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			problem.Write(w, r, problem.BadRequest("malformed multipart body"))
			return
		}

		switch part.FormName() {
		case "folder":
			value, err := io.ReadAll(io.LimitReader(part, 1024))
			_ = part.Close()
			if err != nil {
				problem.Write(w, r, problem.BadRequest("unreadable folder field"))
				return
			}
			folder = string(value)

		case "file":
			result, err := h.ingest.Upload(r.Context(), ingest.UploadParams{
				LibraryID: libraryID,
				Folder:    folder,
				Filename:  part.FileName(),
				// La taille n'est pas connue d'une partie multipart. Le backend
				// bascule alors sur un envoi fractionné, ce qui est exactement
				// le comportement voulu pour un flux de taille inconnue.
				Size:    -1,
				Content: part,
			})
			_ = part.Close()

			if err != nil {
				writeIngestError(w, r, err)
				return
			}

			writeJSON(w, http.StatusCreated, uploadedDTO{
				ComicID:   result.ComicID.String(),
				ObjectKey: result.ObjectKey,
				Title:     result.Title,
				Format:    result.Format,
				FileSize:  result.Size,
			})
			return

		default:
			_ = part.Close()
		}
	}

	problem.Write(w, r, problem.Validation(map[string]string{"file": "required"}))
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// requireAdmin refuse la requête si le compte n'est pas administrateur.
func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	v, ok := viewerFrom(w, r)
	if !ok {
		return false
	}
	if !v.IsAdmin {
		problem.Write(w, r, problem.Forbidden("administrator role required"))
		return false
	}
	return true
}

func writeLibraryError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, library.ErrLibraryNotFound):
		problem.Write(w, r, problem.NotFound("library not found"))
	case errors.Is(err, library.ErrBackendNotFound):
		problem.Write(w, r, problem.Validation(map[string]string{"backendId": "unknown backend"}))
	case errors.Is(err, library.ErrInvalidConfig):
		problem.Write(w, r, problem.Validation(map[string]string{"config": err.Error()}))
	default:
		writeInternal(w, r, err)
	}
}

func writeIngestError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ingest.ErrUnsupportedFormat):
		problem.Write(w, r, problem.Validation(map[string]string{
			"file": "accepted formats: " + joinExtensions(),
		}))
	case errors.Is(err, ingest.ErrContentMismatch):
		problem.Write(w, r, problem.Validation(map[string]string{
			"file": "file content does not match its extension",
		}))
	case errors.Is(err, ingest.ErrEmptyName):
		problem.Write(w, r, problem.Validation(map[string]string{"file": "unusable filename"}))
	case errors.Is(err, ingest.ErrAlreadyExists):
		problem.Write(w, r, problem.Validation(map[string]string{
			"file": "a file with this name already exists in the destination folder",
		}))
	case errors.Is(err, ingest.ErrComicNotFound):
		problem.Write(w, r, problem.NotFound("comic not found"))
	case errors.Is(err, ingest.ErrTooManyItems):
		problem.Write(w, r, problem.Validation(map[string]string{
			"ids": "at most 1000 items per request",
		}))
	case errors.Is(err, ingest.ErrTooLarge):
		problem.Write(w, r, problem.Problem{
			Status: http.StatusRequestEntityTooLarge,
			Title:  "Payload Too Large",
			Detail: "the file exceeds the configured upload limit",
		})
	case errors.Is(err, storage.ErrReadOnly):
		problem.Write(w, r, problem.Forbidden("this library is read-only"))

	// Le verrou vient du paquet folders : l'ingestion le reçoit à travers son
	// garde d'écriture, sans le connaître.
	case errors.Is(err, folders.ErrReadOnly):
		problem.Write(w, r, problem.Problem{
			Status: http.StatusConflict,
			Type:   "https://boxincloud.dev/problems/folder-read-only",
			Title:  "Folder Read Only",
			Detail: "this folder, or one of its parents, is protected against changes",
		})
	case errors.Is(err, library.ErrLibraryNotFound):
		problem.Write(w, r, problem.NotFound("library not found"))
	default:
		writeInternal(w, r, err)
	}
}

func joinExtensions() string {
	return strings.Join(ingest.SupportedExtensions(), ", ")
}

// ─── Suppression et déplacement ──────────────────────────────────────────────

/*
DeleteComic retire un album, avec ou sans son fichier.

Le fichier est CONSERVÉ par défaut. Retirer un album d'un catalogue se rattrape ;
effacer un fichier non. Le paramètre doit donc être demandé explicitement, et
l'interface doit poser la question plutôt que de choisir à la place de
l'utilisateur.
*/
func (h *Admin) DeleteComic(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	comicID, err := uuid.Parse(chi.URLParam(r, "comicID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"comicId": "must be a UUID"}))
		return
	}

	// La visibilité est vérifiée par le catalogue, qui porte déjà toute la
	// règle : bibliothèque restreinte comme classification d'âge.
	if _, err := h.catalog.GetComic(r.Context(), v, comicID); err != nil {
		writeCatalogError(w, r, err)
		return
	}

	deleteFile := r.URL.Query().Get("deleteFile") == "true"

	if err := h.ingest.Delete(r.Context(), ingest.DeleteParams{
		ComicID:    comicID,
		DeleteFile: deleteFile,
	}); err != nil {
		writeIngestError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type moveComicRequest struct {
	Folder string `json:"folder"`
}

// MoveComic range un album dans un autre dossier.
func (h *Admin) MoveComic(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	comicID, err := uuid.Parse(chi.URLParam(r, "comicID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"comicId": "must be a UUID"}))
		return
	}

	if _, err := h.catalog.GetComic(r.Context(), v, comicID); err != nil {
		writeCatalogError(w, r, err)
		return
	}

	var req moveComicRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	folder, err := h.ingest.Move(r.Context(), ingest.MoveParams{
		ComicID: comicID,
		Folder:  req.Folder,
	})
	if err != nil {
		if errors.Is(err, ingest.ErrSameFolder) {
			// Le résultat demandé est déjà atteint : ce n'est pas une erreur.
			writeJSON(w, http.StatusOK, map[string]any{"folderPath": req.Folder})
			return
		}
		writeIngestError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"folderPath": folder})
}

type bulkManageRequest struct {
	Action     string   `json:"action"`
	IDs        []string `json:"ids"`
	Folder     string   `json:"folder"`
	DeleteFile bool     `json:"deleteFile"`
}

/*
BulkManage supprime ou déplace une sélection.

Les identifiants sont filtrés sur ce que le compte peut voir AVANT toute
écriture : une sélection ne doit pas pouvoir déborder sur une bibliothèque
inaccessible, même si le client envoie des identifiants arbitraires.
*/
func (h *Admin) BulkManage(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	var req bulkManageRequest
	if !decodeJSONLarge(w, r, &req) {
		return
	}

	if req.Action != "delete" && req.Action != "move" {
		problem.Write(w, r, problem.Validation(map[string]string{
			"action": "must be one of delete, move",
		}))
		return
	}

	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, raw := range req.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			problem.Write(w, r, problem.Validation(map[string]string{
				"ids": "all entries must be valid UUIDs",
			}))
			return
		}
		ids = append(ids, id)
	}

	allowed := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, err := h.catalog.GetComic(r.Context(), v, id); err == nil {
			allowed = append(allowed, id)
		}
	}

	var (
		affected int
		err      error
	)
	if req.Action == "delete" {
		affected, err = h.ingest.BulkDelete(r.Context(), allowed, req.DeleteFile)
	} else {
		affected, err = h.ingest.BulkMove(r.Context(), allowed, req.Folder)
	}
	if err != nil {
		writeIngestError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"affected": affected})
}

// ─── Modification et suppression ─────────────────────────────────────────────

type updateBackendRequest struct {
	Name     *string           `json:"name"`
	Config   map[string]string `json:"config"`
	Secrets  map[string]string `json:"secrets"`
	ReadOnly *bool             `json:"readOnly"`
}

/*
UpdateBackend modifie un stockage.

Les secrets absents sont conservés : un administrateur qui corrige un endpoint ne
doit pas avoir à retaper ses clés, et ne le pourrait pas — elles ne ressortent
jamais de la base.

La configuration résultante est jointe avant d'être enregistrée : valider celle
qui a été envoyée seule reviendrait à valider une configuration qui n'existera
jamais.
*/
func (h *Admin) UpdateBackend(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "backendID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"backendId": "must be a UUID"}))
		return
	}

	var req updateBackendRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	backend, err := h.libraries.UpdateBackend(r.Context(), id, library.UpdateBackendParams{
		Name:     req.Name,
		Config:   req.Config,
		Secrets:  req.Secrets,
		ReadOnly: req.ReadOnly,
	})
	if err != nil {
		writeLibraryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toBackendDTO(backend))
}

// DeleteBackend supprime un stockage, refusé tant qu'il porte des bibliothèques.
func (h *Admin) DeleteBackend(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "backendID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"backendId": "must be a UUID"}))
		return
	}

	if err := h.libraries.DeleteBackend(r.Context(), id); err != nil {
		if errors.Is(err, library.ErrBackendInUse) {
			problem.Write(w, r, problem.Problem{
				Status: http.StatusConflict,
				Type:   "https://boxincloud.dev/problems/backend-in-use",
				Title:  "Storage In Use",
				Detail: "libraries still rely on this storage; delete them first",
			})
			return
		}
		writeLibraryError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetDefaultBackend désigne le stockage proposé par défaut.
func (h *Admin) SetDefaultBackend(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "backendID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"backendId": "must be a UUID"}))
		return
	}

	if err := h.libraries.SetDefaultBackend(r.Context(), id); err != nil {
		writeLibraryError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type updateLibraryRequest struct {
	Name       *string `json:"name"`
	RootPrefix *string `json:"rootPrefix"`
}

/*
UpdateLibrary modifie une bibliothèque.

Changer le préfixe racine ne DÉPLACE rien : les albums déjà indexés pointent des
clés construites avec l'ancien. Le changement décrit où chercher désormais, et un
nouveau parcours reconstruit le catalogue.
*/
func (h *Admin) UpdateLibrary(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "libraryID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"libraryId": "must be a UUID"}))
		return
	}

	var req updateLibraryRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	lib, err := h.libraries.UpdateLibrary(r.Context(), id, req.Name, req.RootPrefix)
	if err != nil {
		writeLibraryError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, libraryAdminDTO{
		ID:         lib.ID.String(),
		BackendID:  lib.BackendID.String(),
		Name:       lib.Name,
		Kind:       lib.Kind,
		RootPrefix: lib.RootPrefix,
		ComicCount: lib.ComicCount,
	})
}

// DeleteLibrary supprime une bibliothèque et tout ce qui s'y rattache.
//
// Les fichiers du stockage restent intacts. L'historique de lecture, lui, ne
// revient pas : l'interface doit le dire avant, pas après.
func (h *Admin) DeleteLibrary(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "libraryID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"libraryId": "must be a UUID"}))
		return
	}

	if err := h.libraries.DeleteLibrary(r.Context(), id); err != nil {
		writeLibraryError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Diagnostic ──────────────────────────────────────────────────────────────

type scanRunDTO struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	StartedAt   string  `json:"startedAt"`
	FinishedAt  *string `json:"finishedAt,omitempty"`
	ObjectsSeen int     `json:"objectsSeen"`
	Added       int     `json:"added"`
	Updated     int     `json:"updated"`
	Removed     int     `json:"removed"`
	Errors      int     `json:"errors"`
	Detail      string  `json:"detail,omitempty"`
}

/*
ScanRuns retourne l'historique des parcours.

C'est le seul endroit où l'on voit POURQUOI un parcours a échoué. Sans lui, un
scan en erreur ne se manifeste que par une bibliothèque qui ne se remplit pas,
sans le moindre indice.
*/
func (h *Admin) ScanRuns(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "libraryID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"libraryId": "must be a UUID"}))
		return
	}

	runs, err := h.libraries.ScanRuns(r.Context(), id, 10)
	if err != nil {
		writeLibraryError(w, r, err)
		return
	}

	out := make([]scanRunDTO, 0, len(runs))
	for _, run := range runs {
		dto := scanRunDTO{
			ID:          run.ID.String(),
			Status:      run.Status,
			StartedAt:   run.StartedAt.UTC().Format(time.RFC3339),
			ObjectsSeen: run.ObjectsSeen,
			Added:       run.Added,
			Updated:     run.Updated,
			Removed:     run.Removed,
			Errors:      run.Errors,
			Detail:      run.Detail,
		}
		if run.FinishedAt != nil {
			finished := run.FinishedAt.UTC().Format(time.RFC3339)
			dto.FinishedAt = &finished
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": out})
}
