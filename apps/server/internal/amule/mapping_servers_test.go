package amule

import (
	"testing"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
)

/*
Traduction des serveurs et de l'état de connexion, tag par tag.

Ces tests construisent les paquets à la main. Ils ne prouvent pas que le démon
parle bien ainsi — c'est le rôle du fichier d'intégration — mais ils couvrent ce
qu'un démon hors ligne ne produira jamais : un serveur connecté, un LowID, un
Kad pare-feu. Aucun de ces états n'est atteignable dans un conteneur de test, et
ce sont pourtant eux qui comptent en production.
*/

// ipv4Tag construit un tag dont la valeur est une adresse IPv4 et un port,
// exactement comme le démon les sérialise.
func ipv4Tag(name ec.TagName, a, b, c, d byte, port uint16, children ...ec.Tag) ec.Tag {
	return ec.Tag{
		Name:     name,
		Type:     ec.TypeIPv4,
		Value:    []byte{a, b, c, d, byte(port >> 8), byte(port)},
		Children: children,
	}
}

// connstateTag construit un tag d'état de connexion à partir du champ de bits.
func connstateTag(state uint64, children ...ec.Tag) ec.Tag {
	tag := ec.Uint(ec.TagConnstate, state)
	tag.Children = children
	return tag
}

// ─── Requêtes ────────────────────────────────────────────────────────────────

// TestMappingRequetesPortentLeNiveauDeDetail vérifie ce dont dépend tout le
// reste : sans niveau de détail complet, le démon réduit le serveur joint à un
// numéro interne et n'envoie ni description, ni nombre d'utilisateurs.
func TestMappingRequetesPortentLeNiveauDeDetail(t *testing.T) {
	cases := []struct {
		nom  string
		req  ec.Packet
		want ec.Opcode
	}{
		{"serveurs", requestServers(), ec.OpGetServerList},
		{"connexion", requestConnection(), ec.OpGetConnstate},
	}

	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			if tc.req.Op != tc.want {
				t.Errorf("opcode = %s, attendu %s", tc.req.Op, tc.want)
			}
			level, ok := tc.req.Uint(ec.TagDetailLevel)
			if !ok {
				t.Fatal("pas de niveau de détail dans la requête")
			}
			if level != uint64(ec.DetailFull) {
				t.Errorf("niveau de détail = %d, attendu %d (complet)",
					level, ec.DetailFull)
			}
		})
	}
}

// ─── Adresses ────────────────────────────────────────────────────────────────

/*
TestDecodeAdresseIPv4 fige la conversion d'adresse.

Les quatre octets d'une valeur IPv4 sont déjà dans l'ordre d'affichage, alors
que la même adresse transportée en ENTIER l'est à l'envers. Les deux chemins
doivent donner la même chaîne pour la même adresse : c'est la seule façon de
détecter qu'on a inversé l'un des deux.
*/
func TestDecodeAdresseIPv4(t *testing.T) {
	cases := []struct {
		nom      string
		octets   []byte
		wantIP   string
		wantPort int
	}{
		{"adresse publique", []byte{176, 103, 48, 36, 0x10, 0x58}, "176.103.48.36", 4184},
		{"boucle locale", []byte{127, 0, 0, 1, 0x12, 0x38}, "127.0.0.1", 4664},
		{"port maximal", []byte{10, 0, 0, 1, 0xFF, 0xFF}, "10.0.0.1", 65535},
		{"tout à zéro", []byte{0, 0, 0, 0, 0, 0}, "0.0.0.0", 0},
		{"tout à un", []byte{255, 255, 255, 255, 0, 1}, "255.255.255.255", 1},
	}

	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			ip, port, ok := serverAddressFromIPv4(ec.TypeIPv4, tc.octets)
			if !ok {
				t.Fatalf("adresse refusée : %v", tc.octets)
			}
			if ip != tc.wantIP || port != tc.wantPort {
				t.Errorf("adresse = %s:%d, attendu %s:%d", ip, port, tc.wantIP, tc.wantPort)
			}
		})
	}
}

