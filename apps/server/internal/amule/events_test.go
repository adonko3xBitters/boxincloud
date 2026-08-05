package amule

import (
	"testing"
	"time"
)

/*
Les tests de la dérivation.

Entièrement hors ligne, et ce n'est pas une commodité : c'est la propriété que
l'ADR cherchait en dérivant les événements plutôt qu'en les relayant. Deux
instantanés écrits à la main, une fonction pure, aucun démon, aucun réseau,
aucun conteneur.
*/

var quand = time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)

// fichier construit un téléchargement à mi-parcours, dans l'état demandé.
func fichier(hash string, status DownloadStatus) Download {
	return Download{
		Hash:     hash,
		Name:     hash + ".cbz",
		Size:     100,
		SizeDone: 42,
		Status:   status,
	}
}

// acquis construit un téléchargement dont toutes les parties sont là — celui
// qui, en quittant la file, est une fin et non une annulation.
func acquis(hash string) Download {
	file := fichier(hash, DownloadDownloading)
	file.SizeDone = file.Size
	return file
}

func envoi(pair, fichier string) Upload {
	return Upload{
		UserHash: pair,
		Name:     "pair " + pair,
		FileHash: fichier,
		FileName: fichier + ".cbz",
	}
}

func serveur(nom, ip string, port int) *Server {
	return &Server{IP: ip, Port: port, Name: nom, Connected: true}
}

// lie décrit un lien réseau : serveur joint (ou nil), et Kad connecté ou non.
func lie(s *Server, kad bool) Connection {
	return Connection{
		Ed2k: Ed2kState{Connected: s != nil, Server: s},
		Kad:  KadState{Running: true, Connected: kad},
	}
}

// etat assemble un instantané minimal : seul ce que le cas éclaire est
// renseigné, le reste vaut zéro.
func etat(connection Connection, downloads []Download, uploads []Upload) *Snapshot {
	return &Snapshot{
		TakenAt:    quand,
		Connection: connection,
		Downloads:  downloads,
		Uploads:    uploads,
	}
}

// hors est le lien réseau au repos : aucun serveur, Kad éteint.
func hors() Connection { return lie(nil, false) }

