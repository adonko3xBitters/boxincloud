package amule

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
)

/*
Les commandes : ce qui AGIT sur le démon.

Tout le reste du module lit. Ici on écrit, et cela change trois choses.

# Une commande emprunte la session de la scrutation

Ouvrir une connexion et s'authentifier coûte une centaine de millisecondes.
Les payer à chaque clic rendrait poussif un geste dont on attend qu'il soit
instantané. La session de la boucle est donc empruntée quand elle existe, et
`ec.Conn` sérialise lui-même les échanges : une commande envoyée pendant une
collecte attend son tour, sans risque de lire la réponse de l'autre.

Quand la boucle dort — personne ne regarde — la commande ouvre sa propre
session. Agir ne doit pas exiger d'avoir un onglet ouvert.

# Une commande réveille la scrutation

Sans cela, une mise en pause resterait invisible jusqu'à cinq secondes, et
l'utilisateur cliquerait une seconde fois en croyant que rien ne s'est passé.

# Le démon ne confirme rien

amuled répond à la plupart de ces opérations par un accusé vide. Il ne dit pas
« mis en pause », il dit « reçu ». La seule confirmation possible est l'état
suivant, d'où le réveil : c'est l'instantané qui fait foi, pas la réponse.
*/

// ErrInvalidHash signale une empreinte qui n'est pas une empreinte.
var ErrInvalidHash = errors.New("amule : empreinte eD2k invalide")

/*
parseHash valide une empreinte avant qu'elle ne parte sur le réseau.

Seize octets en hexadécimal, ni plus ni moins. Le contrôle est fait ICI plutôt
qu'au bord de l'API parce que toutes les commandes qui désignent un fichier en
dépendent, et qu'une empreinte tronquée produirait un tag mal formé — c'est-à-
dire une trame que le démon rejette, avec un message qui ne parle pas de
l'empreinte.
*/
func parseHash(hash string) ([]byte, error) {
	raw, err := hex.DecodeString(strings.TrimSpace(hash))
	if err != nil || len(raw) != 16 {
		return nil, fmt.Errorf("%w : %q", ErrInvalidHash, hash)
	}
	return raw, nil
}

