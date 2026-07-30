// Package storagetest fournit une suite de conformité pour storage.Provider.
//
// Toute implémentation doit passer cette suite. C'est ce qui garantit qu'un
// module métier peut ignorer le backend sous-jacent : si le provider local et
// le provider S3 se comportent identiquement ici, ils se comporteront
// identiquement en production.
//
// Les cas testés sont ceux qui ont réellement divergé entre fournisseurs :
// bornes des plages, longueur négative, offset hors limites, suppression d'un
// objet absent, sémantique des ETag.
package storagetest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
)

// RunSuite exécute la suite de conformité contre un provider.
//
// Le provider doit être vide et accessible en écriture. La suite nettoie ce
// qu'elle crée.
func RunSuite(t *testing.T, p storage.Provider) {
	t.Helper()

	ctx := context.Background()

	if err := p.Ping(ctx); err != nil {
		t.Fatalf("Ping : %v", err)
	}

	t.Run("WriteStatOpen", func(t *testing.T) { testWriteStatOpen(t, p) })
	t.Run("ReadRange", func(t *testing.T) { testReadRange(t, p) })
	t.Run("ReadRangeEdgeCases", func(t *testing.T) { testReadRangeEdges(t, p) })
	t.Run("List", func(t *testing.T) { testList(t, p) })
	t.Run("Delete", func(t *testing.T) { testDelete(t, p) })
	t.Run("Move", func(t *testing.T) { testMove(t, p) })
	t.Run("NotFound", func(t *testing.T) { testNotFound(t, p) })
}

const rangeContent = "0123456789abcdefghijklmnopqrstuvwxyz"

func testWriteStatOpen(t *testing.T, p storage.Provider) {
	ctx := context.Background()
	key := "suite/write/hello.txt"
	body := []byte("bonjour boxincloud")

	if err := p.Write(ctx, key, bytes.NewReader(body), int64(len(body)), "text/plain"); err != nil {
		t.Fatalf("Write : %v", err)
	}
	t.Cleanup(func() { _ = p.Delete(ctx, key) })

	info, err := p.Stat(ctx, key)
	if err != nil {
		t.Fatalf("Stat : %v", err)
	}
	if info.Size != int64(len(body)) {
		t.Errorf("Stat.Size = %d, attendu %d", info.Size, len(body))
	}
	if info.Key != key {
		t.Errorf("Stat.Key = %q, attendu %q", info.Key, key)
	}
	if info.ETag == "" {
		t.Error("Stat.ETag est vide : la détection de modification en dépend")
	}
	if info.ModTime.IsZero() {
		t.Error("Stat.ModTime est nul")
	}

	r, err := p.Open(ctx, key)
	if err != nil {
		t.Fatalf("Open : %v", err)
	}
	defer func() { _ = r.Close() }()

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("lecture : %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("Open a rendu %q, attendu %q", got, body)
	}
}

func testReadRange(t *testing.T, p storage.Provider) {
	ctx := context.Background()
	key := "suite/range/data.bin"
	body := []byte(rangeContent)

	if err := p.Write(ctx, key, bytes.NewReader(body), int64(len(body)), "application/octet-stream"); err != nil {
		t.Fatalf("Write : %v", err)
	}
	t.Cleanup(func() { _ = p.Delete(ctx, key) })

	cases := []struct {
		name           string
		offset, length int64
		want           string
	}{
		{"début", 0, 10, "0123456789"},
		{"milieu", 10, 6, "abcdef"},
		{"un octet", 5, 1, "5"},
		{"jusqu'à la fin", 26, -1, "qrstuvwxyz"},
		{"tout par longueur négative", 0, -1, rangeContent},
		{"dernier octet", 35, 1, "z"},
		{"longueur au-delà de la fin", 30, 100, "uvwxyz"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := p.ReadRange(ctx, key, c.offset, c.length)
			if err != nil {
				t.Fatalf("ReadRange(%d, %d) : %v", c.offset, c.length, err)
			}
			defer func() { _ = r.Close() }()

			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("lecture : %v", err)
			}
			if string(got) != c.want {
				t.Errorf("ReadRange(%d, %d) = %q, attendu %q", c.offset, c.length, got, c.want)
			}
		})
	}
}

