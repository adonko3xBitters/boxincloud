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
	"time"

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
	// TouchEntry met à jour la date d'accès et indique si une ligne existait.
	// Le booléen permet de détecter un fichier orphelin, présent sur disque
	// mais absent de l'inventaire — donc jamais évincé.
	TouchEntry(ctx context.Context, key string) (bool, error)
	TotalSize(ctx context.Context) (int64, error)
	ListForEviction(ctx context.Context, limit int32) ([]Entry, error)
	DeleteEntry(ctx context.Context, key string) error

	// Stats et PurgeEntries servent l'écran d'administration.
	Stats(ctx context.Context) (Stats, error)
	PurgeEntries(ctx context.Context) ([]Entry, error)
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
	touched, err := c.store.TouchEntry(ctx, key)
	if err != nil {
		c.log.Debug("cache : mise à jour de l'accès impossible",
			slog.String("key", key), slog.Any("err", err))
		return r, nil
	}

	// Fichier présent sur disque mais absent de l'inventaire : il ne serait
	// jamais évincé, et occuperait la place indéfiniment. Cela arrive après une
	// purge de la base sans purge du disque, ou si l'inscription à l'inventaire
	// a échoué à l'écriture. On le réinscrit.
	if !touched {
		c.readopt(ctx, key)
	}
	return r, nil
}

// readopt réinscrit à l'inventaire un fichier orphelin.
//
// Le comic d'origine n'est pas récupérable depuis la clé sans l'analyser ; on
// laisse la référence nulle. L'entrée reste évinçable, c'est l'essentiel — et
// une réindexation la recréera correctement.
func (c *Cache) readopt(ctx context.Context, key string) {
	info, err := c.provider.Stat(ctx, key)
	if err != nil {
		return
	}
	if err := c.store.RecordEntry(ctx, key, uuid.Nil, info.Size); err != nil {
		c.log.Debug("cache : réinscription impossible",
			slog.String("key", key), slog.Any("err", err))
		return
	}
	c.log.Debug("cache : fichier orphelin réinscrit à l'inventaire",
		slog.String("key", key), slog.Int64("taille", info.Size))
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

/*
Provider expose le stockage sous-jacent du cache.

Réservé à l'hydratation, qui y dépose une archive normalisée de plusieurs
centaines de méga-octets. `Put` prendrait cette archive en tranche d'octets et
la chargerait donc entière en mémoire ; le provider, lui, écrit en flux.

L'inventaire n'est pas alimenté par ce chemin, et c'est voulu : une archive
hydratée n'est PAS évinçable comme une vignette. La perdre rendrait l'album
illisible jusqu'à une réindexation, alors que perdre une vignette ne coûte
qu'une régénération à la volée.
*/
func (c *Cache) Provider() storage.Provider { return c.provider }

// Delete retire une entrée.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := c.provider.Delete(ctx, key); err != nil && !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	return c.store.DeleteEntry(ctx, key)
}

// Stats décrit l'occupation du cache dérivé.
type Stats struct {
	Entries     int64
	Bytes       int64
	Hits        int64
	MaxBytes    int64
	OldestAt    *time.Time
	NewestHitAt *time.Time
}

// Stats retourne l'inventaire courant.
func (c *Cache) Stats(ctx context.Context) (Stats, error) {
	stats, err := c.store.Stats(ctx)
	if err != nil {
		return Stats{}, err
	}
	stats.MaxBytes = c.maxSize
	return stats, nil
}

/*
Purge vide entièrement le cache dérivé.

Sans danger par construction : tout y est reconstructible depuis les archives
d'origine, et une purge ne coûte qu'une régénération à la prochaine lecture.
C'est le geste utile après un changement de réglage d'imagerie, ou quand on
soupçonne des variantes corrompues.

L'inventaire est vidé d'abord, et les objets ensuite. L'ordre inverse
laisserait, en cas d'interruption, des lignes désignant des fichiers absents —
que le cache servirait alors comme des trous. Dans ce sens-ci, une interruption
laisse au pire des objets orphelins : de la place perdue, jamais une erreur.
*/
func (c *Cache) Purge(ctx context.Context) (int64, int64, error) {
	entries, err := c.store.PurgeEntries(ctx)
	if err != nil {
		return 0, 0, err
	}

	var freed int64
	for _, entry := range entries {
		if err := c.provider.Delete(ctx, entry.Key); err != nil &&
			!errors.Is(err, storage.ErrNotFound) {
			// Un objet récalcitrant ne doit pas interrompre la purge : le
			// suivant se supprimera peut-être, et l'inventaire est déjà vide.
			c.log.Warn("cache : purge partielle",
				slog.String("key", entry.Key), slog.Any("err", err))
			continue
		}
		freed += entry.Size
	}

	return int64(len(entries)), freed, nil
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
