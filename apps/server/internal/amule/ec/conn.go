package ec

import (
	"bufio"
	"context"
	"crypto/md5" //nolint:gosec // imposé par le protocole d'amuled, pas un choix
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// Erreurs qu'un appelant a de bonnes raisons de distinguer.
var (
	// ErrAuthFailed — le démon a refusé nos identifiants.
	ErrAuthFailed = errors.New("ec : authentification refusée par le démon")

	// ErrProtocolVersion — le démon ne parle pas la version que nous parlons.
	ErrProtocolVersion = errors.New("ec : version de protocole incompatible")

	// ErrClosed — la connexion a été fermée.
	ErrClosed = errors.New("ec : connexion fermée")
)

/*
RefusedError : le démon a compris la commande et l'a rejetée.

Un type plutôt qu'une chaîne formatée, parce que le `Detail` doit ressortir
INTACT jusqu'à l'écran. « Kad is disabled in preferences » dit exactement quoi
faire ; noyé dans une phrase à nous, il faudrait le ré-extraire par découpage.

C'est un refus, pas une panne : la distinction porte le statut HTTP.
*/
type RefusedError struct {
	Op     string
	Detail string
}

func (e RefusedError) Error() string {
	if e.Detail == "" {
		return "le démon a refusé " + e.Op
	}
	return "le démon a refusé " + e.Op + " : " + e.Detail
}

var ()

// Conn est une session External Connections avec un démon aMule.
//
// Sûre d'emploi concurrent : les échanges sont sérialisés. Le protocole n'a
// aucun identifiant de corrélation — les réponses arrivent dans l'ordre des
// requêtes — et deux requêtes simultanées se voleraient donc mutuellement leur
// réponse.
type Conn struct {
	conn net.Conn
	r    *bufio.Reader

	mu   sync.Mutex
	auth bool

	// ServerVersion est la version annoncée par le démon à l'authentification.
	ServerVersion string
}

// Options règle l'établissement d'une session.
type Options struct {
	// ClientName et ClientVersion apparaissent dans le journal du démon, à la
	// ligne « Connecting client ». Les renseigner rend une instance
	// identifiable dans les journaux d'un serveur qui en accueille plusieurs.
	ClientName    string
	ClientVersion string

	// DialTimeout borne l'établissement TCP. Zéro prend dix secondes.
	DialTimeout time.Duration

	// RequestTimeout borne un aller-retour. Zéro prend trente secondes.
	RequestTimeout time.Duration
}

func (o Options) dialTimeout() time.Duration {
	if o.DialTimeout <= 0 {
		return 10 * time.Second
	}
	return o.DialTimeout
}

func (o Options) requestTimeout() time.Duration {
	if o.RequestTimeout <= 0 {
		return 30 * time.Second
	}
	return o.RequestTimeout
}

/*
Dial ouvre une session et s'authentifie.

Rien d'utile ne peut être demandé avant l'authentification : le démon refuse
tout autre paquet. Les deux gestes sont donc faits ensemble, plutôt que de
laisser exister une connexion à demi ouverte que chaque appelant devrait penser
à finir.

`password` est le mot de passe EN CLAIR. Il n'est jamais transmis : seul un
condensé salé traverse le réseau.
*/
func Dial(ctx context.Context, address, password string, opts Options) (*Conn, error) {
	dialer := net.Dialer{Timeout: opts.dialTimeout()}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("connexion à %s : %w", address, err)
	}

	c := &Conn{conn: conn, r: bufio.NewReader(conn)}

	if err := c.authenticate(ctx, password, opts); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return c, nil
}

/*
authenticate exécute la poignée de main en deux temps.

	nous → AuthReq        nom, version, version de protocole
	démon → AuthSalt      un sel, à usage unique
	nous → AuthPasswd     condensé du mot de passe salé
	démon → AuthOk        version du démon

Le sel rend inutile la capture de l'échange : le condensé transmis ne vaut que
pour cette connexion.

# Aucune capacité n'est annoncée, et c'est délibéré

Le protocole permet de négocier la compression zlib, un encodage compact des
entiers et un comptage étendu des tags. Rien de tout cela n'est demandé ici.

Sans négociation, les deux côtés s'en tiennent à des entiers de largeur fixe et
à des corps non compressés — c'est-à-dire au cas le plus simple, et au seul que
ce paquet sait lire aujourd'hui. Demander une capacité qu'on ne saurait pas
honorer produirait des trames illisibles à la première réponse volumineuse.

Ces capacités deviendront intéressantes quand les réponses porteront des
milliers de fichiers. Elles se négocieront alors, avec le code qui va avec.
*/
func (c *Conn) authenticate(ctx context.Context, password string, opts Options) error {
	name := opts.ClientName
	if name == "" {
		name = "boxincloud"
	}
	version := opts.ClientVersion
	if version == "" {
		version = "dev"
	}

	/*
		Aucun tag de version de build n'est envoyé, et c'est important.

		Un démon de version publiée REFUSE toute connexion qui en porte un : il
		y voit un client compilé depuis un instantané de développement, donc
		potentiellement incompatible au niveau binaire. Le refus est net et son
		message n'a rien d'évident quand on ne s'y attend pas.
	*/
	req := New(OpAuthReq,
		Text(TagClientName, name),
		Text(TagClientVersion, version),
		Uint(TagProtocolVersion, uint64(ProtocolVersion)),
	)

	resp, err := c.exchange(ctx, req, opts.requestTimeout())
	if err != nil {
		return err
	}

	switch resp.Op {
	case OpAuthSalt:
	case OpAuthFail:
		return authFailure(resp)
	default:
		return fmt.Errorf("réponse inattendue à l'authentification : %s", resp.Op)
	}

	saltTag, ok := resp.Find(TagPasswdSalt)
	if !ok {
		return errors.New("ec : le démon n'a pas fourni de sel d'authentification")
	}
	salt, ok := saltTag.Uint()
	if !ok {
		return fmt.Errorf("ec : sel d'authentification illisible (type %d)", saltTag.Type)
	}

	resp, err = c.exchange(ctx,
		New(OpAuthPasswd, Hash(TagPasswdHash, saltedDigest(password, salt))),
		opts.requestTimeout())
	if err != nil {
		return err
	}

	switch resp.Op {
	case OpAuthOk:
		c.mu.Lock()
		c.auth = true
		if v, ok := resp.Text(TagServerVersion); ok {
			c.ServerVersion = v
		}
		c.mu.Unlock()
		return nil

	case OpAuthFail:
		return authFailure(resp)

	default:
		return fmt.Errorf("réponse inattendue au mot de passe : %s", resp.Op)
	}
}

