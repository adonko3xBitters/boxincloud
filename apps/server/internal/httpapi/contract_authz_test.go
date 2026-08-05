package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

/*
Ce qui est réservé à l'administration l'est vraiment.

Ce fichier existe à cause d'un défaut de conception qu'il corrige. La règle
« cette route est réservée » n'était portée que par un appel à `requireAdmin()`
en tête de chaque gestionnaire — une convention, donc, et une convention ne
protège que ce dont son auteur s'est souvenu. Un middleware `RequireAdmin`
traînait par ailleurs sans être câblé nulle part, ce qui donnait deux mécanismes
pour une seule règle et invitait à croire l'un actif en écrivant l'autre.

Le middleware mort a été supprimé. La convention reste, mais elle n'en est plus
une : chaque route ci-dessous est appelée avec un compte ORDINAIRE et doit
répondre 403. Oublier un garde sur une route nouvelle ne coûte plus une
relecture attentive, cela casse un test.

La liste doit grandir avec les routes. C'est le prix, et il est bas comparé à
celui d'une route d'administration ouverte à tous.
*/

// adminRoute décrit une route que seul un administrateur doit pouvoir appeler.
type adminRoute struct {
	method string
	path   string
	body   any
}

func adminOnlyRoutes(h *contractHarness) []adminRoute {
	library := h.libraryID.String()

	// Un identifiant qui n'existe pas : le garde doit répondre AVANT toute
	// lecture en base. Un 403 est donc attendu même sur une cible absente, et
	// c'est ce qui évite qu'un 404 déguise l'absence de garde.
	absent := "00000000-0000-0000-0000-000000000001"

	return []adminRoute{
		{http.MethodGet, "/api/v1/cache", nil},
		{http.MethodDelete, "/api/v1/cache", nil},

		{http.MethodGet, "/api/v1/storage-backends", nil},
		{http.MethodPost, "/api/v1/storage-backends", map[string]any{
			"name": "essai", "kind": "local",
			"config": map[string]string{"root": "/tmp/essai"},
		}},
		{http.MethodPost, "/api/v1/storage-backends/" + absent + "/test", nil},
		{http.MethodPatch, "/api/v1/storage-backends/" + absent,
			map[string]any{"name": "essai"}},
		{http.MethodDelete, "/api/v1/storage-backends/" + absent, nil},
		{http.MethodPut, "/api/v1/storage-backends/" + absent + "/default", nil},

		{http.MethodPost, "/api/v1/libraries/" + library + "/scan", nil},
		{http.MethodGet, "/api/v1/libraries/" + library + "/scans", nil},

		{http.MethodGet, "/api/v1/accounts", nil},

		// Module eD2k/Kad — réservé en totalité.
		//
		// Il engage la bande passante, les ports et l'adresse IP de l'instance,
		// et la déclaration du démon porte un mot de passe. Les niveaux d'accès
		// plus fins arrivent à l'étape 6 ; d'ici là, la règle est simple, et ces
		// quatre lignes sont ce qui la tient.
		{http.MethodGet, "/api/v1/ed2k/status", nil},
		{http.MethodGet, "/api/v1/ed2k/daemon", nil},
		{http.MethodPut, "/api/v1/ed2k/daemon", map[string]any{
			"host": "amuled", "port": 4712, "password": "essai",
		}},
		{http.MethodDelete, "/api/v1/ed2k/daemon", nil},

		// Les routes de lecture. Elles exposent les adresses IP de pairs
		// tiers, ce qui suffit à les réserver — indépendamment du fait
		// qu'elles décrivent ce que l'instance télécharge.
		{http.MethodGet, "/api/v1/ed2k/snapshot", nil},
		{http.MethodGet, "/api/v1/ed2k/downloads", nil},
		{http.MethodGet, "/api/v1/ed2k/downloads/" + strings.Repeat("ab", 16) + "/sources", nil},
		{http.MethodGet, "/api/v1/ed2k/uploads", nil},
		{http.MethodGet, "/api/v1/ed2k/servers", nil},
		{http.MethodGet, "/api/v1/ed2k/shared", nil},
		{http.MethodGet, "/api/v1/ed2k/stats", nil},

		// Les commandes. Un compte ordinaire ne doit pas pouvoir mettre en
		// pause le téléchargement d'un autre, ni couper le serveur.
		{http.MethodPost, "/api/v1/ed2k/downloads/" + strings.Repeat("ab", 16) + "/action",
			map[string]any{"action": "pause"}},
		{http.MethodPut, "/api/v1/ed2k/downloads/" + strings.Repeat("ab", 16) + "/priority",
			map[string]any{"priority": "high"}},
		{http.MethodPost, "/api/v1/ed2k/links",
			map[string]any{"link": "ed2k://|file|x|1|" + strings.Repeat("AB", 16) + "|/"}},
		{http.MethodPost, "/api/v1/ed2k/servers/connect", nil},
		{http.MethodPost, "/api/v1/ed2k/servers/disconnect", nil},
		{http.MethodPost, "/api/v1/ed2k/kad", map[string]any{"running": false}},
		{http.MethodPost, "/api/v1/ed2k/shared/reload", nil},

		// La recherche. Elle engage le réseau au nom de l'instance, et ses
		// résultats décrivent ce que quelqu'un a cherché.
		{http.MethodPost, "/api/v1/ed2k/search",
			map[string]any{"query": "x", "network": "global"}},
		{http.MethodGet, "/api/v1/ed2k/search", nil},
		{http.MethodDelete, "/api/v1/ed2k/search", nil},
		{http.MethodPost, "/api/v1/ed2k/search/" + strings.Repeat("ab", 16) + "/download", nil},
	}
}

