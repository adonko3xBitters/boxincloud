package amule

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
)

/*
Traduction des réponses EC portant sur les téléchargements et leurs sources.

Ce fichier est le seul du module à connaître à la fois un nom de tag et un type
du domaine. Tout ce qui est écrit ici est transcrit de la façon dont amuled
CONSTRUIT ses réponses — `ECSpecialCoreTags.cpp` pour la forme des tags,
`Constants.h` pour les codes, `PartFile.cpp` pour la façon dont aMule lui-même
relit ces codes. Les tables portent leur source en commentaire : quand une
version d'aMule les changera, c'est là qu'il faudra retourner.

# Trois pièges valent d'être annoncés en tête

 1. Le tag d'un fichier ne porte PAS son empreinte. Sa valeur est l'ECID, un
    compteur interne au démon qui repart de zéro à chaque redémarrage
    (`CECID`, src/libs/ec/cpp/ECID.h). L'empreinte est un ENFANT,
    `TagPartfileHash`, et le démon ne l'envoie pas à tous les niveaux de
    détail. Voir requestDownloads pour la conséquence.

 2. Un tag absent est normal. Aux niveaux de détail incrémentaux, amuled
    n'envoie que ce qui a bougé depuis le dernier envoi sur CETTE connexion
    (`CValueMap`). Un décodeur qui exigerait un champ produirait des erreurs
    aléatoires, une fois sur deux, sans rien de fautif en face. Rien ici n'est
    obligatoire : l'absence laisse le zéro.

 3. Le protocole n'a aucune opération « donne-moi les sources de ce fichier ».
    Voir requestSources, qui porte l'explication complète.
*/

// ─── Codes du démon ──────────────────────────────────────────────────────────

// États d'un fichier en cours, transcrits de src/Constants.h (PS_*).
//
// Le trou entre 10 et l'infini n'est pas décoratif : tout code au-delà vient
// d'une version d'aMule plus récente que cette table, et se traduit en
// DownloadUnknown plutôt qu'en supposition.
const (
	psReady           = 0  // prêt, la file décide de la suite
	psEmpty           = 1  // rien de reçu encore
	psWaitingForHash  = 2  // en attente de vérification
	psHashing         = 3  // vérification en cours
	psError           = 4  // en erreur
	psInsufficient    = 5  // plus de place sur le disque
	psUnknown         = 6  // état indéterminé côté démon
	psPaused          = 7  // suspendu
	psCompleting      = 8  // assemblage
	psComplete        = 9  // terminé
	psAllocating      = 10 // réservation de l'espace disque
	psHighestKnown    = psAllocating
	psTransferringMin = 1 // au moins une source qui transfère : voir mapPartfileStatus
)

// Priorités, transcrites de src/Constants.h (PR_*).
//
// La numérotation surprend : « très basse » vaut 4 et non -1. Le commentaire
// d'aMule l'explique — les priorités étaient signées à l'origine, et la
// sérialisation ne conservait pas le signe.
const (
	prLow        = 0
	prNormal     = 1
	prHigh       = 2
	prVeryHigh   = 3
	prVeryLow    = 4
	prAuto       = 5
	prAutoOffset = 10 // décalage du drapeau « auto », voir mapPartfilePriority
)

// États d'un pair vis-à-vis d'un de nos téléchargements, transcrits de
// src/Constants.h (enum EDownloadState).
//
// dsDownloading vaut ZÉRO. Un décodeur qui ignorerait la présence du tag
// verrait « en train de télécharger » partout où le démon n'a rien envoyé.
const dsDownloading = 0

// hybridHighID est le seuil de src/NetworkFunctions.h : un identifiant en
// dessous est un LowID, c'est-à-dire un pair qui n'accepte pas de connexion
// entrante.
const hybridHighID = 16777216

// ed2kPartSize est la taille d'une partie eD2k, en octets
// (src/include/protocol/ed2k/Constants.h). Le nombre de parties d'un fichier
// s'en déduit : le démon ne l'envoie pas.
const ed2kPartSize = 9728000

// ─── Téléchargements ─────────────────────────────────────────────────────────

