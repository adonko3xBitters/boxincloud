package ingest

import (
	"strings"
	"testing"
)

// Les noms de fichiers viennent du navigateur de quelqu'un. Ils traversent
// ensuite la composition d'une clé d'objet, où un séparateur mal filtré fait
// écrire ailleurs que là où l'utilisateur croyait.
func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		want  string
		fails bool
	}{
		{name: "nom simple", in: "Tintin.cbz", want: "Tintin.cbz"},
		{name: "espaces autour", in: "  Astérix.cbz  ", want: "Astérix.cbz"},
		{name: "chemin POSIX", in: "/etc/passwd/Tintin.cbz", want: "Tintin.cbz"},
		{name: "chemin Windows", in: `C:\Users\x\Tintin.cbz`, want: "Tintin.cbz"},
		{name: "remontée", in: "../../../etc/shadow.cbz", want: "shadow.cbz"},
		{name: "caractère de contrôle", in: "Tin\x00tin\x1f.cbz", want: "Tintin.cbz"},
		{name: "accents conservés", in: "Le Chat du Rabbin — T01.cbz", want: "Le Chat du Rabbin — T01.cbz"},

		{name: "vide", in: "   ", fails: true},
		{name: "points seuls", in: "..", fails: true},
		{name: "séparateurs seuls", in: "///", fails: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sanitizeFilename(tc.in)
			if tc.fails {
				if err == nil {
					t.Fatalf("sanitizeFilename(%q) = %q, une erreur était attendue", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("sanitizeFilename(%q) : %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("sanitizeFilename(%q) = %q, attendu %q", tc.in, got, tc.want)
			}
		})
	}
}

// Un dossier de destination doit rester DANS la bibliothèque, quelle que soit
// la façon dont il est écrit.
func TestSanitizeFolder(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"Tintin", "Tintin"},
		{"/Tintin/", "Tintin"},
		{"Tintin//Albums", "Tintin/Albums"},
		{`Tintin\Albums`, "Tintin/Albums"},
		{"../../../etc", "etc"},
		{"./Tintin/./Albums/..", "Tintin/Albums"},
		{"...", "..."},
	}

	for _, tc := range cases {
		if got := sanitizeFolder(tc.in); got != tc.want {
			t.Errorf("sanitizeFolder(%q) = %q, attendu %q", tc.in, got, tc.want)
		}
	}
}

// La clé finale ne doit jamais sortir du préfixe de la bibliothèque : c'est la
// seule garantie qu'un envoi n'écrit pas dans une AUTRE bibliothèque du même
// bucket.
func TestObjectKeyStaysInsidePrefix(t *testing.T) {
	cases := []struct {
		prefix, folder, name, want string
	}{
		{"bd/", "", "Tintin.cbz", "bd/Tintin.cbz"},
		{"bd", "Tintin", "T11.cbz", "bd/Tintin/T11.cbz"},
		{"/bd/", "/Tintin/", "T11.cbz", "bd/Tintin/T11.cbz"},
		{"", "", "Tintin.cbz", "Tintin.cbz"},
		{"bd/", "../autre", "T11.cbz", "bd/autre/T11.cbz"},
		{"bd/", "../../..", "T11.cbz", "bd/T11.cbz"},
	}

	for _, tc := range cases {
		got := objectKey(tc.prefix, tc.folder, tc.name)
		if got != tc.want {
			t.Errorf("objectKey(%q, %q, %q) = %q, attendu %q",
				tc.prefix, tc.folder, tc.name, got, tc.want)
		}

		if prefix := strings.Trim(tc.prefix, "/"); prefix != "" {
			if !strings.HasPrefix(got, prefix+"/") {
				t.Errorf("la clé %q est sortie du préfixe %q", got, prefix)
			}
		}
	}
}

