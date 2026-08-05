package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

/*
Le module eD2k/Kad, à l'étape 0.

Il n'y a pas encore de client External Connections : ce qui se teste ici est
donc la surface — les états, la déclaration du démon, et surtout ce qui ne doit
jamais sortir. Le reste viendra avec les étapes qui l'apportent.
*/

func decodeEd2kBody(t *testing.T, raw string) map[string]any {
	t.Helper()

	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("réponse illisible : %v\ncorps : %s", err, raw)
	}
	return out
}

// TestIntegrationContractEd2kStatusSansDemon : une instance fraîche a un état,
// pas une panne.
func TestIntegrationContractEd2kStatusSansDemon(t *testing.T) {
	h := newContractHarness(t)

	rec := h.expect(t, http.MethodGet, "/api/v1/ed2k/status", nil, http.StatusOK)
	body := decodeEd2kBody(t, rec.Body.String())

	if body["state"] != "unconfigured" {
		t.Errorf("state = %v, attendu unconfigured", body["state"])
	}
	if body["enabled"] != true {
		t.Errorf("enabled = %v, attendu true dans ce harnais", body["enabled"])
	}
	if detail, _ := body["detail"].(string); detail == "" {
		t.Error("un état doit dire pourquoi il est celui-là")
	}
	// Un montage manquant est la première cause de « le téléchargement est fini
	// mais rien n'arrive » : le répertoire doit être visible depuis l'interface.
	if dir, _ := body["incomingDir"].(string); dir == "" {
		t.Error("incomingDir absent de l'état")
	}
	if _, present := body["daemon"]; present {
		t.Error("aucun démon ne doit être décrit tant qu'aucun n'est déclaré")
	}
}

// TestIntegrationContractEd2kDaemonCycle parcourt la déclaration complète.
func TestIntegrationContractEd2kDaemonCycle(t *testing.T) {
	h := newContractHarness(t)

	h.expect(t, http.MethodGet, "/api/v1/ed2k/daemon", nil, http.StatusNotFound)

	h.expect(t, http.MethodPut, "/api/v1/ed2k/daemon", map[string]any{
		"host": "amuled", "port": 4712, "password": "mot de passe EC", "label": "démon du salon",
	}, http.StatusOK)

	rec := h.expect(t, http.MethodGet, "/api/v1/ed2k/daemon", nil, http.StatusOK)
	body := decodeEd2kBody(t, rec.Body.String())

	if body["host"] != "amuled" {
		t.Errorf("host = %v, attendu amuled", body["host"])
	}
	if body["port"] != float64(4712) {
		t.Errorf("port = %v, attendu 4712", body["port"])
	}

	// L'état global doit suivre : déclarer un démon fait passer le module de
	// « non configuré » à « déclaré, pas connecté ».
	status := decodeEd2kBody(t,
		h.expect(t, http.MethodGet, "/api/v1/ed2k/status", nil, http.StatusOK).Body.String())
	if status["state"] != "disconnected" {
		t.Errorf("state = %v, attendu disconnected une fois le démon déclaré", status["state"])
	}

	h.expect(t, http.MethodDelete, "/api/v1/ed2k/daemon", nil, http.StatusNoContent)
	h.expect(t, http.MethodGet, "/api/v1/ed2k/daemon", nil, http.StatusNotFound)

	// Oublier deux fois réussit : le résultat demandé est atteint dans les deux
	// cas, et une interface n'a pas à savoir si quelqu'un l'a devancée.
	h.expect(t, http.MethodDelete, "/api/v1/ed2k/daemon", nil, http.StatusNoContent)
}

