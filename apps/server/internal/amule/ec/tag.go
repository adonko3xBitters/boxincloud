package ec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/netip"
)

/*
Tag est un champ de paquet. C'est l'unique porteur de donnée du protocole.

Sur le fil :

	uint16  (nom << 1) | présence d'enfants
	uint8   type
	uint32  longueur
	uint16  nombre d'enfants        — seulement si le bit de présence est posé
	…       enfants                 — idem
	octets  valeur

Deux pièges, tous deux invisibles tant qu'on ne teste que contre soi-même :

  - Le bit de poids faible du NOM porte la présence d'enfants. Lire le nom sans
    le décaler donne des noms doublés, donc des tags introuvables.

  - La LONGUEUR annoncée couvre la valeur ET la sérialisation complète des
    enfants, mais PAS le champ « nombre d'enfants » du tag lui-même. La valeur
    se déduit par soustraction, ce qui rend le calcul de longueur d'un tag
    parent solidaire de celui de ses enfants.
*/
type Tag struct {
	Name  TagName
	Type  TagType
	Value []byte

	Children []Tag
}

// tagHeaderSize est la taille du triplet nom/type/longueur.
const tagHeaderSize = 2 + 1 + 4

// ─── Construction ────────────────────────────────────────────────────────────

/*
Uint construit un tag entier.

Le type est le PLUS PETIT qui contienne la valeur, et ce n'est pas une
optimisation : c'est ce que fait amuled, et un lecteur qui supposerait une
largeur fixe se tromperait sur la moitié des tags. Un même nom de tag peut donc
arriver en 8, 16, 32 ou 64 bits selon la valeur du moment — le sel
d'authentification en est l'exemple le plus visible.
*/
func Uint(name TagName, v uint64) Tag {
	switch {
	case v <= 0xFF:
		return Tag{Name: name, Type: TypeUint8, Value: []byte{byte(v)}}
	case v <= 0xFFFF:
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(v))
		return Tag{Name: name, Type: TypeUint16, Value: b}
	case v <= 0xFFFFFFFF:
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, uint32(v))
		return Tag{Name: name, Type: TypeUint32, Value: b}
	default:
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, v)
		return Tag{Name: name, Type: TypeUint64, Value: b}
	}
}

// Text construit un tag chaîne.
//
// La valeur porte son octet nul final : amuled compte le terminateur dans la
// longueur annoncée. L'omettre fait perdre un caractère à la lecture d'en face.
func Text(name TagName, s string) Tag {
	return Tag{Name: name, Type: TypeString, Value: append([]byte(s), 0)}
}

// Hash construit un tag d'empreinte sur seize octets.
func Hash(name TagName, h []byte) Tag {
	value := make([]byte, 16)
	copy(value, h)
	return Tag{Name: name, Type: TypeHash16, Value: value}
}

/*
IPv4 construit un tag d'adresse-et-port, en six octets.

C'est ainsi que le démon DÉSIGNE un serveur dans une commande. Une chaîne
« ip:port » ne convient pas, et c'est un piège coûteux : `EC_TAG_SERVER` en
texte est accepté sans broncher, puis relu comme six octets bruts. Le démon ne
trouve alors aucun serveur à cette adresse, et se rabat silencieusement sur son
comportement par défaut — pour `SERVER_CONNECT`, se connecter à n'importe lequel.

Format : les quatre octets de la notation pointée dans l'ordre où on les écrit,
puis le port en gros-boutien. C'est exactement ce que `serverAddressFromIPv4`
sait relire, à l'autre bout.

Une adresse qui n'est pas une IPv4 littérale rend `false` : le tag ne peut pas
porter un nom d'hôte, et en fabriquer un vide ferait agir le démon sur autre
chose que ce qui était demandé.
*/
func IPv4(name TagName, ip string, port int) (Tag, bool) {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is4() || port < 0 || port > 65535 {
		return Tag{}, false
	}

	octets := addr.As4()
	value := make([]byte, 0, 6)
	value = append(value, octets[:]...)
	value = binary.BigEndian.AppendUint16(value, uint16(port))

	return Tag{Name: name, Type: TypeIPv4, Value: value}, true
}

/*
Empty construit un tag sans valeur.

Le protocole s'en sert comme d'un drapeau : le tag présent vaut oui, le tag
absent vaut non. C'est ainsi que les capacités du client s'annoncent à
l'authentification.

Le type est `TypeCustom` et non `TypeUnknown` : amuled refuse d'écrire un tag de
type inconnu, et un tag qu'on ne peut pas relire ne sert à rien.
*/
func Empty(name TagName) Tag {
	return Tag{Name: name, Type: TypeCustom}
}

// ─── Lecture ─────────────────────────────────────────────────────────────────

// Uint rend la valeur entière du tag, quelle que soit sa largeur.
func (t Tag) Uint() (uint64, bool) {
	switch t.Type {
	case TypeUint8:
		if len(t.Value) < 1 {
			return 0, false
		}
		return uint64(t.Value[0]), true
	case TypeUint16:
		if len(t.Value) < 2 {
			return 0, false
		}
		return uint64(binary.BigEndian.Uint16(t.Value)), true
	case TypeUint32:
		if len(t.Value) < 4 {
			return 0, false
		}
		return uint64(binary.BigEndian.Uint32(t.Value)), true
	case TypeUint64:
		if len(t.Value) < 8 {
			return 0, false
		}
		return binary.BigEndian.Uint64(t.Value), true
	default:
		return 0, false
	}
}

