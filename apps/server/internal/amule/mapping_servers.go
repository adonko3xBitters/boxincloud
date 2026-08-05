package amule

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
)

/*
Traduction des réponses EC qui décrivent les SERVEURS eD2k et l'ÉTAT DE
CONNEXION.

Ce sont deux faces d'une même chose : la liste de ce que le démon connaît, et le
lien qu'il entretient avec l'un d'eux. Le second est d'ailleurs imbriqué dans le
premier — le serveur joint arrive en sous-tag de l'état de connexion, pas dans
la liste.

Deux règles valent pour tout le fichier :

  - Un tag ABSENT n'est jamais une erreur. amuled omet systématiquement ce qui
    vaut zéro : ping jamais mesuré, aucun échec, serveur non épinglé,
    description vide. Traiter l'omission comme un défaut ferait échouer le
    décodage sur la majorité des entrées d'une liste normale.

  - Une exception à la règle du zéro : la PRIORITÉ. amuled n'écrit le tag que
    si elle DIFFÈRE de « normale ». L'absence y signifie donc « normale », et
    non « inconnue » — laisser la chaîne vide afficherait un blanc là où le
    démon dit quelque chose de précis.
*/

// ─── Le champ de bits de l'état de connexion ─────────────────────────────────

/*
Masques du tag d'état de connexion.

L'état entier tient dans UN SEUL octet, construit d'un bloc par le démon
(ECSpecialCoreTags.cpp, constructeur de CEC_ConnState_Tag ; les mêmes masques
sont relus côté client dans ECSpecialTags.h). D'où deux conséquences qu'on ne
devine pas :

  - « Kad connecté » et « Kad démarré » sont des bits SÉPARÉS et indépendants.
    Un Kad qui vient de démarrer cherche ses pairs pendant plusieurs minutes :
    démarré sans être connecté. Déduire l'un de l'autre inventerait un état.

  - Il n'y a PAS de bit pour le pare-feu UDP. Cette information voyage
    ailleurs, dans les statistiques (TagStatsKadFirewalledUDP) — voir
    decodeConnection, qui la récupère si le paquet la porte.

  - Le bit « Kad pare-feu » vaut UN quand Kad est ÉTEINT. Ce n'est pas une
    anomalie du démon : tant que le moteur Kad n'existe pas, il n'a jamais reçu
    la moindre connexion entrante, et sa réponse par défaut à « es-tu
    joignable ? » est donc « non ». Un démon configuré sans Kad rend
    littéralement 0x08 — c'est vérifié en intégration. Ce bit ne veut donc rien
    dire tant que Kad ne tourne pas, et le client officiel d'aMule ne le
    consulte d'ailleurs QUE lorsque Kad est connecté. Nous le transcrivons tel
    quel plutôt que de le masquer — masquer serait décider à la place de
    l'affichage — mais quiconque en fait un voyant doit le conditionner à
    `Kad.Connected`.
*/
const (
	connStateEd2kConnected  uint64 = 0x01 // session ouverte avec un serveur
	connStateEd2kConnecting uint64 = 0x02 // poignée de main en cours
	connStateKadConnected   uint64 = 0x04 // Kad a trouvé le réseau
	connStateKadFirewalled  uint64 = 0x08 // Kad ne reçoit pas les connexions entrantes
	connStateKadRunning     uint64 = 0x10 // le moteur Kad tourne
)

/*
highestLowID est la borne qui sépare un LowID d'un HighID.

Un identifiant STRICTEMENT INFÉRIEUR à cette valeur est un LowID ; autrement
dit tout ce qui tient sur 0x00FFFFFF ou moins. Ce n'est pas une convention
locale : c'est le serveur qui attribue un identifiant dans cette plage quand il
n'a pas réussi à joindre le client en retour, et les deux réseaux — eD2k comme
Kad — s'accordent sur le même seuil.
*/
const highestLowID uint64 = 0x01000000

// Codes de priorité d'un serveur, tels que le démon les numérote.
//
// La numérotation n'est pas ordonnée : « haute » vaut 1 et « basse » vaut 2.
// Trier sur l'entier donnerait donc un classement faux.
const (
	serverPrioNormal uint64 = 0
	serverPrioHigh   uint64 = 1
	serverPrioLow    uint64 = 2
)

