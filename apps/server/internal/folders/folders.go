// Package folders gère l'arborescence d'une bibliothèque.
//
// L'arborescence était jusqu'ici déduite des clés d'objet : elle existait à
// l'affichage, jamais en base. Cela suffisait pour parcourir, mais interdisait
// tout ce qu'on veut y attacher — un verrou, un partage, ou simplement un
// dossier vide créé à l'avance.
//
// La déduction subsiste : le parcours observe les clés et réconcilie. Les deux
// répondent à deux questions distinctes — où sont réellement les fichiers, et ce
// que l'utilisateur a décidé.
package folders

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/library"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
)

var (
	// ErrNotFound signale un dossier inexistant.
	ErrNotFound = errors.New("folders : dossier introuvable")

	// ErrAlreadyExists signale une destination occupée.
	ErrAlreadyExists = errors.New("folders : un dossier de ce nom existe déjà")

	// ErrInvalidName signale un nom inutilisable.
	ErrInvalidName = errors.New("folders : nom de dossier invalide")

	// ErrRootImmutable protège la racine.
	//
	// Elle n'est pas un dossier comme les autres : elle EST la bibliothèque. La
	// renommer ou la supprimer n'aurait aucun sens représentable.
	ErrRootImmutable = errors.New("folders : la racine ne peut être ni renommée ni supprimée")

	// ErrNotEmpty signale une suppression refusée faute de confirmation.
	ErrNotEmpty = errors.New("folders : le dossier n'est pas vide")

	// ErrIntoItself signale un déplacement d'un dossier dans sa descendance.
	ErrIntoItself = errors.New("folders : un dossier ne peut pas être déplacé dans lui-même")

	// ErrTooManyComics borne une opération sur une branche.
	ErrTooManyComics = errors.New("folders : trop d'albums dans cette branche")
)

/*
MaxRelocatable borne le nombre d'albums qu'un renommage peut déplacer.

Renommer un dossier renomme chaque objet qu'il contient, et sur un backend
distant chacun coûte un aller-retour. Deux mille couvrent très largement une
branche réelle ; au-delà, l'opération tiendrait plusieurs minutes et vaut mieux
d'être découpée que subie.
*/
const MaxRelocatable = 2000

// Folder est un nœud de l'arborescence.
type Folder struct {
	ID        uuid.UUID
	LibraryID uuid.UUID
	Path      string
	Name      string
	Depth     int
	Explicit  bool

	// ReadOnly protège d'une écriture. N'affecte pas la visibilité.
	ReadOnly bool

	// HasCode signale un dossier masqué par un code d'accès.
	HasCode bool

	// Unlocked est vrai quand le code a été saisi et n'a pas expiré.
	Unlocked bool

	// ComicCount cumule les albums du dossier ET de ses descendants : c'est ce
	// qu'attend quelqu'un qui replie un nœud.
	ComicCount int
}

// ComicRef désigne un album à déplacer.
type ComicRef struct {
	ID        uuid.UUID
	ObjectKey string
}

// Repository est ce dont le service a besoin de la base.
type Repository interface {
	List(ctx context.Context, libraryIDs []uuid.UUID) ([]Folder, error)
	Get(ctx context.Context, libraryID uuid.UUID, path string) (Folder, error)
	Upsert(ctx context.Context, f Folder) (Folder, error)

	// RenameTree réécrit le préfixe d'une branche entière.
	RenameTree(ctx context.Context, libraryID uuid.UUID, oldPath, newPath, newName, newParent string, depthDelta int) (int64, error)

	DeleteTree(ctx context.Context, libraryID uuid.UUID, path string) (int64, error)
	PruneEmpty(ctx context.Context, libraryID uuid.UUID) (int64, error)

	CountsByExactFolder(ctx context.Context, libraryIDs []uuid.UUID) (map[uuid.UUID]map[string]int, error)
	ComicsInTree(ctx context.Context, libraryID uuid.UUID, path string) ([]ComicRef, error)
	MoveComic(ctx context.Context, id uuid.UUID, objectKey, folderPath string) error
}

// ComicRemover supprime une sélection d'albums.
//
// Injecté plutôt qu'importé : le service d'ingestion porte déjà la règle de
// suppression — exclusion ou effacement, dans le bon ordre — et l'importer ici
// créerait un cycle, les deux dépendant des bibliothèques et du stockage.
type ComicRemover func(ctx context.Context, ids []uuid.UUID, deleteFiles bool) (int, error)

