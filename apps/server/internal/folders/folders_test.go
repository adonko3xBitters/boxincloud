package folders

import (
	"reflect"
	"testing"
)

// Les chemins viennent de clients. Un « ../ » mal filtré désignerait un
// emplacement hors de la bibliothèque, et le renommage y déplacerait des
// fichiers.
func TestNormalizePath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"Tintin", "Tintin"},
		{"/Tintin/", "Tintin"},
		{"Tintin//Albums", "Tintin/Albums"},
		{`Tintin\Albums`, "Tintin/Albums"},
		{"  Tintin  ", "Tintin"},
		{"../../etc", "etc"},
		{"./Tintin/./Albums/..", "Tintin/Albums"},
		{"...", ""},
		{"///", ""},
		{"BD/Franco-belge/Tintin", "BD/Franco-belge/Tintin"},
	}

	for _, tc := range cases {
		if got := NormalizePath(tc.in); got != tc.want {
			t.Errorf("NormalizePath(%q) = %q, attendu %q", tc.in, got, tc.want)
		}
	}
}

// La chaîne des ancêtres commande la création des dossiers intermédiaires et le
// cumul des compteurs. La racine en fait partie : elle porte les albums non
// rangés.
func TestAncestorsOf(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"Tintin", []string{""}},
		{"BD/Tintin", []string{"", "BD"}},
		{"BD/Franco-belge/Tintin", []string{"", "BD", "BD/Franco-belge"}},
	}

	for _, tc := range cases {
		if got := ancestorsOf(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ancestorsOf(%q) = %v, attendu %v", tc.in, got, tc.want)
		}
	}
}

func TestPathHelpers(t *testing.T) {
	cases := []struct {
		path   string
		parent string
		last   string
		depth  int
	}{
		{"", "", "", 0},
		{"Tintin", "", "Tintin", 1},
		{"BD/Tintin", "BD", "Tintin", 2},
		{"BD/Franco-belge/Tintin", "BD/Franco-belge", "Tintin", 3},
	}

	for _, tc := range cases {
		if got := parentOf(tc.path); got != tc.parent {
			t.Errorf("parentOf(%q) = %q, attendu %q", tc.path, got, tc.parent)
		}
		if got := lastSegment(tc.path); got != tc.last {
			t.Errorf("lastSegment(%q) = %q, attendu %q", tc.path, got, tc.last)
		}
		if got := depthOf(tc.path); got != tc.depth {
			t.Errorf("depthOf(%q) = %d, attendu %d", tc.path, got, tc.depth)
		}
	}
}

// La composition de clés est ce qui garantit qu'un déplacement reste DANS la
// bibliothèque.
func TestJoinKeyAndFolderOfKey(t *testing.T) {
	cases := []struct {
		prefix, path, key string
	}{
		{"bd", "Tintin", "bd/Tintin"},
		{"", "Tintin", "Tintin"},
		{"bd", "", "bd"},
		{"", "", ""},
	}

	for _, tc := range cases {
		if got := joinKey(tc.prefix, tc.path); got != tc.key {
			t.Errorf("joinKey(%q, %q) = %q, attendu %q", tc.prefix, tc.path, got, tc.key)
		}
	}

	folders := []struct {
		key, prefix, want string
	}{
		{"bd/Tintin/T11.cbz", "bd", "Tintin"},
		{"bd/T11.cbz", "bd", ""},
		{"bd/BD/Franco/T11.cbz", "bd", "BD/Franco"},
		{"T11.cbz", "", ""},
	}

	for _, tc := range folders {
		if got := folderOfKey(tc.key, tc.prefix); got != tc.want {
			t.Errorf("folderOfKey(%q, %q) = %q, attendu %q", tc.key, tc.prefix, got, tc.want)
		}
	}
}