// Text rend la valeur textuelle, sans son octet nul final.
func (t Tag) Text() (string, bool) {
	if t.Type != TypeString {
		return "", false
	}
	return string(bytes.TrimRight(t.Value, "\x00")), true
}

// Hash rend une empreinte de seize octets.
func (t Tag) Hash() ([]byte, bool) {
	if t.Type != TypeHash16 || len(t.Value) != 16 {
		return nil, false
	}
	return t.Value, true
}

// Find cherche un enfant par son nom, en profondeur d'un seul niveau.
func (t Tag) Find(name TagName) (Tag, bool) {
	for _, child := range t.Children {
		if child.Name == name {
			return child, true
		}
	}
	return Tag{}, false
}

// ─── Sérialisation ───────────────────────────────────────────────────────────

/*
wireLen calcule la longueur à annoncer pour ce tag.

Valeur propre, plus la sérialisation complète de chaque enfant : son en-tête,
son propre champ de comptage s'il a lui-même des enfants, et sa longueur. Le
champ de comptage du tag COURANT n'y est pas — c'est le parent qui l'écrit, et
le lecteur d'en face fait la même soustraction.
*/
func (t Tag) wireLen() uint32 {
	// La valeur ne peut pas dépasser quatre gigaoctets : à la lecture, le
	// cadrage plafonne déjà le corps entier, et à l'écriture nous ne
	// construisons que ce que nous avons nous-mêmes assemblé.
	length := uint32(len(t.Value)) //nolint:gosec // borné par le cadrage
	for _, child := range t.Children {
		length += tagHeaderSize + child.wireLen()
		if len(child.Children) > 0 {
			length += 2
		}
	}
	return length
}

func (t Tag) encode(buf *bytes.Buffer) error {
	if len(t.Children) > 0xFFFE {
		// Le comptage étendu existe, mais il se négocie. Tant qu'il ne l'est
		// pas, écrire plus de 0xFFFE enfants produirait une trame qu'aucun des
		// deux côtés ne relit pareil.
		return fmt.Errorf("tag %s : %d enfants, maximum 65534 sans négociation",
			t.Name, len(t.Children))
	}

	name := uint16(t.Name) << 1
	if len(t.Children) > 0 {
		name |= 1
	}

	var header [tagHeaderSize]byte
	binary.BigEndian.PutUint16(header[0:2], name)
	header[2] = byte(t.Type)
	binary.BigEndian.PutUint32(header[3:7], t.wireLen())
	buf.Write(header[:])

	if len(t.Children) > 0 {
		var count [2]byte
		//nolint:gosec // borné en tête de fonction à 0xFFFE
		binary.BigEndian.PutUint16(count[:], uint16(len(t.Children)))
		buf.Write(count[:])

		for _, child := range t.Children {
			if err := child.encode(buf); err != nil {
				return err
			}
		}
	}

	buf.Write(t.Value)
	return nil
}

/*
decodeTag lit un tag et ses descendants.

`depth` borne la récursion. Le protocole n'impose aucune limite, et un pair
malveillant — ou simplement un décodage qui a déraillé — peut donc décrire un
arbre arbitrairement profond depuis quelques octets.
*/
func decodeTag(r *bytes.Reader, depth int) (Tag, error) {
	const maxDepth = 16
	if depth > maxDepth {
		return Tag{}, fmt.Errorf("tags imbriqués sur plus de %d niveaux", maxDepth)
	}

	var header [tagHeaderSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return Tag{}, fmt.Errorf("en-tête de tag : %w", err)
	}

	raw := binary.BigEndian.Uint16(header[0:2])
	tag := Tag{
		Name: TagName(raw >> 1),
		Type: TagType(header[2]),
	}
	hasChildren := raw&1 != 0
	declared := binary.BigEndian.Uint32(header[3:7])

	var childrenLen uint32
	if hasChildren {
		var count [2]byte
		if _, err := io.ReadFull(r, count[:]); err != nil {
			return Tag{}, fmt.Errorf("tag %s : nombre d'enfants : %w", tag.Name, err)
		}

		n := int(binary.BigEndian.Uint16(count[:]))
		tag.Children = make([]Tag, 0, min(n, 64))

		for i := 0; i < n; i++ {
			child, err := decodeTag(r, depth+1)
			if err != nil {
				return Tag{}, fmt.Errorf("tag %s, enfant %d : %w", tag.Name, i, err)
			}
			tag.Children = append(tag.Children, child)

			childrenLen += tagHeaderSize + child.wireLen()
			if len(child.Children) > 0 {
				childrenLen += 2
			}
		}
	}

	// Sans ce contrôle, la soustraction repasserait sous zéro et demanderait
	// une allocation de près de quatre gigaoctets à partir d'un tag malformé.
	if declared < childrenLen {
		return Tag{}, fmt.Errorf(
			"tag %s : longueur annoncée %d inférieure à celle de ses enfants (%d)",
			tag.Name, declared, childrenLen)
	}

	valueLen := declared - childrenLen
	if r.Len() < int(valueLen) {
		return Tag{}, fmt.Errorf(
			"tag %s : valeur de %d octets annoncée, %d restants dans la trame",
			tag.Name, valueLen, r.Len())
	}

	if valueLen > 0 {
		tag.Value = make([]byte, valueLen)
		if _, err := io.ReadFull(r, tag.Value); err != nil {
			return Tag{}, fmt.Errorf("tag %s : valeur : %w", tag.Name, err)
		}
	}

	return tag, nil
}
