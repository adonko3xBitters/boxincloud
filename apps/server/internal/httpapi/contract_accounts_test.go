package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

/*
Administration des comptes.

Les garde-fous comptent autant que les opérations : une instance dont le dernier
administrateur se rétrograde n'est plus administrable du tout, et rien dans
l'interface ne permet d'y revenir. Ils sont donc exercés ici, pas seulement
décrits.
*/
func TestIntegrationContractAccounts(t *testing.T) {
	h := newContractHarness(t)
	var createdID string

	t.Run("liste", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/accounts", nil, http.StatusOK)

		var payload struct {
			Accounts []struct {
				Username string `json:"username"`
				Role     string `json:"role"`
			} `json:"accounts"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		// Le harnais installe deux comptes : l'administrateur de
		// l'installation, et un compte ordinaire dont les tests d'autorisation
		// ont besoin pour prouver ce qui lui est refusé.
		roles := map[string]int{}
		for _, account := range payload.Accounts {
			roles[account.Role]++
		}
		if roles["admin"] != 1 {
			t.Errorf("comptes = %+v, attendu exactement un administrateur", payload.Accounts)
		}
		if len(payload.Accounts) != 2 {
			t.Errorf("comptes = %+v, attendu les deux comptes du harnais", payload.Accounts)
		}
	})

	t.Run("création", func(t *testing.T) {
		rec := h.expect(t, http.MethodPost, "/api/v1/accounts", map[string]any{
			"username": "lecteur",
			"password": "un mot de passe solide",
			"role":     "user",
		}, http.StatusCreated)

		var payload struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		createdID = payload.ID
		if payload.Role != "user" {
			t.Errorf("role = %q, attendu user", payload.Role)
		}
	})

	// Le contrat interdit déjà un mot de passe de moins de douze caractères ;
	// la requête est donc envoyée sans validation, pour vérifier que le serveur
	// le refuse lui aussi. Un client qui ignore le contrat doit se heurter au
	// serveur, pas le traverser.
	t.Run("motDePasseTropCourt", func(t *testing.T) {
		h.expectRejected(t, http.MethodPost, "/api/v1/accounts", map[string]any{
			"username": "fragile",
			"password": "court",
		}, http.StatusUnprocessableEntity)
	})

	t.Run("identifiantDéjàPris", func(t *testing.T) {
		h.expect(t, http.MethodPost, "/api/v1/accounts", map[string]any{
			"username": "lecteur",
			"password": "un autre mot de passe",
		}, http.StatusUnprocessableEntity)
	})

	t.Run("modification", func(t *testing.T) {
		if createdID == "" {
			t.Skip("compte non créé")
		}
		rec := h.expect(t, http.MethodPatch, "/api/v1/accounts/"+createdID, map[string]any{
			"displayName": "Un lecteur",
			"restricted":  true,
		}, http.StatusOK)

		var payload struct {
			DisplayName string `json:"displayName"`
			Restricted  bool   `json:"restricted"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.DisplayName != "Un lecteur" || !payload.Restricted {
			t.Errorf("compte = %+v", payload)
		}
	})

	// Le seul administrateur ne peut pas se rétrograder : plus personne ne
	// pourrait alors rendre le rôle à quiconque.
	t.Run("dernierAdministrateurProtégé", func(t *testing.T) {
		me := h.expect(t, http.MethodGet, "/api/v1/me", nil, http.StatusOK)

		var user struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(me.Body.Bytes(), &user); err != nil {
			t.Fatal(err)
		}

		h.expect(t, http.MethodPatch, "/api/v1/accounts/"+user.ID,
			map[string]any{"role": "user"}, http.StatusUnprocessableEntity)

		h.expect(t, http.MethodDelete, "/api/v1/accounts/"+user.ID,
			nil, http.StatusUnprocessableEntity)
	})

	t.Run("accèsBibliothèque", func(t *testing.T) {
		if createdID == "" {
			t.Skip("compte non créé")
		}
		library := "/api/v1/libraries/" + h.libraryID.String() + "/access"

		h.expect(t, http.MethodPost, library,
			map[string]any{"userId": createdID, "canWrite": true}, http.StatusOK)

		rec := h.expect(t, http.MethodGet, library, nil, http.StatusOK)

		var payload struct {
			Grants []struct {
				UserID   string `json:"userId"`
				CanWrite bool   `json:"canWrite"`
			} `json:"grants"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Grants) != 1 || !payload.Grants[0].CanWrite {
			t.Errorf("accès = %+v", payload.Grants)
		}

		h.expect(t, http.MethodGet, "/api/v1/accounts/"+createdID+"/library-access",
			nil, http.StatusOK)

		h.expect(t, http.MethodDelete, library+"/"+createdID, nil, http.StatusNoContent)
	})

	t.Run("désactivation", func(t *testing.T) {
		if createdID == "" {
			t.Skip("compte non créé")
		}
		h.expect(t, http.MethodDelete, "/api/v1/accounts/"+createdID, nil, http.StatusNoContent)

		rec := h.expect(t, http.MethodGet, "/api/v1/accounts", nil, http.StatusOK)

		var payload struct {
			Accounts []struct {
				ID string `json:"id"`
			} `json:"accounts"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		for _, a := range payload.Accounts {
			if a.ID == createdID {
				t.Error("le compte désactivé apparaît encore dans la liste")
			}
		}
	})
}

/*
Un compte désactivé perd l'accès tout de suite.

Le jeton d'accès est autoporteur : sans vérification de l'état du compte, il
resterait valable jusqu'à son expiration — un quart d'heure pendant lequel un
compte que l'administrateur croit fermé continue de lire.
*/
func TestIntegrationDisabledAccountLosesAccess(t *testing.T) {
	h := newContractHarness(t)

	rec := h.expect(t, http.MethodPost, "/api/v1/accounts", map[string]any{
		"username": "ephemere",
		"password": "un mot de passe solide",
	}, http.StatusCreated)

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	// Connexion du nouveau compte, puis usage de SON jeton.
	tokens := h.expect(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": "ephemere", "password": "un mot de passe solide",
		"deviceName": "Test", "platform": "web",
	}, http.StatusOK)

	var issued struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(tokens.Body.Bytes(), &issued); err != nil {
		t.Fatal(err)
	}

	admin := h.token
	h.token = issued.AccessToken
	h.expect(t, http.MethodGet, "/api/v1/me", nil, http.StatusOK)

	// Désactivation par l'administrateur.
	h.token = admin
	h.expect(t, http.MethodDelete, "/api/v1/accounts/"+created.ID, nil, http.StatusNoContent)

	// Le jeton n'a pas expiré, mais le compte n'existe plus.
	h.token = issued.AccessToken
	h.expect(t, http.MethodGet, "/api/v1/me", nil, http.StatusUnauthorized)
	h.token = admin
}