// Service gère l'arborescence.
type Service struct {
	repo      Repository
	libraries *library.Service
	log       *slog.Logger
	remove    ComicRemover
	locks     LockRepository
}

func NewService(repo Repository, libraries *library.Service, log *slog.Logger) *Service {
	return &Service{repo: repo, libraries: libraries, log: log}
}

// ─── Lecture ─────────────────────────────────────────────────────────────────

/*
Tree retourne l'arborescence, à plat, parents avant enfants.

Les compteurs sont cumulés en une passe : les dossiers étant triés par chemin,
un enfant est toujours rencontré après son parent, et il suffit de remonter la
chaîne des ancêtres. Recompter par requête pour chaque nœud coûterait une
requête par dossier.
*/
func (s *Service) Tree(ctx context.Context, userID uuid.UUID, libraryIDs []uuid.UUID) ([]Folder, error) {
	if len(libraryIDs) == 0 {
		return []Folder{}, nil
	}

	all, err := s.repo.List(ctx, libraryIDs)
	if err != nil {
		return nil, err
	}

	unlocked, err := s.unlockedPaths(ctx, userID, libraryIDs)
	if err != nil {
		return nil, err
	}

	hidden, err := s.LockedPaths(ctx, userID, libraryIDs)
	if err != nil {
		return nil, err
	}

	/*
		Les branches masquées sont retirées AVANT le cumul des compteurs.

		Sans cela, la racine annoncerait un total incluant ce qu'elle cache, et
		la simple soustraction révélerait l'existence — et le volume — d'un
		dossier qu'un code est censé dissimuler. Un compteur qui ne colle pas
		avec la somme de ses enfants est un aveu.
	*/
	list := make([]Folder, 0, len(all))
	for _, folder := range all {
		if hiddenUnder(folder.Path, hidden) {
			continue
		}
		if folder.HasCode {
			folder.Unlocked = unlocked[folder.Path]
		}
		list = append(list, folder)
	}

	counts, err := s.repo.CountsByExactFolder(ctx, libraryIDs)
	if err != nil {
		return nil, err
	}

	// Comptes exacts d'abord.
	for i := range list {
		if byPath, ok := counts[list[i].LibraryID]; ok {
			list[i].ComicCount = byPath[list[i].Path]
		}
	}

	// Les albums d'un dossier masqué ne remontent nulle part : leur dossier
	// n'est plus dans la liste, donc leur compte exact n'est jamais lu.

	// Puis cumul vers les ancêtres.
	index := make(map[uuid.UUID]map[string]int, len(libraryIDs))
	for i := range list {
		if index[list[i].LibraryID] == nil {
			index[list[i].LibraryID] = make(map[string]int)
		}
		index[list[i].LibraryID][list[i].Path] = i
	}

	for i := range list {
		exact := list[i].ComicCount
		if exact == 0 {
			continue
		}
		for _, ancestor := range ancestorsOf(list[i].Path) {
			if j, ok := index[list[i].LibraryID][ancestor]; ok {
				list[j].ComicCount += exact
			}
		}
	}

	sort.SliceStable(list, func(a, b int) bool {
		if list[a].LibraryID != list[b].LibraryID {
			return list[a].LibraryID.String() < list[b].LibraryID.String()
		}
		return list[a].Path < list[b].Path
	})
	return list, nil
}

// Get retourne un dossier par son chemin.
func (s *Service) Get(ctx context.Context, libraryID uuid.UUID, path string) (Folder, error) {
	return s.repo.Get(ctx, libraryID, NormalizePath(path))
}

// ─── Création ────────────────────────────────────────────────────────────────

/*
Create inscrit un dossier, ancêtres compris.

Rien n'est écrit dans le backend : un magasin d'objets n'a pas de répertoires,
seulement des clés. Le dossier existe donc d'abord dans boxincloud, et prendra
corps dans le stockage au premier fichier qui y sera déposé.
*/
func (s *Service) Create(ctx context.Context, libraryID uuid.UUID, path string) (Folder, error) {
	clean := NormalizePath(path)
	if clean == "" {
		return Folder{}, fmt.Errorf("%w : chemin vide", ErrInvalidName)
	}

	if _, err := s.libraries.GetLibrary(ctx, libraryID); err != nil {
		return Folder{}, err
	}
	if err := s.EnsureWritable(ctx, libraryID, clean); err != nil {
		return Folder{}, err
	}

	if existing, err := s.repo.Get(ctx, libraryID, clean); err == nil {
		return existing, ErrAlreadyExists
	} else if !errors.Is(err, ErrNotFound) {
		return Folder{}, err
	}

	// Les ancêtres sont créés en passant : demander « BD/Franco-belge/Tintin »
	// suppose les deux premiers, qu'aucun fichier n'occupe forcément.
	var created Folder
	for _, ancestor := range append(ancestorsOf(clean), clean) {
		if ancestor == "" {
			continue
		}
		f, err := s.repo.Upsert(ctx, folderAt(libraryID, ancestor, ancestor == clean))
		if err != nil {
			return Folder{}, err
		}
		created = f
	}
	return created, nil
}

