package amule

import (
	"net"
	"strconv"
	"time"
)

/*
Dérivation des événements, par comparaison de deux instantanés.

EC est un protocole requête/réponse : amuled n'émet aucun message spontané, et
« un téléchargement a démarré » n'existe nulle part dans le protocole. Ce n'est
pas un message à relayer, c'est une CONCLUSION — tirée de la différence entre
l'instantané précédent et le courant. Voir
docs/adr/005-temps-reel-sse-evenements-derives.md.

# La table

	download.started      un hash apparaît dans la file, pas déjà terminé
	download.completed    le statut passe à « completed », OU le fichier quitte
	                      la file en étant acquis — amuled retire de la file ce
	                      qu'il a fini d'assembler
	download.removed      le fichier quitte la file sans être acquis : annulé
	download.paused       le statut passe à « paused » ou « stopped »
	download.resumed      le statut quitte « paused »/« stopped » pour un état
	                      actif
	download.error        le statut passe à « erroneous »
	server.connected      le lien eD2k s'établit — ou bascule vers un autre
	                      serveur, ce qui produit la paire déconnecté/connecté
	server.disconnected   le lien eD2k tombe
	kad.connected         Kad a trouvé le réseau
	kad.disconnected      Kad l'a perdu
	upload.started        un couple (pair, fichier) apparaît dans les envois
	upload.completed      il en disparaît
	daemon.connected      une session EC est ouverte avec amuled — produit par
	                      la scrutation, pas par diff (voir poller.go)
	daemon.disconnected   la session EC est perdue ou suspendue

Un téléchargement ne produit qu'UN événement par comparaison : les statuts sont
exclusifs, et annoncer à la fois « repris » et « terminé » pour un même passage
ferait afficher deux notifications pour un seul fait.

# Les deux angles morts, assumés

**Un téléchargement qui apparaît ET se termine entre deux instantanés ne produit
qu'un « terminé ».** La latence d'un événement vaut la période de scrutation :
ce qui se passe entièrement dans l'intervalle n'est jamais observé, seulement
son résultat. L'ADR le nomme et en tire la conséquence qui compte — le pont vers
la bibliothèque ne devra pas reposer sur l'événement mais sur l'état du
répertoire d'arrivée.

**Le premier instantané ne produit AUCUN événement.** Au démarrage tout est
nouveau : dériver un « démarré » par fichier de la file inonderait l'interface
de notifications pour des téléchargements vieux de trois jours. L'état initial
se transmet autrement — l'instantané complet, que le concentrateur envoie à
l'ouverture du flux.
*/

// EventKind identifie un changement observé.
//
// Les valeurs sont en anglais et ne sont pas décoratives : elles deviennent le
// nom du champ `event:` d'une trame SSE, donc le nom que le navigateur écoute.
// C'est un contrat au même titre qu'un chemin d'API.
type EventKind string

const (
	EventDownloadStarted   EventKind = "download.started"
	EventDownloadCompleted EventKind = "download.completed"
	EventDownloadRemoved   EventKind = "download.removed"
	EventDownloadPaused    EventKind = "download.paused"
	EventDownloadResumed   EventKind = "download.resumed"
	EventDownloadError     EventKind = "download.error"

	EventServerConnected    EventKind = "server.connected"
	EventServerDisconnected EventKind = "server.disconnected"

	EventKadConnected    EventKind = "kad.connected"
	EventKadDisconnected EventKind = "kad.disconnected"

	EventUploadStarted   EventKind = "upload.started"
	EventUploadCompleted EventKind = "upload.completed"

	// Les deux suivants ne sortent pas de diff : ils décrivent NOTRE session
	// avec amuled, pas l'état du réseau eD2k. La distinction compte pour
	// l'interface — un démon injoignable et un démon connecté à aucun serveur
	// n'appellent pas le même message.
	EventDaemonConnected    EventKind = "daemon.connected"
	EventDaemonDisconnected EventKind = "daemon.disconnected"
)

/*
Event est un changement observé entre deux instantanés.

Volontairement plat et mince. Un événement n'a pas à porter l'état complet de ce
qu'il désigne : l'instantané courant l'accompagne sur le même flux, et le
dupliquer dans chaque événement multiplierait les octets envoyés à vingt onglets
pour une information qu'ils ont déjà. Ce qui est ici est le strict nécessaire
pour afficher une notification et savoir quelle ligne rafraîchir.
*/
type Event struct {
	Kind EventKind `json:"kind"`

	// Hash identifie le fichier concerné — l'empreinte eD2k pour un
	// téléchargement comme pour un envoi. Vide pour les événements de réseau.
	//
	// C'est la clé stable : le nom peut changer, et le numéro interne du démon
	// change d'un redémarrage à l'autre.
	Hash string `json:"hash,omitempty"`

	// Name est le libellé lisible : nom du fichier, nom du serveur.
	Name string `json:"name,omitempty"`

	// Detail complète en une ligne : adresse du serveur, pair servi, cause d'un
	// changement d'état de session.
	Detail string `json:"detail,omitempty"`

	// At est la date de l'instantané qui a révélé le changement — pas celle du
	// changement lui-même, qui est inconnue et se situe quelque part dans
	// l'intervalle de scrutation.
	At time.Time `json:"at"`
}

