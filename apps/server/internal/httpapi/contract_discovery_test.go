package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/comicfixture"
)

/*
Recherche fédérée, de bout en bout.

Un vrai catalogue OPDS est monté pour la durée du test, et l'instance
l'interroge à travers ses vraies routes. Ce que ce fichier vérifie n'est pas
l'analyse d'un flux — les tests du paquet `discovery` s'en chargent — mais ce
que le contrat promet : qui a le droit de déclarer un catalogue, ce qui ne
ressort jamais, et ce que rend une recherche quand la moitié des catalogues est
à terre.
*/

// opdsCatalogue monte un catalogue OPDS 1.2 minimal mais complet.
func opdsCatalogue(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/opds", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml;profile=opds-catalog")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Bibliothèque d'à côté</title>
  <link rel="search" type="application/atom+xml;profile=opds-catalog"
        href="/opds/search?q={searchTerms}"/>
</feed>`))
	})

	// Un vrai CBZ : l'ingestion vérifie la signature du contenu avant
	// d'écrire, un fichier bidon serait refusé — et le test ne prouverait
	// alors que le refus.
	mux.HandleFunc("/dl/garage.cbz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.comicbook+zip")
		_, _ = w.Write(importableCBZ)
	})

	// Le même contenu, mais servi sans extension dans l'adresse ni nom
	// déclaré : c'est ce que font Komga et Kavita.
	mux.HandleFunc("/api/v1/books/42/file", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.comicbook+zip")
		_, _ = w.Write(importableCBZ)
	})

	mux.HandleFunc("/opds/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml;profile=opds-catalog")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:dc="http://purl.org/dc/terms/">
  <entry>
    <title>Le Garage hermétique</title>
    <author><name>Mœbius</name></author>
    <dc:language>fr</dc:language>
    <link rel="http://opds-spec.org/image" href="/img/garage.jpg" type="image/jpeg"/>
    <link rel="http://opds-spec.org/acquisition"
          href="/dl/garage.cbz" type="application/vnd.comicbook+zip"/>
  </entry>
</feed>`))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// importableCBZ est rempli par le premier test qui en a besoin.
var importableCBZ []byte