// ─── Renommage et déplacement ────────────────────────────────────────────────

/*
Relocate renomme ou déplace une branche.

Le dossier d'un album découle de la clé de son objet : renommer un dossier, c'est
renommer chacun des objets qu'il contient. Les deux gestes — changer de nom,
changer de parent — sont le même en dessous, et sont donc traités ensemble.

L'ordre est celui du déplacement d'un album : l'objet bouge d'abord, le catalogue
suit. L'inverse ferait pointer la base vers des clés qui n'existent pas encore, et
toute lecture de page échouerait dans l'intervalle.

Un échec en cours de route laisse la branche à moitié déplacée. C'est
volontairement le cas retenu plutôt qu'un retour arrière : défaire des
déplacements déjà réussis multiplierait les occasions de perdre un fichier, alors
qu'un état mixte reste entièrement lisible — chaque album pointe la clé où il se
trouve réellement.
*/
func (s *Service) Relocate(ctx context.Context, libraryID uuid.UUID, oldPath, newPath string) (Folder, error) {
	source := NormalizePath(oldPath)
	target := NormalizePath(newPath)

	if source == "" {
		return Folder{}, ErrRootImmutable
	}
	if target == "" {
		return Folder{}, fmt.Errorf("%w : destination vide", ErrInvalidName)
	}
	if source == target {
		return s.repo.Get(ctx, libraryID, source)
	}
	if strings.HasPrefix(target, source+"/") {
		return Folder{}, ErrIntoItself
	}

	if _, err := s.repo.Get(ctx, libraryID, source); err != nil {
		return Folder{}, err
	}

	// La protection porte sur la source ET sur la destination : déplacer une
	// branche hors d'un dossier protégé le viderait, y déplacer une branche le
	// modifierait tout autant.
	if err := s.EnsureWritable(ctx, libraryID, source); err != nil {
		return Folder{}, err
	}
	if err := s.EnsureWritable(ctx, libraryID, target); err != nil {
		return Folder{}, err
	}
	if _, err := s.repo.Get(ctx, libraryID, target); err == nil {
		return Folder{}, ErrAlreadyExists
	} else if !errors.Is(err, ErrNotFound) {
		return Folder{}, err
	}

	lib, err := s.libraries.GetLibrary(ctx, libraryID)
	if err != nil {
		return Folder{}, err
	}

	comics, err := s.repo.ComicsInTree(ctx, libraryID, source)
	if err != nil {
		return Folder{}, err
	}
	if len(comics) > MaxRelocatable {
		return Folder{}, fmt.Errorf("%w : %d albums, maximum %d",
			ErrTooManyComics, len(comics), MaxRelocatable)
	}

	provider, err := s.libraries.ProviderForLibrary(ctx, lib)
	if err != nil {
		return Folder{}, err
	}

	prefix := strings.Trim(lib.RootPrefix, "/")
	oldKeyPrefix := joinKey(prefix, source)
	newKeyPrefix := joinKey(prefix, target)

	for _, comic := range comics {
		if !strings.HasPrefix(comic.ObjectKey, oldKeyPrefix+"/") {
			// La clé ne suit pas le dossier enregistré : le catalogue et le
			// stockage ont divergé. On saute plutôt que de composer une clé
			// fausse, et un parcours remettra les choses d'aplomb.
			s.log.Warn("clé hors du dossier annoncé, non déplacée",
				slog.String("comic_id", comic.ID.String()),
				slog.String("key", comic.ObjectKey))
			continue
		}

		newKey := newKeyPrefix + strings.TrimPrefix(comic.ObjectKey, oldKeyPrefix)

		if err := provider.Move(ctx, comic.ObjectKey, newKey); err != nil {
			if errors.Is(err, storage.ErrAlreadyExists) {
				return Folder{}, fmt.Errorf("%w : %s", ErrAlreadyExists, newKey)
			}
			return Folder{}, err
		}

		folder := folderOfKey(newKey, prefix)
		if err := s.repo.MoveComic(ctx, comic.ID, newKey, folder); err != nil {
			s.log.Error("album déplacé mais catalogue non mis à jour",
				slog.String("comic_id", comic.ID.String()),
				slog.String("to", newKey), slog.Any("err", err))
			return Folder{}, err
		}
	}

	// Les ancêtres de la destination doivent exister avant qu'on y rattache la
	// branche : déplacer vers « BD/Tintin » suppose « BD ».
	for _, ancestor := range ancestorsOf(target) {
		if ancestor == "" {
			continue
		}
		if _, err := s.repo.Upsert(ctx, folderAt(libraryID, ancestor, false)); err != nil {
			return Folder{}, err
		}
	}

	depthDelta := depthOf(target) - depthOf(source)
	if _, err := s.repo.RenameTree(
		ctx, libraryID, source, target, lastSegment(target), parentOf(target), depthDelta,
	); err != nil {
		return Folder{}, err
	}

	if _, err := s.repo.PruneEmpty(ctx, libraryID); err != nil {
		s.log.Debug("élagage impossible", slog.Any("err", err))
	}

	return s.repo.Get(ctx, libraryID, target)
}

