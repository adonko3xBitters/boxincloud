package amule

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/amuletest"
)

/*
Les commandes, contre un vrai amuled.

Les tests unitaires figent la validation et les tables. Ils ne peuvent rien dire
de ce qui compte ici : que le démon COMPRENNE ce qu'on lui envoie.

C'est le risque propre aux commandes. amuled répond à la plupart d'entre elles
par un accusé vide — il dit « reçu », pas « fait ». Une commande dont l'opcode
serait faux, ou dont le tag porterait la mauvaise forme, obtiendrait donc
exactement la même réponse qu'une commande juste. La seule vérification
possible est de regarder l'ÉTAT ensuite.

Le fichier de test est un lien ed2k inventé : le démon est hors ligne, il ne
téléchargera jamais rien, mais il met la demande en file et lui applique les
gestes. C'est tout ce qu'il faut.
*/

// Un lien ed2k valide vers un fichier qui n'existe pas. La forme est correcte,
// ce qui suffit : le démon la valide, l'inscrit en file, et n'ira jamais
// chercher les octets puisqu'il n'a ni serveur ni Kad.
const lienDeTest = "ed2k://|file|boxincloud-essai.bin|1048576|" +
	"0123456789ABCDEF0123456789ABCDEF|/"

const hashDeTest = "0123456789abcdef0123456789abcdef"

// testService construit un service branché sur un démon jetable.
//
// Le dépôt est doublé — la persistance n'a rien à voir avec ce qui est vérifié
// ici — mais le démon, lui, est réel.
func testService(t *testing.T, env amuletest.Env) *Service {
	t.Helper()

	svc := newTestService(t, &fakeRepo{}, enabled())

	if _, err := svc.SetDaemon(context.Background(), SetDaemonParams{
		Host:     env.Host,
		Port:     env.Port,
		Password: env.Password,
	}); err != nil {
		t.Fatalf("déclaration du démon : %v", err)
	}
	return svc
}

// waitForDownload attend qu'un fichier apparaisse dans la file, et le rend.
//
// L'ajout d'un lien est asynchrone côté démon : il accuse réception, puis
// inscrit. Sans cette attente, le test suivant lirait une file encore vide et
// accuserait la commande d'avoir échoué.
func waitForDownload(t *testing.T, svc *Service, hash string) Download {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := serviceDialer{svc: svc}.Open(ctx)
		if err != nil {
			t.Fatalf("session : %v", err)
		}

		resp, err := conn.Do(ctx, requestDownloads())
		_ = conn.Close()
		if err != nil {
			t.Fatalf("file de téléchargement : %v", err)
		}

		downloads, err := decodeDownloads(resp)
		if err != nil {
			t.Fatalf("décodage de la file : %v", err)
		}
		for _, d := range downloads {
			if strings.EqualFold(d.Hash, hash) {
				return d
			}
		}
		time.Sleep(250 * time.Millisecond)
	}

	t.Fatalf("le fichier %s n'est jamais apparu dans la file", hash)
	return Download{}
}

/*
TestIntegrationCycleDeCommandes est le test central de l'étape.

Il suit un fichier d'un bout à l'autre : ajouté par un lien, mis en pause,
repris, puis supprimé. Chaque geste est vérifié sur l'ÉTAT du démon, pas sur sa
réponse — parce que sa réponse ne dit rien.
*/
func TestIntegrationCycleDeCommandes(t *testing.T) {
	env := amuletest.Start(t)
	svc := testService(t, env)
	ctx := context.Background()

	// ── Ajout ────────────────────────────────────────────────────────────
	if err := svc.AddLink(ctx, lienDeTest); err != nil {
		t.Fatalf("ajout du lien : %v", err)
	}

	download := waitForDownload(t, svc, hashDeTest)
	t.Logf("fichier en file : %q, %d octets, état %q",
		download.Name, download.Size, download.Status)

	if download.Name == "" {
		t.Error("le fichier est en file mais sans nom : le lien a été mal analysé")
	}
	if download.Size != 1048576 {
		t.Errorf("taille %d, attendu 1048576 — le lien a été mal analysé", download.Size)
	}

	// ── Pause ────────────────────────────────────────────────────────────
	if err := svc.ActOnDownload(ctx, hashDeTest, DownloadPause); err != nil {
		t.Fatalf("mise en pause : %v", err)
	}

	paused := waitForStatus(t, svc, hashDeTest, DownloadPaused)
	t.Logf("après pause : état %q", paused.Status)

	// ── Reprise ──────────────────────────────────────────────────────────
	//
	// On vérifie qu'on QUITTE l'état de pause, sans exiger un état précis :
	// un fichier repris sur un démon hors ligne est « en attente », pas « en
	// cours », et attendre « downloading » ferait échouer un test sur une
	// propriété qui n'a rien à voir avec la commande.
	if err := svc.ActOnDownload(ctx, hashDeTest, DownloadResume); err != nil {
		t.Fatalf("reprise : %v", err)
	}

	resumed := waitForNotStatus(t, svc, hashDeTest, DownloadPaused)
	t.Logf("après reprise : état %q", resumed.Status)

	// ── Priorité ─────────────────────────────────────────────────────────
	if err := svc.SetDownloadPriority(ctx, hashDeTest, PriorityHigh); err != nil {
		t.Fatalf("changement de priorité : %v", err)
	}

	prioritised := waitFor(t, svc, hashDeTest, func(d Download) bool {
		return d.Priority == PriorityHigh
	}, "priorité haute")
	t.Logf("après priorité : %q", prioritised.Priority)

	// ── Suppression ──────────────────────────────────────────────────────
	if err := svc.ActOnDownload(ctx, hashDeTest, DownloadCancel); err != nil {
		t.Fatalf("suppression : %v", err)
	}

	waitForAbsence(t, svc, hashDeTest)
}

