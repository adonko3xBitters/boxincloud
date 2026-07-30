package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/indexer"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
)

/*
Suppression et déplacement d'albums.

Deux opérations que rien ne permettait : un album mal nommé, en double ou rangé
au mauvais endroit y restait définitivement, à moins d'aller manipuler le bucket
à la main.

Les deux touchent au stockage ET au catalogue, et l'ordre des gestes compte :
c'est là que se joue la différence entre une erreur rattrapable et une perte.
*/

var (
	// ErrComicNotFound signale un album inexistant ou déjà retiré.
	ErrComicNotFound = errors.New("ingest : album introuvable")

	// ErrSameFolder signale un déplacement sans effet.
	ErrSameFolder = errors.New("ingest : l'album est déjà dans ce dossier")
)

// ManageRepository couvre ce dont la gestion a besoin du catalogue.
type ManageRepository interface {
	GetComic(ctx context.Context, id uuid.UUID) (indexer.Comic, error)

	// ExcludeComic retire l'album du catalogue en gardant sa ligne, et donc la
	// progression de lecture qui y est attachée.
	ExcludeComic(ctx context.Context, id uuid.UUID) error

	// PurgeComic efface la ligne, une fois le fichier supprimé du backend.
	PurgeComic(ctx context.Context, id uuid.UUID) error

	MoveComic(ctx context.Context, id uuid.UUID, objectKey, folderPath string) error
}

// DeleteParams décrit une suppression.
type DeleteParams struct {
	ComicID uuid.UUID

	// DeleteFile supprime aussi l'objet dans le backend.
	//
	// Faux par défaut, et c'est le bon défaut : retirer un album d'un catalogue
	// se rattrape, effacer un fichier non.
	DeleteFile bool
}

/*
Delete retire un album.

Sans le fichier : la ligne est marquée comme exclue plutôt que détruite. La
progression de lecture, les favoris et les notes y sont rattachés, et les
effacer priverait d'historique quelqu'un qui remettrait le fichier en place. Le
scan respecte cette exclusion — sans quoi l'album réapparaîtrait au parcours
suivant, sans que rien n'explique pourquoi.

Avec le fichier : l'objet est supprimé du backend, PUIS la ligne est effacée.
L'ordre importe. Effacer la ligne d'abord et échouer sur le backend laisserait
un fichier orphelin, invisible du catalogue mais bien présent — et qu'un scan
réintroduirait comme un nouvel album. L'ordre inverse, en cas d'échec, laisse
une ligne dont l'objet a disparu : c'est exactement ce que le scan sait traiter.
*/
func (s *Service) Delete(ctx context.Context, p DeleteParams) error {
	comic, err := s.manage.GetComic(ctx, p.ComicID)
	if err != nil {
		return ErrComicNotFound
	}

	// Retirer un album d'un dossier protégé le modifie tout autant que d'y en
	// ajouter un : la protection vaut dans les deux sens.
	if err := s.ensureWritable(ctx, comic.LibraryID, folderOfObjectKey(ctx, s, comic)); err != nil {
		return err
	}

	if !p.DeleteFile {
		return s.manage.ExcludeComic(ctx, p.ComicID)
	}

	lib, err := s.libraries.GetLibrary(ctx, comic.LibraryID)
	if err != nil {
		return err
	}
	provider, err := s.libraries.ProviderForLibrary(ctx, lib)
	if err != nil {
		return err
	}

	if err := provider.Delete(ctx, comic.ObjectKey); err != nil {
		return err
	}

	return s.manage.PurgeComic(ctx, p.ComicID)
}

// MoveParams décrit un déplacement.
type MoveParams struct {
	ComicID uuid.UUID

	// Folder est le dossier de destination, relatif au préfixe de la
	// bibliothèque. Vide pour la racine.
	Folder string
}

