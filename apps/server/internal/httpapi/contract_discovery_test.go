package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

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

	// request demande un import et rend sa ligne de suivi, encore en attente.
	request := func(t *testing.T, body map[string]any) importRecord {
		t.Helper()
		rec := h.expect(t, http.MethodPost, "/api/v1/discovery/import", body,
			http.StatusAccepted)

		var record importRecord
		if err := json.Unmarshal(rec.Body.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		if record.Status != "queued" {
			t.Fatalf("statut initial = %q, attendu queued", record.Status)
		}
		return record
	}

	t.Run("l'album arrive dans la bibliothèque", func(t *testing.T) {
		record := request(t, map[string]any{
			"sourceId":  source.ID,
			"href":      catalogue.URL + "/dl/garage.cbz",
			"libraryId": h.libraryID.String(),
			"folder":    "Importés",
			"title":     "Le Garage hermétique",
		})

		// Rien n'est encore arrivé : la requête a rendu 202, pas le fichier.
		// C'est tout l'objet du passage en tâche de fond.
		if record.ComicID != "" {
			t.Error("un album existe déjà alors que le job n'a pas tourné")
		}

		h.runImport(t, record.ID)

		done := h.findImport(t, record.ID)
		if done.Status != "done" {
			t.Fatalf("statut = %q (%s : %s)", done.Status, done.ErrorCode, done.ErrorDetail)
		}
		if done.FileSize != int64(len(importableCBZ)) {
			t.Errorf("taille = %d, attendu %d", done.FileSize, len(importableCBZ))
		}
		// Le nom vient de l'adresse, et le dossier demandé est respecté : c'est
		// ce que l'indexation analysera ensuite.
		if !strings.Contains(done.ObjectKey, "Importés/garage.cbz") {
			t.Errorf("clé = %q, attendue dans le dossier demandé", done.ObjectKey)
		}

		// L'album est réellement consultable, pas seulement consigné.
		h.expect(t, http.MethodGet, "/api/v1/comics/"+done.ComicID, nil, http.StatusOK)
	})

	t.Run("un lien sans nom est nommé par son titre", func(t *testing.T) {
		record := request(t, map[string]any{
			"sourceId":  source.ID,
			"href":      catalogue.URL + "/api/v1/books/42/file",
			"libraryId": h.libraryID.String(),
			"title":     "Arzach",
		})
		h.runImport(t, record.ID)

		done := h.findImport(t, record.ID)
		if !strings.Contains(done.ObjectKey, "Arzach.cbz") {
			t.Errorf("clé = %q : un lien muet doit être nommé par son titre",
				done.ObjectKey)
		}
	})

	t.Run("un catalogue éteint est consigné, pas rendu en erreur HTTP", func(t *testing.T) {
		/*
			Le cœur du passage en tâche de fond. La demande est acceptée —
			personne ne peut savoir avant d'essayer que le catalogue ne répondra
			pas — et l'échec atterrit dans la ligne de suivi, avec un code que
			l'interface traduit.

			Sans cela, un import de fond serait une action qu'on lance et qui
			disparaît.
		*/
		mortel := opdsCatalogue(t)
		autre := h.expect(t, http.MethodPost, "/api/v1/discovery/sources", map[string]any{
			"name": "Bientôt éteint",
			"url":  mortel.URL + "/opds",
		}, http.StatusCreated)

		var mortelSource struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(autre.Body.Bytes(), &mortelSource); err != nil {
			t.Fatal(err)
		}

		record := request(t, map[string]any{
			"sourceId":  mortelSource.ID,
			"href":      mortel.URL + "/dl/garage.cbz",
			"libraryId": h.libraryID.String(),
			"title":     "Jamais arrivé",
		})

		mortel.Close()
		h.runImport(t, record.ID)

		done := h.findImport(t, record.ID)
		if done.Status != "failed" {
			t.Fatalf("statut = %q, attendu failed", done.Status)
		}
		if done.ErrorCode == "" {
			t.Error("un échec doit porter un code : l'interface le traduit")
		}
		if done.ErrorDetail == "" {
			t.Error("un échec doit porter un diagnostic : c'est ce qui permet de comprendre")
		}
	})

	t.Run("une adresse étrangère est refusée tout de suite", func(t *testing.T) {
		/*
			Le refus le plus important du module, et il reste SYNCHRONE malgré
			le passage en tâche de fond : l'adresse est une propriété de la
			demande, pas une découverte du téléchargement.

			Le différer ferait croire à l'utilisateur que son import est parti,
			et transformerait un refus de sécurité en ligne d'historique.
		*/
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

	t.Run("importer deux fois le même fichier échoue à l'exécution", func(t *testing.T) {
		// L'objet existe déjà : l'écraser remplacerait une édition par une
		// autre et rendrait fausse la progression de lecture qui y est
		// attachée. Le savoir demande d'interroger le stockage, donc le refus
		// ne peut venir qu'au moment du dépôt.
		record := request(t, map[string]any{
			"sourceId":  source.ID,
			"href":      catalogue.URL + "/dl/garage.cbz",
			"libraryId": h.libraryID.String(),
			"folder":    "Importés",
			"title":     "Le Garage hermétique",
		})
		h.runImport(t, record.ID)

		done := h.findImport(t, record.ID)
		if done.Status != "failed" || done.ErrorCode != "exists" {
			t.Errorf("import = %+v, attendu un échec avec le code exists", done)
		}
	})
}

// importRecord est la ligne de suivi d'un import, telle que l'API la rend.
type importRecord struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	ErrorCode   string `json:"errorCode"`
	ErrorDetail string `json:"errorDetail"`
	ComicID     string `json:"comicId"`
	ObjectKey   string `json:"objectKey"`
	FileSize    int64  `json:"fileSize"`
}

// runImport déroule le job sans démarrer de workers.
//
// Le harnais tourne avec la file désactivée — le contrat porte sur l'API, pas
// sur l'ordonnancement — mais le chemin exécuté ici est exactement celui du
// worker, ce qui est ce qui donne au test sa valeur.
func (h *contractHarness) runImport(t *testing.T, id string) {
	t.Helper()

	importID, err := uuid.Parse(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.core.RunImport(context.Background(), importID); err != nil {
		t.Fatalf("exécution de l'import : %v", err)
	}
}

// findImport relit une ligne de suivi via l'API, comme le fait l'interface.
func (h *contractHarness) findImport(t *testing.T, id string) importRecord {
	t.Helper()

	rec := h.expect(t, http.MethodGet, "/api/v1/discovery/imports", nil, http.StatusOK)

	var payload struct {
		Items []importRecord `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, item := range payload.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("import %s absent du suivi", id)
	return importRecord{}
}

/*
Rapprochement de métadonnées.

Le harnais n'a aucune base enregistrée — les tests d'intégration ne doivent
joindre aucun service tiers. Ce qui est vérifié ici est donc le contrat de la
route, pas le contenu des fiches : les fournisseurs ont leurs propres tests,
contre de vraies réponses HTTP servies localement.

Une instance sans base enregistrée est un cas réel, pas un artefact de test :
c'est celle d'un serveur familial sur un réseau fermé. Elle doit rendre une
liste vide, pas une erreur.
*/
func TestIntegrationContractDiscoveryDescribe(t *testing.T) {
	h := newContractHarness(t)

	t.Run("une œuvre sans base enregistrée rend une liste vide", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet,
			"/api/v1/discovery/describe?title=L%27Incal&author=Moebius", nil, http.StatusOK)

		var payload struct {
			Candidates []struct {
				Title      string  `json:"title"`
				Confidence float64 `json:"confidence"`
			} `json:"candidates"`
			Sources []struct {
				Name string `json:"name"`
			} `json:"sources"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}

		// Des listes vides plutôt que `null` : l'interface reçoit du JSON, et
		// `null` n'y est pas une liste vide.
		if payload.Candidates == nil || payload.Sources == nil {
			t.Errorf("listes nulles : %s", rec.Body.String())
		}
	})

	t.Run("sans titre ni ISBN, la demande est refusée", func(t *testing.T) {
		// Il n'y a rien à chercher : interroger trois bases publiques avec une
		// requête vide serait leur faire perdre leur temps et le nôtre.
		h.expect(t, http.MethodGet, "/api/v1/discovery/describe?year=1981",
			nil, http.StatusUnprocessableEntity)
	})

	t.Run("le rapprochement est ouvert à tout compte", func(t *testing.T) {
		// C'est une consultation de bases publiques : elle ne révèle rien du
		// contenu de l'instance, et la réserver aux administrateurs priverait
		// les autres de la correction de leurs propres albums.
		rec := h.callAs(t, h.userToken, http.MethodGet,
			"/api/v1/discovery/describe?title=L%27Incal", nil)
		if rec.Code != http.StatusOK {
			t.Errorf("statut %d pour un compte ordinaire : %s", rec.Code, rec.Body.String())
		}
	})
}

