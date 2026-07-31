package archive

import (
	"fmt"
	"io"
	"sort"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

/*
Lecture d'un PDF de bande dessinée.

Les images sont EXTRAITES, pas rendues. La nuance est décisive : rendre une page
demanderait un moteur de rendu PDF — pdfium ou MuPDF, tous deux en C, tous deux
incompatibles avec un binaire statique — et produirait une image pixellisée à
une définition arbitraire, forcément inférieure ou supérieure à l'originale.

L'extraction, elle, rend les octets tels qu'ils sont dans le fichier. Pour un
scan de bande dessinée, cela suffit et c'est même mieux : ces PDF-là sont
presque toujours un JPEG plein format par page, déposé sans transformation. On
récupère donc exactement la planche numérisée.

La contrepartie est nommée : un PDF de bande dessinée NATIVE — du texte et des
vecteurs, sans image de fond — ne donnera rien à extraire. Le cas existe (les
éditions numériques natives) et se solde par une erreur explicite plutôt que
par un album vide. Le rendre correctement demanderait le moteur de rendu qu'on
a écarté ; le prétendre serait pire que de le refuser.
*/

// WalkPDFImages parcourt les images d'un PDF, une par page, dans l'ordre des
// pages.
//
// Le lecteur passé à `visit` n'est valable que pendant l'appel.
func WalkPDFImages(rs io.ReadSeeker, visit func(ExtractedEntry) error) error {
	images, err := api.ExtractImagesRaw(rs, nil, model.NewDefaultConfiguration())
	if err != nil {
		return fmt.Errorf("%w : %v", ErrCorrupted, err)
	}

	found := 0

	for _, perPage := range images {
		// Une page peut porter plusieurs images — un fond scanné plus un
		// filigrane, par exemple. On les prend toutes, dans un ordre stable :
		// le tri par numéro d'objet est arbitraire mais reproductible, là où
		// l'ordre d'une map ne l'est pas.
		objects := make([]int, 0, len(perPage))
		for obj := range perPage {
			objects = append(objects, obj)
		}
		sort.Ints(objects)

		for _, obj := range objects {
			img := perPage[obj]
			if img.Reader == nil {
				continue
			}

			// Le nom porte le numéro de page sur quatre chiffres : c'est lui
			// qui déterminera l'ordre de lecture après hydratation, l'index du
			// CBZ triant par ordre naturel des noms.
			name := fmt.Sprintf("page-%04d-%03d.%s", img.PageNr, found%1000, extensionFor(img))

			found++
			if err := visit(ExtractedEntry{Name: name, Reader: img.Reader}); err != nil {
				return err
			}
		}
	}

	if found == 0 {
		return fmt.Errorf("%w : le PDF ne contient aucune image extractible "+
			"(bande dessinée native en texte et vecteurs ?)", ErrNoPages)
	}
	return nil
}

// extensionFor déduit l'extension du type que pdfcpu a reconnu.
//
// Elle doit être une extension d'image reconnue par `IsImage`, faute de quoi
// l'entrée serait écartée à la réindexation du CBZ produit — l'album serait
// hydraté puis déclaré vide, ce qui est le pire des deux mondes.
func extensionFor(img model.Image) string {
	switch img.FileType {
	case "jpg", "jpeg":
		return "jpg"
	case "png":
		return "png"
	case "tif", "tiff":
		return "tif"
	case "webp":
		return "webp"
	default:
		// pdfcpu écrit du PNG pour tout ce qu'il a dû recomposer.
		return "png"
	}
}
