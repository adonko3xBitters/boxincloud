// Package ingest fait entrer du contenu dans une bibliothèque.
//
// Jusqu'ici, remplir boxincloud demandait un terminal : téléverser dans le
// bucket avec un autre outil, puis lancer `boxincloudctl scan`. Autant dire que
// le produit n'était utilisable que par celui qui l'avait installé.
//
// Ce paquet ouvre la voie normale : envoyer un fichier, et le voir apparaître.
package ingest

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/indexer"
	"github.com/adonko3xBitters/boxincloud/server/internal/library"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
)

var (
	// ErrUnsupportedFormat signale un fichier qui n'est pas une archive de BD.
	ErrUnsupportedFormat = errors.New("ingest : format non pris en charge")

	// ErrContentMismatch signale un fichier dont le contenu dément l'extension.
	ErrContentMismatch = errors.New("ingest : le contenu ne correspond pas à l'extension")

	// ErrAlreadyExists signale un objet déjà présent à cette destination.
	ErrAlreadyExists = errors.New("ingest : un fichier de ce nom existe déjà")

	// ErrEmptyName signale un nom de fichier vide après nettoyage.
	ErrEmptyName = errors.New("ingest : nom de fichier inutilisable")

	// ErrTooLarge signale un envoi au-delà de la taille autorisée.
	ErrTooLarge = errors.New("ingest : fichier trop volumineux")
)

// Repository couvre ce dont l'ingestion a besoin du catalogue.
type Repository interface {
	UpsertComic(ctx context.Context, p indexer.UpsertComicParams) (indexer.Comic, bool, error)
	SetFolder(ctx context.Context, id uuid.UUID, folder string) error
	EnqueueIndexComic(ctx context.Context, comicID uuid.UUID) error
}

// Scanner déclenche un parcours complet de bibliothèque.
type Scanner interface {
	EnqueueScanLibrary(ctx context.Context, libraryID uuid.UUID) error
}

// FolderRegistrar inscrit un dossier rencontré dans l'arborescence.
//
// Injecté plutôt qu'importé : le paquet folders dépend déjà des bibliothèques et
// du stockage, comme celui-ci, et l'importer créerait un cycle.
type FolderRegistrar func(ctx context.Context, libraryID uuid.UUID, path string) error

// WriteGuard refuse une écriture dans un dossier protégé en lecture seule.
//
// Injecté pour la même raison que le registrar : le paquet folders dépend des
// bibliothèques et du stockage, comme celui-ci.
type WriteGuard func(ctx context.Context, libraryID uuid.UUID, path string) error

// Service reçoit les fichiers, les fait entrer au catalogue, et les gère
// ensuite : suppression, déplacement.
type Service struct {
	libraries *library.Service
	repo      Repository
	manage    ManageRepository
	scanner   Scanner
	registrar FolderRegistrar
	guard     WriteGuard
	log       *slog.Logger

	// derived est le stockage du cache dérivé. Il n'est consulté que pour une
	// chose : effacer l'archive hydratée d'un album qu'on purge. Sans elle,
	// supprimer un CBR laisserait derrière lui sa version normalisée — plusieurs
	// centaines de méga-octets que plus rien ne référence et que rien ne
	// viendrait jamais nettoyer.
	derived storage.Provider

	// maxSize borne un envoi. Zéro signifie « sans limite ».
	maxSize int64
}

func NewService(
	libraries *library.Service,
	repo Repository,
	manage ManageRepository,
	scanner Scanner,
	derived storage.Provider,
	maxSize int64,
	log *slog.Logger,
) *Service {
	return &Service{
		libraries: libraries,
		repo:      repo,
		manage:    manage,
		scanner:   scanner,
		derived:   derived,
		maxSize:   maxSize,
		log:       log,
	}
}

// UploadParams décrit un fichier à faire entrer.
type UploadParams struct {
	LibraryID uuid.UUID

	// Folder est le dossier de destination dans la bibliothèque, relatif à son
	// préfixe. Vide pour la racine.
	Folder string

	// Filename est le nom d'origine, tel que fourni par le client. Il est
	// nettoyé avant d'être utilisé comme clé.
	Filename string

	// Size est la taille annoncée, ou -1 si inconnue. Une taille connue permet
	// un envoi direct ; sinon le backend bascule sur un envoi fractionné.
	Size int64

	Content io.Reader
}

