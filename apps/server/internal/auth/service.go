package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUserExists      = errors.New("auth : ce nom d'utilisateur est déjà pris")
	ErrSetupClosed     = errors.New("auth : l'installation est déjà effectuée")
	ErrSessionRevoked  = errors.New("auth : session révoquée")
	ErrForbidden       = errors.New("auth : droits insuffisants")
	ErrUsernameInvalid = errors.New("auth : nom d'utilisateur invalide")
)

// User est la vue applicative d'un compte.
type User struct {
	ID           uuid.UUID
	Username     string
	Email        string
	Role         string
	DisplayName  string
	Restricted   bool
	MaxAgeRating *int16
}

func (u User) IsAdmin() bool { return u.Role == "admin" }

// Device est un appareil connecté.
type Device struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Name       string
	Platform   string
	AppVersion string
	LastSeenAt time.Time
}

// Session est un refresh token actif.
type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	DeviceID  *uuid.UUID
	ParentID  *uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// Tokens est ce qu'une connexion réussie retourne.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	User         User
	DeviceID     uuid.UUID
}

// Repository est ce dont l'authentification a besoin de la persistance.
type Repository interface {
	CountUsers(ctx context.Context) (int64, error)
	CreateUser(ctx context.Context, u User, passwordHash string) (User, error)
	GetUser(ctx context.Context, id uuid.UUID) (User, error)
	GetUserByUsername(ctx context.Context, username string) (User, string, error)
	SetUserPassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	TouchUserLogin(ctx context.Context, id uuid.UUID) error

	UpsertDevice(ctx context.Context, d Device) (Device, error)
	ListDevices(ctx context.Context, userID uuid.UUID) ([]Device, error)
	DeleteDevice(ctx context.Context, userID, deviceID uuid.UUID) error
	DeviceExists(ctx context.Context, userID, deviceID uuid.UUID) (bool, error)
	RevokeDeviceSessions(ctx context.Context, userID, deviceID uuid.UUID) (int64, error)

	CreateSession(ctx context.Context, s Session, tokenHash []byte, userAgent string, ip *netip.Addr) (Session, error)
	GetSessionByTokenHash(ctx context.Context, hash []byte) (Session, error)
	RevokeSessionChain(ctx context.Context, id uuid.UUID) (int64, error)
	RevokeSession(ctx context.Context, id uuid.UUID) error
	RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) (int64, error)
}

// Service porte la logique d'authentification.
type Service struct {
	repo       Repository
	issuer     *TokenIssuer
	refreshTTL time.Duration
	log        *slog.Logger

	// liveness évite de relire la base à chaque requête pour savoir si le
	// compte porté par un jeton est toujours actif. Voir liveness.go.
	liveness *livenessCache

	// deviceLiveness fait de même pour les appareils : révoquer un téléphone
	// perdu doit couper l'accès sans attendre l'expiration de son jeton.
	deviceLiveness *livenessCache
}

func NewService(repo Repository, issuer *TokenIssuer, refreshTTL time.Duration, log *slog.Logger) *Service {
	return &Service{
		repo:           repo,
		issuer:         issuer,
		refreshTTL:     refreshTTL,
		log:            log,
		liveness:       newLivenessCache(),
		deviceLiveness: newLivenessCache(),
	}
}

// ─── Première installation ───────────────────────────────────────────────────