/*
requestDownloads construit la requête de file de téléchargement.

Le niveau de détail demandé est `DetailFull`, et ce n'est pas un excès de
prudence. `DetailUpdate` fait retourner amuled AVANT d'écrire le nom, la taille
et l'empreinte (`CEC_SharedFile_Tag`, ECSpecialCoreTags.cpp) : la réponse ne
contient plus que des compteurs rattachés à un ECID, qui ne survit pas au
redémarrage du démon. Comme l'empreinte est notre clé stable, elle doit arriver
à chaque tour.

Piège pour plus tard : amuled sert cette requête depuis un cache de trames
pré-encodées dès que le client a négocié `FlagUTF8Numbers` ET
`FlagLargeTagCount` (ExternalConn.cpp, EC_OP_GET_DLOAD_QUEUE). Le paquet
change alors de forme. Notre session ne négocie ni l'un ni l'autre — si cela
change, ce décodeur est à revérifier.
*/
func requestDownloads() ec.Packet {
	return ec.New(ec.OpGetDloadQueue,
		ec.Uint(ec.TagDetailLevel, uint64(ec.DetailFull)),
	)
}

/*
decodeDownloads traduit la réponse du démon.

Rend une tranche vide, jamais nil, quand la file l'est : « aucun
téléchargement » est un état normal, pas une absence de réponse, et un appelant
qui itère dessus ne doit pas avoir à distinguer les deux.
*/
func decodeDownloads(p ec.Packet) ([]Download, error) {
	// OpSharedFiles est accepté parce que la mise à jour globale
	// (OpGetUpdate) mêle fichiers partagés et fichiers en cours dans un
	// paquet qui porte ce code — voir Get_EC_Response_GetUpdate. Les tags
	// TagKnownfile qui s'y trouvent aussi sont simplement ignorés ici.
	if p.Op != ec.OpDloadQueue && p.Op != ec.OpSharedFiles {
		return nil, fmt.Errorf(
			"file de téléchargement : réponse %s, attendu %s", p.Op, ec.OpDloadQueue)
	}

	downloads := make([]Download, 0, len(p.Tags))
	for _, tag := range p.Tags {
		if tag.Name != ec.TagPartfile {
			continue
		}
		// Un tag de fichier SANS enfant est un marqueur de présence : il dit
		// « ce fichier existe toujours », rien de plus. Le construire
		// donnerait une entrée sans nom ni taille — le fantôme contre lequel
		// amulegui se garde explicitement (ExternalConn.cpp, alive marker).
		if len(tag.Children) == 0 {
			continue
		}
		downloads = append(downloads, downloadFromTag(tag))
	}
	return downloads, nil
}

/*
decodeDownloadIDs rend la correspondance ECID → empreinte lue dans la même
réponse.

Elle existe parce que le démon désigne un fichier par son ECID partout ailleurs
— notamment dans les tags de pairs, seul endroit où les sources sont décrites.
Sans cette table, une source ne peut être rattachée à aucun fichier. Voir
requestSources.

Les fichiers dont le démon n'a pas renvoyé l'empreinte sont absents de la table :
on ne peut rien en dire.
*/
func decodeDownloadIDs(p ec.Packet) (map[uint32]string, error) {
	if p.Op != ec.OpDloadQueue && p.Op != ec.OpSharedFiles {
		return nil, fmt.Errorf(
			"file de téléchargement : réponse %s, attendu %s", p.Op, ec.OpDloadQueue)
	}

	ids := make(map[uint32]string, len(p.Tags))
	for _, tag := range p.Tags {
		if tag.Name != ec.TagPartfile || len(tag.Children) == 0 {
			continue
		}
		ecid, ok := tag.Uint()
		if !ok {
			continue
		}
		if hash := dlChildHashHex(tag, ec.TagPartfileHash); hash != "" {
			ids[uint32(ecid)] = hash
		}
	}
	return ids, nil
}

