package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/accounts"
	"github.com/adonko3xBitters/boxincloud/server/internal/auth"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/middleware"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
)

// Accounts administre les comptes et leurs accès.
type Accounts struct {
	svc *accounts.Service
}

func NewAccounts(svc *accounts.Service) *Accounts { return &Accounts{svc: svc} }

type accountDTO struct {
	ID           string  `json:"id"`
	Username     string  `json:"username"`
	Email        string  `json:"email,omitempty"`
	Role         string  `json:"role"`
	DisplayName  string  `json:"displayName,omitempty"`
	Restricted   bool    `json:"restricted"`
	MaxAgeRating *int16  `json:"maxAgeRating,omitempty"`
	LastLoginAt  *string `json:"lastLoginAt,omitempty"`
	CreatedAt    string  `json:"createdAt"`
}

func toAccountDTO(a accounts.Account) accountDTO {
	return accountDTO{
		ID:           a.ID.String(),
		Username:     a.Username,
		Email:        a.Email,
		Role:         a.Role,
		DisplayName:  a.DisplayName,
		Restricted:   a.Restricted,
		MaxAgeRating: a.MaxAgeRating,
		LastLoginAt:  a.LastLoginAt,
		CreatedAt:    a.CreatedAt,
	}
}

func (h *Accounts) List(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	list, err := h.svc.List(r.Context())
	if err != nil {
		writeInternal(w, r, err)
		return
	}

	out := make([]accountDTO, 0, len(list))
	for _, a := range list {
		out = append(out, toAccountDTO(a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": out})
}

type createAccountRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	Role        string `json:"role"`
	DisplayName string `json:"displayName"`
}

func (h *Accounts) Create(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var req createAccountRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Role == "" {
		req.Role = "user"
	}

	account, err := h.svc.Create(r.Context(), accounts.CreateParams{
		Username:    req.Username,
		Email:       req.Email,
		Password:    req.Password,
		Role:        req.Role,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		writeAccountError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toAccountDTO(account))
}

type updateAccountRequest struct {
	DisplayName  *string `json:"displayName"`
	Email        *string `json:"email"`
	Role         *string `json:"role"`
	Restricted   *bool   `json:"restricted"`
	MaxAgeRating *int16  `json:"maxAgeRating"`
	Password     *string `json:"password"`
}

/*
Update applique les changements demandés sur un compte.

Un seul point d'entrée pour le profil, le rôle, la restriction et le mot de
passe : ce sont quatre réglages d'une même fiche, et les séparer en quatre
requêtes obligerait l'interface à gérer quatre échecs partiels pour un seul
formulaire.
*/
func (h *Accounts) Update(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	actor, ok := claimsFrom(w, r)
	if !ok {
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"userId": "invalid"}))
		return
	}

	var req updateAccountRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	ctx := r.Context()
	account, err := h.svc.Get(ctx, targetID)
	if err != nil {
		writeAccountError(w, r, err)
		return
	}

	if req.DisplayName != nil || req.Email != nil {
		account, err = h.svc.UpdateProfile(ctx, targetID, req.DisplayName, req.Email)
		if err != nil {
			writeAccountError(w, r, err)
			return
		}
	}

	if req.Role != nil {
		account, err = h.svc.SetRole(ctx, actor.UserID, targetID, *req.Role)
		if err != nil {
			writeAccountError(w, r, err)
			return
		}
	}

	if req.Restricted != nil {
		account, err = h.svc.SetRestriction(ctx, targetID, *req.Restricted, req.MaxAgeRating)
		if err != nil {
			writeAccountError(w, r, err)
			return
		}
	}

	if req.Password != nil {
		if err := h.svc.ResetPassword(ctx, targetID, *req.Password); err != nil {
			writeAccountError(w, r, err)
			return
		}
	}

	writeJSON(w, http.StatusOK, toAccountDTO(account))
}

