// Package progress gère la progression de lecture et sa synchronisation.
//
// Le point délicat : deux appareils peuvent lire le même album hors ligne, puis
// se synchroniser. La règle retenue est **la page la plus avancée gagne**, sauf
// remise à zéro explicite.
//
// C'est ce qu'attend un lecteur : on ne perd jamais sa progression, et lire sur
// tablette puis reprendre sur téléphone reprend au bon endroit. Un « dernière
// écriture gagne » ferait régresser la position dès que les horloges des
// appareils divergent un peu — et elles divergent toujours.
//
// La règle est appliquée en SQL, dans la clause ON CONFLICT de l'upsert : aucun
// chemin de code ne peut la contourner, y compris un futur import en masse.
package progress

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidCursor = errors.New("progress : curseur de synchronisation invalide")

// Status reprend l'état de lecture d'un album.
type Status string

const (
	StatusUnread     Status = "unread"
	StatusInProgress Status = "in_progress"
	StatusRead       Status = "read"
)

// Valid indique si un statut est reconnu.
func (s Status) Valid() bool {
	switch s {
	case StatusUnread, StatusInProgress, StatusRead:
		return true
	default:
		return false
	}
}

// Progress est la progression d'un utilisateur sur un album.
type Progress struct {
	ComicID    uuid.UUID
	Page       int32
	PageCount  int32
	Status     Status
	ReadCount  int32
	Version    int64
	DeviceID   *uuid.UUID
	StartedAt  *time.Time
	FinishedAt *time.Time
	UpdatedAt  time.Time
}

// Percent retourne l'avancement en pourcentage.
func (p Progress) Percent() float64 {
	if p.PageCount <= 0 {
		return 0
	}
	return float64(p.Page+1) / float64(p.PageCount) * 100
}

// Update est une écriture de progression envoyée par un client.
type Update struct {
	ComicID   uuid.UUID
	Page      int32
	PageCount int32
	Status    Status
	DeviceID  *uuid.UUID
}

// Repository est ce dont le module a besoin de la persistance.
type Repository interface {
	Upsert(ctx context.Context, userID uuid.UUID, u Update) (Progress, error)
	Get(ctx context.Context, userID, comicID uuid.UUID) (Progress, error)
	ListByComics(ctx context.Context, userID uuid.UUID, comicIDs []uuid.UUID) ([]Progress, error)
	ListSince(ctx context.Context, userID uuid.UUID, since time.Time, limit int32) ([]Progress, error)
	ListInProgress(ctx context.Context, userID uuid.UUID, limit int32) ([]Progress, error)
	Delete(ctx context.Context, userID, comicID uuid.UUID) error
}

// Service porte la logique de progression.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// Record enregistre une progression.
func (s *Service) Record(ctx context.Context, userID uuid.UUID, u Update) (Progress, error) {
	if !u.Status.Valid() {
		// Statut absent ou inconnu : on le déduit de la position plutôt que de
		// rejeter. Un client qui n'envoie qu'un numéro de page est le cas le
		// plus courant, et le plus simple à implémenter côté mobile.
		u.Status = deriveStatus(u.Page, u.PageCount)
	}
	if u.Page < 0 {
		u.Page = 0
	}
	return s.repo.Upsert(ctx, userID, u)
}

// deriveStatus déduit l'état de lecture de la position.
func deriveStatus(page, pageCount int32) Status {
	switch {
	case pageCount > 0 && page >= pageCount-1:
		return StatusRead
	case page > 0:
		return StatusInProgress
	default:
		return StatusUnread
	}
}

func (s *Service) Get(ctx context.Context, userID, comicID uuid.UUID) (Progress, error) {
	return s.repo.Get(ctx, userID, comicID)
}