// downloadFromTag traduit un tag de fichier.
//
// Ne rend pas d'erreur, et c'est délibéré : au niveau du tag, tout est
// facultatif. Une valeur manquante est une valeur que le démon n'a pas jugé
// utile de répéter, pas une trame abîmée — la trame, elle, a déjà été validée
// par le codec.
func downloadFromTag(tag ec.Tag) Download {
	d := Download{
		Hash:     dlChildHashHex(tag, ec.TagPartfileHash),
		Name:     dlChildText(tag, ec.TagPartfileName),
		Size:     dlChildInt64(tag, ec.TagPartfileSizeFull),
		SizeDone: dlChildInt64(tag, ec.TagPartfileSizeDone),
		SizeXfer: dlChildInt64(tag, ec.TagPartfileSizeXfer),
		Speed:    dlChildInt64(tag, ec.TagPartfileSpeed),
		Category: int(dlChildInt64(tag, ec.TagPartfileCat)),
		Sources: SourceCounts{
			Total:        int(dlChildInt64(tag, ec.TagPartfileSourceCount)),
			NotCurrent:   int(dlChildInt64(tag, ec.TagPartfileSourceCountNotCurrent)),
			Transferring: int(dlChildInt64(tag, ec.TagPartfileSourceCountXfer)),
			A4AF:         int(dlChildInt64(tag, ec.TagPartfileSourceCountA4AF)),
		},
		AvailableParts: int(dlChildInt64(tag, ec.TagPartfileAvailableParts)),
	}

	d.PartCount = ed2kPartCount(d.Size)

	status, statusKnown := dlChildUint(tag, ec.TagPartfileStatus)
	stopped := dlChildInt64(tag, ec.TagPartfileStopped) != 0
	d.Status = mapPartfileStatus(status, statusKnown, stopped, d.Sources.Transferring)

	if prio, ok := dlChildUint(tag, ec.TagPartfilePrio); ok {
		d.Priority = mapPartfilePriority(prio)
	}

	// Zéro veut dire « jamais vu complet », pas « vu complet le 1er janvier
	// 1970 ». Rendre l'époque Unix serait affiché tel quel par l'interface.
	if seen := dlChildInt64(tag, ec.TagPartfileLastSeenComp); seen > 0 {
		t := time.Unix(seen, 0).UTC()
		d.LastSeenComplete = &t
	}

	d.ETA = partfileETA(d.Size, d.SizeDone, d.Speed, d.Status)
	return d
}

/*
mapPartfileStatus traduit le code d'état du démon.

Transcrit de `CPartFile::getPartfileStatus` (src/PartFile.cpp) : c'est la
fonction avec laquelle aMule affiche lui-même cette colonne, donc la seule
lecture de ces codes qui fasse autorité.

Deux subtilités qui ne se devinent pas :

  - « arrêté » n'est PAS un code d'état, c'est un tag séparé
    (TagPartfileStopped). Un fichier peut très bien être « prêt » ET arrêté.
    Le drapeau l'emporte sur presque tout — mais pas sur « terminé », ni sur
    les états de vérification et de réservation, qui décrivent un travail en
    cours que l'arrêt ne suspend pas.

  - « en cours » ne vient d'aucun code. Les codes 0, 1 et 6 signifient tous
    « la file s'en occupe » ; c'est le NOMBRE DE SOURCES QUI TRANSFÈRENT qui
    départage « en cours » de « en attente ».
*/
func mapPartfileStatus(code uint64, known, stopped bool, transferring int) DownloadStatus {
	// Sans code, il ne reste que le drapeau. Un fichier arrêté est arrêté,
	// même si le démon n'a pas répété son état.
	if !known {
		if stopped {
			return DownloadStopped
		}
		return DownloadWaiting
	}

	// Hors du champ du drapeau « arrêté » : ces deux états décrivent un
	// traitement local, que l'arrêt du transfert ne concerne pas.
	switch code {
	case psHashing, psWaitingForHash:
		return DownloadHashing
	case psAllocating:
		return DownloadAllocating
	}

	if code > psHighestKnown {
		return DownloadUnknown
	}

	var status DownloadStatus
	switch code {
	case psCompleting:
		status = DownloadCompleting
	case psComplete:
		status = DownloadCompleted
	case psPaused:
		status = DownloadPaused
	case psError:
		status = DownloadErroneous
	case psInsufficient:
		// aMule affiche « Insufficient disk space ». Le domaine n'a pas de
		// constante pour ce cas et c'est bien une erreur : le fichier
		// n'avancera plus tant que rien n'est fait. Le détail se perd, la
		// nature du problème non.
		status = DownloadErroneous
	default: // psReady, psEmpty, psUnknown
		if transferring >= psTransferringMin {
			status = DownloadDownloading
		} else {
			status = DownloadWaiting
		}
	}

	if stopped && code != psComplete {
		return DownloadStopped
	}
	return status
}

