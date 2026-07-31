package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

/*
Cache dérivé.

Tout y est reconstructible, ce qui rend la purge sans danger — mais pas sans
conséquence : elle doit rendre ce qu'elle a supprimé, sinon l'interface n'a rien
d'utile à afficher, et ce qu'elle efface doit se régénérer, sinon « sans danger »
n'est qu'une affirmation dans un commentaire.
*/
func TestIntegrationContractCache(t *testing.T) {
	h := newContractHarness(t)

	t.Run("statistiques", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/cache", nil, http.StatusOK)

		var payload struct {
			Entries  int64 `json:"entries"`
			Bytes    int64 `json:"bytes"`
			Hits     int64 `json:"hits"`
			MaxBytes int64 `json:"maxBytes"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Entries < 0 || payload.Bytes < 0 {
			t.Errorf("statistiques négatives : %+v", payload)
		}
	})

	t.Run("une lecture de page peuple le cache", func(t *testing.T) {
		h.expect(t, http.MethodGet,
			"/api/v1/comics/"+h.comicID.String()+"/pages/0?width=320", nil, http.StatusOK)

		rec := h.expect(t, http.MethodGet, "/api/v1/cache", nil, http.StatusOK)

		var payload struct {
			Entries int64 `json:"entries"`
			Bytes   int64 `json:"bytes"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Entries == 0 || payload.Bytes == 0 {
			t.Errorf("cache vide après une lecture de page : %+v", payload)
		}
	})

	t.Run("purge", func(t *testing.T) {
		h.expect(t, http.MethodDelete, "/api/v1/cache", nil, http.StatusOK)

		rec := h.expect(t, http.MethodGet, "/api/v1/cache", nil, http.StatusOK)

		var after struct {
			Entries int64 `json:"entries"`
			Bytes   int64 `json:"bytes"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &after); err != nil {
			t.Fatal(err)
		}
		if after.Entries != 0 || after.Bytes != 0 {
			t.Errorf("après purge : %+v, attendu un inventaire vide", after)
		}
	})

	t.Run("la page se resert après la purge", func(t *testing.T) {
		h.expect(t, http.MethodGet,
			"/api/v1/comics/"+h.comicID.String()+"/pages/0?width=320", nil, http.StatusOK)
	})
}

/*
Révocation d'un appareil.

Le geste après un téléphone perdu. Il doit couper cet appareil-là, et tout de
suite : un jeton d'accès est autoporteur, et sans vérification par appareil il
resterait valable un quart d'heure — que celui qui a ramassé le téléphone
emploierait à autre chose qu'à lire.
*/
func TestIntegrationContractDevices(t *testing.T) {
	h := newContractHarness(t)

	var deviceID string

	t.Run("liste", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/me/devices", nil, http.StatusOK)

		var payload struct {
			Devices []struct {
				ID      string `json:"id"`
				Current bool   `json:"current"`
			} `json:"devices"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Devices) == 0 {
			t.Fatal("aucun appareil : la connexion du harnais aurait dû en créer un")
		}
		deviceID = payload.Devices[0].ID
	})

	t.Run("appareil inconnu", func(t *testing.T) {
		h.expect(t, http.MethodDelete,
			"/api/v1/me/devices/00000000-0000-0000-0000-000000000001",
			nil, http.StatusNotFound)
	})

	t.Run("identifiant invalide", func(t *testing.T) {
		h.expect(t, http.MethodDelete, "/api/v1/me/devices/pas-un-uuid",
			nil, http.StatusUnprocessableEntity)
	})

	t.Run("révocation", func(t *testing.T) {
		if deviceID == "" {
			t.Skip("aucun appareil listé")
		}

		rec := h.expect(t, http.MethodDelete, "/api/v1/me/devices/"+deviceID,
			nil, http.StatusOK)

		var payload struct {
			RevokedSessions int64 `json:"revokedSessions"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.RevokedSessions < 1 {
			t.Errorf("sessions révoquées = %d, attendu au moins 1", payload.RevokedSessions)
		}
	})

	t.Run("le jeton de cet appareil ne vaut plus rien", func(t *testing.T) {
		if deviceID == "" {
			t.Skip("aucun appareil listé")
		}
		h.expect(t, http.MethodGet, "/api/v1/me", nil, http.StatusUnauthorized)
	})
}