// TestDecodeAdresseIPv4RefuseCeQuiNEnEstPas est le test qui protège du piège.
//
// Un entier de quatre octets a exactement la bonne taille pour passer pour une
// adresse. Sans contrôle de type, le numéro interne d'un serveur deviendrait
// une adresse plausible que rien ne signalerait comme fausse.
func TestDecodeAdresseIPv4RefuseCeQuiNEnEstPas(t *testing.T) {
	cases := []struct {
		nom    string
		typ    ec.TagType
		octets []byte
	}{
		{"entier de quatre octets", ec.TypeUint32, []byte{0, 0, 0, 42}},
		{"entier de six octets", ec.TypeUint64, []byte{1, 2, 3, 4, 5, 6}},
		{"IPv4 tronquée", ec.TypeIPv4, []byte{127, 0, 0, 1, 0x12}},
		{"valeur vide", ec.TypeIPv4, nil},
	}

	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			if ip, port, ok := serverAddressFromIPv4(tc.typ, tc.octets); ok {
				t.Errorf("accepté à tort : %s:%d", ip, port)
			}
		})
	}
}

// TestDecodeAdresseEntier couvre le repli par sous-tag, où l'adresse arrive
// sous forme d'entier avec l'octet de poids faible en tête.
func TestDecodeAdresseEntier(t *testing.T) {
	cases := []struct {
		nom    string
		valeur uint64
		want   string
	}{
		// 127.0.0.1 : le 127 est l'octet de poids faible.
		{"boucle locale", 0x0100007F, "127.0.0.1"},
		{"adresse publique", 0x243067B0, "176.103.48.36"},
		// Le démon écrit ses entiers sur le plus petit type qui les contienne :
		// cette adresse-là tient sur un seul octet.
		{"adresse tenant sur un octet", 0x01, "1.0.0.0"},
		{"nulle", 0x00, "0.0.0.0"},
	}

	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			if got := serverIPv4FromUint(tc.valeur); got != tc.want {
				t.Errorf("adresse = %s, attendu %s", got, tc.want)
			}
		})
	}
}

// ─── Liste de serveurs ───────────────────────────────────────────────────────

// TestDecodeServersEntreeComplete décode une entrée telle que le démon la
// produit au niveau de détail complet.
func TestDecodeServersEntreeComplete(t *testing.T) {
	p := ec.New(ec.OpServerList,
		ipv4Tag(ec.TagServer, 176, 103, 48, 36, 4184,
			ec.Text(ec.TagServerName, "eMule Security"),
			ec.Text(ec.TagServerDesc, "https://www.emule-security.org"),
			ec.Text(ec.TagServerVersion, "17.15"),
			ec.Uint(ec.TagServerPing, 42),
			ec.Uint(ec.TagServerUsers, 123456),
			ec.Uint(ec.TagServerUsersMax, 200000),
			ec.Uint(ec.TagServerFiles, 9876543),
			ec.Uint(ec.TagServerFailed, 3),
			ec.Uint(ec.TagServerStatic, 1),
			ec.Uint(ec.TagServerPrio, serverPrioHigh),
		),
	)

	servers, err := decodeServers(p)
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("%d serveurs, attendu 1", len(servers))
	}

	want := Server{
		IP:          "176.103.48.36",
		Port:        4184,
		Name:        "eMule Security",
		Description: "https://www.emule-security.org",
		Version:     "17.15",
		Ping:        42,
		Users:       123456,
		MaxUsers:    200000,
		Files:       9876543,
		Failed:      3,
		Static:      true,
		Priority:    PriorityHigh,
	}
	if servers[0] != want {
		t.Errorf("serveur = %+v\nattendu    %+v", servers[0], want)
	}
}

