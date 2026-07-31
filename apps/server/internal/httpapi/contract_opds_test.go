package httpapi_test

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

/*
Catalogue OPDS sortant.

Ces tests ne valident pas le contrat OpenAPI — l'OPDS n'y figure pas, et n'a pas
à y figurer : c'est un protocole public figé depuis 2018, dont la spécification
est ailleurs. Ce qui est vérifié ici est ce dont un lecteur tiers a réellement
besoin pour fonctionner, et que rien d'autre ne garantit.

Trois choses reviennent à chaque test, parce que ce sont celles qui cassent en
premier chez les clients réels : le type de contenu qui distingue navigation et
acquisition, les adresses ABSOLUES, et l'en-tête `WWW-Authenticate` sans lequel
un lecteur n'ouvre jamais sa boîte d'identifiants.
*/

// opdsGet interroge le catalogue en Basic, comme le ferait un lecteur tiers.
func opdsGet(t *testing.T, h *contractHarness, path, user, password string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if user != "" {
		req.SetBasicAuth(user, password)
	}

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

// Les identifiants du compte que le harnais installe.
const (
	opdsUser     = "contract"
	opdsPassword = "un mot de passe solide"
)

type opdsTestFeed struct {
	XMLName xml.Name `xml:"feed"`
	Title   string   `xml:"title"`
	ID      string   `xml:"id"`
	Links   []struct {
		Rel  string `xml:"rel,attr"`
		Href string `xml:"href,attr"`
		Type string `xml:"type,attr"`
	} `xml:"link"`
	Entries []struct {
		Title string `xml:"title"`
		ID    string `xml:"id"`
		Links []struct {
			Rel  string `xml:"rel,attr"`
			Href string `xml:"href,attr"`
			Type string `xml:"type,attr"`
		} `xml:"link"`
	} `xml:"entry"`
}

func parseFeed(t *testing.T, rec *httptest.ResponseRecorder) opdsTestFeed {
	t.Helper()

	var feed opdsTestFeed
	if err := xml.Unmarshal(rec.Body.Bytes(), &feed); err != nil {
		t.Fatalf("flux illisible : %v\n%s", err, rec.Body.String())
	}
	return feed
}

/*
TestIntegrationOPDSRequiresBasicAuth couvre ce qui rend le catalogue utilisable.

Le refus doit porter `WWW-Authenticate`. Sans cet en-tête, un lecteur tiers
n'ouvre pas sa boîte d'identifiants : l'utilisateur voit un catalogue vide, ou
une erreur, sans qu'on lui ait jamais demandé de se connecter.
*/
func TestIntegrationOPDSRequiresBasicAuth(t *testing.T) {
	h := newContractHarness(t)

	rec := opdsGet(t, h, "/opds", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("statut %d, attendu 401", rec.Code)
	}

	challenge := rec.Header().Get("WWW-Authenticate")
	if !strings.HasPrefix(challenge, "Basic ") {
		t.Errorf("WWW-Authenticate = %q : sans lui, aucun lecteur tiers ne "+
			"demande ses identifiants", challenge)
	}

	// Un mot de passe faux est refusé de la même façon.
	rec = opdsGet(t, h, "/opds", opdsUser, "mauvais mot de passe")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("statut %d pour un mot de passe faux, attendu 401", rec.Code)
	}
}

