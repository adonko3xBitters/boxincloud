package handlers

import (
	"math"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/gen"
)

/*
Traduction des types du domaine vers ceux du contrat.

Deux modèles voisins mais distincts, et il faut le supporter plutôt que de les
fusionner. Le domaine porte des `time.Duration` et des `int` parce que c'est
naturel en Go ; le contrat porte des secondes et des `int32` parce que c'est ce
qu'un client JSON sait lire sans ambiguïté. Les faire coïncider de force
obligerait l'un des deux à se tordre.

Ce fichier est mécanique et le restera. C'est le prix d'une frontière, et il se
paie une fois.
*/

// mapSlice applique une conversion à toute une tranche.
//
// Rend TOUJOURS une tranche non nulle : `nil` se sérialise en `null`, et une
// interface qui reçoit `null` là où elle attend une liste doit ajouter un test
// à chaque endroit. Une liste vide se parcourt sans rien dire.
func mapSlice[In, Out any](in []In, convert func(In) Out) []Out {
	out := make([]Out, 0, len(in))
	for _, item := range in {
		out = append(out, convert(item))
	}
	return out
}

// nowUTC date une réponse qui ne vient pas d'un instantané.
func nowUTC() time.Time { return time.Now().UTC() }

/*
asInt32 borne une valeur du domaine pour le contrat.

Les compteurs du domaine sont des `int` parce qu'ils viennent d'un démon qui
peut annoncer ce qu'il veut ; le contrat les déclare en `int32` parce qu'aucun
client n'a besoin de plus pour un nombre d'utilisateurs ou de fichiers.

Écrête plutôt que de retourner : un nombre de sources négatif à l'écran serait
plus déroutant qu'un nombre plafonné.
*/
func asInt32(v int) int32 {
	switch {
	case v > math.MaxInt32:
		return math.MaxInt32
	case v < math.MinInt32:
		return math.MinInt32
	default:
		return int32(v)
	}
}

func apiDownload(d amule.Download) gen.Ed2kDownload {
	out := gen.Ed2kDownload{
		Hash:           d.Hash,
		Name:           d.Name,
		Size:           d.Size,
		SizeDone:       d.SizeDone,
		SizeXfer:       d.SizeXfer,
		Speed:          d.Speed,
		Status:         gen.Ed2kDownloadStatus(d.Status),
		Priority:       gen.Ed2kPriority(d.Priority),
		Category:       asInt32(d.Category),
		PartCount:      asInt32(d.PartCount),
		AvailableParts: asInt32(d.AvailableParts),
		Sources: gen.Ed2kSourceCounts{
			Total:        asInt32(d.Sources.Total),
			NotCurrent:   asInt32(d.Sources.NotCurrent),
			Transferring: asInt32(d.Sources.Transferring),
			A4af:         asInt32(d.Sources.A4AF),
		},
		LastSeenComplete: d.LastSeenComplete,
	}

	// L'ETA voyage en SECONDES, pas en durée Go.
	//
	// Le contrat est lu par du TypeScript et du Dart, qui n'ont pas de type
	// durée. Envoyer « 1h23m » obligerait chaque client à écrire son analyseur,
	// et l'absence reste nil — une ETA infinie ne se représente pas.
	if d.ETA != nil {
		seconds := int64(d.ETA.Seconds())
		out.EtaSeconds = &seconds
	}
	return out
}

func apiSource(s amule.Source) gen.Ed2kSource {
	return gen.Ed2kSource{
		UserHash:       s.UserHash,
		Name:           s.Name,
		Software:       s.Software,
		Version:        s.Version,
		Ip:             s.IP,
		Port:           asInt32(s.Port),
		LowId:          s.LowID,
		Speed:          s.Speed,
		QueueRank:      asInt32(s.QueueRank),
		AvailableParts: asInt32(s.AvailableParts),
		Downloading:    s.Downloading,
	}
}

func apiUpload(u amule.Upload) gen.Ed2kUpload {
	return gen.Ed2kUpload{
		UserHash:        u.UserHash,
		Name:            u.Name,
		Software:        u.Software,
		Version:         u.Version,
		Ip:              u.IP,
		Port:            asInt32(u.Port),
		FileHash:        u.FileHash,
		FileName:        u.FileName,
		Speed:           u.Speed,
		Transferred:     u.Transferred,
		TotalUploaded:   u.TotalUploaded,
		SessionUploaded: u.SessionUploaded,
	}
}

func apiQueuedPeer(q amule.QueuedPeer) gen.Ed2kQueuedPeer {
	return gen.Ed2kQueuedPeer{
		UserHash:    q.UserHash,
		Name:        q.Name,
		Ip:          q.IP,
		Port:        asInt32(q.Port),
		FileHash:    q.FileHash,
		Score:       asInt32(q.Score),
		WaitedSince: q.WaitedSince,
	}
}

func apiSharedFile(f amule.SharedFile) gen.Ed2kSharedFile {
	return gen.Ed2kSharedFile{
		Hash:        f.Hash,
		Name:        f.Name,
		Size:        f.Size,
		Path:        f.Path,
		Priority:    gen.Ed2kPriority(f.Priority),
		Requests:    f.Requests,
		Accepted:    f.Accepted,
		Transferred: f.Transferred,
		Complete:    f.Complete,
	}
}

func apiServer(s amule.Server) gen.Ed2kServer {
	return gen.Ed2kServer{
		Ip:          s.IP,
		Port:        asInt32(s.Port),
		Name:        s.Name,
		Description: s.Description,
		Version:     s.Version,
		Ping:        asInt32(s.Ping),
		Users:       asInt32(s.Users),
		MaxUsers:    asInt32(s.MaxUsers),
		Files:       asInt32(s.Files),
		Failed:      asInt32(s.Failed),
		Static:      s.Static,
		Priority:    gen.Ed2kPriority(s.Priority),
		Connected:   s.Connected,
	}
}

func apiConnection(c amule.Connection) gen.Ed2kConnection {
	out := gen.Ed2kConnection{
		Ed2k: gen.Ed2kNetworkState{
			Connected:  c.Ed2k.Connected,
			Connecting: c.Ed2k.Connecting,
			ClientId:   int64(c.Ed2k.ClientID),
			Id:         gen.Ed2kIdType(c.Ed2k.ID),
		},
		Kad: gen.Ed2kKadState{
			Running:       c.Kad.Running,
			Connected:     c.Kad.Connected,
			Firewalled:    c.Kad.Firewalled,
			FirewalledUdp: c.Kad.FirewalledUDP,
		},
	}

	if c.Ed2k.Server != nil {
		server := apiServer(*c.Ed2k.Server)
		out.Ed2k.Server = &server
	}
	return out
}

func apiStats(s amule.Stats) gen.Ed2kStats {
	return gen.Ed2kStats{
		UpSpeed:           s.UpSpeed,
		DownSpeed:         s.DownSpeed,
		UpLimit:           s.UpLimit,
		DownLimit:         s.DownLimit,
		UpOverhead:        s.UpOverhead,
		DownOverhead:      s.DownOverhead,
		TotalSources:      asInt32(s.TotalSources),
		BannedPeers:       asInt32(s.BannedPeers),
		UploadQueueLength: asInt32(s.UploadQueueLength),
		Ed2kUsers:         asInt32(s.Ed2kUsers),
		KadUsers:          asInt32(s.KadUsers),
		Ed2kFiles:         asInt32(s.Ed2kFiles),
		KadFiles:          asInt32(s.KadFiles),
	}
}