func TestIntegrationContractAdminOnly(t *testing.T) {
	h := newContractHarness(t)

	for _, route := range adminOnlyRoutes(h) {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			rec := h.callAs(t, h.userToken, route.method, route.path, route.body)

			if rec.Code != http.StatusForbidden {
				t.Errorf("statut %d, attendu 403 — un compte ordinaire atteint une "+
					"route d'administration\ncorps : %s", rec.Code, rec.Body.String())
			}
		})
	}
}

/*
TestIntegrationContractAdminStillReachable est le contrepoids indispensable.

Un garde posé trop haut refuserait tout le monde, administrateur compris, et le
test précédent passerait quand même — brillamment. Les deux ne valent
qu'ensemble.
*/
func TestIntegrationContractAdminStillReachable(t *testing.T) {
	h := newContractHarness(t)

	for _, route := range adminOnlyRoutes(h) {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			rec := h.callAs(t, h.token, route.method, route.path, route.body)

			if rec.Code == http.StatusForbidden {
				t.Errorf("403 pour un administrateur : le garde refuse tout le monde\n"+
					"corps : %s", rec.Body.String())
			}
		})
	}
}

/*
TestIntegrationContractSearchStaysOpen : chercher n'est pas administrer.

La contrepartie du test ci-dessus. Une route ouverte qui deviendrait réservée
par excès de zèle serait une régression silencieuse : personne ne signale qu'une
fonctionnalité a cessé d'exister pour les comptes ordinaires, ils croient
simplement qu'elle n'a jamais marché.
*/
func TestIntegrationContractSearchStaysOpen(t *testing.T) {
	h := newContractHarness(t)

	open := []string{
		"/api/v1/search?q=tintin",
		"/api/v1/libraries",
		"/api/v1/home",
	}

	for _, path := range open {
		t.Run(path, func(t *testing.T) {
			rec := h.callAs(t, h.userToken, http.MethodGet, path, nil)
			if rec.Code != http.StatusOK {
				t.Errorf("statut %d, attendu 200 : cette route est ouverte à tout "+
					"compte\n%s", rec.Code, rec.Body.String())
			}
		})
	}
}

/*
callAs exécute un appel sous une session donnée, sans valider le contrat.

La validation est délibérément court-circuitée : ces tests portent sur le
STATUT, et passer par `expect` obligerait à décrire des corps de requête
parfaitement valides pour des appels qui ne doivent jamais aboutir.
*/
func (h *contractHarness) callAs(
	t *testing.T, token, method, path string, body any,
) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}