/*
TestIntegrationCommandesSansSpectateur.

La scrutation n'ouvre de session que lorsqu'un navigateur regarde. Une commande
doit néanmoins partir : on peut vouloir mettre en pause depuis le CLI, ou juste
après avoir ouvert la page, avant que le flux ne soit établi.

Ici aucune scrutation n'est armée du tout — le service n'a jamais été démarré —
et la commande doit ouvrir sa propre session.
*/
func TestIntegrationCommandesSansSpectateur(t *testing.T) {
	env := amuletest.Start(t)
	svc := testService(t, env)

	if svc.Snapshot() != nil {
		t.Fatal("un instantané existe alors que la scrutation n'a jamais été armée")
	}

	if err := svc.AddLink(context.Background(), lienDeTest); err != nil {
		t.Fatalf("commande sans scrutation armée : %v", err)
	}
	waitForDownload(t, svc, hashDeTest)
}

/*
TestIntegrationKadRefusEstRemonte.

Le démon de test a Kad désactivé dans sa configuration — c'est ce qui le garde
hors ligne, et il n'est pas question de le rallumer pour un test : un test
d'intégration qui rejoint le réseau réel n'est plus un test, c'est un pari.

Le refus vaut mieux qu'un succès, et prouve davantage :

  - le démon a COMPRIS la commande. Un opcode inconnu produirait une autre
    erreur — ou pire, un accusé vide et silencieux.
  - son explication nous parvient telle quelle, au lieu d'être écrasée par un
    « la commande a échoué » qui n'apprendrait rien.

Ce que ce test ne prouve pas : que Kad démarre vraiment. Cela demanderait un
démon autorisé à sortir, et se vérifie à la main.
*/
func TestIntegrationKadRefusEstRemonte(t *testing.T) {
	env := amuletest.Start(t)
	svc := testService(t, env)

	if running := kadRunning(t, svc); running {
		t.Fatal("Kad tourne déjà : le démon de test doit démarrer sans lui")
	}

	err := svc.StartKad(context.Background())
	if err == nil {
		t.Fatal("le démon a accepté de démarrer un Kad désactivé dans sa configuration")
	}

	// L'explication du démon doit traverser : c'est elle qui dira à un
	// administrateur d'aller regarder amule.conf plutôt que le réseau.
	if !strings.Contains(strings.ToLower(err.Error()), "disabled") {
		t.Errorf("erreur = %v — l'explication du démon a été perdue en chemin", err)
	}
	t.Logf("refus remonté tel quel : %v", err)
}

// ─── Attentes ────────────────────────────────────────────────────────────────

func waitFor(t *testing.T, svc *Service, hash string, ok func(Download) bool, what string) Download {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	var last Download

	for time.Now().Before(deadline) {
		last = waitForDownload(t, svc, hash)
		if ok(last) {
			return last
		}
		time.Sleep(250 * time.Millisecond)
	}

	t.Fatalf("%s jamais atteint ; dernier état : %+v", what, last)
	return Download{}
}

func waitForStatus(t *testing.T, svc *Service, hash string, status DownloadStatus) Download {
	t.Helper()
	return waitFor(t, svc, hash, func(d Download) bool { return d.Status == status },
		"état "+string(status))
}

func waitForNotStatus(t *testing.T, svc *Service, hash string, status DownloadStatus) Download {
	t.Helper()
	return waitFor(t, svc, hash, func(d Download) bool { return d.Status != status },
		"sortie de l'état "+string(status))
}

func waitForAbsence(t *testing.T, svc *Service, hash string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := serviceDialer{svc: svc}.Open(ctx)
		if err != nil {
			t.Fatalf("session : %v", err)
		}
		resp, err := conn.Do(ctx, requestDownloads())
		_ = conn.Close()
		if err != nil {
			t.Fatalf("file : %v", err)
		}

		downloads, err := decodeDownloads(resp)
		if err != nil {
			t.Fatalf("décodage : %v", err)
		}

		present := false
		for _, d := range downloads {
			if strings.EqualFold(d.Hash, hash) {
				present = true
				break
			}
		}
		if !present {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}

	t.Fatal("le fichier est toujours en file après suppression")
}

func kadRunning(t *testing.T, svc *Service) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := serviceDialer{svc: svc}.Open(ctx)
	if err != nil {
		t.Fatalf("session : %v", err)
	}
	defer func() { _ = conn.Close() }()

	resp, err := conn.Do(ctx, requestConnection())
	if err != nil {
		t.Fatalf("état de connexion : %v", err)
	}

	connection, err := decodeConnection(resp)
	if err != nil {
		t.Fatalf("décodage de l'état : %v", err)
	}
	return connection.Kad.Running
}
