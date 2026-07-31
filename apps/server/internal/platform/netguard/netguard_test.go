package netguard

import (
	"errors"
	"testing"
)

// TestCheckRefuse couvre ce que la liste doit arrêter.
//
// Chaque cas correspond à une adresse qu'un service de métadonnées d'instance
// écoute réellement, ou à une notation qui sert surtout à contourner un filtre.
func TestCheckRefuse(t *testing.T) {
	cases := []string{
		"http://169.254.169.254",
		"169.254.169.254:80",
		"http://[fe80::1]:9000",
		"http://[fd00:ec2::254]",
		"http://0.0.0.0:9000",
		"0.1.2.3",
	}

	for _, endpoint := range cases {
		t.Run(endpoint, func(t *testing.T) {
			err := Check(endpoint)
			if err == nil {
				t.Fatalf("Check(%q) = nil, attendu un refus", endpoint)
			}
			var forbidden ErrForbidden
			if !errors.As(err, &forbidden) {
				t.Fatalf("erreur de type %T, attendu ErrForbidden", err)
			}
			if forbidden.Reason == "" {
				t.Error("un refus doit dire pourquoi")
			}
		})
	}
}

// TestCheckAccepte est la moitié qui compte le plus.
//
// Un garde-fou qui refuserait `minio:9000` ou `192.168.1.10` casserait
// l'installation auto-hébergée type, c'est-à-dire tout le public du projet.
func TestCheckAccepte(t *testing.T) {
	cases := []string{
		"",
		"minio:9000",
		"http://minio:9000",
		"https://s3.eu-west-3.amazonaws.com",
		"192.168.1.10:9000",
		"http://127.0.0.1:9000",
		"[::1]:9000",
		"10.0.0.5",
		"https://komga.maison.lan/opds/v2",
	}

	for _, endpoint := range cases {
		t.Run(endpoint, func(t *testing.T) {
			if err := Check(endpoint); err != nil {
				t.Errorf("Check(%q) = %v, attendu nil", endpoint, err)
			}
		})
	}
}

func TestHost(t *testing.T) {
	cases := map[string]string{
		"minio:9000":            "minio",
		"https://s3.exemple.fr": "s3.exemple.fr",
		"http://10.0.0.1:9000":  "10.0.0.1",
		"s3.exemple.fr":         "s3.exemple.fr",
		"[::1]:9000":            "::1",
		"[fd00::1]":             "fd00::1",
		"  minio:9000  ":        "minio",
	}

	for input, want := range cases {
		if got := Host(input); got != want {
			t.Errorf("Host(%q) = %q, attendu %q", input, got, want)
		}
	}
}
