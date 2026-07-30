package archive_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/adonko3xBitters/boxincloud/server/internal/archive"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage/local"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage/storagetest"
	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/comicfixture"
)

// hostArchive écrit une archive sur un provider local et retourne de quoi la
// lire, avec un compteur de requêtes Range.
func hostArchive(t *testing.T, data []byte) (*storagetest.RangeCounter, string, int64) {
	t.Helper()

	dir := t.TempDir()
	key := "serie/album.cbz"
	full := filepath.Join(dir, filepath.FromSlash(key))

	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, data, 0o600); err != nil {
		t.Fatal(err)
	}

	p, err := local.New(local.Options{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	return storagetest.NewRangeCounter(p), key, int64(len(data))
}

func TestReadZipIndex(t *testing.T) {
	built := comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 8})
	p, key, size := hostArchive(t, built.Data)

	idx, err := archive.ReadZipIndex(context.Background(), p, key, size)
	if err != nil {
		t.Fatalf("ReadZipIndex : %v", err)
	}

	if len(idx.Pages) != 8 {
		t.Fatalf("%d pages indexées, attendu 8", len(idx.Pages))
	}

	for i, page := range idx.Pages {
		want := built.PageNames[i]
		if page.Name != want {
			t.Errorf("page %d : nom %q, attendu %q", i, page.Name, want)
		}
		if page.DataOffset <= 0 {
			t.Errorf("page %d (%s) : DataOffset nul", i, page.Name)
		}
		if page.DataSize <= 0 {
			t.Errorf("page %d (%s) : DataSize nulle", i, page.Name)
		}
		if page.Size != int64(len(built.PageContents[want])) {
			t.Errorf("page %d : Size = %d, attendu %d", i, page.Size, len(built.PageContents[want]))
		}
	}
}

// Le test central du projet : lire une page précise ne doit coûter qu'une seule
// requête Range et transférer les octets de cette page, pas ceux de l'archive.
func TestOpenEntryCostsOneRangeRequest(t *testing.T) {
	// 40 pages en 800×1200 : une archive de plusieurs mégaoctets, assez grande
	// pour que la différence soit sans ambiguïté.
	built := comicfixture.BuildCBZ(t, comicfixture.Options{
		Pages:      40,
		PageWidth:  800,
		PageHeight: 1200,
	})
	p, key, size := hostArchive(t, built.Data)

	idx, err := archive.ReadZipIndex(context.Background(), p, key, size)
	if err != nil {
		t.Fatalf("ReadZipIndex : %v", err)
	}

	// L'index est fait : on repart de zéro pour ne mesurer que la lecture.
	p.Reset()

	const wanted = 11 // la 12ᵉ page
	page := idx.Pages[wanted]

	r, err := archive.OpenEntry(context.Background(), p, key, page)
	if err != nil {
		t.Fatalf("OpenEntry : %v", err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("lecture : %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("fermeture : %v", err)
	}

	want := built.PageContents[built.PageNames[wanted]]
	if !bytes.Equal(got, want) {
		t.Fatalf("contenu de la page %d incorrect (%d octets lus, %d attendus)", wanted, len(got), len(want))
	}

	if p.Calls() != 1 {
		t.Errorf("%d requêtes Range pour servir une page, attendu 1", p.Calls())
	}
	if p.Bytes() >= size {
		t.Errorf("%d octets transférés pour une archive de %d : la page n'a pas été lue par plage",
			p.Bytes(), size)
	}

	t.Logf("page %d servie en %s — archive de %d octets, soit %.2f %% transférés",
		wanted+1, p, size, float64(p.Bytes())/float64(size)*100)
}

// L'indexation elle-même ne doit pas télécharger l'archive : c'est ce qui rend
// le scan d'une bibliothèque distante viable.
func TestIndexingDoesNotDownloadArchive(t *testing.T) {
	built := comicfixture.BuildCBZ(t, comicfixture.Options{
		Pages:      30,
		PageWidth:  800,
		PageHeight: 1200,
	})
	p, key, size := hostArchive(t, built.Data)

	if _, err := archive.ReadZipIndex(context.Background(), p, key, size); err != nil {
		t.Fatalf("ReadZipIndex : %v", err)
	}

	if p.Bytes() >= size {
		t.Errorf("l'indexation a transféré %d octets pour une archive de %d", p.Bytes(), size)
	}
	t.Logf("indexation de %d pages : %s — %.2f %% de l'archive",
		len(built.PageNames), p, float64(p.Bytes())/float64(size)*100)
}

// Les JPEG étant déjà compressés, beaucoup d'outils produisent des archives
// stockées plutôt que dégonflées. Les deux doivent fonctionner.
func TestReadZipIndexStoredEntries(t *testing.T) {
	built := comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 4, Store: true})
	p, key, size := hostArchive(t, built.Data)

	idx, err := archive.ReadZipIndex(context.Background(), p, key, size)
	if err != nil {
		t.Fatalf("ReadZipIndex : %v", err)
	}

	for i, page := range idx.Pages {
		if page.Compression != archive.CompressionStore {
			t.Errorf("page %d : compression %d, attendu Store", i, page.Compression)
		}

		r, err := archive.OpenEntry(context.Background(), p, key, page)
		if err != nil {
			t.Fatalf("OpenEntry(%s) : %v", page.Name, err)
		}
		got, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatalf("lecture de %s : %v", page.Name, err)
		}
		if !bytes.Equal(got, built.PageContents[page.Name]) {
			t.Errorf("contenu incorrect pour %s", page.Name)
		}
	}
}

