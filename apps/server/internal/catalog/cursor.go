package catalog

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrBadCursor signale un curseur illisible.
var ErrBadCursor = errors.New("catalog : curseur invalide")

// Sort désigne un ordre de tri.
type Sort string

const (
	SortRecent   Sort = "recent"   // date d'ajout, du plus récent au plus ancien
	SortTitle    Sort = "title"    // alphabétique
	SortReleased Sort = "released" // date de parution
)

// ParseSort valide un ordre demandé, en retombant sur le défaut.
//
// Une valeur inconnue n'est pas une erreur : le tri est une préférence
// d'affichage, et refuser la requête pour un paramètre mal orthographié serait
// disproportionné.
func ParseSort(s string) Sort {
	switch Sort(s) {
	case SortTitle:
		return SortTitle
	case SortReleased:
		return SortReleased
	default:
		return SortRecent
	}
}

// Cursor est la position de pagination des albums.
//
// Le champ significatif dépend du tri : la date d'ajout, le titre ou la date de
// parution. L'identifiant complète toujours, pour que l'ordre soit total —
// deux albums de même titre restent départagés, donc aucun n'est sauté ni servi
// deux fois.
type Cursor struct {
	Sort       Sort
	CreatedAt  time.Time
	Title      string
	ReleasedAt *time.Time
	ID         uuid.UUID
}

// EncodeCursor produit une chaîne opaque pour le client.
//
// L'encodage base64 ne protège rien — ce n'est pas son rôle. Il signale que la
// valeur ne doit pas être interprétée ni construite par le client, ce qui
// laisse la liberté de changer de stratégie de pagination sans rompre l'API.
func EncodeCursor(c Cursor) string {
	var key string
	switch c.Sort {
	case SortTitle:
		key = c.Title
	case SortReleased:
		if c.ReleasedAt != nil {
			key = c.ReleasedAt.UTC().Format("2006-01-02")
		}
	default:
		key = c.CreatedAt.UTC().Format(time.RFC3339Nano)
	}

	// Le tri fait partie du curseur : changer de tri en cours de pagination
	// rendrait la position précédente absurde, et il vaut mieux le détecter
	// que de servir des résultats incohérents.
	raw := string(c.Sort) + "|" + key + "|" + c.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor relit un curseur. Une chaîne vide n'est pas une erreur : c'est
// la première page.
func DecodeCursor(s string) (*Cursor, error) {
	if s == "" {
		return nil, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrBadCursor
	}

	parts := strings.SplitN(string(raw), "|", 3)
	if len(parts) != 3 {
		return nil, ErrBadCursor
	}

	id, err := uuid.Parse(parts[2])
	if err != nil {
		return nil, fmt.Errorf("%w : identifiant illisible", ErrBadCursor)
	}

	cursor := &Cursor{Sort: ParseSort(parts[0]), ID: id}

	switch cursor.Sort {
	case SortTitle:
		cursor.Title = parts[1]
	case SortReleased:
		if parts[1] != "" {
			d, err := time.Parse("2006-01-02", parts[1])
			if err != nil {
				return nil, fmt.Errorf("%w : date illisible", ErrBadCursor)
			}
			cursor.ReleasedAt = &d
		}
	default:
		ts, err := time.Parse(time.RFC3339Nano, parts[1])
		if err != nil {
			return nil, fmt.Errorf("%w : horodatage illisible", ErrBadCursor)
		}
		cursor.CreatedAt = ts
	}

	return cursor, nil
}
