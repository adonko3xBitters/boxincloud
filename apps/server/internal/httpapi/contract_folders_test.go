package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/comicfixture"
)

/*
L'arborescence, de bout en bout.

Elle n'est plus déduite mais persistée, ce qui ouvre deux risques nouveaux : que
la base et le stockage divergent, et qu'un dossier créé à la main disparaisse au
premier parcours. Les deux sont exercés ici.
*/
func TestIntegrationFolders(t *testing.T) {
	h := newContractHarness(t)
	lib := h.libraryID.String()
	archive := comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 3})

	upload := func(t *testing.T, folder, name string) string {
		t.Helper()
		rec := h.upload(t, lib, folder, name, archive.Data)
		if rec.Code != http.StatusCreated {
			t.Fatalf("téléversement : %d — %s", rec.Code, truncate(rec.Body.String(), 200))
		}
		var payload struct {
			ComicID string `json:"comicId"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		id, err := uuid.Parse(payload.ComicID)
		if err != nil {
			t.Fatal(err)
		}
		if err := h.indexNow(context.Background(), id); err != nil {
			t.Fatalf("indexation : %v", err)
		}
		return payload.ComicID
	}

	tree := func(t *testing.T) map[string]int {
		t.Helper()
		rec := h.expect(t, http.MethodGet, "/api/v1/folders", nil, http.StatusOK)

		var payload struct {
			Folders []struct {
				Path       string `json:"path"`
				ComicCount int    `json:"comicCount"`
			} `json:"folders"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}

		out := make(map[string]int, len(payload.Folders))
		for _, f := range payload.Folders {
			out[f.Path] = f.ComicCount
		}
		return out
	}

	t.Run("création d'un dossier vide", func(t *testing.T) {
		h.expect(t, http.MethodPost, "/api/v1/folders", map[string]any{
			"libraryId": lib,
			"path":      "BD/Franco-belge/Tintin",
		}, http.StatusCreated)

		got := tree(t)
		for _, path := range []string{"BD", "BD/Franco-belge", "BD/Franco-belge/Tintin"} {
			if _, ok := got[path]; !ok {
				t.Errorf("dossier %q absent — les ancêtres devraient être créés", path)
			}
		}
	})

	/*
		Un dossier créé à la main survit à un parcours.

		C'est ce qui distingue un dossier voulu d'un dossier constaté : le second
		n'existe que par la présence de fichiers, le premier par une décision.
		Sans cette distinction, créer un dossier à l'avance pour y ranger ensuite
		serait impossible.
	*/
	t.Run("un dossier vide survit au parcours", func(t *testing.T) {
		h.expect(t, http.MethodPost, "/api/v1/libraries/"+lib+"/scan", nil, http.StatusAccepted)

		if _, ok := tree(t)["BD/Franco-belge/Tintin"]; !ok {
			t.Error("le dossier créé à la main a disparu au parcours")
		}
	})

	t.Run("les compteurs cumulent la branche", func(t *testing.T) {
		upload(t, "Manga/Naruto", "Naruto T01.cbz")
		upload(t, "Manga/Naruto", "Naruto T02.cbz")
		upload(t, "Manga", "One Shot.cbz")

		got := tree(t)
		if got["Manga/Naruto"] != 2 {
			t.Errorf("Manga/Naruto = %d, attendu 2", got["Manga/Naruto"])
		}
		if got["Manga"] != 3 {
			t.Errorf("Manga = %d, attendu 3 (cumul de la branche)", got["Manga"])
		}
	})

	t.Run("renommage d'une branche", func(t *testing.T) {
		id := upload(t, "Ancien", "Album.cbz")

		h.expect(t, http.MethodPut, "/api/v1/folders/path", map[string]any{
			"libraryId": lib,
			"path":      "Ancien",
			"newPath":   "Nouveau",
		}, http.StatusOK)

		got := tree(t)
		if _, ok := got["Ancien"]; ok {
			t.Error("l'ancien dossier subsiste")
		}
		if got["Nouveau"] != 1 {
			t.Errorf("Nouveau = %d, attendu 1", got["Nouveau"])
		}

		// L'album doit rester lisible : le catalogue pointe la nouvelle clé.
		h.expect(t, http.MethodGet, "/api/v1/comics/"+id+"/pages/0", nil, http.StatusOK)

		detail := h.expect(t, http.MethodGet, "/api/v1/comics/"+id, nil, http.StatusOK)
		var comic struct {
			FolderPath string `json:"folderPath"`
		}
		if err := json.Unmarshal(detail.Body.Bytes(), &comic); err != nil {
			t.Fatal(err)
		}
		if comic.FolderPath != "Nouveau" {
			t.Errorf("folderPath = %q, attendu Nouveau", comic.FolderPath)
		}
	})

	t.Run("déplacement sous un autre parent", func(t *testing.T) {
		id := upload(t, "Orphelin", "Perdu.cbz")

		h.expect(t, http.MethodPut, "/api/v1/folders/path", map[string]any{
			"libraryId": lib,
			"path":      "Orphelin",
			"newPath":   "BD/Orphelin",
		}, http.StatusOK)

		if tree(t)["BD/Orphelin"] != 1 {
			t.Error("le dossier n'a pas été rattaché à BD")
		}
		h.expect(t, http.MethodGet, "/api/v1/comics/"+id+"/pages/0", nil, http.StatusOK)
	})

	// Déplacer un dossier dans sa propre descendance n'a pas de sens
	// représentable, et détruirait la branche en cours de route.
	t.Run("déplacement dans sa descendance refusé", func(t *testing.T) {
		h.expect(t, http.MethodPut, "/api/v1/folders/path", map[string]any{
			"libraryId": lib,
			"path":      "Manga",
			"newPath":   "Manga/Naruto/Manga",
		}, http.StatusUnprocessableEntity)
	})

	t.Run("la racine est protégée", func(t *testing.T) {
		h.expect(t, http.MethodPut, "/api/v1/folders/path", map[string]any{
			"libraryId": lib, "path": "", "newPath": "Ailleurs",
		}, http.StatusUnprocessableEntity)

		h.expect(t, http.MethodDelete,
			fmt.Sprintf("/api/v1/libraries/%s/folders?path=", lib), nil, http.StatusUnprocessableEntity)
	})

	// Supprimer une branche pleine sans le dire est refusé : le geste est trop
	// lourd de conséquences pour être distrait.
	t.Run("suppression d'un dossier plein refusée", func(t *testing.T) {
		path := url.QueryEscape("Manga/Naruto")
		h.expect(t, http.MethodDelete,
			fmt.Sprintf("/api/v1/libraries/%s/folders?path=%s", lib, path),
			nil, http.StatusConflict)

		if tree(t)["Manga/Naruto"] != 2 {
			t.Error("le refus n'a pas laissé le dossier intact")
		}
	})

	t.Run("suppression confirmée", func(t *testing.T) {
		path := url.QueryEscape("Manga/Naruto")
		rec := h.expect(t, http.MethodDelete,
			fmt.Sprintf("/api/v1/libraries/%s/folders?path=%s&deleteComics=true", lib, path),
			nil, http.StatusOK)

		var payload struct {
			RemovedComics int `json:"removedComics"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.RemovedComics != 2 {
			t.Errorf("removedComics = %d, attendu 2", payload.RemovedComics)
		}

		got := tree(t)
		if _, ok := got["Manga/Naruto"]; ok {
			t.Error("le dossier subsiste après suppression")
		}
		if got["Manga"] != 1 {
			t.Errorf("Manga = %d, attendu 1 après retrait de la sous-branche", got["Manga"])
		}
	})

	t.Run("dossier inexistant", func(t *testing.T) {
		h.expect(t, http.MethodPut, "/api/v1/folders/path", map[string]any{
			"libraryId": lib, "path": "Inexistant", "newPath": "Ailleurs",
		}, http.StatusNotFound)
	})

	t.Run("destination occupée", func(t *testing.T) {
		h.expect(t, http.MethodPost, "/api/v1/folders",
			map[string]any{"libraryId": lib, "path": "Occupe"}, http.StatusCreated)
		h.expect(t, http.MethodPost, "/api/v1/folders",
			map[string]any{"libraryId": lib, "path": "Autre"}, http.StatusCreated)

		h.expect(t, http.MethodPut, "/api/v1/folders/path", map[string]any{
			"libraryId": lib, "path": "Autre", "newPath": "Occupe",
		}, http.StatusUnprocessableEntity)
	})
}
