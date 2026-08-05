package amule

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
)

/*
Traduction des réponses EC qui décrivent les COMPTEURS, les ENVOIS et les
FICHIERS PARTAGÉS.

Trois règles valent pour tout le fichier :

  - Un tag ABSENT n'est jamais une erreur. amuled n'écrit un tag que lorsqu'il a
    quelque chose à dire, et il en omet d'autres selon le niveau de détail
    demandé. Un champ sans tag garde sa valeur nulle ; seul un opcode inattendu
    fait échouer un décodage, parce que lui seul signifie « ce paquet ne parle
    pas de ce que je crois ».

  - Les DÉBITS du domaine sont en octets par seconde, et tous ceux que ce
    fichier lit le sont déjà côté amuled. C'est le piège inverse de l'habituel :
    les plafonds se CONFIGURENT en kilo-octets par seconde dans amule.conf, mais
    le démon les multiplie par 1024 avant de les mettre sur le fil
    (ExternalConn.cpp, `thePrefs::GetMaxUpload()*1024.0`). Reconvertir ici
    donnerait des plafonds mille fois trop grands. Les débits instantanés, eux,
    viennent de compteurs documentés « bytes/second » (Statistics.h,
    `CPreciseRateCounter::GetRate`). Le seul champ réellement en kilo-octets de
    tout le protocole est le débit DESCENDANT d'un pair — un tag de type
    « double » qui appartient aux téléchargements, pas à ce fichier.

  - Les empreintes sont rendues en hexadécimal MINUSCULE. C'est la forme qu'on
    stocke, qu'on compare et qu'on met dans une URL ; deux graphies de la même
    empreinte donneraient deux fichiers là où il n'y en a qu'un.
*/

// ─── Codes du démon ──────────────────────────────────────────────────────────

/*
uploadStateUploading est le code d'un pair à qui les données partent VRAIMENT.

amuled numérote les états d'envoi de 0 à 8 (Constants.h, `enum EUploadState`) :
0 est le seul qui décrive un transfert en cours ; les huit autres décrivent des
pairs qui occupent une place dans la file sans rien recevoir — en attente de
créneau, en cours de connexion, rappel en attente, bannis, en erreur.

Le nom des huit autres ne nous sert à rien : ce qui compte est la frontière, et
elle est binaire.
*/
const uploadStateUploading uint64 = 0

/*
Codes de priorité d'un fichier partagé.

Deux choses à savoir, et aucune ne se devine :

  - La numérotation n'est pas ordonnée. « Très basse » vaut 4, au-dessus de
    « très haute » qui vaut 3. Trier sur l'entier classerait à l'envers.

  - La priorité AUTOMATIQUE ne se dit pas par un code mais par un DÉCALAGE de
    dix ajouté au code effectif : amuled écrit `priorité + 10` quand il gère la
    priorité lui-même (ECSpecialCoreTags.cpp), et son propre client relit
    `code % 10` avec `code >= 10` comme drapeau (TextClient.cpp). Lire le code
    brut donnerait donc une priorité inconnue là où le démon dit « automatique ».
*/
const (
	sharePrioLow      uint64 = 0
	sharePrioNormal   uint64 = 1
	sharePrioHigh     uint64 = 2
	sharePrioVeryHigh uint64 = 3
	sharePrioVeryLow  uint64 = 4
	sharePrioAuto     uint64 = 5
	sharePrioRelease  uint64 = 6

	// sharePrioAutoOffset est le décalage qui marque « géré par le démon ».
	sharePrioAutoOffset uint64 = 10
)