// ─── Suppression ─────────────────────────────────────────────────────────────

// DeleteParams décrit une suppression de dossier.
type DeleteParams struct {
	LibraryID uuid.UUID
	Path      string

	// DeleteComics autorise la suppression d'un dossier qui contient encore des
	// albums. Sans ce drapeau, un dossier non vide est refusé — supprimer une
	// branche entière ne doit jamais être un geste distrait.
	DeleteComics bool

	// DeleteFiles supprime aussi les fichiers du stockage. Irréversible.
	DeleteFiles bool
}

/*
Delete retire un dossier.

Trois degrés, du plus sûr au plus définitif : refuser si le dossier contient des
albums, retirer les albums du catalogue en laissant les fichiers, ou tout
effacer. Le premier est le défaut.
*/
func (s *Service) Delete(ctx context.Context, p DeleteParams) (int, error) {
	path := NormalizePath(p.Path)
	if path == "" {
		return 0, ErrRootImmutable
	}

	if _, err := s.repo.Get(ctx, p.LibraryID, path); err != nil {
		return 0, err
	}
	if err := s.EnsureWritable(ctx, p.LibraryID, path); err != nil {
		return 0, err
	}

	comics, err := s.repo.ComicsInTree(ctx, p.LibraryID, path)
	if err != nil {
		return 0, err
	}

	if len(comics) > 0 && !p.DeleteComics {
		return len(comics), ErrNotEmpty
	}
	if len(comics) > MaxRelocatable {
		return len(comics), fmt.Errorf("%w : %d albums, maximum %d",
			ErrTooManyComics, len(comics), MaxRelocatable)
	}

	if len(comics) > 0 {
		if err := s.removeComics(ctx, p, comics); err != nil {
			return 0, err
		}
	}

	if _, err := s.repo.DeleteTree(ctx, p.LibraryID, path); err != nil {
		return 0, err
	}
	return len(comics), nil
}

// removeComics confie la suppression des albums au service d'ingestion, qui
// porte déjà la règle : exclusion ou effacement, dans le bon ordre.
func (s *Service) removeComics(ctx context.Context, p DeleteParams, comics []ComicRef) error {
	if s.remove == nil {
		return errors.New("folders : suppression d'albums non câblée")
	}

	ids := make([]uuid.UUID, 0, len(comics))
	for _, c := range comics {
		ids = append(ids, c.ID)
	}

	_, err := s.remove(ctx, ids, p.DeleteFiles)
	return err
}

// SetComicRemover câble la suppression d'albums.
func (s *Service) SetComicRemover(remove ComicRemover) { s.remove = remove }

// ─── Réconciliation ──────────────────────────────────────────────────────────

/*
Observe inscrit les dossiers rencontrés par un parcours, puis élague.

Appelé à la fin d'un scan : les dossiers observés dans les clés sont créés s'ils
manquent, et ceux qui n'existaient que par la présence de fichiers désormais
partis disparaissent. Les dossiers créés à la main survivent à l'élagage.
*/
func (s *Service) Observe(ctx context.Context, libraryID uuid.UUID, paths []string) error {
	if err := s.ensurePaths(ctx, libraryID, paths); err != nil {
		return err
	}

	_, err := s.repo.PruneEmpty(ctx, libraryID)
	return err
}