// Toutes les pages doivent être lisibles, pas seulement celles testées à la
// main : une erreur d'offset se voit souvent sur la première ou la dernière.
func TestAllPagesRoundTrip(t *testing.T) {
	built := comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 12})
	p, key, size := hostArchive(t, built.Data)

	idx, err := archive.ReadZipIndex(context.Background(), p, key, size)
	if err != nil {
		t.Fatalf("ReadZipIndex : %v", err)
	}

	for i, page := range idx.Pages {
		r, err := archive.OpenEntry(context.Background(), p, key, page)
		if err != nil {
			t.Fatalf("OpenEntry(page %d) : %v", i, err)
		}
		got, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatalf("lecture de la page %d : %v", i, err)
		}
		if !bytes.Equal(got, built.PageContents[page.Name]) {
			t.Errorf("page %d (%s) : contenu incorrect", i, page.Name)
		}
	}
}

// Sans tri naturel, « page10 » précéderait « page2 » et l'album se lirait dans
// le désordre — le défaut le plus visible qu'un lecteur puisse avoir.
func TestPagesAreSortedNaturally(t *testing.T) {
	built := comicfixture.BuildCBZ(t, comicfixture.Options{
		Pages:      12,
		NameFormat: func(i int) string { return fmt.Sprintf("page%d.jpg", i+1) }, // sans zéros
	})
	p, key, size := hostArchive(t, built.Data)

	idx, err := archive.ReadZipIndex(context.Background(), p, key, size)
	if err != nil {
		t.Fatalf("ReadZipIndex : %v", err)
	}

	var got []string
	for _, page := range idx.Pages {
		got = append(got, page.Name)
	}

	want := []string{
		"page1.jpg", "page2.jpg", "page3.jpg", "page4.jpg", "page5.jpg", "page6.jpg",
		"page7.jpg", "page8.jpg", "page9.jpg", "page10.jpg", "page11.jpg", "page12.jpg",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordre des pages incorrect :\nobtenu  %v\nattendu %v", got, want)
		}
	}
}

func TestComicInfoIsExtracted(t *testing.T) {
	built := comicfixture.BuildCBZ(t, comicfixture.Options{
		Pages:     3,
		ComicInfo: comicfixture.SampleComicInfo,
	})
	p, key, size := hostArchive(t, built.Data)

	idx, err := archive.ReadZipIndex(context.Background(), p, key, size)
	if err != nil {
		t.Fatalf("ReadZipIndex : %v", err)
	}

	if idx.ComicInfo == nil {
		t.Fatal("ComicInfo.xml non détecté")
	}
	if len(idx.Pages) != 3 {
		t.Errorf("%d pages, attendu 3 : ComicInfo.xml a été compté comme une page", len(idx.Pages))
	}

	r, err := archive.OpenEntry(context.Background(), p, key, *idx.ComicInfo)
	if err != nil {
		t.Fatalf("OpenEntry(ComicInfo) : %v", err)
	}
	got, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != comicfixture.SampleComicInfo {
		t.Error("contenu de ComicInfo.xml incorrect")
	}
}

// Les archives créées sur macOS embarquent __MACOSX/ et des fichiers ._nom :
// des images valides par l'extension, du bruit pour le lecteur.
func TestJunkEntriesAreIgnored(t *testing.T) {
	built := comicfixture.BuildCBZ(t, comicfixture.Options{
		Pages: 3,
		ExtraFiles: map[string][]byte{
			"__MACOSX/._page01.jpg": []byte("junk"),
			"._page02.jpg":          []byte("junk"),
			".DS_Store":             []byte("junk"),
			"Thumbs.db":             []byte("junk"),
			"lisezmoi.txt":          []byte("pas une image"),
		},
	})
	p, key, size := hostArchive(t, built.Data)

	idx, err := archive.ReadZipIndex(context.Background(), p, key, size)
	if err != nil {
		t.Fatalf("ReadZipIndex : %v", err)
	}

	if len(idx.Pages) != 3 {
		var names []string
		for _, p := range idx.Pages {
			names = append(names, p.Name)
		}
		t.Errorf("%d pages, attendu 3 : %v", len(idx.Pages), names)
	}
}

