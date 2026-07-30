// Package imaging redimensionne et réencode les pages.
//
// L'interface Processor isole le moteur de traitement du reste du code. M1
// fournit une implémentation en Go pur : elle ne demande pas cgo, permet un
// build CGO_ENABLED=0 et fonctionne sur toutes les architectures.
//
// Une implémentation libvips (via govips) viendra derrière la même interface :
// elle est 4 à 8 fois plus rapide et bien plus économe en mémoire sur des
// planches haute résolution, et sait produire du WebP et de l'AVIF. Elle impose
// en revanche cgo et une image Docker plus lourde, ce qui n'a pas sa place dans
// le jalon qui doit d'abord prouver l'accès aléatoire au stockage objet.
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
)

// Format identifie un format d'image en sortie.
type Format string

const (
	FormatJPEG Format = "jpeg"
	FormatPNG  Format = "png"
	// FormatWebP et FormatAVIF seront produits par l'implémentation libvips.
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
