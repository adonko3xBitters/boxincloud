package indexer_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"github.com/adonko3xBitters/boxincloud/server/internal/archive"
	"github.com/adonko3xBitters/boxincloud/server/internal/indexer"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage/local"
)

/*
Hydratation, de bout en bout.

Le PDF sert de véhicule parce que c'est le seul des deux formats sans accès
aléatoire qu'on puisse FABRIQUER ici : créer un RAR demande l'outil `rar`, qui
n'est pas libre. La machinerie testée est pourtant la même — même écriture du
CBZ, même dépôt dans le cache, même relecture par l'index ZIP. Seul le
parcourreur diffère, et c'est lui que `archive/hydrate_test.go` couvre quand
l'outil est présent.

Ce que ces tests vérifient tient en une phrase : après hydratation, un format
sans accès aléatoire se lit exactement comme un CBZ — par un ReadRange sur des
offsets persistés. C'est la promesse du projet, et elle ne doit pas avoir
d'exception.
*/

func TestHydratePDF(t *testing.T) {
	ctx := context.Background()

	source, cache := providers(t)
	key := "bd/album.pdf"
	writePDF(t, ctx, source, key, 3)

	comicID := uuid.Must(uuid.NewV7())

	hydrated, err := indexer.Hydrate(ctx, source, cache, comicID, key, archive.FormatPDF)
	if err != nil {
		t.Fatalf("Hydrate : %v", err)
	}

	if hydrated != indexer.HydratedKey(comicID) {
		t.Errorf("clé = %q, attendu %q", hydrated, indexer.HydratedKey(comicID))
	}

	// L'archive produite doit être lisible par le chemin normal — c'est tout
	// l'intérêt de la conversion.
	info, err := cache.Stat(ctx, hydrated)
	if err != nil {
		t.Fatalf("archive hydratée introuvable : %v", err)
	}

	idx, err := archive.ReadZipIndex(ctx, cache, hydrated, info.Size)
	if err != nil {
		t.Fatalf("index de l'archive hydratée : %v", err)
	}

	if len(idx.Pages) != 3 {
		t.Fatalf("pages = %d, attendu 3", len(idx.Pages))
	}
}

func TestHydratePDFPagesLisiblesParRange(t *testing.T) {
	ctx := context.Background()

	source, cache := providers(t)
	key := "bd/album.pdf"
	writePDF(t, ctx, source, key, 2)

	comicID := uuid.Must(uuid.NewV7())
	hydrated, err := indexer.Hydrate(ctx, source, cache, comicID, key, archive.FormatPDF)
	if err != nil {
		t.Fatalf("Hydrate : %v", err)
	}

	info, _ := cache.Stat(ctx, hydrated)
	idx, err := archive.ReadZipIndex(ctx, cache, hydrated, info.Size)
	if err != nil {
		t.Fatal(err)
	}

	// ★ Le point que tout le reste sert : servir une page ne demande qu'un
	// ReadRange sur des coordonnées. Si ceci passe, un CBR et un PDF coûtent
	// exactement ce que coûte un CBZ.
	for i, entry := range idx.Pages {
		r, err := archive.OpenEntry(ctx, cache, hydrated, entry)
		if err != nil {
			t.Fatalf("page %d : %v", i, err)
		}

		var buf bytes.Buffer
		if _, err := buf.ReadFrom(r); err != nil {
			t.Fatalf("page %d : lecture : %v", i, err)
		}
		_ = r.Close()

		if _, _, err := image.Decode(bytes.NewReader(buf.Bytes())); err != nil {
			t.Errorf("page %d : les octets servis ne sont pas une image : %v", i, err)
		}
	}
}

func TestHydrateStockeSansRecompresser(t *testing.T) {
	// Les images d'un PDF sont déjà compressées : les redéflater coûterait du
	// processeur pour un ou deux pour cent. Stored a en plus le mérite de
	// rendre l'offset directement exploitable, sans décompresseur.
	ctx := context.Background()

	source, cache := providers(t)
	writePDF(t, ctx, source, "album.pdf", 2)

	comicID := uuid.Must(uuid.NewV7())
	hydrated, err := indexer.Hydrate(ctx, source, cache, comicID, "album.pdf", archive.FormatPDF)
	if err != nil {
		t.Fatal(err)
	}

	info, _ := cache.Stat(ctx, hydrated)
	idx, _ := archive.ReadZipIndex(ctx, cache, hydrated, info.Size)

	for _, entry := range idx.Pages {
		if entry.Compression != archive.CompressionStore {
			t.Errorf("%s : compression = %d, attendu stored", entry.Name, entry.Compression)
		}
	}
}

func TestHydrateRefuseUnFormatQuiNEnADemandePas(t *testing.T) {
	// Hydrater un CBZ serait une copie inutile de plusieurs centaines de
	// méga-octets. L'appel est une erreur de programmation, pas un cas limite.
	ctx := context.Background()
	source, cache := providers(t)

	_, err := indexer.Hydrate(ctx, source, cache,
		uuid.Must(uuid.NewV7()), "album.cbz", archive.FormatCBZ)

	if err == nil {
		t.Fatal("Hydrate = nil sur un CBZ")
	}
	if !strings.Contains(err.Error(), "besoin d'hydratation") {
		t.Errorf("erreur = %v, attendu un refus explicite", err)
	}
}

func TestHydrateSurArchiveIllisible(t *testing.T) {
	ctx := context.Background()
	source, cache := providers(t)

	if err := source.Write(ctx, "faux.pdf",
		bytes.NewReader([]byte("ceci n'est pas un PDF")), 21, "application/pdf"); err != nil {
		t.Fatal(err)
	}

	_, err := indexer.Hydrate(ctx, source, cache,
		uuid.Must(uuid.NewV7()), "faux.pdf", archive.FormatPDF)

	if err == nil {
		t.Fatal("Hydrate = nil sur des octets arbitraires")
	}
}

// ─── Outils ──────────────────────────────────────────────────────────────────

func providers(t *testing.T) (storage.Provider, storage.Provider) {
	t.Helper()

	// Le provider local exige que sa racine existe : il refuse de la créer,
	// pour qu'un chemin mal saisi échoue tout de suite plutôt que de produire
	// une arborescence fantôme quelque part.
	root := t.TempDir()

	make := func(name string) storage.Provider {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		p, err := local.New(local.Options{Root: dir})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}

	return make("source"), make("cache")
}

// writePDF fabrique un PDF d'une image par page et le dépose sur le provider.
//
// Un vrai PDF, produit par la bibliothèque qui le relira ensuite : un fichier
// fabriqué à la main testerait ma compréhension du format, pas l'extraction.
func writePDF(t *testing.T, ctx context.Context, p storage.Provider, key string, pages int) {
	t.Helper()

	images := make([]io.Reader, 0, pages)
	for i := range pages {
		images = append(images, bytes.NewReader(pageJPEG(t, i)))
	}

	var out bytes.Buffer
	if err := api.ImportImages(nil, &out, images,
		pdfcpu.DefaultImportConfig(), model.NewDefaultConfiguration()); err != nil {
		t.Fatalf("création du PDF : %v", err)
	}

	if err := p.Write(ctx, key, bytes.NewReader(out.Bytes()),
		int64(out.Len()), "application/pdf"); err != nil {
		t.Fatal(err)
	}
}

// pageJPEG produit une planche unie, reconnaissable à sa teinte.
func pageJPEG(t *testing.T, index int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 120, 180))
	shade := uint8(40 + index*60)
	for y := range 180 {
		for x := range 120 {
			img.Set(x, y, color.RGBA{R: shade, G: 90, B: 160, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