// ListForComics retourne la progression sur un lot d'albums.
//
// Sert à annoter une grille de couvertures en une requête, plutôt qu'une par
// vignette affichée.
func (s *Service) ListForComics(ctx context.Context, userID uuid.UUID, comicIDs []uuid.UUID) (map[uuid.UUID]Progress, error) {
	if len(comicIDs) == 0 {
		return map[uuid.UUID]Progress{}, nil
	}

	rows, err := s.repo.ListByComics(ctx, userID, comicIDs)
	if err != nil {
		return nil, err
	}

	out := make(map[uuid.UUID]Progress, len(rows))
	for _, p := range rows {
		out[p.ComicID] = p
	}
	return out, nil
}

// ContinueReading retourne les albums commencés mais non terminés.
func (s *Service) ContinueReading(ctx context.Context, userID uuid.UUID, limit int32) ([]Progress, error) {
	return s.repo.ListInProgress(ctx, userID, clampLimit(limit))
}

func (s *Service) Delete(ctx context.Context, userID, comicID uuid.UUID) error {
	return s.repo.Delete(ctx, userID, comicID)
}

// ─── Synchronisation ─────────────────────────────────────────────────────────

const (
	DefaultSyncLimit = 200
	MaxSyncLimit     = 1000
)

func clampLimit(n int32) int32 {
	switch {
	case n <= 0:
		return DefaultSyncLimit
	case n > MaxSyncLimit:
		return MaxSyncLimit
	default:
		return n
	}
}

// SyncResult est une tranche de changements serveur.
type SyncResult struct {
	Changes []Progress
	// Cursor à renvoyer au prochain appel.
	Cursor string
	// HasMore indique qu'une nouvelle page est disponible immédiatement — un
	// client qui rattrape une longue absence doit boucler sans attendre.
	HasMore bool
}

// Pull retourne les progressions modifiées depuis le curseur du client.
//
// Le curseur est l'horodatage de la dernière modification vue. Il est renvoyé
// tel quel par le client, qui n'a pas à l'interpréter.
func (s *Service) Pull(ctx context.Context, userID uuid.UUID, cursor string, limit int32) (SyncResult, error) {
	since, err := decodeCursor(cursor)
	if err != nil {
		return SyncResult{}, err
	}

	l := clampLimit(limit)

	// Une ligne de plus que demandé : sa présence signale qu'il en reste.
	changes, err := s.repo.ListSince(ctx, userID, since, l+1)
	if err != nil {
		return SyncResult{}, err
	}

	hasMore := int64(len(changes)) > int64(l)
	if hasMore {
		changes = changes[:l]
	}

	// Curseur inchangé s'il n'y a rien de neuf : le client ne doit pas avancer
	// dans le temps sans avoir rien reçu, sinon une écriture concurrente
	// arrivée entre-temps serait sautée.
	next := cursor
	if len(changes) > 0 {
		next = encodeCursor(changes[len(changes)-1].UpdatedAt)
	}

	return SyncResult{Changes: changes, Cursor: next, HasMore: hasMore}, nil
}

// Push applique un lot de progressions accumulées hors ligne.
//
// Chaque écriture passe par la même règle de résolution que les écritures en
// ligne : un lot rejoué deux fois ne peut pas faire régresser la position.
// C'est ce qui rend la synchronisation sûre en cas de reprise après échec.
func (s *Service) Push(ctx context.Context, userID uuid.UUID, updates []Update) ([]Progress, error) {
	out := make([]Progress, 0, len(updates))

	for _, u := range updates {
		p, err := s.Record(ctx, userID, u)
		if err != nil {
			return out, err
		}
		out = append(out, p)
	}
	return out, nil
}

// encodeCursor et decodeCursor : l'horodatage en RFC 3339 à la nanoseconde.
//
// Lisible dans les logs et dans une requête, ce qui rend le diagnostic d'un
// problème de synchronisation nettement plus simple qu'avec une valeur opaque.
func encodeCursor(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func decodeCursor(s string) (time.Time, error) {
	if s == "" {
		// Première synchronisation : tout depuis l'origine.
		return time.Unix(0, 0).UTC(), nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, ErrInvalidCursor
	}
	return t, nil
}
