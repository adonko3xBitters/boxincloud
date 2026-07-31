package archive_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adonko3xBitters/boxincloud/server/internal/archive"
)

/*
Lecture des formats sans accès aléatoire.

Ces tests construisent de vraies archives plutôt que des octets fabriqués à la
main : un RAR écrit à la main testerait ma compréhension du format, pas le
décodeur. Le RAR exige l'outil `rar`, qui n'est pas libre et n'est présent
nulle part par défaut — le test se saute proprement en son absence plutôt que
d'échouer sur une machine qui ne l'a pas.
*/

func TestWalkRARIgnoreLesNonImages(t *testing.T) {
	path := buildRAR(t, map[string][]byte{
		"page-01.jpg":   jpegBytes(t),
		"page-02.jpg":   jpegBytes(t),
		"ComicInfo.xml": []byte("<ComicInfo/>"),
		"notes.txt":     []byte("rien à voir"),
	})

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()

	var names []string
	err = archive.WalkRAR(file, func(e archive.ExtractedEntry) error {
		names = append(names, e.Name)
		// Le lecteur DOIT être consommé : le décodeur avance en flux, et
		// sauter une entrée décalerait toutes les suivantes.
		if _, err := readAll(e); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkRAR : %v", err)
	}

	if len(names) != 2 {
		t.Errorf("entrées = %v, attendu les deux seules images", names)
	}
	for _, name := range names {
		if !strings.HasSuffix(name, ".jpg") {
			t.Errorf("entrée non-image retenue : %s", name)
		}
	}
}

func TestWalkRARSansImage(t *testing.T) {
	path := buildRAR(t, map[string][]byte{"notes.txt": []byte("rien")})

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()

	err = archive.WalkRAR(file, func(archive.ExtractedEntry) error { return nil })
	if err == nil {
		t.Fatal("WalkRAR = nil sur une archive sans image, attendu ErrNoPages")
	}
	if !strings.Contains(err.Error(), "aucune page") {
		t.Errorf("erreur = %v, attendu ErrNoPages", err)
	}
}

func TestWalkRARCorrompu(t *testing.T) {
	// Une archive tronquée doit être refusée franchement, pas produire zéro
	// page en silence — la distinction change ce que l'utilisateur voit.
	err := archive.WalkRAR(bytes.NewReader([]byte("ceci n'est pas un rar")),
		func(archive.ExtractedEntry) error { return nil })

	if err == nil {
		t.Fatal("WalkRAR = nil sur des octets arbitraires")
	}
}

// ─── Outils ──────────────────────────────────────────────────────────────────

// buildRAR fabrique une archive RAR avec l'outil du système.
func buildRAR(t *testing.T, files map[string][]byte) string {
	t.Helper()

	rar, err := exec.LookPath("rar")
	if err != nil {
		t.Skip("l'outil `rar` est absent : test sauté")
	}

	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(source, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	target := filepath.Join(dir, "album.cbr")
	cmd := exec.Command(rar, "a", "-ep", target, ".")
	cmd.Dir = source
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("création de l'archive : %v\n%s", err, out)
	}
	return target
}

// jpegBytes produit une planche minuscule mais valide.
func jpegBytes(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 8, 12))
	for y := range 12 {
		for x := range 8 {
			img.Set(x, y, color.RGBA{R: 200, G: 120, B: 60, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func readAll(e archive.ExtractedEntry) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(e.Reader)
	return buf.Bytes(), err
}