/*
TestTableDesEvenements parcourt la table documentée en tête de events.go.

Une ligne par transition, et l'ordre des événements compte : c'est celui dans
lequel l'interface les recevra.
*/
func TestTableDesEvenements(t *testing.T) {
	joint := serveur("DonkeyServer", "1.2.3.4", 4242)
	autre := serveur("PeerBooter", "5.6.7.8", 4661)

	cas := []struct {
		nom      string
		previous *Snapshot
		current  *Snapshot
		attendus []EventKind
	}{
		{
			nom:      "téléchargement apparu",
			previous: etat(hors(), nil, nil),
			current:  etat(hors(), []Download{fichier("a", DownloadWaiting)}, nil),
			attendus: []EventKind{EventDownloadStarted},
		},
		{
			nom:      "téléchargement terminé par son statut",
			previous: etat(hors(), []Download{fichier("a", DownloadDownloading)}, nil),
			current:  etat(hors(), []Download{fichier("a", DownloadCompleted)}, nil),
			attendus: []EventKind{EventDownloadCompleted},
		},
		{
			nom: "téléchargement terminé en quittant la file",
			// amuled retire de la file ce qu'il a fini : sur un petit fichier,
			// c'est la SEULE façon d'en voir la fin.
			previous: etat(hors(), []Download{acquis("a")}, nil),
			current:  etat(hors(), nil, nil),
			attendus: []EventKind{EventDownloadCompleted},
		},
		{
			nom:      "une fin déjà annoncée ne se réannonce pas en quittant la file",
			previous: etat(hors(), []Download{fichier("a", DownloadCompleted)}, nil),
			current:  etat(hors(), nil, nil),
			attendus: nil,
		},
		{
			nom:      "téléchargement annulé",
			previous: etat(hors(), []Download{fichier("a", DownloadDownloading)}, nil),
			current:  etat(hors(), nil, nil),
			attendus: []EventKind{EventDownloadRemoved},
		},
		{
			nom:      "téléchargement mis en pause",
			previous: etat(hors(), []Download{fichier("a", DownloadDownloading)}, nil),
			current:  etat(hors(), []Download{fichier("a", DownloadPaused)}, nil),
			attendus: []EventKind{EventDownloadPaused},
		},
		{
			nom:      "arrêté se présente comme une pause",
			previous: etat(hors(), []Download{fichier("a", DownloadDownloading)}, nil),
			current:  etat(hors(), []Download{fichier("a", DownloadStopped)}, nil),
			attendus: []EventKind{EventDownloadPaused},
		},
		{
			nom:      "téléchargement repris",
			previous: etat(hors(), []Download{fichier("a", DownloadPaused)}, nil),
			current:  etat(hors(), []Download{fichier("a", DownloadDownloading)}, nil),
			attendus: []EventKind{EventDownloadResumed},
		},
		{
			nom:      "reprise sans source disponible reste une reprise",
			previous: etat(hors(), []Download{fichier("a", DownloadStopped)}, nil),
			current:  etat(hors(), []Download{fichier("a", DownloadWaiting)}, nil),
			attendus: []EventKind{EventDownloadResumed},
		},
		{
			nom: "une pause qui se termine n'est pas une reprise",
			// Sans la priorité donnée à la fin, ce cas produirait « repris »
			// PUIS « terminé » : deux notifications pour un seul fait.
			previous: etat(hors(), []Download{fichier("a", DownloadPaused)}, nil),
			current:  etat(hors(), []Download{fichier("a", DownloadCompleted)}, nil),
			attendus: []EventKind{EventDownloadCompleted},
		},
		{
			nom:      "téléchargement en erreur",
			previous: etat(hors(), []Download{fichier("a", DownloadDownloading)}, nil),
			current:  etat(hors(), []Download{fichier("a", DownloadErroneous)}, nil),
			attendus: []EventKind{EventDownloadError},
		},
		{
			nom: "un détail d'exécution ne fait pas un événement",
			// « en attente » qui devient « reçoit des données » est dans
			// l'instantané ; en faire une notification noierait les vrais faits.
			previous: etat(hors(), []Download{fichier("a", DownloadWaiting)}, nil),
			current:  etat(hors(), []Download{fichier("a", DownloadDownloading)}, nil),
			attendus: nil,
		},
		{
			nom:      "serveur connecté",
			previous: etat(hors(), nil, nil),
			current:  etat(lie(joint, false), nil, nil),
			attendus: []EventKind{EventServerConnected},
		},
		{
			nom:      "serveur déconnecté",
			previous: etat(lie(joint, false), nil, nil),
			current:  etat(hors(), nil, nil),
			attendus: []EventKind{EventServerDisconnected},
		},
		{
			nom: "bascule d'un serveur à l'autre",
			// La bascule prend moins d'une seconde : aucun instantané ne voit
			// l'état déconnecté, et sans cette paire l'interface afficherait
			// encore l'ancien serveur.
			previous: etat(lie(joint, false), nil, nil),
			current:  etat(lie(autre, false), nil, nil),
			attendus: []EventKind{EventServerDisconnected, EventServerConnected},
		},
		{
			nom:      "kad connecté",
			previous: etat(hors(), nil, nil),
			current:  etat(lie(nil, true), nil, nil),
			attendus: []EventKind{EventKadConnected},
		},
		{
			nom:      "kad déconnecté",
			previous: etat(lie(nil, true), nil, nil),
			current:  etat(hors(), nil, nil),
			attendus: []EventKind{EventKadDisconnected},
		},
		{
			nom:      "envoi démarré",
			previous: etat(hors(), nil, nil),
			current:  etat(hors(), nil, []Upload{envoi("pair1", "a")}),
			attendus: []EventKind{EventUploadStarted},
		},
		{
			nom:      "envoi terminé",
			previous: etat(hors(), nil, []Upload{envoi("pair1", "a")}),
			current:  etat(hors(), nil, nil),
			attendus: []EventKind{EventUploadCompleted},
		},
		{
			nom: "le même fichier vers deux pairs fait deux envois",
			// La clé est le couple (pair, fichier) : sans cela, le second pair
			// ne produirait aucun événement.
			previous: etat(hors(), nil, []Upload{envoi("pair1", "a")}),
			current:  etat(hors(), nil, []Upload{envoi("pair1", "a"), envoi("pair2", "a")}),
			attendus: []EventKind{EventUploadStarted},
		},
		{
			nom:      "rien n'a changé",
			previous: etat(lie(joint, true), []Download{fichier("a", DownloadDownloading)}, nil),
			current:  etat(lie(joint, true), []Download{fichier("a", DownloadDownloading)}, nil),
			attendus: nil,
		},
	}

	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			verifieKinds(t, diff(c.previous, c.current), c.attendus)
		})
	}
}

/*
TestUnTelechargementApparuEtTermineNeDitQueLaFin — premier angle mort de l'ADR.

Ce n'est pas un défaut à corriger : la latence d'un événement vaut la période de
scrutation, et ce qui se passe entièrement dans l'intervalle n'est jamais
observé. Le test existe pour que le comportement reste CONSCIENT — et pour
rappeler la conséquence qui compte : le pont vers la bibliothèque devra reposer
sur l'état du répertoire d'arrivée, pas sur cet événement.
*/
func TestUnTelechargementApparuEtTermineNeDitQueLaFin(t *testing.T) {
	previous := etat(hors(), nil, nil)
	current := etat(hors(), []Download{fichier("a", DownloadCompleted)}, nil)

	verifieKinds(t, diff(previous, current), []EventKind{EventDownloadCompleted})
}