/*
mapPartfilePriority traduit le code de priorité.

amuled ne transmet pas le drapeau « automatique » à part : il l'ajoute à la
priorité, décalée de dix (`file->IsAutoDownPriority() ? GetDownPriority() + 10
: GetDownPriority()`, ECSpecialCoreTags.cpp). Un lecteur qui prendrait le code
au pied de la lettre verrait donc des priorités inexistantes — 11, 12 — et
n'afficherait jamais « auto ».

La priorité calculée par le démon est perdue au passage. C'est le choix du
domaine : `PriorityAuto` dit qui décide, et c'est ce que l'utilisateur a réglé.

Un code non reconnu laisse la valeur vide plutôt que d'inventer « normale ».
Le domaine n'a pas de constante « inconnue » pour les priorités, et prétendre
« normale » sur un code que nous ne comprenons pas ferait taire un vrai écart.
*/
func mapPartfilePriority(code uint64) Priority {
	if code >= prAutoOffset {
		return PriorityAuto
	}

	switch code {
	case prLow:
		return PriorityLow
	case prNormal:
		return PriorityNormal
	case prHigh:
		return PriorityHigh
	case prVeryHigh:
		return PriorityVeryHigh
	case prVeryLow:
		return PriorityVeryLow
	case prAuto:
		return PriorityAuto
	default:
		return ""
	}
}

/*
partfileETA calcule le temps restant.

Le démon ne l'envoie pas : elle se déduit de ce qui reste et du débit du
moment. Rendue nil dès qu'elle n'aurait pas de sens — débit nul, fichier en
pause, arrêté, en erreur ou terminé. C'est l'engagement pris par le type
Download : l'interface n'a pas à détecter « l'infini ».

Le calcul se fait en SECONDES avant conversion. Multiplier les octets restants
par une nanoseconde déborderait un entier signé de 64 bits au-delà de neuf
gigaoctets restants, ce qui n'a rien d'exotique pour ce logiciel.
*/
func partfileETA(size, done, speed int64, status DownloadStatus) *time.Duration {
	switch status {
	case DownloadPaused, DownloadStopped, DownloadCompleted,
		DownloadCompleting, DownloadErroneous:
		return nil
	}
	if speed <= 0 {
		return nil
	}

	remaining := size - done
	if remaining <= 0 {
		return nil
	}

	eta := time.Duration(remaining/speed) * time.Second
	return &eta
}

// ed2kPartCount rend le nombre de parties d'un fichier de cette taille.
//
// Transcrit de `CKnownFile::SetFileSize` (src/KnownFile.cpp), y compris le
// rattrapage du dernier morceau : un fichier dont la taille est un multiple
// exact de la taille de partie n'a pas de partie supplémentaire vide.
func ed2kPartCount(size int64) int {
	if size <= 0 {
		return 0
	}
	count := size/ed2kPartSize + 1
	if size%ed2kPartSize == 0 {
		count--
	}
	return int(count)
}

// ─── Sources ─────────────────────────────────────────────────────────────────

