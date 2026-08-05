package config

import (
	"strings"
	"testing"
)

func TestParseByteSize(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"1024", 1024, false},
		{"512B", 512, false},
		{"1KB", 1 << 10, false},
		{"10MB", 10 << 20, false},
		{"10GB", 10 << 30, false},
		{"1.5GB", 1610612736, false},
		{"2TB", 2 << 40, false},
		{" 10gb ", 10 << 30, false},
		{"", 0, true},
		{"abc", 0, true},
		{"-1GB", 0, true},
	}

	for _, c := range cases {
		got, err := parseByteSize(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseByteSize(%q) : erreur attendue, obtenu %d", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseByteSize(%q) : erreur inattendue %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseByteSize(%q) = %d, attendu %d", c.in, got, c.want)
		}
	}
}

func TestLoadRequiresDatabaseURLAndSecretKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("BOXINCLOUD_SECRET_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() devrait échouer sans DATABASE_URL ni BOXINCLOUD_SECRET_KEY")
	}
	for _, want := range []string{"DATABASE_URL", "BOXINCLOUD_SECRET_KEY"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("le message d'erreur devrait mentionner %s, obtenu :\n%v", want, err)
		}
	}
}

func TestLoadRejectsShortSecretKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/boxincloud")
	t.Setenv("BOXINCLOUD_SECRET_KEY", "deadbeef") // 4 octets

	_, err := Load()
	if err == nil {
		t.Fatal("Load() devrait rejeter une clé de moins de 32 octets")
	}
	if !strings.Contains(err.Error(), "32 octets") {
		t.Errorf("le message devrait expliquer la taille attendue, obtenu :\n%v", err)
	}
}

func TestLoadValid(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/boxincloud")
	t.Setenv("BOXINCLOUD_SECRET_KEY", strings.Repeat("ab", 32)) // 32 octets
	t.Setenv("BOXINCLOUD_CACHE_MAX_SIZE", "2GB")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() : %v", err)
	}
	if len(cfg.SecretKey) != 32 {
		t.Errorf("SecretKey fait %d octets, attendu 32", len(cfg.SecretKey))
	}
	if cfg.Cache.MaxSize != 2<<30 {
		t.Errorf("Cache.MaxSize = %d, attendu %d", cfg.Cache.MaxSize, int64(2<<30))
	}
	// Sans BOXINCLOUD_JWT_SECRET, la clé JWT est dérivée de la clé maître.
	if string(cfg.Auth.JWTSecret) != string(cfg.SecretKey) {
		t.Error("JWTSecret devrait se rabattre sur SecretKey quand il n'est pas défini")
	}
}

// TestLoadEd2kDisabledByDefault vérifie l'interrupteur le plus important du
// module : il est fermé tant que personne ne l'a ouvert.
//
// Une inversion de ce défaut ferait ouvrir des ports pair-à-pair à toutes les
// instances existantes lors d'une simple mise à jour, sans que personne ne
// l'ait demandé. C'est le genre de régression qu'un test doit rendre bruyante.
func TestLoadEd2kDisabledByDefault(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/boxincloud")
	t.Setenv("BOXINCLOUD_SECRET_KEY", strings.Repeat("ab", 32))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() : %v", err)
	}
	if cfg.Ed2k.Enabled {
		t.Error("le module eD2k doit être désactivé par défaut")
	}
}

// TestLoadEd2kRejectsAbsurdPollInterval : les bornes ne valent que module actif.
func TestLoadEd2kRejectsAbsurdPollInterval(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/boxincloud")
	t.Setenv("BOXINCLOUD_SECRET_KEY", strings.Repeat("ab", 32))
	t.Setenv("BOXINCLOUD_ED2K_ENABLED", "true")
	t.Setenv("BOXINCLOUD_ED2K_POLL_INTERVAL", "10ms")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() devrait refuser une cadence de scrutation de 10ms")
	}
	if !strings.Contains(err.Error(), "BOXINCLOUD_ED2K_POLL_INTERVAL") {
		t.Errorf("le message devrait nommer la variable fautive, obtenu :\n%v", err)
	}
}

// TestLoadIgnoresEd2kWhenDisabled est la moitié qui compte.
//
// Un contrôle posé trop haut empêcherait un serveur de bibliothèque de démarrer
// à cause d'une variable héritée d'un essai, pour un module éteint.
func TestLoadIgnoresEd2kWhenDisabled(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/boxincloud")
	t.Setenv("BOXINCLOUD_SECRET_KEY", strings.Repeat("ab", 32))
	t.Setenv("BOXINCLOUD_ED2K_ENABLED", "false")
	t.Setenv("BOXINCLOUD_ED2K_POLL_INTERVAL", "10ms")
	t.Setenv("BOXINCLOUD_ED2K_INCOMING_DIR", "")

	if _, err := Load(); err != nil {
		t.Fatalf("une configuration eD2k aberrante ne doit rien casser module éteint : %v", err)
	}
}
