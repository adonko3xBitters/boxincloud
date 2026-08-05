package amule

import "math"

/*
Conversions bornées des valeurs venues du démon.

Tout ce que la traduction lit arrive en `uint64` : c'est le type que rend un tag
entier, quelle que soit sa largeur sur le fil. Les types du domaine, eux, sont
plus étroits — un port tient dans un `int`, un débit dans un `int64`, un
identifiant interne dans un `uint32`.

Une conversion directe est silencieusement fausse pour une valeur aberrante :
`int64(v)` d'un `uint64` au-delà de 2^63 rend un NÉGATIF, et un débit négatif
traverse ensuite toute l'application jusqu'à l'écran, où il n'a aucun sens.

Ces fonctions écrêtent au lieu de retourner. La valeur reste fausse — elle
l'était déjà — mais elle reste du bon côté de zéro, et les comparaisons qui la
suivent gardent un sens.

# Pourquoi écrêter plutôt qu'échouer

Parce qu'un compteur aberrant n'est pas une raison de jeter tout l'instantané.
Un débit incohérent affiche un chiffre incohérent ; un instantané refusé fait
disparaître la file de téléchargement entière de l'écran. Le second est
franchement pire, et pour un cas qui, sur un démon local, ne se produit pas.
*/

// asInt64 borne une valeur du démon dans un int64 positif.
func asInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}

// asInt borne une valeur du démon dans un int positif.
//
// Passe par asInt64 : sur une plateforme 32 bits, int est plus étroit qu'int64,
// et la borne doit tenir compte des deux.
func asInt(v uint64) int {
	if v > uint64(math.MaxInt) {
		return math.MaxInt
	}
	return int(v)
}

// asUint32 borne un identifiant interne du démon.
//
// Ces valeurs sont des numéros de fichier ou de client attribués par amuled :
// elles tiennent dans 32 bits par construction, et une valeur au-delà signale
// un décodage qui a dérivé plutôt qu'un démon qui compte haut.
func asUint32(v uint64) uint32 {
	if v > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(v)
}

// asByte extrait un octet, en écartant explicitement le reste.
//
// Sert au découpage d'une adresse IPv4 reçue comme entier. Le masque dit que la
// troncature est VOULUE, là où une conversion nue laisse croire à un oubli.
func asByte(v uint64) byte {
	return byte(v & 0xFF)
}
