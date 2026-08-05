package ec_test

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/amuletest"
)

/*
Le codec, confronté à un vrai amuled.

Les tests unitaires figent l'encodage octet par octet contre la spécification.
Ils ne peuvent pas dire si la spécification a été bien LUE — c'est le rôle de ce
fichier, et c'est la même doctrine que pour le stockage : le protocole ne se
teste pas avec des mocks.

Ce qu'un démon réel apporte et qu'aucune doublure ne donnerait : un sel
aléatoire de largeur variable à chaque connexion, une version de protocole
vérifiée strictement, et un refus authentique quand le mot de passe est faux.
*/

func dial(t *testing.T, env amuletest.Env, password string) (*ec.Conn, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return ec.Dial(ctx,
		net.JoinHostPort(env.Host, strconv.Itoa(env.Port)),
		password,
		ec.Options{ClientName: "boxincloud", ClientVersion: "test"},
	)
}

// TestIntegrationPoigneeDeMain est le test qui valide tout le codec d'un coup.
//
// Pour qu'il passe, il faut que le cadrage, l'encodage des tags, le décalage du
// nom, l'arithmétique des longueurs, le boutisme, la largeur variable des
// entiers et la formule du condensé salé soient TOUS justes. Un seul d'entre
// eux de travers et le démon refuse.
func TestIntegrationPoigneeDeMain(t *testing.T) {
	env := amuletest.Start(t)

	conn, err := dial(t, env, env.Password)
	if err != nil {
		t.Fatalf("authentification refusée par un vrai amuled : %v", err)
	}
	defer func() { _ = conn.Close() }()

	if conn.ServerVersion == "" {
		t.Error("le démon n'a pas annoncé sa version")
	}
	t.Logf("démon authentifié, version %q", conn.ServerVersion)
}

/*
TestIntegrationMotDePasseRefuse est la moitié qui compte.

Sans elle, un codec qui enverrait n'importe quoi dans le tag de mot de passe
passerait le test précédent le jour où le démon accepterait tout. On vérifie
donc aussi qu'un mauvais mot de passe est bien REFUSÉ — et que le refus se
présente comme tel plutôt que comme une erreur de transport.
*/
func TestIntegrationMotDePasseRefuse(t *testing.T) {
	env := amuletest.Start(t)

	_, err := dial(t, env, env.Password+"-faux")
	if err == nil {
		t.Fatal("un mot de passe faux a été accepté")
	}
	if !errors.Is(err, ec.ErrAuthFailed) {
		t.Errorf("erreur = %v, attendu ErrAuthFailed — un refus ne doit pas se "+
			"présenter comme une panne de transport", err)
	}
}

// TestIntegrationAllerRetourApresAuthentification vérifie qu'une session sert
// à quelque chose.
//
// Une poignée de main réussie ne prouve pas que le reste du protocole est
// compris : la négociation ne couvre qu'une poignée de tags. Le premier vrai
// échange, lui, passe par le chemin nominal du décodeur.
func TestIntegrationAllerRetourApresAuthentification(t *testing.T) {
	env := amuletest.Start(t)

	conn, err := dial(t, env, env.Password)
	if err != nil {
		t.Fatalf("connexion : %v", err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := conn.Do(ctx, ec.New(ec.OpGetConnstate))
	if err != nil {
		t.Fatalf("GetConnstate : %v", err)
	}

	// Le démon répond par des données diverses portant l'état de connexion.
	// On ne les interprète pas encore — c'est le travail de l'étape qui les
	// affichera — mais la réponse doit être structurée, pas vide.
	if resp.Op != ec.OpMiscData {
		t.Errorf("réponse = %s, attendu %s", resp.Op, ec.OpMiscData)
	}
	if len(resp.Tags) == 0 {
		t.Error("réponse sans aucun tag : le décodage a probablement dérivé")
	}

	if _, ok := resp.Find(ec.TagConnstate); !ok {
		t.Errorf("pas de tag d'état de connexion dans la réponse ; tags reçus : %v",
			tagNames(resp))
	}
}

/*
TestIntegrationPlusieursRequetesSurUneSession.

Le protocole n'a aucun identifiant de corrélation : les réponses arrivent dans
l'ordre des requêtes, et rien ne permettrait d'en attribuer une à la mauvaise.
Ce test vérifie qu'une session enchaîne correctement — un décalage d'une seule
trame ferait lire la réponse précédente à chaque appel, ce qui produirait des
données cohérentes mais systématiquement en retard d'un coup.
*/
func TestIntegrationPlusieursRequetesSurUneSession(t *testing.T) {
	env := amuletest.Start(t)

	conn, err := dial(t, env, env.Password)
	if err != nil {
		t.Fatalf("connexion : %v", err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Deux opérations DIFFÉRENTES, en alternance : si les réponses se
	// décalaient d'un cran, l'opcode reçu ne correspondrait plus à la requête.
	for i := 0; i < 4; i++ {
		if resp, err := conn.Do(ctx, ec.New(ec.OpGetConnstate)); err != nil {
			t.Fatalf("tour %d, GetConnstate : %v", i, err)
		} else if resp.Op != ec.OpMiscData {
			t.Fatalf("tour %d : réponse %s, attendu %s", i, resp.Op, ec.OpMiscData)
		}

		if resp, err := conn.Do(ctx, ec.New(ec.OpStatReq)); err != nil {
			t.Fatalf("tour %d, StatReq : %v", i, err)
		} else if resp.Op != ec.OpStats {
			t.Fatalf("tour %d : réponse %s, attendu %s", i, resp.Op, ec.OpStats)
		}
	}
}

// TestIntegrationStatistiquesSontLisibles décode une réponse VOLUMINEUSE.
//
// La poignée de main ne transporte que des tags plats et courts. Les
// statistiques, elles, portent des tags imbriqués et des entiers de plusieurs
// largeurs — c'est le premier endroit où une erreur d'arithmétique de longueur
// se manifesterait.
func TestIntegrationStatistiquesSontLisibles(t *testing.T) {
	env := amuletest.Start(t)

	conn, err := dial(t, env, env.Password)
	if err != nil {
		t.Fatalf("connexion : %v", err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := conn.Do(ctx, ec.New(ec.OpStatReq))
	if err != nil {
		t.Fatalf("StatReq : %v", err)
	}

	// Le décodeur refuse déjà toute trame dont il resterait des octets : que
	// cette réponse soit décodée est en soi la preuve que les longueurs
	// tombent juste. On vérifie en plus qu'elle porte des débits, qui sont
	// exactement ce que l'interface affichera.
	if _, ok := resp.Uint(ec.TagStatsUlSpeed); !ok {
		t.Errorf("pas de débit montant lisible ; tags reçus : %v", tagNames(resp))
	}
	if _, ok := resp.Uint(ec.TagStatsDlSpeed); !ok {
		t.Errorf("pas de débit descendant lisible ; tags reçus : %v", tagNames(resp))
	}
}

// tagNames rend les noms des tags de premier niveau, pour les messages d'échec.
//
// Un test qui dit « tag absent » sans dire ce qui est arrivé à la place oblige
// à rejouer la session à la main pour comprendre.
func tagNames(p ec.Packet) string {
	names := make([]string, 0, len(p.Tags))
	for _, tag := range p.Tags {
		names = append(names, tag.Name.String())
	}
	return strings.Join(names, ", ")
}