func TestIntegrationContractDiscovery(t *testing.T) {
	h := newContractHarness(t)
	catalogue := opdsCatalogue(t)

	var sourceID string

	t.Run("déclarer un catalogue le joint d'abord", func(t *testing.T) {
		rec := h.expect(t, http.MethodPost, "/api/v1/discovery/sources", map[string]any{
			"name": "Bibliothèque d'à côté",
			"url":  catalogue.URL + "/opds",
		}, http.StatusCreated)

		var created struct {
			ID       string `json:"id"`
			Kind     string `json:"kind"`
			Enabled  bool   `json:"enabled"`
			Password string `json:"password"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		if created.Kind != "opds" || !created.Enabled {
			t.Errorf("catalogue mal enregistré : %+v", created)
		}
		if created.Password != "" {
			t.Error("un mot de passe ne doit jamais ressortir")
		}
		sourceID = created.ID
	})

	t.Run("une adresse qui ne répond pas est refusée à la saisie", func(t *testing.T) {
		// Le port est fermé : le catalogue ne peut pas être joint. L'erreur
		// doit se voir maintenant, pas à la première recherche où elle
		// passerait pour « aucun résultat ».
		h.expect(t, http.MethodPost, "/api/v1/discovery/sources", map[string]any{
			"name": "Éteint",
			"url":  "http://127.0.0.1:1/opds",
		}, http.StatusUnprocessableEntity)
	})

	t.Run("une adresse de métadonnées d'instance est refusée", func(t *testing.T) {
		// L'URL est jointe par le SERVEUR : c'est une SSRF, et 169.254.169.254
		// est l'adresse où les fournisseurs de nuage délivrent des
		// identifiants d'instance.
		h.expect(t, http.MethodPost, "/api/v1/discovery/sources", map[string]any{
			"name": "Curieux",
			"url":  "http://169.254.169.254/latest/meta-data/",
		}, http.StatusUnprocessableEntity)
	})

	t.Run("la recherche agrège et rapporte l'état de chaque catalogue", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet,
			"/api/v1/discovery/search?q=garage", nil, http.StatusOK)

		var payload struct {
			Results []struct {
				Title        string `json:"title"`
				SourceName   string `json:"sourceName"`
				CoverURL     string `json:"coverUrl"`
				InLibrary    bool   `json:"inLibrary"`
				Acquisitions []struct {
					Href string `json:"href"`
				} `json:"acquisitions"`
			} `json:"results"`
			Sources []struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
				Error string `json:"error"`
			} `json:"sources"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}

		if len(payload.Results) != 1 {
			t.Fatalf("%d résultats, attendu 1 : %s", len(payload.Results), rec.Body.String())
		}
		got := payload.Results[0]
		if got.Title != "Le Garage hermétique" {
			t.Errorf("titre = %q", got.Title)
		}
		if got.SourceName == "" {
			t.Error("la provenance doit accompagner chaque résultat")
		}
		// Les adresses du flux distant sont relatives ; rendues telles quelles,
		// le navigateur les résoudrait contre boxincloud et n'afficherait rien.
		if got.CoverURL == "" || got.CoverURL[:4] != "http" {
			t.Errorf("couverture non résolue : %q", got.CoverURL)
		}
		if len(got.Acquisitions) != 1 {
			t.Errorf("liens d'acquisition perdus : %+v", got.Acquisitions)
		}

		if len(payload.Sources) != 1 || payload.Sources[0].Error != "" {
			t.Errorf("état des catalogues : %+v", payload.Sources)
		}
	})

	t.Run("un catalogue à terre n'emporte pas les autres", func(t *testing.T) {
		// Enregistré directement en base : le service refuse d'enregistrer un
		// catalogue injoignable, ce qui est le bon comportement. On simule donc
		// celui qui tombe APRÈS avoir été enregistré, cas de loin le plus
		// fréquent en exploitation.
		mortal := opdsCatalogue(t)
		rec := h.expect(t, http.MethodPost, "/api/v1/discovery/sources", map[string]any{
			"name": "Bientôt éteint",
			"url":  mortal.URL + "/opds",
		}, http.StatusCreated)
		if rec.Code != http.StatusCreated {
			t.Fatal(rec.Body.String())
		}
		mortal.Close()

		search := h.expect(t, http.MethodGet,
			"/api/v1/discovery/search?q=garage", nil, http.StatusOK)

		var payload struct {
			Results []struct {
				Title string `json:"title"`
			} `json:"results"`
			Sources []struct {
				Name  string `json:"name"`
				Error string `json:"error"`
			} `json:"sources"`
		}
		if err := json.Unmarshal(search.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}

		// Le catalogue debout a répondu. C'est tout l'enjeu : un 502 ici
		// rendrait la fonctionnalité inutilisable dès qu'un service tiers
		// s'arrête, ce qui arrive tout le temps.
		if len(payload.Results) == 0 {
			t.Error("un catalogue éteint a emporté les résultats des autres")
		}

		errs := map[string]string{}
		for _, source := range payload.Sources {
			errs[source.Name] = source.Error
		}
		if errs["Bientôt éteint"] == "" {
			t.Error("le catalogue éteint doit être signalé, sinon l'utilisateur " +
				"croit que le titre n'existe pas")
		}
		if errs["Bibliothèque d'à côté"] != "" {
			t.Errorf("le catalogue debout est marqué en erreur : %q",
				errs["Bibliothèque d'à côté"])
		}
	})

	t.Run("l'essai rend un verdict, pas une erreur HTTP", func(t *testing.T) {
		rec := h.expect(t, http.MethodPost,
			"/api/v1/discovery/sources/"+sourceID+"/test", nil, http.StatusOK)

		var verdict struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &verdict); err != nil {
			t.Fatal(err)
		}
		if !verdict.OK {
			t.Errorf("essai en échec sur un catalogue debout : %s", rec.Body.String())
		}
	})

	t.Run("la liste ne rend jamais de mot de passe", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/discovery/sources", nil, http.StatusOK)
		if body := rec.Body.String(); jsonContainsKey(body, "password") ||
			jsonContainsKey(body, "secret") {
			t.Errorf("un secret figure dans la réponse : %s", body)
		}
	})

	t.Run("retirer un catalogue", func(t *testing.T) {
		h.expect(t, http.MethodDelete,
			"/api/v1/discovery/sources/"+sourceID, nil, http.StatusNoContent)

		rec := h.expect(t, http.MethodGet, "/api/v1/discovery/sources", nil, http.StatusOK)
		var payload struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		for _, item := range payload.Items {
			if item.ID == sourceID {
				t.Error("le catalogue retiré est toujours listé")
			}
		}
	})
}