/*
clientSoftwareNames traduit le code numérique du logiciel d'un pair.

Transcrit de la table de correspondance d'aMule (ClientSoftware.h pour les
codes, DataToText.cpp pour les libellés). La numérotation est trouée et un même
programme y apparaît plusieurs fois : Shareaza et MLDonkey ont changé de code en
changeant de schéma de version, et les anciens codes restent en circulation tant
que d'anciens clients tournent.

Le code 0x36 — « inconnu » du point de vue d'amuled — est délibérément ABSENT de
cette table : le rendre en clair afficherait « inconnu » là où l'on peut au
moins dire quel code a été vu. Voir clientSoftwareName.
*/
var clientSoftwareNames = map[uint64]string{
	0x00: "eMule",
	0x35: "eMule", // ancien schéma de version, toujours en circulation
	0x01: "cDonkey",
	0x02: "(l/x)Mule",
	0x03: "aMule",
	0x04: "Shareaza",
	0x28: "Shareaza",
	0x44: "Shareaza",
	0x05: "eMule+",
	0x06: "HydraNode",
	0x0a: "MLDonkey",
	0x34: "MLDonkey",
	0x98: "MLDonkey",
	0x14: "lphant",
	0x32: "eDonkeyHybrid",
	0x33: "eDonkey",
	0xff: "eMule compatible", // dit d'un client qui parle le dialecte sans être eMule
}

// ─── Statistiques ────────────────────────────────────────────────────────────

/*
requestStats construit la demande des compteurs du démon.

# Pourquoi le niveau « complet »

Le niveau de détail ne fait pas que doser le bavardage : il change la LISTE des
compteurs renvoyés (ExternalConn.cpp, Get_EC_Response_StatRequest). Les quatre
niveaux se comportent ainsi :

  - « mise à jour » (DetailUpdate) ne renvoie RIEN. Le démon sort immédiatement.
  - « ligne de commande » et « web » renvoient les débits, les plafonds, la
    longueur de la file d'envoi, le nombre de sources et la taille des deux
    réseaux.
  - « complet » y ajoute le SURDÉBIT montant et descendant et le nombre de pairs
    bannis — trois champs que notre `Stats` porte et qu'aucun autre niveau ne
    donne.
  - « incrémental » renvoie les mêmes compteurs que « complet », mais engage le
    démon dans un dialogue à état : il ne réémet plus que ce qui a changé depuis
    notre dernière demande, pour CETTE connexion. Un instantané qui dépend de
    l'historique de la connexion n'est pas un instantané.

D'où « complet », le seul niveau qui remplisse `Stats` sans mémoire cachée.
*/
func requestStats() ec.Packet {
	return ec.New(ec.OpStatReq,
		ec.Uint(ec.TagDetailLevel, uint64(ec.DetailFull)),
	)
}

/*
decodeStats traduit la réponse en compteurs.

# Ce que ce décodeur ignore volontairement

La réponse porte AUSSI le tag d'état de connexion : amuled l'agrafe à la fin de
toute réponse aux statistiques (ExternalConn.cpp, le `AddTag(CEC_ConnState_Tag)`
qui suit l'appel). Il n'est pas lu ici. `decodeConnection` sait déjà le lire, et
deux décodeurs pour un même tag, c'est deux vérités qui divergeront le jour où
l'un des deux sera corrigé. L'appelant qui veut l'état passe le même paquet à
`decodeConnection` — c'est prévu, ce décodeur-là ne vérifie pas l'opcode.

Sont ignorés de même : les lignes de journal (un tag imbriqué de chaînes que le
démon vide à chaque demande — le lire ici en ferait un canal de journalisation
déguisé), les compteurs Kad d'indexation, et les cumuls d'octets émis et reçus,
qui n'ont pas de place dans `Stats`.
*/
func decodeStats(p ec.Packet) (Stats, error) {
	// Le protocole n'a aucun identifiant de corrélation : les réponses arrivent
	// dans l'ordre des requêtes, et si elles se décalaient d'un cran on
	// décoderait ici un paquet qui parle d'autre chose. Un opcode inattendu se
	// signale plutôt que de rendre des compteurs à zéro qui passeraient pour un
	// démon au repos.
	if p.Op != ec.OpStats {
		return Stats{}, fmt.Errorf(
			"statistiques : réponse %s, attendu %s", p.Op, ec.OpStats)
	}

	var stats Stats
	for _, tag := range p.Tags {
		v, ok := tag.Uint()
		if !ok {
			// Tag non entier : le journal, l'adresse Kad. Rien à en tirer ici.
			continue
		}

		switch tag.Name {
		case ec.TagStatsUlSpeed:
			stats.UpSpeed = int64(v)
		case ec.TagStatsDlSpeed:
			stats.DownSpeed = int64(v)

		// Plafonds déjà convertis en octets par seconde par le démon. Zéro n'est
		// pas « débit nul » mais « aucune limite » : c'est la valeur
		// qu'amuled donne à un plafond désactivé (UNLIMITED vaut 0 chez lui), et
		// c'est ce que documente `Stats`.
		case ec.TagStatsUlSpeedLimit:
			stats.UpLimit = int64(v)
		case ec.TagStatsDlSpeedLimit:
			stats.DownLimit = int64(v)

		case ec.TagStatsUpOverhead:
			stats.UpOverhead = int64(v)
		case ec.TagStatsDownOverhead:
			stats.DownOverhead = int64(v)

		case ec.TagStatsTotalSrcCount:
			stats.TotalSources = int(v)
		case ec.TagStatsBannedCount:
			stats.BannedPeers = int(v)
		case ec.TagStatsUlQueueLen:
			stats.UploadQueueLength = int(v)

		case ec.TagStatsEd2kUsers:
			stats.Ed2kUsers = int(v)
		case ec.TagStatsKadUsers:
			stats.KadUsers = int(v)
		case ec.TagStatsEd2kFiles:
			stats.Ed2kFiles = int(v)
		case ec.TagStatsKadFiles:
			stats.KadFiles = int(v)
		}
	}

	return stats, nil
}

