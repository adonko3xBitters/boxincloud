package amule

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/amuletest"
)

/*
La traduction des téléchargements, confrontée à un vrai amuled.

Les tests unitaires figent la traduction contre la forme que NOUS supposons.
Ils ne peuvent pas dire si cette forme est la bonne — c'est le rôle de ce
fichier, et c'est la même doctrine que pour le codec et le stockage.

Ce qu'un démon réel apporte ici et qu'aucune doublure ne donnerait :

  - la confirmation que la réponse porte bien un tag par fichier, nommé
    TagPartfile, dont les champs sont les ENFANTS ;
  - la confirmation qu'amuled accepte le niveau de détail demandé et qu'il
    répond avec l'opcode attendu — deux points sur lesquels une supposition
    fausse ne se verrait qu'en production ;
  - la largeur réelle des entiers, que le démon choisit tag par tag selon la
    valeur du moment.
*/

// dlDial ouvre une session authentifiée sur le démon de test.
func dlDial(t *testing.T, env amuletest.Env) *ec.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := ec.Dial(ctx,
		net.JoinHostPort(env.Host, strconv.Itoa(env.Port)),
		env.Password,
		ec.Options{ClientName: "boxincloud", ClientVersion: "test"},
	)
	if err != nil {
		t.Fatalf("connexion au démon de test : %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

/*
TestIntegrationDecodeDownloadsFileVide est le chemin nominal du décodeur.

Le démon de test n'a aucun téléchargement, et c'est précisément ce qui rend ce
test utile : il prouve que la requête est acceptée telle qu'on la construit,
que la réponse porte l'opcode attendu, et qu'une file vide traverse le décodeur
sans erreur ni tranche nil. C'est l'état dans lequel une instance neuve passe
ses premières minutes, et celui qu'un décodeur trop exigeant casserait le
premier.
*/
func TestIntegrationDecodeDownloadsFileVide(t *testing.T) {
	env := amuletest.Start(t)
	conn := dlDial(t, env)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resp, err := conn.Do(ctx, requestDownloads())
	if err != nil {
		t.Fatalf("le démon a refusé la requête de file : %v", err)
	}
	if resp.Op != ec.OpDloadQueue {
		t.Fatalf("réponse %s, attendu %s", resp.Op, ec.OpDloadQueue)
	}

	downloads, err := decodeDownloads(resp)
	if err != nil {
		t.Fatalf("décodage d'une file vide : %v", err)
	}
	if downloads == nil {
		t.Fatal("tranche nil rendue par un démon qui a bien répondu")
	}
	if len(downloads) != 0 {
		t.Fatalf("%d téléchargements sur un démon neuf : %+v", len(downloads), downloads)
	}
}

/*
TestIntegrationDecodeDownloadsAvecUnFichier met un vrai fichier dans la file.

Un lien ed2k suffit à créer un fichier partiel, sans réseau : le démon de test
est hors ligne, il ne trouvera jamais de source, mais il enregistre le fichier,
lui donne un état et une priorité, et le décrit dans ses réponses. C'est le
seul moyen d'observer les tags qui n'existent pas sur une file vide —
l'empreinte, le nom, la taille, l'état, la priorité.

Sur l'empreinte, ce test vaut plus que tous les tests unitaires réunis : le
lien porte une empreinte connue de nous, et le démon la renvoie sous forme
d'octets bruts. Si la lecture du tag hash16 ou la mise en hexadécimal était
fausse, la comparaison échouerait ici et nulle part ailleurs.
*/
func TestIntegrationDecodeDownloadsAvecUnFichier(t *testing.T) {
	env := amuletest.Start(t)
	conn := dlDial(t, env)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const (
		wantHash = "0123456789abcdef0123456789abcdef"
		wantName = "boxincloud-integration.bin"
		wantSize = int64(104_857_600) // cent mébioctets, jamais téléchargés
	)
	link := fmt.Sprintf("ed2k://|file|%s|%d|%s|/", wantName, wantSize, wantHash)

	resp, err := conn.Do(ctx, ec.New(ec.OpAddLink, ec.Text(ec.TagString, link)))
	if err != nil {
		t.Fatalf("le démon a refusé le lien ed2k : %v", err)
	}
	if resp.Op != ec.OpNoop {
		t.Fatalf("réponse %s à l'ajout du lien, attendu %s", resp.Op, ec.OpNoop)
	}

	// Le fichier n'apparaît pas dans l'instant : amuled réserve d'abord son
	// espace disque. Scruter est plus honnête qu'attendre une durée choisie
	// au jugé, qui serait tantôt trop longue tantôt trop courte.
	var found Download
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := conn.Do(ctx, requestDownloads())
		if err != nil {
			t.Fatalf("requête de file : %v", err)
		}
		downloads, err := decodeDownloads(resp)
		if err != nil {
			t.Fatalf("décodage de la file : %v", err)
		}
		if len(downloads) > 0 {
			found = downloads[0]
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("le lien a été accepté mais aucun fichier n'est apparu dans la file")
		}
		time.Sleep(500 * time.Millisecond)
	}

	if found.Hash != wantHash {
		t.Errorf("empreinte %q, attendu %q", found.Hash, wantHash)
	}
	if found.Name != wantName {
		t.Errorf("nom %q, attendu %q", found.Name, wantName)
	}
	if found.Size != wantSize {
		t.Errorf("taille %d, attendu %d", found.Size, wantSize)
	}

	// Aucun octet ne peut être arrivé : le démon est hors ligne et n'a jamais
	// vu une source.
	if found.SizeDone != 0 {
		t.Errorf("%d octets acquis sur un démon hors ligne", found.SizeDone)
	}
	if found.Sources.Total != 0 {
		t.Errorf("%d sources connues sur un démon hors ligne", found.Sources.Total)
	}

	// Le nombre de parties se déduit de la taille, le démon ne l'envoie pas.
	// Cent mébioctets font onze parties de 9 728 000 octets.
	if want := ed2kPartCount(wantSize); found.PartCount != want {
		t.Errorf("nombre de parties %d, attendu %d", found.PartCount, want)
	}

	// L'état doit être traduit, quel qu'il soit : la chaîne vide dirait que le
	// code reçu n'a traversé aucune branche de la table.
	if found.Status == "" {
		t.Error("état vide : le code du démon n'a pas été traduit")
	}
	if found.Status == DownloadUnknown {
		t.Errorf("état non reconnu : la table des états ne couvre pas ce que "+
			"répond ce démon (version %q)", conn.ServerVersion)
	}
	// Sans source, il n'y a rien à recevoir : ni « en cours », ni ETA.
	if found.Status == DownloadDownloading {
		t.Error("état « en cours » sur un fichier sans aucune source")
	}
	if found.ETA != nil {
		t.Errorf("ETA %v calculée sans le moindre débit", *found.ETA)
	}

	if found.Priority == "" {
		t.Error("priorité vide : le code du démon n'a pas été traduit")
	}

	t.Logf("fichier décodé : %s, %d octets, état %q, priorité %q, %d parties",
		found.Name, found.Size, found.Status, found.Priority, found.PartCount)

	// La table ECID → empreinte est ce qui permettra de rattacher une source à
	// son fichier. Elle se lit dans la même réponse, et elle doit être
	// peuplée dès qu'il y a un fichier.
	resp, err = conn.Do(ctx, requestDownloads())
	if err != nil {
		t.Fatalf("requête de file : %v", err)
	}
	ids, err := decodeDownloadIDs(resp)
	if err != nil {
		t.Fatalf("lecture des ECID : %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("%d correspondances ECID → empreinte, une seule attendue : %v", len(ids), ids)
	}
	for ecid, hash := range ids {
		if hash != wantHash {
			t.Errorf("ECID %d → empreinte %q, attendu %q", ecid, hash, wantHash)
		}
		if ecid == 0 {
			t.Error("ECID nul : la valeur du tag de fichier n'a pas été lue")
		}
	}
}

/*
TestIntegrationDecodeSourcesSansPair vérifie la seule requête que le protocole
permette pour obtenir des pairs.

Il n'existe aucune opération « sources de ce fichier » : la mise à jour globale
est le seul endroit où amuled décrit les pairs un par un, et elle n'existe qu'au
niveau incrémental. Ce test le prouve dans les deux sens — le démon accepte
cette combinaison-là, et il répond avec un opcode qui n'a rien d'évident
(OpSharedFiles, alors qu'on a demandé une mise à jour).

Le démon de test étant hors ligne, aucun pair ne peut apparaître. Ce qui est
vérifié est donc que le chemin traverse proprement : requête acceptée, réponse
reconnue, liste vide.
*/
func TestIntegrationDecodeSourcesSansPair(t *testing.T) {
	env := amuletest.Start(t)
	conn := dlDial(t, env)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resp, err := conn.Do(ctx, requestSources("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("le démon a refusé la requête de mise à jour : %v", err)
	}
	if resp.Op != ec.OpSharedFiles {
		t.Fatalf("réponse %s, attendu %s — la forme supposée par decodeSources "+
			"ne correspond pas à ce démon (version %q)",
			resp.Op, ec.OpSharedFiles, conn.ServerVersion)
	}

	sources, err := decodeSources(resp)
	if err != nil {
		t.Fatalf("décodage des sources : %v", err)
	}
	if sources == nil {
		t.Fatal("tranche nil rendue par un démon qui a bien répondu")
	}
	if len(sources) != 0 {
		t.Fatalf("%d sources sur un démon hors ligne : %+v", len(sources), sources)
	}
}
