// Package accounts administre les comptes et leurs accès aux bibliothèques.
//
// Séparé de `auth`, qui répond à « qui es-tu ». Ici on répond à « qui a le
// droit de quoi » : créer un compte, changer un rôle, ouvrir une bibliothèque à
// quelqu'un. Deux préoccupations distinctes, et les garder séparées permettra
// de restreindre l'une sans toucher l'autre.
package accounts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/auth"
)

var (
	// ErrNotFound signale un compte inexistant ou déjà supprimé.
	ErrNotFound = errors.New("accounts : compte introuvable")

	// ErrLastAdmin protège l'instance contre sa propre mise hors service.
	//
	// Une instance sans administrateur ne peut plus être administrée du tout :
	// ni créer de compte, ni déclarer un stockage, ni rendre à quiconque le rôle
	// perdu. Il n'existe aucun chemin de retour depuis l'interface.
	ErrLastAdmin = errors.New("accounts : c'est le dernier administrateur")

	// ErrSelfDemotion signale une tentative de se retirer ses propres droits.
	ErrSelfDemotion = errors.New("accounts : on ne se retire pas ses propres droits")

	// ErrInvalidRole signale un rôle inconnu.
	ErrInvalidRole = errors.New("accounts : rôle inconnu")

	// ErrUsernameTaken signale un identifiant déjà pris.
	ErrUsernameTaken = errors.New("accounts : identifiant déjà utilisé")

	// ErrWeakPassword signale un mot de passe trop court.
	ErrWeakPassword = errors.New("accounts : mot de passe trop court")
)

// MinPasswordLength est la longueur minimale d'un mot de passe.
//
// Douze plutôt qu'une exigence de composition : la longueur protège mieux que
// les majuscules et les chiffres imposés, qui poussent surtout à des variantes
// prévisibles du même mot.
const MinPasswordLength = 12

// Roles énumère les rôles acceptés.
var Roles = []string{"admin", "user"}

// Account est la vue applicative d'un compte.
//
// Le hachage du mot de passe n'y figure pas et ne doit jamais y figurer.
type Account struct {
	ID           uuid.UUID
	Username     string
	Email        string
	Role         string
	DisplayName  string
	Restricted   bool
	MaxAgeRating *int16
	LastLoginAt  *string
	CreatedAt    string
}

// LibraryGrant est un accès accordé à un compte sur une bibliothèque.
type LibraryGrant struct {
	LibraryID uuid.UUID
	UserID    uuid.UUID
	CanWrite  bool
}

// Repository est ce dont le service a besoin de la base.
type Repository interface {
	List(ctx context.Context) ([]Account, error)
	Get(ctx context.Context, id uuid.UUID) (Account, error)
	CountAdmins(ctx context.Context) (int64, error)

	UpdateProfile(ctx context.Context, id uuid.UUID, displayName, email *string) (Account, error)
	SetRole(ctx context.Context, id uuid.UUID, role string) (Account, error)
	SetRestriction(ctx context.Context, id uuid.UUID, restricted bool, maxAgeRating *int16) (Account, error)
	SetPassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	SoftDelete(ctx context.Context, id uuid.UUID) error

	GrantsForUser(ctx context.Context, userID uuid.UUID) ([]LibraryGrant, error)
	GrantsForLibrary(ctx context.Context, libraryID uuid.UUID) ([]LibraryGrant, error)
	Grant(ctx context.Context, libraryID, userID uuid.UUID, canWrite bool) error
	Revoke(ctx context.Context, libraryID, userID uuid.UUID) error
}

// Service administre les comptes.
type Service struct {
	repo Repository
	auth *auth.Service
	log  *slog.Logger
}

func NewService(repo Repository, authService *auth.Service, log *slog.Logger) *Service {
	return &Service{repo: repo, auth: authService, log: log}
}

// ─── Lecture ─────────────────────────────────────────────────────────────────