/*
requestSources construit la requête des sources d'un fichier.

# Le protocole ne sait pas répondre à cette question

Il n'existe aucune opération EC « sources de ce fichier ». La liste des
opérations est close (src/libs/ec/cpp/ECCodes.h) et aucune n'accepte un
fichier en paramètre pour rendre des pairs :

  - OpGetDloadQueue rend des fichiers, avec des COMPTEURS de sources, jamais
    les sources elles-mêmes ;
  - OpGetUloadQueue rend les pairs que NOUS servons — des envois, pas des
    sources ;
  - OpGetUpdate est le seul endroit où amuled décrit les pairs un par un, et
    il les envoie TOUS, dans un tag conteneur unique, sans filtre possible.

C'est ainsi qu'amulegui procède : il tient la liste complète des pairs et
rattache chacun à son fichier par l'ECID que porte `TagClientRequestFile`.

Conséquence directe sur cette fonction : `hash` ne part pas sur le fil, parce
que le démon n'en ferait rien. Il reste dans la signature parce que l'appelant
doit dire de quel fichier il parle, et parce que le tri se fait à l'arrivée —
voir decodeSourcesByFile, qui groupe par ECID, et decodeDownloadIDs, qui donne
la correspondance ECID → empreinte. Les deux vont ensemble : ni l'une ni
l'autre ne suffit.

Le niveau `DetailIncUpdate` est imposé, pas choisi : amuled ne répond RIEN à
OpGetUpdate demandé à un autre niveau (ExternalConn.cpp). Il implique des
réponses différentielles — le deuxième appel sur une même connexion ne répète
pas ce qui n'a pas bougé — d'où l'importance de ne jamais exiger un tag ici.
*/
func requestSources(hash string) ec.Packet {
	req := ec.New(ec.OpGetUpdate,
		ec.Uint(ec.TagDetailLevel, uint64(ec.DetailIncUpdate)),
	)

	// L'empreinte est jointe quand elle est lisible. amuled l'ignore
	// aujourd'hui — sa boucle de traitement d'OpGetUpdate ne regarde aucun tag
	// de la requête — mais la trame dit alors ce qu'on voulait, et une capture
	// réseau reste interprétable.
	if raw, err := hex.DecodeString(strings.ToLower(hash)); err == nil && len(raw) == 16 {
		req.Tags = append(req.Tags, ec.Hash(ec.TagPartfile, raw))
	}
	return req
}

/*
decodeSources traduit la réponse en sources d'un fichier.

Attention à ce qu'elle rend : TOUS les pairs décrits par le démon, tous
fichiers confondus. Le protocole ne permet pas de faire autrement, et le type
Source n'a aucun champ où loger le fichier concerné. Pour obtenir les sources
d'UN fichier, passer par decodeSourcesByFile. Voir requestSources.
*/
func decodeSources(p ec.Packet) ([]Source, error) {
	byFile, err := decodeSourcesByFile(p)
	if err != nil {
		return nil, err
	}

	sources := make([]Source, 0, len(byFile))
	for _, group := range byFile {
		sources = append(sources, group...)
	}
	return sources, nil
}

/*
decodeSourcesByFile groupe les pairs par le fichier qu'ils nous fournissent.

La clé est l'ECID du fichier, tel que le démon le désigne. La traduire en
empreinte demande la table de decodeDownloadIDs, prise sur la MÊME session : un
ECID est un compteur interne, il ne veut rien dire d'une connexion à l'autre ni
d'un redémarrage à l'autre.

Les pairs qui ne demandent aucun fichier — ceux à qui nous envoyons seulement —
portent un ECID nul et sont écartés : ce ne sont pas des sources.
*/
func decodeSourcesByFile(p ec.Packet) (map[uint32][]Source, error) {
	// OpSharedFiles est le code que porte la réponse à OpGetUpdate
	// (Get_EC_Response_GetUpdate) — le nom trompe, le paquet contient aussi
	// les pairs. OpUloadQueue est accepté pour la file d'envoi, dont les tags
	// de pair ont exactement la même forme.
	if p.Op != ec.OpSharedFiles && p.Op != ec.OpUloadQueue {
		return nil, fmt.Errorf(
			"sources : réponse %s, attendu %s ou %s", p.Op, ec.OpSharedFiles, ec.OpUloadQueue)
	}

	byFile := make(map[uint32][]Source)

	for _, tag := range p.Tags {
		if tag.Name != ec.TagClient {
			continue
		}

		// Deux formes coexistent selon l'opération, et il faut les distinguer
		// sans se tromper : dans la mise à jour globale, amuled range les
		// pairs sous un tag CONTENEUR nommé lui aussi TagClient ; dans la file
		// d'envoi, chaque pair est un tag de premier niveau. Un conteneur se
		// reconnaît à ce que ses enfants portent le même nom que lui.
		if isClientContainer(tag) {
			for _, child := range tag.Children {
				addSource(byFile, child)
			}
			continue
		}
		addSource(byFile, tag)
	}
	return byFile, nil
}