/*
TestDecodeServersTagsAbsents est le cas NORMAL, pas le cas dégradé.

Le démon omet tout ce qui vaut zéro : un serveur jamais joint n'a ni ping, ni
compteur d'échecs, ni description. Une entrée réduite à son adresse et à son nom
est ce qu'on reçoit le plus souvent, et elle doit se décoder sans bruit.

Le seul champ qui ne suive pas la règle du zéro est la priorité : son absence
signifie « normale », valeur que le démon n'écrit jamais.
*/
func TestDecodeServersTagsAbsents(t *testing.T) {
	p := ec.New(ec.OpServerList,
		ipv4Tag(ec.TagServer, 10, 0, 0, 1, 4661,
			ec.Text(ec.TagServerName, "serveur nu"),
		),
	)

	servers, err := decodeServers(p)
	if err != nil {
		t.Fatalf("un serveur sans détail a fait échouer le décodage : %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("%d serveurs, attendu 1", len(servers))
	}

	got := servers[0]
	if got.IP != "10.0.0.1" || got.Port != 4661 || got.Name != "serveur nu" {
		t.Errorf("identité perdue : %+v", got)
	}
	if got.Ping != 0 || got.Failed != 0 || got.Users != 0 || got.Static {
		t.Errorf("un tag absent a produit autre chose que le zéro : %+v", got)
	}
	if got.Priority != PriorityNormal {
		t.Errorf("priorité = %q, attendu %q — le démon omet le tag quand elle "+
			"est normale, l'absence n'est pas une inconnue",
			got.Priority, PriorityNormal)
	}
}

// TestDecodeServersRepliParSousTags couvre le niveau de détail incrémental, où
// l'adresse n'est plus dans la valeur du tag conteneur mais dans deux sous-tags
// entiers.
func TestDecodeServersRepliParSousTags(t *testing.T) {
	p := ec.New(ec.OpServerList,
		ec.Tag{
			Name: ec.TagServer,
			// Valeur du conteneur : le numéro interne du serveur, pas une
			// adresse. Le décodeur ne doit surtout pas la lire comme telle.
			Type:  ec.TypeUint32,
			Value: []byte{0x00, 0x00, 0x30, 0x39},
			Children: []ec.Tag{
				ec.Text(ec.TagServerName, "par sous-tags"),
				ec.Uint(ec.TagServerIP, 0x0100007F),
				ec.Uint(ec.TagServerPort, 4242),
			},
		},
	)

	servers, err := decodeServers(p)
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if servers[0].IP != "127.0.0.1" {
		t.Errorf("adresse = %q, attendu 127.0.0.1 — le numéro interne du "+
			"conteneur a probablement été pris pour une adresse", servers[0].IP)
	}
	if servers[0].Port != 4242 {
		t.Errorf("port = %d, attendu 4242", servers[0].Port)
	}
}

// TestDecodeServersListeVide vérifie qu'une liste vide se décode sans erreur.
//
// C'est l'état d'un démon neuf, et celui de tout démon dont le réseau eD2k est
// désactivé : il répond une liste vide sans regarder ce qu'il connaît.
func TestDecodeServersListeVide(t *testing.T) {
	servers, err := decodeServers(ec.New(ec.OpServerList))
	if err != nil {
		t.Fatalf("une liste vide a fait échouer le décodage : %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("%d serveurs pour une réponse vide", len(servers))
	}
	if servers == nil {
		t.Error("liste nil : un appelant qui itère dessus n'a pas à distinguer " +
			"vide et absent")
	}
}

// TestDecodeServersIgnoreLesTagsEtrangers vérifie qu'un tag qui n'est pas un
// serveur ne devient pas un serveur fantôme.
func TestDecodeServersIgnoreLesTagsEtrangers(t *testing.T) {
	p := ec.New(ec.OpServerList,
		ec.Text(ec.TagString, "un message quelconque"),
		ipv4Tag(ec.TagServer, 1, 2, 3, 4, 4661, ec.Text(ec.TagServerName, "vrai")),
	)

	servers, err := decodeServers(p)
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if len(servers) != 1 || servers[0].Name != "vrai" {
		t.Errorf("serveurs = %+v, attendu la seule entrée « vrai »", servers)
	}
}

// TestDecodeServersOpcodeInattendu vérifie qu'une réponse qui parle d'autre
// chose se signale.
//
// Le protocole n'a pas d'identifiant de corrélation : un décalage d'une trame
// donnerait ici une liste vide, indistinguable d'un démon sans serveur.
func TestDecodeServersOpcodeInattendu(t *testing.T) {
	if _, err := decodeServers(ec.New(ec.OpStats)); err == nil {
		t.Error("une réponse de statistiques a été acceptée comme liste de serveurs")
	}
}

// TestDecodeServersConserveLOrdre vérifie que l'ordre du démon survit.
func TestDecodeServersConserveLOrdre(t *testing.T) {
	p := ec.New(ec.OpServerList,
		ipv4Tag(ec.TagServer, 1, 1, 1, 1, 1, ec.Text(ec.TagServerName, "un")),
		ipv4Tag(ec.TagServer, 2, 2, 2, 2, 2, ec.Text(ec.TagServerName, "deux")),
		ipv4Tag(ec.TagServer, 3, 3, 3, 3, 3, ec.Text(ec.TagServerName, "trois")),
	)

	servers, err := decodeServers(p)
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	for i, want := range []string{"un", "deux", "trois"} {
		if servers[i].Name != want {
			t.Errorf("serveur %d = %q, attendu %q", i, servers[i].Name, want)
		}
	}
}

// ─── Priorités ───────────────────────────────────────────────────────────────

// TestDecodeServersPriorite fige la table, qui n'est PAS ordonnée : « haute »
// vaut 1 et « basse » vaut 2.
func TestDecodeServersPriorite(t *testing.T) {
	cases := []struct {
		code uint64
		want Priority
	}{
		{serverPrioNormal, PriorityNormal},
		{serverPrioHigh, PriorityHigh},
		{serverPrioLow, PriorityLow},
		// Le démon lui-même ramène à « normale » toute priorité hors table.
		{7, PriorityNormal},
		{255, PriorityNormal},
	}

	for _, tc := range cases {
		if got := serverPriority(tc.code); got != tc.want {
			t.Errorf("priorité du code %d = %q, attendu %q", tc.code, got, tc.want)
		}
	}
}

// ─── Champ de bits de l'état de connexion ────────────────────────────────────

/*
TestDecodeConnectionChampDeBits parcourt le champ de bits cas par cas.

Chaque bit est vérifié seul, puis en combinaison. Un masque décalé d'un cran
donnerait un état parfaitement cohérent et faux — « Kad démarré » au lieu de
« Kad pare-feu » — que rien d'autre ne rattraperait.

Le cas qui compte le plus est le dernier : Kad démarré SANS être connecté. C'est
l'état d'un Kad qui cherche ses pairs, il dure plusieurs minutes, et c'est
exactement là qu'une déduction de l'un vers l'autre mentirait.
*/
func TestDecodeConnectionChampDeBits(t *testing.T) {
	cases := []struct {
		nom   string
		state uint64
		want  Connection
	}{
		{
			nom:   "tout éteint",
			state: 0x00,
			want:  Connection{Ed2k: Ed2kState{ID: IDNone}},
		},
		{
			nom:   "eD2k connecté",
			state: connStateEd2kConnected,
			want:  Connection{Ed2k: Ed2kState{Connected: true, ID: IDNone}},
		},
		{
			nom:   "eD2k en cours de connexion",
			state: connStateEd2kConnecting,
			want:  Connection{Ed2k: Ed2kState{Connecting: true, ID: IDNone}},
		},
		{
			nom:   "Kad connecté",
			state: connStateKadConnected,
			want: Connection{
				Ed2k: Ed2kState{ID: IDNone},
				Kad:  KadState{Connected: true},
			},
		},
		{
			// Ce cas n'est pas théorique : c'est LITTÉRALEMENT ce qu'un démon
			// configuré sans Kad renvoie, et le test d'intégration le vérifie.
			// Un moteur Kad éteint n'a jamais reçu de connexion entrante, donc
			// il se déclare derrière un pare-feu. Le bit ne signifie rien tant
			// que Kad ne tourne pas.
			nom:   "Kad pare-feu, moteur éteint",
			state: connStateKadFirewalled,
			want: Connection{
				Ed2k: Ed2kState{ID: IDNone},
				Kad:  KadState{Firewalled: true},
			},
		},
		{
			nom:   "Kad démarré",
			state: connStateKadRunning,
			want: Connection{
				Ed2k: Ed2kState{ID: IDNone},
				Kad:  KadState{Running: true},
			},
		},
		{
			nom:   "Kad démarré mais pas encore connecté",
			state: connStateKadRunning,
			want: Connection{
				Ed2k: Ed2kState{ID: IDNone},
				Kad:  KadState{Running: true, Connected: false},
			},
		},
		{
			nom:   "Kad démarré et connecté",
			state: connStateKadRunning | connStateKadConnected,
			want: Connection{
				Ed2k: Ed2kState{ID: IDNone},
				Kad:  KadState{Running: true, Connected: true},
			},
		},
		{
			nom:   "Kad démarré, connecté, derrière un pare-feu",
			state: connStateKadRunning | connStateKadConnected | connStateKadFirewalled,
			want: Connection{
				Ed2k: Ed2kState{ID: IDNone},
				Kad:  KadState{Running: true, Connected: true, Firewalled: true},
			},
		},
		{
			nom:   "les deux réseaux à plein",
			state: connStateEd2kConnected | connStateKadConnected | connStateKadRunning,
			want: Connection{
				Ed2k: Ed2kState{Connected: true, ID: IDNone},
				Kad:  KadState{Running: true, Connected: true},
			},
		},
		{
			nom:   "eD2k connecté pendant que Kad démarre",
			state: connStateEd2kConnected | connStateKadRunning,
			want: Connection{
				Ed2k: Ed2kState{Connected: true, ID: IDNone},
				Kad:  KadState{Running: true},
			},
		},
		{
			// Les bits au-delà du cinquième ne sont pas attribués. Un démon
			// plus récent pourrait en poser ; ils ne doivent contaminer aucun
			// des champs connus.
			nom:   "bits inconnus posés",
			state: 0xE0,
			want:  Connection{Ed2k: Ed2kState{ID: IDNone}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			conn, err := decodeConnection(ec.New(ec.OpMiscData, connstateTag(tc.state)))
			if err != nil {
				t.Fatalf("décodage : %v", err)
			}
			if conn != tc.want {
				t.Errorf("état (champ de bits 0x%02X) = %+v\nattendu%+v",
					tc.state, conn, tc.want)
			}
		})
	}
}

// ─── Identifiant eD2k ────────────────────────────────────────────────────────

/*
TestDecodeConnectionTypeDIdentifiant fige le seuil LowID/HighID.

Le seuil est une frontière stricte : le dernier LowID est 0x00FFFFFF, le premier
HighID est 0x01000000. Une comparaison en « inférieur ou égal » au lieu de
« strictement inférieur » ferait basculer un HighID entier du mauvais côté, et
l'utilisateur verrait un diagnostic de connectivité inversé.
*/
func TestDecodeConnectionTypeDIdentifiant(t *testing.T) {
	cases := []struct {
		nom  string
		id   uint64
		want IDType
	}{
		{"identifiant nul", 0, IDLow},
		{"petit identifiant", 42, IDLow},
		{"dernier LowID", 0x00FFFFFF, IDLow},
		{"premier HighID", 0x01000000, IDHigh},
		{"adresse routable", 0x0100007F, IDHigh},
		{"identifiant maximal", 0xFFFFFFFF, IDHigh},
	}

	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			conn, err := decodeConnection(ec.New(ec.OpMiscData,
				connstateTag(connStateEd2kConnected,
					ec.Uint(ec.TagEd2kID, tc.id),
				),
			))
			if err != nil {
				t.Fatalf("décodage : %v", err)
			}
			if conn.Ed2k.ID != tc.want {
				t.Errorf("identifiant 0x%08X classé %q, attendu %q",
					tc.id, conn.Ed2k.ID, tc.want)
			}
			if uint64(conn.Ed2k.ClientID) != tc.id {
				t.Errorf("identifiant = 0x%08X, attendu 0x%08X",
					conn.Ed2k.ClientID, tc.id)
			}
		})
	}
}