func testReadRangeEdges(t *testing.T, p storage.Provider) {
	ctx := context.Background()
	key := "suite/range/edges.bin"
	body := []byte(rangeContent)

	if err := p.Write(ctx, key, bytes.NewReader(body), int64(len(body)), ""); err != nil {
		t.Fatalf("Write : %v", err)
	}
	t.Cleanup(func() { _ = p.Delete(ctx, key) })

	t.Run("offset négatif", func(t *testing.T) {
		r, err := p.ReadRange(ctx, key, -1, 10)
		if err == nil {
			_ = r.Close()
			t.Fatal("un offset négatif devrait être refusé")
		}
		if !errors.Is(err, storage.ErrInvalidRange) {
			t.Errorf("attendu ErrInvalidRange, obtenu %v", err)
		}
	})

	t.Run("offset au-delà de la taille", func(t *testing.T) {
		r, err := p.ReadRange(ctx, key, int64(len(body))+100, 10)
		if err == nil {
			_ = r.Close()
			t.Fatal("un offset au-delà de la taille devrait être refusé")
		}
		if !errors.Is(err, storage.ErrInvalidRange) {
			t.Errorf("attendu ErrInvalidRange, obtenu %v", err)
		}
	})

	t.Run("longueur nulle", func(t *testing.T) {
		r, err := p.ReadRange(ctx, key, 5, 0)
		if err != nil {
			t.Fatalf("ReadRange(5, 0) : %v", err)
		}
		defer func() { _ = r.Close() }()

		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("lecture : %v", err)
		}
		if len(got) != 0 {
			t.Errorf("une longueur nulle devrait rendre 0 octet, obtenu %d", len(got))
		}
	})
}

func testList(t *testing.T, p storage.Provider) {
	ctx := context.Background()

	keys := []string{
		"suite/list/a/one.cbz",
		"suite/list/a/two.cbz",
		"suite/list/b/three.cbz",
		"suite/other/four.cbz",
	}
	for _, k := range keys {
		if err := p.Write(ctx, k, strings.NewReader("x"), 1, ""); err != nil {
			t.Fatalf("Write %s : %v", k, err)
		}
		t.Cleanup(func() { _ = p.Delete(ctx, k) })
	}

	collect := func(prefix string) map[string]int64 {
		t.Helper()
		got := make(map[string]int64)
		if err := p.List(ctx, prefix, func(o storage.ObjectInfo) error {
			got[o.Key] = o.Size
			return nil
		}); err != nil {
			t.Fatalf("List(%q) : %v", prefix, err)
		}
		return got
	}

	t.Run("préfixe complet", func(t *testing.T) {
		got := collect("suite/list/")
		if len(got) != 3 {
			t.Errorf("List(\"suite/list/\") a rendu %d objets, attendu 3 : %v", len(got), got)
		}
		for _, want := range keys[:3] {
			if _, ok := got[want]; !ok {
				t.Errorf("objet manquant : %s", want)
			}
		}
		if _, ok := got["suite/other/four.cbz"]; ok {
			t.Error("List a rendu un objet hors du préfixe")
		}
	})

	t.Run("préfixe imbriqué", func(t *testing.T) {
		got := collect("suite/list/a/")
		if len(got) != 2 {
			t.Errorf("attendu 2 objets, obtenu %d : %v", len(got), got)
		}
	})

	t.Run("préfixe sans correspondance", func(t *testing.T) {
		got := collect("suite/inexistant/")
		if len(got) != 0 {
			t.Errorf("un préfixe sans correspondance devrait rendre une liste vide, obtenu %v", got)
		}
	})

	t.Run("interruption par le callback", func(t *testing.T) {
		sentinel := errors.New("stop")
		count := 0
		err := p.List(ctx, "suite/list/", func(storage.ObjectInfo) error {
			count++
			return sentinel
		})
		if !errors.Is(err, sentinel) {
			t.Errorf("List devrait remonter l'erreur du callback, obtenu %v", err)
		}
		if count != 1 {
			t.Errorf("List devrait s'arrêter au premier objet, %d appels", count)
		}
	})
}

func testDelete(t *testing.T, p storage.Provider) {
	ctx := context.Background()
	key := "suite/delete/gone.txt"

	if err := p.Write(ctx, key, strings.NewReader("éphémère"), -1, ""); err != nil {
		t.Fatalf("Write : %v", err)
	}
	if err := p.Delete(ctx, key); err != nil {
		t.Fatalf("Delete : %v", err)
	}
	if _, err := p.Stat(ctx, key); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("après Delete, Stat devrait rendre ErrNotFound, obtenu %v", err)
	}

	// Supprimer un objet absent n'est pas une erreur : cela rend les jobs de
	// nettoyage idempotents.
	if err := p.Delete(ctx, key); err != nil {
		t.Errorf("supprimer un objet absent ne devrait pas échouer, obtenu %v", err)
	}
}