/*
Le genre d'une source, et les gabarits de scraping.

Ce qui est vérifié ici est la charnière ajoutée au contrat : une source peut
désormais déclarer un `kind` autre qu'OPDS, et l'administration doit pouvoir
savoir lesquels cette instance sait traiter.

Aucun gabarit n'est livré aujourd'hui (voir
internal/discovery/scraper/templates/README.md), et c'est précisément ce que le
premier cas fixe : la liste vide est une réponse NORMALE, pas une panne. Le jour
où un gabarit est livré, ce test dira que le contrat tient encore.
*/
func TestIntegrationContractScraperTemplates(t *testing.T) {
	h := newContractHarness(t)

	t.Run("les gabarits chargés sont proposables tels quels", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet,
			"/api/v1/discovery/scraper-templates", nil, http.StatusOK)

		var payload struct {
			Items []struct {
				Kind     string   `json:"kind"`
				ID       string   `json:"id"`
				Name     string   `json:"name"`
				Homepage string   `json:"homepage"`
				Mirrors  []string `json:"mirrors"`
			} `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}

		// `null` n'est pas une liste vide en JavaScript, et l'interface itère
		// dessus sans se demander laquelle des deux elle a reçue.
		if payload.Items == nil {
			t.Fatalf("liste nulle plutôt que vide : %s", rec.Body.String())
		}

		var reference bool
		for _, item := range payload.Items {
			// Le genre doit être composable tel quel : l'interface le recopie
			// dans le formulaire de création, elle ne le reconstruit pas.
			if item.Kind != "scraper:"+item.ID {
				t.Errorf("genre %q incohérent avec l'identifiant %q", item.Kind, item.ID)
			}
			if item.Name == "" {
				t.Errorf("gabarit %q sans nom affichable", item.ID)
			}
			if item.ID == "comicshelf" {
				reference = true
				// Les miroirs sont ce qui permet à l'administration de montrer
				// d'où viendront les requêtes avant qu'il n'active la source.
				if len(item.Mirrors) == 0 {
					t.Error("gabarit sans miroir : l'écran ne peut rien montrer")
				}
				if item.Homepage == "" {
					t.Error("gabarit sans page d'accueil : impossible d'aller voir le site")
				}
			}
		}

		if !reference {
			t.Errorf("le gabarit de référence n'est pas proposé : %s", rec.Body.String())
		}
	})

	/*
		Un gabarit chargé devient un genre de source acceptable.

		Le catalogue est refusé au bout du compte — ses miroirs sont en
		`.example` et ne répondent pas — mais le refus doit venir de l'ESSAI,
		pas d'un genre incompris. C'est ce qui distingue « le site ne répond
		pas » de « ce gabarit n'existe pas », deux diagnostics que
		l'administrateur ne traite pas de la même façon.
	*/
	t.Run("un gabarit chargé est reconnu comme genre", func(t *testing.T) {
		rec := h.expect(t, http.MethodPost, "/api/v1/discovery/sources", map[string]any{
			"name": "Comic Shelf",
			"kind": "scraper:comicshelf",
		}, http.StatusUnprocessableEntity)

		if strings.Contains(rec.Body.String(), "genre de catalogue inconnu") {
			t.Errorf("le gabarit chargé n'a pas été reconnu : %s", rec.Body.String())
		}
	})

	t.Run("kind: opds explicite vaut le défaut", func(t *testing.T) {
		catalogue := opdsCatalogue(t)

		rec := h.expect(t, http.MethodPost, "/api/v1/discovery/sources", map[string]any{
			"name": "Catalogue explicite",
			"kind": "opds",
			"url":  catalogue.URL + "/opds",
		}, http.StatusCreated)

		var created struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		if created.Kind != "opds" {
			t.Errorf("genre = %q", created.Kind)
		}
	})

	/*
		Un gabarit absent est refusé À LA SAISIE.

		C'est le cas qu'il ne faut surtout pas laisser passer : sans ce contrôle,
		la source serait enregistrée puis traitée par le client OPDS, qui
		demanderait un flux Atom à une page HTML. L'échec serait tardif, et son
		message ne dirait rien de la vraie cause.
	*/
	t.Run("un gabarit inconnu est refusé", func(t *testing.T) {
		h.expect(t, http.MethodPost, "/api/v1/discovery/sources", map[string]any{
			"name": "Gabarit fantôme",
			"kind": "scraper:absent",
			"url":  "https://exemple.test",
		}, http.StatusUnprocessableEntity)
	})

	// Le contrat borne la forme du genre. Un client qui enverrait n'importe
	// quoi doit être arrêté par la validation d'entrée, pas par le handler.
	t.Run("un genre hors du motif est refusé par le contrat", func(t *testing.T) {
		rec := h.callWith(t, http.MethodPost, "/api/v1/discovery/sources",
			map[string]any{
				"name": "Genre douteux",
				"kind": "SCRAPER:Majuscules",
				"url":  "https://exemple.test",
			}, false)

		if rec.Code == http.StatusCreated {
			t.Errorf("un genre hors motif a été accepté : %s", rec.Body.String())
		}
	})
}
