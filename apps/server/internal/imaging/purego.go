package imaging

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/webp"
	xdraw "golang.org/x/image/draw"
)

// PureGo est l'implémentation de référence de Processor, sans cgo.
//
// Elle produit les quatre formats du projet. Le WebP et l'AVIF passent par des
// encodeurs compilés en WebAssembly et exécutés par wazero : c'est du Go pur au
// sens qui compte ici — `CGO_ENABLED=0` compile, le binaire reste unique, et
// une compilation croisée vers ARM ne demande aucune chaîne C.
type PureGo struct {
	// Quality par défaut pour les formats avec perte. Zéro vaut 85, un bon
	// compromis pour de la bande dessinée.
	Quality int
}

/*
Réglages des encodeurs modernes, et pourquoi ceux-là.

Mesuré sur une planche synthétique de 1600×2400 — aplats, trait encré, bruit
de scan — et sur ses réductions :

	                  page 1600×2400     vignette 320 px
	JPEG q85           846 Ko /  52 ms    38,9 Ko /   2 ms
	WebP q80           503 Ko / 148 ms    27,1 Ko /   6 ms
	AVIF q60 vitesse 8 322 Ko / 2,1 s     14,6 Ko / 130 ms
	AVIF q60 vitesse 10 533 Ko / 663 ms

Ces chiffres décident de tout le reste, et pas dans le sens qu'on attendait :
**l'AVIF assez rapide pour être encodé pendant qu'un lecteur attend sa page est
moins bon que le WebP sur les deux axes à la fois** — plus gros ET presque aussi
lent. L'AVIF qui gagne vraiment coûte deux secondes.

Le format suit donc qui paie l'encodage. Une page qu'on transcode pendant que
quelqu'un attend : WebP, 40 % de moins qu'en JPEG pour 100 ms. Une vignette
encodée une fois et servie par soixante dans une grille : AVIF, 62 % de moins,
et les 130 ms sont payées une seule fois dans l'existence de l'album.

La qualité 60 en AVIF n'est pas la qualité 60 en JPEG : les échelles n'ont
aucun rapport, et 60 y correspond à peu près à ce qu'on attend d'un q85 JPEG.
*/
const (
	webpQuality = 80
	avifQuality = 60
	// Vitesse 8 sur 10 : le compromis retenu là où l'encodage n'est pas dans
	// le chemin d'attente. Voir le tableau ci-dessus pour ce que coûtent les
	// crans voisins.
	avifSpeed = 8
)

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
	case FormatWebP:
		if err := webp.Encode(dst, img, webp.Options{Quality: webpQuality}); err != nil {
			return out, fmt.Errorf("imaging : encodage WebP : %w", err)
		}
	case FormatAVIF:
		if err := avif.Encode(dst, img, avif.Options{
			Quality: avifQuality,
			Speed:   avifSpeed,
		}); err != nil {
			return out, fmt.Errorf("imaging : encodage AVIF : %w", err)
		}
	default:
		return out, fmt.Errorf("%w : %s", ErrUnsupportedFormat, opts.Format)
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
