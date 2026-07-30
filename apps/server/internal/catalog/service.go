// Package catalog expose la consultation de la bibliothèque : séries, albums,
// recherche.
//
// Toutes les lectures passent par la résolution des bibliothèques visibles par
// l'utilisateur courant. C'est la seule porte d'entrée : un handler ne peut pas
// interroger le catalogue sans avoir traversé ce filtre, ce qui rend impossible
// la fuite d'un album d'une bibliothèque restreinte par oubli d'un contrôle.
package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound  = errors.New("catalog : ressource introuvable")
	ErrForbidden = errors.New("catalog : accès refusé à cette bibliothèque")
)

// Viewer décrit qui consulte. Porté par le contexte d'appel plutôt que déduit,
// pour que le filtrage soit explicite dans chaque signature.
type Viewer struct {
	UserID       uuid.UUID
	IsAdmin      bool
	MaxAgeRating *int16
}

// Comic est la vue applicative d'un album.
type Comic struct {
	ID         uuid.UUID
	LibraryID  uuid.UUID
	SeriesID   *uuid.UUID
	SeriesName string
	Title      string
	Number     string
	Volume     *int16
	Summary    string
	Format     string
	PageCount  int32
	State      string
	AgeRating  *int16
	Language   string
	FileSize   int64
	ReleasedAt *time.Time
	CreatedAt  time.Time

	// CoverPlaceholder est un data-URI de quelques centaines d'octets, affiché
	// flouté pendant le chargement de la vraie couverture.
	CoverPlaceholder string
	FileName         string
	FolderPath       string
}

// Series est la vue applicative d'une série.
type Series struct {
	ID           uuid.UUID
	LibraryID    uuid.UUID
	Name         string
	SortName     string
	Description  string
	Publisher    string
	ComicCount   int32
	CoverComicID *uuid.UUID
}

// Library est la vue applicative d'une bibliothèque.
type Library struct {
	ID         uuid.UUID
	Name       string
	Kind       string
	ComicCount int32
}

// Page est une tranche de résultats paginés.
//
// NextCursor est opaque : le client le renvoie tel quel, sans jamais le
// construire lui-même. Cela laisse la liberté de changer la stratégie de
// pagination sans rompre le contrat.
type Page[T any] struct {
	Items      []T
	NextCursor string
}

// Repository est ce dont le catalogue a besoin de la persistance.
type Repository interface {
	ListVisibleLibraries(ctx context.Context, v Viewer) ([]Library, error)
	CanAccessLibrary(ctx context.Context, v Viewer, libraryID uuid.UUID) (bool, error)

	ListComics(ctx context.Context, p ListComicsParams) ([]Comic, error)
	SearchComics(ctx context.Context, p SearchParams) ([]Comic, error)
	GetComic(ctx context.Context, id uuid.UUID) (Comic, error)
	ListComicsBySeries(ctx context.Context, seriesID uuid.UUID) ([]Comic, error)

	ListSeries(ctx context.Context, p ListSeriesParams) ([]Series, error)
	SearchSeries(ctx context.Context, p SearchParams) ([]Series, error)
	GetSeries(ctx context.Context, id uuid.UUID) (Series, error)

	ListRecent(ctx context.Context, p ListComicsParams) ([]Comic, error)
	ListNextInSeries(ctx context.Context, v Viewer, libraryIDs []uuid.UUID, limit int32) ([]Comic, error)
}

// ListComicsParams décrit une page d'albums à lister.
type ListComicsParams struct {
	UserID        uuid.UUID
	LibraryIDs    []uuid.UUID
	SeriesID      *uuid.UUID
	State         string
	ReadStatus    string
	Folder        *string
	FavoritesOnly bool
	Sort          Sort
	MaxAgeRating  *int16
	Cursor        *Cursor
	Limit         int32
}

// ListSeriesParams décrit une page de séries.
type ListSeriesParams struct {
	LibraryIDs []uuid.UUID
	AfterSort  string
	Limit      int32
}