func (h *Accounts) Delete(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	actor, ok := claimsFrom(w, r)
	if !ok {
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"userId": "invalid"}))
		return
	}

	if err := h.svc.Delete(r.Context(), actor.UserID, targetID); err != nil {
		writeAccountError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Accès aux bibliothèques ─────────────────────────────────────────────────

type grantDTO struct {
	LibraryID string `json:"libraryId"`
	UserID    string `json:"userId"`
	CanWrite  bool   `json:"canWrite"`
}

// ListGrants retourne les accès explicites d'un compte.
func (h *Accounts) ListGrants(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"userId": "invalid"}))
		return
	}

	grants, err := h.svc.GrantsForUser(r.Context(), userID)
	if err != nil {
		writeAccountError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"grants": toGrantDTOs(grants)})
}

// ListLibraryGrants retourne les comptes autorisés sur une bibliothèque.
func (h *Accounts) ListLibraryGrants(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	libraryID, err := uuid.Parse(chi.URLParam(r, "libraryID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"libraryId": "invalid"}))
		return
	}

	grants, err := h.svc.GrantsForLibrary(r.Context(), libraryID)
	if err != nil {
		writeAccountError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"grants": toGrantDTOs(grants)})
}

type grantRequest struct {
	UserID   string `json:"userId"`
	CanWrite bool   `json:"canWrite"`
}

/*
Grant ouvre une bibliothèque à un compte.

Rappel du modèle, que l'interface doit relayer : une bibliothèque sans aucune
autorisation explicite est visible de tous. Le PREMIER accès accordé la referme
donc pour tout le monde d'autre.
*/
func (h *Accounts) Grant(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	libraryID, err := uuid.Parse(chi.URLParam(r, "libraryID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"libraryId": "invalid"}))
		return
	}

	var req grantRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"userId": "invalid"}))
		return
	}

	if err := h.svc.Grant(r.Context(), libraryID, userID, req.CanWrite); err != nil {
		writeAccountError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, grantDTO{
		LibraryID: libraryID.String(),
		UserID:    userID.String(),
		CanWrite:  req.CanWrite,
	})
}

func (h *Accounts) Revoke(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	libraryID, err := uuid.Parse(chi.URLParam(r, "libraryID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"libraryId": "invalid"}))
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		problem.Write(w, r, problem.Validation(map[string]string{"userId": "invalid"}))
		return
	}

	if err := h.svc.Revoke(r.Context(), libraryID, userID); err != nil {
		writeAccountError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toGrantDTOs(grants []accounts.LibraryGrant) []grantDTO {
	out := make([]grantDTO, 0, len(grants))
	for _, g := range grants {
		out = append(out, grantDTO{
			LibraryID: g.LibraryID.String(),
			UserID:    g.UserID.String(),
			CanWrite:  g.CanWrite,
		})
	}
	return out
}

func claimsFrom(w http.ResponseWriter, r *http.Request) (auth.Claims, bool) {
	claims, ok := middleware.ClaimsFrom(r.Context())
	if !ok {
		problem.Write(w, r, problem.Unauthorized("authentication required"))
		return auth.Claims{}, false
	}
	return claims, true
}

func writeAccountError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, accounts.ErrNotFound):
		problem.Write(w, r, problem.NotFound("account not found"))

	case errors.Is(err, accounts.ErrLastAdmin):
		problem.Write(w, r, problem.Validation(map[string]string{
			"role": "this is the last administrator; promote another account first",
		}))

	case errors.Is(err, accounts.ErrSelfDemotion):
		problem.Write(w, r, problem.Validation(map[string]string{
			"userId": "self",
		}))

	case errors.Is(err, accounts.ErrInvalidRole):
		problem.Write(w, r, problem.Validation(map[string]string{"role": "one-of"}))

	case errors.Is(err, accounts.ErrWeakPassword):
		problem.Write(w, r, problem.Validation(map[string]string{
			"password": "at least 12 characters",
		}))

	case errors.Is(err, auth.ErrUserExists):
		problem.Write(w, r, problem.Validation(map[string]string{"username": "taken"}))

	case errors.Is(err, auth.ErrUsernameInvalid):
		problem.Write(w, r, problem.Validation(map[string]string{
			"username": "format",
		}))

	default:
		writeInternal(w, r, err)
	}
}
