// Package crypto chiffre les secrets stockés en base.
//
// Sert principalement aux identifiants des backends de stockage : ils doivent
// être réutilisables par le serveur (donc réversibles, pas hachés) mais ne
// jamais être lisibles par quelqu'un qui obtiendrait un accès à la base.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

var (
	ErrInvalidKey         = errors.New("crypto : la clé doit faire 32 octets")
	ErrCiphertextTooShort = errors.New("crypto : chiffré trop court ou corrompu")
	ErrDecryptionFailed   = errors.New("crypto : déchiffrement impossible (mauvaise clé ou données altérées)")
)

// Sealer chiffre et déchiffre en AES-256-GCM.
//
// GCM est authentifié : une altération du chiffré est détectée au
// déchiffrement plutôt que de produire silencieusement des octets erronés.
type Sealer struct {
	aead cipher.AEAD
}

// NewSealer construit un Sealer à partir d'une clé de 32 octets
// (BOXINCLOUD_SECRET_KEY).
func NewSealer(key []byte) (*Sealer, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto : %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto : %w", err)
	}

	return &Sealer{aead: aead}, nil
}

// Seal chiffre plaintext. Le nonce est généré aléatoirement et préfixé au
// résultat, de sorte que le chiffré est autonome.
func (s *Sealer) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto : génération du nonce : %w", err)
	}
	return s.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Open déchiffre un chiffré produit par Seal.
func (s *Sealer) Open(ciphertext []byte) ([]byte, error) {
	n := s.aead.NonceSize()
	if len(ciphertext) < n+s.aead.Overhead() {
		return nil, ErrCiphertextTooShort
	}

	nonce, body := ciphertext[:n], ciphertext[n:]
	plaintext, err := s.aead.Open(nil, nonce, body, nil)
	if err != nil {
		// L'erreur sous-jacente n'apporte rien et pourrait fuiter dans un log.
		return nil, ErrDecryptionFailed
	}
	return plaintext, nil
}