/*
Une rétrogradation prend effet sans attendre l'expiration du jeton.

Le rôle voyage dans le jeton ; s'il n'était pas relu, un administrateur
rétrogradé garderait ses pouvoirs un quart d'heure — précisément le temps où
l'on veut qu'il ne les ait plus.
*/
func TestIntegrationDemotionTakesEffectImmediately(t *testing.T) {
	h := newContractHarness(t)

	rec := h.expect(t, http.MethodPost, "/api/v1/accounts", map[string]any{
		"username": "second",
		"password": "un mot de passe solide",
		"role":     "admin",
	}, http.StatusCreated)

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	tokens := h.expect(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": "second", "password": "un mot de passe solide",
		"deviceName": "Test", "platform": "web",
	}, http.StatusOK)

	var issued struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(tokens.Body.Bytes(), &issued); err != nil {
		t.Fatal(err)
	}

	admin := h.token

	// Le second administrateur peut administrer.
	h.token = issued.AccessToken
	h.expect(t, http.MethodGet, "/api/v1/accounts", nil, http.StatusOK)

	// Rétrogradé par le premier.
	h.token = admin
	h.expect(t, http.MethodPatch, "/api/v1/accounts/"+created.ID,
		map[string]any{"role": "user"}, http.StatusOK)

	// Son jeton porte encore « admin », mais ce n'est plus le rôle qui compte.
	h.token = issued.AccessToken
	h.expect(t, http.MethodGet, "/api/v1/accounts", nil, http.StatusForbidden)
	h.token = admin
}
