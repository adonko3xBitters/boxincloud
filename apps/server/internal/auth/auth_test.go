package auth_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/auth"
)

// ─── Mots de passe ───────────────────────────────────────────────────────────

func TestHashAndVerifyPassword(t *testing.T) {
	const password = "un mot de passe correct"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword : %v", err)
	}

	if strings.Contains(hash, password) {
		t.Fatal("le hachage contient le mot de passe en clair")
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("format inattendu : %s", hash)
	}

	if err := auth.VerifyPassword(password, hash); err != nil {
		t.Errorf("VerifyPassword sur le bon mot de passe : %v", err)
	}
	if err := auth.VerifyPassword("un autre mot de passe", hash); err == nil {
		t.Error("VerifyPassword devrait rejeter un mauvais mot de passe")
	}
}

// Deux hachages du même mot de passe doivent différer : sans sel aléatoire,
// une fuite de la base révélerait quels comptes partagent un mot de passe.
func TestHashIsSalted(t *testing.T) {
	const password = "le même mot de passe"

	a, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	b, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}

	if a == b {
		t.Fatal("deux hachages du même mot de passe sont identiques : le sel n'est pas aléatoire")
	}
	for _, h := range []string{a, b} {
		if err := auth.VerifyPassword(password, h); err != nil {
			t.Errorf("hachage non vérifiable : %v", err)
		}
	}
}