/*
TestDecodeConnectionIdentifiantIgnoreLeBouchon protège du faux HighID.

Pendant la poignée de main avec un serveur, le démon envoie 0xFFFFFFFF dans le
tag d'identifiant. Cette valeur est au-dessus du seuil : lue sans précaution,
elle afficherait un HighID triomphant alors qu'aucun serveur n'a encore rien
attribué.
*/
func TestDecodeConnectionIdentifiantIgnoreLeBouchon(t *testing.T) {
	conn, err := decodeConnection(ec.New(ec.OpMiscData,
		connstateTag(connStateEd2kConnecting,
			ec.Uint(ec.TagEd2kID, 0xFFFFFFFF),
		),
	))
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}

	if conn.Ed2k.ID != IDNone {
		t.Errorf("type d'identifiant = %q pendant une connexion en cours, "+
			"attendu %q", conn.Ed2k.ID, IDNone)
	}
	if conn.Ed2k.ClientID != 0 {
		t.Errorf("identifiant = 0x%08X, attendu 0 — 0xFFFFFFFF est un bouchon",
			conn.Ed2k.ClientID)
	}
	if !conn.Ed2k.Connecting {
		t.Error("l'état « en cours de connexion » a été perdu")
	}
}

// ─── Serveur imbriqué ────────────────────────────────────────────────────────

