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

/*
Création d'une bibliothèque par l'API, comme le fait le formulaire.

Le test manquait, et son absence a laissé passer un défaut qui cassait la
PREMIÈRE action de tout nouvel utilisateur : le handler forçait le type à
« comics » quand l'énumération de la base dit « comic ». Toute création sans
type explicite répondait 500.

Rien ne l'attrapait parce que le harnais crée ses bibliothèques par le service,
sans passer par la route. La leçon vaut d'être écrite : un chemin que seul
l'utilisateur emprunte doit être testé par le chemin de l'utilisateur.
*/
func TestIntegrationContractCreateLibrary(t *testing.T) {
	h := newContractHarness(t)

	var backendID string

	t.Run("backend disponible", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/storage-backends", nil, http.StatusOK)

		var payload struct {
			Backends []struct {
				ID string `json:"id"`
			} `json:"backends"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Backends) == 0 {
			t.Fatal("aucun backend : le harnais aurait dû en créer un")
		}
		backendID = payload.Backends[0].ID
	})

	t.Run("sans type — le cas du formulaire", func(t *testing.T) {
		if backendID == "" {
			t.Skip("aucun backend")
		}

		rec := h.expect(t, http.MethodPost, "/api/v1/libraries", map[string]any{
			"name":       "Sans type",
			"backendId":  backendID,
			"rootPrefix": "sans-type/",
		}, http.StatusCreated)

		var lib struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &lib); err != nil {
			t.Fatal(err)
		}
		if lib.Kind != "comic" {
			t.Errorf("type = %q, attendu le défaut « comic »", lib.Kind)
		}
	})

	t.Run("avec un type valide", func(t *testing.T) {
		if backendID == "" {
			t.Skip("aucun backend")
		}

		h.expect(t, http.MethodPost, "/api/v1/libraries", map[string]any{
			"name":       "Mangas",
			"backendId":  backendID,
			"kind":       "manga",
			"rootPrefix": "manga/",
		}, http.StatusCreated)
	})

	/*
		Un type inconnu n'est pas testable ICI, et c'est une bonne nouvelle.

		Le contrat déclare désormais l'énumération, si bien que le harnais —
		qui valide chaque requête contre lui — refuse d'émettre l'appel. La
		valeur est donc arrêtée avant même de partir, chez tout client engendré
		depuis le contrat.

		Le garde du handler reste néanmoins en place : un client qui ignore le
		contrat peut envoyer n'importe quoi, et le serveur doit répondre 422
		plutôt que de laisser PostgreSQL produire une erreur interne. C'est
		`validLibraryKind` qui s'en charge, et sa raison d'être est là.
	*/
}

/*
Déplacement d'un album vers une autre bibliothèque.

Le déplacement dans un dossier existait ; celui-ci non. Ce qu'il change tient en
trois écritures qui doivent aller ensemble : la clé de l'objet, la bibliothèque
d'appartenance, et la série — détachée, parce qu'elle appartient à la
bibliothèque d'origine et que rien ne garantit son existence à l'arrivée.

Les deux bibliothèques partagent ici le même backend, donc la copie reste côté
serveur. Le chemin entre backends DISTINCTS — où les octets transitent par le
serveur — n'est pas couvert : il demanderait deux stockages dans le harnais.
C'est une réserve, pas un oubli.
*/
func TestIntegrationContractMoveBetweenLibraries(t *testing.T) {
	h := newContractHarness(t)

	var backendID, targetID string

	t.Run("seconde bibliothèque", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/storage-backends", nil, http.StatusOK)

		var backends struct {
			Backends []struct {
				ID string `json:"id"`
			} `json:"backends"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &backends); err != nil {
			t.Fatal(err)
		}
		backendID = backends.Backends[0].ID

		rec = h.expect(t, http.MethodPost, "/api/v1/libraries", map[string]any{
			"name":       "Archives",
			"backendId":  backendID,
			"rootPrefix": "archives/",
		}, http.StatusCreated)

		var lib struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &lib); err != nil {
			t.Fatal(err)
		}
		targetID = lib.ID
	})

	t.Run("déplacement", func(t *testing.T) {
		if targetID == "" {
			t.Skip("pas de seconde bibliothèque")
		}

		rec := h.expect(t, http.MethodPost, "/api/v1/comics/manage", map[string]any{
			"action":    "move",
			"ids":       []string{h.comicID.String()},
			"libraryId": targetID,
			"folder":    "",
		}, http.StatusOK)

		var result struct {
			Affected int `json:"affected"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Affected != 1 {
			t.Fatalf("déplacés = %d, attendu 1", result.Affected)
		}
	})

	t.Run("l'album a changé de bibliothèque", func(t *testing.T) {
		if targetID == "" {
			t.Skip("pas de seconde bibliothèque")
		}

		rec := h.expect(t, http.MethodGet, "/api/v1/comics/"+h.comicID.String(), nil, http.StatusOK)

		var comic struct {
			LibraryID string `json:"libraryId"`
			SeriesID  string `json:"seriesId"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &comic); err != nil {
			t.Fatal(err)
		}

		if comic.LibraryID != targetID {
			t.Errorf("bibliothèque = %s, attendu %s", comic.LibraryID, targetID)
		}
		// La série appartenait à la bibliothèque d'origine : la garder
		// produirait une série dont les tomes sont ailleurs.
		if comic.SeriesID != "" {
			t.Errorf("série = %s, attendu détachée", comic.SeriesID)
		}
	})

	t.Run("les compteurs des deux bibliothèques suivent", func(t *testing.T) {
		if targetID == "" {
			t.Skip("pas de seconde bibliothèque")
		}

		rec := h.expect(t, http.MethodGet, "/api/v1/libraries", nil, http.StatusOK)

		var payload struct {
			Libraries []struct {
				ID         string `json:"id"`
				ComicCount int    `json:"comicCount"`
			} `json:"libraries"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}

		for _, lib := range payload.Libraries {
			if lib.ID != targetID {
				continue
			}
			// Le compteur est une colonne stockée : sans rafraîchissement
			// explicite, il resterait à zéro jusqu'au prochain parcours.
			if lib.ComicCount != 1 {
				t.Errorf("albums dans la destination = %d, attendu 1", lib.ComicCount)
			}
		}
	})
}

/*
Un stockage injoignable se dit à la saisie, pas en 500.

Le formulaire annonce lui-même que « le stockage est joint avant d'être
enregistré ». Un échec de connexion est donc un RÉSULTAT attendu de cette
vérification, pas une panne du serveur.

Il répondait pourtant « une erreur inattendue est survenue », sans rien dire de
plus, à quelqu'un qui avait simplement écrit `https://` devant un service en
clair. Le serveur connaissait la cause et la jetait.

Ce test vérifie les deux moitiés du correctif : le code de statut, et la
présence du diagnostic brut — sans lui, l'utilisateur sait qu'il s'est trompé
mais pas où.
*/
func TestIntegrationContractUnreachableBackendIsAValidationError(t *testing.T) {
	h := newContractHarness(t)

	rec := h.expect(t, http.MethodPost, "/api/v1/storage-backends", map[string]any{
		"name": "MinIO éteint",
		"kind": "s3",
		"config": map[string]string{
			// Port fermé : la connexion échoue sans ambiguïté.
			"endpoint": "127.0.0.1:1",
			"bucket":   "comics",
		},
		"secrets": map[string]string{
			"access_key": "x", "secret_key": "y",
		},
	}, http.StatusUnprocessableEntity)

	var payload struct {
		Detail string            `json:"detail"`
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}

	if payload.Errors["endpoint"] != "unreachable" {
		t.Errorf("champ fautif = %+v, attendu endpoint: unreachable", payload.Errors)
	}

	// Le diagnostic doit dire ce qui s'est passé, pas répéter la règle.
	if payload.Detail == "" || payload.Detail == "One or more fields are invalid." {
		t.Errorf("diagnostic générique : %q", payload.Detail)
	}
}