func TestIntegrationOPDSRoot(t *testing.T) {
	h := newContractHarness(t)

	rec := opdsGet(t, h, "/opds", opdsUser, opdsPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("statut %d : %s", rec.Code, rec.Body.String())
	}

	/*
		Le type distingue navigation et acquisition, et les lecteurs s'y fient.
		Une racine annoncée comme flux d'acquisition ferait chercher des
		téléchargements à un lecteur qui n'en trouverait aucun.
	*/
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "kind=navigation") {
		t.Errorf("Content-Type = %q, attendu un flux de navigation", contentType)
	}

	feed := parseFeed(t, rec)
	if len(feed.Entries) == 0 {
		t.Fatal("racine vide : rien par où entrer")
	}

	// La recherche est ANNONCÉE : la spécification OPDS n'a pas d'URL
	// conventionnelle, et un lecteur qui ne trouve pas ce lien conclut que le
	// catalogue ne sait pas chercher.
	var search string
	for _, link := range feed.Links {
		if link.Rel == "search" {
			search = link.Href
		}
	}
	if search == "" {
		t.Error("aucun lien de recherche annoncé")
	}

	// Les adresses doivent être ABSOLUES : un lecteur tiers résout mal le
	// relatif, et certains ne le résolvent pas du tout.
	for _, entry := range feed.Entries {
		for _, link := range entry.Links {
			if !strings.HasPrefix(link.Href, "http") {
				t.Errorf("adresse relative dans le flux : %q", link.Href)
			}
		}
	}
}

