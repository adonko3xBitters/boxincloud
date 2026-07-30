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
Verrouillage des dossiers.

Un masquage qui ne tient qu'à l'affichage ne masque rien : il faut vérifier que
le contenu disparaît AUSSI de la recherche, de l'accès direct par identifiant, et
des compteurs — un total qui ne colle pas avec la somme de ses enfants est un
aveu.
*/
func TestIntegrationFolderLocks(t *testing.T) {
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

	secret := upload(t, "Privé", "Secret.cbz")
	upload(t, "Public", "Ouvert.cbz")

	// Relevés juste avant le masquage, dans le sous-test qui le pose : mesurer
	// plus tôt donnerait une base périmée dès qu'un album entre entre-temps.
	var rootBefore, privateBefore int

	// ─── Lecture seule ───────────────────────────────────────────────────────

	t.Run("lecture seule protège la branche", func(t *testing.T) {
		h.expect(t, http.MethodPut, "/api/v1/folders/lock", map[string]any{
			"libraryId": lib, "path": "Public", "readOnly": true,
		}, http.StatusOK)

		// Le dossier reste parfaitement visible.
		if tree(t)["Public"] != 1 {
			t.Error("un dossier en lecture seule ne doit rien masquer")
		}

		// Mais il refuse toute modification.
		h.expect(t, http.MethodPut, "/api/v1/folders/path", map[string]any{
			"libraryId": lib, "path": "Public", "newPath": "Renommé",
		}, http.StatusConflict)

		rec := h.upload(t, lib, "Public", "Intrus.cbz", archive.Data)
		if rec.Code != http.StatusConflict {
			t.Errorf("dépôt dans un dossier protégé = %d, attendu 409", rec.Code)
		}
	})

	t.Run("la protection s'hérite", func(t *testing.T) {
		rec := h.upload(t, lib, "Public/Sous-dossier", "Intrus.cbz", archive.Data)
		if rec.Code != http.StatusConflict {
			t.Errorf("dépôt sous un dossier protégé = %d, attendu 409", rec.Code)
		}
	})

	t.Run("déprotection", func(t *testing.T) {
		h.expect(t, http.MethodPut, "/api/v1/folders/lock", map[string]any{
			"libraryId": lib, "path": "Public", "readOnly": false,
		}, http.StatusOK)

		rec := h.upload(t, lib, "Public", "Autorisé.cbz", archive.Data)
		if rec.Code != http.StatusCreated {
			t.Errorf("dépôt après déprotection = %d, attendu 201", rec.Code)
		}
	})

	// ─── Code d'accès ────────────────────────────────────────────────────────

	t.Run("un code masque la branche", func(t *testing.T) {
		before := tree(t)
		rootBefore, privateBefore = before[""], before["Privé"]

		h.expect(t, http.MethodPut, "/api/v1/folders/lock", map[string]any{
			"libraryId": lib, "path": "Privé", "code": "2947",
		}, http.StatusOK)

		if _, visible := tree(t)["Privé"]; visible {
			t.Error("le dossier masqué apparaît encore dans l'arborescence")
		}
	})

	/*
		Le compteur de la racine ne doit pas trahir.

		Si la racine annonçait un total incluant ce qu'elle cache, la simple
		soustraction révélerait l'existence et le volume du dossier masqué.
	*/
	/*
		Le compteur de la racine doit PERDRE ce qui est masqué.

		S'il conservait le total, la simple comparaison avec la somme des
		branches visibles révélerait l'existence — et le volume — du dossier
		caché. Un compteur qui ne colle pas avec ses enfants est un aveu.
	*/
	t.Run("les compteurs ne trahissent pas", func(t *testing.T) {
		rootAfter := tree(t)[""]

		if want := rootBefore - privateBefore; rootAfter != want {
			t.Errorf("racine = %d après masquage, attendu %d (%d moins les %d masqués)",
				rootAfter, want, rootBefore, privateBefore)
		}
	})

	t.Run("le contenu masqué sort des listes", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/comics?limit=100", nil, http.StatusOK)
		var payload struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		for _, item := range payload.Items {
			if item.ID == secret {
				t.Error("l'album masqué apparaît dans la liste")
			}
		}
	})

	t.Run("le contenu masqué sort de la recherche", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/search?q=secret", nil, http.StatusOK)
		var payload struct {
			Comics []struct {
				ID string `json:"id"`
			} `json:"comics"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		for _, c := range payload.Comics {
			if c.ID == secret {
				t.Error("l'album masqué apparaît dans la recherche")
			}
		}
	})

	/*
		Connaître l'identifiant ne suffit pas.

		Sans ce contrôle, le code ne protégerait que l'affichage : n'importe qui
		ayant vu l'album avant le verrouillage — ou l'ayant deviné — y accéderait
		encore.
	*/
	t.Run("l'accès direct est refusé", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/comics/"+secret, nil, http.StatusNotFound)
		h.expect(t, http.MethodGet, "/api/v1/comics/"+secret+"/manifest", nil, http.StatusNotFound)
		h.expect(t, http.MethodGet, "/api/v1/comics/"+secret+"/pages/0", nil, http.StatusNotFound)
		h.expect(t, http.MethodGet, "/api/v1/comics/"+secret+"/cover", nil, http.StatusNotFound)
	})

	t.Run("code erroné", func(t *testing.T) {
		h.expect(t, http.MethodPost, "/api/v1/folders/unlock", map[string]any{
			"libraryId": lib, "path": "Privé", "code": "0000",
		}, http.StatusUnprocessableEntity)

		h.expect(t, http.MethodGet, "/api/v1/comics/"+secret, nil, http.StatusNotFound)
	})

	t.Run("déverrouillage", func(t *testing.T) {
		h.expect(t, http.MethodPost, "/api/v1/folders/unlock", map[string]any{
			"libraryId": lib, "path": "Privé", "code": "2947",
		}, http.StatusOK)

		if tree(t)["Privé"] != 1 {
			t.Error("le dossier déverrouillé n'apparaît pas")
		}
		h.expect(t, http.MethodGet, "/api/v1/comics/"+secret, nil, http.StatusOK)
		h.expect(t, http.MethodGet, "/api/v1/comics/"+secret+"/pages/0", nil, http.StatusOK)
	})

	t.Run("refermeture", func(t *testing.T) {
		path := url.QueryEscape("Privé")
		h.expect(t, http.MethodDelete,
			fmt.Sprintf("/api/v1/libraries/%s/folders/unlock?path=%s", lib, path),
			nil, http.StatusNoContent)

		h.expect(t, http.MethodGet, "/api/v1/comics/"+secret, nil, http.StatusNotFound)
	})

	/*
		Changer le code révoque les déverrouillages.

		Un accès obtenu avec l'ancien code ne doit pas survivre au nouveau —
		sinon changer le code ne servirait à rien contre celui à qui on veut
		justement le retirer.
	*/
	t.Run("changer le code révoque les accès", func(t *testing.T) {
		h.expect(t, http.MethodPost, "/api/v1/folders/unlock", map[string]any{
			"libraryId": lib, "path": "Privé", "code": "2947",
		}, http.StatusOK)
		h.expect(t, http.MethodGet, "/api/v1/comics/"+secret, nil, http.StatusOK)

		h.expect(t, http.MethodPut, "/api/v1/folders/lock", map[string]any{
			"libraryId": lib, "path": "Privé", "code": "1358",
		}, http.StatusOK)

		h.expect(t, http.MethodGet, "/api/v1/comics/"+secret, nil, http.StatusNotFound)
	})

	t.Run("retrait du code", func(t *testing.T) {
		h.expect(t, http.MethodPut, "/api/v1/folders/lock", map[string]any{
			"libraryId": lib, "path": "Privé", "code": "",
		}, http.StatusOK)

		h.expect(t, http.MethodGet, "/api/v1/comics/"+secret, nil, http.StatusOK)
		if tree(t)["Privé"] != 1 {
			t.Error("le dossier ne réapparaît pas après retrait du code")
		}
	})

	t.Run("code trop court", func(t *testing.T) {
		h.expect(t, http.MethodPut, "/api/v1/folders/lock", map[string]any{
			"libraryId": lib, "path": "Privé", "code": "12",
		}, http.StatusUnprocessableEntity)
	})
}