// ─── Envois ──────────────────────────────────────────────────────────────────

/*
requestUploads construit la demande de la file d'envoi.

Même niveau de détail que partout ailleurs, et pour la même raison qu'aux
statistiques : « incrémental » ferait tenir au démon un journal des valeurs déjà
transmises sur cette connexion et n'enverrait plus que les différences. Il
donnerait en échange la version détaillée du client et le nombre de parties
qu'il détient — deux choses dont `Upload` n'a que faire.

« Complet » ajoute, lui, ce qui nous manque vraiment : le NOM du fichier servi.
Aux niveaux inférieurs, un envoi n'est identifié que par le numéro interne du
fichier chez le démon, qui ne survit pas à son redémarrage.
*/
func requestUploads() ec.Packet {
	return ec.New(ec.OpGetUloadQueue,
		ec.Uint(ec.TagDetailLevel, uint64(ec.DetailFull)),
	)
}

/*
decodeUploads sépare les transferts en cours des pairs qui attendent.

# Le critère

Une seule réponse porte les deux populations, sous des tags de pair identiques.
Ce qui les distingue est l'ÉTAT D'ENVOI (TagClientUploadState), et lui seul :
l'état 0 — « en cours d'envoi » — décrit un pair à qui les données partent ;
tout autre code décrit un pair qui occupe une place sans rien recevoir.

Ce critère est préférable aux deux autres qu'on pourrait imaginer :

  - Se fier au débit serait faux. Un pair qui vient d'obtenir son créneau a un
    débit nul pendant quelques secondes, et il n'est pas pour autant en attente.

  - Se fier à la liste d'où vient le pair serait fragile. Selon la version du
    démon, cette réponse contient la seule liste des envois actifs ou les deux
    listes concaténées ; l'état, lui, est écrit sur chaque pair dans tous les
    cas. Trier sur l'état donne le bon résultat dans les deux situations, et la
    liste des attentes est simplement vide quand le démon ne l'envoie pas.

Un tag d'état absent compte comme « en cours d'envoi » : c'est la valeur nulle,
et c'est aussi le contenu principal de cette réponse.

# Ce que cette réponse NE PORTE PAS

L'empreinte du fichier servi. Le démon ne transmet que son numéro interne
(TagClientUploadFile), inutilisable hors de sa mémoire. `FileHash` reste donc
vide des deux côtés ; le rapprochement, s'il devient nécessaire, se fait à
l'étage au-dessus en croisant ce numéro avec la valeur propre des tags de la
réponse « fichiers partagés », qui porte le même numéro et l'empreinte.

Le temps d'attente d'un pair non plus : amuled a mis ce tag en commentaire dans
son propre code. `WaitedSince` reste nil, ce qui est la réponse honnête à
« depuis quand ? ».
*/
func decodeUploads(p ec.Packet) ([]Upload, []QueuedPeer, error) {
	if p.Op != ec.OpUloadQueue {
		return nil, nil, fmt.Errorf(
			"file d'envoi : réponse %s, attendu %s", p.Op, ec.OpUloadQueue)
	}

	uploads := make([]Upload, 0, len(p.Tags))
	queued := make([]QueuedPeer, 0)

	for _, tag := range p.Tags {
		if tag.Name != ec.TagClient {
			continue
		}

		if uploading(tag) {
			uploads = append(uploads, decodeUpload(tag))
		} else {
			queued = append(queued, decodeQueuedPeer(tag))
		}
	}

	return uploads, queued, nil
}

