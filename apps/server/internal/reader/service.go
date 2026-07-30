// Package reader sert les pages d'un album.
//
// C'est le chemin chaud du produit : à chaque tourner de page, une requête
// arrive ici. Tout y est conçu pour que le coût reste constant quelle que soit
// la taille de l'archive — un ReadRange unique sur les coordonnées persistées
// en M1, jamais de relecture de l'index.
package reader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/archive"
	"github.com/adonko3xBitters/boxincloud/server/internal/cache"
	"github.com/adonko3xBitters/boxincloud/server/internal/imaging"
	"github.com/adonko3xBitters/boxincloud/server/internal/library"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
)

var (
	ErrNotFound     = errors.New("reader : page introuvable")
	ErrNotReady     = errors.New("reader : album non indexé")
	ErrPageOutRange = errors.New("reader : numéro de page hors limites")
)

// Comic est ce que le lecteur a besoin de savoir d'un album.
type Comic struct {
	ID        uuid.UUID
	LibraryID uuid.UUID
	ObjectKey string
	Format    string
	State     string
	PageCount int32
	CoverPage int32
}

// Page est une entrée de comic_pages : les coordonnées d'accès aléatoire.
type Page struct {
	Index       int32
	EntryName   string
	DataOffset  int64
	DataSize    int64
	Size        int64
	Compression int16
	Width       *int32
	Height      *int32
	IsDouble    bool
}

// Repository est ce dont le lecteur a besoin de la persistance.
type Repository interface {
	GetComic(ctx context.Context, id uuid.UUID) (Comic, error)
	GetPage(ctx context.Context, comicID uuid.UUID, index int32) (Page, error)
	ListPages(ctx context.Context, comicID uuid.UUID) ([]Page, error)
}

// Service sert les pages et les couvertures.
type Service struct {
	repo      Repository
	libraries *library.Service
	cache     *cache.Cache
	imaging   imaging.Processor
	log       *slog.Logger
}

func NewService(repo Repository, libs *library.Service, c *cache.Cache, p imaging.Processor, log *slog.Logger) *Service {
	return &Service{repo: repo, libraries: libs, cache: c, imaging: p, log: log}
}

// ─── Manifeste ───────────────────────────────────────────────────────────────

// Manifest décrit un album pour le lecteur.
//
// Renvoyé en une requête à l'ouverture : le client connaît alors les dimensions
// de toutes les pages et peut réserver la mise en page avant d'avoir reçu la
// moindre image. C'est ce qui supprime le décalage visuel pendant la lecture —
// le défaut le plus agaçant d'un lecteur web.
type Manifest struct {
	ComicID   uuid.UUID
	PageCount int32
	Pages     []ManifestPage
}

// ManifestPage ne porte que ce dont le client a besoin. Les offsets restent
// côté serveur : ils décrivent la structure interne de l'archive et n'ont
// aucune raison d'être exposés.
type ManifestPage struct {
	Index    int32
	Width    *int32
	Height   *int32
	IsDouble bool
}

func (s *Service) Manifest(ctx context.Context, comicID uuid.UUID) (Manifest, error) {
	comic, err := s.repo.GetComic(ctx, comicID)
	if err != nil {
		return Manifest{}, err
	}
	if comic.State != "ready" {
		return Manifest{}, fmt.Errorf("%w : état %s", ErrNotReady, comic.State)
	}

	pages, err := s.repo.ListPages(ctx, comicID)
	if err != nil {
		return Manifest{}, err
	}

	out := make([]ManifestPage, 0, len(pages))
	for _, p := range pages {
		out = append(out, ManifestPage{
			Index:    p.Index,
			Width:    p.Width,
			Height:   p.Height,
			IsDouble: p.IsDouble,
		})
	}

	return Manifest{
		ComicID:   comic.ID,
		PageCount: comic.PageCount,
		Pages:     out,
	}, nil
}

// ─── Service de page ─────────────────────────────────────────────────────────

// PageContent est une page prête à être servie.
type PageContent struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
	// ETag identifie la variante servie. Une variante est immuable : même
	// album, même page, même largeur, même format donnent toujours les mêmes
	// octets. Le client peut donc la garder indéfiniment.
	ETag string
}

// PageRequest décrit la page demandée.
type PageRequest struct {
	ComicID uuid.UUID
	Index   int32
	// Width demande un redimensionnement. Zéro sert la page d'origine.
	Width int
}

