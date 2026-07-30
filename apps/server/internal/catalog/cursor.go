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

// Cursor est la position de pagination des albums.
//
// (created_at, id) forme un ordre total : deux albums créés à la même
// microseconde restent départagés par leur identifiant, donc aucun n'est sauté
// ni servi deux fois.
type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// EncodeCursor produit une chaîne opaque pour le client.
//
// L'encodage base64 ne protège rien — ce n'est pas son rôle. Il signale que la
// valeur ne doit pas être interprétée ni construite par le client, ce qui
// laisse la liberté de changer de stratégie de pagination sans rompre l'API.
func EncodeCursor(c Cursor) string {
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID.String()
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

	tsPart, idPart, ok := strings.Cut(string(raw), "|")
	if !ok {
		return nil, ErrBadCursor
	}

	ts, err := time.Parse(time.RFC3339Nano, tsPart)
	if err != nil {
		return nil, fmt.Errorf("%w : horodatage illisible", ErrBadCursor)
	}
	id, err := uuid.Parse(idPart)
	if err != nil {
		return nil, fmt.Errorf("%w : identifiant illisible", ErrBadCursor)
	}

	return &Cursor{CreatedAt: ts, ID: id}, nil
}