func TestReadZipIndexRejectsNonZip(t *testing.T) {
	p, key, size := hostArchive(t, bytes.Repeat([]byte("pas une archive zip"), 100))

	_, err := archive.ReadZipIndex(context.Background(), p, key, size)
	if !errors.Is(err, archive.ErrCorrupted) {
		t.Errorf("attendu ErrCorrupted, obtenu %v", err)
	}
}

func TestReadZipIndexRejectsTooShort(t *testing.T) {
	p, key, size := hostArchive(t, []byte("court"))

	_, err := archive.ReadZipIndex(context.Background(), p, key, size)
	if !errors.Is(err, archive.ErrCorrupted) {
		t.Errorf("attendu ErrCorrupted, obtenu %v", err)
	}
}

func TestReadZipIndexRejectsArchiveWithoutImages(t *testing.T) {
	built := comicfixture.BuildCBZ(t, comicfixture.Options{
		Pages: 0,
		ExtraFiles: map[string][]byte{
			"lisezmoi.txt": []byte("aucune image ici"),
		},
	})
	p, key, size := hostArchive(t, built.Data)

	_, err := archive.ReadZipIndex(context.Background(), p, key, size)
	if !errors.Is(err, archive.ErrNoPages) {
		t.Errorf("attendu ErrNoPages, obtenu %v", err)
	}
}

// Une archive tronquée est un cas réel : téléversement interrompu, disque
// plein. Le message doit être clair, et surtout le serveur ne doit pas paniquer.
func TestReadZipIndexRejectsTruncatedArchive(t *testing.T) {
	built := comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 5})
	truncated := built.Data[:len(built.Data)/2]

	p, key, size := hostArchive(t, truncated)

	if _, err := archive.ReadZipIndex(context.Background(), p, key, size); err == nil {
		t.Error("une archive tronquée devrait être rejetée")
	}
}

func TestDetectFormat(t *testing.T) {
	cases := map[string]archive.Format{
		"serie/t01.cbz":    archive.FormatCBZ,
		"serie/T01.CBZ":    archive.FormatCBZ,
		"serie/t01.zip":    archive.FormatCBZ,
		"serie/t01.cbr":    archive.FormatCBR,
		"serie/t01.rar":    archive.FormatCBR,
		"serie/t01.cb7":    archive.FormatCB7,
		"serie/album.pdf":  archive.FormatPDF,
		"serie/roman.epub": archive.FormatEPUB,
	}

	for key, want := range cases {
		got, err := archive.DetectFormat(key)
		if err != nil {
			t.Errorf("DetectFormat(%q) : %v", key, err)
			continue
		}
		if got != want {
			t.Errorf("DetectFormat(%q) = %q, attendu %q", key, got, want)
		}
	}

	for _, key := range []string{"notes.txt", "image.jpg", "sans-extension"} {
		if _, err := archive.DetectFormat(key); !errors.Is(err, archive.ErrUnsupportedFormat) {
			t.Errorf("DetectFormat(%q) devrait rendre ErrUnsupportedFormat, obtenu %v", key, err)
		}
	}
}

// Seul le CBZ permet de servir une page sans hydratation préalable. C'est ce
// qui détermine la stratégie appliquée à l'ingestion.
func TestOnlyCBZSupportsRandomAccess(t *testing.T) {
	if !archive.FormatCBZ.SupportsRandomAccess() {
		t.Error("CBZ devrait supporter l'accès aléatoire")
	}
	for _, f := range []archive.Format{
		archive.FormatCBR, archive.FormatCB7, archive.FormatPDF, archive.FormatEPUB,
	} {
		if f.SupportsRandomAccess() {
			t.Errorf("%s ne devrait pas prétendre supporter l'accès aléatoire", f)
		}
	}
}

// Le lecteur doit fonctionner à l'identique sur n'importe quel backend : c'est
// toute la raison d'être de storage.Provider.
func TestWorksOverAnyProvider(t *testing.T) {
	built := comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 5})

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.cbz"), built.Data, 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := local.New(local.Options{Root: dir})
	if err != nil {
		t.Fatal(err)
	}

	var provider storage.Provider = p
	idx, err := archive.ReadZipIndex(context.Background(), provider, "a.cbz", int64(len(built.Data)))
	if err != nil {
		t.Fatalf("ReadZipIndex : %v", err)
	}
	if len(idx.Pages) != 5 {
		t.Errorf("%d pages, attendu 5", len(idx.Pages))
	}
}