/*
diff compare deux instantanés et en déduit les événements.

`previous` nul rend une liste vide : c'est le premier instantané, et tout y est
nouveau. Voir la note sur les angles morts, plus haut.

L'ordre est déterministe — réseau, puis téléchargements, puis envois, chacun
dans l'ordre des tranches comparées. Un ordre qui dépendrait du parcours d'une
map rendrait les tests instables et l'affichage sautillant.
*/
func diff(previous, current *Snapshot) []Event {
	if previous == nil || current == nil {
		return nil
	}

	at := current.TakenAt

	var events []Event
	events = append(events, connectionEvents(previous, current, at)...)
	events = append(events, downloadEvents(previous, current, at)...)
	events = append(events, uploadEvents(previous, current, at)...)
	return events
}

// connectionEvents dérive les changements de lien réseau.
func connectionEvents(previous, current *Snapshot, at time.Time) []Event {
	var events []Event

	was, now := previous.Connection.Ed2k, current.Connection.Ed2k
	switch {
	case !was.Connected && now.Connected:
		events = append(events, serverEvent(EventServerConnected, now.Server, at))

	case was.Connected && !now.Connected:
		events = append(events, serverEvent(EventServerDisconnected, was.Server, at))

	case was.Connected && now.Connected && serverKey(was.Server) != serverKey(now.Server):
		/*
			Bascule d'un serveur à l'autre sans qu'aucun instantané n'ait vu
			l'état déconnecté — ce qui est le cas courant, la bascule prenant
			moins d'une seconde.

			Sans cette branche, l'interface continuerait d'afficher le serveur
			précédent jusqu'à la prochaine déconnexion réelle.
		*/
		events = append(events,
			serverEvent(EventServerDisconnected, was.Server, at),
			serverEvent(EventServerConnected, now.Server, at))
	}

	wasKad, nowKad := previous.Connection.Kad, current.Connection.Kad
	switch {
	case !wasKad.Connected && nowKad.Connected:
		events = append(events, Event{Kind: EventKadConnected, At: at})
	case wasKad.Connected && !nowKad.Connected:
		events = append(events, Event{Kind: EventKadDisconnected, At: at})
	}

	return events
}

// serverEvent construit un événement de serveur, en tolérant l'absence de
// description : le démon peut se dire connecté sans avoir encore renseigné le
// serveur, et cela ne doit pas coûter un panic.
func serverEvent(kind EventKind, server *Server, at time.Time) Event {
	event := Event{Kind: kind, At: at}
	if server == nil {
		return event
	}
	event.Name = server.Name
	event.Detail = serverKey(server)
	return event
}

// serverKey identifie un serveur par son adresse, la seule chose qui ne change
// pas quand il renomme sa bannière.
func serverKey(server *Server) string {
	if server == nil {
		return ""
	}
	return net.JoinHostPort(server.IP, strconv.Itoa(server.Port))
}

/*
downloadEvents dérive les changements de la file de téléchargement.

Deux parcours, et pas un seul : les fichiers présents dans le courant donnent
les apparitions et les changements d'état, ceux qui n'y sont plus donnent les
fins. Les tranches sont parcourues dans leur ordre, les maps ne servent qu'à
retrouver un hash.
*/
func downloadEvents(previous, current *Snapshot, at time.Time) []Event {
	before := indexDownloads(previous.Downloads)
	after := indexDownloads(current.Downloads)

	var events []Event

	for i := range current.Downloads {
		file := &current.Downloads[i]

		was, known := before[file.Hash]
		if !known {
			/*
				Apparu depuis la dernière comparaison.

				Un fichier qui apparaît DÉJÀ terminé n'a pas « démarré » du point
				de vue de l'utilisateur : il a été téléchargé entièrement dans
				l'intervalle. Annoncer les deux ferait s'afficher « démarré »
				puis « terminé » dans la même seconde, ce qui décrit mal ce qui
				s'est passé.
			*/
			if file.Status == DownloadCompleted {
				events = append(events, downloadEvent(EventDownloadCompleted, file, at))
				continue
			}
			events = append(events, downloadEvent(EventDownloadStarted, file, at))
			continue
		}

		if kind, changed := statusChange(was.Status, file.Status); changed {
			events = append(events, downloadEvent(kind, file, at))
		}
	}

	for i := range previous.Downloads {
		file := &previous.Downloads[i]
		if _, still := after[file.Hash]; still {
			continue
		}

		/*
			Sorti de la file. amuled y retire ce qu'il a fini d'assembler, donc
			une disparition est le cas NORMAL d'un téléchargement réussi — c'est
			même la seule façon de voir la fin d'un fichier assez petit pour que
			l'état « completed » ne soit jamais observé.

			Reste à distinguer la fin de l'annulation, sans quoi supprimer un
			téléchargement à mi-parcours annoncerait un fichier prêt.
		*/
		switch {
		case file.Status == DownloadCompleted:
			// Déjà annoncé quand le statut est passé à « terminé » : le
			// réannoncer ferait deux notifications pour un fichier.
		case file.Status == DownloadCompleting || acquired(file):
			events = append(events, downloadEvent(EventDownloadCompleted, file, at))
		default:
			events = append(events, downloadEvent(EventDownloadRemoved, file, at))
		}
	}

	return events
}

