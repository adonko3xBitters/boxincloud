// Package auth gère l'authentification : mots de passe, jetons, sessions.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"

	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidCredentials = errors.New("auth : identifiants invalides")
	ErrPasswordTooShort   = errors.New("auth : le mot de passe doit faire au moins 10 caractères")
	ErrHashFormat         = errors.New("auth : format de hachage non reconnu")
)

// MinPasswordLength : 10 caractères.
//
// La longueur prime sur la complexité imposée — une règle « majuscule +
// chiffre + symbole » produit surtout des mots de passe courts et prévisibles.
// Le hachage argon2id absorbe le reste.
const MinPasswordLength = 10

// Paramètres argon2id, recommandations OWASP 2024 (profil m=19 MiB, t=2).
//
// Le parallélisme suit le nombre de cœurs, borné à 4 : au-delà, le gain est
// nul et la mémoire consommée par vérification devient un vecteur de déni de
// service sur une machine modeste — et le public visé s'auto-héberge souvent
// sur un NAS ou un Raspberry Pi.
var defaultParams = argonParams{
	memory:      19 * 1024, // Kio
	iterations:  2,
	parallelism: uint8(min(runtime.NumCPU(), 4)),
	saltLength:  16,
	keyLength:   32,
}

type argonParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

// HashPassword produit un hachage argon2id encodé au format PHC standard.
//
// Le format embarque les paramètres : durcir la configuration plus tard
// n'invalide pas les hachages existants, qui restent vérifiables avec les
// leurs.
func HashPassword(password string) (string, error) {
	if len(password) < MinPasswordLength {
		return "", ErrPasswordTooShort
	}
	return HashSecret(password)
}

/*
HashSecret hache un secret court sans imposer la politique des mots de passe.

Même primitive, même coût, mais sans le minimum de dix caractères : celui-ci
protège l'accès au SERVEUR, alors qu'un code de dossier ne masque qu'une branche
à quelqu'un déjà authentifié. Exiger la même longueur pousserait à le noter à
côté, ce qui protège moins qu'un code court réellement retenu.

L'appelant reste responsable de sa propre longueur minimale.
*/
func HashSecret(secret string) (string, error) {
	return hashWithParams(secret, defaultParams)
}

func hashWithParams(password string, p argonParams) (string, error) {
	salt := make([]byte, p.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth : génération du sel : %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memory, p.iterations, p.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword compare un mot de passe à son hachage.
//
// La comparaison est à temps constant : une comparaison naïve fuiterait le
// nombre d'octets corrects par sa durée.
func VerifyPassword(password, encoded string) error {
	p, salt, want, err := decodeHash(encoded)
	if err != nil {
		return err
	}

	got := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)

	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrInvalidCredentials
	}
	return nil
}

// NeedsRehash indique si un hachage a été produit avec des paramètres plus
// faibles que ceux en vigueur.
//
// Permet de durcir progressivement : à la prochaine connexion réussie, le
// mot de passe est réhaché avec les paramètres courants, sans rien demander à
// l'utilisateur.
func NeedsRehash(encoded string) bool {
	p, _, _, err := decodeHash(encoded)
	if err != nil {
		return true // format inconnu : à remplacer
	}
	return p.memory < defaultParams.memory ||
		p.iterations < defaultParams.iterations ||
		p.keyLength < defaultParams.keyLength
}

func decodeHash(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argonParams{}, nil, nil, ErrHashFormat
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return argonParams{}, nil, nil, ErrHashFormat
	}
	if version != argon2.Version {
		return argonParams{}, nil, nil, fmt.Errorf("%w : version argon2 %d", ErrHashFormat, version)
	}

	var p argonParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism); err != nil {
		return argonParams{}, nil, nil, ErrHashFormat
	}

	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, ErrHashFormat
	}
	key, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, ErrHashFormat
	}

	p.saltLength = safeUint32(len(salt))
	p.keyLength = safeUint32(len(key))

	return p, salt, key, nil
}

// DummyVerify effectue un hachage sur un mot de passe factice.
//
// Appelé quand l'utilisateur demandé n'existe pas, pour que la connexion prenne
// le même temps qu'avec un compte réel. Sans cela, la durée de réponse
// permettrait d'énumérer les comptes existants.
func DummyVerify(password string) {
	_ = argon2.IDKey([]byte(password), make([]byte, defaultParams.saltLength),
		defaultParams.iterations, defaultParams.memory,
		defaultParams.parallelism, defaultParams.keyLength)
}

// safeUint32 borne une longueur lue dans un hachage stocké.
//
// La valeur vient de la base : elle est de confiance en pratique, mais une
// conversion non bornée resterait une conversion non bornée, et le linter a
// raison de le signaler.
func safeUint32(v int) uint32 {
	if v < 0 {
		return 0
	}
	if v > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(v)
}
