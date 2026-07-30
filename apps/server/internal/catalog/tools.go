package catalog

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrTooManyItems borne une action en lot.
var ErrTooManyItems = errors.New("catalog : trop d'éléments dans le lot")

// ErrUnknownAction signale une action en lot que le service ne connaît pas.
//
// C'est une erreur de la requête, pas du serveur : un client qui invente une
// action doit recevoir un refus explicite, pas une erreur interne.
var ErrUnknownAction = errors.New("catalog : action en lot inconnue")

// MaxBulkItems : au-delà, l'action est refusée.
//
// Mille albums couvrent largement une sélection réelle — « tout marquer comme
// lu » sur une série entière — tout en écartant une requête qui bloquerait la
// base plusieurs secondes.
const MaxBulkItems = 1000

// ToolsRepository porte les actions de gestion.
type ToolsRepository interface {
	ListFolders(ctx context.Context, libraryIDs []uuid.UUID) (map[string]int, error)

	SetFavorite(ctx context.Context, userID, comicID uuid.UUID, favorite bool) error
	ListFavorites(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)

	SetRating(ctx context.Context, userID, comicID uuid.UUID, rating int16) error
	ClearRating(ctx context.Context, userID, comicID uuid.UUID) error
	ListRatings(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]int16, error)

	EditComic(ctx context.Context, id uuid.UUID, edit ComicEdit) (Comic, error)

	BulkMarkRead(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) (int64, error)
	BulkMarkUnread(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) (int64, error)
	BulkSetFavorite(ctx context.Context, userID uuid.UUID, ids []uuid.UUID, favorite bool) (int64, error)
}

// ComicEdit décrit une édition manuelle.
//
// Chaque champ non nul est appliqué ET verrouillé : une réindexation ne doit
// jamais écraser une correction saisie à la main. C'est la contrepartie
// indispensable de l'automatisme — sans elle, corriger un titre serait vain.
type ComicEdit struct {
	Title    *string
	Number   *string
	Summary  *string
	Language *string
}

// LockedFields liste les champs que cette édition doit verrouiller.
func (e ComicEdit) LockedFields() []string {
	var fields []string
	if e.Title != nil {
		fields = append(fields, "title")
	}
	if e.Number != nil {
		fields = append(fields, "number")
	}
	if e.Summary != nil {
		fields = append(fields, "summary")
	}
	if e.Language != nil {
		fields = append(fields, "language")
	}
	return fields
}

// IsEmpty indique qu'aucun champ n'est modifié.
func (e ComicEdit) IsEmpty() bool { return len(e.LockedFields()) == 0 }

// ─── Service ─────────────────────────────────────────────────────────────────

// Tools expose les actions de gestion de bibliothèque.
//
// Séparé du service de consultation : lire un catalogue et le modifier sont
// deux préoccupations distinctes, et les garder séparées permettra d'exiger
// des droits différents en M7.
type Tools struct {
	repo    ToolsRepository
	catalog *Service
}

func NewTools(repo ToolsRepository, catalog *Service) *Tools {
	return &Tools{repo: repo, catalog: catalog}
}

// FolderCounts retourne les dossiers observés et leur nombre d'albums.
func (t *Tools) FolderCounts(ctx context.Context, v Viewer, libraryID *uuid.UUID) (map[string]int, error) {
	ids, err := t.catalog.visibleLibraryIDs(ctx, v, libraryID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return map[string]int{}, nil
	}
	return t.repo.ListFolders(ctx, ids)
}

// ─── Favoris et notes ────────────────────────────────────────────────────────

func (t *Tools) SetFavorite(ctx context.Context, v Viewer, comicID uuid.UUID, favorite bool) error {
	// L'accès est vérifié avant toute écriture : sans cela, on pourrait mettre
	// en favori un album d'une bibliothèque à laquelle on n'a pas droit, et en
	// déduire son existence.
	if _, err := t.catalog.GetComic(ctx, v, comicID); err != nil {
		return err
	}
	return t.repo.SetFavorite(ctx, v.UserID, comicID, favorite)
}