/*
Move range un album dans un autre dossier.

Le dossier d'un album n'est pas une propriété du catalogue : il découle de la
clé de l'objet dans le backend. Déplacer un album, c'est donc renommer l'objet,
et le catalogue suit.

L'objet est déplacé d'abord. Mettre le catalogue à jour en premier ferait
pointer la base vers une clé qui n'existe pas encore : le temps que le backend
réponde, toute lecture de page échouerait.
*/
func (s *Service) Move(ctx context.Context, p MoveParams) (string, error) {
	comic, err := s.manage.GetComic(ctx, p.ComicID)
	if err != nil {
		return "", ErrComicNotFound
	}

	lib, err := s.libraries.GetLibrary(ctx, comic.LibraryID)
	if err != nil {
		return "", err
	}

	name := path.Base(comic.ObjectKey)
	target := objectKey(lib.RootPrefix, p.Folder, name)

	if target == comic.ObjectKey {
		return "", ErrSameFolder
	}

	// Source et destination : sortir un album d'un dossier protégé le vide,
	// l'y faire entrer le modifie.
	if err := s.ensureWritable(ctx, lib.ID, indexer.FolderOf(comic.ObjectKey, lib.RootPrefix)); err != nil {
		return "", err
	}
	if err := s.ensureWritable(ctx, lib.ID, p.Folder); err != nil {
		return "", err
	}

	provider, err := s.libraries.ProviderForLibrary(ctx, lib)
	if err != nil {
		return "", err
	}

	if err := provider.Move(ctx, comic.ObjectKey, target); err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			return "", fmt.Errorf("%w : %s", ErrAlreadyExists, target)
		}
		return "", err
	}

	folder := indexer.FolderOf(target, lib.RootPrefix)
	s.registerFolder(ctx, lib.ID, folder)

	if err := s.manage.MoveComic(ctx, p.ComicID, target, folder); err != nil {
		// L'objet a bougé mais le catalogue l'ignore : il pointe une clé qui
		// n'existe plus. Un scan rétablira la cohérence ; le signaler permet
		// d'en informer l'utilisateur plutôt que de laisser un album muet.
		s.log.Error("album déplacé mais catalogue non mis à jour",
			slog.String("comic_id", p.ComicID.String()),
			slog.String("from", comic.ObjectKey),
			slog.String("to", target),
			slog.Any("err", err))
		return "", err
	}

	return folder, nil
}

// BulkDelete supprime une sélection.
//
// Les échecs individuels n'interrompent pas le lot : sur cinquante albums, un
// objet manquant ne doit pas empêcher les quarante-neuf autres d'être traités.
// Le compte des réussites est retourné, et les erreurs tracées.
func (s *Service) BulkDelete(ctx context.Context, ids []uuid.UUID, deleteFile bool) (int, error) {
	if len(ids) > MaxBulkItems {
		return 0, ErrTooManyItems
	}

	done := 0
	for _, id := range ids {
		if err := s.Delete(ctx, DeleteParams{ComicID: id, DeleteFile: deleteFile}); err != nil {
			s.log.Warn("suppression impossible",
				slog.String("comic_id", id.String()), slog.Any("err", err))
			continue
		}
		done++
	}
	return done, nil
}

// BulkMove range une sélection dans un dossier.
func (s *Service) BulkMove(ctx context.Context, ids []uuid.UUID, folder string) (int, error) {
	if len(ids) > MaxBulkItems {
		return 0, ErrTooManyItems
	}

	done := 0
	for _, id := range ids {
		if _, err := s.Move(ctx, MoveParams{ComicID: id, Folder: folder}); err != nil {
			// Un album déjà dans le dossier visé n'est pas un échec : le
			// résultat demandé est atteint.
			if errors.Is(err, ErrSameFolder) {
				done++
				continue
			}
			s.log.Warn("déplacement impossible",
				slog.String("comic_id", id.String()), slog.Any("err", err))
			continue
		}
		done++
	}
	return done, nil
}

// ErrTooManyItems borne une opération en lot.
var ErrTooManyItems = errors.New("ingest : trop d'éléments dans le lot")

// MaxBulkItems reprend la borne du catalogue : mille albums couvrent une
// sélection réelle sans bloquer la base plusieurs secondes.
const MaxBulkItems = 1000

// FolderOfKey extrait le dossier d'une clé, relatif à un préfixe.
//
// Exposé pour l'API, qui doit annoncer la destination réelle après déplacement :
// le dossier demandé est nettoyé, et celui qui en ressort peut différer.
func FolderOfKey(objectKey, rootPrefix string) string {
	return strings.TrimSuffix(indexer.FolderOf(objectKey, rootPrefix), "/")
}

// folderOfObjectKey retrouve le dossier d'un album à partir de sa clé.
//
// La bibliothèque est relue pour son préfixe : le dossier enregistré sur l'album
// pourrait avoir divergé de la clé réelle, et c'est la clé qui fait foi.
func folderOfObjectKey(ctx context.Context, s *Service, comic indexer.Comic) string {
	lib, err := s.libraries.GetLibrary(ctx, comic.LibraryID)
	if err != nil {
		return ""
	}
	return indexer.FolderOf(comic.ObjectKey, lib.RootPrefix)
}
