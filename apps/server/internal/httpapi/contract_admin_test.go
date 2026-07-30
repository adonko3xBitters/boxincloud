package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

/*
Administration des stockages et des bibliothèques.

Créer était possible ; modifier et supprimer ne l'étaient pas. Les deux
suppressions sont les opérations les plus lourdes de conséquences du produit —
elles emportent par cascade tout ce qui s'y rattache — et leurs refus comptent
donc autant que leurs succès.
*/
func TestIntegrationContractStorageAdmin(t *testing.T) {
	h := newContractHarness(t)

	var backendID string

	t.Run("relevé du stockage existant", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/storage-backends", nil, http.StatusOK)

		var payload struct {
			Backends []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"backends"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Backends) != 1 {
			t.Fatalf("stockages = %d, attendu 1", len(payload.Backends))
		}
		backendID = payload.Backends[0].ID
	})

	t.Run("renommage", func(t *testing.T) {
		rec := h.expect(t, http.MethodPatch, "/api/v1/storage-backends/"+backendID,
			map[string]any{"name": "minio-renommé"}, http.StatusOK)

		var payload struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Name != "minio-renommé" {
			t.Errorf("nom = %q après renommage", payload.Name)
		}
	})

	/*
		Les secrets survivent à une modification qui ne les mentionne pas.

		C'est la propriété qui rend l'écran utilisable : ils ne ressortent jamais
		de la base, donc personne ne peut les retaper. Si une modification de nom
		les effaçait, le stockage deviendrait injoignable au premier renommage.
	*/
	t.Run("les identifiants survivent au renommage", func(t *testing.T) {
		rec := h.expect(t, http.MethodPost, "/api/v1/storage-backends/"+backendID+"/test",
			nil, http.StatusOK)

		var payload struct {
			OK     bool   `json:"ok"`
			Detail string `json:"detail"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if !payload.OK {
			t.Errorf("le stockage ne répond plus après renommage : %s", payload.Detail)
		}
	})

	// Supprimer un stockage encore porteur emporterait ses bibliothèques par
	// cascade, et avec elles la progression de lecture de tout le monde.
	t.Run("suppression refusée tant qu'il porte", func(t *testing.T) {
		h.expect(t, http.MethodDelete, "/api/v1/storage-backends/"+backendID,
			nil, http.StatusConflict)
	})

	t.Run("stockage par défaut", func(t *testing.T) {
		h.expect(t, http.MethodPut, "/api/v1/storage-backends/"+backendID+"/default",
			nil, http.StatusNoContent)
	})
}

func TestIntegrationContractLibraryAdmin(t *testing.T) {
	h := newContractHarness(t)
	lib := h.libraryID.String()

	t.Run("renommage", func(t *testing.T) {
		rec := h.expect(t, http.MethodPatch, "/api/v1/libraries/"+lib,
			map[string]any{"name": "Ma collection"}, http.StatusOK)

		var payload struct {
			Name       string `json:"name"`
			RootPrefix string `json:"rootPrefix"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Name != "Ma collection" {
			t.Errorf("nom = %q", payload.Name)
		}
		// Le préfixe n'était pas dans la requête : il doit être intact.
		if payload.RootPrefix != "bd/" {
			t.Errorf("rootPrefix = %q, attendu bd/ — une modification partielle ne doit rien écraser", payload.RootPrefix)
		}
	})

	t.Run("historique des parcours", func(t *testing.T) {
		h.expect(t, http.MethodPost, "/api/v1/libraries/"+lib+"/scan", nil, http.StatusAccepted)

		rec := h.expect(t, http.MethodGet, "/api/v1/libraries/"+lib+"/scans", nil, http.StatusOK)

		var payload struct {
			Runs []struct {
				Status      string `json:"status"`
				ObjectsSeen int    `json:"objectsSeen"`
			} `json:"runs"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		// Le harnais a indexé en direct : au moins un parcours a eu lieu.
		if len(payload.Runs) == 0 {
			t.Error("aucun parcours dans l'historique")
		}
	})

	// La suppression emporte tout par cascade : elle est exercée en dernier.
	t.Run("suppression", func(t *testing.T) {
		h.expect(t, http.MethodDelete, "/api/v1/libraries/"+lib, nil, http.StatusNoContent)

		rec := h.expect(t, http.MethodGet, "/api/v1/libraries", nil, http.StatusOK)
		var payload struct {
			Libraries []struct {
				ID string `json:"id"`
			} `json:"libraries"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		for _, l := range payload.Libraries {
			if l.ID == lib {
				t.Error("la bibliothèque supprimée apparaît encore")
			}
		}

		// Ses albums ont disparu avec elle.
		h.expect(t, http.MethodGet, "/api/v1/comics/"+h.comicID.String(), nil, http.StatusNotFound)
	})
}
