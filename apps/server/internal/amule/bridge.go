package amule

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
)

/*
Le pont : un téléchargement terminé devient un album.

C'est ce qui justifie ce module DANS ce projet. Sans lui, boxincloud offrirait
une belle façade à aMule et rien de plus ; avec lui, un fichier trouvé sur le
réseau se retrouve dans la bibliothèque, indexé, lisible depuis le navigateur et
depuis le téléphone, sans qu'on ait rien recopié.

# La règle est portée par la catégorie

« Linux » reste sur disque, « BD » entre dans la bibliothèque. C'est le seul
découpage qui ait du sens : un client eD2k sert à récupérer toutes sortes de
choses, et seule une partie a sa place dans un catalogue d'albums.

# La détection repose sur l'ÉTAT, jamais sur l'événement

L'ADR-005 le dit : un téléchargement qui démarre et se termine entre deux
instantanés ne produit aucun événement. Un pont branché sur les événements
manquerait donc silencieusement des fichiers — et le symptôme serait « il en
manque un de temps en temps », le pire des défauts à diagnostiquer.

On regarde donc la file : tout fichier terminé qu'on n'a pas encore traité est
publié. La table des publications tient la mémoire, et son unicité sur
l'empreinte rend l'opération idempotente sans verrou.

# Le fichier est lu par un Provider, jamais par os.Open

Le répertoire d'arrivée du démon est déclaré comme un backend local en LECTURE
SEULE. La règle n°1 du projet tient donc à la lettre, et un défaut de notre côté
ne peut pas abîmer la zone de travail du démon.
*/

// Destination dit ce que devient un fichier d'une catégorie donnée.
type Destination struct {
	Category int
	Label    string

	// LibraryID est nil pour « laisser sur disque », qui est le défaut.
	LibraryID *uuid.UUID
	Folder    string
}

// Publishes returns true quand cette destination fait entrer le fichier dans
// une bibliothèque.
func (d Destination) Publishes() bool { return d.LibraryID != nil }

// PublicationStatus est l'issue d'une tentative.
type PublicationStatus string

const (
	// PublicationPending — retenue, pas encore traitée.
	PublicationPending PublicationStatus = "pending"

	// PublicationPublished — le fichier est devenu un album.
	PublicationPublished PublicationStatus = "published"

	/*
		PublicationSkipped — la catégorie laisse le fichier sur disque.

		Ce n'est PAS un échec, et l'inscrire compte : sans trace, chaque tour de
		scrutation reconsidérerait les mêmes fichiers indéfiniment.
	*/
	PublicationSkipped PublicationStatus = "skipped"

	// PublicationError — la publication a échoué. Le détail dit pourquoi.
	PublicationError PublicationStatus = "error"
)

// Publication est la trace de ce que le pont a fait d'un fichier.
type Publication struct {
	Hash     string
	Name     string
	Size     int64
	Category int

	Status PublicationStatus
	Detail string

	LibraryID *uuid.UUID
	ComicID   *uuid.UUID
}

/*
Publisher est ce que le pont attend de l'ingestion.

Interface déclarée ICI, au point d'usage : le module ne doit pas importer
`ingest`, et c'est `wiring.go` qui les met en présence. Voir CONTRIBUTING.md,
règle 2.
*/
type Publisher interface {
	Publish(ctx context.Context, libraryID uuid.UUID, folder, filename string,
		size int64, content io.Reader) (uuid.UUID, error)
}

// SetPublisher branche l'ingestion. Sans elle, le pont ne publie rien et le dit.
func (s *Service) SetPublisher(p Publisher) { s.publisher = p }

/*
SetIncoming branche le répertoire d'arrivée du démon.

Un `storage.Provider`, et non un chemin : c'est ce qui fait que ce module ne
lit jamais le disque directement. Le provider est ouvert en lecture seule par
`wiring.go`.
*/
func (s *Service) SetIncoming(p storage.Provider) { s.incoming = p }

/*
publishCompleted examine un instantané et publie ce qui est terminé.

Appelé après chaque collecte. Le coût est nul quand rien n'a fini : une lecture
en base par fichier terminé, et rien du tout sinon.
*/
func (s *Service) publishCompleted(ctx context.Context, snapshot *Snapshot) {
	if snapshot == nil || s.publisher == nil || s.incoming == nil {
		return
	}

	for _, download := range snapshot.Downloads {
		if download.Status != DownloadCompleted {
			continue
		}
		if err := s.publish(ctx, download); err != nil {
			s.log.Warn("publication d'un téléchargement terminé",
				slog.String("hash", download.Hash),
				slog.String("name", download.Name),
				slog.Any("err", err))
		}
	}
}