// isClientContainer distingue le tag conteneur du tag de pair.
func isClientContainer(tag ec.Tag) bool {
	for _, child := range tag.Children {
		if child.Name == ec.TagClient {
			return true
		}
	}
	return false
}

func addSource(byFile map[uint32][]Source, tag ec.Tag) {
	if len(tag.Children) == 0 {
		return
	}
	fileID, ok := dlChildUint(tag, ec.TagClientRequestFile)
	if !ok || fileID == 0 {
		return
	}
	byFile[uint32(fileID)] = append(byFile[uint32(fileID)], sourceFromTag(tag))
}

// sourceFromTag traduit un tag de pair.
//
// Transcrit de `CEC_UpDownClient_Tag` (ECSpecialCoreTags.cpp). Comme pour les
// fichiers, aucun champ n'est obligatoire.
func sourceFromTag(tag ec.Tag) Source {
	s := Source{
		UserHash:       dlChildHashHex(tag, ec.TagClientHash),
		Name:           dlChildText(tag, ec.TagClientName),
		Version:        dlChildText(tag, ec.TagClientSoftVerStr),
		Port:           int(dlChildInt64(tag, ec.TagClientUserPort)),
		QueueRank:      int(dlChildInt64(tag, ec.TagClientRemoteQueueRank)),
		AvailableParts: int(dlChildInt64(tag, ec.TagClientAvailableParts)),
	}

	if soft, ok := dlChildUint(tag, ec.TagClientSoftware); ok {
		s.Software = mapClientSoftware(soft)
	}

	// LowID se lit sur l'identifiant eD2k, pas sur l'adresse : c'est
	// l'identifiant attribué par le serveur qui dit si le pair est joignable
	// (src/updownclient.h, HasLowID).
	if id, ok := dlChildUint(tag, ec.TagClientUserID); ok {
		s.LowID = id < hybridHighID
	}

	// L'adresse d'un pair en LowID n'est pas joignable : la publier laisserait
	// croire qu'on peut l'atteindre. Le type Source promet le vide dans ce cas.
	if !s.LowID {
		if ip, ok := dlChildUint(tag, ec.TagClientUserIP); ok {
			s.IP = dlIPv4(uint32(ip))
		}
	}

	// dsDownloading vaut zéro : sans le témoin de présence, un tag absent
	// passerait pour « en train de télécharger ».
	if state, ok := dlChildUint(tag, ec.TagClientDownloadState); ok {
		s.Downloading = state == dsDownloading
	}

	// Le débit descendant arrive en KIO/s, et en DOUBLE — c'est-à-dire, dans ce
	// protocole, sous forme de texte. amuled ne l'envoie que pour un pair qui
	// transfère réellement.
	if kbps, ok := dlChildFloat(tag, ec.TagClientDownSpeed); ok {
		s.Speed = int64(kbps * 1024)
	}
	return s
}

/*
mapClientSoftware nomme le logiciel du pair.

Transcrit de `GetSoftName` (src/DataToText.cpp) et de l'énumération
EClientSoftware (src/include/protocol/ed2k/ClientSoftware.h). Les valeurs sont
espacées et plusieurs codes désignent le même logiciel : ce sont des versions
successives du champ d'identification, pas des variantes.

Un code inconnu rend la chaîne vide, comme le fait aMule.
*/
func mapClientSoftware(code uint64) string {
	switch code {
	case 0x00, 0x35: // SO_EMULE, SO_OLDEMULE
		return "eMule"
	case 0x01:
		return "cDonkey"
	case 0x02:
		return "(l/x)Mule"
	case 0x03:
		return "aMule"
	case 0x04, 0x28, 0x44: // SO_SHAREAZA et ses deux successeurs
		return "Shareaza"
	case 0x05:
		return "eMule+"
	case 0x06:
		return "HydraNode"
	case 0x0A, 0x98: // SO_NEW2_MLDONKEY, SO_NEW_MLDONKEY
		return "MLDonkey"
	case 0x14:
		return "lphant"
	case 0x32:
		return "eDonkeyHybrid"
	case 0x33:
		return "eDonkey"
	case 0x34:
		return "MLDonkey (ancien)"
	case 0x36:
		return "inconnu"
	case 0xFF:
		return "compatible eMule"
	default:
		return ""
	}
}

