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
	"github.com/adonko3xBitters/boxincloud/server/internal/library"
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

	// RefreshSeries recalcule les compteurs et élague les séries vidées.
	RefreshSeries(ctx context.Context, libraryID uuid.UUID) error

	// RefreshLibraryCount recompte les albums visibles d'une bibliothèque.
	RefreshLibraryCount(ctx context.Context, libraryID uuid.UUID) error

	MoveComic(ctx context.Context, id uuid.UUID, objectKey, folderPath string) error

	// MoveComicToLibrary rattache un album à une autre bibliothèque en même
	// temps qu'il change de clé. Les deux dans la même écriture : un album ne
	// doit jamais appartenir à une bibliothèque tout en désignant un objet
	// rangé chez une autre.
	MoveComicToLibrary(ctx context.Context, id, libraryID uuid.UUID, objectKey, folderPath string) error
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
		if err := s.manage.ExcludeComic(ctx, p.ComicID); err != nil {
			return err
		}
		s.refreshSeries(ctx, comic.LibraryID)
		return nil
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

	if err := s.manage.PurgeComic(ctx, p.ComicID); err != nil {
		return err
	}

	s.removeHydrated(ctx, p.ComicID)
	s.refreshSeries(ctx, comic.LibraryID)
	return nil
}

/*
removeHydrated efface l'archive normalisée d'un album purgé.

Un CBR ou un PDF laisse dans le cache dérivé une copie CBZ de plusieurs
centaines de méga-octets. Purger l'album sans elle la rendrait orpheline : plus
rien ne la référencerait, et l'éviction du cache ne la connaît pas — elle n'y
est pas inscrite, précisément parce qu'elle ne doit pas être évincée tant que
l'album vit.

L'échec est journalisé sans faire échouer la suppression. L'album est déjà parti
de la base et du stockage ; refuser l'opération pour un fichier de cache
laisserait un état plus incohérent que celui qu'on cherche à éviter.
*/
func (s *Service) removeHydrated(ctx context.Context, comicID uuid.UUID) {
	if s.derived == nil {
		return
	}

	key := indexer.HydratedKey(comicID)
	if err := s.derived.Delete(ctx, key); err != nil {
		s.log.Warn("archive hydratée non supprimée",
			slog.String("key", key), slog.Any("err", err))
	}
}

/*
refreshSeries recalcule les compteurs et retire les séries vidées.

Sans cela, une série dont on supprime le dernier tome continue de s'afficher
dans la barre latérale avec un compteur qui ne correspond à rien. Cliquer dessus
donne une liste vide, ce qui ressemble à un défaut d'affichage alors que c'est
une donnée périmée.

L'échec est tracé mais n'annule pas la suppression : l'album est bel et bien
parti, et un prochain parcours remettra les compteurs d'aplomb.
*/
func (s *Service) refreshSeries(ctx context.Context, libraryID uuid.UUID) {
	if err := s.manage.RefreshSeries(ctx, libraryID); err != nil {
		s.log.Warn("compteurs de séries non rafraîchis",
			slog.String("library_id", libraryID.String()), slog.Any("err", err))
	}

	// Le compteur de la bibliothèque est une colonne stockée, et il n'était
	// rafraîchi qu'en fin de parcours. Supprimer un album le laissait figé : la
	// barre latérale annonçait vingt-et-un albums devant une grille vide,
	// jusqu'au prochain scan.
	if err := s.manage.RefreshLibraryCount(ctx, libraryID); err != nil {
		s.log.Warn("compteur de bibliothèque non rafraîchi",
			slog.String("library_id", libraryID.String()), slog.Any("err", err))
	}
}

