package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"time"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/auth"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/middleware"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
)

// Auth expose les endpoints d'authentification.
type Auth struct {
	svc *auth.Service
}

func NewAuth(svc *auth.Service) *Auth { return &Auth{svc: svc} }

// ─── Représentations ─────────────────────────────────────────────────────────

type userDTO struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email,omitempty"`
	DisplayName string    `json:"displayName,omitempty"`
	Role        string    `json:"role"`
	Restricted  bool      `json:"restricted"`
}

func toUserDTO(u auth.User) userDTO {
	return userDTO{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        u.Role,
		Restricted:  u.Restricted,
	}
}

type tokensDTO struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
	DeviceID     uuid.UUID `json:"deviceId"`
	User         userDTO   `json:"user"`
}

func toTokensDTO(t auth.Tokens) tokensDTO {
	return tokensDTO{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresAt:    t.ExpiresAt,
		DeviceID:     t.DeviceID,
		User:         toUserDTO(t.User),
	}
}

// ─── Première installation ───────────────────────────────────────────────────

// Status indique l'état de l'instance, sans authentification.
//
// Sert à l'application web pour décider si elle affiche l'assistant de
// première installation ou l'écran de connexion.
func (h *Auth) Status(w http.ResponseWriter, r *http.Request) {
	needsSetup, err := h.svc.NeedsSetup(r.Context())
	if err != nil {
		problem.Write(w, r, problem.Internal())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needsSetup": needsSetup})
}

type setupRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Setup crée le premier administrateur.
//
// Ouvert sans authentification tant qu'aucun compte n'existe — c'est le seul
// moyen d'amorcer une instance neuve. Le service referme définitivement la
// porte dès qu'un compte est créé.
func (h *Auth) Setup(w http.ResponseWriter, r *http.Request) {
	var req setupRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	user, err := h.svc.Setup(r.Context(), req.Username, req.Email, req.Password)
	switch {
	case errors.Is(err, auth.ErrSetupClosed):
		problem.Write(w, r, problem.Forbidden("setup has already been completed"))
		return
	case errors.Is(err, auth.ErrPasswordTooShort):
		problem.Write(w, r, problem.Validation(map[string]string{
			"password": "must be at least 10 characters",
		}))
		return
	case errors.Is(err, auth.ErrUsernameInvalid):
		problem.Write(w, r, problem.Validation(map[string]string{"username": err.Error()}))
		return
	case errors.Is(err, auth.ErrUserExists):
		problem.Write(w, r, problem.Validation(map[string]string{"username": "already taken"}))
		return
	case err != nil:
		problem.Write(w, r, problem.Internal())
		return
	}

	writeJSON(w, http.StatusCreated, toUserDTO(user))
}

// ─── Connexion ───────────────────────────────────────────────────────────────

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`

	DeviceID   string `json:"deviceId,omitempty"`
	DeviceName string `json:"deviceName,omitempty"`
	Platform   string `json:"platform,omitempty"`
	AppVersion string `json:"appVersion,omitempty"`
}

func (h *Auth) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	params := auth.LoginParams{
		Username:   req.Username,
		Password:   req.Password,
		DeviceName: req.DeviceName,
		Platform:   req.Platform,
		AppVersion: req.AppVersion,
		UserAgent:  r.UserAgent(),
		IP:         clientIP(r),
	}
	if req.DeviceID != "" {
		if id, err := uuid.Parse(req.DeviceID); err == nil {
			params.DeviceID = id
		}
	}

	tokens, err := h.svc.Login(r.Context(), params)
	if err != nil {
		// Un seul message quelle que soit la cause : distinguer « compte
		// inconnu » de « mot de passe erroné » permettrait d'énumérer les
		// comptes existants.
		problem.Write(w, r, problem.Unauthorized("invalid username or password"))
		return
	}

	writeJSON(w, http.StatusOK, toTokensDTO(tokens))
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *Auth) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.RefreshToken == "" {
		problem.Write(w, r, problem.Validation(map[string]string{"refreshToken": "is required"}))
		return
	}

	tokens, err := h.svc.Refresh(r.Context(), req.RefreshToken, r.UserAgent(), clientIP(r))
	switch {
	case errors.Is(err, auth.ErrSessionRevoked):
		// Réutilisation détectée : toute la chaîne a été révoquée. Le client
		// doit repasser par une connexion complète.
		p := problem.Unauthorized("session revoked — please sign in again")
		p.Type = "https://boxincloud.dev/problems/session-revoked"
		problem.Write(w, r, p)
		return
	case err != nil:
		problem.Write(w, r, problem.Unauthorized("invalid or expired refresh token"))
		return
	}

	writeJSON(w, http.StatusOK, toTokensDTO(tokens))
}

func (h *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		problem.Write(w, r, problem.Internal())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// LogoutAll révoque toutes les sessions du compte courant.
func (h *Auth) LogoutAll(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFrom(r.Context())
	if !ok {
		problem.Write(w, r, problem.Unauthorized("authentication required"))
		return
	}

	revoked, err := h.svc.LogoutAll(r.Context(), claims.UserID)
	if err != nil {
		problem.Write(w, r, problem.Internal())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"revokedSessions": revoked})
}

// ─── Compte courant ──────────────────────────────────────────────────────────

func (h *Auth) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFrom(r.Context())
	if !ok {
		problem.Write(w, r, problem.Unauthorized("authentication required"))
		return
	}

	writeJSON(w, http.StatusOK, userDTO{
		ID:       claims.UserID,
		Username: claims.Username,
		Role:     claims.Role,
	})
}

type deviceDTO struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Platform   string    `json:"platform"`
	AppVersion string    `json:"appVersion,omitempty"`
	LastSeenAt time.Time `json:"lastSeenAt"`
	Current    bool      `json:"current"`
}

func (h *Auth) ListDevices(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFrom(r.Context())
	if !ok {
		problem.Write(w, r, problem.Unauthorized("authentication required"))
		return
	}

	devices, err := h.svc.ListDevices(r.Context(), claims.UserID)
	if err != nil {
		problem.Write(w, r, problem.Internal())
		return
	}

	out := make([]deviceDTO, 0, len(devices))
	for _, d := range devices {
		out = append(out, deviceDTO{
			ID:         d.ID,
			Name:       d.Name,
			Platform:   d.Platform,
			AppVersion: d.AppVersion,
			LastSeenAt: d.LastSeenAt,
			Current:    d.ID == claims.DeviceID,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": out})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// maxRequestBody borne la taille d'un corps JSON.
//
// Les corps traités ici sont des identifiants et des jetons : quelques
// centaines d'octets suffisent. La borne évite qu'une requête volumineuse
// mobilise la mémoire du serveur.
const maxRequestBody = 64 << 10

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxRequestBody))
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		problem.Write(w, r, problem.BadRequest("malformed JSON body"))
		return false
	}
	return true
}

// clientIP extrait l'adresse du pair.
//
// Volontairement basé sur RemoteAddr seul : les en-têtes X-Forwarded-For sont
// usurpables tant qu'aucun proxy de confiance n'est configuré, et cette adresse
// servira à limiter le débit sur l'authentification. Le support des proxys
// viendra explicitement, avec une liste d'adresses de confiance.
func clientIP(r *http.Request) *netip.Addr {
	addrPort, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		// Certaines configurations rendent RemoteAddr sans port.
		if addr, err := netip.ParseAddr(r.RemoteAddr); err == nil {
			return &addr
		}
		return nil
	}
	addr := addrPort.Addr()
	return &addr
}
