// Package imaging redimensionne et réencode les pages.
//
// L'interface Processor isole le moteur de traitement du reste du code. M1
// fournit une implémentation en Go pur : elle ne demande pas cgo, permet un
// build CGO_ENABLED=0 et fonctionne sur toutes les architectures.
//
// Les quatre formats de sortie sont produits sans cgo : le JPEG et le PNG par
// la bibliothèque standard, le WebP et l'AVIF par des encodeurs compilés en
// WebAssembly qu'exécute wazero. Le binaire reste unique et se compile en
// croisé sans chaîne C.
//
// Une implémentation libvips reste possible derrière la même interface, pour
// les instances qui indexent des dizaines de milliers d'albums : elle est
// nettement plus rapide et plus économe en mémoire. Elle impose cgo, ce qui est
// un prix que la grande majorité des installations n'a aucune raison de payer.
package imaging

import (
	"errors"
	"fmt"
	"image"
	"io"

	// Enregistrent les décodeurs auprès de image.Decode.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	// En entrée aussi : les CBZ récents contiennent de plus en plus de pages
	// déjà en WebP ou en AVIF, et un album illisible parce qu'on ne décode pas
	// son format serait une drôle de façon de se moderniser.
	_ "github.com/gen2brain/avif"
)

// Format identifie un format d'image en sortie.
type Format string

const (
	FormatJPEG Format = "jpeg"
	FormatPNG  Format = "png"
	FormatWebP Format = "webp"
	FormatAVIF Format = "avif"
)

// ContentType retourne le type MIME associé au format.
func (f Format) ContentType() string {
	switch f {
	case FormatJPEG:
		return "image/jpeg"
	case FormatPNG:
		return "image/png"
	case FormatWebP:
		return "image/webp"
	case FormatAVIF:
		return "image/avif"
	default:
		return "application/octet-stream"
	}
}

// Extension retourne l'extension de fichier associée, utilisée dans les clés de
// cache.
func (f Format) Extension() string {
	switch f {
	case FormatJPEG:
		return ".jpg"
	case FormatPNG:
		return ".png"
	case FormatWebP:
		return ".webp"
	case FormatAVIF:
		return ".avif"
	default:
		return ".bin"
	}
}

var (
	// ErrUnsupportedFormat signale un format que le moteur ne sait pas produire.
	ErrUnsupportedFormat = errors.New("imaging : format de sortie non supporté")
	// ErrDecode signale une image illisible.
	ErrDecode = errors.New("imaging : image illisible")
)

// Tailles de vignettes produites à l'indexation.
//
// Trois tailles couvrent les usages : grille dense, grille confortable, page de
// détail. Au-delà, le client demande la page elle-même.
const (
	ThumbSmall  = 160
	ThumbMedium = 320
	ThumbLarge  = 640
)

// ThumbSizes est l'ensemble des largeurs générées pour une couverture.
var ThumbSizes = []int{ThumbSmall, ThumbMedium, ThumbLarge}

// PlaceholderWidth est la largeur du LQIP — l'aperçu affiché flouté pendant le
// chargement d'une couverture.
//
// 16 px produit un JPEG d'environ 870 octets une fois en base64 — l'essentiel
// étant l'en-tête et les tables de quantification, presque incompressibles à
// cette taille. Élargir n'ajouterait donc que peu à la charge utile, mais le
// flou appliqué à l'affichage rend tout détail supplémentaire invisible :
// 16 px est le point où l'on paie le minimum pour tout le bénéfice.
//
// Coût mesuré : ~52 Ko de JSON supplémentaires sur une page de 60 albums, en
// échange de 60 aperçus disponibles immédiatement. Le compromis penche
// nettement du bon côté sur un réseau local, et reste acceptable à distance.
const PlaceholderWidth = 16

// Options décrit une transformation.
type Options struct {
	// Width est la largeur cible. La hauteur suit le ratio d'origine.
	// Zéro laisse l'image à sa taille.
	Width int
	// Format de sortie.
	Format Format
	// Quality de 1 à 100, pour les formats avec perte. Zéro prend le défaut.
	Quality int
}

// Info décrit une image sans la transformer.
type Info struct {
	Width  int
	Height int
	Format string // format source, tel que détecté
}

// IsDouble indique si l'image est une double planche.
//
// Un ratio nettement supérieur à 1 signale une planche double : le lecteur doit
// l'afficher seule en mode double page, sans l'apparier avec une voisine.
func (i Info) IsDouble() bool {
	return i.Height > 0 && float64(i.Width)/float64(i.Height) > 1.2
}

// Processor transforme des images.
//
// Les implémentations doivent être sûres d'emploi concurrent : plusieurs
// vignettes se génèrent en parallèle pendant un scan.
type Processor interface {
	// Inspect lit les dimensions sans décoder l'image entière.
	Inspect(r io.Reader) (Info, error)
	// Transform lit une image, la redimensionne et la réencode.
	Transform(dst io.Writer, src io.Reader, opts Options) (Info, error)
}

// Inspect lit les dimensions d'une image sans la décoder entièrement.
//
// image.DecodeConfig ne lit que l'en-tête : quelques centaines d'octets au lieu
// de plusieurs mégaoctets. Sur un scan de milliers de pages, la différence est
// décisive.
func Inspect(r io.Reader) (Info, error) {
	cfg, format, err := image.DecodeConfig(r)
	if err != nil {
		return Info{}, fmt.Errorf("%w : %w", ErrDecode, err)
	}
	return Info{Width: cfg.Width, Height: cfg.Height, Format: format}, nil
}
