package library

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// Folder est un nœud de l'arborescence d'une bibliothèque.
//
// L'arbre est renvoyé à plat : le client le reconstitue en une passe, ce qui
// évite une requête récursive côté base pour un résultat qu'un parcours simple
// suffit à bâtir.
type Folder struct {
	// Path est le chemin complet, relatif au préfixe de la bibliothèque.
	// Vide pour la racine.
	Path string
	// Name est le dernier segment — ce qui s'affiche dans l'arbre.
	Name string
	// Depth vaut 0 à la racine.
	Depth int
	// ComicCount compte les albums de ce dossier et de ses sous-dossiers.
	ComicCount int
}

// BuildFolderTree complète une liste de dossiers observés.
//
// Deux traitements que la base ne fait pas :
//
//   - les dossiers intermédiaires sans album direct sont ajoutés. Une
//     bibliothèque contenant seulement « BD/Franco-belge/Tintin » doit afficher
//     « BD » et « Franco-belge », sinon l'arbre est troué.
//   - les compteurs remontent : un dossier affiche le total de sa branche, ce
//     qu'attend quelqu'un qui replie un nœud.
func BuildFolderTree(observed map[string]int) []Folder {
	totals := make(map[string]int, len(observed)*2)

	for path, count := range observed {
		// Le dossier lui-même, plus chacun de ses ancêtres.
		totals[path] += count

		for {
			idx := strings.LastIndex(path, "/")
			if idx < 0 {
				break
			}
			path = path[:idx]
			totals[path] += count
		}
	}

	// La racine agrège tout, y compris les albums qui n'ont pas de dossier.
	if _, ok := totals[""]; !ok {
		total := 0
		for _, count := range observed {
			total += count
		}
		totals[""] = total
	}

	out := make([]Folder, 0, len(totals))
	for path, count := range totals {
		name := path
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			name = path[idx+1:]
		}

		depth := 0
		if path != "" {
			depth = strings.Count(path, "/") + 1
		}

		out = append(out, Folder{
			Path:       path,
			Name:       name,
			Depth:      depth,
			ComicCount: count,
		})
	}

	// Tri lexicographique : il place naturellement un parent avant ses enfants,
	// ce qui permet au client de construire l'arbre en une seule passe.
	sortFolders(out)
	return out
}

func sortFolders(folders []Folder) {
	for i := 1; i < len(folders); i++ {
		for j := i; j > 0 && folders[j].Path < folders[j-1].Path; j-- {
			folders[j], folders[j-1] = folders[j-1], folders[j]
		}
	}
}

// FolderLister est ce dont le service a besoin pour l'arborescence.
type FolderLister interface {
	ListFolders(ctx context.Context, libraryIDs []uuid.UUID) (map[string]int, error)
}

// Folders retourne l'arborescence des bibliothèques visibles.
func (s *Service) Folders(ctx context.Context, lister FolderLister, libraryIDs []uuid.UUID) ([]Folder, error) {
	observed, err := lister.ListFolders(ctx, libraryIDs)
	if err != nil {
		return nil, err
	}
	return BuildFolderTree(observed), nil
}