/*
TestDecodeConnectionServeurImbrique vérifie la remontée du serveur joint.

Le serveur auquel on est connecté n'apparaît pas dans la liste des serveurs :
il est en SOUS-TAG de l'état de connexion. C'est le seul endroit du protocole
qui dise « celui-ci », et c'est aussi le seul endroit d'où `Connected` peut
venir.
*/
func TestDecodeConnectionServeurImbrique(t *testing.T) {
	conn, err := decodeConnection(ec.New(ec.OpMiscData,
		connstateTag(connStateEd2kConnected,
			ipv4Tag(ec.TagServer, 176, 103, 48, 36, 4184,
				ec.Text(ec.TagServerName, "eMule Security"),
				ec.Uint(ec.TagServerUsers, 1000),
				ec.Uint(ec.TagServerPrio, serverPrioLow),
			),
			ec.Uint(ec.TagEd2kID, 0x0100007F),
		),
	))
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}

	if conn.Ed2k.Server == nil {
		t.Fatal("serveur joint absent alors qu'il est dans le paquet")
	}
	got := *conn.Ed2k.Server

	if got.IP != "176.103.48.36" || got.Port != 4184 {
		t.Errorf("adresse = %s:%d, attendu 176.103.48.36:4184", got.IP, got.Port)
	}
	if got.Name != "eMule Security" || got.Users != 1000 {
		t.Errorf("serveur mal décodé : %+v", got)
	}
	if got.Priority != PriorityLow {
		t.Errorf("priorité = %q, attendu %q", got.Priority, PriorityLow)
	}
	if !got.Connected {
		t.Error("le serveur joint n'est pas marqué connecté — c'est pourtant " +
			"la seule chose que ce sous-tag apprend de plus que la liste")
	}
}

