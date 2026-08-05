package ec

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

/*
Le codec, vérifié octet par octet.

Un aller-retour encode/décode ne prouve presque rien : il passe tout aussi bien
si les deux moitiés partagent la même erreur. Les tests qui comptent ici sont
ceux qui comparent la sortie à une séquence d'octets écrite à la main d'après la
spécification — c'est la seule façon d'attraper un boutisme inversé, un décalage
oublié ou une longueur mal comptée avant que le démon ne les rejette en silence.
*/

// TestEncodeOctetParOctet fige la trame d'un paquet minimal.
//
// Chaque octet est justifié en commentaire. Si ce test tombe, ce n'est pas la
// valeur attendue qu'il faut ajuster : c'est l'encodage qui a dérivé.
func TestEncodeOctetParOctet(t *testing.T) {
	packet := New(OpAuthReq, Uint(TagProtocolVersion, uint64(ProtocolVersion)))

	got, err := packet.encode()
	if err != nil {
		t.Fatalf("encode : %v", err)
	}

	want := []byte{
		0x02, // opcode OpAuthReq

		0x00, 0x01, // un tag

		// TagProtocolVersion vaut 0x0002 ; décalé d'un bit vers la gauche, sans
		// enfants, il s'écrit 0x0004. C'est LE piège du format : le bit de
		// poids faible du nom porte la présence d'enfants.
		0x00, 0x04,

		0x03, // TypeUint16 — 0x0204 ne tient pas sur un octet

		0x00, 0x00, 0x00, 0x02, // longueur de la valeur : deux octets

		0x02, 0x04, // 0x0204, gros-boutiste
	}

	if !bytes.Equal(got, want) {
		t.Errorf("encodage =\n  %s\nattendu\n  %s", hex.EncodeToString(got), hex.EncodeToString(want))
	}
}

// TestTagNomEtEnfantsPartagentUnChamp vérifie le décalage dans les deux sens.
func TestTagNomEtEnfantsPartagentUnChamp(t *testing.T) {
	parent := Tag{
		Name:     TagClientName,
		Type:     TypeCustom,
		Children: []Tag{Uint(TagProtocolVersion, 1)},
	}

	var buf bytes.Buffer
	if err := parent.encode(&buf); err != nil {
		t.Fatalf("encode : %v", err)
	}

	raw := buf.Bytes()
	nameField := uint16(raw[0])<<8 | uint16(raw[1])

	if nameField&1 == 0 {
		t.Error("le bit de présence d'enfants n'est pas posé")
	}
	if TagName(nameField>>1) != TagClientName {
		t.Errorf("nom relu = 0x%04X, attendu 0x%04X", nameField>>1, TagClientName)
	}

	decoded, err := decodeTag(bytes.NewReader(raw), 0)
	if err != nil {
		t.Fatalf("decodeTag : %v", err)
	}
	if len(decoded.Children) != 1 {
		t.Fatalf("%d enfants relus, attendu 1", len(decoded.Children))
	}
	if decoded.Children[0].Name != TagProtocolVersion {
		t.Errorf("enfant = 0x%04X, attendu 0x%04X", decoded.Children[0].Name, TagProtocolVersion)
	}
}

/*
TestLongueurAnnonceeCouvreLesEnfants fige l'arithmétique la plus subtile.

La longueur d'un tag couvre sa valeur ET la sérialisation complète de ses
enfants, mais PAS son propre champ de comptage. Une erreur d'un seul octet ici
décale tout ce qui suit dans la trame, et le symptôme est un tag suivant
absurde plutôt qu'une erreur de longueur.
*/
func TestLongueurAnnonceeCouvreLesEnfants(t *testing.T) {
	enfant := Uint(TagProtocolVersion, 1) // en-tête 7 + valeur 1 = 8 octets
	parent := Tag{
		Name:     TagClientName,
		Type:     TypeString,
		Value:    []byte("ab\x00"), // 3 octets
		Children: []Tag{enfant},
	}

	const attendu = 3 + 7 + 1 // valeur + en-tête de l'enfant + valeur de l'enfant
	if got := parent.wireLen(); got != attendu {
		t.Errorf("wireLen = %d, attendu %d", got, attendu)
	}

	// Et l'aller-retour doit retrouver exactement la valeur, ce qui n'arrive
	// que si les deux côtés font la même soustraction.
	var buf bytes.Buffer
	if err := parent.encode(&buf); err != nil {
		t.Fatalf("encode : %v", err)
	}
	decoded, err := decodeTag(bytes.NewReader(buf.Bytes()), 0)
	if err != nil {
		t.Fatalf("decodeTag : %v", err)
	}
	if text, _ := decoded.Text(); text != "ab" {
		t.Errorf("valeur relue = %q, attendu \"ab\"", text)
	}
}

