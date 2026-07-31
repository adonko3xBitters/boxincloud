/*
Package netguard refuse les adresses qu'un serveur n'a aucune raison de joindre.

Le problème se pose partout où une URL saisie depuis l'interface devient une
requête émise PAR le serveur : l'adresse d'un backend S3, celle d'un catalogue
OPDS distant. C'est la définition d'une SSRF — un administrateur, ou une session
d'administrateur compromise, fait sonder le réseau interne depuis l'intérieur.

La parade habituelle est de refuser les adresses privées. Elle est ici hors de
question : `minio:9000`, `192.168.1.10` et `localhost` sont les adresses
NORMALES d'une instance auto-hébergée, et le catalogue qu'on fédère est souvent
le Komga du même réseau. Les interdire ne sécuriserait rien, cela casserait le
cas d'usage principal.

Ce qui est refusé, c'est ce qui n'a aucune raison légitime d'être ni un stockage
d'albums ni un catalogue :

  - Le lien-local IPv4 169.254.0.0/16, et 169.254.169.254 en particulier. C'est
    le service de métadonnées d'AWS, GCP, Azure, DigitalOcean et Hetzner ; il y
    délivre des identifiants d'instance à qui sait le demander.
  - Le lien-local IPv6 fe80::/10 et fd00:ec2::254, son équivalent chez AWS.
  - 0.0.0.0/8, qui désigne « cet hôte » et sert surtout à contourner les filtres
    écrits trop vite.

La liste est courte parce qu'elle doit l'être : chaque entrée supplémentaire est
une adresse que quelqu'un, quelque part, utilise légitimement.
*/
package netguard

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ErrForbidden signale une adresse que le serveur refuse de joindre.
type ErrForbidden struct {
	Endpoint string
	Reason   string
}

func (e ErrForbidden) Error() string {
	return fmt.Sprintf("adresse refusée (%s) : %s", e.Endpoint, e.Reason)
}

/*
Check refuse une adresse manifestement illégitime.

Le contrôle porte sur ce qui est saisi, pas sur ce que le nom résoudra. La
résolution peut changer entre la vérification et l'usage — c'est la faille
« time-of-check to time-of-use », et la fermer demanderait un résolveur maison
branché sur le transport HTTP.

Ce n'est pas fait, et il vaut mieux le dire que le laisser croire : le contrôle
arrête une saisie hostile, pas un nom de domaine qui pointerait vers le service
de métadonnées. Contre ce second cas, la vraie défense est ailleurs — refuser au
conteneur l'accès au réseau de métadonnées.
*/
func Check(endpoint string) error {
	if endpoint == "" {
		return nil
	}

	host := Host(endpoint)
	if host == "" {
		return ErrForbidden{Endpoint: endpoint, Reason: "adresse illisible"}
	}

	ip := net.ParseIP(host)
	if ip == nil {
		// Un nom, pas une adresse : rien à vérifier ici. Voir la réserve
		// ci-dessus.
		return nil
	}

	if reason := forbiddenReason(ip); reason != "" {
		return ErrForbidden{Endpoint: endpoint, Reason: reason}
	}
	return nil
}

func forbiddenReason(ip net.IP) string {
	if ip.IsLinkLocalUnicast() {
		return "adresse de lien-local, où répondent les services de métadonnées d'instance"
	}
	if ip.IsUnspecified() {
		return "adresse non spécifiée"
	}

	// fd00:ec2::254 — métadonnées AWS en IPv6. Adresse unique locale par
	// ailleurs légitime, donc testée précisément plutôt que par préfixe.
	if ip.Equal(net.ParseIP("fd00:ec2::254")) {
		return "service de métadonnées d'instance"
	}

	// 0.0.0.0/8 en entier : « cet hôte, ce réseau ».
	if v4 := ip.To4(); v4 != nil && v4[0] == 0 {
		return "adresse réservée « cet hôte »"
	}

	return ""
}

/*
Host extrait l'hôte d'une adresse.

Les formes acceptées sont celles que les clients S3 tolèrent, et qu'on retrouve
donc dans les configurations recopiées : `minio:9000`, `https://s3.exemple.fr`,
`s3.exemple.fr`, `[::1]:9000`.
*/
func Host(endpoint string) string {
	raw := strings.TrimSpace(endpoint)

	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return ""
		}
		return parsed.Hostname()
	}

	if host, _, err := net.SplitHostPort(raw); err == nil {
		return host
	}

	// Ni schéma ni port : l'adresse est l'hôte. Les crochets d'une IPv6 nue
	// sont retirés, `net.ParseIP` ne les acceptant pas.
	return strings.Trim(raw, "[]")
}
