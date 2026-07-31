package archive

import (
	"fmt"
	"io"

	"github.com/bodgit/sevenzip"
)

/*
Lecture d'une archive 7z.

Même raisonnement que le RAR : le format autorise en principe un accès par
entrée, mais ses flux sont compressés par blocs partagés — plusieurs fichiers
dans un même bloc LZMA. Lire la page 40 impose alors de décompresser tout le
bloc qui la contient, dont les 39 précédentes si elles y sont. L'accès aléatoire
au sens du projet — une requête Range, une page — n'y est pas possible.

D'où l'hydratation, exactement comme pour le CBR.

Le 7z exige de pouvoir revenir en arrière dans le fichier : son index est en fin
d'archive et renvoie à des blocs situés n'importe où. L'appelant lui fournit
donc un lecteur navigable, pas un flux.
*/

// WalkSevenZip parcourt les images d'une archive 7z, dans l'ordre du fichier.
//
// L'ordre importe peu : l'index du CBZ produit trie ensuite par ordre naturel
// des noms, qui est le seul ordre de lecture qui vaille.
func WalkSevenZip(r io.ReaderAt, size int64, visit func(ExtractedEntry) error) error {
	reader, err := sevenzip.NewReader(r, size)
	if err != nil {
		return fmt.Errorf("%w : %v", ErrCorrupted, err)
	}

	found := 0

	for _, file := range reader.File {
		if file.FileInfo().IsDir() || isJunk(file.Name) || !IsImage(file.Name) {
			continue
		}

		entry, err := file.Open()
		if err != nil {
			return fmt.Errorf("%w : %v", ErrCorrupted, err)
		}

		err = visit(ExtractedEntry{Name: file.Name, Reader: entry})
		_ = entry.Close()
		if err != nil {
			return err
		}
		found++
	}

	if found == 0 {
		return ErrNoPages
	}
	return nil
}