// TestDecodeConnectionSansServeur vérifie qu'un état déconnecté ne fabrique pas
// de serveur.
func TestDecodeConnectionSansServeur(t *testing.T) {
	conn, err := decodeConnection(ec.New(ec.OpMiscData, connstateTag(0)))
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if conn.Ed2k.Server != nil {
		t.Errorf("serveur inventé alors qu'aucun n'est joint : %+v", *conn.Ed2k.Server)
	}
}

/*
TestDecodeConnectionServeurReduitAUnNumero couvre le niveau de détail
incrémental.

Le sous-tag s'y réduit au numéro interne du serveur chez le démon : ni adresse,
ni nom. Ce numéro ne survit pas au redémarrage du démon et ne signifie rien
ailleurs. Remonter un `Server` qui ne dirait que « connecté » ferait afficher
une ligne vide dans l'interface ; on préfère ne rien affirmer.
*/
func TestDecodeConnectionServeurReduitAUnNumero(t *testing.T) {
	conn, err := decodeConnection(ec.New(ec.OpMiscData,
		connstateTag(connStateEd2kConnected,
			ec.Uint(ec.TagServer, 12345),
		),
	))
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if conn.Ed2k.Server != nil {
		t.Errorf("serveur remonté à partir d'un simple numéro interne : %+v",
			*conn.Ed2k.Server)
	}
	if !conn.Ed2k.Connected {
		t.Error("l'état connecté a été perdu avec le serveur")
	}
}