// ─── Serveurs eD2k ───────────────────────────────────────────────────────────

/*
requestServers construit la demande de la liste des serveurs connus.

Le niveau de détail est envoyé explicitement. Le démon prend « complet » par
défaut quand le tag manque, mais ce défaut est une valeur de SON code : l'écrire
nous met à l'abri du jour où il changerait, et dit au lecteur ce qu'on attend.

Attention à un court-circuit du démon : si le réseau eD2k est désactivé dans ses
préférences, il répond une liste VIDE sans même regarder ce qu'il connaît. Une
liste vide ne prouve donc pas que l'utilisateur n'a aucun serveur.
*/
func requestServers() ec.Packet {
	return ec.New(ec.OpGetServerList,
		ec.Uint(ec.TagDetailLevel, uint64(ec.DetailFull)),
	)
}

/*
decodeServers traduit la réponse en liste de serveurs.

L'ordre du démon est conservé : c'est celui de son fichier server.met, et le
réordonner ici priverait l'interface de la seule notion d'ordre qui existe.

Ce que cette réponse NE PORTE PAS : quel serveur est le serveur connecté. Le
démon ne marque rien dans cette liste, il place le serveur joint en sous-tag de
l'état de connexion. `Connected` reste donc faux partout ici, et c'est
`decodeConnection` qui porte l'information ; le rapprochement des deux — par
adresse IP et port, les seuls identifiants stables d'un redémarrage à l'autre —
se fait à l'étage au-dessus, là où l'on tient les deux réponses à la fois.
*/
func decodeServers(p ec.Packet) ([]Server, error) {
	// Le protocole n'a aucun identifiant de corrélation : si les réponses se
	// décalaient d'un cran, on décoderait ici un paquet qui parle d'autre
	// chose. Un opcode inattendu se signale, il ne se traverse pas en rendant
	// une liste vide qui passerait pour « aucun serveur ».
	if p.Op != ec.OpServerList {
		return nil, fmt.Errorf(
			"liste de serveurs : réponse %s, attendu %s", p.Op, ec.OpServerList)
	}

	servers := make([]Server, 0, len(p.Tags))
	for _, tag := range p.Tags {
		if tag.Name != ec.TagServer {
			continue
		}
		servers = append(servers, decodeServer(tag))
	}
	return servers, nil
}

/*
decodeServer traduit un tag de serveur.

L'adresse ne se lit pas dans un sous-tag mais dans la VALEUR PROPRE du tag
conteneur, encodée en IPv4 : quatre octets d'adresse dans l'ordre de la notation
pointée, puis le port sur deux octets en gros-boutiste. Le piège est qu'à
d'autres niveaux de détail cette même valeur porte un simple entier — le numéro
interne du serveur chez le démon — qui parserait en une adresse plausible et
fausse. D'où le contrôle du TYPE avant toute lecture, et non de la longueur.

Les sous-tags d'adresse servent de repli. Ils n'existent qu'au niveau de détail
incrémental, où ils portent l'adresse sous forme d'entier.
*/
func decodeServer(tag ec.Tag) Server {
	server := Server{
		// La priorité normale est le défaut du démon, pas une supposition :
		// il n'écrit le tag que pour les deux autres valeurs.
		Priority: PriorityNormal,
	}

	if ip, port, ok := serverAddressFromIPv4(tag.Type, tag.Value); ok {
		server.IP = ip
		server.Port = port
	}

	for _, child := range tag.Children {
		switch child.Name {
		case ec.TagServerName:
			server.Name, _ = child.Text()
		case ec.TagServerDesc:
			server.Description, _ = child.Text()
		case ec.TagServerVersion:
			server.Version, _ = child.Text()

		case ec.TagServerIP:
			// Repli du niveau incrémental. Le tag peut arriver en IPv4 comme
			// en entier selon le chemin d'écriture ; les deux se ramènent à la
			// même notation pointée.
			if ip, _, ok := serverAddressFromIPv4(child.Type, child.Value); ok {
				server.IP = ip
			} else if v, ok := child.Uint(); ok {
				server.IP = serverIPv4FromUint(v)
			}
		case ec.TagServerPort:
			if v, ok := child.Uint(); ok {
				server.Port = int(v)
			}

		case ec.TagServerPing:
			if v, ok := child.Uint(); ok {
				server.Ping = int(v)
			}
		case ec.TagServerUsers:
			if v, ok := child.Uint(); ok {
				server.Users = int(v)
			}
		case ec.TagServerUsersMax:
			if v, ok := child.Uint(); ok {
				server.MaxUsers = int(v)
			}
		case ec.TagServerFiles:
			if v, ok := child.Uint(); ok {
				server.Files = int(v)
			}
		case ec.TagServerFailed:
			if v, ok := child.Uint(); ok {
				server.Failed = int(v)
			}

		case ec.TagServerStatic:
			if v, ok := child.Uint(); ok {
				server.Static = v != 0
			}
		case ec.TagServerPrio:
			if v, ok := child.Uint(); ok {
				server.Priority = serverPriority(v)
			}
		}
	}

	return server
}