func TestIntegrationOPDSAcquisitionFeed(t *testing.T) {
	h := newContractHarness(t)

	rec := opdsGet(t, h, "/opds/recent", opdsUser, opdsPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("statut %d : %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Header().Get("Content-Type"), "kind=acquisition") {
		t.Errorf("Content-Type = %q, attendu un flux d'acquisition",
			rec.Header().Get("Content-Type"))
	}

	feed := parseFeed(t, rec)
	if len(feed.Entries) == 0 {
		t.Fatal("aucun album dans les ajouts récents")
	}

	entry := feed.Entries[0]

	var acquisition, thumbnail string
	for _, link := range entry.Links {
		switch link.Rel {
		case "http://opds-spec.org/acquisition":
			acquisition = link.Href
			/*
				Le type MIME du format, et pas un type générique : c'est lui qui
				permet à un lecteur de savoir s'il sait ouvrir le fichier AVANT
				de le télécharger. Un octet-stream ferait apparaître tous les
				albums comme des fichiers inconnus.
			*/
			if !strings.Contains(link.Type, "comicbook") {
				t.Errorf("type d'acquisition = %q, attendu un type de bande dessinée",
					link.Type)
			}
		case "http://opds-spec.org/image/thumbnail":
			thumbnail = link.Href
		}
	}

	if acquisition == "" {
		t.Error("aucun lien de téléchargement : le flux n'est pas une acquisition")
	}
	if thumbnail == "" {
		t.Error("aucune vignette : le lecteur affichera une grille vide")
	}
}

// TestIntegrationOPDSSearch vérifie le chemin complet de la recherche.
//
// Document OpenSearch d'abord, puis la requête qu'il décrit — c'est exactement
// le parcours que le client OPDS de ce projet fait sur un catalogue distant,
// servi ici dans l'autre sens.
func TestIntegrationOPDSSearch(t *testing.T) {
	h := newContractHarness(t)

	rec := opdsGet(t, h, "/opds/search.xml", opdsUser, opdsPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("statut %d sur le document OpenSearch", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "{searchTerms}") {
		t.Errorf("document OpenSearch sans gabarit : %s", body)
	}
	if !strings.Contains(body, "http") {
		t.Error("le gabarit doit porter une adresse absolue")
	}

	rec = opdsGet(t, h, "/opds/search?q=Tintin", opdsUser, opdsPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("statut %d : %s", rec.Code, rec.Body.String())
	}
	feed := parseFeed(t, rec)
	if len(feed.Entries) == 0 {
		t.Error("la recherche ne rend rien sur un titre présent dans le harnais")
	}
}

/*
TestIntegrationOPDSDownload vérifie qu'un album se télécharge entier.

C'est le seul chemin du projet qui rende le fichier complet. Il va à l'encontre
de tout le reste — l'accès aléatoire par requête Range est la promesse du
projet — et il existe parce qu'un lecteur tiers n'a aucun moyen de faire
autrement.
*/
func TestIntegrationOPDSDownload(t *testing.T) {
	h := newContractHarness(t)

	feed := parseFeed(t, opdsGet(t, h, "/opds/recent", opdsUser, opdsPassword))
	if len(feed.Entries) == 0 {
		t.Fatal("aucun album à télécharger")
	}

	var href string
	for _, link := range feed.Entries[0].Links {
		if link.Rel == "http://opds-spec.org/acquisition" {
			href = link.Href
		}
	}
	if href == "" {
		t.Fatal("aucun lien d'acquisition")
	}

	// L'adresse est absolue ; le harnais n'interroge que le chemin.
	at := strings.Index(href, "/opds/")
	rec := opdsGet(t, h, href[at:], opdsUser, opdsPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("statut %d : %s", rec.Code, rec.Body.String())
	}

	if rec.Body.Len() == 0 {
		t.Error("archive vide")
	}
	// Un CBZ est un ZIP : la signature prouve qu'on a servi le fichier
	// d'origine et non une transformation.
	if got := rec.Body.Bytes(); len(got) < 4 || string(got[:2]) != "PK" {
		t.Errorf("les octets ne sont pas une archive ZIP : % x", got[:min(8, len(got))])
	}

	/*
		Le nom de fichier compte : c'est lui que le lecteur tiers affichera et
		classera. Un identifiant technique rendrait la bibliothèque illisible
		chez lui.
	*/
	disposition := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disposition, "filename") {
		t.Errorf("Content-Disposition = %q, sans nom de fichier", disposition)
	}
	if !strings.Contains(disposition, ".cbz") {
		t.Errorf("Content-Disposition = %q, sans extension : le lecteur ne "+
			"saura pas quoi en faire", disposition)
	}
}

/*
TestIntegrationOPDSSeesExactlyWhatTheAPISees est le test qui compte le plus.

Un catalogue OPDS est une SECONDE PORTE sur les mêmes données. S'il applique
d'autres règles que l'API, tout le travail de restriction par profil devient
contournable en collant une adresse dans un lecteur tiers.

La première version de ce test vérifiait qu'un compte ordinaire ne voyait
aucune bibliothèque. Elle avait tort, et le code avait raison : dans ce projet,
une bibliothèque SANS autorisation explicite est visible de tous. L'invariant
correct n'est pas « le compte ordinaire ne voit rien », c'est « les deux portes
montrent la même chose au même compte » — et c'est aussi le seul qui résiste à
un changement du modèle de droits.
*/
func TestIntegrationOPDSSeesExactlyWhatTheAPISees(t *testing.T) {
	h := newContractHarness(t)

	compare := func(t *testing.T, token, user, password string) {
		t.Helper()

		// Ce que l'API montre à ce compte.
		rec := h.callAs(t, token, http.MethodGet, "/api/v1/libraries", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("statut %d sur l'API : %s", rec.Code, rec.Body.String())
		}

		var payload struct {
			Libraries []struct {
				ID string `json:"id"`
			} `json:"libraries"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}

		expected := map[string]bool{}
		for _, library := range payload.Libraries {
			expected["urn:boxincloud:opds:library:"+library.ID] = true
		}

		// Ce que l'OPDS montre au même compte.
		feed := parseFeed(t, opdsGet(t, h, "/opds", user, password))
		seen := map[string]bool{}
		for _, entry := range feed.Entries {
			if strings.HasPrefix(entry.ID, "urn:boxincloud:opds:library:") {
				seen[entry.ID] = true
			}
		}

		for id := range expected {
			if !seen[id] {
				t.Errorf("%s est visible par l'API mais absente de l'OPDS", id)
			}
		}
		for id := range seen {
			if !expected[id] {
				t.Errorf("%s est annoncée par l'OPDS alors que l'API la masque : "+
					"le catalogue est une porte dérobée", id)
			}
		}
	}

	t.Run("administrateur", func(t *testing.T) {
		compare(t, h.token, opdsUser, opdsPassword)
	})

	t.Run("compte ordinaire", func(t *testing.T) {
		compare(t, h.userToken, "ordinaire", "un autre mot de passe solide")
	})
}
