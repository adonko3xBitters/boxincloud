package handlers

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/catalog"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/logging"
	"github.com/adonko3xBitters/boxincloud/server/internal/reader"
)

// Reader sert les pages et les couvertures.
type Reader struct {
	svc     *reader.Service
	catalog *catalog.Service
}

func NewReader(svc *reader.Service, cat *catalog.Service) *Reader {
	return &Reader{svc: svc, catalog: cat}
}

// ─── Manifeste ───────────────────────────────────────────────────────────────

type manifestDTO struct {
	ComicID   uuid.UUID         `json:"comicId"`
	PageCount int32             `json:"pageCount"`
	Pages     []manifestPageDTO `json:"pages"`
}

type manifestPageDTO struct {
	Index    int32  `json:"index"`
	Width    *int32 `json:"width,omitempty"`
	Height   *int32 `json:"height,omitempty"`
	IsDouble bool   `json:"isDouble"`
}

// Manifest décrit un album pour le lecteur, en une seule requête.
//
// Le client obtient toutes les dimensions avant la première image et peut donc
// réserver la mise en page — aucun décalage visuel pendant la lecture.
func (h *Reader) Manifest(w http.ResponseWriter, r *http.Request) {
	comicID, ok := h.authorizedComic(w, r)
	if !ok {
		return
	}
	h.ServeManifest(w, r, comicID)
}

/*
ServeManifest, ServePage et ServeCover servent un album DÉJÀ autorisé.

Extraites pour que le partage public les réutilise : il autorise autrement — par
un jeton de lien plutôt que par un compte — mais sert exactement les mêmes
octets, avec les mêmes en-têtes de cache et les mêmes requêtes conditionnelles.
Les dupliquer aurait garanti qu'une des deux copies finisse par diverger.
*/
func (h *Reader) ServeManifest(w http.ResponseWriter, r *http.Request, comicID uuid.UUID) {
	manifest, err := h.svc.Manifest(r.Context(), comicID)
	if err != nil {
		writeReaderError(w, r, err)
		return
	}

	pages := make([]manifestPageDTO, 0, len(manifest.Pages))
	for _, p := range manifest.Pages {
		pages = append(pages, manifestPageDTO{
			Index: p.Index, Width: p.Width, Height: p.Height, IsDouble: p.IsDouble,
		})
	}

	writeJSON(w, http.StatusOK, manifestDTO{
		ComicID:   manifest.ComicID,
		PageCount: manifest.PageCount,
		Pages:     pages,
	})
}

// ─── Pages ───────────────────────────────────────────────────────────────────

// Page sert une page d'album.
func (h *Reader) Page(w http.ResponseWriter, r *http.Request) {
	comicID, ok := h.authorizedComic(w, r)
	if !ok {
		return
	}
	h.ServePage(w, r, comicID)
}

// ServePage sert une page d'un album déjà autorisé.
func (h *Reader) ServePage(w http.ResponseWriter, r *http.Request, comicID uuid.UUID) {
	index, err := strconv.ParseInt(chi.URLParam(r, "index"), 10, 32)
	if err != nil {
		problem.Write(w, r, problem.BadRequest("invalid page index"))
		return
	}

	content, err := h.svc.GetPage(r.Context(), reader.PageRequest{
		ComicID: comicID,
		Index:   int32(index),
		Width:   int(intParam(r, "width", 0)),
		Accept:  r.Header.Get("Accept"),
	})
	if err != nil {
		writeReaderError(w, r, err)
		return
	}
	defer func() { _ = content.Body.Close() }()

	// Une variante de page est immuable : même album, même page, même largeur
	// donnent toujours les mêmes octets. Le client peut la garder un an.
	writeImage(w, r, content, "public, max-age=31536000, immutable")
}

// Cover sert une vignette de couverture.
func (h *Reader) Cover(w http.ResponseWriter, r *http.Request) {
	comicID, ok := h.authorizedComic(w, r)
	if !ok {
		return
	}
	h.ServeCover(w, r, comicID)
}

