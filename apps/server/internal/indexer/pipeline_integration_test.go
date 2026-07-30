package indexer_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/archive"
	"github.com/adonko3xBitters/boxincloud/server/internal/cache"
	"github.com/adonko3xBitters/boxincloud/server/internal/imaging"
	"github.com/adonko3xBitters/boxincloud/server/internal/indexer"
	"github.com/adonko3xBitters/boxincloud/server/internal/library"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/crypto"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sqlc"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage/local"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage/s3"
	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/comicfixture"
	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/miniotest"
	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/pgtest"
)

// harness assemble le pipeline complet au-dessus d'un PostgreSQL et d'un MinIO
// réels.
type harness struct {
	queries   *sqlc.Queries
	libraries *library.Service
	runner    *indexer.DirectRunner
	provider  storage.Provider
	libraryID uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	pool := pgtest.Start(t)
	minio := miniotest.Start(t)

	queries := sqlc.New(pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	key := bytes.Repeat([]byte{0x2b}, 32)
	sealer, err := crypto.NewSealer(key)
	if err != nil {
		t.Fatal(err)
	}

	libraries := library.NewService(library.NewPostgresRepository(queries), sealer, log)

	backend, err := libraries.CreateBackend(context.Background(), library.CreateBackendParams{
		Name: "minio-test",
		Kind: storage.KindS3,
		Config: map[string]string{
			"endpoint":   minio.Endpoint,
			"bucket":     minio.Bucket,
			"use_ssl":    "false",
			"path_style": "true",
		},
		Secrets: map[string]string{
			"access_key": minio.AccessKey,
			"secret_key": minio.SecretKey,
		},
	})
	if err != nil {
		t.Fatalf("création du backend : %v", err)
	}

	lib, err := libraries.CreateLibrary(context.Background(), library.CreateLibraryParams{
		Name:       "test",
		BackendID:  backend.ID,
		RootPrefix: "bd/",
	})
	if err != nil {
		t.Fatalf("création de la bibliothèque : %v", err)
	}

	cacheProvider, err := local.New(local.Options{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	provider, err := s3.New(s3.Options{
		Endpoint:  minio.Endpoint,
		Bucket:    minio.Bucket,
		AccessKey: minio.AccessKey,
		SecretKey: minio.SecretKey,
		PathStyle: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	runner := indexer.NewDirectRunner(indexer.Deps{
		Libraries: libraries,
		Repo:      indexer.NewPostgresRepository(queries, pool, noopEnqueuer{}),
		Cache:     cache.New(cacheProvider, newMemCacheStore(), 0, log),
		Imaging:   imaging.NewPureGo(),
		Log:       log,
	})

	return &harness{
		queries:   queries,
		libraries: libraries,
		runner:    runner,
		provider:  provider,
		libraryID: lib.ID,
	}
}

func (h *harness) upload(t *testing.T, key string, data []byte) {
	t.Helper()
	if err := h.provider.Write(context.Background(), key, bytes.NewReader(data), int64(len(data)), ""); err != nil {
		t.Fatalf("téléversement de %s : %v", key, err)
	}
}

// Le test de bout en bout du jalon : téléverser des archives sur un stockage
// objet, les scanner, les indexer, puis relire une page arbitraire.
func TestIntegrationScanAndIndex(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	tintin := comicfixture.BuildCBZ(t, comicfixture.Options{
		Pages:     20,
		ComicInfo: comicfixture.SampleComicInfo,
	})
	asterix := comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 12})

	h.upload(t, "bd/Les Aventures de Tintin - T11 - Le Secret de la Licorne.cbz", tintin.Data)
	h.upload(t, "bd/Astérix - T01 - Astérix le Gaulois.cbz", asterix.Data)
	h.upload(t, "bd/notes.txt", []byte("pas une bande dessinée"))
	// Hors préfixe de la bibliothèque : ne doit pas être indexé.
	h.upload(t, "autre/ignoré.cbz", asterix.Data)

	stats, err := h.runner.ScanAndIndex(ctx, h.libraryID)
	if err != nil {
		t.Fatalf("ScanAndIndex : %v", err)
	}

	if stats.ObjectsSeen != 2 {
		t.Errorf("%d objets vus, attendu 2 (notes.txt et le hors-préfixe doivent être ignorés)", stats.ObjectsSeen)
	}
	if stats.Added != 2 {
		t.Errorf("%d albums ajoutés, attendu 2", stats.Added)
	}
	if stats.Errors != 0 {
		t.Errorf("%d erreurs pendant le scan", stats.Errors)
	}

	comics, err := h.queries.ListComicsByLibrary(ctx, sqlc.ListComicsByLibraryParams{
		LibraryID: h.libraryID,
		Limit:     100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(comics) != 2 {
		t.Fatalf("%d albums en base, attendu 2", len(comics))
	}

	for _, c := range comics {
		if c.State != sqlc.ComicStateReady {
			t.Errorf("%s : état %s, attendu ready (%v)", c.Title, c.State, c.StateDetail)
		}
		if c.PageCount == 0 {
			t.Errorf("%s : aucune page comptée", c.Title)
		}

		count, err := h.queries.CountComicPages(ctx, c.ID)
		if err != nil {
			t.Fatal(err)
		}
		if count != int64(c.PageCount) {
			t.Errorf("%s : %d lignes comic_pages pour %d pages annoncées", c.Title, count, c.PageCount)
		}
	}
}

// Le point de validation central : relire une page depuis l'index persisté, en
// une seule requête Range, sans jamais rouvrir l'index de l'archive.
func TestIntegrationServePageFromStoredIndex(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	built := comicfixture.BuildCBZ(t, comicfixture.Options{
		Pages:      30,
		PageWidth:  800,
		PageHeight: 1200,
	})
	key := "bd/Série - T01 - Album.cbz"
	h.upload(t, key, built.Data)

	if _, err := h.runner.ScanAndIndex(ctx, h.libraryID); err != nil {
		t.Fatalf("ScanAndIndex : %v", err)
	}

	comic, err := h.queries.GetComicByObjectKey(ctx, sqlc.GetComicByObjectKeyParams{
		LibraryID: h.libraryID,
		ObjectKey: key,
	})
	if err != nil {
		t.Fatal(err)
	}

	const wanted = 17

	// Le chemin chaud réel : une lecture en base, puis une lecture par plage.
	// L'index de l'archive n'est jamais relu.
	row, err := h.queries.GetComicPage(ctx, sqlc.GetComicPageParams{
		ComicID: comic.ID,
		Index:   wanted,
	})
	if err != nil {
		t.Fatalf("GetComicPage : %v", err)
	}

	counter := newRangeCounter(h.provider)

	r, err := archive.OpenEntry(ctx, counter, key, archive.Entry{
		Name:        row.EntryName,
		DataOffset:  *row.DataOffset,
		DataSize:    *row.DataSize,
		Size:        *row.Size,
		Compression: archive.Compression(*row.Compression),
	})
	if err != nil {
		t.Fatalf("OpenEntry : %v", err)
	}
	got, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}

	want := built.PageContents[built.PageNames[wanted]]
	if !bytes.Equal(got, want) {
		t.Fatalf("contenu de la page %d incorrect", wanted)
	}

	if counter.calls != 1 {
		t.Errorf("%d requêtes Range pour servir une page, attendu 1", counter.calls)
	}
	if counter.bytes >= int64(len(built.Data)) {
		t.Errorf("%d octets transférés pour une archive de %d", counter.bytes, len(built.Data))
	}

	t.Logf("page %d servie en %d requête Range, %d octets sur %d (%.2f %%)",
		wanted+1, counter.calls, counter.bytes, len(built.Data),
		float64(counter.bytes)/float64(len(built.Data))*100)
}

// Les dimensions sont persistées pour que le client puisse réserver la mise en
// page avant réception de l'image.
func TestIntegrationPageDimensionsAreStored(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	key := "bd/Série - T01 - Album.cbz"
	h.upload(t, key, comicfixture.BuildCBZ(t, comicfixture.Options{
		Pages:      5,
		PageWidth:  700,
		PageHeight: 1000,
	}).Data)

	if _, err := h.runner.ScanAndIndex(ctx, h.libraryID); err != nil {
		t.Fatal(err)
	}

	comic, err := h.queries.GetComicByObjectKey(ctx, sqlc.GetComicByObjectKeyParams{
		LibraryID: h.libraryID, ObjectKey: key,
	})
	if err != nil {
		t.Fatal(err)
	}

	pages, err := h.queries.ListComicPages(ctx, comic.ID)
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range pages {
		if p.Width == nil || *p.Width != 700 {
			t.Errorf("page %d : largeur %v, attendu 700", p.Index, p.Width)
		}
		if p.Height == nil || *p.Height != 1000 {
			t.Errorf("page %d : hauteur %v, attendu 1000", p.Index, p.Height)
		}
		if p.IsDouble {
			t.Errorf("page %d marquée double alors qu'elle est en portrait", p.Index)
		}
	}
}

// Rejouer un scan sur une bibliothèque inchangée ne doit rien créer ni
// réindexer : c'est la condition pour qu'un scan interrompu puisse simplement
// être relancé.
func TestIntegrationRescanIsIdempotent(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	h.upload(t, "bd/Série - T01 - Album.cbz",
		comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 6}).Data)

	first, err := h.runner.ScanAndIndex(ctx, h.libraryID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Added != 1 {
		t.Fatalf("premier scan : %d ajouts, attendu 1", first.Added)
	}

	second, err := h.runner.ScanAndIndex(ctx, h.libraryID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Added != 0 || second.Updated != 0 {
		t.Errorf("rescan : %d ajouts et %d modifications, attendu 0 et 0", second.Added, second.Updated)
	}

	count, err := h.queries.CountComicsByLibrary(ctx, h.libraryID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("%d albums après deux scans, attendu 1", count)
	}
}

// Un objet modifié doit être réindexé : l'ETag et la taille sont ce qui permet
// de le détecter sans relire le contenu.
func TestIntegrationModifiedObjectIsReindexed(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	key := "bd/Série - T01 - Album.cbz"
	h.upload(t, key, comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 6}).Data)

	if _, err := h.runner.ScanAndIndex(ctx, h.libraryID); err != nil {
		t.Fatal(err)
	}

	// Remplacement par une version à 10 pages.
	h.upload(t, key, comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 10}).Data)

	stats, err := h.runner.ScanAndIndex(ctx, h.libraryID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Updated != 1 {
		t.Errorf("%d modifications détectées, attendu 1", stats.Updated)
	}

	comic, err := h.queries.GetComicByObjectKey(ctx, sqlc.GetComicByObjectKeyParams{
		LibraryID: h.libraryID, ObjectKey: key,
	})
	if err != nil {
		t.Fatal(err)
	}
	if comic.PageCount != 10 {
		t.Errorf("%d pages après réindexation, attendu 10", comic.PageCount)
	}

	count, err := h.queries.CountComicPages(ctx, comic.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 10 {
		t.Errorf("%d lignes comic_pages, attendu 10 : l'ancien index n'a pas été remplacé", count)
	}
}

// Un objet disparu du backend est marqué, jamais supprimé : un bucket
// momentanément démonté ne doit pas détruire les données des utilisateurs.
func TestIntegrationMissingObjectIsMarkedNotDeleted(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	keep := "bd/Série - T01 - Album.cbz"
	gone := "bd/Série - T02 - Album.cbz"

	h.upload(t, keep, comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 4}).Data)
	h.upload(t, gone, comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 4}).Data)

	if _, err := h.runner.ScanAndIndex(ctx, h.libraryID); err != nil {
		t.Fatal(err)
	}

	if err := h.provider.Delete(ctx, gone); err != nil {
		t.Fatal(err)
	}

	stats, err := h.runner.ScanAndIndex(ctx, h.libraryID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Removed != 1 {
		t.Errorf("%d disparitions détectées, attendu 1", stats.Removed)
	}

	// La ligne existe toujours, seulement marquée.
	if _, err := h.queries.GetComicByObjectKey(ctx, sqlc.GetComicByObjectKeyParams{
		LibraryID: h.libraryID, ObjectKey: gone,
	}); err != nil {
		t.Errorf("la ligne du comic disparu a été supprimée : %v", err)
	}

	visible, err := h.queries.CountComicsByLibrary(ctx, h.libraryID)
	if err != nil {
		t.Fatal(err)
	}
	if visible != 1 {
		t.Errorf("%d albums visibles, attendu 1", visible)
	}
}