// SearchParams décrit une recherche.
type SearchParams struct {
	LibraryIDs   []uuid.UUID
	Query        string
	MaxAgeRating *int16
	Limit        int32
}

// Service porte la logique de consultation.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// Limites de pagination.
//
// Le plafond protège d'une requête qui demanderait la bibliothèque entière :
// une réponse de dix mille albums coûterait autant au serveur qu'au client, et
// la grille virtualisée du web n'en affiche jamais autant à la fois.
const (
	DefaultLimit = 50
	MaxLimit     = 200
)

func clampLimit(n int32) int32 {
	switch {
	case n <= 0:
		return DefaultLimit
	case n > MaxLimit:
		return MaxLimit
	default:
		return n
	}
}

// ─── Bibliothèques ───────────────────────────────────────────────────────────

func (s *Service) ListLibraries(ctx context.Context, v Viewer) ([]Library, error) {
	return s.repo.ListVisibleLibraries(ctx, v)
}

// CanAccessLibrary indique si le viewer peut consulter une bibliothèque.
//
// Exposée pour que l'ingestion applique la MÊME règle de visibilité que la
// consultation, sans la réimplémenter : deux formulations d'une même règle
// finissent toujours par diverger.
func (s *Service) CanAccessLibrary(ctx context.Context, v Viewer, libraryID uuid.UUID) (bool, error) {
	return s.repo.CanAccessLibrary(ctx, v, libraryID)
}

// visibleLibraryIDs résout l'ensemble des bibliothèques que le viewer peut
// consulter, éventuellement restreint à une seule.
//
// Retourne une erreur si la bibliothèque demandée existe mais n'est pas
// accessible — plutôt qu'une liste vide, qui masquerait la cause.
func (s *Service) visibleLibraryIDs(ctx context.Context, v Viewer, only *uuid.UUID) ([]uuid.UUID, error) {
	if only != nil {
		allowed, err := s.repo.CanAccessLibrary(ctx, v, *only)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, ErrForbidden
		}
		return []uuid.UUID{*only}, nil
	}

	libs, err := s.repo.ListVisibleLibraries(ctx, v)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(libs))
	for _, l := range libs {
		ids = append(ids, l.ID)
	}
	return ids, nil
}

// ─── Albums ──────────────────────────────────────────────────────────────────

// ListComicsQuery décrit ce que l'appelant demande, avant résolution des accès.
type ListComicsQuery struct {
	LibraryID *uuid.UUID
	SeriesID  *uuid.UUID
	State     string
	// ReadStatus filtre sur la progression : unread, in_progress, read.
	// Vide, aucun filtre.
	ReadStatus string
	// Folder restreint à un dossier et ses sous-dossiers.
	Folder        *string
	FavoritesOnly bool
	Sort          string
	Cursor        string
	Limit         int32
}

