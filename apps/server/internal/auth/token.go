package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrTokenInvalid = errors.New("auth : jeton invalide")
	ErrTokenExpired = errors.New("auth : jeton expiré")
)

// Claims est le contenu d'un jeton d'accès.
//
// Volontairement minimal : le jeton est présenté à chaque requête et sa taille
// compte. Tout ce qui peut être relu en base — préférences, bibliothèques
// accessibles — n'y figure pas.
type Claims struct {
	UserID   uuid.UUID `json:"sub"`
	Username string    `json:"usr"`
	Role     string    `json:"rol"`
	DeviceID uuid.UUID `json:"dev,omitempty"`
	IssuedAt int64     `json:"iat"`
	Expires  int64     `json:"exp"`
}

// TokenIssuer signe et vérifie les jetons d'accès.
//
// JWT HS256 écrit à la main plutôt qu'une dépendance : le besoin tient en
// cinquante lignes, et les bibliothèques JWT ont un historique fourni de
// vulnérabilités liées à leur généralité — algorithme « none », confusion
// entre clé publique et secret HMAC. Ici un seul algorithme est accepté, et
// il est codé en dur.
type TokenIssuer struct {
	secret    []byte
	accessTTL time.Duration
}

func NewTokenIssuer(secret []byte, accessTTL time.Duration) *TokenIssuer {
	return &TokenIssuer{secret: secret, accessTTL: accessTTL}
}

// header JWT fixe : un seul algorithme accepté, aucune négociation possible.
var jwtHeader = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

// Issue produit un jeton d'accès signé.
func (t *TokenIssuer) Issue(userID uuid.UUID, username, role string, deviceID uuid.UUID) (string, time.Time, error) {
	now := time.Now()
	expires := now.Add(t.accessTTL)

	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		DeviceID: deviceID,
		IssuedAt: now.Unix(),
		Expires:  expires.Unix(),
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth : sérialisation des claims : %w", err)
	}

	body := jwtHeader + "." + base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + t.sign(body), expires, nil
}

// Verify valide la signature et l'expiration d'un jeton.
func (t *TokenIssuer) Verify(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrTokenInvalid
	}

	// La signature est vérifiée AVANT toute désérialisation du contenu : on ne
	// traite jamais des données non authentifiées.
	body := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(t.sign(body)), []byte(parts[2])) {
		return Claims{}, ErrTokenInvalid
	}
	// L'en-tête doit être exactement celui que l'on émet : cela ferme la
	// famille d'attaques par substitution d'algorithme.
	if parts[0] != jwtHeader {
		return Claims{}, ErrTokenInvalid
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrTokenInvalid
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, ErrTokenInvalid
	}
	if time.Now().Unix() >= claims.Expires {
		return Claims{}, ErrTokenExpired
	}
	return claims, nil
}

func (t *TokenIssuer) sign(body string) string {
	mac := hmac.New(sha256.New, t.secret)
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// ─── Refresh tokens ──────────────────────────────────────────────────────────

// refreshTokenBytes : 32 octets d'entropie, largement au-delà du devinable.
const refreshTokenBytes = 32

// NewRefreshToken produit un jeton opaque et son empreinte.
//
// Le jeton part au client, l'empreinte seule est stockée. Une fuite de la base
// ne donne donc aucune session utilisable — c'est la même logique que pour un
// mot de passe, sans le coût d'un argon2 à chaque rafraîchissement.
func NewRefreshToken() (token string, hash []byte, err error) {
	raw := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("auth : génération du refresh token : %w", err)
	}

	token = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, sum[:], nil
}

// HashRefreshToken recalcule l'empreinte d'un jeton présenté par un client.
func HashRefreshToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
