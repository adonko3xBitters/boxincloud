package amule

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/amuletest"
)

/*
La traduction des serveurs et de l'état de connexion, confrontée à un vrai
amuled.

Les tests unitaires figent une lecture de la spécification ; ils ne peuvent pas
dire si elle a été bien LUE. Un tag mal nommé, un champ de bits décalé, une
adresse à l'envers : tout cela passe les tests unitaires sans broncher, parce
que les paquets y sont construits par le même code qui les relit.

Le démon de test est délibérément HORS LIGNE — ni serveur, ni Kad, ni Internet.
Ce n'est pas une limite, c'est le cas qu'on veut prouver en premier : un état
entièrement déconnecté doit se décoder proprement, sans erreur et sans deviner.
Ce que le conteneur ne peut pas produire — un serveur joint, un LowID, un Kad
pare-feu — est couvert par les tests unitaires.
*/

// dialDaemon ouvre une session authentifiée avec le démon de test.
func dialDaemon(t *testing.T, env amuletest.Env) *ec.Conn {
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

// exchange fait un aller-retour borné dans le temps.
func exchange(t *testing.T, conn *ec.Conn, req ec.Packet) ec.Packet {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := conn.Do(ctx, req)
	if err != nil {
		t.Fatalf("requête %s : %v", req.Op, err)
	}
	return resp
}

// ─── Liste de serveurs ───────────────────────────────────────────────────────

/*
TestIntegrationDecodeServersListeVide vérifie le cas d'un démon neuf.

Une liste vide est le seul état qu'un démon hors ligne connaisse, et c'est aussi
celui qu'on verra chez tout utilisateur au premier démarrage. Il doit se décoder
sans erreur et rendre une tranche vide et non nulle : un appelant qui itère
dessus n'a pas à distinguer les deux.

Ce test prouve aussi, à lui seul, que l'opcode attendu est le bon — le décodeur
refuse toute autre réponse.
*/
func TestIntegrationDecodeServersListeVide(t *testing.T) {
	env := amuletest.Start(t)
	conn := dialDaemon(t, env)

	resp := exchange(t, conn, requestServers())

	servers, err := decodeServers(resp)
	if err != nil {
		t.Fatalf("décodage de la liste : %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("%d serveurs sur un démon configuré sans liste : %+v",
			len(servers), servers)
	}
	if servers == nil {
		t.Error("liste nil plutôt que vide")
	}
}

/*
TestIntegrationDecodeServersEntreeReelle décode une entrée écrite par le démon.

C'est le test qui compte : on ajoute un serveur, le démon le range dans sa
liste, le resérialise à sa façon, et on vérifie qu'on retrouve exactement ce
qu'on a mis. L'adresse est le point sensible — elle voyage dans la VALEUR du tag
conteneur, en quatre octets suivis du port, et une inversion d'ordre produirait
une adresse plausible que seul un aller-retour comme celui-ci démasque.

Deux réglages préalables, tous deux imposés par le démon :

  - le réseau eD2k doit être ACTIVÉ dans ses préférences. Sinon il court-circuite
    la demande et répond une liste vide sans regarder ce qu'il connaît — le
    serveur serait bien ajouté, et invisible ;
  - la modification se fait au niveau de détail complet, seul niveau où le démon
    ne touche QUE les réglages qu'on lui transmet. Aux autres niveaux, un
    réglage absent de la requête est remis à faux, ce qui reconfigurerait au
    passage la connexion automatique et le réseau Kad.

Aucune connexion n'est tentée : la connexion automatique reste éteinte, et le
serveur ajouté n'est qu'une ligne dans une liste.
*/
func TestIntegrationDecodeServersEntreeReelle(t *testing.T) {
	env := amuletest.Start(t)
	conn := dialDaemon(t, env)

	// Activation du réseau eD2k, sans rien changer d'autre.
	exchange(t, conn, ec.New(ec.OpSetPreferences,
		ec.Uint(ec.TagDetailLevel, uint64(ec.DetailFull)),
		ec.Tag{
			Name:     ec.TagPrefsConnections,
			Type:     ec.TypeCustom,
			Children: []ec.Tag{ec.Uint(ec.TagNetworkEd2k, 1)},
		},
	))

	const (
		wantIP   = "176.103.48.36"
		wantPort = 4184
		wantName = "serveur de test boxincloud"
	)

	exchange(t, conn, ec.New(ec.OpServerAdd,
		ec.Text(ec.TagServerAddress, wantIP+":"+strconv.Itoa(wantPort)),
		ec.Text(ec.TagServerName, wantName),
	))

	servers, err := decodeServers(exchange(t, conn, requestServers()))
	if err != nil {
		t.Fatalf("décodage de la liste : %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("%d serveurs après un ajout, attendu 1 : %+v", len(servers), servers)
	}

	got := servers[0]
	t.Logf("serveur relu du démon : %+v", got)

	if got.IP != wantIP {
		t.Errorf("adresse = %q, attendu %q — quatre octets à l'endroit, "+
			"c'est ce que ce test existe pour vérifier", got.IP, wantIP)
	}
	if got.Port != wantPort {
		t.Errorf("port = %d, attendu %d", got.Port, wantPort)
	}
	if got.Name != wantName {
		t.Errorf("nom = %q, attendu %q", got.Name, wantName)
	}

	// Le démon n'écrit pas le tag de priorité quand elle est normale : c'est
	// exactement le cas d'un serveur ajouté à la main, et l'occasion de
	// vérifier que l'absence est bien traduite en « normale » et non en vide.
	if got.Priority != PriorityNormal {
		t.Errorf("priorité = %q, attendu %q pour un serveur fraîchement ajouté",
			got.Priority, PriorityNormal)
	}

	// Rien dans cette réponse ne désigne le serveur connecté. Le démon place
	// cette information ailleurs, et une liste qui l'affirmerait serait fausse.
	if got.Connected {
		t.Error("serveur marqué connecté alors que la liste ne porte jamais " +
			"cette information")
	}
}

// ─── État de connexion ───────────────────────────────────────────────────────

/*
TestIntegrationDecodeConnectionDeconnecte décode l'état d'un démon hors ligne.

Un état entièrement à faux ressemble à un décodage raté, et c'est précisément
pour cela qu'il faut le prouver contre un vrai démon : ici, les faux sont la
VÉRITÉ. Le conteneur n'a ni serveur, ni Kad, ni accès réseau.

Le point le plus important est `Kad.Running`. Le démon en fait un bit distinct
de « Kad connecté », et un décodeur qui déduirait l'un de l'autre — ou qui
supposerait qu'un Kad éteint est simplement « pas encore connecté » — rendrait
ici la même chose qu'un décodeur juste. On vérifie donc explicitement qu'il vaut
faux parce que le démon le dit, pas parce qu'on l'a supposé.

Ce test a établi un fait qu'aucune lecture de la spécification ne laissait
prévoir : un démon SANS Kad rend 0x08, c'est-à-dire « Kad derrière un
pare-feu ». Le moteur Kad n'existant pas, il n'a jamais rien reçu, et sa réponse
par défaut à « es-tu joignable ? » est « non ». On fige ce comportement ici :
c'est la preuve que le bit de pare-feu ne signifie rien tant que Kad ne tourne
pas, et le jour où une version d'aMule changera d'avis, ce test le dira.
*/
func TestIntegrationDecodeConnectionDeconnecte(t *testing.T) {
	env := amuletest.Start(t)
	conn := dialDaemon(t, env)

	resp := exchange(t, conn, requestConnection())

	// Le tag doit être là, et lisible comme entier : c'est ce qui distingue
	// « le démon dit tout à faux » de « on n'a rien su lire ».
	stateTag, ok := resp.Find(ec.TagConnstate)
	if !ok {
		t.Fatalf("pas de tag d'état dans la réponse %s", resp.Op)
	}
	raw, ok := stateTag.Uint()
	if !ok {
		t.Fatalf("champ de bits illisible (type %d)", stateTag.Type)
	}
	t.Logf("champ de bits rendu par le démon : 0x%02X", raw)

	// Le seul bit qu'un démon hors ligne pose. S'il en posait un autre — ou
	// plus celui-là — c'est que le protocole a bougé, et mieux vaut le voir
	// ici que dans un affichage.
	if raw != connStateKadFirewalled {
		t.Errorf("champ de bits = 0x%02X, attendu 0x%02X (« Kad pare-feu » seul) "+
			"pour un démon configuré sans eD2k et sans Kad",
			raw, connStateKadFirewalled)
	}

	state, err := decodeConnection(resp)
	if err != nil {
		t.Fatalf("décodage de l'état : %v", err)
	}

	want := Connection{
		Ed2k: Ed2kState{ID: IDNone},
		Kad:  KadState{Firewalled: true},
	}
	if state != want {
		t.Errorf("état = %+v\nattendu %+v", state, want)
	}

	// Redites délibérées : ces quatre-là sont les affirmations du test, et un
	// échec doit dire laquelle est fausse plutôt que d'afficher deux structures
	// à comparer à l'œil.
	if state.Kad.Running {
		t.Error("Kad annoncé démarré alors que le démon est configuré sans Kad")
	}
	if state.Kad.Connected {
		t.Error("Kad annoncé connecté alors qu'il ne tourne même pas")
	}
	if state.Ed2k.Connected || state.Ed2k.Connecting {
		t.Error("eD2k annoncé connecté ou en cours de connexion sur un démon hors ligne")
	}
	if state.Ed2k.Server != nil {
		t.Errorf("serveur joint inventé : %+v", *state.Ed2k.Server)
	}
}

/*
TestIntegrationDecodeConnectionSurviteALaReponseDeStatistiques.

Le démon place le même tag d'état dans plusieurs réponses : celle qu'il donne à
notre demande, et celles qu'il pousse de lui-même. C'est pourquoi le décodeur ne
vérifie aucun opcode. Ce test l'exerce sur la réponse d'un aller-retour de
statistiques, qui est le second chemin par lequel cet état nous parviendra quand
la scrutation existera.

Le tag n'y est pas garanti présent — le démon ne le pousse que lorsque l'état
CHANGE. Son absence est donc normale ici, et le test ne la traite pas comme un
échec ; ce qu'il vérifie est que s'il est là, il se lit.
*/
func TestIntegrationDecodeConnectionDansUneAutreReponse(t *testing.T) {
	env := amuletest.Start(t)
	conn := dialDaemon(t, env)

	resp := exchange(t, conn, ec.New(ec.OpStatReq))

	if _, ok := resp.Find(ec.TagConnstate); !ok {
		t.Skip("le démon n'a pas joint d'état à ses statistiques — il ne le " +
			"fait que sur changement")
	}

	state, err := decodeConnection(resp)
	if err != nil {
		t.Fatalf("décodage de l'état porté par une réponse %s : %v", resp.Op, err)
	}
	if state.Ed2k.Connected {
		t.Error("eD2k annoncé connecté sur un démon hors ligne")
	}
}