func (s *Service) ListComics(ctx context.Context, v Viewer, q ListComicsQuery) (Page[Comic], error) {
	libraryIDs, err := s.visibleLibraryIDs(ctx, v, q.LibraryID)
	if err != nil {
		return Page[Comic]{}, err
	}
	if len(libraryIDs) == 0 {
		return Page[Comic]{Items: []Comic{}}, nil
	}

	cursor, err := DecodeCursor(q.Cursor)
	if err != nil {
		return Page[Comic]{}, err
	}

	sort := ParseSort(q.Sort)

	// Un curseur produit sous un autre tri désigne une position qui n'a plus
	// de sens : on repart du début plutôt que de servir des résultats
	// incohérents. C'est ce qui arrive quand l'utilisateur change de tri sans
	// que le client pense à réinitialiser sa pagination.
	if cursor != nil && cursor.Sort != sort {
		cursor = nil
	}

	limit := clampLimit(q.Limit)

	// Une ligne de plus que demandé : sa présence indique qu'il reste une page,
	// sans avoir à compter le total — un COUNT sur une grande table coûterait
	// plus cher que la requête elle-même.
	comics, err := s.repo.ListComics(ctx, ListComicsParams{
		UserID:        v.UserID,
		LibraryIDs:    libraryIDs,
		SeriesID:      q.SeriesID,
		State:         q.State,
		ReadStatus:    normalizeReadStatus(q.ReadStatus),
		Folder:        q.Folder,
		FavoritesOnly: q.FavoritesOnly,
		Sort:          sort,
		MaxAgeRating:  v.MaxAgeRating,
		Cursor:        cursor,
		Limit:         limit + 1,
	})
	if err != nil {
		return Page[Comic]{}, err
	}

	return paginate(comics, limit, func(c Comic) string {
		return EncodeCursor(Cursor{
			Sort:       sort,
			CreatedAt:  c.CreatedAt,
			Title:      c.Title,
			ReleasedAt: c.ReleasedAt,
			ID:         c.ID,
		})
	}), nil
}

// normalizeReadStatus écarte les valeurs inconnues.
//
// Comme pour le tri, un filtre mal orthographié ne justifie pas de rejeter la
// requête : on l'ignore, et l'utilisateur voit tout.
func normalizeReadStatus(s string) string {
	switch s {
	case "unread", "in_progress", "read":
		return s
	default:
		return ""
	}
}

func (s *Service) GetComic(ctx context.Context, v Viewer, id uuid.UUID) (Comic, error) {
	comic, err := s.repo.GetComic(ctx, id)
	if err != nil {
		return Comic{}, err
	}

	// Le contrôle d'accès porte sur la bibliothèque, pas sur l'album : on ne
	// peut pas atteindre un album d'une bibliothèque restreinte en devinant son
	// identifiant.
	allowed, err := s.repo.CanAccessLibrary(ctx, v, comic.LibraryID)
	if err != nil {
		return Comic{}, err
	}
	if !allowed {
		// Pas « interdit » mais « introuvable » : répondre 403 confirmerait
		// l'existence de l'album à quelqu'un qui n'y a pas droit.
		return Comic{}, ErrNotFound
	}

	if !s.ageAllowed(v, comic.AgeRating) {
		return Comic{}, ErrNotFound
	}
	return comic, nil
}

// ageAllowed applique la limite de classification d'âge d'un profil restreint.
func (s *Service) ageAllowed(v Viewer, rating *int16) bool {
	if v.MaxAgeRating == nil || rating == nil {
		return true
	}
	return *rating <= *v.MaxAgeRating
}

// ─── Séries ──────────────────────────────────────────────────────────────────

func (s *Service) ListSeries(ctx context.Context, v Viewer, libraryID *uuid.UUID, after string, limit int32) (Page[Series], error) {
	libraryIDs, err := s.visibleLibraryIDs(ctx, v, libraryID)
	if err != nil {
		return Page[Series]{}, err
	}
	if len(libraryIDs) == 0 {
		return Page[Series]{Items: []Series{}}, nil
	}

	l := clampLimit(limit)

	series, err := s.repo.ListSeries(ctx, ListSeriesParams{
		LibraryIDs: libraryIDs,
		AfterSort:  after,
		Limit:      l + 1,
	})
	if err != nil {
		return Page[Series]{}, err
	}

	// Les séries se paginent sur sort_name, qui est unique par bibliothèque et
	// déjà l'ordre d'affichage : pas besoin d'un curseur composite.
	return paginate(series, l, func(s Series) string { return s.SortName }), nil
}

