package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/catalog"
	"github.com/adonko3xBitters/boxincloud/server/internal/discovery"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
)

/*
Recherche fédérée et administration des catalogues.

Deux natures de routes dans un même fichier, et deux niveaux d'accès.

Chercher est ouvert à tout compte : c'est une consultation, et les résultats
sont filtrés par le profil du lecteur comme le reste du catalogue.

Déclarer un catalogue est réservé à l'administration, et pour une raison plus
forte que l'habitude : l'URL enregistrée est une adresse que le SERVEUR ira
joindre, depuis l'intérieur du réseau. Laisser un compte ordinaire en ajouter
une reviendrait à lui offrir un sondeur de réseau interne.
*/

// Discovery expose la recherche fédérée.
type Discovery struct {
	svc     *discovery.Service
	catalog *catalog.Service
}

func NewDiscovery(svc *discovery.Service, cat *catalog.Service) *Discovery {
	return &Discovery{svc: svc, catalog: cat}
}

// ─── Représentations ─────────────────────────────────────────────────────────

type discoverySourceDTO struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	URL      string    `json:"url"`
	Kind     string    `json:"kind"`
	Enabled  bool      `json:"enabled"`
	Username string    `json:"username,omitempty"`
	// LastError est le message rendu par le dernier essai. Vide quand le
	// catalogue répond.
	LastError     string     `json:"lastError,omitempty"`
	LastCheckedAt *time.Time `json:"lastCheckedAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

// Le mot de passe n'a pas de champ, et c'est le point : il entre par les
// requêtes de création et de modification, il ne ressort jamais.
func toDiscoverySourceDTO(s discovery.Source) discoverySourceDTO {
	return discoverySourceDTO{
		ID:            s.ID,
		Name:          s.Name,
		URL:           s.URL,
		Kind:          string(s.Kind),
		Enabled:       s.Enabled,
		Username:      s.Username,
		LastError:     s.LastError,
		LastCheckedAt: s.LastCheckAt,
		CreatedAt:     s.CreatedAt,
	}
}

// ─── Recherche ───────────────────────────────────────────────────────────────

/*
Search interroge les catalogues fédérés.

La réponse est toujours 200, même quand tous les catalogues sont injoignables.
Ce n'est pas de la complaisance : la requête a été traitée, et son résultat —
« aucun catalogue n'a répondu » — est porté par la liste des états. Répondre 502
obligerait l'interface à traiter comme une panne ce qui est une information.
*/
func (h *Discovery) Search(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	query := discovery.Query{
		Text:  r.URL.Query().Get("q"),
		Limit: int(intParam(r, "limit", 40)),
	}

	result, err := h.svc.Search(r.Context(), query, h.localView(v))
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

/*
localView donne au service de quoi marquer ce que l'instance possède déjà.

La clôture est liée au lecteur courant : la recherche locale filtre par profil,
et sans cette liaison un profil restreint apprendrait l'existence de titres
qu'il n'a pas le droit de voir.
*/
func (h *Discovery) localView(v catalog.Viewer) discovery.LocalCatalog {
	return func(ctx context.Context, text string, limit int) ([]string, error) {
		found, err := h.catalog.Search(ctx, v, text, nil, int32(limit))
		if err != nil {
			return nil, err
		}

		titles := make([]string, 0, len(found.Comics)+len(found.Series))
		for _, comic := range found.Comics {
			titles = append(titles, comic.Title)
		}
		for _, series := range found.Series {
			titles = append(titles, series.Name)
		}
		return titles, nil
	}
}

// ─── Administration des catalogues ───────────────────────────────────────────

func (h *Discovery) ListSources(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	sources, err := h.svc.List(r.Context())
	if err != nil {
		writeInternal(w, r, err)
		return
	}

	items := make([]discoverySourceDTO, 0, len(sources))
	for _, source := range sources {
		items = append(items, toDiscoverySourceDTO(source))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type createDiscoverySourceRequest struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Enabled  *bool  `json:"enabled"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Discovery) CreateSource(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var req createDiscoverySourceRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	source, err := h.svc.Create(r.Context(), discovery.CreateParams{
		Name:     req.Name,
		URL:      req.URL,
		Enabled:  enabled,
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		writeDiscoveryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toDiscoverySourceDTO(source))
}

type updateDiscoverySourceRequest struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Enabled  *bool  `json:"enabled"`
	Username string `json:"username"`
	// Un pointeur, pas une chaîne : l'absence du champ conserve le mot de passe
	// enregistré, une chaîne vide l'efface. Un formulaire qui renvoie tous ses
	// champs ne doit pas effacer un secret qu'il n'affiche pas.
	Password *string `json:"password"`
}

func (h *Discovery) UpdateSource(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	id, ok := pathUUID(w, r, "sourceID")
	if !ok {
		return
	}

	var req updateDiscoverySourceRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	params := discovery.UpdateParams{
		Name:     req.Name,
		URL:      req.URL,
		Enabled:  req.Enabled == nil || *req.Enabled,
		Username: req.Username,
	}
	if req.Password != nil {
		params.Password = *req.Password
		params.PasswordSet = true
	}

	source, err := h.svc.Update(r.Context(), id, params)
	if err != nil {
		writeDiscoveryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDiscoverySourceDTO(source))
}

func (h *Discovery) DeleteSource(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	id, ok := pathUUID(w, r, "sourceID")
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeDiscoveryError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TestSource rejoue l'essai de connexion sur un catalogue enregistré.
func (h *Discovery) TestSource(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	id, ok := pathUUID(w, r, "sourceID")
	if !ok {
		return
	}

	if err := h.svc.Probe(r.Context(), id); err != nil {
		if errors.Is(err, discovery.ErrSourceNotFound) {
			problem.Write(w, r, problem.NotFound("discovery source not found"))
			return
		}
		/*
			L'essai a eu lieu et a échoué : c'est un résultat, pas une panne du
			serveur. D'où le 200.

			Le texte rendu est le DIAGNOSTIC brut du catalogue distant —
			« connection refused », « 404 Not Found ». C'est la seule exception
			à la règle qui veut que le serveur n'envoie jamais de prose : ce
			message ne vient pas de boxincloud, il vient du service tiers, et
			aucun catalogue de traductions ne peut couvrir ce qu'un serveur
			inconnu répondra. L'interface l'affiche comme un détail technique
			sous un titre traduit, pas comme une phrase à lire.
		*/
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeDiscoveryError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, discovery.ErrSourceNotFound):
		problem.Write(w, r, problem.NotFound("discovery source not found"))
	case errors.Is(err, discovery.ErrNoSearch):
		problem.Write(w, r, problem.Validation(map[string]string{"url": "no-search"}))
	case errors.Is(err, discovery.ErrInvalidSource):
		problem.Write(w, r, problem.Validation(map[string]string{"url": "invalid"}))
	default:
		writeInternal(w, r, err)
	}
}

// pathUUID lit un identifiant porté par le chemin, ou répond 400.
//
// Distinct de `uuidParam`, qui lit un paramètre de requête où l'absence est
// légitime. Ici, un segment d'URL absent n'existe pas : la route ne
// correspondrait pas.
func pathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		problem.Write(w, r, problem.BadRequest("invalid "+name))
		return uuid.UUID{}, false
	}
	return id, true
}