// MoveParams décrit un déplacement.
type MoveParams struct {
	ComicID uuid.UUID

	// Folder est le dossier de destination, relatif au préfixe de la
	// bibliothèque. Vide pour la racine.
	Folder string

	// LibraryID désigne une autre bibliothèque de destination. Nul pour rester
	// dans la même, qui est le cas courant.
	//
	// Changer de bibliothèque peut changer d'espace de stockage : les octets
	// transitent alors par le serveur, faute de copie possible entre deux
	// backends distincts.
	LibraryID *uuid.UUID
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

	source, err := s.libraries.GetLibrary(ctx, comic.LibraryID)
	if err != nil {
		return "", err
	}

	destination := source
	if p.LibraryID != nil && *p.LibraryID != comic.LibraryID {
		destination, err = s.libraries.GetLibrary(ctx, *p.LibraryID)
		if err != nil {
			return "", err
		}
	}

	name := path.Base(comic.ObjectKey)
	target := objectKey(destination.RootPrefix, p.Folder, name)

	if destination.ID == source.ID && target == comic.ObjectKey {
		return "", ErrSameFolder
	}

	// Source et destination : sortir un album d'un dossier protégé le vide,
	// l'y faire entrer le modifie. Les deux bibliothèques sont consultées
	// séparément — un dossier verrouillé chez l'une ne dit rien de l'autre.
	if err := s.ensureWritable(ctx, source.ID, indexer.FolderOf(comic.ObjectKey, source.RootPrefix)); err != nil {
		return "", err
	}
	if err := s.ensureWritable(ctx, destination.ID, p.Folder); err != nil {
		return "", err
	}

	if err := s.relocate(ctx, source, destination, comic.ObjectKey, target); err != nil {
		return "", err
	}

	folder := indexer.FolderOf(target, destination.RootPrefix)
	s.registerFolder(ctx, destination.ID, folder)

	update := func() error {
		if destination.ID == source.ID {
			return s.manage.MoveComic(ctx, p.ComicID, target, folder)
		}
		return s.manage.MoveComicToLibrary(ctx, p.ComicID, destination.ID, target, folder)
	}

	if err := update(); err != nil {
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

	// Les compteurs des DEUX bibliothèques bougent : l'une perd un tome, l'autre
	// en gagne un. N'en rafraîchir qu'une laisserait une série fantôme.
	if destination.ID != source.ID {
		s.refreshSeries(ctx, source.ID)
		s.refreshSeries(ctx, destination.ID)
	}

	return folder, nil
}

/*
relocate déplace les octets, par le chemin le moins coûteux disponible.

Dans un même backend, la copie se fait côté serveur : ranger une intégrale de
cinq cents méga-octets dans un autre dossier ne doit pas la faire transiter par
nous. C'est la raison d'être de `Provider.Move`.

Entre deux backends distincts, cette copie n'existe pas — un MinIO ne sait pas
copier depuis un Backblaze. Les octets passent alors par le serveur, en flux :
lire, écrire, puis effacer la source. L'ordre compte. Effacer d'abord perdrait
le fichier si l'écriture échouait ; effacer en dernier laisse au pire un doublon,
qu'un parcours de la bibliothèque d'origine signalera.
*/
func (s *Service) relocate(
	ctx context.Context,
	source, destination library.Library,
	from, to string,
) error {
	sourceProvider, err := s.libraries.ProviderForLibrary(ctx, source)
	if err != nil {
		return err
	}

	if source.BackendID == destination.BackendID {
		if err := sourceProvider.Move(ctx, from, to); err != nil {
			if errors.Is(err, storage.ErrAlreadyExists) {
				return fmt.Errorf("%w : %s", ErrAlreadyExists, to)
			}
			return err
		}
		return nil
	}

	destinationProvider, err := s.libraries.ProviderForLibrary(ctx, destination)
	if err != nil {
		return err
	}

	info, err := sourceProvider.Stat(ctx, from)
	if err != nil {
		return err
	}

	reader, err := sourceProvider.Open(ctx, from)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	if err := destinationProvider.Write(ctx, to, reader, info.Size, ""); err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			return fmt.Errorf("%w : %s", ErrAlreadyExists, to)
		}
		return err
	}

	if err := sourceProvider.Delete(ctx, from); err != nil {
		// La copie a réussi : l'album est lisible à sa nouvelle place. La source
		// restée derrière est un doublon, pas une perte — un parcours de la
		// bibliothèque d'origine le fera réapparaître, et l'utilisateur pourra
		// le supprimer. Échouer ici annulerait un déplacement qui a fonctionné.
		s.log.Warn("source non supprimée après déplacement entre backends",
			slog.String("from", from), slog.Any("err", err))
	}

	return nil
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
func (s *Service) BulkMove(
	ctx context.Context,
	ids []uuid.UUID,
	folder string,
	libraryID *uuid.UUID,
) (int, error) {
	if len(ids) > MaxBulkItems {
		return 0, ErrTooManyItems
	}

	done := 0
	for _, id := range ids {
		params := MoveParams{ComicID: id, Folder: folder, LibraryID: libraryID}
		if _, err := s.Move(ctx, params); err != nil {
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
