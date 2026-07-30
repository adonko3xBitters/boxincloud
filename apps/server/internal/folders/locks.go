package folders

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/auth"
)

/*
Verrouillage des dossiers.

Deux verrous indépendants, parce qu'ils répondent à deux besoins distincts.

La LECTURE SEULE protège d'une fausse manœuvre. Le dossier reste parfaitement
visible ; on ne peut simplement plus le renommer, le déplacer, y déposer un
fichier ni en supprimer un. C'est le verrou d'une collection qu'on a fini de
ranger, et il s'hérite : protéger « BD » protège tout ce qu'il contient.

Le CODE D'ACCÈS masque. Le dossier et son contenu disparaissent des listages,
de la recherche et de l'accès direct tant que le code n'a pas été saisi. C'est le
verrou d'un serveur partagé, où tout le monde n'a pas à voir toute la
bibliothèque.

Ils se cumulent librement : masqué sans être protégé, protégé sans être masqué.
*/

var (
	// ErrReadOnly signale une écriture dans un dossier protégé.
	ErrReadOnly = errors.New("folders : dossier en lecture seule")

	// ErrWrongCode signale un code d'accès erroné.
	ErrWrongCode = errors.New("folders : code d'accès incorrect")

	// ErrNotLocked signale un déverrouillage sur un dossier sans code.
	ErrNotLocked = errors.New("folders : ce dossier n'a pas de code d'accès")

	// ErrCodeTooShort signale un code trop court.
	ErrCodeTooShort = errors.New("folders : code d'accès trop court")
)

/*
MinCodeLength est la longueur minimale d'un code d'accès.

Quatre, pas douze. Ce n'est pas un mot de passe de compte : il ne protège pas
l'accès au serveur mais la visibilité d'un dossier pour quelqu'un qui est déjà
connecté. Exiger douze caractères pousserait à le noter à côté, ce qui protège
moins qu'un code court réellement retenu.
*/
const MinCodeLength = 4

/*
UnlockTTL borne la durée d'un déverrouillage.

Deux heures : de quoi lire une soirée sans ressaisir, assez court pour qu'un
poste laissé ouvert se referme de lui-même. Le déverrouillage est enregistré par
compte, donc révocable — c'est la moitié de l'intérêt d'un code.
*/
const UnlockTTL = 2 * time.Hour

// Lock décrit l'état de verrouillage d'un dossier.
type Lock struct {
	ReadOnly    bool
	HasCode     bool
	UnlockedFor *time.Time
}

// LockedFolder est un dossier masqué, avec l'échéance de son déverrouillage.
type LockedFolder struct {
	ID            uuid.UUID
	LibraryID     uuid.UUID
	Path          string
	UnlockedUntil *time.Time
}

// LockRepository porte les verrous.
type LockRepository interface {
	SetReadOnly(ctx context.Context, libraryID uuid.UUID, path string, readOnly bool) (Folder, error)
	SetAccessCode(ctx context.Context, libraryID uuid.UUID, path string, hash *string) (Folder, error)
	AccessCode(ctx context.Context, libraryID uuid.UUID, path string) (uuid.UUID, *string, error)

	LockedFolders(ctx context.Context, userID uuid.UUID, libraryIDs []uuid.UUID) ([]LockedFolder, error)
	Unlock(ctx context.Context, userID, folderID uuid.UUID, until time.Time) error
	Relock(ctx context.Context, userID, folderID uuid.UUID) error
	RevokeUnlocks(ctx context.Context, folderID uuid.UUID) error

	TreeReadOnly(ctx context.Context, libraryID uuid.UUID, path string) (bool, error)
}

// SetLockRepository câble le dépôt des verrous.
func (s *Service) SetLockRepository(repo LockRepository) { s.locks = repo }

// ─── Lecture seule ───────────────────────────────────────────────────────────

// SetReadOnly protège ou déprotège un dossier.
func (s *Service) SetReadOnly(
	ctx context.Context, libraryID uuid.UUID, path string, readOnly bool,
) (Folder, error) {
	clean := NormalizePath(path)
	if clean == "" {
		return Folder{}, ErrRootImmutable
	}
	if _, err := s.repo.Get(ctx, libraryID, clean); err != nil {
		return Folder{}, err
	}
	return s.locks.SetReadOnly(ctx, libraryID, clean, readOnly)
}

/*
EnsureWritable refuse l'écriture dans un dossier protégé.

La protection s'hérite : le dossier lui-même ou n'importe lequel de ses ancêtres
suffit à interdire. C'est ce qu'on attend de « verrouiller BD » — sans quoi il
faudrait protéger chaque sous-dossier un par un, et le premier oublié annulerait
tout l'effort.
*/
func (s *Service) EnsureWritable(ctx context.Context, libraryID uuid.UUID, path string) error {
	if s.locks == nil {
		return nil
	}

	locked, err := s.locks.TreeReadOnly(ctx, libraryID, NormalizePath(path))
	if err != nil {
		return err
	}
	if locked {
		return fmt.Errorf("%w : %s", ErrReadOnly, path)
	}
	return nil
}

