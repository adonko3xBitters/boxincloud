package ec

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

/*
Packet est le corps d'une trame : une opération et ses champs.

Sur le fil :

	uint8   opcode
	uint16  nombre de tags
	…       tags

Le paquet se comporte comme un tag racine dont on n'écrirait ni le nom, ni le
type, ni la longueur — d'où l'absence de longueur ici : c'est l'en-tête de trame
qui la porte.
*/
type Packet struct {
	Op   Opcode
	Tags []Tag
}

// New construit un paquet.
func New(op Opcode, tags ...Tag) Packet {
	return Packet{Op: op, Tags: tags}
}

// Find cherche un tag de premier niveau.
//
// Rend le premier trouvé : le protocole autorise les répétitions, et c'est le
// premier qui fait foi partout où nous en lisons un seul.
func (p Packet) Find(name TagName) (Tag, bool) {
	for _, tag := range p.Tags {
		if tag.Name == name {
			return tag, true
		}
	}
	return Tag{}, false
}

// Text lit un tag textuel de premier niveau.
func (p Packet) Text(name TagName) (string, bool) {
	tag, ok := p.Find(name)
	if !ok {
		return "", false
	}
	return tag.Text()
}

// Uint lit un tag entier de premier niveau.
func (p Packet) Uint(name TagName) (uint64, bool) {
	tag, ok := p.Find(name)
	if !ok {
		return 0, false
	}
	return tag.Uint()
}

func (p Packet) encode() ([]byte, error) {
	if len(p.Tags) > 0xFFFE {
		return nil, fmt.Errorf("paquet %s : %d tags, maximum 65534 sans négociation",
			p.Op, len(p.Tags))
	}

	var buf bytes.Buffer
	buf.WriteByte(byte(p.Op))

	var count [2]byte
	//nolint:gosec // borné juste au-dessus à 0xFFFE
	binary.BigEndian.PutUint16(count[:], uint16(len(p.Tags)))
	buf.Write(count[:])

	for _, tag := range p.Tags {
		if err := tag.encode(&buf); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

/*
decodePacket lit un corps de trame.

Les octets restants après le dernier tag sont une erreur, pas un détail. Ils
signifient que notre lecture a divergé de ce qu'a écrit le démon — longueur mal
calculée, type mal compris — et le paquet rendu serait alors plausible et faux.
Mieux vaut le dire ici que le laisser remonter en champ vide.
*/
func decodePacket(body []byte) (Packet, error) {
	if len(body) < 3 {
		return Packet{}, fmt.Errorf("corps de %d octets, minimum 3 (opcode et compteur)", len(body))
	}

	r := bytes.NewReader(body)

	op, err := r.ReadByte()
	if err != nil {
		return Packet{}, fmt.Errorf("opcode : %w", err)
	}

	var count [2]byte
	if _, err := r.Read(count[:]); err != nil {
		return Packet{}, fmt.Errorf("nombre de tags : %w", err)
	}

	packet := Packet{Op: Opcode(op)}
	n := int(binary.BigEndian.Uint16(count[:]))
	packet.Tags = make([]Tag, 0, min(n, 64))

	for i := 0; i < n; i++ {
		tag, err := decodeTag(r, 0)
		if err != nil {
			return Packet{}, fmt.Errorf("paquet %s, tag %d : %w", packet.Op, i, err)
		}
		packet.Tags = append(packet.Tags, tag)
	}

	if r.Len() != 0 {
		return Packet{}, fmt.Errorf(
			"paquet %s : %d octets non consommés après %d tags — le décodage a divergé",
			packet.Op, r.Len(), n)
	}

	return packet, nil
}
