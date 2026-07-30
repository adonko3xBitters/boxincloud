package folders

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

/*
Partage.

Deux mécanismes, dont un seul sort du périmètre authentifié.

Le PARTAGE ENTRE COMPTES reprend exactement le modèle des bibliothèques : un
dossier sans autorisation explicite est visible de tous ceux qui voient la
bibliothèque, et le premier accès accordé le referme pour les autres. Deux règles
différentes pour deux niveaux de la même arborescence auraient été impossibles à
retenir.

Le LIEN PUBLIC ouvre sans compte : qui a l'URL voit le contenu. C'est utile pour
prêter un album, et c'est aussi la seule porte de boxincloud qui ne demande rien.
Elle est donc étroite par construction — portée d'un dossier ou d'un album,
échéance obligatoire, révocable, et refusée sur une branche masquée par un code.
*/

var (
	// ErrShareNotFound signale un lien inconnu, révoqué ou expiré.
	//
	// Les trois cas se confondent volontairement : distinguer « expiré » de
	// « inexistant » confirmerait à un inconnu qu'un lien a existé.
	ErrShareNotFound = errors.New("folders : lien de partage introuvable")

	// ErrShareOnLockedFolder refuse un lien public sur une branche masquée.
	ErrShareOnLockedFolder = errors.New("folders : ce dossier est masqué par un code d'accès")

	// ErrShareExpiryRequired impose une échéance.
	ErrShareExpiryRequired = errors.New("folders : un lien public doit expirer")

	// ErrShareExpiryTooFar borne l'échéance.
	ErrShareExpiryTooFar = errors.New("folders : échéance trop lointaine")
)

/*
MaxShareTTL borne la durée d'un lien public.

Un an. Ce n'est pas une protection contre l'abus — c'est une protection contre
l'oubli : un lien de dix ans finit par circuler bien plus loin que ce que son
auteur avait en tête, et personne ne se souvient d'aller le fermer.
*/
const MaxShareTTL = 365 * 24 * time.Hour

// FolderGrant est un accès accordé à un compte sur un dossier.
type FolderGrant struct {
	FolderID    uuid.UUID
	UserID      uuid.UUID
	Username    string
	DisplayName string
	CanWrite    bool
}

// ShareLink est un lien public.
//
// Token n'est renseigné qu'à la création : seul son hachage est conservé, et il
// ne peut donc plus être relu ensuite.
type ShareLink struct {
	ID         uuid.UUID
	LibraryID  uuid.UUID
	FolderPath *string
	ComicID    *uuid.UUID
	Label      string
	CreatedBy  uuid.UUID
	ExpiresAt  time.Time
	LastUsedAt *time.Time
	UseCount   int64
	CreatedAt  time.Time

	Token string
}

// ShareRepository porte le partage.
type ShareRepository interface {
	GrantFolder(ctx context.Context, folderID, userID uuid.UUID, canWrite bool) error
	RevokeFolder(ctx context.Context, folderID, userID uuid.UUID) error
	FolderGrants(ctx context.Context, folderID uuid.UUID) ([]FolderGrant, error)
	RestrictedFolders(ctx context.Context, userID uuid.UUID, libraryIDs []uuid.UUID) ([]string, error)
	CanWriteFolder(ctx context.Context, userID, libraryID uuid.UUID, path string) (bool, error)

	CreateShare(ctx context.Context, link ShareLink, tokenHash []byte) (ShareLink, error)
	ShareByHash(ctx context.Context, tokenHash []byte) (ShareLink, error)
	ListShares(ctx context.Context, libraryIDs []uuid.UUID) ([]ShareLink, error)
	RevokeShare(ctx context.Context, id uuid.UUID) (int64, error)
	TouchShare(ctx context.Context, id uuid.UUID) error

	TreeHasAccessCode(ctx context.Context, libraryID uuid.UUID, path string) (bool, error)

	// ComicFolder retourne la bibliothèque et le dossier d'un album.
	ComicFolder(ctx context.Context, comicID uuid.UUID) (uuid.UUID, string, error)

	// ComicInScope dit si un album se trouve sous un chemin donné.
	ComicInScope(ctx context.Context, comicID, libraryID uuid.UUID, path string) (bool, error)

	// ComicsInScope liste les albums sous un chemin.
	ComicsInScope(ctx context.Context, libraryID uuid.UUID, path string) ([]uuid.UUID, error)
}

// SetShareRepository câble le dépôt de partage.
func (s *Service) SetShareRepository(repo ShareRepository) { s.shares = repo }

