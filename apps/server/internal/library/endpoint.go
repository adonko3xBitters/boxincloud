package library

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

/*
Contrôle des adresses de backend.

Un backend S3 est une adresse que le SERVEUR va joindre, saisie depuis
l'interface. C'est la définition d'une SSRF : un administrateur — ou une session
d'administrateur compromise — peut faire émettre des requêtes depuis l'intérieur
du réseau, vers des cibles que l'extérieur n'atteint pas.

La parade habituelle est de refuser les adresses privées. Elle est ici hors de
question : `minio:9000`, `192.168.1.10` et `localhost` sont les adresses
NORMALES d'une instance auto-hébergée. Les interdire ne sécuriserait rien, cela
casserait le cas d'usage principal.

Ce qui est refusé, c'est ce qui n'a aucune raison légitime d'être un stockage
d'albums :

  - Le lien-local IPv4 169.254.0.0/16, et 169.254.169.254 en particulier. C'est
    le service de métadonnées d'AWS, GCP, Azure, DigitalOcean et Hetzner ; il
    y délivre des identifiants d'instance à qui sait le demander. Aucun serveur
    S3 n'y répond jamais.
  - Le lien-local IPv6 fe80::/10 et fd00:ec2::254, son équivalent chez AWS.
  - 0.0.0.0/8, qui désigne « cet hôte » et sert surtout à contourner les
    filtres écrits trop vite.

La liste est courte parce qu'elle doit l'être : chaque entrée supplémentaire
est une adresse que quelqu'un, quelque part, utilise légitimement.
*/

// ErrForbiddenEndpoint signale une adresse que le serveur refuse de joindre.
type ErrForbiddenEndpoint struct {
	Endpoint string
	Reason   string
}

func (e ErrForbiddenEndpoint) Error() string {
	return fmt.Sprintf("adresse de backend refusée (%s) : %s", e.Endpoint, e.Reason)
}

func (e ErrForbiddenEndpoint) Is(target error) bool {
	return target == ErrInvalidConfig
}

/*
CheckEndpoint refuse une adresse manifestement illégitime.

Le contrôle porte sur ce qui est saisi, pas sur ce que le nom résoudra. La
résolution peut changer entre la vérification et l'usage — c'est la faille
« time-of-check to time-of-use », et la fermer demanderait un résolveur maison
branché sur le transport HTTP.

Ce n'est pas fait, et il vaut mieux le dire que le laisser croire : le contrôle
arrête une saisie hostile, pas un nom de domaine qui pointerait vers le service
de métadonnées. Contre ce second cas, la vraie défense est ailleurs — refuser
au conteneur l'accès au réseau de métadonnées.
*/
func CheckEndpoint(endpoint string) error {
	if endpoint == "" {
		return nil
	}

	host := hostOf(endpoint)
	if host == "" {
		return ErrForbiddenEndpoint{Endpoint: endpoint, Reason: "adresse illisible"}
	}

	ip := net.ParseIP(host)
	if ip == nil {
		// Un nom, pas une adresse : rien à vérifier ici. Voir la réserve
		// ci-dessus.
		return nil
	}

	if reason := forbiddenReason(ip); reason != "" {
		return ErrForbiddenEndpoint{Endpoint: endpoint, Reason: reason}
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
hostOf extrait l'hôte d'une adresse de backend.

Les formes acceptées sont celles que les clients S3 tolèrent, et qu'on retrouve
donc dans les configurations recopiées : `minio:9000`, `https://s3.exemple.fr`,
`s3.exemple.fr`, `[::1]:9000`.
*/
func hostOf(endpoint string) string {
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