// ─── Pare-feu UDP ────────────────────────────────────────────────────────────

/*
TestDecodeConnectionPareFeuUDP couvre le champ qui n'est PAS dans le champ de
bits.

Le pare-feu UDP de Kad n'a pas de bit d'état : le démon le transmet avec ses
statistiques. Quand le tag d'état voyage dans une réponse de statistiques — ce
que fait le démon lorsqu'il pousse un changement de lui-même — l'information est
juste à côté, et on la prend.
*/
func TestDecodeConnectionPareFeuUDP(t *testing.T) {
	t.Run("présent dans le paquet", func(t *testing.T) {
		conn, err := decodeConnection(ec.New(ec.OpStats,
			connstateTag(connStateKadRunning|connStateKadConnected),
			ec.Uint(ec.TagStatsKadFirewalledUDP, 1),
		))
		if err != nil {
			t.Fatalf("décodage : %v", err)
		}
		if !conn.Kad.FirewalledUDP {
			t.Error("pare-feu UDP non repris alors que le paquet le porte")
		}
		if conn.Kad.Firewalled {
			t.Error("le pare-feu UDP a contaminé le pare-feu TCP : les deux " +
				"sont distincts, Kad cherche en UDP et transfère en TCP")
		}
	})

	t.Run("absent du paquet", func(t *testing.T) {
		conn, err := decodeConnection(ec.New(ec.OpMiscData,
			connstateTag(connStateKadRunning|connStateKadFirewalled),
		))
		if err != nil {
			t.Fatalf("décodage : %v", err)
		}
		if conn.Kad.FirewalledUDP {
			t.Error("pare-feu UDP affirmé alors que rien ne le dit")
		}
		if !conn.Kad.Firewalled {
			t.Error("pare-feu TCP perdu")
		}
	})
}

// ─── Erreurs ─────────────────────────────────────────────────────────────────

/*
TestDecodeConnectionTagAbsent vérifie qu'un paquet sans état d'état se signale.

C'est la seule absence qui soit une erreur dans ce fichier, et pour une raison
précise : rendre un `Connection` vide ferait passer « je n'ai pas su lire » pour
« tout est déconnecté », ce qui est un diagnostic, pas une panne.
*/
func TestDecodeConnectionTagAbsent(t *testing.T) {
	if _, err := decodeConnection(ec.New(ec.OpMiscData)); err == nil {
		t.Error("un paquet sans tag d'état a été accepté")
	}
}

// TestDecodeConnectionChampDeBitsIllisible vérifie qu'un tag d'état d'un type
// inattendu se signale plutôt que de valoir zéro.
func TestDecodeConnectionChampDeBitsIllisible(t *testing.T) {
	_, err := decodeConnection(ec.New(ec.OpMiscData,
		ec.Text(ec.TagConnstate, "pas un entier"),
	))
	if err == nil {
		t.Error("un champ de bits textuel a été accepté")
	}
}

// TestDecodeConnectionAccepteToutOpcode vérifie que l'état se décode d'où qu'il
// vienne.
//
// Le démon place ce tag dans sa réponse à notre demande explicite, mais aussi
// dans les notifications qu'il pousse de lui-même. Exiger un opcode précis
// interdirait le second usage — celui de la scrutation.
func TestDecodeConnectionAccepteToutOpcode(t *testing.T) {
	for _, op := range []ec.Opcode{ec.OpMiscData, ec.OpStats} {
		conn, err := decodeConnection(ec.New(op, connstateTag(connStateEd2kConnected)))
		if err != nil {
			t.Errorf("paquet %s refusé : %v", op, err)
			continue
		}
		if !conn.Ed2k.Connected {
			t.Errorf("paquet %s : état mal lu", op)
		}
	}
}