// Result décrit ce qui a été créé.
type Result struct {
	ComicID   uuid.UUID
	ObjectKey string
	Title     string
	Format    string
	Size      int64
}

/*
Upload écrit un fichier dans le backend puis l'inscrit au catalogue.

Le contenu est acheminé en flux jusqu'au backend : une archive de cinq cents
méga-octets ne doit jamais transiter par la mémoire du serveur, ni par un
fichier temporaire dont personne ne surveillerait la taille.

L'indexation n'est pas faite ici mais enfilée. Lire l'index d'une archive
demande plusieurs allers-retours au serveur d'objets ; les attendre ferait
patienter l'utilisateur devant une requête qui a déjà réussi.
*/
func (s *Service) Upload(ctx context.Context, p UploadParams) (Result, error) {
	lib, err := s.libraries.GetLibrary(ctx, p.LibraryID)
	if err != nil {
		return Result{}, err
	}

	name, err := sanitizeFilename(p.Filename)
	if err != nil {
		return Result{}, err
	}

	format, ok := DetectFormat(name)
	if !ok {
		return Result{}, fmt.Errorf("%w : %s", ErrUnsupportedFormat, path.Ext(name))
	}

	if s.maxSize > 0 && p.Size > s.maxSize {
		return Result{}, fmt.Errorf("%w : %d octets, maximum %d", ErrTooLarge, p.Size, s.maxSize)
	}

	// Le contenu est vérifié avant d'écrire quoi que ce soit. Renommer un
	// exécutable en .cbz ne doit pas suffire à le déposer dans le bucket : le
	// fichier y resterait, servi ensuite à tous les clients.
	reader := bufio.NewReaderSize(p.Content, magicPeek)
	head, err := reader.Peek(magicPeek)
	if err != nil && !errors.Is(err, io.EOF) {
		return Result{}, fmt.Errorf("ingest : lecture de l'en-tête : %w", err)
	}
	if !matchesFormat(head, format) {
		return Result{}, fmt.Errorf("%w : annoncé %s", ErrContentMismatch, format)
	}

	provider, err := s.libraries.ProviderForLibrary(ctx, lib)
	if err != nil {
		return Result{}, err
	}

	// La destination peut être protégée : le vérifier AVANT d'écrire évite de
	// déposer un objet qu'il faudrait ensuite retirer.
	if err := s.ensureWritable(ctx, lib.ID, p.Folder); err != nil {
		return Result{}, err
	}

	key := objectKey(lib.RootPrefix, p.Folder, name)

	// Un envoi ne doit jamais écraser un fichier existant en silence : la
	// progression de lecture des utilisateurs est attachée à l'objet, et la
	// remplacer par une autre édition la rendrait fausse sans prévenir.
	if _, err := provider.Stat(ctx, key); err == nil {
		return Result{}, fmt.Errorf("%w : %s", ErrAlreadyExists, key)
	} else if !errors.Is(err, storage.ErrNotFound) {
		return Result{}, err
	}

	// Une taille bornée est imposée au flux lui-même, pas seulement à la valeur
	// annoncée : un client peut annoncer un petit fichier et en envoyer un
	// énorme.
	var body io.Reader = reader
	if s.maxSize > 0 {
		body = &limitedReader{r: reader, remaining: s.maxSize}
	}

	if err := provider.Write(ctx, key, body, p.Size, contentTypeFor(format)); err != nil {
		return Result{}, err
	}

	info, err := provider.Stat(ctx, key)
	if err != nil {
		return Result{}, err
	}

	meta := indexer.ParseFilename(key)
	title := meta.Title
	if title == "" {
		title = strings.TrimSuffix(name, path.Ext(name))
	}

	folder := indexer.FolderOf(key, lib.RootPrefix)

	comic, _, err := s.repo.UpsertComic(ctx, indexer.UpsertComicParams{
		LibraryID:  lib.ID,
		ObjectKey:  key,
		FileSize:   info.Size,
		FileETag:   info.ETag,
		Format:     string(format),
		Title:      title,
		FolderPath: folder,
	})
	if err != nil {
		// L'objet est écrit mais absent du catalogue. Le laisser en place plutôt
		// que de le supprimer : un scan le rattrapera, alors qu'une suppression
		// perdrait un envoi qui a pourtant abouti.
		s.log.Error("album téléversé mais non inscrit au catalogue",
			slog.String("key", key), slog.Any("err", err))
		return Result{}, err
	}

	if err := s.repo.SetFolder(ctx, comic.ID, folder); err != nil {
		s.log.Debug("dossier non enregistré", slog.String("key", key), slog.Any("err", err))
	}

	// Le dossier entre dans l'arborescence tout de suite : déposer un album
	// dans un dossier neuf doit le faire apparaître, pas attendre un parcours.
	s.registerFolder(ctx, lib.ID, folder)

	if err := s.repo.EnqueueIndexComic(ctx, comic.ID); err != nil {
		s.log.Warn("indexation non enfilée",
			slog.String("comic_id", comic.ID.String()), slog.Any("err", err))
	}

	return Result{
		ComicID:   comic.ID,
		ObjectKey: key,
		Title:     title,
		Format:    string(format),
		Size:      info.Size,
	}, nil
}