// GetPage sert une page.
//
// Deux chemins :
//
//   - la variante est en cache → on la sert directement, aucun accès au
//     backend de stockage ;
//   - sinon → un unique ReadRange sur l'archive distante, transcodage si
//     nécessaire, mise en cache, puis service.
//
// Le cas non transcodé (Width = 0) est servi en flux, sans jamais charger la
// page entière en mémoire.
func (s *Service) GetPage(ctx context.Context, req PageRequest) (PageContent, error) {
	comic, err := s.repo.GetComic(ctx, req.ComicID)
	if err != nil {
		return PageContent{}, err
	}
	if comic.State != "ready" {
		return PageContent{}, fmt.Errorf("%w : état %s", ErrNotReady, comic.State)
	}
	if req.Index < 0 || req.Index >= comic.PageCount {
		return PageContent{}, fmt.Errorf("%w : page %d sur %d", ErrPageOutRange, req.Index, comic.PageCount)
	}

	page, err := s.repo.GetPage(ctx, req.ComicID, req.Index)
	if err != nil {
		return PageContent{}, err
	}

	// Sans redimensionnement, la page d'origine est servie telle quelle depuis
	// l'archive. La mettre en cache reviendrait à dupliquer ce que le backend
	// contient déjà, pour un gain nul sur le nombre de requêtes.
	if req.Width <= 0 {
		body, err := s.openOriginal(ctx, comic, page)
		if err != nil {
			return PageContent{}, err
		}
		return PageContent{
			Body:        body,
			ContentType: contentTypeOf(page.EntryName),
			Size:        page.Size,
			ETag:        pageETag(comic.ID, page.Index, 0, "orig"),
		}, nil
	}

	key := cache.PageKey(comic.ID, int(page.Index), req.Width, imaging.FormatJPEG)

	if body, err := s.cache.Get(ctx, key); err == nil {
		return PageContent{
			Body:        body,
			ContentType: imaging.FormatJPEG.ContentType(),
			ETag:        pageETag(comic.ID, page.Index, req.Width, string(imaging.FormatJPEG)),
		}, nil
	} else if !errors.Is(err, storage.ErrNotFound) {
		s.log.Warn("lecture du cache impossible", slog.String("key", key), slog.Any("err", err))
	}

	transcoded, err := s.transcode(ctx, comic, page, req.Width)
	if err != nil {
		return PageContent{}, err
	}

	if err := s.cache.Put(ctx, key, comic.ID, transcoded, imaging.FormatJPEG.ContentType()); err != nil {
		// Le cache est un accélérateur, pas une source de vérité : son échec ne
		// doit pas empêcher de servir la page.
		s.log.Warn("écriture du cache impossible", slog.String("key", key), slog.Any("err", err))
	}

	return PageContent{
		Body:        io.NopCloser(bytes.NewReader(transcoded)),
		ContentType: imaging.FormatJPEG.ContentType(),
		Size:        int64(len(transcoded)),
		ETag:        pageETag(comic.ID, page.Index, req.Width, string(imaging.FormatJPEG)),
	}, nil
}

// openOriginal ouvre les octets d'origine d'une page.
//
// ★ Le point où se tient la promesse du projet : un seul ReadRange, sur les
// coordonnées lues en base. L'index de l'archive n'est jamais relu.
func (s *Service) openOriginal(ctx context.Context, comic Comic, page Page) (io.ReadCloser, error) {
	lib, err := s.libraries.GetLibrary(ctx, comic.LibraryID)
	if err != nil {
		return nil, err
	}
	provider, err := s.libraries.ProviderForLibrary(ctx, lib)
	if err != nil {
		return nil, err
	}

	return archive.OpenEntry(ctx, provider, comic.ObjectKey, archive.Entry{
		Name:        page.EntryName,
		DataOffset:  page.DataOffset,
		DataSize:    page.DataSize,
		Size:        page.Size,
		Compression: compressionOf(page.Compression),
	})
}

// compressionOf convertit la méthode de compression persistée.
//
// La colonne est un smallint signé ; les valeurs légales du format ZIP sont 0
// (stored) et 8 (deflate). Une valeur négative viendrait d'une base corrompue :
// on la ramène à 0 plutôt que de la convertir en un entier non signé énorme.
func compressionOf(v int16) archive.Compression {
	if v < 0 {
		return archive.CompressionStore
	}
	return archive.Compression(uint16(v))
}