// ─── Partage entre comptes ───────────────────────────────────────────────────

/*
GrantFolder ouvre un dossier à un compte.

Attention au modèle, que l'interface doit relayer : un dossier SANS aucune
autorisation explicite est visible de tous ceux qui voient la bibliothèque. Le
PREMIER accès accordé le referme donc pour tout le monde d'autre — le geste
restreint autant qu'il ouvre.
*/
func (s *Service) GrantFolder(
	ctx context.Context, libraryID uuid.UUID, path string, userID uuid.UUID, canWrite bool,
) error {
	folder, err := s.repo.Get(ctx, libraryID, NormalizePath(path))
	if err != nil {
		return err
	}
	return s.shares.GrantFolder(ctx, folder.ID, userID, canWrite)
}

// RevokeFolder retire l'accès d'un compte à un dossier.
func (s *Service) RevokeFolder(
	ctx context.Context, libraryID uuid.UUID, path string, userID uuid.UUID,
) error {
	folder, err := s.repo.Get(ctx, libraryID, NormalizePath(path))
	if err != nil {
		return err
	}
	return s.shares.RevokeFolder(ctx, folder.ID, userID)
}

// FolderGrants liste les comptes autorisés sur un dossier.
func (s *Service) FolderGrants(
	ctx context.Context, libraryID uuid.UUID, path string,
) ([]FolderGrant, error) {
	folder, err := s.repo.Get(ctx, libraryID, NormalizePath(path))
	if err != nil {
		return nil, err
	}
	return s.shares.FolderGrants(ctx, folder.ID)
}

/*
HiddenPaths réunit tout ce que ce compte ne doit pas voir.

Les dossiers masqués par un code et les dossiers restreints à d'autres comptes
produisent la même chose : des chemins à retirer de la vue. Le catalogue les
consomme ensemble, sans avoir à savoir lequel des deux mécanismes les a produits
— c'est ce qui permet d'en ajouter un troisième plus tard sans le retoucher.
*/
func (s *Service) HiddenPaths(
	ctx context.Context, userID uuid.UUID, libraryIDs []uuid.UUID,
) ([]string, error) {
	locked, err := s.LockedPaths(ctx, userID, libraryIDs)
	if err != nil {
		return nil, err
	}

	if s.shares == nil {
		return locked, nil
	}

	restricted, err := s.shares.RestrictedFolders(ctx, userID, libraryIDs)
	if err != nil {
		return nil, err
	}

	return append(locked, restricted...), nil
}

// ─── Liens publics ───────────────────────────────────────────────────────────

// ShareParams décrit un lien à créer.
type ShareParams struct {
	LibraryID uuid.UUID

	// Exactement l'un des deux.
	FolderPath *string
	ComicID    *uuid.UUID

	Label     string
	CreatedBy uuid.UUID
	ExpiresAt time.Time
}

/*
CreateShare produit un lien public.

Le jeton est tiré au hasard sur trente-deux octets et n'est retourné QU'ICI :
seul son hachage est conservé, comme un mot de passe. Perdre le lien oblige donc
à en créer un autre, ce qui est le comportement voulu — un lien qu'on peut relire
en base est un lien qu'une fuite de base livre en clair.

Un lien sur une branche masquée par un code est refusé : publier ce qu'on vient
de cacher n'a aucun sens, et les deux réglages coexisteraient en se contredisant.
*/
func (s *Service) CreateShare(ctx context.Context, p ShareParams) (ShareLink, error) {
	if p.ExpiresAt.IsZero() {
		return ShareLink{}, ErrShareExpiryRequired
	}
	if p.ExpiresAt.Before(time.Now()) {
		return ShareLink{}, ErrShareExpiryRequired
	}
	if p.ExpiresAt.After(time.Now().Add(MaxShareTTL)) {
		return ShareLink{}, ErrShareExpiryTooFar
	}

	if (p.FolderPath == nil) == (p.ComicID == nil) {
		return ShareLink{}, fmt.Errorf("%w : un dossier OU un album", ErrInvalidName)
	}

	// La cible doit exister, et ne pas être masquée.
	scope := ""
	if p.FolderPath != nil {
		scope = NormalizePath(*p.FolderPath)
		if scope != "" {
			if _, err := s.repo.Get(ctx, p.LibraryID, scope); err != nil {
				return ShareLink{}, err
			}
		}
		p.FolderPath = &scope
	} else {
		libraryID, path, err := s.shares.ComicFolder(ctx, *p.ComicID)
		if err != nil {
			return ShareLink{}, err
		}
		// La bibliothèque vient de l'album, pas de l'appelant : un lien ne doit
		// pas pouvoir désigner un album d'ailleurs.
		p.LibraryID = libraryID
		scope = path
	}

	masked, err := s.shares.TreeHasAccessCode(ctx, p.LibraryID, scope)
	if err != nil {
		return ShareLink{}, err
	}
	if masked {
		return ShareLink{}, ErrShareOnLockedFolder
	}

	token, hash, err := newShareToken()
	if err != nil {
		return ShareLink{}, err
	}

	link, err := s.shares.CreateShare(ctx, ShareLink{
		ID:         uuid.Must(uuid.NewV7()),
		LibraryID:  p.LibraryID,
		FolderPath: p.FolderPath,
		ComicID:    p.ComicID,
		Label:      p.Label,
		CreatedBy:  p.CreatedBy,
		ExpiresAt:  p.ExpiresAt,
	}, hash)
	if err != nil {
		return ShareLink{}, err
	}

	link.Token = token
	return link, nil
}

