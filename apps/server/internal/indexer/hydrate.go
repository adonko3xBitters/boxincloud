package indexer

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/archive"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
)

/*
Hydratation d'une archive sans accès aléatoire.

Le projet tient sur une promesse : servir une page coûte UNE requête Range sur
l'archive distante. Le ZIP la permet nativement. Le RAR ne le permettra jamais —
ses archives solides compressent les fichiers comme un flux continu — et le PDF
demanderait un moteur de rendu qu'on ne veut pas embarquer.

Plutôt que d'exclure ces formats ou de mentir sur leur support, on les convertit
une fois pour toutes : lire l'original de bout en bout, réécrire ses images en
CBZ, déposer ce CBZ dans le cache dérivé. Toute lecture ultérieure retombe
exactement sur le chemin du CBZ — un ReadRange par page, sans exception ni
branchement.

Le coût est payé une fois, à l'indexation, plutôt qu'à chaque page et pour
toujours. Et il est payé par le serveur, jamais par le lecteur.

Trois décisions méritent d'être dites.

**Le CBZ produit est en STORED, sans compression.** Les images qu'on y remet
sont déjà des JPEG ou des PNG : les redéflater coûterait du temps de processeur
pour gagner un ou deux pour cent. Stored a en plus le mérite de rendre
`DataOffset` directement exploitable — la page se sert sans même passer par un
décompresseur.

**Rien n'est écrit dans le stockage de l'utilisateur.** L'archive hydratée vit
dans le cache dérivé, qui est le seul emplacement dont boxincloud est
propriétaire. Le fichier d'origine n'est ni modifié, ni déplacé, ni doublé chez
lui.

**Le passage par un fichier temporaire est assumé.** Écrire un ZIP demande de
revenir en arrière pour son index de fin, ce qu'un flux vers un stockage objet
ne permet pas. L'alternative — tout garder en mémoire — ferait tomber le
serveur sur une intégrale de huit cents méga-octets.
*/

// HydratedPrefix est le préfixe des archives normalisées dans le cache dérivé.
const HydratedPrefix = "hydrated"

// HydratedKey construit la clé de l'archive normalisée d'un album.
func HydratedKey(comicID uuid.UUID) string {
	return path.Join(HydratedPrefix, comicID.String()+".cbz")
}

// maxHydratedBytes borne la taille de l'archive produite.
//
// Une intégrale numérisée dépasse rarement le gigaoctet ; la borne protège d'un
// PDF qui prétendrait contenir dix mille pages de trente méga-octets — cas
// improbable, mais qui remplirait le disque du serveur avant qu'on s'en
// aperçoive.
const maxHydratedBytes = 4 << 30

// ErrHydrationTooLarge signale une archive dont l'hydratation dépasse la borne.
var ErrHydrationTooLarge = errors.New("indexer : archive hydratée trop volumineuse")

/*
Hydrate convertit une archive sans accès aléatoire en CBZ dans le cache.

Retourne la clé de l'archive produite. L'appelant l'enregistre sur l'album : à
partir de là, toute lecture passe par le cache et non plus par le stockage
d'origine.
*/
func Hydrate(
	ctx context.Context,
	source storage.Provider,
	cache storage.Provider,
	comicID uuid.UUID,
	key string,
	format archive.Format,
) (string, error) {
	// Le format est vérifié avant tout accès au stockage. Hydrater un CBZ est
	// une erreur de programmation, pas un cas limite : elle doit coûter un
	// retour immédiat, pas un aller-retour réseau sur une archive de plusieurs
	// centaines de méga-octets.
	if format.SupportsRandomAccess() {
		return "", fmt.Errorf("%w : %s n'a pas besoin d'hydratation",
			archive.ErrUnsupportedFormat, format)
	}

	temp, err := os.CreateTemp("", "boxincloud-hydrate-*.cbz")
	if err != nil {
		return "", fmt.Errorf("fichier temporaire : %w", err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()

	written, err := writeNormalized(ctx, source, key, format, temp)
	if err != nil {
		return "", err
	}

	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("relecture du temporaire : %w", err)
	}

	target := HydratedKey(comicID)
	if err := cache.Write(ctx, target, temp, written, "application/vnd.comicbook+zip"); err != nil {
		return "", fmt.Errorf("écriture de l'archive hydratée : %w", err)
	}

	return target, nil
}

// writeNormalized lit l'original et écrit le CBZ. Retourne sa taille.
func writeNormalized(
	ctx context.Context,
	source storage.Provider,
	key string,
	format archive.Format,
	out *os.File,
) (int64, error) {
	original, err := source.Open(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("ouverture de %q : %w", key, err)
	}
	defer func() { _ = original.Close() }()

	writer := zip.NewWriter(out)

	var total int64
	add := func(entry archive.ExtractedEntry) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		w, err := writer.CreateHeader(&zip.FileHeader{
			Name:   path.Base(entry.Name),
			Method: zip.Store,
		})
		if err != nil {
			return err
		}

		n, err := io.Copy(w, entry.Reader)
		if err != nil {
			return err
		}

		total += n
		if total > maxHydratedBytes {
			return ErrHydrationTooLarge
		}
		return nil
	}

	switch format {
	case archive.FormatCBR:
		err = archive.WalkRAR(original, add)

	case archive.FormatPDF:
		// pdfcpu exige de pouvoir revenir en arrière dans le fichier : la table
		// de références croisées d'un PDF est en fin de document et renvoie à
		// des objets situés n'importe où. Un flux ne suffit donc pas, et
		// l'original transite par le disque avant d'être lu.
		var seeker io.ReadSeeker
		seeker, err = spool(original)
		if err == nil {
			err = archive.WalkPDFImages(seeker, add)
		}

	default:
		// Un format sans accès aléatoire que personne n'a appris à lire : le
		// CB7 et l'EPUB sont dans ce cas. Refusé nommément plutôt que traité
		// comme une archive vide.
		err = fmt.Errorf("%w : hydratation de %s non implémentée",
			archive.ErrUnsupportedFormat, format)
	}

	if err != nil {
		_ = writer.Close()
		return 0, err
	}

	if err := writer.Close(); err != nil {
		return 0, fmt.Errorf("clôture de l'archive : %w", err)
	}

	size, err := out.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	return size, nil
}

// spool recopie un flux dans un fichier temporaire navigable.
//
// Le fichier est supprimé de l'arborescence dès sa création : il n'existe plus
// que par son descripteur, et disparaît donc même si le processus est tué. Rien
// à nettoyer, rien à oublier de nettoyer.
func spool(r io.Reader) (io.ReadSeeker, error) {
	f, err := os.CreateTemp("", "boxincloud-pdf-*")
	if err != nil {
		return nil, err
	}
	if err := os.Remove(f.Name()); err != nil {
		_ = f.Close()
		return nil, err
	}

	if _, err := io.Copy(f, io.LimitReader(r, maxHydratedBytes)); err != nil {
		_ = f.Close()
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}