// ComicInfo.xml doit primer sur l'analyse du nom de fichier.
func TestIntegrationComicInfoTakesPrecedence(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Le nom de fichier annonce une autre série et un autre numéro que le
	// ComicInfo.xml embarqué.
	key := "bd/Nom Trompeur - T99 - Faux Titre.cbz"
	h.upload(t, key, comicfixture.BuildCBZ(t, comicfixture.Options{
		Pages:     5,
		ComicInfo: comicfixture.SampleComicInfo,
	}).Data)

	if _, err := h.runner.ScanAndIndex(ctx, h.libraryID); err != nil {
		t.Fatal(err)
	}

	comic, err := h.queries.GetComicByObjectKey(ctx, sqlc.GetComicByObjectKeyParams{
		LibraryID: h.libraryID, ObjectKey: key,
	})
	if err != nil {
		t.Fatal(err)
	}

	if comic.Title != "Le Secret de la Licorne" {
		t.Errorf("titre = %q, attendu celui du ComicInfo.xml", comic.Title)
	}
	if comic.Number == nil || *comic.Number != "11" {
		t.Errorf("numéro = %v, attendu 11 (celui du ComicInfo.xml)", comic.Number)
	}
	if comic.Language == nil || *comic.Language != "fr" {
		t.Errorf("langue = %v, attendu fr", comic.Language)
	}

	series, err := h.queries.ListSeriesByLibrary(ctx, h.libraryID)
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 1 || series[0].Name != "Les Aventures de Tintin" {
		t.Errorf("séries = %v, attendu [Les Aventures de Tintin]", series)
	}
	// L'article de tête est retiré pour le classement.
	if len(series) == 1 && series[0].SortName != "aventures de tintin" {
		t.Errorf("sort_name = %q, attendu \"aventures de tintin\"", series[0].SortName)
	}
}

