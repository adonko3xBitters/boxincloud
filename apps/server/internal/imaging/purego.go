package imaging

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	xdraw "golang.org/x/image/draw"
)

// PureGo est l'implémentation de référence de Processor, sans cgo.
//
// Suffisante pour l'usage courant : générer les vignettes d'une couverture et
// réduire une planche à la largeur demandée par le client. Elle sera doublée
// d'une implémentation libvips pour les instances qui indexent des milliers
// d'albums.
type PureGo struct {
	// Quality par défaut pour les formats avec perte. Zéro vaut 85, un bon
	// compromis pour de la bande dessinée.
	Quality int
}

var _ Processor = (*PureGo)(nil)

func NewPureGo() *PureGo { return &PureGo{Quality: 85} }

func (p *PureGo) Inspect(r io.Reader) (Info, error) { return Inspect(r) }

func (p *PureGo) Transform(dst io.Writer, src io.Reader, opts Options) (Info, error) {
	img, format, err := image.Decode(src)
	if err != nil {
		return Info{}, fmt.Errorf("%w : %w", ErrDecode, err)
	}

	bounds := img.Bounds()
	info := Info{Width: bounds.Dx(), Height: bounds.Dy(), Format: format}

	// On ne réduit jamais vers le haut : agrandir une planche scannée ne fait
	// qu'alourdir le transfert sans rien apporter visuellement.
	if opts.Width > 0 && opts.Width < info.Width {
		img = resizeToWidth(img, opts.Width)
		bounds = img.Bounds()
	}

	out := Info{Width: bounds.Dx(), Height: bounds.Dy(), Format: format}

	quality := opts.Quality
	if quality <= 0 {
		quality = p.Quality
	}
	if quality <= 0 {
		quality = 85
	}

	switch opts.Format {
	case FormatJPEG, "":
		if err := jpeg.Encode(dst, img, &jpeg.Options{Quality: quality}); err != nil {
			return out, fmt.Errorf("imaging : encodage JPEG : %w", err)
		}
	case FormatPNG:
		enc := png.Encoder{CompressionLevel: png.DefaultCompression}
		if err := enc.Encode(dst, img); err != nil {
			return out, fmt.Errorf("imaging : encodage PNG : %w", err)
		}
	default:
		return out, fmt.Errorf("%w : %s (l'implémentation Go pur produit du JPEG et du PNG ; "+
			"WebP et AVIF demanderont le moteur libvips)", ErrUnsupportedFormat, opts.Format)
	}

	return out, nil
}

// resizeToWidth redimensionne en conservant le ratio.
//
// CatmullRom plutôt qu'un filtre bilinéaire : sur du trait encré et du
// lettrage — l'essentiel d'une planche de BD — la différence de netteté est
// nettement visible, pour un coût acceptable à la génération d'une vignette.
func resizeToWidth(src image.Image, width int) image.Image {
	b := src.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return src
	}

	height := int(float64(width) * float64(b.Dy()) / float64(b.Dx()))
	if height < 1 {
		height = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	return dst
}