// ─── Lecture de tags ─────────────────────────────────────────────────────────
//
// Toutes ces fonctions rendent le zéro sur un enfant absent ou d'un type
// inattendu. C'est le contrat annoncé en tête de fichier, et le lecteur y
// gagne : un décodage ne comporte aucun chemin d'erreur à suivre.

func dlChildUint(tag ec.Tag, name ec.TagName) (uint64, bool) {
	child, ok := tag.Find(name)
	if !ok {
		return 0, false
	}
	return child.Uint()
}

func dlChildInt64(tag ec.Tag, name ec.TagName) int64 {
	v, ok := dlChildUint(tag, name)
	if !ok {
		return 0
	}
	// Le protocole n'a que des entiers non signés. Une valeur qui déborderait
	// l'entier signé est une valeur qu'on a mal lue : la rendre négative
	// tromperait tout le reste de la chaîne.
	if v > 1<<63-1 {
		return 0
	}
	return int64(v)
}

func dlChildText(tag ec.Tag, name ec.TagName) string {
	child, ok := tag.Find(name)
	if !ok {
		return ""
	}
	s, _ := child.Text()
	return s
}

// dlChildHashHex rend une empreinte en hexadécimal MINUSCULE.
//
// La casse n'est pas un détail d'affichage : l'empreinte est notre clé, elle
// sert d'identifiant dans les URL et en base. Deux casses feraient deux clés.
func dlChildHashHex(tag ec.Tag, name ec.TagName) string {
	child, ok := tag.Find(name)
	if !ok {
		return ""
	}
	raw, ok := child.Hash()
	if !ok {
		return ""
	}
	return hex.EncodeToString(raw)
}

/*
dlChildFloat lit un tag de type double.

Le protocole ne transporte pas de flottant binaire : amuled écrit le nombre en
TEXTE et pose le type « double » (`CECTag::CECTag(name, double)`, ECTag.cpp),
en expliquant qu'aucune façon sûre de transmettre un flottant n'a été trouvée.
La conséquence pratique est qu'on ne peut pas lire ce tag comme un entier, ni
comme une chaîne — `Tag.Text` refuse un type qui n'est pas TypeString — et
qu'il faut donc l'analyser ici.

La notation dépend de ce qu'écrit la bibliothèque standard C++ : « 1.5 » comme
« 1.23457e+06 » sont possibles. ParseFloat accepte les deux.
*/
func dlChildFloat(tag ec.Tag, name ec.TagName) (float64, bool) {
	child, ok := tag.Find(name)
	if !ok {
		return 0, false
	}

	switch child.Type {
	case ec.TypeDouble:
		text := strings.TrimRight(string(child.Value), "\x00")
		v, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			return 0, false
		}
		return v, true

	default:
		// Un démon plus ancien pouvait envoyer ce même champ en entier.
		v, ok := child.Uint()
		if !ok {
			return 0, false
		}
		return float64(v), true
	}
}

/*
dlIPv4 met une adresse en forme.

L'ordre des octets surprend : amuled range l'adresse dans un entier « en ordre
anti-hôte » et l'imprime en commençant par l'octet de POIDS FAIBLE
(`Uint32toStringIP`, src/NetworkFunctions.h). Lire l'entier comme un gros-
boutiste rendrait l'adresse à l'envers — 1.0.0.127 au lieu de 127.0.0.1.
*/
func dlIPv4(v uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", v&0xFF, (v>>8)&0xFF, (v>>16)&0xFF, (v>>24)&0xFF)
}