// TestUintChoisitLaPlusPetiteLargeur : amuled fait de même, et un lecteur qui
// supposerait une largeur fixe se tromperait sur la moitié des tags.
func TestUintChoisitLaPlusPetiteLargeur(t *testing.T) {
	cases := []struct {
		valeur uint64
		typ    TagType
		octets int
	}{
		{0, TypeUint8, 1},
		{0xFF, TypeUint8, 1},
		{0x100, TypeUint16, 2},
		{0xFFFF, TypeUint16, 2},
		{0x10000, TypeUint32, 4},
		{0xFFFFFFFF, TypeUint32, 4},
		{0x100000000, TypeUint64, 8},
	}

	for _, c := range cases {
		tag := Uint(TagPasswdSalt, c.valeur)
		if tag.Type != c.typ {
			t.Errorf("Uint(%#x) : type %d, attendu %d", c.valeur, tag.Type, c.typ)
		}
		if len(tag.Value) != c.octets {
			t.Errorf("Uint(%#x) : %d octets, attendu %d", c.valeur, len(tag.Value), c.octets)
		}
		if got, ok := tag.Uint(); !ok || got != c.valeur {
			t.Errorf("Uint(%#x) relu = %#x (ok=%v)", c.valeur, got, ok)
		}
	}
}

// TestTexteTransporteSonOctetNul : amuled compte le terminateur dans la
// longueur, et l'omettre fait perdre un caractère à la lecture d'en face.
func TestTexteTransporteSonOctetNul(t *testing.T) {
	tag := Text(TagClientName, "boxincloud")

	if len(tag.Value) != len("boxincloud")+1 {
		t.Errorf("%d octets, attendu %d", len(tag.Value), len("boxincloud")+1)
	}
	if tag.Value[len(tag.Value)-1] != 0 {
		t.Error("la valeur ne se termine pas par un octet nul")
	}
	if got, _ := tag.Text(); got != "boxincloud" {
		t.Errorf("relu %q", got)
	}
}

func TestAllerRetourPaquet(t *testing.T) {
	original := New(OpAuthReq,
		Text(TagClientName, "boxincloud"),
		Text(TagClientVersion, "0.1.0"),
		Uint(TagProtocolVersion, uint64(ProtocolVersion)),
		Hash(TagPasswdHash, bytes.Repeat([]byte{0xAB}, 16)),
		Empty(TagCanZlib),
	)

	raw, err := original.encode()
	if err != nil {
		t.Fatalf("encode : %v", err)
	}

	decoded, err := decodePacket(raw)
	if err != nil {
		t.Fatalf("decodePacket : %v", err)
	}

	if decoded.Op != original.Op {
		t.Errorf("opcode = %s, attendu %s", decoded.Op, original.Op)
	}
	if len(decoded.Tags) != len(original.Tags) {
		t.Fatalf("%d tags, attendu %d", len(decoded.Tags), len(original.Tags))
	}

	if name, _ := decoded.Text(TagClientName); name != "boxincloud" {
		t.Errorf("nom du client = %q", name)
	}
	if v, _ := decoded.Uint(TagProtocolVersion); v != uint64(ProtocolVersion) {
		t.Errorf("version de protocole = %#x", v)
	}
	hash, ok := func() ([]byte, bool) {
		tag, found := decoded.Find(TagPasswdHash)
		if !found {
			return nil, false
		}
		return tag.Hash()
	}()
	if !ok || !bytes.Equal(hash, bytes.Repeat([]byte{0xAB}, 16)) {
		t.Errorf("empreinte relue = %x (ok=%v)", hash, ok)
	}
}

