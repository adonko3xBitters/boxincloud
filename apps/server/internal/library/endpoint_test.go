package library

import (
	"errors"
	"testing"
)

/*
Adresses de backend refusées.

La liste des refus doit rester courte : chaque entrée de trop est une adresse
que quelqu'un, quelque part, utilise légitimement. Ces tests fixent donc les
deux bords — ce qui est refusé, et surtout ce qui ne doit jamais l'être.
*/
func TestCheckEndpointRefuse(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
	}{
		{"métadonnées AWS, GCP, Azure", "169.254.169.254"},
		{"métadonnées avec port", "169.254.169.254:80"},
		{"métadonnées avec schéma", "http://169.254.169.254/latest/meta-data/"},
		{"lien-local quelconque", "169.254.1.1:9000"},
		{"lien-local IPv6", "[fe80::1]:9000"},
		{"métadonnées AWS en IPv6", "[fd00:ec2::254]:80"},
		{"cet hôte", "0.0.0.0:9000"},
		{"réservé 0.0.0.0/8", "0.1.2.3"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := CheckEndpoint(c.endpoint)
			if err == nil {
				t.Fatalf("CheckEndpoint(%q) = nil, attendu un refus", c.endpoint)
			}
			// Le refus doit se présenter comme une configuration invalide, pour
			// que l'API réponde 422 et non 500.
			if !errors.Is(err, ErrInvalidConfig) {
				t.Errorf("erreur %v, attendu ErrInvalidConfig", err)
			}
		})
	}
}

/*
Ce qui doit passer.

Le plus important des deux tests. Refuser les adresses privées serait la parade
habituelle contre la SSRF ; elle casserait ici le cas d'usage principal, où le
stockage est un MinIO sur la même machine ou le même réseau.
*/
func TestCheckEndpointAccepte(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
	}{
		{"MinIO du compose", "minio:9000"},
		{"localhost", "localhost:9000"},
		{"boucle locale", "127.0.0.1:9000"},
		{"boucle locale IPv6", "[::1]:9000"},
		{"réseau domestique", "192.168.1.10:9000"},
		{"réseau privé 10/8", "10.0.0.5:9000"},
		{"adresse unique locale IPv6", "[fd00::1]:9000"},
		{"S3 public", "s3.eu-west-3.amazonaws.com"},
		{"avec schéma", "https://s3.exemple.fr"},
		{"vide — backend local", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := CheckEndpoint(c.endpoint); err != nil {
				t.Errorf("CheckEndpoint(%q) = %v, attendu nil", c.endpoint, err)
			}
		})
	}
}

func TestHostOf(t *testing.T) {
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
		if got := hostOf(input); got != want {
			t.Errorf("hostOf(%q) = %q, attendu %q", input, got, want)
		}
	}
}