// uploading dit si ce pair reçoit des données de nous en ce moment.
func uploading(tag ec.Tag) bool {
	state, ok := tag.Find(ec.TagClientUploadState)
	if !ok {
		return true
	}
	code, ok := state.Uint()
	if !ok {
		return true
	}
	return code == uploadStateUploading
}

// decodeUpload traduit un tag de pair en transfert sortant.
func decodeUpload(tag ec.Tag) Upload {
	var upload Upload

	for _, child := range tag.Children {
		switch child.Name {
		case ec.TagClientHash:
			upload.UserHash = tagHashHex(child)
		case ec.TagClientName:
			upload.Name, _ = child.Text()

		case ec.TagClientSoftware:
			if v, ok := child.Uint(); ok {
				upload.Software = clientSoftwareName(v)
			}
		case ec.TagClientSoftVerStr:
			// La version est déjà mise en forme par le démon (« 0.60d »). Le tag
			// numérique de version, lui, n'existe qu'au niveau incrémental.
			upload.Version, _ = child.Text()

		case ec.TagClientUserIP:
			if v, ok := child.Uint(); ok {
				// Même convention que pour les serveurs : l'octet de poids
				// faible est le premier nombre de la notation pointée.
				upload.IP = serverIPv4FromUint(v)
			}
		case ec.TagClientUserPort:
			if v, ok := child.Uint(); ok {
				upload.Port = int(v)
			}

		case ec.TagPartfileName:
			// Dans un tag de pair, ce nom est celui du fichier QU'ON LUI SERT,
			// pas d'un téléchargement à nous.
			upload.FileName, _ = child.Text()

		case ec.TagClientUpSpeed:
			// Déjà en octets par seconde : le démon divise une somme d'octets
			// par une durée en millisecondes (UploadClient.cpp).
			if v, ok := child.Uint(); ok {
				upload.Speed = int64(v)
			}

		case ec.TagClientUploadSession:
			if v, ok := child.Uint(); ok {
				upload.SessionUploaded = int64(v)
				// Le démon n'expose pas d'autre compteur d'octets SORTANTS pour
				// ce transfert : `Transferred` et `SessionUploaded` portent donc
				// la même valeur. Le tag voisin qui ressemble à un compteur de
				// transfert (TagPartfileSizeXfer) mesure l'autre sens — ce que
				// ce pair NOUS a envoyé — et le recopier ici afficherait un
				// envoi qui n'a pas eu lieu.
				upload.Transferred = int64(v)
			}
		case ec.TagClientUploadTotal:
			if v, ok := child.Uint(); ok {
				upload.TotalUploaded = int64(v)
			}
		}
	}

	return upload
}

// decodeQueuedPeer traduit un tag de pair en attente.
//
// Moins de champs que pour un envoi, et ce n'est pas un oubli : un pair qui
// attend n'a ni débit ni fichier en cours de transfert. Ce qui le décrit est sa
// note, et c'est elle qui dira s'il sera servi bientôt.
func decodeQueuedPeer(tag ec.Tag) QueuedPeer {
	var peer QueuedPeer

	for _, child := range tag.Children {
		switch child.Name {
		case ec.TagClientHash:
			peer.UserHash = tagHashHex(child)
		case ec.TagClientName:
			peer.Name, _ = child.Text()

		case ec.TagClientUserIP:
			if v, ok := child.Uint(); ok {
				peer.IP = serverIPv4FromUint(v)
			}
		case ec.TagClientUserPort:
			if v, ok := child.Uint(); ok {
				peer.Port = int(v)
			}

		case ec.TagClientScore:
			if v, ok := child.Uint(); ok {
				peer.Score = int(v)
			}
		}
	}

	return peer
}