// jsonContainsKey cherche une clé JSON dans un corps de réponse.
func jsonContainsKey(body, key string) bool {
	return len(body) > 0 && contains(body, `"`+key+`"`)
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

/*
Import d'un résultat vers une bibliothèque.

Deux choses à prouver, et la seconde compte plus que la première : que le
fichier arrive bien, et qu'on ne puisse pas faire télécharger n'importe quoi par
l'instance.
*/
func TestIntegrationContractDiscoveryImport(t *testing.T) {
	importableCBZ = comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 3}).Data

	h := newContractHarness(t)
	catalogue := opdsCatalogue(t)

	rec := h.expect(t, http.MethodPost, "/api/v1/discovery/sources", map[string]any{
		"name": "Voisin",
		"url":  catalogue.URL + "/opds",
	}, http.StatusCreated)

	var source struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &source); err != nil {
		t.Fatal(err)
	}

	t.Run("l'album arrive dans la bibliothèque", func(t *testing.T) {
		rec := h.expect(t, http.MethodPost, "/api/v1/discovery/import", map[string]any{
			"sourceId":  source.ID,
			"href":      catalogue.URL + "/dl/garage.cbz",
			"libraryId": h.libraryID.String(),
			"folder":    "Importés",
			"title":     "Le Garage hermétique",
		}, http.StatusCreated)

		var imported struct {
			ComicID   string `json:"comicId"`
			ObjectKey string `json:"objectKey"`
			Format    string `json:"format"`
			FileSize  int64  `json:"fileSize"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &imported); err != nil {
			t.Fatal(err)
		}

		if imported.ComicID == "" {
			t.Fatal("aucun album créé")
		}
		if imported.Format != "cbz" {
			t.Errorf("format = %q", imported.Format)
		}
		if imported.FileSize != int64(len(importableCBZ)) {
			t.Errorf("taille = %d, attendu %d", imported.FileSize, len(importableCBZ))
		}
		// Le nom vient de l'adresse, et le dossier demandé est respecté : c'est
		// ce que l'indexation analysera ensuite.
		if !strings.Contains(imported.ObjectKey, "Importés/garage.cbz") {
			t.Errorf("clé = %q, attendue dans le dossier demandé", imported.ObjectKey)
		}

		// L'album est réellement consultable, pas seulement rendu par la
		// réponse de l'import.
		h.expect(t, http.MethodGet, "/api/v1/comics/"+imported.ComicID, nil, http.StatusOK)
	})

	t.Run("un lien sans nom est nommé par son titre", func(t *testing.T) {
		rec := h.expect(t, http.MethodPost, "/api/v1/discovery/import", map[string]any{
			"sourceId":  source.ID,
			"href":      catalogue.URL + "/api/v1/books/42/file",
			"libraryId": h.libraryID.String(),
			"title":     "Arzach",
		}, http.StatusCreated)

		var imported struct {
			ObjectKey string `json:"objectKey"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &imported); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(imported.ObjectKey, "Arzach.cbz") {
			t.Errorf("clé = %q : un lien muet doit être nommé par son titre",
				imported.ObjectKey)
		}
	})

	t.Run("une adresse étrangère au catalogue est refusée", func(t *testing.T) {
		// Le refus le plus important du module. Sans lui, cette route ferait
		// télécharger n'importe quoi par l'instance, depuis l'intérieur de son
		// réseau.
		autre := opdsCatalogue(t)

		h.expect(t, http.MethodPost, "/api/v1/discovery/import", map[string]any{
			"sourceId":  source.ID,
			"href":      autre.URL + "/dl/garage.cbz",
			"libraryId": h.libraryID.String(),
		}, http.StatusUnprocessableEntity)

		h.expect(t, http.MethodPost, "/api/v1/discovery/import", map[string]any{
			"sourceId":  source.ID,
			"href":      "http://169.254.169.254/latest/meta-data/",
			"libraryId": h.libraryID.String(),
		}, http.StatusUnprocessableEntity)
	})

	t.Run("importer deux fois le même fichier est refusé", func(t *testing.T) {
		// L'objet existe déjà : l'écraser remplacerait une édition par une
		// autre et rendrait fausse la progression de lecture qui y est
		// attachée.
		h.expect(t, http.MethodPost, "/api/v1/discovery/import", map[string]any{
			"sourceId":  source.ID,
			"href":      catalogue.URL + "/dl/garage.cbz",
			"libraryId": h.libraryID.String(),
			"folder":    "Importés",
			"title":     "Le Garage hermétique",
		}, http.StatusUnprocessableEntity)
	})
}