func (t *Tools) Favorites(ctx context.Context, v Viewer) ([]uuid.UUID, error) {
	return t.repo.ListFavorites(ctx, v.UserID)
}

// SetRating note un album de 1 à 5. Une note de 0 la retire.
func (t *Tools) SetRating(ctx context.Context, v Viewer, comicID uuid.UUID, rating int16) error {
	if _, err := t.catalog.GetComic(ctx, v, comicID); err != nil {
		return err
	}
	if rating <= 0 {
		return t.repo.ClearRating(ctx, v.UserID, comicID)
	}
	if rating > 5 {
		rating = 5
	}
	return t.repo.SetRating(ctx, v.UserID, comicID, rating)
}

func (t *Tools) Ratings(ctx context.Context, v Viewer) (map[uuid.UUID]int16, error) {
	return t.repo.ListRatings(ctx, v.UserID)
}

// ─── Édition ─────────────────────────────────────────────────────────────────

func (t *Tools) EditComic(ctx context.Context, v Viewer, comicID uuid.UUID, edit ComicEdit) (Comic, error) {
	if _, err := t.catalog.GetComic(ctx, v, comicID); err != nil {
		return Comic{}, err
	}
	if edit.IsEmpty() {
		return t.catalog.GetComic(ctx, v, comicID)
	}
	return t.repo.EditComic(ctx, comicID, edit)
}

// ─── Actions en lot ──────────────────────────────────────────────────────────

// BulkAction désigne une action applicable à une sélection.
type BulkAction string

const (
	BulkRead       BulkAction = "read"
	BulkUnread     BulkAction = "unread"
	BulkFavorite   BulkAction = "favorite"
	BulkUnfavorite BulkAction = "unfavorite"
)

// Bulk applique une action à une sélection d'albums.
//
// Les identifiants sont filtrés sur les bibliothèques visibles avant toute
// écriture : une sélection ne peut pas déborder sur ce à quoi l'utilisateur
// n'a pas accès, même si le client envoie des identifiants arbitraires.
func (t *Tools) Bulk(ctx context.Context, v Viewer, action BulkAction, ids []uuid.UUID) (int64, error) {
	// L'action se valide avant tout le reste : inutile d'interroger la base
	// pour une requête qu'on refusera de toute façon.
	switch action {
	case BulkRead, BulkUnread, BulkFavorite, BulkUnfavorite:
	default:
		return 0, ErrUnknownAction
	}

	if len(ids) == 0 {
		return 0, nil
	}
	if len(ids) > MaxBulkItems {
		return 0, ErrTooManyItems
	}

	allowed, err := t.filterAccessible(ctx, v, ids)
	if err != nil {
		return 0, err
	}
	if len(allowed) == 0 {
		return 0, nil
	}

	switch action {
	case BulkRead:
		return t.repo.BulkMarkRead(ctx, v.UserID, allowed)
	case BulkUnread:
		return t.repo.BulkMarkUnread(ctx, v.UserID, allowed)
	case BulkFavorite:
		return t.repo.BulkSetFavorite(ctx, v.UserID, allowed, true)
	case BulkUnfavorite:
		return t.repo.BulkSetFavorite(ctx, v.UserID, allowed, false)
	default:
		return 0, ErrUnknownAction
	}
}

// filterAccessible ne conserve que les albums visibles par le viewer.
func (t *Tools) filterAccessible(ctx context.Context, v Viewer, ids []uuid.UUID) ([]uuid.UUID, error) {
	// Le contrôle passe par GetComic, qui porte déjà toute la règle de
	// visibilité — bibliothèque restreinte comme classification d'âge. La
	// réimplémenter ici la ferait diverger.
	out := make([]uuid.UUID, 0, len(ids))

	for _, id := range ids {
		if _, err := t.catalog.GetComic(ctx, v, id); err == nil {
			out = append(out, id)
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	return out, nil
}