// ─── Fichiers partagés ───────────────────────────────────────────────────────

/*
requestSharedFiles construit la demande de la liste des partages.

Le niveau de détail décide ici de bien plus que du volume :

  - « incrémental » fait répondre le démon par RIEN du tout. Il ne construit
    aucune réponse pour ce niveau (ExternalConn.cpp) et l'appelant attendrait
    une réponse qui ne vient pas.

  - « mise à jour » ne renvoie que les compteurs, sans nom, sans empreinte, sans
    taille — et, si la connexion l'a négocié, seulement les fichiers modifiés
    depuis la demande précédente.

  - « complet » renvoie chaque fichier vivant, en entier, sans mémoire d'une
    demande à l'autre.

Une note pour plus tard : à ce niveau, un démon récent emprunte un chemin de
réponse accéléré si la connexion a négocié le comptage étendu des tags ET
l'encodage compact des entiers. Notre session ne négocie ni l'un ni l'autre
(voir ec.Conn), et le jour où elle le ferait, c'est ce chemin-là qu'il faudrait
revérifier en premier.
*/
func requestSharedFiles() ec.Packet {
	return ec.New(ec.OpGetSharedFiles,
		ec.Uint(ec.TagDetailLevel, uint64(ec.DetailFull)),
	)
}

/*
decodeSharedFiles traduit la réponse en liste de partages.

L'ordre du démon est conservé. Il n'a rien de significatif — c'est celui de sa
table interne — mais le réordonner ici obligerait l'interface à re-trier une
liste déjà triée pour rien.

La liste mélange les fichiers entiers et les téléchargements en cours : un
fichier incomplet est partagé lui aussi dès qu'une de ses parties est complète,
et c'est ainsi que le réseau se propage. Voir `SharedFile.Complete`.
*/
func decodeSharedFiles(p ec.Packet) ([]SharedFile, error) {
	if p.Op != ec.OpSharedFiles {
		return nil, fmt.Errorf(
			"fichiers partagés : réponse %s, attendu %s", p.Op, ec.OpSharedFiles)
	}

	files := make([]SharedFile, 0, len(p.Tags))
	for _, tag := range p.Tags {
		if tag.Name != ec.TagKnownfile {
			continue
		}
		files = append(files, decodeSharedFile(tag))
	}
	return files, nil
}

/*
decodeSharedFile traduit un tag de fichier partagé.

# Les compteurs sont pris en cumul, pas en session

Le démon envoie chaque compteur en deux exemplaires : depuis son démarrage, et
depuis toujours. Ce sont les seconds qui sont lus ici. `SharedFile` sert à dire
quel fichier intéresse le réseau, et un compteur remis à zéro à chaque
redémarrage du démon répondrait surtout « quand a-t-il redémarré ».

# La complétude ne se lit nulle part

Aucun tag ne dit si le fichier est entier. Le seul indice est le CHEMIN : pour un
fichier complet, le démon envoie son emplacement sur le disque ; pour un
téléchargement en cours, il envoie le nom de son fichier de suivi, qui est un
numéro d'ordre suivi de « .part ». C'est le test que fait le client en ligne de
commande d'aMule lui-même (TextClient.cpp) pour afficher « [PartFile] », et
c'est donc ce test qui est transcrit ici — faute de mieux, et en le sachant.
*/
func decodeSharedFile(tag ec.Tag) SharedFile {
	file := SharedFile{
		// amuled écrit toujours la priorité d'un partage, mais la valeur normale
		// reste le défaut le plus sûr si le tag venait à manquer : c'est celle
		// qu'il donne lui-même à un fichier nouvellement partagé.
		Priority: PriorityNormal,
	}

	for _, child := range tag.Children {
		switch child.Name {
		case ec.TagPartfileHash:
			file.Hash = tagHashHex(child)
		case ec.TagPartfileName:
			file.Name, _ = child.Text()
		case ec.TagPartfileSizeFull:
			if v, ok := child.Uint(); ok {
				file.Size = int64(v)
			}

		case ec.TagKnownfileFilename:
			path, _ := child.Text()
			file.Path = path
			file.Complete = path != "" && !isPartMetName(path)

		case ec.TagKnownfilePrio:
			if v, ok := child.Uint(); ok {
				file.Priority = sharedFilePriority(v)
			}

		case ec.TagKnownfileReqCountAll:
			if v, ok := child.Uint(); ok {
				file.Requests = int64(v)
			}
		case ec.TagKnownfileAcceptCountAll:
			if v, ok := child.Uint(); ok {
				file.Accepted = int64(v)
			}
		case ec.TagKnownfileXferredAll:
			if v, ok := child.Uint(); ok {
				file.Transferred = int64(v)
			}
		}
	}

	return file
}

