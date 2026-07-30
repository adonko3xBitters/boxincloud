package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/comicfixture"
)

/*
Le téléversement, de bout en bout.

C'est la seule voie par laquelle du contenu entre dans boxincloud sans passer
par un terminal. Elle écrit dans un vrai backend d'objets, et un défaut y laisse
des fichiers illisibles ou, pire, des fichiers écrits hors de la bibliothèque
visée. Elle est donc exercée contre un MinIO réel, pas contre une doublure.
*/

// uploadBody compose un corps multipart, `folder` avant `file` comme l'exige le
// contrat — le serveur lit les parties dans l'ordre, sans mise en tampon.
func uploadBody(t *testing.T, folder, filename string, content []byte) (io.Reader, string) {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("folder", folder); err != nil {
		t.Fatal(err)
	}

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	return &buf, writer.FormDataContentType()
}

// upload exécute un téléversement et retourne la réponse brute.
//
// Il ne passe pas par h.call : le contrat décrit un corps multipart, que le
// validateur de requêtes ne sait pas reconstruire. La RÉPONSE, elle, reste
// validée par les tests qui en ont besoin.
func (h *contractHarness) upload(
	t *testing.T, libraryID, folder, filename string, content []byte,
) *httptest.ResponseRecorder {
	t.Helper()

	body, contentType := uploadBody(t, folder, filename, content)

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/libraries/"+libraryID+"/upload", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+h.token)

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func TestIntegrationUpload(t *testing.T) {
	h := newContractHarness(t)
	lib := h.libraryID.String()

	archive := comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 3})

	t.Run("dépôt à la racine", func(t *testing.T) {
		rec := h.upload(t, lib, "", "Maus.cbz", archive.Data)
		if rec.Code != http.StatusCreated {
			t.Fatalf("statut %d, attendu 201 — %s", rec.Code, truncate(rec.Body.String(), 300))
		}

		var payload struct {
			ComicID   string `json:"comicId"`
			ObjectKey string `json:"objectKey"`
			Title     string `json:"title"`
			Format    string `json:"format"`
			FileSize  int64  `json:"fileSize"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}

		if payload.ObjectKey != "bd/Maus.cbz" {
			t.Errorf("objectKey = %q, attendu bd/Maus.cbz", payload.ObjectKey)
		}
		if payload.Format != "cbz" {
			t.Errorf("format = %q, attendu cbz", payload.Format)
		}
		if payload.FileSize != int64(len(archive.Data)) {
			t.Errorf("fileSize = %d, attendu %d", payload.FileSize, len(archive.Data))
		}

		// L'album doit être immédiatement consultable, même si son indexation
		// n'a pas encore tourné : sans cela, l'utilisateur téléverse et ne voit
		// rien apparaître.
		h.expect(t, http.MethodGet, "/api/v1/comics/"+payload.ComicID, nil, http.StatusOK)
	})

	t.Run("dépôt dans un dossier", func(t *testing.T) {
		rec := h.upload(t, lib, "Intégrales/Spiegelman", "Maus II.cbz", archive.Data)
		if rec.Code != http.StatusCreated {
			t.Fatalf("statut %d, attendu 201 — %s", rec.Code, truncate(rec.Body.String(), 300))
		}

		var payload struct {
			ObjectKey string `json:"objectKey"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.ObjectKey != "bd/Intégrales/Spiegelman/Maus II.cbz" {
			t.Errorf("objectKey = %q", payload.ObjectKey)
		}
	})

	// Le dossier apparaît dans l'arbre : c'est ce qui rend un envoi visible
	// dans la barre latérale plutôt que perdu à la racine.
	t.Run("le dossier entre dans l'arbre", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/folders", nil, http.StatusOK)

		var payload struct {
			Folders []struct {
				Path string `json:"path"`
			} `json:"folders"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}

		var found bool
		for _, f := range payload.Folders {
			if f.Path == "Intégrales/Spiegelman" {
				found = true
			}
		}
		if !found {
			t.Errorf("dossier absent de l'arbre : %+v", payload.Folders)
		}
	})

	// Écraser un objet existant rendrait fausse, sans prévenir, la progression
	// de lecture de tous ceux qui l'avaient commencé.
	t.Run("un doublon est refusé", func(t *testing.T) {
		rec := h.upload(t, lib, "", "Maus.cbz", archive.Data)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("statut %d, attendu 422 pour un doublon — %s",
				rec.Code, truncate(rec.Body.String(), 200))
		}
	})

	t.Run("extension refusée", func(t *testing.T) {
		rec := h.upload(t, lib, "", "notes.txt", []byte("bonjour"))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("statut %d, attendu 422 — %s", rec.Code, truncate(rec.Body.String(), 200))
		}
	})

	/*
		Renommer un exécutable en .cbz ne doit pas suffire.

		Sans cette vérification, n'importe quel compte pourrait déposer un
		binaire dans le bucket, où il resterait, servi ensuite par le serveur à
		tous les clients de la bibliothèque.
	*/
	t.Run("contenu démentant l'extension", func(t *testing.T) {
		machO := []byte{0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01, 0x00}
		rec := h.upload(t, lib, "", "cheval-de-troie.cbz", machO)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("statut %d, attendu 422 — %s", rec.Code, truncate(rec.Body.String(), 200))
		}
	})

	/*
		La clé ne doit jamais sortir du préfixe de la bibliothèque.

		Un dossier « ../../ » ferait écrire à côté — dans une autre bibliothèque
		du même bucket, ou à sa racine.
	*/
	t.Run("remontée de dossier neutralisée", func(t *testing.T) {
		rec := h.upload(t, lib, "../../evasion", "Persepolis II.cbz", archive.Data)
		if rec.Code != http.StatusCreated {
			t.Fatalf("statut %d — %s", rec.Code, truncate(rec.Body.String(), 300))
		}

		var payload struct {
			ObjectKey string `json:"objectKey"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if want := "bd/evasion/Persepolis II.cbz"; payload.ObjectKey != want {
			t.Errorf("objectKey = %q, attendu %q", payload.ObjectKey, want)
		}
	})

	t.Run("nom de fichier réduit à un chemin", func(t *testing.T) {
		rec := h.upload(t, lib, "", "/../../etc/Blacksad.cbz", archive.Data)
		if rec.Code != http.StatusCreated {
			t.Fatalf("statut %d — %s", rec.Code, truncate(rec.Body.String(), 300))
		}

		var payload struct {
			ObjectKey string `json:"objectKey"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if want := "bd/Blacksad.cbz"; payload.ObjectKey != want {
			t.Errorf("objectKey = %q, attendu %q", payload.ObjectKey, want)
		}
	})

	t.Run("bibliothèque inconnue", func(t *testing.T) {
		rec := h.upload(t, "019fb42d-0000-7000-8000-000000000000", "", "X.cbz", archive.Data)
		if rec.Code != http.StatusNotFound {
			t.Errorf("statut %d, attendu 404 — %s", rec.Code, truncate(rec.Body.String(), 200))
		}
	})
}

// L'administration passe par les mêmes routes que le reste : ses réponses
// doivent donc être conformes au contrat publié, comme toutes les autres.
func TestIntegrationContractAdmin(t *testing.T) {
	h := newContractHarness(t)

	t.Run("listeBackends", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/storage-backends", nil, http.StatusOK)
	})

	t.Run("scan", func(t *testing.T) {
		h.expect(t, http.MethodPost,
			fmt.Sprintf("/api/v1/libraries/%s/scan", h.libraryID), nil, http.StatusAccepted)
	})

	t.Run("bibliothèqueSansNom", func(t *testing.T) {
		h.expect(t, http.MethodPost, "/api/v1/libraries", map[string]any{
			"name":      "",
			"backendId": "019fb42d-0000-7000-8000-000000000000",
		}, http.StatusUnprocessableEntity)
	})
}