func (s *Service) GetSeries(ctx context.Context, v Viewer, id uuid.UUID) (Series, []Comic, error) {
	series, err := s.repo.GetSeries(ctx, id)
	if err != nil {
		return Series{}, nil, err
	}

	allowed, err := s.repo.CanAccessLibrary(ctx, v, series.LibraryID)
	if err != nil {
		return Series{}, nil, err
	}
	if !allowed {
		return Series{}, nil, ErrNotFound
	}

	comics, err := s.repo.ListComicsBySeries(ctx, id)
	if err != nil {
		return Series{}, nil, err
	}

	// Le filtrage par âge s'applique aussi aux albums d'une série : une série
	// tout public peut contenir un tome plus âgé.
	filtered := make([]Comic, 0, len(comics))
	for _, c := range comics {
		if s.ageAllowed(v, c.AgeRating) {
			filtered = append(filtered, c)
		}
	}

	return series, filtered, nil
}

// ─── Recherche ───────────────────────────────────────────────────────────────

// SearchResult agrège albums et séries d'une même requête.
type SearchResult struct {
	Comics []Comic
	Series []Series
}

// MinQueryLength : en deçà, la similarité trigramme rend n'importe quoi et la
// requête devient un balayage complet.
const MinQueryLength = 2

func (s *Service) Search(ctx context.Context, v Viewer, query string, libraryID *uuid.UUID, limit int32) (SearchResult, error) {
	if len([]rune(query)) < MinQueryLength {
		return SearchResult{Comics: []Comic{}, Series: []Series{}}, nil
	}

	libraryIDs, err := s.visibleLibraryIDs(ctx, v, libraryID)
	if err != nil {
		return SearchResult{}, err
	}
	if len(libraryIDs) == 0 {
		return SearchResult{Comics: []Comic{}, Series: []Series{}}, nil
	}

	p := SearchParams{
		LibraryIDs:   libraryIDs,
		Query:        query,
		MaxAgeRating: v.MaxAgeRating,
		Limit:        clampLimit(limit),
	}

	comics, err := s.repo.SearchComics(ctx, p)
	if err != nil {
		return SearchResult{}, fmt.Errorf("recherche d'albums : %w", err)
	}
	series, err := s.repo.SearchSeries(ctx, p)
	if err != nil {
		return SearchResult{}, fmt.Errorf("recherche de séries : %w", err)
	}

	return SearchResult{Comics: comics, Series: series}, nil
}

// ─── Étagères d'accueil ──────────────────────────────────────────────────────

// Home rassemble ce qui s'affiche sur la page d'accueil.
type Home struct {
	Recent       []Comic
	NextInSeries []Comic
}

// GetHome construit la page d'accueil.
//
// « Reprendre la lecture » n'est pas ici : il relève de la progression, dont le
// catalogue n'a pas à dépendre. Le handler compose les deux.
func (s *Service) GetHome(ctx context.Context, v Viewer, limit int32) (Home, error) {
	libraryIDs, err := s.visibleLibraryIDs(ctx, v, nil)
	if err != nil {
		return Home{}, err
	}
	if len(libraryIDs) == 0 {
		return Home{Recent: []Comic{}, NextInSeries: []Comic{}}, nil
	}

	l := clampLimit(limit)

	recent, err := s.repo.ListRecent(ctx, ListComicsParams{
		LibraryIDs:   libraryIDs,
		MaxAgeRating: v.MaxAgeRating,
		Limit:        l,
	})
	if err != nil {
		return Home{}, err
	}

	next, err := s.repo.ListNextInSeries(ctx, v, libraryIDs, l)
	if err != nil {
		return Home{}, err
	}

	return Home{Recent: recent, NextInSeries: next}, nil
}

// ─── Pagination ──────────────────────────────────────────────────────────────

// paginate coupe la ligne excédentaire et en dérive le curseur suivant.
func paginate[T any](items []T, limit int32, cursorOf func(T) string) Page[T] {
	if int64(len(items)) <= int64(limit) {
		if items == nil {
			items = []T{}
		}
		return Page[T]{Items: items}
	}

	items = items[:limit]
	return Page[T]{
		Items:      items,
		NextCursor: cursorOf(items[len(items)-1]),
	}
}
