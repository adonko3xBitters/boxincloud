// Package comicfixture fabrique des archives de BD pour les tests.
//
// Les fixtures sont générées plutôt que versionnées : on évite des binaires
// dans le dépôt, on garde la maîtrise du contenu exact, et on peut produire à
// la demande une archive volumineuse pour démontrer qu'une page se sert sans
// la télécharger.
package comicfixture

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// Options décrit l'archive à produire.
type Options struct {
	// Pages est le nombre de pages générées.
	Pages int

	// PageWidth et PageHeight dimensionnent chaque page.
	PageWidth, PageHeight int

	// NameFormat produit le nom d'une entrée à partir de son index 0-based.
	// Par défaut : "page01.jpg", "page02.jpg"…
	NameFormat func(i int) string

	// Store désactive la compression (méthode 0 au lieu de deflate). Les JPEG
	// étant déjà compressés, beaucoup d'outils de BD produisent des archives
	// stockées : le cas doit être couvert.
	Store bool

	// ComicInfo ajoute un ComicInfo.xml avec ce contenu.
	ComicInfo string

	// ExtraFiles ajoute des entrées supplémentaires (bruit, fichiers macOS…).
	ExtraFiles map[string][]byte
}

// Pages n'a volontairement pas de valeur par défaut : une archive sans page
// est un cas de test légitime (archive vide, contenant seulement du bruit), et
// un défaut silencieux le rendrait impossible à exprimer.
func (o *Options) applyDefaults() {
	if o.PageWidth == 0 {
		o.PageWidth = 120
	}
	if o.PageHeight == 0 {
		o.PageHeight = 180
	}
	if o.NameFormat == nil {
		o.NameFormat = func(i int) string { return fmt.Sprintf("page%02d.jpg", i+1) }
	}
}

// Built décrit l'archive produite et ce qu'elle est censée contenir, pour que
// les tests puissent vérifier sans redériver la même logique.
type Built struct {
	Data []byte
	// PageNames est la liste des noms de pages, dans l'ordre de lecture attendu.
	PageNames []string
	// PageContents associe chaque nom de page à ses octets décompressés.
	PageContents map[string][]byte
}

// BuildCBZ produit une archive CBZ en mémoire.
func BuildCBZ(t *testing.T, opts Options) Built {
	t.Helper()
	opts.applyDefaults()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	method := zip.Deflate
	if opts.Store {
		method = zip.Store
	}

	built := Built{PageContents: make(map[string][]byte)}

	for i := range opts.Pages {
		name := opts.NameFormat(i)
		content := renderPage(t, i, opts.PageWidth, opts.PageHeight)

		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			t.Fatalf("comicfixture : création de %s : %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("comicfixture : écriture de %s : %v", name, err)
		}

		built.PageNames = append(built.PageNames, name)
		built.PageContents[name] = content
	}

	if opts.ComicInfo != "" {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: "ComicInfo.xml", Method: zip.Deflate})
		if err != nil {
			t.Fatalf("comicfixture : création de ComicInfo.xml : %v", err)
		}
		if _, err := w.Write([]byte(opts.ComicInfo)); err != nil {
			t.Fatalf("comicfixture : écriture de ComicInfo.xml : %v", err)
		}
	}

	for name, content := range opts.ExtraFiles {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			t.Fatalf("comicfixture : création de %s : %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("comicfixture : écriture de %s : %v", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("comicfixture : fermeture de l'archive : %v", err)
	}

	built.Data = buf.Bytes()
	return built
}

// renderPage produit une image JPEG déterministe et reconnaissable.
//
// Chaque page a une teinte propre dérivée de son index : un test qui lit la
// page 12 peut vérifier qu'il a bien obtenu la page 12, et pas une voisine.
func renderPage(t *testing.T, index, w, h int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	base := toByte((index*37 + 30) % 256)

	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{
				R: base,
				G: toByte((x * 255) / max(w-1, 1)),
				B: toByte((y * 255) / max(h-1, 1)),
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("comicfixture : encodage JPEG : %v", err)
	}
	return buf.Bytes()
}

// toByte borne une composante de couleur à l'intervalle valide.
func toByte(v int) uint8 {
	switch {
	case v < 0:
		return 0
	case v > 255:
		return 255
	default:
		return uint8(v)
	}
}

// SampleComicInfo est un ComicInfo.xml minimal mais réaliste.
const SampleComicInfo = `<?xml version="1.0" encoding="utf-8"?>
<ComicInfo xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <Title>Le Secret de la Licorne</Title>
  <Series>Les Aventures de Tintin</Series>
  <Number>11</Number>
  <Volume>1943</Volume>
  <Summary>Tintin achète un modèle réduit de La Licorne aux puces.</Summary>
  <Year>1943</Year>
  <Month>6</Month>
  <Writer>Hergé</Writer>
  <Penciller>Hergé</Penciller>
  <Publisher>Casterman</Publisher>
  <Genre>Aventure</Genre>
  <LanguageISO>fr</LanguageISO>
  <PageCount>62</PageCount>
  <AgeRating>Everyone</AgeRating>
</ComicInfo>`