// Une archive corrompue ne doit pas faire échouer le scan complet : elle est
// marquée en erreur, les autres albums restent lisibles.
func TestIntegrationCorruptedArchiveIsIsolated(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	h.upload(t, "bd/Bon - T01 - Album.cbz",
		comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 5}).Data)
	h.upload(t, "bd/Cassé - T01 - Album.cbz", []byte("ceci n'est pas une archive zip"))

	stats, err := h.runner.ScanAndIndex(ctx, h.libraryID)
	if err != nil {
		t.Fatalf("le scan ne devrait pas échouer à cause d'une archive corrompue : %v", err)
	}
	if stats.ObjectsSeen != 2 {
		t.Errorf("%d objets vus, attendu 2", stats.ObjectsSeen)
	}

	comics, err := h.queries.ListComicsByLibrary(ctx, sqlc.ListComicsByLibraryParams{
		LibraryID: h.libraryID, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	var ready, failed int
	for _, c := range comics {
		switch c.State {
		case sqlc.ComicStateReady:
			ready++
		case sqlc.ComicStateError:
			failed++
			if c.StateDetail == nil || *c.StateDetail == "" {
				t.Errorf("%s est en erreur sans explication", c.Title)
			}
		}
	}
	if ready != 1 || failed != 1 {
		t.Errorf("%d albums prêts et %d en erreur, attendu 1 et 1", ready, failed)
	}
}

// ─── Doublures ───────────────────────────────────────────────────────────────

type noopEnqueuer struct{}

func (noopEnqueuer) EnqueueIndexComic(context.Context, uuid.UUID) error { return nil }

// memCacheStore garde l'inventaire du cache en mémoire : ces tests portent sur
// le pipeline, pas sur l'éviction.
type memCacheStore struct {
	sizes map[string]int64
}

func newMemCacheStore() *memCacheStore {
	return &memCacheStore{sizes: make(map[string]int64)}
}

func (m *memCacheStore) RecordEntry(_ context.Context, key string, _ uuid.UUID, size int64) error {
	m.sizes[key] = size
	return nil
}
func (m *memCacheStore) TouchEntry(_ context.Context, key string) (bool, error) {
	_, ok := m.sizes[key]
	return ok, nil
}
func (m *memCacheStore) TotalSize(context.Context) (int64, error) {
	var total int64
	for _, s := range m.sizes {
		total += s
	}
	return total, nil
}
func (m *memCacheStore) ListForEviction(context.Context, int32) ([]cache.Entry, error) {
	return nil, nil
}
func (m *memCacheStore) DeleteEntry(_ context.Context, key string) error {
	delete(m.sizes, key)
	return nil
}

type rangeCounter struct {
	storage.Provider
	calls int
	bytes int64
}

func newRangeCounter(p storage.Provider) *rangeCounter {
	return &rangeCounter{Provider: p}
}

func (c *rangeCounter) ReadRange(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	r, err := c.Provider.ReadRange(ctx, key, offset, length)
	if err != nil {
		return nil, err
	}
	c.calls++
	return &countingReader{ReadCloser: r, counter: c}, nil
}

type countingReader struct {
	io.ReadCloser
	counter *rangeCounter
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.counter.bytes += int64(n)
	return n, err
}
