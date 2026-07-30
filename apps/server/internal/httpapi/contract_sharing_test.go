package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/comicfixture"
)

/*
Partage.

Deux mécanismes aux enjeux très différents. Le partage entre comptes RESTREINT —
le vérifier, c'est vérifier que les autres perdent l'accès. Le lien public OUVRE
sans compte : c'est la seule porte du serveur qui ne demande rien, et chacun de
ses garde-fous doit être exercé plutôt que décrit.
*/
func TestIntegrationSharing(t *testing.T) {
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

	shared := upload(t, "Partagé", "Prêté.cbz")
	upload(t, "Réservé", "Privé.cbz")

	// ─── Partage entre comptes ───────────────────────────────────────────────

	var readerID, readerToken string

	t.Run("préparation d'un second compte", func(t *testing.T) {
		rec := h.expect(t, http.MethodPost, "/api/v1/accounts", map[string]any{
			"username": "invite", "password": "un mot de passe solide",
		}, http.StatusCreated)

		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		readerID = created.ID

		tokens := h.expect(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
			"username": "invite", "password": "un mot de passe solide",
			"deviceName": "Test", "platform": "web",
		}, http.StatusOK)

		var issued struct {
			AccessToken string `json:"accessToken"`
		}
		if err := json.Unmarshal(tokens.Body.Bytes(), &issued); err != nil {
			t.Fatal(err)
		}
		readerToken = issued.AccessToken
	})

	/*
		Le premier accès accordé REFERME le dossier pour les autres.

		C'est le point que l'interface doit relayer et que ce test fixe : partager
		n'est pas seulement ouvrir à quelqu'un, c'est fermer à tous les autres.
	*/
	t.Run("partager referme pour les autres", func(t *testing.T) {
		admin := h.token
		h.token = readerToken

		// Avant : l'invité voit les deux dossiers.
		before := foldersOf(t, h)
		if _, ok := before["Réservé"]; !ok {
			t.Fatal("l'invité ne voit pas Réservé avant partage")
		}

		h.token = admin
		h.expect(t, http.MethodPost, "/api/v1/folders/access", map[string]any{
			"libraryId": lib, "path": "Réservé", "userId": h.adminID.String(),
		}, http.StatusOK)

		h.token = readerToken
		after := foldersOf(t, h)
		if _, ok := after["Réservé"]; ok {
			t.Error("le dossier reste visible d'un compte non autorisé")
		}
		if _, ok := after["Partagé"]; !ok {
			t.Error("un dossier sans autorisation explicite devrait rester visible")
		}
		h.token = admin
	})

	t.Run("accorder l'accès le rend de nouveau visible", func(t *testing.T) {
		admin := h.token
		h.expect(t, http.MethodPost, "/api/v1/folders/access", map[string]any{
			"libraryId": lib, "path": "Réservé", "userId": readerID,
		}, http.StatusOK)

		h.token = readerToken
		if _, ok := foldersOf(t, h)["Réservé"]; !ok {
			t.Error("le dossier reste invisible malgré l'accès accordé")
		}
		h.token = admin
	})

	t.Run("retrait de l'accès", func(t *testing.T) {
		admin := h.token
		path := url.QueryEscape("Réservé")
		h.expect(t, http.MethodDelete,
			fmt.Sprintf("/api/v1/libraries/%s/folders/access/%s?path=%s", lib, readerID, path),
			nil, http.StatusNoContent)

		h.token = readerToken
		if _, ok := foldersOf(t, h)["Réservé"]; ok {
			t.Error("le dossier reste visible après retrait de l'accès")
		}
		h.token = admin
	})

	// ─── Liens publics ───────────────────────────────────────────────────────

	var token string

	t.Run("création d'un lien de dossier", func(t *testing.T) {
		rec := h.expect(t, http.MethodPost, "/api/v1/share-links", map[string]any{
			"libraryId":  lib,
			"folderPath": "Partagé",
			"label":      "Pour Camille",
			"expiresAt":  time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		}, http.StatusCreated)

		var link struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &link); err != nil {
			t.Fatal(err)
		}
		if link.Token == "" {
			t.Fatal("le jeton devrait être retourné à la création")
		}
		token = link.Token
	})

	// Le jeton n'existe qu'une fois : seul son hachage est conservé.
	t.Run("le jeton n'est plus relisible", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/share-links", nil, http.StatusOK)

		var payload struct {
			Links []struct {
				Token string `json:"token"`
			} `json:"links"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		for _, l := range payload.Links {
			if l.Token != "" {
				t.Error("un jeton ressort de la liste : il devrait être irrécupérable")
			}
		}
	})

	/*
		Le lien fonctionne SANS aucun jeton d'authentification.

		C'est tout l'objet du mécanisme, et c'est aussi ce qui le rend risqué :
		le test le vérifie en retirant explicitement le jeton du harnais.
	*/
	t.Run("accès public sans compte", func(t *testing.T) {
		admin := h.token
		h.token = ""
		defer func() { h.token = admin }()

		rec := h.expect(t, http.MethodGet, "/api/v1/share/"+token, nil, http.StatusOK)

		var payload struct {
			Scope  string `json:"scope"`
			Comics []struct {
				ID string `json:"id"`
			} `json:"comics"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Scope != "folder" || len(payload.Comics) != 1 {
			t.Fatalf("contenu partagé = %+v", payload)
		}
		if payload.Comics[0].ID != shared {
			t.Errorf("album partagé = %s, attendu %s", payload.Comics[0].ID, shared)
		}

		h.expect(t, http.MethodGet,
			"/api/v1/share/"+token+"/comics/"+shared+"/manifest", nil, http.StatusOK)
		h.expect(t, http.MethodGet,
			"/api/v1/share/"+token+"/comics/"+shared+"/pages/0", nil, http.StatusOK)
		h.expect(t, http.MethodGet,
			"/api/v1/share/"+token+"/comics/"+shared+"/cover", nil, http.StatusOK)
	})

	/*
		Un lien ne donne accès qu'à sa portée.

		Sans cette vérification à chaque requête, connaître un lien vers un
		dossier suffirait à lire toute la bibliothèque en changeant l'identifiant
		dans l'URL.
	*/
	t.Run("la portée est étanche", func(t *testing.T) {
		admin := h.token
		h.token = ""
		defer func() { h.token = admin }()

		other := h.comicID.String()
		h.expect(t, http.MethodGet,
			"/api/v1/share/"+token+"/comics/"+other+"/manifest", nil, http.StatusNotFound)
		h.expect(t, http.MethodGet,
			"/api/v1/share/"+token+"/comics/"+other+"/pages/0", nil, http.StatusNotFound)
		h.expect(t, http.MethodGet,
			"/api/v1/share/"+token+"/comics/"+other+"/cover", nil, http.StatusNotFound)
	})

	t.Run("jeton inventé", func(t *testing.T) {
		admin := h.token
		h.token = ""
		defer func() { h.token = admin }()

		h.expect(t, http.MethodGet, "/api/v1/share/pas-un-vrai-jeton", nil, http.StatusNotFound)
	})

	t.Run("échéance obligatoire", func(t *testing.T) {
		h.expect(t, http.MethodPost, "/api/v1/share-links", map[string]any{
			"libraryId": lib, "folderPath": "Partagé",
			"expiresAt": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
		}, http.StatusUnprocessableEntity)
	})

	t.Run("échéance plafonnée", func(t *testing.T) {
		h.expect(t, http.MethodPost, "/api/v1/share-links", map[string]any{
			"libraryId": lib, "folderPath": "Partagé",
			"expiresAt": time.Now().AddDate(5, 0, 0).UTC().Format(time.RFC3339),
		}, http.StatusUnprocessableEntity)
	})

	/*
		Un lien public sur un dossier masqué est refusé.

		Les deux intentions se contredisent : publier ce qu'on vient de cacher
		annulerait le code d'accès sans le dire.
	*/
	t.Run("lien refusé sur un dossier masqué", func(t *testing.T) {
		h.expect(t, http.MethodPut, "/api/v1/folders/lock", map[string]any{
			"libraryId": lib, "path": "Réservé", "code": "2947",
		}, http.StatusOK)

		h.expect(t, http.MethodPost, "/api/v1/share-links", map[string]any{
			"libraryId": lib, "folderPath": "Réservé",
			"expiresAt": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		}, http.StatusUnprocessableEntity)
	})

	t.Run("révocation", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/share-links", nil, http.StatusOK)

		var payload struct {
			Links []struct {
				ID string `json:"id"`
			} `json:"links"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Links) == 0 {
			t.Fatal("aucun lien à révoquer")
		}

		h.expect(t, http.MethodDelete,
			"/api/v1/share-links/"+payload.Links[0].ID, nil, http.StatusNoContent)

		admin := h.token
		h.token = ""
		h.expect(t, http.MethodGet, "/api/v1/share/"+token, nil, http.StatusNotFound)
		h.token = admin
	})
}

// foldersOf retourne l'arborescence visible du compte courant.
func foldersOf(t *testing.T, h *contractHarness) map[string]int {
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
