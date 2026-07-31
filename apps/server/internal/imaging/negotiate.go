package imaging

import (
	"strconv"
	"strings"
)

// Choix du format de sortie d'après ce que le client déclare savoir lire.
//
// Le principe est celui de la négociation de contenu HTTP : le client annonce
// ses formats dans `Accept`, le serveur choisit parmi ceux qu'il sait produire.
// Rien d'exotique — c'est ce que fait n'importe quel CDN d'images.
//
// Un choix mérite d'être expliqué : **seule une mention explicite compte.** Un
// client qui envoie un joker — curl, un script, une bibliothèque HTTP qui n'a
// pas d'opinion — reçoit du JPEG, alors que « tous types acceptés » l'autorise
// littéralement à recevoir n'importe quoi.
//
// C'est délibéré. Ce joker ne veut pas dire « je sais tout lire », il veut dire
// « je n'ai rien à déclarer », et les deux se ressemblent surtout quand on a
// tort. Les navigateurs qui savent lire du WebP ou de l'AVIF le nomment tous
// explicitement ; il n'existe donc aucun client qu'on prive d'un format en
// étant strict, alors qu'il existe des clients qu'on casserait en étant
// permissif.

// Negotiate choisit le premier format offert que le client accepte.
//
// Les formats sont donnés par préférence du serveur, du meilleur au moins bon.
// Le dernier fait office de repli : il est retourné quand rien ne correspond,
// et doit donc être un format que tout le monde lit.
func Negotiate(accept string, offered ...Format) Format {
	if len(offered) == 0 {
		return FormatJPEG
	}
	fallback := offered[len(offered)-1]

	if accept == "" {
		return fallback
	}

	accepted := parseAccept(accept)
	for _, format := range offered {
		if accepted[format.ContentType()] {
			return format
		}
	}
	return fallback
}

// parseAccept retourne les types MIME explicitement acceptés.
//
// Les jokers sont ignorés — voir plus haut. Un `q=0` exclut : c'est la façon
// normative de dire « surtout pas celui-là », et un client qui prend la peine
// de l'écrire a une raison.
func parseAccept(header string) map[string]bool {
	out := make(map[string]bool)

	for _, part := range strings.Split(header, ",") {
		fields := strings.Split(strings.TrimSpace(part), ";")
		media := strings.ToLower(strings.TrimSpace(fields[0]))
		if media == "" || strings.Contains(media, "*") {
			continue
		}

		quality := 1.0
		for _, parameter := range fields[1:] {
			name, value, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if !found || !strings.EqualFold(strings.TrimSpace(name), "q") {
				continue
			}
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				quality = parsed
			}
		}

		if quality > 0 {
			out[media] = true
		}
	}

	return out
}