/*
Move déplace sans recopier par le réseau, et refuse d'écraser.

Les deux backends y arrivent par des chemins très différents — copie côté
serveur pour S3, renommage pour un système de fichiers — et c'est justement ce
que cette suite existe pour rapprocher : leurs comportements observables doivent
être indiscernables.
*/
func testMove(t *testing.T, p storage.Provider) {
	ctx := context.Background()
	const content = "Le Secret de la Licorne"

	from := "suite/move/source.cbz"
	to := "suite/move/ranges/destination.cbz"

	if err := p.Write(ctx, from, strings.NewReader(content), -1, ""); err != nil {
		t.Fatalf("Write : %v", err)
	}

	if err := p.Move(ctx, from, to); err != nil {
		t.Fatalf("Move : %v", err)
	}

	// La source a disparu…
	if _, err := p.Stat(ctx, from); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("la source subsiste après Move : %v", err)
	}

	// …et la destination porte exactement les mêmes octets.
	reader, err := p.Open(ctx, to)
	if err != nil {
		t.Fatalf("Open de la destination : %v", err)
	}
	moved, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatalf("lecture de la destination : %v", err)
	}
	if string(moved) != content {
		t.Errorf("contenu déplacé = %q, attendu %q", moved, content)
	}

	t.Run("destination occupée", func(t *testing.T) {
		other := "suite/move/occupe.cbz"
		if err := p.Write(ctx, other, strings.NewReader("un autre album"), -1, ""); err != nil {
			t.Fatalf("Write : %v", err)
		}

		// Écraser détruirait un album que personne n'a demandé à perdre.
		if err := p.Move(ctx, other, to); !errors.Is(err, storage.ErrAlreadyExists) {
			t.Errorf("Move vers une destination occupée = %v, attendu ErrAlreadyExists", err)
		}

		// La source doit être intacte : un refus ne détruit rien.
		if _, err := p.Stat(ctx, other); err != nil {
			t.Errorf("la source a disparu malgré le refus : %v", err)
		}
	})

	t.Run("source absente", func(t *testing.T) {
		err := p.Move(ctx, "suite/move/jamais-ecrit.cbz", "suite/move/ailleurs.cbz")
		if !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("Move depuis une source absente = %v, attendu ErrNotFound", err)
		}
	})

	t.Run("même clé", func(t *testing.T) {
		// Un déplacement sur place est un non-événement, pas une erreur : cela
		// rend l'appelant plus simple, qui n'a pas à comparer les chemins.
		if err := p.Move(ctx, to, to); err != nil {
			t.Errorf("Move sur place = %v, attendu nil", err)
		}
		if _, err := p.Stat(ctx, to); err != nil {
			t.Errorf("l'objet a disparu après un déplacement sur place : %v", err)
		}
	})
}

func testNotFound(t *testing.T, p storage.Provider) {
	ctx := context.Background()
	key := "suite/absent/nulle-part.cbz"

	if _, err := p.Stat(ctx, key); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Stat : attendu ErrNotFound, obtenu %v", err)
	}

	if r, err := p.Open(ctx, key); !errors.Is(err, storage.ErrNotFound) {
		if err == nil {
			_ = r.Close()
		}
		t.Errorf("Open : attendu ErrNotFound, obtenu %v", err)
	}

	if r, err := p.ReadRange(ctx, key, 0, 10); !errors.Is(err, storage.ErrNotFound) {
		if err == nil {
			_ = r.Close()
		}
		t.Errorf("ReadRange : attendu ErrNotFound, obtenu %v", err)
	}
}

// RangeCounter enveloppe un Provider en comptant les appels à ReadRange et les
// octets transférés.
//
// Sert à démontrer la promesse centrale du projet : servir une page ne doit
// coûter qu'une requête Range et quelques kilo-octets, pas le téléchargement de
// l'archive.
//
// Les compteurs sont atomiques : l'indexation lit les en-têtes locaux en
// parallèle, donc l'instrumentation doit l'être aussi.
type RangeCounter struct {
	storage.Provider

	calls atomic.Int64
	bytes atomic.Int64
}

func NewRangeCounter(p storage.Provider) *RangeCounter {
	return &RangeCounter{Provider: p}
}

func (c *RangeCounter) ReadRange(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	r, err := c.Provider.ReadRange(ctx, key, offset, length)
	if err != nil {
		return nil, err
	}
	c.calls.Add(1)
	return &countingReader{ReadCloser: r, counter: c}, nil
}

// Calls retourne le nombre de requêtes Range effectuées.
func (c *RangeCounter) Calls() int64 { return c.calls.Load() }

// Bytes retourne le nombre d'octets effectivement transférés.
func (c *RangeCounter) Bytes() int64 { return c.bytes.Load() }

func (c *RangeCounter) Reset() {
	c.calls.Store(0)
	c.bytes.Store(0)
}

func (c *RangeCounter) String() string {
	return fmt.Sprintf("%d requête(s) Range, %d octets", c.Calls(), c.Bytes())
}

type countingReader struct {
	io.ReadCloser
	counter *RangeCounter
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.counter.bytes.Add(int64(n))
	return n, err
}
