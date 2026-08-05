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
Le collecteur en entier, contre un vrai amuled.

Les tests des traducteurs prouvent chacun sa réponse. Celui-ci prouve ce
qu'aucun d'eux ne peut prouver seul : que les six allers-retours s'enchaînent
sur UNE session sans se décaler.

C'est le risque propre à ce protocole. External Connections n'a pas
d'identifiant de corrélation — les réponses arrivent dans l'ordre des requêtes —
si bien qu'une seule requête mal formée décale tout ce qui suit. Le symptôme
n'est pas une erreur : ce sont des statistiques lues dans une liste de serveurs,
c'est-à-dire des champs vides et un instantané plausible et faux.
*/
func TestIntegrationCollecteurInstantaneComplet(t *testing.T) {
	env := amuletest.Start(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := ec.Dial(ctx,
		net.JoinHostPort(env.Host, strconv.Itoa(env.Port)),
		env.Password,
		ec.Options{ClientName: "boxincloud", ClientVersion: "test"},
	)
	if err != nil {
		t.Fatalf("connexion au démon : %v", err)
	}
	defer func() { _ = conn.Close() }()

	snapshot, err := ecCollector{}.Collect(ctx, conn)
	if err != nil {
		t.Fatalf("collecte d'un instantané complet : %v", err)
	}

	if snapshot.TakenAt.IsZero() {
		t.Error("instantané sans horodatage")
	}

	/*
		Le démon de test est volontairement hors ligne : ni serveur, ni Kad, ni
		fichier partagé, ni téléchargement. Les listes vides sont donc le
		résultat ATTENDU, et les vérifier a du sens — un décodeur qui inventerait
		des entrées à partir d'une réponse vide serait bien pire qu'un décodeur
		qui échoue.
	*/
	if snapshot.Connection.Ed2k.Connected {
		t.Error("le démon de test est hors ligne, il ne peut pas être connecté à un serveur")
	}
	if snapshot.Connection.Kad.Running {
		t.Error("Kad est désactivé dans le démon de test")
	}
	if n := len(snapshot.Downloads); n != 0 {
		t.Errorf("%d téléchargements sur un démon vierge", n)
	}

	/*
		Les plafonds de débit sont le témoin, et ils existent pour cela.

		Sur un démon au repos, tous les autres compteurs valent légitimement
		zéro : un instantané entièrement nul serait alors indiscernable d'un
		décodeur qui n'aurait rien lu. Le démon de test porte donc deux plafonds
		dans sa configuration — valeurs connues d'avance, stables, et non
		nulles. Sans elles, ce test ne prouverait que l'absence d'erreur.

		Ils vérifient aussi la conversion d'unité au passage : aMule les
		configure en kilo-octets par seconde et les rend en octets.
	*/
	if snapshot.Stats.DownLimit != amuletest.MaxDownloadBytesPerSecond {
		t.Errorf("plafond descendant %d octets/s, attendu %d — les statistiques "+
			"n'ont pas été décodées, ou l'unité a été convertie deux fois",
			snapshot.Stats.DownLimit, amuletest.MaxDownloadBytesPerSecond)
	}
	if snapshot.Stats.UpLimit != amuletest.MaxUploadBytesPerSecond {
		t.Errorf("plafond montant %d octets/s, attendu %d",
			snapshot.Stats.UpLimit, amuletest.MaxUploadBytesPerSecond)
	}

	t.Logf("instantané : %d téléchargements, %d serveurs, %d partagés, "+
		"plafonds %d/%d octets/s",
		len(snapshot.Downloads), len(snapshot.Servers), len(snapshot.SharedFiles),
		snapshot.Stats.DownLimit, snapshot.Stats.UpLimit)
}

/*
TestIntegrationCollecteurEnchainementRepete est le test du décalage.

Un décalage d'une trame ne se voit pas au premier tour : la première réponse
correspond encore à la première requête. Il apparaît au tour suivant, quand la
session a pris un cran de retard. Répéter la collecte sur la MÊME session est
donc la seule façon de l'attraper.
*/
func TestIntegrationCollecteurEnchainementRepete(t *testing.T) {
	env := amuletest.Start(t)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	conn, err := ec.Dial(ctx,
		net.JoinHostPort(env.Host, strconv.Itoa(env.Port)),
		env.Password,
		ec.Options{ClientName: "boxincloud", ClientVersion: "test"},
	)
	if err != nil {
		t.Fatalf("connexion au démon : %v", err)
	}
	defer func() { _ = conn.Close() }()

	var first uint64
	for tour := range 3 {
		snapshot, err := ecCollector{}.Collect(ctx, conn)
		if err != nil {
			t.Fatalf("tour %d : %v", tour, err)
		}

		// Le plafond de débit ne change pas d'un tour à l'autre : s'il varie,
		// c'est qu'on ne lit plus la même chose.
		limit := uint64(snapshot.Stats.DownLimit) //nolint:gosec // plafond, toujours positif
		if tour == 0 {
			first = limit
			continue
		}
		if limit != first {
			t.Fatalf("tour %d : plafond descendant %d, %d au premier tour — "+
				"les réponses se sont décalées", tour, limit, first)
		}
	}
}