func TestDetectFormat(t *testing.T) {
	cases := []struct {
		in    string
		want  Format
		found bool
	}{
		{"Tintin.cbz", FormatCBZ, true},
		{"Tintin.CBZ", FormatCBZ, true},
		{"Tintin.zip", FormatCBZ, true},
		{"Tintin.cbr", FormatCBR, true},
		{"Tintin.rar", FormatCBR, true},
		{"Tintin.pdf", FormatPDF, true},
		{"Tintin.txt", "", false},
		{"Tintin", "", false},
		{"Tintin.cbz.exe", "", false},
	}

	for _, tc := range cases {
		got, found := DetectFormat(tc.in)
		if found != tc.found || (found && got != tc.want) {
			t.Errorf("DetectFormat(%q) = (%q, %v), attendu (%q, %v)",
				tc.in, got, found, tc.want, tc.found)
		}
	}
}

/*
Le contenu prime sur l'extension.

Renommer un exécutable en `.cbz` ne doit pas suffire à le déposer dans le
bucket : il y resterait, servi ensuite à tous les clients de la bibliothèque.
*/
func TestMatchesFormat(t *testing.T) {
	cases := []struct {
		name   string
		head   []byte
		format Format
		want   bool
	}{
		{"zip standard", []byte{'P', 'K', 3, 4, 0, 0, 0, 0}, FormatCBZ, true},
		{"zip vide", []byte{'P', 'K', 5, 6, 0, 0, 0, 0}, FormatCBZ, true},
		{"rar 4", []byte{'R', 'a', 'r', '!', 0x1a, 0x07, 0x00, 0}, FormatCBR, true},
		{"rar 5", []byte{'R', 'a', 'r', '!', 0x1a, 0x07, 0x01, 0x00}, FormatCBR, true},
		{"pdf", []byte("%PDF-1.7"), FormatPDF, true},

		{"exécutable Mach-O renommé en cbz", []byte{0xcf, 0xfa, 0xed, 0xfe, 0, 0, 0, 0}, FormatCBZ, false},
		{"ELF renommé en cbz", []byte{0x7f, 'E', 'L', 'F', 0, 0, 0, 0}, FormatCBZ, false},
		{"zip présenté comme pdf", []byte{'P', 'K', 3, 4, 0, 0, 0, 0}, FormatPDF, false},
		{"pdf présenté comme zip", []byte("%PDF-1.7"), FormatCBZ, false},
		{"fichier vide", []byte{}, FormatCBZ, false},
		{"tronqué", []byte{'P'}, FormatCBZ, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchesFormat(tc.head, tc.format); got != tc.want {
				t.Errorf("matchesFormat(%v, %q) = %v, attendu %v", tc.head, tc.format, got, tc.want)
			}
		})
	}
}

/*
La lecture bornée doit ÉCHOUER au dépassement, pas s'arrêter.

io.LimitReader rend io.EOF à la limite, ce qui ferait passer un fichier tronqué
pour un fichier complet : l'objet serait écrit, à moitié, sans que rien ne le
signale.
*/
func TestLimitedReaderFailsInsteadOfTruncating(t *testing.T) {
	source := strings.NewReader(strings.Repeat("x", 100))
	reader := &limitedReader{r: source, remaining: 10}

	buf := make([]byte, 64)
	read := 0

	for {
		n, err := reader.Read(buf)
		read += n
		if err != nil {
			if err != ErrTooLarge {
				t.Fatalf("erreur = %v, attendu ErrTooLarge", err)
			}
			break
		}
		if read > 100 {
			t.Fatal("lecture sans fin")
		}
	}

	if read != 10 {
		t.Errorf("octets lus = %d, attendu 10", read)
	}
}

func TestLimitedReaderPassesUnderLimit(t *testing.T) {
	source := strings.NewReader("court")
	reader := &limitedReader{r: source, remaining: 1000}

	buf := make([]byte, 64)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("lecture : %v", err)
	}
	if string(buf[:n]) != "court" {
		t.Errorf("lu %q, attendu %q", buf[:n], "court")
	}
}
