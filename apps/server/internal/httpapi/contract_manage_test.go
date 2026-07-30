package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/comicfixture"
)

/*
Suppression et déplacement.

Deux opérations qui touchent au stockage ET au catalogue. L'enjeu n'est pas
qu'elles fonctionnent — c'est qu'elles ne détruisent rien quand elles échouent,
et que le parcours de bibliothèque ne défasse pas ce que l'utilisateur a décidé.
*/
func TestIntegrationManageComics(t *testing.T) {
	h := newContractHarness(t)
	lib := h.libraryID.String()
	archive := comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 3})

	// Le téléversement enfile l'indexation ; les workers étant à l'arrêt dans
	// ce harnais, on l'exécute ici pour que les pages soient servables.
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

	t.Run("déplacement", func(t *testing.T) {
		id := upload(t, "", "Blacksad.cbz")

		rec := h.expect(t, http.MethodPut, "/api/v1/comics/"+id+"/folder",
			map[string]any{"folder": "Polar/Blacksad"}, http.StatusOK)

		var moved struct {
			FolderPath string `json:"folderPath"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &moved); err != nil {
			t.Fatal(err)
		}
		if moved.FolderPath != "Polar/Blacksad" {
			t.Errorf("folderPath = %q", moved.FolderPath)
		}

		// L'album doit rester lisible : le catalogue pointe la nouvelle clé.
		h.expect(t, http.MethodGet, "/api/v1/comics/"+id+"/pages/0", nil, http.StatusOK)

		detail := h.expect(t, http.MethodGet, "/api/v1/comics/"+id, nil, http.StatusOK)
		var comic struct {
			FolderPath string `json:"folderPath"`
			FileName   string `json:"fileName"`
		}
		if err := json.Unmarshal(detail.Body.Bytes(), &comic); err != nil {
			t.Fatal(err)
		}
		if comic.FolderPath != "Polar/Blacksad" || comic.FileName != "Blacksad.cbz" {
			t.Errorf("album après déplacement : %+v", comic)
		}
	})

	t.Run("déplacement vers une destination occupée", func(t *testing.T) {
		first := upload(t, "Doublon", "Meme Nom.cbz")
		second := upload(t, "", "Meme Nom.cbz")

		h.expect(t, http.MethodPut, "/api/v1/comics/"+second+"/folder",
			map[string]any{"folder": "Doublon"}, http.StatusUnprocessableEntity)

		// Le refus ne doit rien avoir cassé : les deux albums restent lisibles.
		h.expect(t, http.MethodGet, "/api/v1/comics/"+first+"/pages/0", nil, http.StatusOK)
		h.expect(t, http.MethodGet, "/api/v1/comics/"+second+"/pages/0", nil, http.StatusOK)
	})

	/*
		Retiré du catalogue, le fichier reste — et le parcours ne le ramène pas.

		C'est le point délicat : le scan utilise `deleted_at` pour signaler les
		objets disparus, et le remet à NULL dès qu'il retrouve l'objet. Sans une
		marque distincte portant la décision de l'utilisateur, l'album
		réapparaîtrait au parcours suivant.
	*/
	t.Run("retrait du catalogue, fichier conservé", func(t *testing.T) {
		id := upload(t, "", "A Retirer.cbz")

		h.expect(t, http.MethodDelete, "/api/v1/comics/"+id, nil, http.StatusNoContent)
		h.expect(t, http.MethodGet, "/api/v1/comics/"+id, nil, http.StatusNotFound)

		// Un parcours complet ne doit pas le faire revenir.
		h.expect(t, http.MethodPost, "/api/v1/libraries/"+lib+"/scan", nil, http.StatusAccepted)
		h.expect(t, http.MethodGet, "/api/v1/comics/"+id, nil, http.StatusNotFound)
	})

	t.Run("suppression du fichier", func(t *testing.T) {
		id := upload(t, "", "A Supprimer.cbz")

		h.expect(t, http.MethodDelete, "/api/v1/comics/"+id+"?deleteFile=true",
			nil, http.StatusNoContent)
		h.expect(t, http.MethodGet, "/api/v1/comics/"+id, nil, http.StatusNotFound)
	})

	t.Run("lot : déplacement", func(t *testing.T) {
		a := upload(t, "", "Lot A.cbz")
		b := upload(t, "", "Lot B.cbz")

		rec := h.expect(t, http.MethodPost, "/api/v1/comics/manage", map[string]any{
			"action": "move",
			"ids":    []string{a, b},
			"folder": "Rangés",
		}, http.StatusOK)

		var payload struct {
			Affected int `json:"affected"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Affected != 2 {
			t.Errorf("affected = %d, attendu 2", payload.Affected)
		}
	})

	// Un identifiant inconnu est filtré avant écriture, pas compté.
	t.Run("lot : identifiant étranger ignoré", func(t *testing.T) {
		a := upload(t, "", "Lot C.cbz")

		rec := h.expect(t, http.MethodPost, "/api/v1/comics/manage", map[string]any{
			"action": "delete",
			"ids":    []string{a, "019fb42d-0000-7000-8000-000000000000"},
		}, http.StatusOK)

		var payload struct {
			Affected int `json:"affected"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Affected != 1 {
			t.Errorf("affected = %d, attendu 1", payload.Affected)
		}
	})

	t.Run("lot : action inconnue", func(t *testing.T) {
		h.expectRejected(t, http.MethodPost, "/api/v1/comics/manage", map[string]any{
			"action": "incinerer",
			"ids":    []string{},
		}, http.StatusUnprocessableEntity)
	})
}