/*
TestDecodeRefuseCeQuiNeTombePasJuste.

Un décodeur qui accepte une trame incohérente rend un paquet plausible et faux.
C'est le défaut que ce paquet doit rendre impossible : mieux vaut une erreur
nommée qu'un champ vide qu'on ira chercher ailleurs pendant une heure.
*/
func TestDecodeRefuseCeQuiNeTombePasJuste(t *testing.T) {
	cases := []struct {
		nom  string
		body []byte
	}{
		{"corps vide", nil},
		{"opcode seul", []byte{0x02}},
		{
			"un tag annoncé, aucun fourni",
			[]byte{0x02, 0x00, 0x01},
		},
		{
			"valeur plus longue que la trame",
			[]byte{
				0x02, 0x00, 0x01,
				0x00, 0x04, 0x03,
				0xFF, 0xFF, 0xFF, 0xFF, // longueur aberrante
			},
		},
		{
			"octets en trop après le dernier tag",
			[]byte{
				0x02, 0x00, 0x01,
				0x00, 0x04, 0x03, 0x00, 0x00, 0x00, 0x02, 0x02, 0x04,
				0xDE, 0xAD, // personne ne les a demandés
			},
		},
	}

	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			if _, err := decodePacket(c.body); err == nil {
				t.Error("décodage accepté, une erreur était attendue")
			}
		})
	}
}

// TestDecodeNePaniquePasSurDesOctetsAleatoires.
//
// Ces octets viennent du réseau. Une panique dans le décodeur serait un déni de
// service déclenchable par n'importe quel pair joignable.
func TestDecodeNePaniquePasSurDesOctetsAleatoires(t *testing.T) {
	// Séquence pseudo-aléatoire déterministe : un test qui échoue doit pouvoir
	// être rejoué à l'identique.
	seed := byte(0x17)
	for i := 0; i < 2000; i++ {
		body := make([]byte, i%64)
		for j := range body {
			seed = seed*13 + 7
			body[j] = seed
		}
		_, _ = decodePacket(body) // seule l'absence de panique est vérifiée
	}
}

// ─── Cadrage ─────────────────────────────────────────────────────────────────

func TestTrameAllerRetour(t *testing.T) {
	var buf bytes.Buffer
	if err := writeFrame(&buf, flagBase, []byte("bonjour")); err != nil {
		t.Fatalf("writeFrame : %v", err)
	}

	// En-tête de huit octets, puis le corps.
	if buf.Len() != headerSize+7 {
		t.Errorf("trame de %d octets, attendu %d", buf.Len(), headerSize+7)
	}

	flags, body, err := readFrame(&buf, maxBodyPreAuth)
	if err != nil {
		t.Fatalf("readFrame : %v", err)
	}
	if flags != flagBase {
		t.Errorf("drapeaux = 0x%08X, attendu 0x%08X", uint32(flags), uint32(flagBase))
	}
	if string(body) != "bonjour" {
		t.Errorf("corps = %q", body)
	}
}

// TestTrameRefuseUnDrapeauInconnu : un drapeau qu'on ne comprend pas change la
// façon de lire ce qui suit. Poursuivre produirait des champs muets.
func TestTrameRefuseUnDrapeauInconnu(t *testing.T) {
	var buf bytes.Buffer
	if err := writeFrame(&buf, flagBase|0x0400, []byte("x")); err != nil {
		t.Fatalf("writeFrame : %v", err)
	}

	_, _, err := readFrame(&buf, maxBodyPreAuth)
	if err == nil {
		t.Fatal("drapeau inconnu accepté")
	}
	if !strings.Contains(err.Error(), "0x") {
		t.Errorf("le message devrait nommer le drapeau fautif : %v", err)
	}
}

