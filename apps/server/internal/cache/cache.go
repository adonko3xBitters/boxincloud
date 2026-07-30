// Package cache gère le cache dérivé : vignettes, couvertures et pages
// transcodées.
//
// Tout ce qu'il contient est reconstructible à partir des archives d'origine.
// Le vider ne perd aucune donnée utilisateur — c'est ce qui autorise une
// éviction agressive et une purge sans précaution.
//
// Le cache s'appuie sur un storage.Provider : par défaut le disque local, mais
// rien n'empêche de le placer sur un bucket dédié pour partager le cache entre
// plusieurs instances.
package cache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strconv"
	"sync"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/imaging"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
)

// Store est la vue que le cache a de la base : il y tient l'inventaire des
// entrées pour pouvoir évincer les moins récemment utilisées.
//
// Interface déclarée ici plutôt qu'importée : le cache ne dépend d'aucun autre
// module métier.
type Store interface {
	RecordEntry(ctx context.Context, key string, comicID uuid.UUID, size int64) error
	TouchEntry(ctx context.Context, key string) error
	TotalSize(ctx context.Context) (int64, error)
	ListForEviction(ctx context.Context, limit int32) ([]Entry, error)
	DeleteEntry(ctx context.Context, key string) error
}

// Entry est une entrée de l'inventaire.
type Entry struct {
	Key  string
	Size int64
}

// Cache stocke et sert les données dérivées.
type Cache struct {
	provider storage.Provider
	store    Store
	maxSize  int64
	log      *slog.Logger

	// L'éviction ne doit pas être déclenchée en parallèle par plusieurs
	// écritures : elles se disputeraient les mêmes entrées.
	evictMu sync.Mutex
}

func New(provider storage.Provider, store Store, maxSize int64, log *slog.Logger) *Cache {
	return &Cache{provider: provider, store: store, maxSize: maxSize, log: log}
}

// ─── Clés ────────────────────────────────────────────────────────────────────

// PageKey construit la clé d'une page transcodée.
//
// La largeur et le format font partie de la clé : chaque variante est une
// entrée distincte, immuable, que le client peut mettre en cache indéfiniment.
func PageKey(comicID uuid.UUID, index, width int, format imaging.Format) string {
	name := "orig"
	if width > 0 {
		name = "w" + strconv.Itoa(width)
	}
	return path.Join("page", comicID.String(), strconv.Itoa(index), name+format.Extension())
}

// CoverKey construit la clé d'une vignette de couverture.
func CoverKey(comicID uuid.UUID, width int, format imaging.Format) string {
	return path.Join("cover", comicID.String(), "w"+strconv.Itoa(width)+format.Extension())
}

// ─── Lecture et écriture ─────────────────────────────────────────────────────

// Get ouvre une entrée du cache.
//
// Retourne storage.ErrNotFound si l'entrée n'existe pas — l'appelant la
// régénère alors.
func (c *Cache) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	r, err := c.provider.Open(ctx, key)
	if err != nil {
		return nil, err
	}

	// La date de dernier accès pilote l'éviction. L'échec de sa mise à jour ne
	// doit jamais empêcher de servir la donnée.
	if err := c.store.TouchEntry(ctx, key); err != nil {
		c.log.Debug("cache : mise à jour de l'accès impossible",
			slog.String("key", key), slog.Any("err", err))
	}
	return r, nil
}

// Has indique si une entrée est présente, sans la lire.
func (c *Cache) Has(ctx context.Context, key string) bool {
	_, err := c.provider.Stat(ctx, key)
	return err == nil
}

// Put écrit une entrée et l'inscrit à l'inventaire.
func (c *Cache) Put(ctx context.Context, key string, comicID uuid.UUID, data []byte, contentType string) error {
	if err := c.provider.Write(ctx, key, bytesReader(data), int64(len(data)), contentType); err != nil {
		return fmt.Errorf("cache : écriture de %q : %w", key, err)
	}

	if err := c.store.RecordEntry(ctx, key, comicID, int64(len(data))); err != nil {
		// L'objet est écrit mais absent de l'inventaire : il ne sera jamais
		// évincé. On le signale sans faire échouer l'opération, la donnée
		// servie restant correcte.
		c.log.Warn("cache : inscription à l'inventaire impossible",
			slog.String("key", key), slog.Any("err", err))
	}

	c.evictIfNeeded(ctx)
	return nil
}

// Delete retire une entrée.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := c.provider.Delete(ctx, key); err != nil && !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	return c.store.DeleteEntry(ctx, key)
}

// ─── Éviction ────────────────────────────────────────────────────────────────

// evictIfNeeded ramène le cache sous sa taille maximale.
//
// Politique LRU : on retire les entrées les moins récemment servies. Elles sont
// reconstructibles, donc une éviction trop zélée ne coûte qu'une régénération.
//
// L'éviction est déclenchée par les écritures plutôt que par une tâche
// périodique : elle intervient exactement quand la taille augmente, et une
// instance au repos ne fait rien.
func (c *Cache) evictIfNeeded(ctx context.Context) {
	if c.maxSize <= 0 {
		return // cache non borné
	}

	if !c.evictMu.TryLock() {
		return // une éviction est déjà en cours
	}
	defer c.evictMu.Unlock()

	total, err := c.store.TotalSize(ctx)
	if err != nil {
		c.log.Warn("cache : taille totale indisponible", slog.Any("err", err))
		return
	}
	if total <= c.maxSize {
		return
	}

	// On descend sous 90 % du plafond plutôt que juste sous le plafond :
	// évincer par lots évite de relancer une éviction à chaque écriture.
	target := c.maxSize * 90 / 100
	toFree := total - target

	entries, err := c.store.ListForEviction(ctx, 500)
	if err != nil {
		c.log.Warn("cache : liste d'éviction indisponible", slog.Any("err", err))
		return
	}

	var freed int64
	var removed int

	for _, e := range entries {
		if freed >= toFree {
			break
		}
		if err := c.Delete(ctx, e.Key); err != nil {
			c.log.Debug("cache : éviction impossible",
				slog.String("key", e.Key), slog.Any("err", err))
			continue
		}
		freed += e.Size
		removed++
	}

	if removed > 0 {
		c.log.Info("cache : éviction",
			slog.Int("entrées", removed),
			slog.Int64("octets_libérés", freed),
			slog.Int64("taille_avant", total),
			slog.Int64("plafond", c.maxSize),
		)
	}
}

// bytesReader évite d'importer bytes juste pour cela dans les appelants.
func bytesReader(b []byte) io.Reader { return &sliceReader{data: b} }

type sliceReader struct {
	data []byte
	pos  int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