// NeedsSetup indique si l'instance n'a encore aucun compte.
//
// Tant que c'est le cas, la création du premier administrateur est ouverte sans
// authentification — c'est le seul moyen d'amorcer une instance neuve. Dès
// qu'un compte existe, la porte se ferme définitivement.
func (s *Service) NeedsSetup(ctx context.Context) (bool, error) {
	count, err := s.repo.CountUsers(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// Setup crée le premier administrateur.
func (s *Service) Setup(ctx context.Context, username, email, password string) (User, error) {
	needs, err := s.NeedsSetup(ctx)
	if err != nil {
		return User{}, err
	}
	if !needs {
		return User{}, ErrSetupClosed
	}
	return s.createUser(ctx, username, email, password, "admin")
}

// CreateUser crée un compte. Réservé aux administrateurs, contrôlé par
// l'appelant.
func (s *Service) CreateUser(ctx context.Context, username, email, password, role string) (User, error) {
	if role != "admin" && role != "user" {
		role = "user"
	}
	return s.createUser(ctx, username, email, password, role)
}

func (s *Service) createUser(ctx context.Context, username, email, password, role string) (User, error) {
	if err := validateUsername(username); err != nil {
		return User{}, err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}

	return s.repo.CreateUser(ctx, User{
		ID:          uuid.Must(uuid.NewV7()),
		Username:    username,
		Email:       email,
		Role:        role,
		DisplayName: username,
	}, hash)
}

// ─── Connexion ───────────────────────────────────────────────────────────────

// LoginParams décrit une tentative de connexion.
type LoginParams struct {
	Username string
	Password string

	// Appareil. DeviceID vide crée un nouvel appareil ; le client le conserve
	// ensuite pour que ses reconnexions soient rattachées au même.
	DeviceID   uuid.UUID
	DeviceName string
	Platform   string
	AppVersion string

	UserAgent string
	IP        *netip.Addr
}

// Login authentifie un utilisateur et ouvre une session.
func (s *Service) Login(ctx context.Context, p LoginParams) (Tokens, error) {
	user, hash, err := s.repo.GetUserByUsername(ctx, p.Username)
	if err != nil {
		// Compte inexistant : on effectue tout de même un hachage, pour que la
		// réponse prenne le même temps qu'avec un compte réel. Sans cela, la
		// durée permettrait d'énumérer les comptes.
		DummyVerify(p.Password)
		return Tokens{}, ErrInvalidCredentials
	}

	if err := VerifyPassword(p.Password, hash); err != nil {
		return Tokens{}, ErrInvalidCredentials
	}

	// Durcissement progressif : si le hachage date de paramètres plus faibles,
	// on le remplace à la volée, sans rien demander à l'utilisateur.
	if NeedsRehash(hash) {
		if newHash, err := HashPassword(p.Password); err == nil {
			if err := s.repo.SetUserPassword(ctx, user.ID, newHash); err != nil {
				s.log.Warn("réhachage du mot de passe impossible",
					slog.String("user_id", user.ID.String()), slog.Any("err", err))
			}
		}
	}

	device, err := s.resolveDevice(ctx, user.ID, p)
	if err != nil {
		return Tokens{}, err
	}

	if err := s.repo.TouchUserLogin(ctx, user.ID); err != nil {
		s.log.Warn("horodatage de connexion impossible", slog.Any("err", err))
	}

	return s.issueTokens(ctx, user, device.ID, nil, p.UserAgent, p.IP)
}

func (s *Service) resolveDevice(ctx context.Context, userID uuid.UUID, p LoginParams) (Device, error) {
	id := p.DeviceID
	if id == uuid.Nil {
		id = uuid.Must(uuid.NewV7())
	}

	name := p.DeviceName
	if name == "" {
		name = "Appareil inconnu"
	}
	platform := p.Platform
	if platform == "" {
		platform = "unknown"
	}

	return s.repo.UpsertDevice(ctx, Device{
		ID:         id,
		UserID:     userID,
		Name:       name,
		Platform:   platform,
		AppVersion: p.AppVersion,
	})
}

// ─── Rafraîchissement ────────────────────────────────────────────────────────

// Refresh échange un refresh token contre un nouveau couple de jetons.
//
// Le jeton présenté est révoqué et remplacé — rotation systématique. Si un
// jeton déjà révoqué est présenté, c'est qu'il a été volé : toute la chaîne de
// rotation est alors révoquée, ce qui déconnecte à la fois l'attaquant et
// l'utilisateur légitime. Une déconnexion est préférable à une session
// compromise silencieusement conservée.
func (s *Service) Refresh(ctx context.Context, refreshToken, userAgent string, ip *netip.Addr) (Tokens, error) {
	hash := HashRefreshToken(refreshToken)

	session, err := s.repo.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return Tokens{}, ErrTokenInvalid
	}

	if session.RevokedAt != nil {
		revoked, revErr := s.repo.RevokeSessionChain(ctx, session.ID)
		if revErr != nil {
			s.log.Error("révocation de chaîne impossible", slog.Any("err", revErr))
		}
		s.log.Warn("réutilisation d'un refresh token détectée — chaîne révoquée",
			slog.String("user_id", session.UserID.String()),
			slog.String("session_id", session.ID.String()),
			slog.Int64("sessions_révoquées", revoked),
		)
		return Tokens{}, ErrSessionRevoked
	}

	if time.Now().After(session.ExpiresAt) {
		return Tokens{}, ErrTokenExpired
	}

	user, err := s.repo.GetUser(ctx, session.UserID)
	if err != nil {
		return Tokens{}, ErrTokenInvalid
	}

	if err := s.repo.RevokeSession(ctx, session.ID); err != nil {
		return Tokens{}, fmt.Errorf("auth : rotation impossible : %w", err)
	}

	var deviceID uuid.UUID
	if session.DeviceID != nil {
		deviceID = *session.DeviceID
	}

	return s.issueTokens(ctx, user, deviceID, &session.ID, userAgent, ip)
}

func (s *Service) issueTokens(ctx context.Context, user User, deviceID uuid.UUID, parentID *uuid.UUID, userAgent string, ip *netip.Addr) (Tokens, error) {
	accessToken, expiresAt, err := s.issuer.Issue(user.ID, user.Username, user.Role, deviceID)
	if err != nil {
		return Tokens{}, err
	}

	refreshToken, refreshHash, err := NewRefreshToken()
	if err != nil {
		return Tokens{}, err
	}

	var devicePtr *uuid.UUID
	if deviceID != uuid.Nil {
		devicePtr = &deviceID
	}

	if _, err := s.repo.CreateSession(ctx, Session{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    user.ID,
		DeviceID:  devicePtr,
		ParentID:  parentID,
		ExpiresAt: time.Now().Add(s.refreshTTL),
	}, refreshHash, userAgent, ip); err != nil {
		return Tokens{}, err
	}

	return Tokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         user,
		DeviceID:     deviceID,
	}, nil
}