// ServeCover sert la couverture d'un album déjà autorisé.
func (h *Reader) ServeCover(w http.ResponseWriter, r *http.Request, comicID uuid.UUID) {
	content, err := h.svc.GetCover(
		r.Context(), comicID, int(intParam(r, "width", 0)), r.Header.Get("Accept"))
	if err != nil {
		writeReaderError(w, r, err)
		return
	}
	defer func() { _ = content.Body.Close() }()

	// Plus courte que pour les pages : une couverture peut changer si l'album
	// est réindexé après correction de ses métadonnées.
	writeImage(w, r, content, "private, max-age=86400")
}

// writeImage écrit une image avec sa validation de cache.
func writeImage(w http.ResponseWriter, r *http.Request, content reader.PageContent, cacheControl string) {
	/*
		Sans ce Vary, la même URL servirait des octets différents selon le
		client, et n'importe quel cache intermédiaire — un proxy d'entreprise,
		un CDN devant l'instance — servirait joyeusement l'AVIF du premier
		visiteur au suivant, qui ne saurait pas le lire.

		L'en-tête doit être posé avant le 304 : une réponse conditionnelle est
		une réponse de cache, et c'est justement elle qu'il faut cadrer.
	*/
	w.Header().Set("Vary", "Accept")

	// Requête conditionnelle : si le client a déjà cette variante, on lui
	// répond 304 sans transférer un octet. Sur un album de soixante pages
	// relu, c'est tout le trafic d'images qui disparaît.
	if match := r.Header.Get("If-None-Match"); match != "" && match == content.ETag {
		w.Header().Set("ETag", content.ETag)
		w.Header().Set("Cache-Control", cacheControl)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", content.ContentType)
	w.Header().Set("ETag", content.ETag)
	w.Header().Set("Cache-Control", cacheControl)
	if content.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(content.Size, 10))
	}
	// Les pages sont des images d'utilisateurs : on interdit au navigateur de
	// deviner un autre type, ce qui fermerait la porte à une archive contenant
	// un faux « .jpg » interprété comme du HTML.
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if _, err := io.Copy(w, content.Body); err != nil {
		// L'en-tête est déjà parti : on ne peut plus signaler l'erreur au
		// client. Le plus fréquent est de toute façon une simple fermeture
		// d'onglet en cours de chargement.
		logging.FromContext(r.Context()).Debug("écriture de l'image interrompue", "err", err)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// authorizedComic résout l'album demandé et vérifie que le viewer y a accès.
//
// Le contrôle passe par le catalogue, seul détenteur de la règle de visibilité
// des bibliothèques : le lecteur n'a pas à la réimplémenter, et ne peut donc
// pas en diverger.
func (h *Reader) authorizedComic(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return uuid.Nil, false
	}

	id, err := uuid.Parse(chi.URLParam(r, "comicID"))
	if err != nil {
		problem.Write(w, r, problem.BadRequest("invalid comic id"))
		return uuid.Nil, false
	}

	if _, err := h.catalog.GetComic(r.Context(), v, id); err != nil {
		writeCatalogError(w, r, err)
		return uuid.Nil, false
	}
	return id, true
}

func writeReaderError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, reader.ErrNotFound):
		problem.Write(w, r, problem.NotFound("page not found"))
	case errors.Is(err, reader.ErrPageOutRange):
		problem.Write(w, r, problem.Validation(map[string]string{
			"index": "range",
		}))
	case errors.Is(err, reader.ErrNotReady):
		p := problem.Problem{
			Type:   "https://boxincloud.dev/problems/not-indexed",
			Title:  "Comic Not Indexed",
			Status: http.StatusConflict,
			Detail: "this comic has not finished indexing yet",
		}
		problem.Write(w, r, p)
	default:
		writeInternal(w, r, err)
	}
}