/*
TestIntegrationContractEd2kNeRenvoieJamaisLeMotDePasse est le test qui compte le
plus de ce fichier.

Le mot de passe External Connections ouvre le pilotage complet du démon. Il
entre par cette route, il est scellé, et il ne doit ressortir par aucune des
trois portes qui existent : la réponse de la création, celle de la lecture, et
l'état global.
*/
func TestIntegrationContractEd2kNeRenvoieJamaisLeMotDePasse(t *testing.T) {
	h := newContractHarness(t)

	const password = "seCr3t-ec-tres-reconnaissable"

	created := h.expect(t, http.MethodPut, "/api/v1/ed2k/daemon", map[string]any{
		"host": "amuled", "port": 4712, "password": password,
	}, http.StatusOK)

	read := h.expect(t, http.MethodGet, "/api/v1/ed2k/daemon", nil, http.StatusOK)
	status := h.expect(t, http.MethodGet, "/api/v1/ed2k/status", nil, http.StatusOK)

	for name, rec := range map[string]string{
		"réponse de création": created.Body.String(),
		"lecture du démon":    read.Body.String(),
		"état du module":      status.Body.String(),
	} {
		if strings.Contains(rec, password) {
			t.Errorf("le mot de passe EC apparaît dans la %s :\n%s", name, rec)
		}
	}
}

// TestIntegrationContractEd2kRefuseUneSaisieImpossible : les erreurs désignent
// le champ fautif, pas le formulaire entier.
func TestIntegrationContractEd2kRefuseUneSaisieImpossible(t *testing.T) {
	h := newContractHarness(t)

	cases := []struct {
		nom   string
		body  map[string]any
		champ string
	}{
		{
			"service de métadonnées d'instance",
			map[string]any{"host": "169.254.169.254", "port": 4712, "password": "x"},
			"host",
		},
		{
			"mot de passe vide",
			map[string]any{"host": "amuled", "port": 4712, "password": ""},
			"password",
		},
	}

	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			// Le contrat interdit déjà ces corps (minLength) : on les envoie
			// sans validation d'entrée, pour vérifier que le SERVEUR les refuse
			// de lui-même. Le contrat ne protège que les clients qui le suivent.
			h.expectRejected(t, http.MethodPut, "/api/v1/ed2k/daemon", c.body,
				http.StatusUnprocessableEntity)
		})
	}

	// Le champ fautif doit être nommé : sur un formulaire à quatre champs dont
	// un port et un mot de passe, un message global fait chercher.
	rec := h.callWith(t, http.MethodPut, "/api/v1/ed2k/daemon",
		map[string]any{"host": "169.254.169.254", "port": 4712, "password": "x"}, false)

	body := decodeEd2kBody(t, rec.Body.String())
	errs, _ := body["errors"].(map[string]any)
	if _, named := errs["host"]; !named {
		t.Errorf("le champ host doit être désigné, obtenu : %v", body["errors"])
	}
}

/*
TestIntegrationContractEd2kEventsOuvreUnFlux vérifie le chemin complet du flux.

Trois choses s'y jouent, et aucune ne se voit ailleurs : que le jeton en
paramètre d'URL soit accepté — EventSource ne sait pas porter d'en-tête —, que
l'état complet parte AVANT tout changement, et que la réponse porte les en-têtes
sans lesquels le flux meurt derrière un reverse-proxy.
*/
func TestIntegrationContractEd2kEventsOuvreUnFlux(t *testing.T) {
	h := newContractHarness(t)

	// Le flux ne se termine pas de lui-même : c'est le client qui s'en va. Ici,
	// un contexte borné joue ce rôle.
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/ed2k/events?token="+h.token, nil).WithContext(ctx)

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("statut %d, attendu 200 — corps : %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Errorf("Content-Type = %q, attendu text/event-stream", got)
	}
	if got := rec.Header().Get("X-Accel-Buffering"); got != "no" {
		t.Errorf("X-Accel-Buffering = %q — sans lui le flux meurt derrière un proxy", got)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: status") {
		t.Errorf("le premier message doit porter l'état complet, obtenu :\n%s", body)
	}
	if !strings.Contains(body, "unconfigured") {
		t.Errorf("l'état initial doit être celui du module, obtenu :\n%s", body)
	}
}

// TestIntegrationContractEd2kEventsRefuseUnAnonyme : la route est ouverte au
// jeton d'URL, pas à l'absence de jeton.
func TestIntegrationContractEd2kEventsRefuseUnAnonyme(t *testing.T) {
	h := newContractHarness(t)

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/ed2k/events", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("statut %d, attendu 401 pour un flux sans jeton", rec.Code)
	}
}
