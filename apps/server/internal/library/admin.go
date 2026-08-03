package library

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

/*
Administration des backends et des bibliothèques.

Créer était possible ; modifier et supprimer ne l'étaient pas. Une configuration
qu'on ne peut que poser une fois oblige à repartir de zéro pour corriger un
endpoint mal tapé — et à perdre au passage tout ce qui s'y rattachait.
*/

var (
	// ErrBackendInUse refuse de supprimer un backend encore porteur.
	//
	// Sa suppression emporterait ses bibliothèques par cascade, et avec elles la
	// progression de lecture de tout le monde. Le refus est explicite pour que
	// l'ordre des opérations soit une décision, pas une découverte.
	ErrBackendInUse = errors.New("library : ce stockage porte encore des bibliothèques")

	// ErrLastBackend protège le dernier stockage déclaré.
	ErrLastBackend = errors.New("library : c'est le dernier stockage déclaré")
)

// ScanRun résume un parcours de bibliothèque.
type ScanRun struct {
	ID          uuid.UUID
	Status      string
	StartedAt   time.Time
	FinishedAt  *time.Time
	ObjectsSeen int
	Added       int
	Updated     int
	Removed     int
	Errors      int
	Detail      string
}

// AdminRepository couvre ce que l'administration ajoute au dépôt.
type AdminRepository interface {
	UpdateBackend(ctx context.Context, id uuid.UUID, p UpdateBackendParams) (Backend, error)
	DeleteBackend(ctx context.Context, id uuid.UUID) error
	CountLibrariesUsing(ctx context.Context, backendID uuid.UUID) (int64, error)
	SetDefaultBackend(ctx context.Context, id uuid.UUID) error

	UpdateLibrary(ctx context.Context, id uuid.UUID, name, rootPrefix *string) (Library, error)
	DeleteLibrary(ctx context.Context, id uuid.UUID) error

	ScanRuns(ctx context.Context, libraryID uuid.UUID, limit int32) ([]ScanRun, error)
}

// SetAdminRepository câble les opérations d'administration.
func (s *Service) SetAdminRepository(repo AdminRepository) { s.admin = repo }

// UpdateBackendParams décrit une modification de backend.
//
// Les champs nuls sont laissés tels quels. Les secrets en particulier : un
// administrateur qui corrige un endpoint ne doit pas avoir à retaper ses clés,
// et ne le pourrait de toute façon pas — elles ne ressortent jamais de la base.
type UpdateBackendParams struct {
	Name       *string
	Config     map[string]string
	Secrets    map[string]string
	ReadOnly   *bool
	SecretsEnc []byte
}

/*
UpdateBackend modifie un backend après avoir vérifié qu'il répond toujours.

La vérification porte sur la configuration RÉSULTANTE, pas sur celle envoyée :
une modification partielle doit être jointe avec ce qu'elle laisse en place, sans
quoi on validerait une configuration qui n'existera jamais.
*/
func (s *Service) UpdateBackend(
	ctx context.Context, id uuid.UUID, p UpdateBackendParams,
) (Backend, error) {
	current, currentSecretsEnc, err := s.repo.GetBackend(ctx, id)
	if err != nil {
		return Backend{}, err
	}

	config := current.Config
	if p.Config != nil {
		config = p.Config
	}

	secrets, err := s.openSecrets(currentSecretsEnc)
	if err != nil {
		return Backend{}, err
	}
	if p.Secrets != nil {
		secrets = p.Secrets

		sealed, err := s.sealSecrets(p.Secrets)
		if err != nil {
			return Backend{}, err
		}
		p.SecretsEnc = sealed
	}

	readOnly := current.ReadOnly
	if p.ReadOnly != nil {
		readOnly = *p.ReadOnly
	}

	provider, err := buildProvider(current.Kind, config, secrets, readOnly)
	if err != nil {
		return Backend{}, err
	}
	if err := provider.Ping(ctx); err != nil {
		return Backend{}, fmt.Errorf("%w : %w", ErrBackendUnreachable, err)
	}

	updated, err := s.admin.UpdateBackend(ctx, id, p)
	if err != nil {
		return Backend{}, err
	}

	// Le ping vient de réussir : l'enregistrer évite qu'un stockage
	// fraîchement vérifié s'affiche « jamais testé ».
	if err := s.repo.SetBackendStatus(ctx, id, "ok", ""); err != nil {
		s.log.Debug("état du stockage non enregistré", "err", err)
	} else {
		updated.Status = "ok"
	}

	// Le provider en cache porte l'ancienne configuration : le remplacer évite
	// qu'une lecture de page continue de viser l'ancien endpoint.
	s.mu.Lock()
	s.providers[id] = provider
	s.mu.Unlock()

	return updated, nil
}

/*
DeleteBackend supprime un stockage.

Refusé tant qu'une bibliothèque s'y appuie : la cascade emporterait albums,
dossiers, progression de lecture et partages. Le refus force à supprimer les
bibliothèques d'abord, ce qui rend la perte visible et volontaire.

Les fichiers du backend ne sont jamais touchés — boxincloud oublie un stockage,
il ne le vide pas.
*/
func (s *Service) DeleteBackend(ctx context.Context, id uuid.UUID) error {
	count, err := s.admin.CountLibrariesUsing(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w : %d bibliothèque(s)", ErrBackendInUse, count)
	}

	if err := s.admin.DeleteBackend(ctx, id); err != nil {
		return err
	}

	s.Invalidate(id)
	return nil
}

// SetDefaultBackend désigne le stockage utilisé par défaut.
func (s *Service) SetDefaultBackend(ctx context.Context, id uuid.UUID) error {
	if _, _, err := s.repo.GetBackend(ctx, id); err != nil {
		return err
	}
	return s.admin.SetDefaultBackend(ctx, id)
}

/*
UpdateLibrary modifie une bibliothèque.

Le préfixe racine est modifiable, et c'est délibérément une opération lourde de
conséquences : les albums déjà indexés pointent des clés construites avec
l'ancien. Ils ne sont PAS déplacés — le changement décrit où chercher désormais,
et un nouveau parcours reconstruit le catalogue. L'interface doit le dire.
*/
func (s *Service) UpdateLibrary(
	ctx context.Context, id uuid.UUID, name, rootPrefix *string,
) (Library, error) {
	if _, err := s.repo.GetLibrary(ctx, id); err != nil {
		return Library{}, err
	}
	return s.admin.UpdateLibrary(ctx, id, name, rootPrefix)
}

/*
DeleteLibrary supprime une bibliothèque et tout ce qui s'y rattache.

Albums, dossiers, progression de lecture, favoris, notes et partages disparaissent
par cascade. Les FICHIERS du stockage restent intacts : recréer la bibliothèque
sur le même préfixe et relancer un parcours les retrouve tous — mais l'historique
de lecture, lui, ne revient pas.
*/
func (s *Service) DeleteLibrary(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repo.GetLibrary(ctx, id); err != nil {
		return err
	}
	return s.admin.DeleteLibrary(ctx, id)
}

// ScanRuns retourne l'historique des parcours d'une bibliothèque.
//
// C'est le seul endroit où l'on voit pourquoi un parcours a échoué : sans lui,
// un scan en erreur ne se manifeste que par une bibliothèque qui ne se remplit
// pas, sans indice.
func (s *Service) ScanRuns(ctx context.Context, libraryID uuid.UUID, limit int32) ([]ScanRun, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	return s.admin.ScanRuns(ctx, libraryID, limit)
}