/*
ResolveShare retrouve un lien à partir de son jeton.

La comparaison porte sur le hachage, pas sur le jeton : la base ne contient
jamais de quoi reconstituer une URL. Un lien révoqué ou expiré est traité comme
inexistant, et le dit avec la même erreur — distinguer les deux confirmerait à un
inconnu qu'un lien a existé.
*/
func (s *Service) ResolveShare(ctx context.Context, token string) (ShareLink, error) {
	if s.shares == nil || token == "" {
		return ShareLink{}, ErrShareNotFound
	}

	link, err := s.shares.ShareByHash(ctx, hashShareToken(token))
	if err != nil {
		return ShareLink{}, ErrShareNotFound
	}

	// L'usage est enregistré sans bloquer : savoir qu'un lien sert, et quand,
	// aide à décider de le révoquer.
	if err := s.shares.TouchShare(ctx, link.ID); err != nil {
		s.log.Debug("compteur de lien non mis à jour", "err", err)
	}
	return link, nil
}

// ListShares liste les liens actifs des bibliothèques visibles.
func (s *Service) ListShares(ctx context.Context, libraryIDs []uuid.UUID) ([]ShareLink, error) {
	if s.shares == nil || len(libraryIDs) == 0 {
		return []ShareLink{}, nil
	}
	return s.shares.ListShares(ctx, libraryIDs)
}

// RevokeShare ferme un lien immédiatement.
func (s *Service) RevokeShare(ctx context.Context, id uuid.UUID) error {
	affected, err := s.shares.RevokeShare(ctx, id)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrShareNotFound
	}
	return nil
}

/*
ShareCovers dit si un album entre dans la portée d'un lien.

Le contrôle est fait à chaque requête plutôt qu'une fois à l'ouverture : un lien
de dossier donne accès à ce que le dossier contient MAINTENANT, et un album qui
en sort doit cesser d'être accessible sans qu'il faille penser à révoquer.
*/
func (s *Service) ShareCovers(ctx context.Context, link ShareLink, comicID uuid.UUID) (bool, error) {
	if link.ComicID != nil {
		return *link.ComicID == comicID, nil
	}
	if link.FolderPath == nil {
		return false, nil
	}
	return s.shares.ComicInScope(ctx, comicID, link.LibraryID, *link.FolderPath)
}

// SharedComics liste les albums accessibles par un lien de dossier.
func (s *Service) SharedComics(ctx context.Context, link ShareLink) ([]uuid.UUID, error) {
	if link.FolderPath == nil {
		if link.ComicID == nil {
			return nil, ErrShareNotFound
		}
		return []uuid.UUID{*link.ComicID}, nil
	}
	return s.shares.ComicsInScope(ctx, link.LibraryID, *link.FolderPath)
}

// ─── Jetons ──────────────────────────────────────────────────────────────────

/*
newShareToken tire un jeton et son hachage.

Trente-deux octets d'aléa : un lien public n'a pas d'autre protection que
l'imprévisibilité de son adresse, et une URL circule. Encodé en base64 sans
remplissage pour rester lisible dans une barre d'adresse et un message.
*/
func newShareToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("folders : génération du jeton : %w", err)
	}

	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, hashShareToken(token), nil
}

// hashShareToken hache un jeton pour la comparaison en base.
//
// SHA-256 sans sel, et c'est correct ici : le jeton fait 256 bits d'aléa, donc
// aucune table précalculée ne l'atteint. Un argon2 par requête coûterait cher
// pour rien sur une valeur qui n'a rien d'un mot de passe humain.
func hashShareToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
