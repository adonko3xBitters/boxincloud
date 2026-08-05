package amule

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/amuletest"
)

// ─── Construction de la requête ──────────────────────────────────────────────

/*
TestRequestSearchNEnvoiePasLesFiltresVides.

Le démon distingue « pas de filtre de taille » d'une taille minimale nulle. Un
décodeur qui enverrait systématiquement un zéro changerait donc le sens de la
requête — et le symptôme serait une recherche qui ne rend rien, sans erreur.
*/
func TestRequestSearchNEnvoiePasLesFiltresVides(t *testing.T) {
	packet, err := requestSearch(SearchParams{Query: "debian", Network: SearchGlobal})
	if err != nil {
		t.Fatalf("requestSearch : %v", err)
	}

	root, ok := packet.Find(ec.TagSearchType)
	if !ok {
		t.Fatal("le tag racine de recherche est absent")
	}

	absents := []ec.TagName{
		ec.TagSearchMinSize,
		ec.TagSearchMaxSize,
		ec.TagSearchAvailability,
		ec.TagSearchExtension,
	}
	for _, name := range absents {
		if _, present := root.Find(name); present {
			t.Errorf("le filtre %s est envoyé alors qu'il n'a pas été demandé", name)
		}
	}

	// Le terme, lui, doit être là.
	if term, _ := root.Find(ec.TagSearchName); func() bool {
		v, _ := term.Text()
		return v != "debian"
	}() {
		t.Error("le terme recherché n'a pas été transmis")
	}
}

// TestRequestSearchPorteLeReseauEnValeur : forme surprenante mais imposée — le
// tag racine porte le réseau comme VALEUR, pas comme enfant.
func TestRequestSearchPorteLeReseauEnValeur(t *testing.T) {
	for network, code := range searchNetworkCodes {
		packet, err := requestSearch(SearchParams{Query: "x", Network: network})
		if err != nil {
			t.Fatalf("%s : %v", network, err)
		}

		root, ok := packet.Find(ec.TagSearchType)
		if !ok {
			t.Fatalf("%s : tag racine absent", network)
		}
		if v, _ := root.Uint(); v != code {
			t.Errorf("%s : code %d, attendu %d", network, v, code)
		}
	}
}

func TestRequestSearchRefuseCeQuiNaPasDeSens(t *testing.T) {
	cases := map[string]SearchParams{
		"terme vide":        {Query: "   ", Network: SearchGlobal},
		"réseau inconnu":    {Query: "x", Network: SearchNetwork("bittorrent")},
		"réseau non défini": {Query: "x"},
	}

	for nom, params := range cases {
		t.Run(nom, func(t *testing.T) {
			if _, err := requestSearch(params); err == nil {
				t.Error("requête acceptée alors qu'elle n'a pas de sens")
			}
		})
	}
}

// ─── Décodage ────────────────────────────────────────────────────────────────

/*
TestDecodeSearchStatusTraduitLaSentinelle.

Le démon rend 0xFFFF pour « aucune recherche en cours ». Le prendre au pied de
la lettre afficherait 65535 % de progression — un chiffre qu'un utilisateur
signalerait comme un bogue, et qui n'en serait pas un du côté du démon.
*/
func TestDecodeSearchStatusTraduitLaSentinelle(t *testing.T) {
	cases := []struct {
		brut     uint64
		progress int
		complete bool
	}{
		{0, 0, false},
		{45, 45, false},
		{99, 99, false},
		{100, 100, true},
		{0xFFFF, 100, true},
	}

	for _, c := range cases {
		status, err := decodeSearchStatus(ec.New(ec.OpSearchProgress,
			ec.Uint(ec.TagSearchStatus, c.brut)))
		if err != nil {
			t.Fatalf("brut=%d : %v", c.brut, err)
		}
		if status.Progress != c.progress || status.Complete != c.complete {
			t.Errorf("brut=%d → %d%% complete=%v, attendu %d%% complete=%v",
				c.brut, status.Progress, status.Complete, c.progress, c.complete)
		}
	}
}

