package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

/*
Limitation de débit.

Elle protège la seule route publique qui vérifie un secret. Un défaut ne se voit
qu'au moment où quelqu'un essaie de forcer un mot de passe — c'est-à-dire jamais
pendant le développement, et une seule fois en production.
*/
func TestLimiterBurst(t *testing.T) {
	l := newLimiter(Limit{Burst: 3, Every: time.Second})
	now := time.Now()

	for i := 1; i <= 3; i++ {
		if ok, _ := l.allow("a", now); !ok {
			t.Fatalf("requête %d refusée, attendu acceptée", i)
		}
	}

	ok, retry := l.allow("a", now)
	if ok {
		t.Fatal("quatrième requête acceptée, attendu refusée")
	}
	if retry <= 0 || retry > time.Second {
		t.Errorf("délai avant réessai = %v, attendu entre 0 et 1 s", retry)
	}
}

func TestLimiterReconstitution(t *testing.T) {
	l := newLimiter(Limit{Burst: 2, Every: time.Second})
	now := time.Now()

	l.allow("a", now)
	l.allow("a", now)
	if ok, _ := l.allow("a", now); ok {
		t.Fatal("seau vidé mais requête acceptée")
	}

	// Une seconde plus tard, un jeton exactement — pas deux.
	if ok, _ := l.allow("a", now.Add(time.Second)); !ok {
		t.Error("jeton non reconstitué après une seconde")
	}
	if ok, _ := l.allow("a", now.Add(time.Second)); ok {
		t.Error("deux jetons reconstitués en une seconde, attendu un seul")
	}
}

func TestLimiterNeDepassePasLeBurst(t *testing.T) {
	// Une longue inactivité ne doit pas accumuler une réserve : sans plafond,
	// une adresse silencieuse depuis une heure pourrait tenter trois mille
	// connexions d'affilée.
	l := newLimiter(Limit{Burst: 2, Every: time.Second})
	now := time.Now()

	l.allow("a", now)
	later := now.Add(time.Hour)

	if ok, _ := l.allow("a", later); !ok {
		t.Fatal("première requête après attente refusée")
	}
	if ok, _ := l.allow("a", later); !ok {
		t.Fatal("deuxième requête après attente refusée")
	}
	if ok, _ := l.allow("a", later); ok {
		t.Error("troisième requête acceptée : le burst n'est pas plafonné")
	}
}

func TestLimiterCloisonneLesAdresses(t *testing.T) {
	l := newLimiter(Limit{Burst: 1, Every: time.Minute})
	now := time.Now()

	l.allow("a", now)
	if ok, _ := l.allow("b", now); !ok {
		t.Error("une adresse en bloque une autre")
	}
}

func TestLimiterBalayage(t *testing.T) {
	l := newLimiter(Limit{Burst: 1, Every: time.Second})
	now := time.Now()

	l.allow("a", now)
	if len(l.buckets) != 1 {
		t.Fatalf("seaux = %d, attendu 1", len(l.buckets))
	}

	l.sweep(now.Add(2*time.Hour), time.Hour)
	if len(l.buckets) != 0 {
		t.Errorf("seaux après balayage = %d, attendu 0", len(l.buckets))
	}
}

func TestRateLimitRepond429(t *testing.T) {
	handler := RateLimit(Limit{Burst: 1, Every: time.Minute})(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

	request := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		r.RemoteAddr = "203.0.113.7:54321"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		return rec
	}

	if got := request().Code; got != http.StatusOK {
		t.Fatalf("première requête = %d, attendu 200", got)
	}

	rec := request()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("deuxième requête = %d, attendu 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After absent : le client n'a aucun moyen de savoir quand réessayer")
	}
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		name       string
		remoteAddr string
		forwarded  string
		want       string
	}{
		{"direct", "203.0.113.7:54321", "", "203.0.113.7"},
		{"derrière un proxy", "10.0.0.1:1234", "203.0.113.7", "203.0.113.7"},
		{"chaîne de proxys", "10.0.0.1:1234", "203.0.113.7, 10.0.0.2", "203.0.113.7"},
		{"espaces", "10.0.0.1:1234", "  203.0.113.7  , 10.0.0.2", "203.0.113.7"},
		{"IPv6 direct", "[2001:db8::1]:443", "", "2001:db8::1"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = c.remoteAddr
			if c.forwarded != "" {
				r.Header.Set("X-Forwarded-For", c.forwarded)
			}

			if got := clientIP(r); got != c.want {
				t.Errorf("clientIP = %q, attendu %q", got, c.want)
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("sur une page", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Accept", "text/html,application/xhtml+xml")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)

		for header, want := range map[string]string{
			"X-Content-Type-Options": "nosniff",
			"X-Frame-Options":        "DENY",
			"Referrer-Policy":        "same-origin",
		} {
			if got := rec.Header().Get(header); got != want {
				t.Errorf("%s = %q, attendu %q", header, got, want)
			}
		}

		csp := rec.Header().Get("Content-Security-Policy")
		if csp == "" {
			t.Fatal("aucune politique de contenu sur un document")
		}
		for _, directive := range []string{
			"frame-ancestors 'none'", "object-src 'none'", "base-uri 'self'",
		} {
			if !contains(csp, directive) {
				t.Errorf("politique de contenu sans %q : %s", directive, csp)
			}
		}
	})

	t.Run("pas de politique sur une image", func(t *testing.T) {
		// Trois cents octets d'en-tête par vignette finissent par se voir sur
		// une grille qui en charge soixante.
		r := httptest.NewRequest(http.MethodGet, "/api/v1/comics/x/pages/0", nil)
		r.Header.Set("Accept", "image/webp,image/*")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)

		if rec.Header().Get("Content-Security-Policy") != "" {
			t.Error("politique de contenu posée sur une image")
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Error("nosniff manquant sur une image, alors que c'est là qu'il compte le plus")
		}
	})
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