// statusChange traduit une transition de statut en événement, ou dit qu'elle
// n'en mérite aucun.
//
// L'ordre des tests n'est pas indifférent : une fin et une erreur priment sur
// une reprise, faute de quoi un fichier qui passe de « en pause » à « terminé »
// produirait un « repris » que personne n'a demandé.
func statusChange(was, now DownloadStatus) (EventKind, bool) {
	if was == now {
		return "", false
	}

	switch {
	case now == DownloadCompleted:
		return EventDownloadCompleted, true

	case now == DownloadErroneous:
		return EventDownloadError, true

	case suspended(now) && !suspended(was):
		return EventDownloadPaused, true

	case suspended(was) && active(now):
		return EventDownloadResumed, true
	}

	// Tout le reste — « waiting » qui devient « downloading », « hashing » qui
	// devient « allocating » — est du détail d'exécution. Il est dans
	// l'instantané, et en faire des notifications noierait les vrais faits.
	return "", false
}

// suspended regroupe les deux façons dont un téléchargement cesse à la demande
// de l'utilisateur. L'interface les présente pareil : une barre à l'arrêt.
func suspended(status DownloadStatus) bool {
	return status == DownloadPaused || status == DownloadStopped
}

// active dit qu'un téléchargement est de nouveau dans la file de travail —
// qu'il reçoive des données ou qu'il attende une source.
func active(status DownloadStatus) bool {
	return status == DownloadDownloading || status == DownloadWaiting
}

// acquired dit qu'un fichier avait tout ce qu'il fallait au moment où on l'a vu
// pour la dernière fois.
func acquired(file *Download) bool {
	return file.Size > 0 && file.SizeDone >= file.Size
}

func downloadEvent(kind EventKind, file *Download, at time.Time) Event {
	return Event{Kind: kind, Hash: file.Hash, Name: file.Name, At: at}
}

func indexDownloads(files []Download) map[string]*Download {
	index := make(map[string]*Download, len(files))
	for i := range files {
		index[files[i].Hash] = &files[i]
	}
	return index
}

/*
uploadEvents dérive les changements des transferts sortants.

La clé est le couple (pair, fichier) : le même pair peut recevoir deux fichiers,
et le même fichier partir vers dix pairs.

« Terminé » est un raccourci assumé — un envoi disparaît de la liste qu'il soit
allé au bout ou que le pair se soit déconnecté, et le protocole ne donne rien
qui permette de trancher. L'interface doit donc dire « envoi terminé », pas
« fichier envoyé ».
*/
func uploadEvents(previous, current *Snapshot, at time.Time) []Event {
	before := indexUploads(previous.Uploads)
	after := indexUploads(current.Uploads)

	var events []Event

	for i := range current.Uploads {
		upload := &current.Uploads[i]
		if _, known := before[uploadKey(upload)]; !known {
			events = append(events, uploadEvent(EventUploadStarted, upload, at))
		}
	}

	for i := range previous.Uploads {
		upload := &previous.Uploads[i]
		if _, still := after[uploadKey(upload)]; !still {
			events = append(events, uploadEvent(EventUploadCompleted, upload, at))
		}
	}

	return events
}

func uploadKey(upload *Upload) string {
	return upload.UserHash + "|" + upload.FileHash
}

func uploadEvent(kind EventKind, upload *Upload, at time.Time) Event {
	return Event{
		Kind:   kind,
		Hash:   upload.FileHash,
		Name:   upload.FileName,
		Detail: upload.Name,
		At:     at,
	}
}

func indexUploads(uploads []Upload) map[string]*Upload {
	index := make(map[string]*Upload, len(uploads))
	for i := range uploads {
		index[uploadKey(&uploads[i])] = &uploads[i]
	}
	return index
}