// ─── État de connexion ───────────────────────────────────────────────────────

// requestConnection construit la demande de l'état de connexion.
//
// Le niveau de détail compte ici plus qu'ailleurs : c'est lui qui décide si le
// serveur joint arrive en entier ou réduit à son numéro interne, lequel ne
// signifie rien hors du démon et ne survit pas à son redémarrage.
func requestConnection() ec.Packet {
	return ec.New(ec.OpGetConnstate,
		ec.Uint(ec.TagDetailLevel, uint64(ec.DetailFull)),
	)
}

/*
decodeConnection traduit l'état des deux réseaux.

L'opcode du paquet n'est délibérément PAS vérifié. Le démon place ce même tag
dans deux réponses différentes : celle qu'il donne à notre demande explicite, et
celle qu'il pousse de lui-même quand l'état change. Exiger un opcode précis
interdirait le second usage, qui est justement celui de la scrutation.

En revanche, l'absence du tag d'état EST une erreur : sans lui il n'y a rien à
lire, et rendre un `Connection` vide ferait passer « je n'ai pas su lire » pour
« tout est déconnecté ».
*/
func decodeConnection(p ec.Packet) (Connection, error) {
	tag, ok := p.Find(ec.TagConnstate)
	if !ok {
		return Connection{}, fmt.Errorf(
			"état de connexion : paquet %s sans tag %s", p.Op, ec.TagConnstate)
	}

	state, ok := tag.Uint()
	if !ok {
		return Connection{}, fmt.Errorf(
			"état de connexion : champ de bits illisible (type %d)", tag.Type)
	}

	conn := Connection{
		Ed2k: Ed2kState{
			Connected:  state&connStateEd2kConnected != 0,
			Connecting: state&connStateEd2kConnecting != 0,
			ID:         IDNone,
		},
		Kad: KadState{
			Running:    state&connStateKadRunning != 0,
			Connected:  state&connStateKadConnected != 0,
			Firewalled: state&connStateKadFirewalled != 0,
		},
	}

	/*
		Le pare-feu UDP de Kad n'a pas de bit dans ce champ : le démon le
		transmet avec ses statistiques. Quand le tag d'état voyage DANS une
		réponse de statistiques — c'est le cas des notifications spontanées —
		l'information est là, à côté, et il serait absurde de la laisser.
		Sinon le champ reste faux, ce qui est le cas par défaut : « pas de
		pare-feu UDP constaté ».
	*/
	if v, ok := p.Uint(ec.TagStatsKadFirewalledUDP); ok {
		conn.Kad.FirewalledUDP = v != 0
	}

	/*
		Le serveur joint est un SOUS-TAG de l'état, pas une entrée marquée dans
		la liste des serveurs. On le remonte tel quel, en posant le drapeau que
		la liste ne porte jamais.

		Au niveau de détail incrémental ce sous-tag se réduit au numéro interne
		du serveur : ni adresse, ni nom. Un `Server` qui ne dirait que
		« connecté » vaudrait moins que rien dans une interface — on préfère
		alors ne rien remonter et laisser `Server` à nil, ce qui est la réponse
		honnête à « lequel ? ».
	*/
	if srvTag, ok := tag.Find(ec.TagServer); ok {
		if server := decodeServer(srvTag); server.IP != "" || server.Name != "" {
			server.Connected = true
			conn.Ed2k.Server = &server
		}
	}

	/*
		L'identifiant eD2k n'a de sens que connecté.

		Pendant la poignée de main, le démon envoie 0xFFFFFFFF — un bouchon, pas
		un identifiant. Le laisser passer produirait un HighID affiché avec
		aplomb alors qu'aucun serveur ne nous a encore rien attribué. On ne lit
		donc ce tag que si la connexion est établie ; sinon l'identifiant reste
		à zéro et le type à « aucun ».
	*/
	if conn.Ed2k.Connected {
		if v, ok := tag.Find(ec.TagEd2kID); ok {
			if id, ok := v.Uint(); ok {
				conn.Ed2k.ClientID = uint32(id) //nolint:gosec // identifiant eD2k, 32 bits par construction
				conn.Ed2k.ID = idType(id)
			}
		}
	}

	return conn, nil
}