// ─── Code d'accès ────────────────────────────────────────────────────────────

/*
SetAccessCode pose ou retire le code d'un dossier.

Un code vide le retire. Dans les deux cas, les déverrouillages en cours sont
révoqués : un accès obtenu avec l'ancien code ne doit pas survivre au nouveau, et
retirer un code ne doit pas laisser traîner des autorisations devenues sans
objet.
*/
func (s *Service) SetAccessCode(
	ctx context.Context, libraryID uuid.UUID, path, code string,
) (Folder, error) {
	clean := NormalizePath(path)
	if clean == "" {
		return Folder{}, ErrRootImmutable
	}

	folder, err := s.repo.Get(ctx, libraryID, clean)
	if err != nil {
		return Folder{}, err
	}

	var hash *string
	if code != "" {
		if len([]rune(code)) < MinCodeLength {
			return Folder{}, fmt.Errorf("%w : %d caractères minimum", ErrCodeTooShort, MinCodeLength)
		}
		hashed, err := auth.HashSecret(code)
		if err != nil {
			return Folder{}, err
		}
		hash = &hashed
	}

	updated, err := s.locks.SetAccessCode(ctx, libraryID, clean, hash)
	if err != nil {
		return Folder{}, err
	}

	if err := s.locks.RevokeUnlocks(ctx, folder.ID); err != nil {
		return Folder{}, err
	}
	return updated, nil
}

/*
Unlock vérifie le code et ouvre le dossier pour ce compte.

Le déverrouillage n'est pas porté par le jeton d'accès : celui-ci est
autoporteur, donc ni révocable ni modifiable une fois émis. Il est enregistré en
base, avec une échéance.
*/
func (s *Service) Unlock(
	ctx context.Context, userID, libraryID uuid.UUID, path, code string,
) (time.Time, error) {
	clean := NormalizePath(path)

	folderID, hash, err := s.locks.AccessCode(ctx, libraryID, clean)
	if err != nil {
		return time.Time{}, err
	}
	if hash == nil {
		return time.Time{}, ErrNotLocked
	}

	if err := auth.VerifyPassword(code, *hash); err != nil {
		return time.Time{}, ErrWrongCode
	}

	until := time.Now().Add(UnlockTTL)
	if err := s.locks.Unlock(ctx, userID, folderID, until); err != nil {
		return time.Time{}, err
	}
	return until, nil
}

// Relock referme un dossier avant l'échéance.
func (s *Service) Relock(ctx context.Context, userID, libraryID uuid.UUID, path string) error {
	folderID, _, err := s.locks.AccessCode(ctx, libraryID, NormalizePath(path))
	if err != nil {
		return err
	}
	return s.locks.Relock(ctx, userID, folderID)
}

/*
LockedPaths retourne les dossiers masqués pour ce compte.

C'est la fonction que le catalogue consulte à chaque listage. Elle ne retourne
que les dossiers dont le code n'a PAS été saisi : un dossier déverrouillé se
comporte exactement comme un dossier ordinaire.
*/
func (s *Service) LockedPaths(
	ctx context.Context, userID uuid.UUID, libraryIDs []uuid.UUID,
) ([]string, error) {
	if s.locks == nil || len(libraryIDs) == 0 {
		return []string{}, nil
	}

	list, err := s.locks.LockedFolders(ctx, userID, libraryIDs)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(list))
	for _, folder := range list {
		if folder.UnlockedUntil != nil {
			continue
		}
		out = append(out, folder.Path)
	}
	return out, nil
}

// LocksOf retourne l'état de verrouillage des dossiers d'une bibliothèque.
//
// Sert à l'affichage : un cadenas dans l'arborescence, et la distinction entre
// un dossier masqué qu'on a ouvert et un dossier qu'on n'a jamais vu.
func (s *Service) LocksOf(
	ctx context.Context, userID uuid.UUID, libraryIDs []uuid.UUID,
) (map[string]Lock, error) {
	if s.locks == nil || len(libraryIDs) == 0 {
		return map[string]Lock{}, nil
	}

	list, err := s.locks.LockedFolders(ctx, userID, libraryIDs)
	if err != nil {
		return nil, err
	}

	out := make(map[string]Lock, len(list))
	for _, folder := range list {
		out[folder.Path] = Lock{HasCode: true, UnlockedFor: folder.UnlockedUntil}
	}
	return out, nil
}

// hiddenUnder indique si un chemin tombe sous l'un des dossiers masqués.
func hiddenUnder(path string, locked []string) bool {
	for _, prefix := range locked {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