func TestHashPasswordRejectsShort(t *testing.T) {
	for _, p := range []string{"", "court", "123456789"} {
		if _, err := auth.HashPassword(p); !errors.Is(err, auth.ErrPasswordTooShort) {
			t.Errorf("mot de passe %q : attendu ErrPasswordTooShort, obtenu %v", p, err)
		}
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	for _, h := range []string{
		"",
		"pas un hachage",
		"$argon2id$",
		"$bcrypt$v=19$m=1,t=1,p=1$c2VsZWw$Y2xl",
		"$argon2id$v=19$m=1,t=1,p=1$pas-du-base64!$Y2xl",
	} {
		if err := auth.VerifyPassword("peu importe", h); err == nil {
			t.Errorf("hachage malformé accepté : %q", h)
		}
	}
}

// Le durcissement progressif : un hachage produit avec des paramètres plus
// faibles doit être signalé, pour être remplacé à la prochaine connexion.
func TestNeedsRehash(t *testing.T) {
	hash, err := auth.HashPassword("un mot de passe correct")
	if err != nil {
		t.Fatal(err)
	}
	if auth.NeedsRehash(hash) {
		t.Error("un hachage frais ne devrait pas demander de réhachage")
	}

	// Paramètres volontairement faibles, format valide.
	weak := "$argon2id$v=19$m=1024,t=1,p=1$c2VsZXNlbGVzZWxlc2U$" +
		"Y2xlY2xlY2xlY2xlY2xlY2xlY2xlY2xlY2xlY2xl"
	if !auth.NeedsRehash(weak) {
		t.Error("un hachage aux paramètres faibles devrait demander un réhachage")
	}
	if !auth.NeedsRehash("format inconnu") {
		t.Error("un format inconnu devrait demander un réhachage")
	}
}

// ─── Jetons d'accès ──────────────────────────────────────────────────────────

func newIssuer() *auth.TokenIssuer {
	return auth.NewTokenIssuer([]byte(strings.Repeat("k", 32)), 15*time.Minute)
}

func TestIssueAndVerifyToken(t *testing.T) {
	issuer := newIssuer()

	userID := uuid.Must(uuid.NewV7())
	deviceID := uuid.Must(uuid.NewV7())

	token, expires, err := issuer.Issue(userID, "nïando", "admin", deviceID)
	if err != nil {
		t.Fatalf("Issue : %v", err)
	}
	if !expires.After(time.Now()) {
		t.Error("le jeton est déjà expiré à l'émission")
	}

	claims, err := issuer.Verify(token)
	if err != nil {
		t.Fatalf("Verify : %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("UserID = %s, attendu %s", claims.UserID, userID)
	}
	if claims.Username != "nïando" {
		t.Errorf("Username = %q", claims.Username)
	}
	if claims.Role != "admin" {
		t.Errorf("Role = %q", claims.Role)
	}
	if claims.DeviceID != deviceID {
		t.Errorf("DeviceID = %s, attendu %s", claims.DeviceID, deviceID)
	}
}

func TestVerifyRejectsTamperedToken(t *testing.T) {
	issuer := newIssuer()

	token, _, err := issuer.Issue(uuid.Must(uuid.NewV7()), "user", "user", uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(token, ".")

	// Charge utile modifiée, signature d'origine conservée.
	tampered := parts[0] + ".eyJzdWIiOiJhZG1pbiJ9." + parts[2]
	if _, err := issuer.Verify(tampered); !errors.Is(err, auth.ErrTokenInvalid) {
		t.Errorf("charge utile altérée : attendu ErrTokenInvalid, obtenu %v", err)
	}

	// Signature modifiée.
	if _, err := issuer.Verify(parts[0] + "." + parts[1] + ".c2lnbmF0dXJlLWJpZG9u"); !errors.Is(err, auth.ErrTokenInvalid) {
		t.Error("signature altérée : le jeton devrait être rejeté")
	}
}

// L'attaque historique sur les JWT : forcer alg=none pour qu'aucune signature
// ne soit vérifiée. L'en-tête étant figé, elle ne peut pas aboutir.
func TestVerifyRejectsAlgNone(t *testing.T) {
	issuer := newIssuer()

	// En-tête alg=none, charge utile arbitraire, signature vide.
	forged := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiIwMDAwMDAwMC0wMDAwLTAwMDAtMDAwMC0wMDAwMDAwMDAwMDAiLCJyb2wiOiJhZG1pbiJ9."

	if _, err := issuer.Verify(forged); err == nil {
		t.Fatal("un jeton alg=none a été accepté")
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	a := auth.NewTokenIssuer([]byte(strings.Repeat("a", 32)), time.Minute)
	b := auth.NewTokenIssuer([]byte(strings.Repeat("b", 32)), time.Minute)

	token, _, err := a.Issue(uuid.Must(uuid.NewV7()), "user", "user", uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Verify(token); !errors.Is(err, auth.ErrTokenInvalid) {
		t.Errorf("attendu ErrTokenInvalid, obtenu %v", err)
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	// TTL négatif : le jeton naît expiré.
	issuer := auth.NewTokenIssuer([]byte(strings.Repeat("k", 32)), -time.Minute)

	token, _, err := issuer.Issue(uuid.Must(uuid.NewV7()), "user", "user", uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := issuer.Verify(token); !errors.Is(err, auth.ErrTokenExpired) {
		t.Errorf("attendu ErrTokenExpired, obtenu %v", err)
	}
}

func TestVerifyRejectsMalformedToken(t *testing.T) {
	issuer := newIssuer()

	for _, token := range []string{"", "abc", "a.b", "a.b.c.d", "....."} {
		if _, err := issuer.Verify(token); err == nil {
			t.Errorf("jeton malformé accepté : %q", token)
		}
	}
}

// ─── Refresh tokens ──────────────────────────────────────────────────────────

func TestRefreshTokenIsOpaqueAndHashed(t *testing.T) {
	token, hash, err := auth.NewRefreshToken()
	if err != nil {
		t.Fatalf("NewRefreshToken : %v", err)
	}

	if len(token) < 40 {
		t.Errorf("jeton trop court (%d caractères) : entropie insuffisante", len(token))
	}
	if len(hash) != 32 {
		t.Errorf("empreinte de %d octets, attendu 32 (SHA-256)", len(hash))
	}
	if strings.Contains(string(hash), token) {
		t.Error("l'empreinte contient le jeton")
	}

	// L'empreinte doit être reproductible : c'est ainsi qu'on retrouve la
	// session à partir du jeton présenté par le client.
	again := auth.HashRefreshToken(token)
	if string(again) != string(hash) {
		t.Error("HashRefreshToken ne reproduit pas l'empreinte d'origine")
	}
}

func TestRefreshTokensAreUnique(t *testing.T) {
	seen := make(map[string]bool, 200)

	for range 200 {
		token, _, err := auth.NewRefreshToken()
		if err != nil {
			t.Fatal(err)
		}
		if seen[token] {
			t.Fatal("collision de refresh token")
		}
		seen[token] = true
	}
}