/*
Ensure inscrit un dossier rencontré, sans élaguer.

Appelé à chaque dépôt de fichier : déposer un album dans un dossier neuf doit le
faire apparaître tout de suite, pas au prochain parcours. L'élagage n'a rien à
faire ici — il suppose une vue complète de la bibliothèque, qu'un dépôt isolé
n'a pas.
*/
func (s *Service) Ensure(ctx context.Context, libraryID uuid.UUID, path string) error {
	return s.ensurePaths(ctx, libraryID, []string{path})
}

func (s *Service) ensurePaths(ctx context.Context, libraryID uuid.UUID, paths []string) error {
	seen := make(map[string]struct{}, len(paths)*2)

	for _, path := range paths {
		clean := NormalizePath(path)
		if clean == "" {
			continue
		}
		for _, node := range append(ancestorsOf(clean), clean) {
			if node == "" {
				continue
			}
			seen[node] = struct{}{}
		}
	}

	// Triés : un parent est ainsi toujours inséré avant ses enfants, ce qui
	// garde l'arborescence cohérente même si l'opération est interrompue.
	ordered := make([]string, 0, len(seen))
	for path := range seen {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)

	for _, path := range ordered {
		if _, err := s.repo.Upsert(ctx, folderAt(libraryID, path, false)); err != nil {
			return err
		}
	}

	// La racine existe toujours : elle porte les albums non rangés.
	_, err := s.repo.Upsert(ctx, folderAt(libraryID, "", true))
	return err
}

// ─── Chemins ─────────────────────────────────────────────────────────────────

/*
NormalizePath ramène un chemin fourni par un client à une forme sûre.

Les segments vides, « . » et « .. » disparaissent, ainsi que les caractères de
contrôle. Sans cela, un dossier « ../../ » désignerait un emplacement hors de la
bibliothèque, et le renommage y déplacerait des fichiers.
*/
func NormalizePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")

	segments := make([]string, 0, 4)
	for _, segment := range strings.Split(path, "/") {
		segment = strings.TrimSpace(segment)
		segment = strings.TrimFunc(segment, unicode.IsControl)
		segment = strings.Trim(segment, ".")
		if segment == "" {
			continue
		}
		segments = append(segments, segment)
	}
	return strings.Join(segments, "/")
}

// ancestorsOf liste les ancêtres d'un chemin, de la racine au parent direct.
func ancestorsOf(path string) []string {
	if path == "" {
		return nil
	}

	segments := strings.Split(path, "/")
	out := make([]string, 0, len(segments))
	out = append(out, "")

	for i := 1; i < len(segments); i++ {
		out = append(out, strings.Join(segments[:i], "/"))
	}
	return out
}

func parentOf(path string) string {
	index := strings.LastIndex(path, "/")
	if index < 0 {
		return ""
	}
	return path[:index]
}

func lastSegment(path string) string {
	index := strings.LastIndex(path, "/")
	if index < 0 {
		return path
	}
	return path[index+1:]
}

func depthOf(path string) int {
	if path == "" {
		return 0
	}
	return strings.Count(path, "/") + 1
}

func folderAt(libraryID uuid.UUID, path string, explicit bool) Folder {
	return Folder{
		ID:        uuid.Must(uuid.NewV7()),
		LibraryID: libraryID,
		Path:      path,
		Name:      lastSegment(path),
		Depth:     depthOf(path),
		Explicit:  explicit,
	}
}

func joinKey(prefix, path string) string {
	if prefix == "" {
		return path
	}
	if path == "" {
		return prefix
	}
	return prefix + "/" + path
}

func folderOfKey(objectKey, prefix string) string {
	rest := objectKey
	if prefix != "" {
		rest = strings.TrimPrefix(rest, prefix+"/")
	}
	index := strings.LastIndex(rest, "/")
	if index < 0 {
		return ""
	}
	return rest[:index]
}

// unlockedPaths indexe les dossiers dont le code a été saisi et n'a pas expiré.
func (s *Service) unlockedPaths(
	ctx context.Context, userID uuid.UUID, libraryIDs []uuid.UUID,
) (map[string]bool, error) {
	if s.locks == nil {
		return map[string]bool{}, nil
	}

	list, err := s.locks.LockedFolders(ctx, userID, libraryIDs)
	if err != nil {
		return nil, err
	}

	out := make(map[string]bool, len(list))
	for _, folder := range list {
		out[folder.Path] = folder.UnlockedUntil != nil
	}
	return out, nil
}