func TestDecodeSearchResults(t *testing.T) {
	hash := strings.Repeat("ab", 16)
	entry := ec.Uint(ec.TagSearchfile, 42)
	entry.Children = []ec.Tag{
		ec.Text(ec.TagPartfileName, "debian-13.iso"),
		ec.Uint(ec.TagPartfileSizeFull, 4_294_967_296),
		hashTag(t, ec.TagPartfileHash, hash),
		ec.Uint(ec.TagPartfileSourceCount, 128),
		ec.Uint(ec.TagPartfileSourceCountXfer, 31),
		ec.Uint(ec.TagPartfileStatus, 0),
	}

	results, err := decodeSearchResults(ec.New(ec.OpSearchResults, entry))
	if err != nil {
		t.Fatalf("decodeSearchResults : %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("%d résultats, attendu 1", len(results))
	}

	got := results[0]
	if got.Name != "debian-13.iso" || got.Size != 4_294_967_296 {
		t.Errorf("nom ou taille faux : %+v", got)
	}
	if got.Hash != hash {
		t.Errorf("empreinte %q, attendu %q", got.Hash, hash)
	}
	if got.Sources != 128 || got.CompleteSources != 31 {
		t.Errorf("sources %d/%d, attendu 128/31", got.CompleteSources, got.Sources)
	}
	if got.AlreadyQueued {
		t.Error("un fichier d'état nul est marqué comme déjà en file")
	}
}

// TestDecodeSearchResultsMarqueCeQuiEstDejaEnFile : le démon réutilise le champ
// d'état d'un téléchargement pour le dire. C'est ce qui permet à l'interface de
// griser le bouton plutôt que de laisser ajouter deux fois le même fichier.
func TestDecodeSearchResultsMarqueCeQuiEstDejaEnFile(t *testing.T) {
	entry := ec.Uint(ec.TagSearchfile, 7)
	entry.Children = []ec.Tag{
		ec.Text(ec.TagPartfileName, "déjà là"),
		ec.Uint(ec.TagPartfileStatus, 1),
	}

	results, err := decodeSearchResults(ec.New(ec.OpSearchResults, entry))
	if err != nil {
		t.Fatalf("decodeSearchResults : %v", err)
	}
	if !results[0].AlreadyQueued {
		t.Error("un fichier déjà en file n'est pas marqué comme tel")
	}
}

// ─── Contre un vrai démon ────────────────────────────────────────────────────

/*
TestIntegrationRechercheEstAnalysee.

Le démon de test est hors ligne : il REFUSE toute recherche. Ce refus est plus
instructif qu'un succès, et c'est sur lui que ce test est bâti.

Pour refuser en ces termes, le démon a dû lire la requête jusqu'au bout : en
extraire le réseau — qui voyage comme VALEUR du tag racine, la forme la plus
facile à se tromper — puis vérifier la connectivité de ce réseau-là. Un tag mal
formé n'irait pas si loin.

La preuve tient au fait que les messages DIFFÈRENT. Une recherche Kad est
refusée parce que Kad ne tourne pas ; une recherche serveur parce qu'eD2k n'est
pas connecté. Si le code de réseau n'arrivait pas, les trois donneraient le
même refus.

Ce que ce test ne prouve pas : que de vrais résultats se décodent. Cela demande
un démon connecté au réseau, ce qui n'a pas sa place dans une suite automatique.
*/
func TestIntegrationRechercheEstAnalysee(t *testing.T) {
	env := amuletest.Start(t)
	svc := testService(t, env)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cases := []struct {
		network SearchNetwork
		attendu string
	}{
		{SearchServer, "ed2k"},
		{SearchGlobal, "ed2k"},
		{SearchKad, "kad"},
	}

	for _, c := range cases {
		t.Run(string(c.network), func(t *testing.T) {
			err := svc.StartSearch(ctx, SearchParams{Query: "debian", Network: c.network})
			if err == nil {
				t.Fatal("recherche acceptée alors que le réseau est hors ligne")
			}

			if !strings.Contains(strings.ToLower(err.Error()), c.attendu) {
				t.Errorf("refus = %v — attendu qu'il porte sur %q, "+
					"le code de réseau n'est donc pas arrivé", err, c.attendu)
			}
			t.Logf("%s → %v", c.network, err)
		})
	}
}

/*
TestIntegrationRechercheFiltreeEstAnalysee.

Les filtres changent la FORME de la requête : quatre enfants de plus sous le tag
racine. Ils ne doivent pas la rendre illisible — et le seul moyen de le savoir
est que le refus porte toujours sur la connectivité, pas sur la syntaxe.
*/
func TestIntegrationRechercheFiltreeEstAnalysee(t *testing.T) {
	env := amuletest.Start(t)
	svc := testService(t, env)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := svc.StartSearch(ctx, SearchParams{
		Query:        "debian",
		Network:      SearchServer,
		FileType:     "Iso",
		Extension:    "iso",
		MinSize:      1 << 20,
		MaxSize:      8 << 30,
		Availability: 5,
	})

	if err == nil {
		t.Fatal("recherche acceptée alors que le réseau est hors ligne")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "connected") {
		t.Errorf("refus = %v — attendu un refus de CONNECTIVITÉ ; "+
			"les filtres ont rendu la requête illisible", err)
	}
}

/*
TestIntegrationProgressionEtResultatsRepondent.

Ces deux-là fonctionnent hors ligne : elles interrogent l'état de la recherche,
pas le réseau. Une liste vide et une progression terminée sont le résultat juste
quand aucune recherche n'a pu démarrer.
*/
func TestIntegrationProgressionEtResultatsRepondent(t *testing.T) {
	env := amuletest.Start(t)
	svc := testService(t, env)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// La progression au repos vaut zéro, et non la sentinelle : le démon ne
	// distingue pas « rien en cours » de « tout juste démarré ». On vérifie
	// donc seulement qu'elle est LISIBLE et dans les bornes.
	status, err := svc.SearchStatus(ctx)
	if err != nil {
		t.Fatalf("progression : %v", err)
	}
	if status.Progress < 0 || status.Progress > 100 {
		t.Errorf("progression hors bornes : %d%%", status.Progress)
	}
	t.Logf("progression au repos : %d%%, terminée = %v", status.Progress, status.Complete)

	results, err := svc.SearchResults(ctx)
	if err != nil {
		t.Fatalf("résultats : %v", err)
	}
	if len(results) != 0 {
		t.Errorf("%d résultats sans recherche", len(results))
	}

	// L'arrêt d'une recherche inexistante ne doit pas échouer : l'interface
	// l'envoie en quittant l'écran, sans savoir si une recherche tournait.
	if err := svc.StopSearch(ctx); err != nil {
		t.Errorf("arrêt d'une recherche inexistante : %v", err)
	}
}

// ─── Journaux ────────────────────────────────────────────────────────────────

// TestIntegrationJournauxSontLisibles : le démon écrit dès son démarrage, ce
// journal n'est donc jamais vide — c'est le seul de ses états qui porte du
// contenu réel sur une instance au repos.
func TestIntegrationJournauxSontLisibles(t *testing.T) {
	env := amuletest.Start(t)
	svc := testService(t, env)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	lines, err := svc.Logs(ctx)
	if err != nil {
		t.Fatalf("journaux : %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("journal vide : un démon qui vient de démarrer a forcément écrit")
	}

	// Aucune ligne vide ne doit passer : elles n'apportent rien et l'espacement
	// se règle à l'affichage.
	for i, line := range lines {
		if strings.TrimSpace(line.Text) == "" {
			t.Errorf("ligne %d vide", i)
		}
	}
	/*
		Le démon envoie TOUT le journal dans un seul tag, séparé par des sauts
		de ligne. Un décodeur qui rendrait ce tag tel quel donnerait une unique
		« ligne » de plusieurs dizaines de kilo-octets — et le test passerait,
		puisqu'elle n'est ni vide ni absente.

		D'où ce contrôle : un démon qui vient de démarrer écrit une vingtaine de
		lignes, et aucune ne doit contenir de saut de ligne.
	*/
	if len(lines) < 5 {
		t.Errorf("%d ligne(s) seulement : le journal n'a probablement pas été découpé",
			len(lines))
	}
	for i, line := range lines {
		if strings.Contains(line.Text, "\n") {
			t.Fatalf("la ligne %d contient un saut de ligne : le découpage n'a pas eu lieu", i)
		}
	}

	t.Logf("%d lignes, la première : %q", len(lines), lines[0].Text)
}

// ─── Liste des serveurs ──────────────────────────────────────────────────────

/*
TestIntegrationImportDeListeEstAccepte.

Sans serveurs, rien ne fonctionne : ni connexion, ni recherche, ni source. C'est
le premier geste sur une instance neuve, et il manquait.

Le démon de test est hors ligne — il n'ira donc pas chercher l'URL — mais il
doit ACCEPTER la commande. Un opcode ou un tag faux produirait un refus, ce qui
est précisément ce qu'on vérifie ici.
*/
func TestIntegrationImportDeListeEstAccepte(t *testing.T) {
	env := amuletest.Start(t)
	svc := testService(t, env)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := svc.UpdateServerList(ctx, "https://upd.emule-security.org/server.met"); err != nil {
		t.Fatalf("import d'une liste de serveurs : %v", err)
	}
}

// TestIntegrationAjoutEtRetraitDUnServeur suit un serveur d'un bout à l'autre.
//
// Vérifié sur l'ÉTAT, comme les autres commandes : le démon n'accuse que
// réception, et une commande mal formée obtiendrait la même réponse qu'une
// commande juste.
func TestIntegrationAjoutEtRetraitDUnServeur(t *testing.T) {
	env := amuletest.Start(t)
	svc := testService(t, env)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// TEST-NET-3, jamais routable. TEST-NET-1 (192.0.2.0/24) serait plus
	// naturel mais aMule le REFUSE : « Server not added ». Il filtre certaines
	// plages réservées, et 203.0.113.0/24 n'en fait pas partie.
	const ip, port = "203.0.113.10", 4661

	if err := svc.AddServer(ctx, ip, port, "serveur d'essai"); err != nil {
		t.Fatalf("ajout : %v", err)
	}

	server := waitForServer(t, svc, ip, port, true)
	if server.Name != "serveur d'essai" {
		t.Errorf("nom = %q, attendu « serveur d'essai »", server.Name)
	}

	if err := svc.RemoveServer(ctx, ip, port); err != nil {
		t.Fatalf("retrait : %v", err)
	}
	waitForServer(t, svc, ip, port, false)
}

/*
TestIntegrationRetraitDUnServeurAbsentNommeLAdresse.

Ce test existe pour une raison précise : il distingue deux refus qui, sans lui,
se confondraient.

`EC_TAG_SERVER` porte SIX OCTETS — quatre pour l'adresse, deux pour le port —
et non la chaîne « ip:port » qu'attend l'ajout. Avec le mauvais type, le démon
ne voit aucune désignation et répond « need to define server to be removed ».
Avec le bon, il cherche vraiment, ne trouve rien, et NOMME l'adresse cherchée.

C'est cette seconde phrase qu'on exige : elle ne peut sortir que d'un tag que
le démon a su relire.
*/
func TestIntegrationRetraitDUnServeurAbsentNommeLAdresse(t *testing.T) {
	env := amuletest.Start(t)
	svc := testService(t, env)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := svc.RemoveServer(ctx, "203.0.113.99", 4661)
	if err == nil {
		t.Fatal("le démon a prétendu retirer un serveur qu'il n'a jamais eu")
	}

	if !strings.Contains(err.Error(), "203.0.113.99") {
		t.Errorf("refus = %v — attendu qu'il nomme l'adresse cherchée ; "+
			"sans elle, le démon n'a pas su relire la désignation", err)
	}
}

// TestRemoveServerRefuseCeQuiNEstPasUneIPv4 : le tag de désignation ne peut pas
// porter un nom d'hôte. Le refuser ici plutôt que d'envoyer six octets nuls,
// qui feraient agir le démon sur autre chose que ce qui était demandé.
func TestRemoveServerRefuseCeQuiNEstPasUneIPv4(t *testing.T) {
	svc := newTestService(t, &fakeRepo{}, enabled())

	for _, ip := range []string{"", "serveur.example.org", "::1", "203.0.113"} {
		var invalid ValidationError
		if err := svc.RemoveServer(t.Context(), ip, 4661); !errors.As(err, &invalid) {
			t.Errorf("%q : erreur = %v, attendu une ValidationError", ip, err)
		}
	}
}

// TestUpdateServerListRefuseUnAutreSchema : c'est le DÉMON qui va chercher
// l'URL. Un `file://` lui ferait lire son propre disque.
func TestUpdateServerListRefuseUnAutreSchema(t *testing.T) {
	svc := newTestService(t, &fakeRepo{}, enabled())

	for _, url := range []string{
		"file:///etc/passwd",
		"/home/moi/server.met",
		"ftp://exemple.org/server.met",
		"",
	} {
		if err := svc.UpdateServerList(t.Context(), url); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("%q : erreur = %v, attendu ErrInvalidURL", url, err)
		}
	}
}

// waitForServer attend qu'un serveur soit présent, ou absent, de la liste.
func waitForServer(t *testing.T, svc *Service, ip string, port int, present bool) Server {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := serviceDialer{svc: svc}.Open(ctx)
		if err != nil {
			t.Fatalf("session : %v", err)
		}
		resp, err := conn.Do(ctx, requestServers())
		_ = conn.Close()
		if err != nil {
			t.Fatalf("liste des serveurs : %v", err)
		}

		servers, err := decodeServers(resp)
		if err != nil {
			t.Fatalf("décodage : %v", err)
		}

		for _, server := range servers {
			if server.IP == ip && server.Port == port {
				if present {
					return server
				}
				break
			}
		}
		if !present {
			found := false
			for _, server := range servers {
				if server.IP == ip && server.Port == port {
					found = true
					break
				}
			}
			if !found {
				return Server{}
			}
		}
		time.Sleep(250 * time.Millisecond)
	}

	t.Fatalf("le serveur %s:%d n'est jamais devenu présent=%v", ip, port, present)
	return Server{}
}
