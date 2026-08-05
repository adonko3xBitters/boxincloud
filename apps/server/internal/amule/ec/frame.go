package ec

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

/*
Cadrage des trames External Connections.

Une trame vaut huit octets d'en-tête puis un corps :

	uint32  drapeaux     (gros-boutiste)
	uint32  longueur     (gros-boutiste, corps seul, en-tête non compté)
	octets  corps        (éventuellement compressé)

Tout ce protocole est gros-boutiste, y compris les entiers dans les tags. C'est
la seule chose qui ne varie jamais.
*/

const headerSize = 8

/*
flagBase est présent dans toute trame, dans les deux sens.

Il ne figure dans aucune énumération : amuled l'écrit en dur au début de chaque
trame. On le reproduit tel quel. Ne pas le poser fait rejeter la trame sans
message utile — le genre de détail qui coûte une soirée si on l'apprend en
tâtonnant plutôt qu'en lisant.
*/
const flagBase Flag = 0x20

/*
maxBodySize borne ce qu'on accepte de lire.

amuled applique 16 Mo avant authentification et 256 Mo après. On garde 16 Mo
tant qu'on n'est pas authentifié, pour la même raison que lui : une longueur
annoncée par un pair inconnu ne doit pas pouvoir faire allouer autant qu'elle
veut.
*/
const (
	maxBodyPreAuth  = 16 << 20
	maxBodyPostAuth = 256 << 20
)

// writeFrame émet une trame complète.
//
// Le corps n'est jamais compressé ici : la compression se négocie à
// l'authentification, et nous ne la demandons pas encore. Voir le commentaire
// de Authenticate.
func writeFrame(w io.Writer, flags Flag, body []byte) error {
	// La longueur voyage sur quatre octets : au-delà, elle serait tronquée et
	// le pair lirait une trame décalée plutôt que de constater une erreur.
	if uint64(len(body)) > math.MaxUint32 {
		return fmt.Errorf("corps de %d octets, maximum %d", len(body), uint64(math.MaxUint32))
	}

	var header [headerSize]byte
	binary.BigEndian.PutUint32(header[0:4], uint32(flags))
	//nolint:gosec // la borne est juste au-dessus
	binary.BigEndian.PutUint32(header[4:8], uint32(len(body)))

	if _, err := w.Write(header[:]); err != nil {
		return fmt.Errorf("écriture de l'en-tête de trame : %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("écriture du corps de trame : %w", err)
	}
	return nil
}

/*
readFrame lit une trame et rend son corps décompressé.

Les drapeaux inconnus sont refusés plutôt qu'ignorés. Un drapeau qu'on ne
comprend pas change la façon de lire ce qui suit — l'encodage des entiers en
dépend, par exemple — et poursuivre produirait des champs muets au lieu d'une
erreur. C'est exactement le défaut que ce paquet doit rendre impossible.
*/
func readFrame(r io.Reader, maxBody uint32) (Flag, []byte, error) {
	var header [headerSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, fmt.Errorf("lecture de l'en-tête de trame : %w", err)
	}

	flags := Flag(binary.BigEndian.Uint32(header[0:4]))
	length := binary.BigEndian.Uint32(header[4:8])

	if length > maxBody {
		return 0, nil, fmt.Errorf(
			"trame de %d octets annoncée, plafond %d — le pair n'est probablement pas un démon aMule",
			length, maxBody)
	}

	if unknown := flags &^ (flagBase | FlagZlib | FlagUTF8Numbers | FlagLargeTagCount); unknown != 0 {
		return 0, nil, fmt.Errorf("drapeaux de trame inconnus : 0x%08X", uint32(unknown))
	}

	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, fmt.Errorf("lecture du corps de trame (%d octets) : %w", length, err)
	}

	if flags&FlagZlib != 0 {
		inflated, err := inflate(body)
		if err != nil {
			return 0, nil, err
		}
		body = inflated
	}

	return flags, body, nil
}

// inflate décompresse un corps zlib.
//
// La taille décompressée n'est pas annoncée : on borne donc la lecture, sinon
// une petite trame malveillante pourrait demander une allocation sans rapport
// avec ce qui a transité.
func inflate(body []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("corps zlib illisible : %w", err)
	}
	defer func() { _ = zr.Close() }()

	out, err := io.ReadAll(io.LimitReader(zr, maxBodyPostAuth))
	if err != nil {
		return nil, fmt.Errorf("décompression du corps : %w", err)
	}
	return out, nil
}