// ─── Traductions élémentaires ────────────────────────────────────────────────

// idType classe un identifiant eD2k.
//
// Un LowID n'est pas une erreur de configuration en soi : il dit seulement que
// le serveur n'a pas réussi à ouvrir une connexion vers le client. C'est
// pourtant la cause première d'un aMule qui « ne trouve pas de sources », et
// c'est pour cela que la distinction remonte jusqu'à l'interface.
func idType(id uint64) IDType {
	if id < highestLowID {
		return IDLow
	}
	return IDHigh
}

// serverPriority traduit le code de priorité d'un serveur.
//
// Un code hors table retombe sur « normale ». C'est ce que fait le démon
// lui-même à la lecture de sa liste : une priorité qu'il ne reconnaît pas est
// ramenée à la valeur normale plutôt que rejetée.
func serverPriority(code uint64) Priority {
	switch code {
	case serverPrioLow:
		return PriorityLow
	case serverPrioHigh:
		return PriorityHigh
	case serverPrioNormal:
		return PriorityNormal
	default:
		return PriorityNormal
	}
}

/*
serverAddressFromIPv4 lit une valeur de tag encodée en IPv4.

Six octets : quatre d'adresse, puis le port en gros-boutiste. Les quatre octets
d'adresse sont déjà dans l'ordre de la notation pointée — le premier est le
premier nombre affiché — ce qui rend la conversion directe.

Le contrôle porte sur le TYPE et pas seulement sur la longueur : un entier de
quatre octets a la bonne taille pour être pris à tort pour une adresse, et
donnerait un résultat plausible qu'aucun test ne rattraperait.

Le codec ne fournit pas d'accesseur pour ce type — il n'en avait pas l'usage —
et le décodage se fait donc ici.
*/
func serverAddressFromIPv4(t ec.TagType, value []byte) (string, int, bool) {
	const ipv4Len = 4 + 2

	if t != ec.TypeIPv4 || len(value) < ipv4Len {
		return "", 0, false
	}

	ip := netip.AddrFrom4([4]byte(value[0:4])).String()
	port := int(binary.BigEndian.Uint16(value[4:6]))
	return ip, port, true
}

/*
serverIPv4FromUint rend la notation pointée d'une adresse transportée en entier.

L'octet de POIDS FAIBLE est le premier nombre de la notation pointée, à
l'inverse de ce qu'on écrirait spontanément : le démon manipule ses adresses
sous forme d'octets réseau réinterprétés en entier, et c'est cette convention
qu'il sérialise. Prendre l'ordre inverse rendrait des adresses retournées —
« 1.0.0.127 » pour « 127.0.0.1 » — donc jamais rapprochées de rien.

La largeur du tag ne dit rien : le démon écrit ses entiers sur le plus petit
type qui les contienne, et « 1.0.0.0 » arrive donc sur un seul octet.
*/
func serverIPv4FromUint(v uint64) string {
	addr := netip.AddrFrom4([4]byte{
		byte(v),
		byte(v >> 8),
		byte(v >> 16),
		byte(v >> 24),
	})
	return addr.String()
}
