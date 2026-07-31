package imaging_test

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"testing"

	"github.com/adonko3xBitters/boxincloud/server/internal/imaging"
)

/*
Le moteur d'images.

Deux choses à prouver, et elles sont de nature différente.

La première est que ce qu'on encode se relit : un encodeur qui écrit des octets
sans erreur mais illisibles produirait des albums entièrement gris, et l'erreur
ne se verrait qu'à l'affichage — c'est-à-dire nulle part dans une suite de
tests. On réencode donc, puis on redécode.

La seconde est que la négociation est stricte. C'est une décision, pas un
détail d'implémentation : elle protège les clients qui ne déclarent rien.
*/

// planche fabrique une image plausible : des aplats et du trait, ce dont sont
// faites les pages de bande dessinée.
func planche(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	for i := 0; i < 12; i++ {
		x, y := (i*97)%width, (i*61)%height
		shade := uint8(20 + i*18)
		draw.Draw(img,
			image.Rect(x, y, x+width/4, y+height/6),
			&image.Uniform{color.RGBA{shade, uint8(255 - shade), shade, 255}},
			image.Point{}, draw.Src)
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if (x+y)%37 < 2 {
				img.Set(x, y, color.Black)
			}
		}
	}
	return img
}

func sourceJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, planche(width, height), &jpeg.Options{Quality: 92}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestTransformRoundTrip vérifie que chaque format annoncé se produit et se
// relit.
//
// Le décodage est le cœur du test. Un `Encode` qui retourne nil prouve
// seulement que l'encodeur n'a pas planté ; il ne prouve pas qu'un navigateur
// affichera quoi que ce soit.
func TestTransformRoundTrip(t *testing.T) {
	source := sourceJPEG(t, 800, 1200)
	processor := imaging.NewPureGo()

	cases := []struct {
		format imaging.Format
		mime   string
	}{
		{imaging.FormatJPEG, "image/jpeg"},
		{imaging.FormatPNG, "image/png"},
		{imaging.FormatWebP, "image/webp"},
		{imaging.FormatAVIF, "image/avif"},
	}

	for _, tc := range cases {
		t.Run(string(tc.format), func(t *testing.T) {
			var out bytes.Buffer
			info, err := processor.Transform(&out, bytes.NewReader(source), imaging.Options{
				Width:  320,
				Format: tc.format,
			})
			if err != nil {
				t.Fatalf("transformation : %v", err)
			}
			if info.Width != 320 {
				t.Errorf("largeur = %d, attendu 320", info.Width)
			}
			if out.Len() == 0 {
				t.Fatal("sortie vide")
			}
			if tc.format.ContentType() != tc.mime {
				t.Errorf("type MIME = %s, attendu %s", tc.format.ContentType(), tc.mime)
			}

			decoded, _, err := image.Decode(bytes.NewReader(out.Bytes()))
			if err != nil {
				t.Fatalf("le format produit ne se relit pas : %v", err)
			}
			if got := decoded.Bounds().Dx(); got != 320 {
				t.Errorf("l'image relue fait %d px de large, attendu 320", got)
			}
		})
	}
}

// TestModernFormatsAreSmaller garde le bénéfice qui justifie tout ce travail.
//
// Sans cette assertion, une régression de réglage — une qualité poussée à 95,
// un encodeur qui retombe sur du sans-perte — passerait inaperçue : les images
// resteraient correctes, seulement plus lourdes qu'en JPEG, ce qui rendrait la
// négociation activement nuisible.
func TestModernFormatsAreSmaller(t *testing.T) {
	source := sourceJPEG(t, 800, 1200)
	processor := imaging.NewPureGo()

	size := func(format imaging.Format) int {
		t.Helper()
		var out bytes.Buffer
		if _, err := processor.Transform(&out, bytes.NewReader(source), imaging.Options{
			Width:  640,
			Format: format,
		}); err != nil {
			t.Fatal(err)
		}
		return out.Len()
	}

	reference := size(imaging.FormatJPEG)
	for _, format := range []imaging.Format{imaging.FormatWebP, imaging.FormatAVIF} {
		if got := size(format); got >= reference {
			t.Errorf("%s pèse %d octets contre %d en JPEG : la négociation "+
				"ferait perdre de la bande passante au lieu d'en gagner",
				format, got, reference)
		}
	}
}

func TestUnsupportedFormat(t *testing.T) {
	var out bytes.Buffer
	_, err := imaging.NewPureGo().Transform(&out, bytes.NewReader(sourceJPEG(t, 64, 64)),
		imaging.Options{Format: imaging.Format("jpeg-xl")})
	if err == nil {
		t.Fatal("un format inconnu doit être refusé")
	}
}

// TestNegotiate fixe la règle : une mention explicite, ou le repli.
func TestNegotiate(t *testing.T) {
	pageOffer := []imaging.Format{imaging.FormatWebP, imaging.FormatJPEG}
	coverOffer := []imaging.Format{imaging.FormatAVIF, imaging.FormatWebP, imaging.FormatJPEG}

	cases := []struct {
		name   string
		accept string
		offer  []imaging.Format
		want   imaging.Format
	}{
		{
			name:   "chrome moderne sur une couverture",
			accept: "image/avif,image/webp,image/apng,image/svg+xml,image/*;q=0.8,*/*;q=0.5",
			offer:  coverOffer,
			want:   imaging.FormatAVIF,
		},
		{
			name:   "chrome moderne sur une page : l'AVIF n'y est pas proposé",
			accept: "image/avif,image/webp,image/apng,*/*;q=0.8",
			offer:  pageOffer,
			want:   imaging.FormatWebP,
		},
		{
			name:   "safari ancien : WebP oui, AVIF non",
			accept: "image/webp,image/png,image/svg+xml,image/*;q=0.8,*/*;q=0.5",
			offer:  coverOffer,
			want:   imaging.FormatWebP,
		},
		{
			name:   "un joker ne vaut pas une déclaration",
			accept: "*/*",
			offer:  coverOffer,
			want:   imaging.FormatJPEG,
		},
		{
			name:   "un joker d'images non plus",
			accept: "image/*",
			offer:  coverOffer,
			want:   imaging.FormatJPEG,
		},
		{
			name:   "aucun en-tête",
			accept: "",
			offer:  coverOffer,
			want:   imaging.FormatJPEG,
		},
		{
			name:   "q=0 exclut ce qu'il nomme",
			accept: "image/avif;q=0,image/webp",
			offer:  coverOffer,
			want:   imaging.FormatWebP,
		},
		{
			name:   "espaces et casse sont sans effet",
			accept: "  IMAGE/AVIF ;q=1.0 ,  image/webp ",
			offer:  coverOffer,
			want:   imaging.FormatAVIF,
		},
		{
			name:   "un client qui ne veut que du JPEG le reçoit",
			accept: "image/jpeg",
			offer:  coverOffer,
			want:   imaging.FormatJPEG,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := imaging.Negotiate(tc.accept, tc.offer...); got != tc.want {
				t.Errorf("Negotiate(%q) = %s, attendu %s", tc.accept, got, tc.want)
			}
		})
	}
}