/*
TestLePremierInstantaneNeProduitAucunEvenement — second angle mort de l'ADR.

Au démarrage tout est nouveau. Dériver un « démarré » par fichier de la file
annoncerait comme neufs des téléchargements vieux de trois jours, et un
« connecté » pour un serveur joint depuis une semaine. Inonder l'interface de
faux événements est pire que de n'en envoyer aucun : l'état initial lui parvient
entier par l'instantané que le concentrateur envoie à l'ouverture du flux.
*/
func TestLePremierInstantaneNeProduitAucunEvenement(t *testing.T) {
	current := etat(
		lie(serveur("DonkeyServer", "1.2.3.4", 4242), true),
		[]Download{
			fichier("a", DownloadDownloading),
			fichier("b", DownloadPaused),
			fichier("c", DownloadCompleted),
		},
		[]Upload{envoi("pair1", "a")},
	)

	if events := diff(nil, current); len(events) != 0 {
		t.Fatalf("le premier instantané a produit %d événements : %v", len(events), kinds(events))
	}
}

// TestEvenementDesigneLeFichierConcerne : sans le hash, l'interface sait qu'il
// s'est passé quelque chose mais pas quelle ligne rafraîchir.
func TestEvenementDesigneLeFichierConcerne(t *testing.T) {
	previous := etat(hors(), nil, nil)
	current := etat(hors(), []Download{fichier("abcdef", DownloadWaiting)}, nil)

	events := diff(previous, current)
	if len(events) != 1 {
		t.Fatalf("événements = %v, attendu un seul", kinds(events))
	}

	event := events[0]
	if event.Hash != "abcdef" {
		t.Errorf("Hash = %q, attendu %q", event.Hash, "abcdef")
	}
	if event.Name != "abcdef.cbz" {
		t.Errorf("Name = %q, attendu le nom du fichier", event.Name)
	}
	if !event.At.Equal(quand) {
		t.Errorf("At = %v, attendu la date de l'instantané qui a révélé le changement", event.At)
	}
}

// TestEvenementDeServeurPorteSonAdresse : deux serveurs peuvent porter la même
// bannière, l'adresse est ce qui les distingue.
func TestEvenementDeServeurPorteSonAdresse(t *testing.T) {
	previous := etat(hors(), nil, nil)
	current := etat(lie(serveur("DonkeyServer", "1.2.3.4", 4242), false), nil, nil)

	events := diff(previous, current)
	if len(events) != 1 {
		t.Fatalf("événements = %v, attendu un seul", kinds(events))
	}
	if events[0].Name != "DonkeyServer" || events[0].Detail != "1.2.3.4:4242" {
		t.Errorf("événement = %+v, attendu le nom et l'adresse du serveur", events[0])
	}
}

/*
TestServeurConnecteSansDescriptionNePaniquePas.

Le démon peut se dire connecté avant d'avoir renseigné le serveur — c'est le cas
pendant la poignée de main. Un instantané un peu en avance sur lui-même ne doit
pas faire tomber la scrutation.
*/
func TestServeurConnecteSansDescriptionNePaniquePas(t *testing.T) {
	previous := etat(hors(), nil, nil)
	current := &Snapshot{
		TakenAt:    quand,
		Connection: Connection{Ed2k: Ed2kState{Connected: true, Server: nil}},
	}

	verifieKinds(t, diff(previous, current), []EventKind{EventServerConnected})
}

/*
TestOrdreDesEvenementsEstStable.

L'ordre vient de tranches parcourues dans leur ordre, jamais du parcours d'une
map. Un ordre qui changerait d'un appel à l'autre rendrait ces tests
intermittents et l'affichage sautillant.
*/
func TestOrdreDesEvenementsEstStable(t *testing.T) {
	previous := etat(
		lie(serveur("DonkeyServer", "1.2.3.4", 4242), false),
		[]Download{fichier("a", DownloadDownloading), fichier("b", DownloadPaused)},
		[]Upload{envoi("pair1", "a")},
	)
	current := etat(
		lie(nil, true),
		[]Download{fichier("b", DownloadDownloading), fichier("c", DownloadWaiting)},
		[]Upload{envoi("pair2", "b")},
	)

	reference := kinds(diff(previous, current))
	if len(reference) == 0 {
		t.Fatal("le cas ne produit aucun événement : il ne teste rien")
	}

	for range 20 {
		verifieKinds(t, diff(previous, current), reference)
	}
}

func kinds(events []Event) []EventKind {
	out := make([]EventKind, 0, len(events))
	for _, event := range events {
		out = append(out, event.Kind)
	}
	return out
}

func verifieKinds(t *testing.T, events []Event, attendus []EventKind) {
	t.Helper()

	got := kinds(events)
	if len(got) != len(attendus) {
		t.Fatalf("événements = %v, attendu %v", got, attendus)
	}
	for i := range got {
		if got[i] != attendus[i] {
			t.Fatalf("événements = %v, attendu %v", got, attendus)
		}
	}
}