// Scan demande un parcours complet d'une bibliothèque.
//
// Utile quand des fichiers sont arrivés par un autre chemin que l'envoi — un
// montage réseau, un rsync, un bucket alimenté par ailleurs.
func (s *Service) Scan(ctx context.Context, libraryID uuid.UUID) error {
	if _, err := s.libraries.GetLibrary(ctx, libraryID); err != nil {
		return err
	}
	return s.scanner.EnqueueScanLibrary(ctx, libraryID)
}

// ─── Formats ─────────────────────────────────────────────────────────────────

// magicPeek est le nombre d'octets examinés en tête de fichier.
//
// Huit suffisent pour toutes les signatures reconnues ; RAR 5 est la plus
// longue avec huit octets exactement.
const magicPeek = 8

// Format désigne un type d'archive accepté.
type Format string

const (
	FormatCBZ Format = "cbz"
	FormatCBR Format = "cbr"
	FormatPDF Format = "pdf"
)

// extensions associe une extension à son format.
//
// `.zip` et `.rar` sont acceptés parce que beaucoup d'archives de BD circulent
// sans l'extension dédiée, et refuser un fichier parfaitement lisible sur ce
// seul motif serait absurde.
var extensions = map[string]Format{
	".cbz": FormatCBZ,
	".zip": FormatCBZ,
	".cbr": FormatCBR,
	".rar": FormatCBR,
	".pdf": FormatPDF,
}

// DetectFormat déduit le format d'un nom de fichier.
func DetectFormat(name string) (Format, bool) {
	format, ok := extensions[strings.ToLower(path.Ext(name))]
	return format, ok
}

// SupportedExtensions liste les extensions acceptées, pour l'affichage côté
// client et la documentation du contrat.
func SupportedExtensions() []string {
	return []string{".cbz", ".zip", ".cbr", ".rar", ".pdf"}
}

// signatures donne les en-têtes possibles de chaque format.
var signatures = map[Format][][]byte{
	// Une archive ZIP vide ou en fin de flux porte d'autres signatures que
	// PK\x03\x04 ; elles sont acceptées pour ne pas rejeter une archive
	// techniquement valide.
	FormatCBZ: {
		{'P', 'K', 0x03, 0x04},
		{'P', 'K', 0x05, 0x06},
		{'P', 'K', 0x07, 0x08},
	},
	FormatCBR: {
		{'R', 'a', 'r', '!', 0x1a, 0x07, 0x00},       // RAR 1.5 à 4.x
		{'R', 'a', 'r', '!', 0x1a, 0x07, 0x01, 0x00}, // RAR 5.0
	},
	FormatPDF: {
		{'%', 'P', 'D', 'F', '-'},
	},
}

func matchesFormat(head []byte, format Format) bool {
	for _, signature := range signatures[format] {
		if bytes.HasPrefix(head, signature) {
			return true
		}
	}
	return false
}

func contentTypeFor(format Format) string {
	switch format {
	case FormatPDF:
		return "application/pdf"
	case FormatCBR:
		return "application/vnd.comicbook-rar"
	default:
		return "application/vnd.comicbook+zip"
	}
}

