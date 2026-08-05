// Package amuletest fournit un démon aMule jetable aux tests d'intégration.
//
// Même doctrine que pgtest et miniotest : on parle au vrai logiciel, pas à une
// doublure. Un codec de protocole ne se valide que contre l'implémentation qui
// l'a inspiré ; une doublure ne fait que confirmer nos propres suppositions.
//
// Chaque appel à Start démarre son propre conteneur : le démon est un processus
// à état (files de téléchargement, serveurs connus, partages) et deux tests qui
// se le partagent finissent par se contaminer.
//
// Le démon est configuré pour être HORS LIGNE : pas de connexion eD2k, pas de
// Kad, pas de téléchargement de liste ipfilter. Un test d'intégration qui
// dépend d'Internet n'est pas un test, c'est un pari.
package amuletest

import (
	"context"
	"crypto/md5" //nolint:gosec // imposé par le format d'amule.conf, pas un choix
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

/*
Plafonds de débit du démon de test, en OCTETS par seconde.

Exportés parce qu'ils sont un point de vérification : un test qui décode les
statistiques les compare à ces valeurs, et c'est ce qui distingue « décodé
correctement » de « tout à zéro parce que rien n'a été lu ».

aMule les configure en kilo-octets et les rend en octets — la conversion est
faite ici une fois, plutôt que dans chaque test.
*/
const (
	MaxDownloadBytesPerSecond = 64 * 1024
	MaxUploadBytesPerSecond   = 32 * 1024
)

// Env décrit le démon démarré pour un test.
type Env struct {
	Host     string // hôte joignable depuis le test
	Port     int    // port EC mappé
	Password string // mot de passe EC en CLAIR
}

const (
	image = "ghcr.io/ngosang/amule:latest"

	// Port EC à l'intérieur du conteneur. Le port publié, lui, est choisi par
	// Docker : plusieurs tests peuvent tourner en parallèle.
	ecPort = "4712/tcp"

	// Mot de passe EC en clair. Aucun secret ici : ce démon vit le temps d'un
	// test et n'écoute que sur une boucle locale.
	ecPassword = "boxincloud"

	// Emplacements dans le conteneur.
	amuleHome = "/home/amule/.aMule"
	amuleConf = amuleHome + "/amule.conf"
	amuleLog  = amuleHome + "/logfile"

	// La configuration est déposée dans /tmp, puis recopiée au démarrage :
	// l'API de copie de Docker refuse d'écrire dans un répertoire qui n'existe
	// pas encore, et ~/.aMule n'est créé que par l'entrypoint de l'image.
	stagedConf = "/tmp/amule.conf"
)

// bootstrap précède l'entrypoint de l'image.
//
// Il installe notre configuration AVANT que l'image ne génère la sienne : celle
// de l'image se connecte aux serveurs eD2k réels et télécharge 3 Mo de liste
// ipfilter au démarrage. L'entrypoint constate que le fichier existe et le
// respecte.
//
// Les services amuleweb et cron sont retirés : le test n'en a aucun besoin, et
// amuleweb ouvrirait une connexion EC parasite au démon — de quoi fausser tout
// test qui observe les clients connectés.
const bootstrap = `mkdir -p ` + amuleHome +
	` && cp ` + stagedConf + ` ` + amuleConf +
	` && rm -rf /etc/services.d/amuleweb /etc/services.d/cron` +
	` && exec /home/amule/entrypoint.sh`

// confTemplate est une configuration aMule minimale et hors ligne.
//
// Les clés absentes prennent leur valeur par défaut : on n'écrit que ce qui
// nous importe. Le %s reçoit l'empreinte du mot de passe EC.
//
// Point qui surprend : ECPassword n'est pas le mot de passe mais son empreinte
// MD5 en hexadécimal minuscule. Le client EC, lui, envoie le mot de passe en
// clair et c'est le démon qui compare les empreintes.
const confTemplate = `[eMule]
Nick=boxincloud-test

# Des plafonds de débit, alors que ce démon ne transfère jamais rien.
#
# Ils ne servent pas à limiter : ils servent de TÉMOIN. Sur un démon au repos,
# tous les compteurs valent légitimement zéro, si bien qu'un instantané
# entièrement nul est indiscernable d'un décodeur qui n'aurait rien lu. Deux
# valeurs non nulles, connues d'avance et stables, rendent la vérification
# possible.
#
# aMule les exprime en kilo-octets par seconde ; l'API du projet, elle, rend
# des octets par seconde. Voir MaxDownloadBytesPerSecond dans ce paquet.
MaxDownload=64
MaxUpload=32

Port=4662
UDPPort=4672
UDPEnable=0
Autoconnect=0
ConnectToKad=0
ConnectToED2K=0
Serverlist=0
AddServerListFromServer=0
AddServerListFromClient=0
Ed2kServersUrl=
KadNodesUrl=
IPFilterAutoLoad=0
IPFilterURL=
UPnPEnabled=0
NewVersionCheck=0
OnlineSignature=0
GeoIPEnabled=0
AutoRescanSharedDirs=0
TempDir=/downloads/temp
IncomingDir=/downloads/incoming
OSDirectory=` + amuleHome + `
[ExternalConnect]
AcceptExternalConnections=1
ECAddress=
ECPort=4712
ECPassword=%s
UPnPECEnabled=0
[WebServer]
Enabled=0
`

// Start démarre un amuled jetable et le rend prêt à accepter une connexion EC.
//
// Marque le test comme long : il est ignoré sous `go test -short`.
func Start(t *testing.T) Env {
	t.Helper()

	if testing.Short() {
		t.Skip("test d'intégration ignoré en mode -short")
	}
	requireDocker(t)

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{ecPort},
		// L'image dérive elle-même ECPassword de GUI_PWD par un sed sur la
		// configuration. Redondant avec ce que nous écrivons, mais c'est le
		// chemin documenté de l'image : si sa génération de configuration
		// change, le mot de passe reste celui que nous annonçons.
		Env: map[string]string{"GUI_PWD": ecPassword},
		Files: []testcontainers.ContainerFile{{
			Reader:            strings.NewReader(fmt.Sprintf(confTemplate, md5Hex(ecPassword))),
			ContainerFilePath: stagedConf,
			FileMode:          0o644,
		}},
		Entrypoint: []string{"/bin/sh", "-c", bootstrap},
		// Deux signaux, parce qu'aucun ne suffit seul :
		//
		//   - le journal d'amuled dit que le serveur EC écoute, mais amuled
		//     l'écrit dans son fichier de log et sa sortie standard reste
		//     tamponnée tant que le démon parle peu — inutile de guetter
		//     `docker logs` ;
		//   - le port, lui, est vérifié par testcontainers DEPUIS l'intérieur
		//     du conteneur. Vu du dehors, le relais de ports de Docker accepte
		//     la connexion avant même qu'amuled n'écoute : un simple dial
		//     réussit instantanément et ne prouve rien.
		WaitingFor: wait.ForAll(
			wait.ForExec([]string{"grep", "-q", "(ECServer) listening", amuleLog}).
				WithPollInterval(100*time.Millisecond).
				WithStartupTimeout(60*time.Second),
			wait.ForListeningPort(ecPort).WithStartupTimeout(60*time.Second),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("amuletest : démarrage du conteneur %s : %v", image, err)
	}
	t.Cleanup(func() {
		// Best effort : un nettoyage qui échoue ne doit pas faire échouer le
		// test, Ryuk ramassera le conteneur de toute façon.
		_ = testcontainers.TerminateContainer(container)
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("amuletest : hôte du conteneur : %v", err)
	}
	port, err := container.MappedPort(ctx, ecPort)
	if err != nil {
		t.Fatalf("amuletest : port EC mappé : %v", err)
	}

	return Env{
		Host:     host,
		Port:     int(port.Num()),
		Password: ecPassword,
	}
}

// ─── Disponibilité de Docker ─────────────────────────────────────────────────

var (
	dockerOnce sync.Once
	dockerErr  error
)

// requireDocker saute le test si aucun démon Docker n'est joignable.
//
// La sonde est faite à part du démarrage du conteneur pour pouvoir distinguer
// les deux cas : pas de Docker, c'est un test qu'on saute ; un amuled qui
// refuse de démarrer alors que Docker répond, c'est un échec à regarder.
func requireDocker(t *testing.T) {
	t.Helper()

	dockerOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		provider, err := testcontainers.NewDockerProvider()
		if err != nil {
			dockerErr = err
			return
		}
		defer func() { _ = provider.Close() }()
		dockerErr = provider.Health(ctx)
	})

	if dockerErr != nil {
		t.Skipf("amuletest : aucun Docker disponible (%v).\n"+
			"Ce test démarre un vrai amuled via testcontainers : démarrez Docker "+
			"(ou Colima/OrbStack) puis relancez.", dockerErr)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// md5Hex rend l'empreinte attendue par aMule pour ECPassword.
func md5Hex(s string) string {
	sum := md5.Sum([]byte(s)) //nolint:gosec // imposé par le format d'amule.conf
	return hex.EncodeToString(sum[:])
}
