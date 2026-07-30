package crypto

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"
)

func newTestSealer(t *testing.T) *Sealer {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	s, err := NewSealer(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSealOpenRoundTrip(t *testing.T) {
	s := newTestSealer(t)
	plaintext := []byte(`{"access_key":"AKIA...","secret_key":"wJalr..."}`)

	sealed, err := s.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal : %v", err)
	}
	if bytes.Contains(sealed, plaintext) {
		t.Fatal("le chiffré contient le clair")
	}

	opened, err := s.Open(sealed)
	if err != nil {
		t.Fatalf("Open : %v", err)
	}
	if !bytes.Equal(opened, plaintext) {
		t.Fatalf("aller-retour : %q, attendu %q", opened, plaintext)
	}
}

func TestSealIsNonDeterministic(t *testing.T) {
	s := newTestSealer(t)
	plaintext := []byte("même contenu")

	a, err := s.Seal(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Seal(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("deux chiffrés du même clair sont identiques : le nonce n'est pas aléatoire")
	}
}

func TestOpenRejectsTamperedCiphertext(t *testing.T) {
	s := newTestSealer(t)

	sealed, err := s.Seal([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	sealed[len(sealed)-1] ^= 0xFF // altère un octet

	if _, err := s.Open(sealed); err == nil {
		t.Fatal("Open devrait rejeter un chiffré altéré")
	}
}

func TestOpenRejectsWrongKey(t *testing.T) {
	a, b := newTestSealer(t), newTestSealer(t)

	sealed, err := a.Seal([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Open(sealed); !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("attendu ErrDecryptionFailed, obtenu %v", err)
	}
}

func TestNewSealerRejectsBadKeyLength(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33, 64} {
		if _, err := NewSealer(make([]byte, n)); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("clé de %d octets : attendu ErrInvalidKey, obtenu %v", n, err)
		}
	}
}

func TestOpenRejectsTooShort(t *testing.T) {
	s := newTestSealer(t)
	if _, err := s.Open([]byte{1, 2, 3}); !errors.Is(err, ErrCiphertextTooShort) {
		t.Fatalf("attendu ErrCiphertextTooShort, obtenu %v", err)
	}
}