/*
do exécute une commande sur le démon.

Emprunte la session de la scrutation, ou en ouvre une. Réveille la boucle après
coup, sauf si l'opération a échoué : rien n'a changé, il n'y a rien à
reconstater.
*/
func (s *Service) do(ctx context.Context, req ec.Packet) error {
	if !s.opts.Enabled {
		return ErrDisabled
	}

	s.session.mu.Lock()
	p := s.session.poller
	s.session.mu.Unlock()

	if p != nil {
		if conn := p.Session(); conn != nil {
			if _, err := conn.Do(ctx, req); err != nil {
				return err
			}
			p.Nudge()
			return nil
		}
	}

	// La boucle dort ou n'a pas de session : on ouvre la nôtre. Agir ne doit
	// pas exiger qu'un onglet soit ouvert quelque part.
	conn, err := serviceDialer{svc: s}.Open(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Do(ctx, req); err != nil {
		return err
	}
	if p != nil {
		p.Nudge()
	}
	return nil
}

// ─── Téléchargements ─────────────────────────────────────────────────────────

// DownloadAction est un geste sur un fichier en cours de réception.
type DownloadAction string

const (
	// DownloadPause suspend le téléchargement en gardant ses sources.
	DownloadPause DownloadAction = "pause"

	// DownloadResume le relance.
	DownloadResume DownloadAction = "resume"

	/*
		DownloadStop libère les sources en plus de suspendre.

		Différent d'une pause, et la nuance compte : reprendre après un arrêt
		fait tout rechercher, ce qui coûte plusieurs minutes avant que le
		transfert ne redémarre. Une pause, elle, repart tout de suite.
	*/
	DownloadStop DownloadAction = "stop"

	/*
		DownloadCancel abandonne le téléchargement et efface ce qui a été reçu.

		Irréversible. C'est la seule commande de ce fichier qui détruit quelque
		chose, et l'interface doit demander confirmation — le service, lui,
		obéit : il n'est pas le bon endroit pour discuter.
	*/
	DownloadCancel DownloadAction = "cancel"
)

var downloadOpcodes = map[DownloadAction]ec.Opcode{
	DownloadPause:  ec.OpPartfilePause,
	DownloadResume: ec.OpPartfileResume,
	DownloadStop:   ec.OpPartfileStop,
	DownloadCancel: ec.OpPartfileDelete,
}

// ActOnDownload applique un geste à un fichier.
func (s *Service) ActOnDownload(ctx context.Context, hash string, action DownloadAction) error {
	opcode, ok := downloadOpcodes[action]
	if !ok {
		return fmt.Errorf("amule : geste inconnu %q", action)
	}

	raw, err := parseHash(hash)
	if err != nil {
		return err
	}
	return s.do(ctx, ec.New(opcode, ec.Hash(ec.TagPartfile, raw)))
}

/*
priorityCodes est la table de mapPartfilePriority, dans l'autre sens.

Écrite à part plutôt qu'inversée par une boucle, pour une raison précise : la
lecture accepte le décalage « auto » — les codes 10 et au-delà — alors que
l'écriture ne doit envoyer que `prAuto`. Une inversion automatique produirait
une table qui sait rendre 11, ce qu'aucun démon n'attend en entrée.

Les deux tables doivent rester d'accord. Un test les compare, plutôt que de
faire confiance à la relecture.
*/
var priorityCodes = map[Priority]uint64{
	PriorityLow:      prLow,
	PriorityNormal:   prNormal,
	PriorityHigh:     prHigh,
	PriorityVeryHigh: prVeryHigh,
	PriorityVeryLow:  prVeryLow,
	PriorityAuto:     prAuto,
}

/*
SetDownloadPriority change la priorité d'un fichier.

`auto` laisse le démon décider : il monte la priorité d'un fichier presque fini
et abaisse celle d'un fichier sans source. C'est un mode, pas une valeur, d'où
son passage par la même énumération.
*/
func (s *Service) SetDownloadPriority(ctx context.Context, hash string, priority Priority) error {
	raw, err := parseHash(hash)
	if err != nil {
		return err
	}

	code, ok := priorityCodes[priority]
	if !ok {
		return fmt.Errorf("amule : priorité inconnue %q", priority)
	}

	tag := ec.Hash(ec.TagPartfile, raw)
	tag.Children = []ec.Tag{ec.Uint(ec.TagPartfilePrio, code)}

	return s.do(ctx, ec.New(ec.OpPartfilePrioSet, tag))
}

// ─── Serveurs ────────────────────────────────────────────────────────────────

/*
ConnectServer joint un serveur, ou laisse le démon choisir.

Une adresse vide demande une connexion automatique : le démon prend le premier
serveur de sa liste qui répond. C'est le geste courant — on veut être connecté,
rarement à un serveur précis.
*/
func (s *Service) ConnectServer(ctx context.Context, ip string, port int) error {
	if ip == "" {
		return s.do(ctx, ec.New(ec.OpConnect))
	}

	tag := ec.Text(ec.TagServer, fmt.Sprintf("%s:%d", ip, port))
	return s.do(ctx, ec.New(ec.OpServerConnect, tag))
}

// DisconnectServer quitte le serveur courant.
func (s *Service) DisconnectServer(ctx context.Context) error {
	return s.do(ctx, ec.New(ec.OpServerDisconnect))
}

// ─── Kad ─────────────────────────────────────────────────────────────────────

// StartKad démarre le moteur Kademlia.
//
// Démarrer ne veut pas dire connecter : Kad met plusieurs minutes à trouver ses
// pairs. L'interface distingue les deux états, et c'est pour cela qu'ils sont
// deux champs et non un.
func (s *Service) StartKad(ctx context.Context) error {
	return s.do(ctx, ec.New(ec.OpKadStart))
}

// StopKad arrête le moteur Kademlia.
func (s *Service) StopKad(ctx context.Context) error {
	return s.do(ctx, ec.New(ec.OpKadStop))
}

// ─── Liens ed2k:// ───────────────────────────────────────────────────────────

// ErrInvalidLink signale un lien qui n'en est pas un.
var ErrInvalidLink = errors.New("amule : lien ed2k:// invalide")

/*
AddLink met un lien ed2k:// en file.

Le contrôle porte sur le PRÉFIXE, et il est volontairement mince : la grammaire
complète des liens ed2k — fichiers, serveurs, listes de serveurs, collections —
est celle d'amuled, qui la connaît mieux que nous et la fait évoluer avec ses
versions. La revalider ici reviendrait à maintenir une seconde grammaire, qui
divergerait, et qui refuserait un jour un lien parfaitement valide.

Ce qu'on refuse en revanche tout de suite, c'est ce qui n'est manifestement pas
un lien ed2k : un magnet, une adresse HTTP, un chemin collé de travers. Le
démon les rejetterait aussi, mais son message ne dirait pas ce qu'on attendait.
*/
func (s *Service) AddLink(ctx context.Context, link string) error {
	link = strings.TrimSpace(link)

	if !strings.HasPrefix(strings.ToLower(link), "ed2k://") {
		return fmt.Errorf("%w : ce module ne traite que les liens ed2k://", ErrInvalidLink)
	}

	return s.do(ctx, ec.New(ec.OpAddLink, ec.Text(ec.TagString, link)))
}

// ─── Partage ─────────────────────────────────────────────────────────────────

// ReloadSharedFiles demande au démon de reparcourir ses répertoires partagés.
//
// Le parcours est asynchrone côté démon : la commande rend la main tout de
// suite, et la liste se met à jour au fil des instantanés suivants.
func (s *Service) ReloadSharedFiles(ctx context.Context) error {
	return s.do(ctx, ec.New(ec.OpSharedfilesReload))
}