func (s *Service) List(ctx context.Context) ([]Account, error) { return s.repo.List(ctx) }

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Account, error) {
	return s.repo.Get(ctx, id)
}

// ─── Création ────────────────────────────────────────────────────────────────

// CreateParams décrit un compte à créer.
type CreateParams struct {
	Username    string
	Email       string
	Password    string
	Role        string
	DisplayName string
}

// Create ouvre un compte.
func (s *Service) Create(ctx context.Context, p CreateParams) (Account, error) {
	username := strings.TrimSpace(p.Username)
	if username == "" {
		return Account{}, fmt.Errorf("%w : identifiant vide", ErrInvalidRole)
	}
	if err := ValidatePassword(p.Password); err != nil {
		return Account{}, err
	}
	if !validRole(p.Role) {
		return Account{}, fmt.Errorf("%w : %q", ErrInvalidRole, p.Role)
	}

	user, err := s.auth.CreateUser(ctx, username, p.Email, p.Password, p.Role)
	if err != nil {
		return Account{}, err
	}

	if p.DisplayName != "" {
		display := p.DisplayName
		if _, err := s.repo.UpdateProfile(ctx, user.ID, &display, nil); err != nil {
			// Le compte existe : ne pas le perdre pour un nom d'affichage. La
			// trace suffit — l'administrateur pourra le corriger d'un clic.
			s.log.Warn("nom d'affichage non enregistré",
				slog.String("user_id", user.ID.String()), slog.Any("err", err))
		}
	}

	// Relu depuis la base plutôt qu'assemblé à la main : le compte créé doit
	// avoir exactement la même forme que celui que renverra la liste, dates
	// comprises. Un assemblage manuel oublie toujours un champ, et le client
	// reçoit alors un objet incomplet là où il en attend un entier.
	return s.repo.Get(ctx, user.ID)
}

// ─── Modification ────────────────────────────────────────────────────────────

// UpdateProfile change le nom affiché et l'adresse.
func (s *Service) UpdateProfile(ctx context.Context, id uuid.UUID, displayName, email *string) (Account, error) {
	return s.repo.UpdateProfile(ctx, id, displayName, email)
}

/*
SetRole change le rôle d'un compte.

Deux garde-fous. On ne se rétrograde pas soi-même : le geste est presque
toujours une erreur de ligne dans un tableau, et il coûterait l'accès à
l'administration. On ne rétrograde pas non plus le dernier administrateur, pour
la même raison, en pire — plus personne ne pourrait rien y faire.
*/
func (s *Service) SetRole(ctx context.Context, actorID, targetID uuid.UUID, role string) (Account, error) {
	if !validRole(role) {
		return Account{}, fmt.Errorf("%w : %q", ErrInvalidRole, role)
	}

	target, err := s.repo.Get(ctx, targetID)
	if err != nil {
		return Account{}, err
	}
	if target.Role == role {
		return target, nil
	}

	if target.Role == "admin" && role != "admin" {
		if actorID == targetID {
			return Account{}, ErrSelfDemotion
		}
		if err := s.ensureNotLastAdmin(ctx); err != nil {
			return Account{}, err
		}
	}

	updated, err := s.repo.SetRole(ctx, targetID, role)
	if err != nil {
		return Account{}, err
	}

	// Le rôle est relu à chaque requête depuis un cache court : le purger rend
	// le changement immédiat plutôt que différé de quelques secondes.
	s.auth.ForgetAccount(targetID)
	return updated, nil
}

// SetRestriction règle le profil restreint et la classification maximale.
func (s *Service) SetRestriction(
	ctx context.Context, id uuid.UUID, restricted bool, maxAgeRating *int16,
) (Account, error) {
	// Une limite d'âge sans restriction active ne s'appliquerait pas : le
	// filtrage du catalogue ne regarde que les profils restreints. Accepter les
	// deux séparément laisserait croire à une protection inexistante.
	if !restricted {
		maxAgeRating = nil
	}
	return s.repo.SetRestriction(ctx, id, restricted, maxAgeRating)
}