// TestTrameRefuseUneLongueurAberrante : une longueur annoncée par un pair
// inconnu ne doit pas pouvoir faire allouer ce qu'elle veut.
func TestTrameRefuseUneLongueurAberrante(t *testing.T) {
	header := []byte{
		0x00, 0x00, 0x00, 0x20, // flagBase
		0x7F, 0xFF, 0xFF, 0xFF, // presque deux gigaoctets
	}

	if _, _, err := readFrame(bytes.NewReader(header), maxBodyPreAuth); err == nil {
		t.Fatal("longueur aberrante acceptée")
	}
}

// ─── Authentification ────────────────────────────────────────────────────────

/*
TestCondenseSale fige la formule contre un vecteur calculé à part.

La valeur attendue ne vient PAS de ce code : elle a été obtenue à la main, hors
de Go, en enchaînant trois md5. C'est ce qui en fait un test — si la formule
dérive, aucune des deux moitiés ne peut se couvrir l'autre.

	md5("boxincloud")                              = 0b6b819c…
	md5("1234ABCD")                                = 3b8cd688…
	md5("0b6b819c…" + "3b8cd688…")                 = 308c7a39…
*/
func TestCondenseSale(t *testing.T) {
	const (
		password = "boxincloud"
		salt     = uint64(0x1234ABCD)
		attendu  = "308c7a39038d8b700ee7ff25b85db623"
	)

	got := hex.EncodeToString(saltedDigest(password, salt))
	if got != attendu {
		t.Errorf("condensé = %s, attendu %s", got, attendu)
	}
}

// TestCondenseSaleSansZerosDeTete : le sel est rendu en hexadécimal sans zéros
// de tête. Le padder donnerait un condensé différent, et le démon répondrait
// « mot de passe incorrect » pour un mot de passe pourtant juste.
func TestCondenseSaleSansZerosDeTete(t *testing.T) {
	avec := saltedDigest("x", 0x0000000F)
	sans := saltedDigest("x", 0xF)

	if !bytes.Equal(avec, sans) {
		t.Error("deux écritures du même sel donnent deux condensés")
	}
}

/*
TestIPv4EncodeAdresseEtPort.

Les octets sont vérifiés UN PAR UN, contre une valeur écrite à la main.

C'est justifié ici et nulle part ailleurs : ce tag ne se relit pas côté client,
il n'existe que pour être compris du démon. Un test qui l'encoderait puis le
décoderait avec notre propre code passerait avec un ordre d'octets inversé, et
la faute ne se verrait qu'à l'usage — sous la forme d'un démon qui agit sur un
serveur autre que celui demandé.
*/
func TestIPv4EncodeAdresseEtPort(t *testing.T) {
	tag, ok := IPv4(TagServer, "203.0.113.10", 4661)
	if !ok {
		t.Fatal("adresse valide refusée")
	}

	if tag.Type != TypeIPv4 {
		t.Errorf("type = %d, attendu TypeIPv4 (%d)", tag.Type, TypeIPv4)
	}

	// 203.0.113.10 dans l'ordre où on l'écrit, puis 4661 en gros-boutien.
	want := []byte{203, 0, 113, 10, 0x12, 0x35}
	if !bytes.Equal(tag.Value, want) {
		t.Errorf("valeur = % x, attendu % x", tag.Value, want)
	}
}

// TestIPv4RefuseCeQuiNEnEstPas : un nom d'hôte, une IPv6 ou un port hors bornes
// n'ont pas de représentation sur six octets. En fabriquer une remplie de zéros
// désignerait 0.0.0.0 — une adresse que le démon chercherait pour de bon.
func TestIPv4RefuseCeQuiNEnEstPas(t *testing.T) {
	for _, tc := range []struct {
		ip   string
		port int
	}{
		{"", 4661},
		{"serveur.example.org", 4661},
		{"::1", 4661},
		{"203.0.113", 4661},
		{"203.0.113.10", -1},
		{"203.0.113.10", 65536},
	} {
		if _, ok := IPv4(TagServer, tc.ip, tc.port); ok {
			t.Errorf("%q:%d accepté, attendu un refus", tc.ip, tc.port)
		}
	}
}