// ─── Déconnexion ─────────────────────────────────────────────────────────────

// Logout révoque la session correspondant à un refresh token.
//
// Idempotent : un jeton inconnu, déjà révoqué ou expiré n'est pas une erreur.
// Se déconnecter deux fois, ou se déconnecter avec un jeton périmé, doit
// aboutir au même état — un client hors ligne au moment de la déconnexion
// rejouera sa requête plus tard.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	session, err := s.repo.GetSessionByTokenHash(ctx, HashRefreshToken(refreshToken))
	if err != nil {
		if errors.Is(err, ErrTokenInvalid) {
			return nil
		}
		return err
	}
	return s.repo.RevokeSession(ctx, session.ID)
}

// LogoutAll révoque toutes les sessions d'un utilisateur.
func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.repo.RevokeAllUserSessions(ctx, userID)
}

// ─── Appareils ───────────────────────────────────────────────────────────────

func (s *Service) ListDevices(ctx context.Context, userID uuid.UUID) ([]Device, error) {
	return s.repo.ListDevices(ctx, userID)
}

/*
RevokeDevice coupe un appareil, tout de suite et pour de bon.

L'ordre des trois opérations n'est pas indifférent. Les sessions d'abord :
tant qu'un jeton de rafraîchissement vit, l'appareil se refait un jeton d'accès
et la révocation ne serait qu'un répit. L'appareil ensuite, ce qui le fait
disparaître de la liste. Le cache enfin, sans quoi le jeton d'accès en cours
resterait accepté pendant sa durée de vie de cache.

C'est le geste utile après un téléphone perdu. « Tout déconnecter » existe
aussi, mais oblige à se reconnecter partout, y compris là où on est en train de
lire — une punition collective pour un seul appareil égaré.
*/
func (s *Service) RevokeDevice(ctx context.Context, userID, deviceID uuid.UUID) (int64, error) {
	exists, err := s.repo.DeviceExists(ctx, userID, deviceID)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, ErrDeviceNotFound
	}

	revoked, err := s.repo.RevokeDeviceSessions(ctx, userID, deviceID)
	if err != nil {
		return 0, err
	}

	if err := s.repo.DeleteDevice(ctx, userID, deviceID); err != nil {
		return 0, err
	}

	s.ForgetDevice(deviceID)
	return revoked, nil
}

// ─── Jetons d'accès ──────────────────────────────────────────────────────────

// VerifyAccessToken valide un jeton d'accès et en retourne les claims.
func (s *Service) VerifyAccessToken(token string) (Claims, error) {
	return s.issuer.Verify(token)
}

// ─── Validation ──────────────────────────────────────────────────────────────

func validateUsername(username string) error {
	if len(username) < 3 || len(username) > 32 {
		return fmt.Errorf("%w : entre 3 et 32 caractères", ErrUsernameInvalid)
	}
	for _, r := range username {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '_', r == '-', r == '.':
		default:
			return fmt.Errorf("%w : lettres, chiffres, tiret, point et souligné uniquement", ErrUsernameInvalid)
		}
	}
	return nil
}