// ResetPassword remplace le mot de passe d'un compte.
//
// Les sessions ouvertes ne sont pas révoquées ici : c'est une action distincte,
// que l'administrateur déclenche s'il soupçonne un vol. Les lier ferait fermer
// toutes les sessions d'un utilisateur qui a simplement oublié son mot de passe.
func (s *Service) ResetPassword(ctx context.Context, id uuid.UUID, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	return s.repo.SetPassword(ctx, id, hash)
}

/*
Delete désactive un compte.

Suppression douce : la progression de lecture, les favoris et les notes restent
attachés à l'identifiant. Les effacer priverait d'historique quelqu'un dont le
compte serait rouvert, et fausserait les compteurs de lecture d'une bibliothèque
partagée.
*/
func (s *Service) Delete(ctx context.Context, actorID, targetID uuid.UUID) error {
	if actorID == targetID {
		return ErrSelfDemotion
	}

	target, err := s.repo.Get(ctx, targetID)
	if err != nil {
		return err
	}
	if target.Role == "admin" {
		if err := s.ensureNotLastAdmin(ctx); err != nil {
			return err
		}
	}

	if err := s.repo.SoftDelete(ctx, targetID); err != nil {
		return err
	}

	// Deux gestes, pour deux durées de vie. Les sessions portent les jetons de
	// rafraîchissement, révoqués ici. Le jeton d'accès, lui, est autoporteur et
	// resterait valable un quart d'heure : purger l'état mis en cache le fait
	// tomber à la requête suivante.
	s.auth.ForgetAccount(targetID)

	_, err = s.auth.LogoutAll(ctx, targetID)
	return err
}

// ─── Accès aux bibliothèques ─────────────────────────────────────────────────

// GrantsForUser liste les bibliothèques explicitement ouvertes à un compte.
func (s *Service) GrantsForUser(ctx context.Context, userID uuid.UUID) ([]LibraryGrant, error) {
	return s.repo.GrantsForUser(ctx, userID)
}

// GrantsForLibrary liste les comptes explicitement autorisés sur une bibliothèque.
func (s *Service) GrantsForLibrary(ctx context.Context, libraryID uuid.UUID) ([]LibraryGrant, error) {
	return s.repo.GrantsForLibrary(ctx, libraryID)
}

/*
Grant ouvre une bibliothèque à un compte.

Attention au modèle en vigueur : une bibliothèque SANS aucune autorisation
explicite est visible de tous. Le premier accès accordé la referme donc pour
tous les autres. C'est cohérent — on ne restreint que ce qu'on a désigné — mais
l'interface doit le dire, sans quoi le premier partage passerait pour une
suppression d'accès général.
*/
func (s *Service) Grant(ctx context.Context, libraryID, userID uuid.UUID, canWrite bool) error {
	if _, err := s.repo.Get(ctx, userID); err != nil {
		return err
	}
	return s.repo.Grant(ctx, libraryID, userID, canWrite)
}

// Revoke retire l'accès d'un compte à une bibliothèque.
func (s *Service) Revoke(ctx context.Context, libraryID, userID uuid.UUID) error {
	return s.repo.Revoke(ctx, libraryID, userID)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (s *Service) ensureNotLastAdmin(ctx context.Context) error {
	count, err := s.repo.CountAdmins(ctx)
	if err != nil {
		return err
	}
	if count <= 1 {
		return ErrLastAdmin
	}
	return nil
}

func validRole(role string) bool {
	for _, r := range Roles {
		if r == role {
			return true
		}
	}
	return false
}

// ValidatePassword applique la seule règle retenue : la longueur.
func ValidatePassword(password string) error {
	if len([]rune(password)) < MinPasswordLength {
		return fmt.Errorf("%w : %d caractères minimum", ErrWeakPassword, MinPasswordLength)
	}
	return nil
}