/*
publish traite UN fichier terminé.

L'inscription en base vient EN PREMIER, et c'est ce qui rend l'opération
idempotente : deux tours qui verraient le même fichier ne produiront qu'une
publication, parce que le second n'obtiendra pas la réservation.
*/
func (s *Service) publish(ctx context.Context, download Download) error {
	claimed, err := s.repo.ClaimPublication(ctx, Publication{
		Hash:     download.Hash,
		Name:     download.Name,
		Size:     download.Size,
		Category: download.Category,
	})
	if err != nil {
		return err
	}
	if !claimed {
		// Déjà traité, ou en cours de traitement par un tour précédent.
		return nil
	}

	destination, err := s.repo.GetDestination(ctx, download.Category)
	if errors.Is(err, ErrNoDestination) || (err == nil && !destination.Publishes()) {
		// Le cas nominal : cette catégorie laisse ses fichiers sur disque.
		return s.repo.SetPublicationResult(ctx, download.Hash, Publication{
			Status: PublicationSkipped,
			Detail: "la catégorie laisse le fichier sur disque",
		})
	}
	if err != nil {
		return s.failPublication(ctx, download.Hash, err)
	}

	if err := s.copyToLibrary(ctx, download, destination); err != nil {
		return s.failPublication(ctx, download.Hash, err)
	}
	return nil
}

/*
copyToLibrary achemine le fichier vers la bibliothèque.

En FLUX : un fichier de plusieurs gigaoctets ne doit jamais traverser la mémoire
du serveur. `ingest.Upload` sait déjà faire cela vers un backend distant ; on ne
lui donne qu'un lecteur.

Le nom du fichier dans le répertoire d'arrivée est celui que le démon lui a
donné, qui est le nom annoncé sur le réseau. C'est aussi celui qu'on veut voir
dans la bibliothèque.
*/
func (s *Service) copyToLibrary(ctx context.Context, download Download, dest Destination) error {
	key := path.Base(download.Name)
	if key == "" || key == "." || key == "/" {
		return fmt.Errorf("nom de fichier inutilisable : %q", download.Name)
	}

	info, err := s.incoming.Stat(ctx, key)
	if err != nil {
		/*
			Le fichier est terminé côté démon mais introuvable chez nous : le
			répertoire d'arrivée n'est pas le bon, ou le volume n'est pas
			partagé entre les deux conteneurs.

			C'est LA cause la plus fréquente d'un pont qui ne fait rien, et le
			message le dit plutôt que de laisser chercher.
		*/
		return fmt.Errorf(
			"fichier introuvable dans le répertoire d'arrivée (%q) — "+
				"BOXINCLOUD_ED2K_INCOMING_DIR pointe-t-il bien sur le volume du démon ? : %w",
			key, err)
	}

	reader, err := s.incoming.Open(ctx, key)
	if err != nil {
		return fmt.Errorf("lecture de %q : %w", key, err)
	}
	defer func() { _ = reader.Close() }()

	comicID, err := s.publisher.Publish(ctx, *dest.LibraryID, dest.Folder, key, info.Size, reader)
	if err != nil {
		return fmt.Errorf("publication dans la bibliothèque : %w", err)
	}

	s.log.Info("téléchargement publié dans la bibliothèque",
		slog.String("name", key),
		slog.String("library", dest.LibraryID.String()),
		slog.String("comic", comicID.String()))

	return s.repo.SetPublicationResult(ctx, download.Hash, Publication{
		Status:    PublicationPublished,
		LibraryID: dest.LibraryID,
		ComicID:   &comicID,
	})
}

// failPublication note l'échec, sans le perdre.
//
// L'état reste consultable depuis l'interface : un fichier qui n'a pas pu être
// publié doit se voir, sinon il disparaît du réseau ET du catalogue.
func (s *Service) failPublication(ctx context.Context, hash string, cause error) error {
	detail := cause.Error()
	if len(detail) > 500 {
		detail = detail[:500] + "…"
	}

	if err := s.repo.SetPublicationResult(ctx, hash, Publication{
		Status: PublicationError,
		Detail: detail,
	}); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

// ─── Destinations ────────────────────────────────────────────────────────────

// ErrNoDestination signale une catégorie sans règle.
var ErrNoDestination = errors.New("amule : aucune destination pour cette catégorie")

// Destinations liste les règles déclarées.
func (s *Service) Destinations(ctx context.Context) ([]Destination, error) {
	if !s.opts.Enabled {
		return nil, ErrDisabled
	}
	return s.repo.ListDestinations(ctx)
}

/*
SetDestination déclare ou remplace la règle d'une catégorie.

Une bibliothèque nulle rétablit le défaut : laisser sur disque. C'est la façon
de défaire une règle sans avoir à la supprimer, et cela évite de faire
disparaître un libellé qu'on voudra revoir.
*/
func (s *Service) SetDestination(ctx context.Context, d Destination) (Destination, error) {
	if !s.opts.Enabled {
		return Destination{}, ErrDisabled
	}

	if d.Category < 0 {
		return Destination{}, ValidationError{Fields: map[string]string{
			"category": "doit être positive ou nulle",
		}}
	}

	d.Label = strings.TrimSpace(d.Label)
	if d.Label == "" {
		return Destination{}, ValidationError{Fields: map[string]string{
			"label": "obligatoire",
		}}
	}

	// Le dossier est normalisé : une barre de tête ou de queue produirait des
	// clés d'objet doublées côté backend.
	d.Folder = strings.Trim(strings.TrimSpace(d.Folder), "/")

	return s.repo.SaveDestination(ctx, d)
}

// Publications liste ce que le pont a fait, le plus récent d'abord.
func (s *Service) Publications(ctx context.Context, limit int) ([]Publication, error) {
	if !s.opts.Enabled {
		return nil, ErrDisabled
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.repo.ListPublications(ctx, limit)
}