// ─── Traductions élémentaires ────────────────────────────────────────────────

/*
clientSoftwareName traduit le code de logiciel d'un pair en nom lisible.

Un code hors table est rendu TEL QUEL, en décimal. C'est délibéré : « inconnu »
ne se distingue pas d'un autre « inconnu », alors qu'un code permet de savoir
qu'un même client non répertorié revient, et de compléter la table le jour où on
l'identifie. Le code 54, qu'amuled affiche « Unknown », passe donc lui aussi par
ce chemin.
*/
func clientSoftwareName(code uint64) string {
	if name, ok := clientSoftwareNames[code]; ok {
		return name
	}
	return strconv.FormatUint(code, 10)
}

/*
sharedFilePriority traduit le code de priorité d'un fichier partagé.

Le décalage de dix est traité en premier : il dit que le démon pilote la
priorité, et c'est cela que l'interface doit montrer. Le niveau effectif du
moment est perdu au passage, faute d'un champ pour le porter — et l'afficher
serait de toute façon trompeur, puisqu'il change sans que l'utilisateur y soit
pour rien.

Le code 6 — « diffusion », au-dessus de « très haute » chez eMule — n'a pas
d'équivalent dans le domaine. Il retombe sur « très haute », ce qui préserve
l'ordre plutôt que d'afficher un blanc. Un code hors table retombe sur
« normale », comme le fait le démon quand il relit une priorité qu'il ne
reconnaît pas.
*/
func sharedFilePriority(code uint64) Priority {
	if code >= sharePrioAutoOffset {
		return PriorityAuto
	}

	switch code {
	case sharePrioLow:
		return PriorityLow
	case sharePrioNormal:
		return PriorityNormal
	case sharePrioHigh:
		return PriorityHigh
	case sharePrioVeryHigh, sharePrioRelease:
		return PriorityVeryHigh
	case sharePrioVeryLow:
		return PriorityVeryLow
	case sharePrioAuto:
		return PriorityAuto
	default:
		return PriorityNormal
	}
}

/*
isPartMetName dit si ce chemin est le nom d'un fichier de suivi de
téléchargement.

La forme est « un numéro d'ordre, puis .part » — le démon construit ce nom en
retirant l'extension .met du fichier de suivi qu'il tient à côté du
téléchargement. La vérification porte sur les DEUX moitiés : sans le contrôle
des chiffres, un vrai fichier partagé nommé « mon film.part » serait pris pour un
téléchargement en cours.
*/
func isPartMetName(path string) bool {
	const suffix = ".part"

	number, found := strings.CutSuffix(path, suffix)
	if !found || number == "" {
		return false
	}
	return strings.IndexFunc(number, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

// tagHashHex rend une empreinte de seize octets en hexadécimal minuscule.
//
// Un tag qui n'est pas une empreinte rend la chaîne vide plutôt qu'une erreur :
// une empreinte manquante se voit tout de suite en aval, alors qu'une empreinte
// inventée à partir d'octets mal typés voyagerait jusqu'en base.
func tagHashHex(tag ec.Tag) string {
	h, ok := tag.Hash()
	if !ok {
		return ""
	}
	return hex.EncodeToString(h)
}
