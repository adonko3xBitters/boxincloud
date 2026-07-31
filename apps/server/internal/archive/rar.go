package archive

import (
	"errors"
	"fmt"
	"io"

	"github.com/nwaples/rardecode/v2"
)

/*
Lecture d'une archive RAR.

Le RAR n'a pas d'équivalent utilisable de l'index de fin du ZIP. Pire : ses
archives « solides » compressent les fichiers comme un flux continu, si bien que
lire la page 40 impose de décompresser les 39 précédentes. L'accès aléatoire y
est donc structurellement impossible — ce n'est pas une limite de cette
implémentation, c'est le format.

D'où l'hydratation : on lit l'archive UNE fois, de bout en bout, et on la
réécrit en CBZ dans le cache dérivé. Toute lecture ultérieure retombe sur le
chemin normal — un ReadRange par page. Le coût est payé une fois, à
l'indexation, plutôt qu'à chaque page et pour toujours.

Le décodeur est en Go pur : l'image du serveur reste un binaire statique, sans
`unrar` à installer ni cgo à compiler.
*/

// ExtractedEntry est une entrée lue en flux depuis une archive sans accès
// aléatoire.
//
// Le lecteur n'est valable que pendant l'appel de la fonction de visite : le
// décodeur avance dans le flux, et le conserver au-delà lirait les octets de
// l'entrée suivante.
type ExtractedEntry struct {
	Name   string
	Reader io.Reader
}

// ErrEncrypted signale une archive protégée par mot de passe.
var ErrEncrypted = errors.New("archive : archive chiffrée")

/*
WalkRAR parcourt les images d'une archive RAR, dans l'ordre du fichier.

L'ordre n'a pas d'importance : l'index du CBZ produit trie ensuite par ordre
naturel des noms, qui est le seul ordre de lecture qui vaille. L'ordre de
stockage dans l'archive n'a aucune raison d'être le bon.
*/
func WalkRAR(r io.Reader, visit func(ExtractedEntry) error) error {
	reader, err := rardecode.NewReader(r)
	if err != nil {
		return fmt.Errorf("%w : %v", ErrCorrupted, err)
	}

	found := 0

	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if errors.Is(err, rardecode.ErrArchivedFileEncrypted) {
				return ErrEncrypted
			}
			return fmt.Errorf("%w : %v", ErrCorrupted, err)
		}

		if header.IsDir || isJunk(header.Name) || !IsImage(header.Name) {
			continue
		}

		found++
		if err := visit(ExtractedEntry{Name: header.Name, Reader: reader}); err != nil {
			return err
		}
	}

	if found == 0 {
		return ErrNoPages
	}
	return nil
}
