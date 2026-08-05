package amule

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
)

/*
Ping décrit ce qu'un aller-retour apprend du démon.

Volontairement mince à cette étape : ce qui compte est de prouver que la chaîne
complète fonctionne — adresse, mot de passe, poignée de main, version de
protocole, échange authentifié. La lecture de l'état réel du moteur, de la file
et des serveurs vient avec les étapes qui l'affichent.
*/
type Ping struct {
	// Address est l'adresse effectivement jointe, telle que le serveur la voit.
	Address string

	// ServerVersion est la version annoncée par le démon.
	ServerVersion string

	// ProtocolVersion est la version parlée par ce client. Le démon exige la
	// même, à l'identique.
	ProtocolVersion uint16

	// Handshake est le temps d'établissement complet : TCP, sel, mot de passe.
	Handshake time.Duration

	// RoundTrip est le temps d'un aller-retour APRÈS authentification. C'est
	// lui qui mesure le coût réel d'une requête, la poignée de main n'ayant
	// lieu qu'une fois.
	RoundTrip time.Duration
}

/*
Ping joint le démon, s'authentifie, et fait un aller-retour.

Ouvre une connexion neuve et la referme : c'est un diagnostic, et il doit
mesurer ce que vit une connexion qui part de rien. La session durable qui
alimentera l'interface est un autre objet, et elle arrive avec la scrutation.

L'aller-retour post-authentification n'est pas une politesse. Sans lui, on
prouverait seulement que le mot de passe est bon — or un démon peut accepter
l'authentification puis refuser tout le reste, par exemple si les versions
divergent sur un point que la poignée de main ne couvre pas.
*/
func (s *Service) Ping(ctx context.Context) (Ping, error) {
	if !s.opts.Enabled {
		return Ping{}, ErrDisabled
	}

	stored, err := s.repo.GetDaemon(ctx)
	if errors.Is(err, ErrNoDaemonRow) {
		return Ping{}, ErrNotConfigured
	}
	if err != nil {
		return Ping{}, err
	}

	password, err := s.sealer.Open(stored.PasswordEnc)
	if err != nil {
		// La clé maître a changé, ou la ligne a été écrite avec une autre.
		// Le dire ainsi évite de chercher du côté du démon un problème qui est
		// entièrement de notre côté.
		return Ping{}, fmt.Errorf(
			"mot de passe EC indéchiffrable — BOXINCLOUD_SECRET_KEY a-t-elle changé ? : %w", err)
	}

	address := net.JoinHostPort(stored.Host, strconv.Itoa(stored.Port))
	result := Ping{Address: address, ProtocolVersion: ec.ProtocolVersion}

	start := time.Now()
	conn, err := ec.Dial(ctx, address, string(password), ec.Options{
		ClientName:    "boxincloud",
		ClientVersion: s.opts.Version,
	})
	if err != nil {
		s.recordState(ctx, StateDisconnected, err.Error(), false)
		return result, err
	}
	defer func() { _ = conn.Close() }()

	result.Handshake = time.Since(start)
	result.ServerVersion = conn.ServerVersion

	// EC_OP_GET_CONNSTATE est la requête la moins coûteuse du protocole : elle
	// ne demande au démon aucun parcours de file ni de bibliothèque partagée.
	// Un ping doit mesurer le transport, pas le travail.
	start = time.Now()
	if _, err := conn.Do(ctx, ec.New(ec.OpGetConnstate)); err != nil {
		s.recordState(ctx, StateDisconnected, err.Error(), false)
		return result, fmt.Errorf("aller-retour après authentification : %w", err)
	}
	result.RoundTrip = time.Since(start)

	s.recordState(ctx, StateConnected, "démon joint et authentifié", true)
	return result, nil
}

// recordState persiste le dernier état constaté, sans jamais faire échouer
// l'opération qui l'a produit.
//
// Un ping réussi ne doit pas se signaler comme échoué parce que l'écriture de
// sa propre trace a raté.
func (s *Service) recordState(ctx context.Context, state State, detail string, seen bool) {
	if err := s.repo.SetState(ctx, state, detail, seen); err != nil {
		s.log.Warn("état du démon non persisté", "err", err)
	}
	s.publishStatus(ctx)
}