// maxPageBytes borne la lecture d'une page à transcoder.
//
// Une planche réelle en très haute définition dépasse rarement 20 Mio ; la
// borne protège d'une entrée d'archive délibérément énorme.
const maxPageBytes = 64 << 20

func (s *Service) transcode(ctx context.Context, comic Comic, page Page, width int) ([]byte, error) {
	src, err := s.openOriginal(ctx, comic, page)
	if err != nil {
		return nil, err
	}
	defer func() { _ = src.Close() }()

	raw, err := io.ReadAll(io.LimitReader(src, maxPageBytes))
	if err != nil {
		return nil, fmt.Errorf("lecture de la page %d : %w", page.Index, err)
	}

	var buf bytes.Buffer
	if _, err := s.imaging.Transform(&buf, bytes.NewReader(raw), imaging.Options{
		Width:  width,
		Format: imaging.FormatJPEG,
	}); err != nil {
		return nil, fmt.Errorf("transcodage de la page %d : %w", page.Index, err)
	}
	return buf.Bytes(), nil
}

// ─── Couvertures ─────────────────────────────────────────────────────────────

// GetCover sert une vignette de couverture.
//
// Les trois tailles sont générées à l'indexation ; ce chemin ne fait donc
// normalement que lire le cache. En cas d'absence — cache purgé, album indexé
// par une version antérieure — la vignette est régénérée à la volée.
func (s *Service) GetCover(ctx context.Context, comicID uuid.UUID, width int) (PageContent, error) {
	if width <= 0 {
		width = imaging.ThumbMedium
	}
	width = nearestThumbSize(width)

	key := cache.CoverKey(comicID, width, imaging.FormatJPEG)

	if body, err := s.cache.Get(ctx, key); err == nil {
		return PageContent{
			Body:        body,
			ContentType: imaging.FormatJPEG.ContentType(),
			ETag:        coverETag(comicID, width),
		}, nil
	}

	comic, err := s.repo.GetComic(ctx, comicID)
	if err != nil {
		return PageContent{}, err
	}
	if comic.State != "ready" {
		return PageContent{}, fmt.Errorf("%w : état %s", ErrNotReady, comic.State)
	}

	page, err := s.repo.GetPage(ctx, comicID, comic.CoverPage)
	if err != nil {
		return PageContent{}, err
	}

	data, err := s.transcode(ctx, comic, page, width)
	if err != nil {
		return PageContent{}, err
	}

	if err := s.cache.Put(ctx, key, comicID, data, imaging.FormatJPEG.ContentType()); err != nil {
		s.log.Warn("écriture de la couverture en cache impossible",
			slog.String("key", key), slog.Any("err", err))
	}

	return PageContent{
		Body:        io.NopCloser(bytes.NewReader(data)),
		ContentType: imaging.FormatJPEG.ContentType(),
		Size:        int64(len(data)),
		ETag:        coverETag(comicID, width),
	}, nil
}

// nearestThumbSize ramène une largeur arbitraire à l'une des trois tailles
// générées.
//
// Sans cela, chaque largeur demandée créerait une entrée de cache distincte, et
// une grille au zoom continu remplirait le disque de variantes d'un pixel
// d'écart.
func nearestThumbSize(width int) int {
	for _, size := range imaging.ThumbSizes {
		if width <= size {
			return size
		}
	}
	return imaging.ThumbSizes[len(imaging.ThumbSizes)-1]
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func pageETag(comicID uuid.UUID, index int32, width int, format string) string {
	return fmt.Sprintf(`"%s-%d-%d-%s"`, comicID.String()[:8], index, width, format)
}

func coverETag(comicID uuid.UUID, width int) string {
	return fmt.Sprintf(`"cover-%s-%d"`, comicID.String()[:8], width)
}

func contentTypeOf(entryName string) string {
	switch {
	case hasSuffixFold(entryName, ".png"):
		return "image/png"
	case hasSuffixFold(entryName, ".gif"):
		return "image/gif"
	case hasSuffixFold(entryName, ".webp"):
		return "image/webp"
	case hasSuffixFold(entryName, ".avif"):
		return "image/avif"
	default:
		return "image/jpeg"
	}
}

func hasSuffixFold(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	return equalFold(s[len(s)-len(suffix):], suffix)
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
