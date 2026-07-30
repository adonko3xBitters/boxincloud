package local_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage/local"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage/storagetest"
)

func newProvider(t *testing.T) *local.Provider {
	t.Helper()
	p, err := local.New(local.Options{Root: t.TempDir()})
	if err != nil {
		t.Fatalf("New : %v", err)
	}
	return p
}

// Le provider local doit passer la même suite de conformité que S3 : c'est ce
// qui permet aux modules métier d'ignorer le backend.
func TestConformance(t *testing.T) {
	storagetest.RunSuite(t, newProvider(t))
}

func TestNewRejectsMissingRoot(t *testing.T) {
	if _, err := local.New(local.Options{Root: ""}); err == nil {
		t.Error("une racine vide devrait être refusée")
	}
	if _, err := local.New(local.Options{Root: "/nulle/part/absolument"}); err == nil {
		t.Error("une racine inexistante devrait être refusée")
	}
}

func TestNewRejectsFileAsRoot(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "fichier")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := local.New(local.Options{Root: file}); err == nil {
		t.Error("un fichier ne devrait pas être accepté comme racine")
	}
}

// La traversée de chemin est le risque principal de ce provider : une clé issue
// d'une archive ou d'une saisie utilisateur ne doit jamais pouvoir sortir de la
// racine.
func TestPathTraversalIsBlocked(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Dir(root)

	secret := filepath.Join(parent, "secret.txt")
	if err := os.WriteFile(secret, []byte("ne doit pas fuiter"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(secret) })

	p, err := local.New(local.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	for _, key := range []string{
		"../secret.txt",
		"../../secret.txt",
		"sub/../../secret.txt",
		"./../secret.txt",
	} {
		t.Run(key, func(t *testing.T) {
			r, err := p.Open(ctx, key)
			if err == nil {
				_ = r.Close()
				t.Fatalf("la clé %q a permis de sortir de la racine", key)
			}
			// Selon le chemin, la clé est soit rejetée, soit résolue vers un
			// fichier inexistant sous la racine. Les deux sont acceptables ;
			// ce qui compte est de ne jamais lire le fichier hors racine.
			if !errors.Is(err, storage.ErrPermissionDenied) && !errors.Is(err, storage.ErrNotFound) {
				t.Errorf("erreur inattendue pour %q : %v", key, err)
			}
		})
	}
}

func TestReadOnlyRefusesWrites(t *testing.T) {
	dir := t.TempDir()
	p, err := local.New(local.Options{Root: dir, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if err := p.Write(ctx, "x.txt", strings.NewReader("x"), 1, ""); !errors.Is(err, storage.ErrReadOnly) {
		t.Errorf("Write : attendu ErrReadOnly, obtenu %v", err)
	}
	if err := p.Delete(ctx, "x.txt"); !errors.Is(err, storage.ErrReadOnly) {
		t.Errorf("Delete : attendu ErrReadOnly, obtenu %v", err)
	}
}

func TestStatOnDirectoryIsNotFound(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "serie"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := local.New(local.Options{Root: dir})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := p.Stat(context.Background(), "serie"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Stat sur un répertoire : attendu ErrNotFound, obtenu %v", err)
	}
}

// Un répertoire ne doit pas apparaître dans List : seuls les objets comptent.
func TestListSkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "serie", "vide"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "serie", "t01.cbz"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	p, err := local.New(local.Options{Root: dir})
	if err != nil {
		t.Fatal(err)
	}

	var keys []string
	if err := p.List(context.Background(), "", func(o storage.ObjectInfo) error {
		keys = append(keys, o.Key)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if len(keys) != 1 || keys[0] != "serie/t01.cbz" {
		t.Errorf("List = %v, attendu [serie/t01.cbz]", keys)
	}
}

// Les clés sont en slash quelle que soit la plateforme : le reste du code ne
// doit jamais voir de séparateur Windows.
func TestKeysUseForwardSlashes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "b", "c.cbz"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	p, err := local.New(local.Options{Root: dir})
	if err != nil {
		t.Fatal(err)
	}

	var key string
	if err := p.List(context.Background(), "", func(o storage.ObjectInfo) error {
		key = o.Key
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if key != "a/b/c.cbz" {
		t.Errorf("clé = %q, attendu \"a/b/c.cbz\"", key)
	}
}