// ─── Chemins ─────────────────────────────────────────────────────────────────

/*
sanitizeFilename ramène un nom fourni par le client à un nom de fichier sûr.

Le nom vient du navigateur de quelqu'un : il peut contenir des séparateurs, des
« .. », des caractères de contrôle, ou être un chemin absolu. Aucun de ces cas
n'est une attaque nécessairement — un glisser-déposer de dossier suffit — mais
tous doivent être neutralisés avant de composer une clé d'objet.
*/
func sanitizeFilename(name string) (string, error) {
	// Seul le dernier segment est retenu, quel que soit le séparateur : un
	// client Windows enverra des antislashs.
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(strings.TrimSpace(name))

	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)

	name = strings.TrimSpace(name)
	name = strings.Trim(name, ".")

	// path.Base rend "/" pour une entrée faite uniquement de séparateurs. Un
	// séparateur retenu comme nom de fichier composerait une clé qui vise un
	// répertoire, pas un objet.
	name = strings.Trim(name, "/")

	if name == "" || name == "." || name == ".." {
		return "", ErrEmptyName
	}
	return name, nil
}

/*
sanitizeFolder ramène un dossier de destination à un chemin relatif sûr.

Les segments vides, « . » et « .. » disparaissent : sans cela, un dossier
« ../../ » ferait écrire hors du préfixe de la bibliothèque, donc hors de ce que
l'utilisateur croyait viser.
*/
func sanitizeFolder(folder string) string {
	folder = strings.ReplaceAll(folder, "\\", "/")

	segments := make([]string, 0, 4)
	for _, segment := range strings.Split(folder, "/") {
		segment = strings.TrimSpace(segment)
		segment = strings.TrimFunc(segment, func(r rune) bool {
			return unicode.IsControl(r)
		})
		if segment == "" || segment == "." || segment == ".." {
			continue
		}
		segments = append(segments, segment)
	}
	return strings.Join(segments, "/")
}

// objectKey compose la clé finale : préfixe de bibliothèque, dossier, nom.
func objectKey(rootPrefix, folder, name string) string {
	parts := make([]string, 0, 3)

	if prefix := strings.Trim(rootPrefix, "/"); prefix != "" {
		parts = append(parts, prefix)
	}
	if clean := sanitizeFolder(folder); clean != "" {
		parts = append(parts, clean)
	}
	parts = append(parts, name)

	return strings.Join(parts, "/")
}

// ─── Lecture bornée ──────────────────────────────────────────────────────────

// limitedReader arrête la lecture au-delà d'un quota et le dit clairement.
//
// io.LimitReader renvoie io.EOF à la limite, ce qui ferait passer un fichier
// tronqué pour un fichier complet : l'objet serait écrit, à moitié.
type limitedReader struct {
	r         io.Reader
	remaining int64
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.remaining <= 0 {
		return 0, ErrTooLarge
	}
	if int64(len(p)) > l.remaining {
		p = p[:l.remaining]
	}
	n, err := l.r.Read(p)
	l.remaining -= int64(n)
	return n, err
}

// SetFolderRegistrar câble l'inscription des dossiers rencontrés.
func (s *Service) SetFolderRegistrar(register FolderRegistrar) { s.registrar = register }

// registerFolder inscrit un dossier sans faire échouer l'opération en cours.
//
// Un album correctement déposé ne doit pas être signalé en échec parce que
// l'arborescence n'a pas suivi : le prochain parcours la remettra d'aplomb.
func (s *Service) registerFolder(ctx context.Context, libraryID uuid.UUID, path string) {
	if s.registrar == nil || path == "" {
		return
	}
	if err := s.registrar(ctx, libraryID, path); err != nil {
		s.log.Warn("dossier non inscrit dans l'arborescence",
			slog.String("path", path), slog.Any("err", err))
	}
}

// SetWriteGuard câble le contrôle de lecture seule.
func (s *Service) SetWriteGuard(guard WriteGuard) { s.guard = guard }

// ensureWritable délègue au garde, s'il est câblé.
func (s *Service) ensureWritable(ctx context.Context, libraryID uuid.UUID, path string) error {
	if s.guard == nil {
		return nil
	}
	return s.guard(ctx, libraryID, path)
}