/*
saltedDigest calcule le condensé attendu par le démon.

	MD5( hex_minuscule(MD5(mot de passe)) + hex_minuscule(MD5(hex_MAJUSCULE(sel))) )

Trois détails, chacun capable de faire échouer l'authentification sans rien dire
d'autre que « mot de passe incorrect » :

  - le mot de passe est haché AVANT d'être salé — c'est cette empreinte, et non
    le mot de passe, qu'amuled conserve dans sa configuration ;
  - le sel est rendu en hexadécimal MAJUSCULE, sans zéros de tête ;
  - les deux condensés intermédiaires sont en hexadécimal MINUSCULE.

MD5 est imposé par le protocole. Ce n'est pas un choix de sécurité, et il n'y a
pas d'alternative : le démon d'en face ne sait faire que cela.
*/
func saltedDigest(password string, salt uint64) []byte {
	passwordHash := md5.Sum([]byte(password)) //nolint:gosec // imposé par le protocole

	// %X rend déjà des majuscules sans zéros de tête, ce qu'attend le démon.
	saltHash := md5.Sum([]byte(fmt.Sprintf("%X", salt))) //nolint:gosec // idem

	combined := md5.Sum([]byte( //nolint:gosec // idem
		hex.EncodeToString(passwordHash[:]) + hex.EncodeToString(saltHash[:])))

	return combined[:]
}

// authFailure traduit un refus en erreur lisible.
//
// Le démon joint souvent une explication en clair — version de protocole
// incompatible, mot de passe vide dans sa configuration. La perdre laisserait
// l'administrateur devant un « refusé » sans cause.
func authFailure(resp Packet) error {
	detail, _ := resp.Text(TagString)

	if strings.Contains(strings.ToLower(detail), "protocol version") {
		return fmt.Errorf("%w : %s (ce client parle 0x%04X)",
			ErrProtocolVersion, detail, ProtocolVersion)
	}
	if detail == "" {
		return ErrAuthFailed
	}
	return fmt.Errorf("%w : %s", ErrAuthFailed, detail)
}

/*
Do envoie une requête et rend la réponse.

Sérialisé : le protocole n'a pas d'identifiant de corrélation, et rien ne
permettrait d'attribuer deux réponses à deux requêtes parties ensemble.
*/
func (c *Conn) Do(ctx context.Context, req Packet) (Packet, error) {
	c.mu.Lock()
	authenticated := c.auth
	c.mu.Unlock()

	if !authenticated {
		return Packet{}, errors.New("ec : session non authentifiée")
	}
	return c.exchange(ctx, req, 30*time.Second)
}

// exchange fait un aller-retour, authentification comprise.
func (c *Conn) exchange(ctx context.Context, req Packet, timeout time.Duration) (Packet, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return Packet{}, ErrClosed
	}

	deadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := c.conn.SetDeadline(deadline); err != nil {
		return Packet{}, fmt.Errorf("échéance de connexion : %w", err)
	}

	body, err := req.encode()
	if err != nil {
		return Packet{}, err
	}
	if err := writeFrame(c.conn, flagBase, body); err != nil {
		return Packet{}, fmt.Errorf("envoi de %s : %w", req.Op, err)
	}

	// Le plafond dépend de l'état : tant que le démon ne nous a pas reconnus,
	// on lui accorde le même crédit qu'il nous accorde.
	maxBody := uint32(maxBodyPreAuth)
	if c.auth {
		maxBody = uint32(maxBodyPostAuth)
	}

	_, respBody, err := readFrame(c.r, maxBody)
	if err != nil {
		return Packet{}, fmt.Errorf("réception après %s : %w", req.Op, err)
	}

	resp, err := decodePacket(respBody)
	if err != nil {
		return Packet{}, fmt.Errorf("réponse à %s : %w", req.Op, err)
	}

	// OpFailed porte une explication du démon : la remonter évite de laisser
	// l'appelant deviner ce qui a déplu.
	if resp.Op == OpFailed {
		detail, _ := resp.Text(TagString)
		return resp, RefusedError{Op: req.Op.String(), Detail: detail}
	}

	return resp, nil
}

// Close ferme la session.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	c.auth = false
	return err
}
