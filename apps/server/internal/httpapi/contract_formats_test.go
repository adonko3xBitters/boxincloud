package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

/*
Négociation de format sur les images.

Le test unitaire du moteur prouve que `Negotiate` choisit bien ; celui-ci prouve
que le serveur s'en sert, et surtout qu'il le déclare. Les deux sont
nécessaires, parce que la partie la plus facile à oublier n'est pas le choix du
format : c'est le `Vary` qui empêche un cache de servir ce format à quelqu'un
qui ne l'a pas demandé.

Une bibliothèque derrière un proxy d'entreprise, sans cet en-tête, servirait
l'AVIF du premier lecteur au suivant — et le suivant peut être l'application
Android, que ce format laisse sans image.
*/
func TestIntegrationContractImageFormats(t *testing.T) {
	h := newContractHarness(t)

	// get émet une requête avec l'en-tête Accept voulu.
	//
	// Sans passer par `expect` : celui-ci construit ses propres requêtes, et
	// c'est justement l'en-tête qu'on veut maîtriser ici.
	get := func(t *testing.T, path, accept string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+h.token)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		h.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s (Accept: %q) → %d\n%s", path, accept, rec.Code, rec.Body.String())
		}
		return rec
	}

	page := "/api/v1/comics/" + h.comicID.String() + "/pages/0?width=320"
	cover := "/api/v1/comics/" + h.comicID.String() + "/cover?width=320"

	const chrome = "image/avif,image/webp,image/apng,image/svg+xml,image/*;q=0.8,*/*;q=0.5"

	t.Run("une page va au WebP, jamais à l'AVIF", func(t *testing.T) {
		rec := get(t, page, chrome)

		// Le client annonce pourtant l'AVIF. Le serveur ne le prend pas, parce
		// que quelqu'un attend cette page et que l'AVIF assez rapide pour
		// tenir dans cette attente est plus gros que le WebP.
		if got := rec.Header().Get("Content-Type"); got != "image/webp" {
			t.Errorf("Content-Type = %s, attendu image/webp", got)
		}
		if rec.Body.Len() == 0 {
			t.Fatal("corps vide")
		}
		// Signature RIFF/WEBP : le type MIME est une promesse, les octets sont
		// la preuve.
		body := rec.Body.Bytes()
		if len(body) < 12 || string(body[0:4]) != "RIFF" || string(body[8:12]) != "WEBP" {
			t.Errorf("les octets ne sont pas du WebP : % x", body[:min(16, len(body))])
		}
	})

	t.Run("une couverture va à l'AVIF", func(t *testing.T) {
		rec := get(t, cover, chrome)

		if got := rec.Header().Get("Content-Type"); got != "image/avif" {
			t.Errorf("Content-Type = %s, attendu image/avif", got)
		}
		// Boîte ftyp d'un fichier ISOBMFF, marque « avif ».
		body := rec.Body.Bytes()
		if len(body) < 12 || string(body[4:8]) != "ftyp" {
			t.Errorf("les octets ne sont pas un fichier ISOBMFF : % x", body[:min(16, len(body))])
		}
	})

	t.Run("un client sans opinion reçoit du JPEG", func(t *testing.T) {
		for _, accept := range []string{"", "*/*", "image/*"} {
			rec := get(t, cover, accept)
			if got := rec.Header().Get("Content-Type"); got != "image/jpeg" {
				t.Errorf("Accept %q → Content-Type %s, attendu image/jpeg", accept, got)
			}
		}
	})

	t.Run("Vary annonce que la réponse dépend de l'Accept", func(t *testing.T) {
		for _, path := range []string{page, cover} {
			rec := get(t, path, chrome)
			if !strings.Contains(rec.Header().Get("Vary"), "Accept") {
				t.Errorf("%s : Vary = %q, il doit mentionner Accept — sans quoi "+
					"un cache partagé sert un format à qui n'en veut pas",
					path, rec.Header().Get("Vary"))
			}
		}
	})

	t.Run("chaque format a son ETag", func(t *testing.T) {
		// Deux variantes qui partageraient un ETag se répondraient 304 l'une à
		// l'autre : le client garderait éternellement le premier format reçu.
		avif := get(t, cover, chrome).Header().Get("ETag")
		jpeg := get(t, cover, "image/jpeg").Header().Get("ETag")

		if avif == "" || jpeg == "" {
			t.Fatal("ETag absent")
		}
		if avif == jpeg {
			t.Errorf("l'AVIF et le JPEG partagent l'ETag %s", avif)
		}
	})

	t.Run("le 304 reste valable dans sa variante", func(t *testing.T) {
		first := get(t, cover, chrome)
		etag := first.Header().Get("ETag")

		req := httptest.NewRequest(http.MethodGet, cover, nil)
		req.Header.Set("Authorization", "Bearer "+h.token)
		req.Header.Set("Accept", chrome)
		req.Header.Set("If-None-Match", etag)

		rec := httptest.NewRecorder()
		h.router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotModified {
			t.Fatalf("attendu 304, reçu %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Vary"), "Accept") {
			t.Error("le 304 doit porter le Vary lui aussi : c'est une réponse " +
				"de cache, et c'est elle qu'il faut cadrer")
		}
	})
}
