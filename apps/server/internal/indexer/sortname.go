package indexer

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// SortName normalise un nom de série pour le tri et l'unicité.
//
// Trois transformations, dans cet ordre :
//
//  1. minuscules ;
//  2. retrait des accents — « Astérix » et « Asterix » doivent désigner la
//     même série, et le classement doit placer Astérix à la lettre A ;
//  3. retrait de l'article de tête — « Les Aventures de Tintin » se classe à
//     « A », pas à « L ». C'est ce qu'attend un lecteur qui parcourt une
//     étagère.
//
// Le désaccentuage doit correspondre exactement à celui de PostgreSQL
// (immutable_unaccent). Une divergence entre les deux crée des séries
// dupliquées : l'index d'unicité porte sur sort_name, et deux graphies du même
// nom passent alors au travers.
func SortName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = removeAccents(s)

	for _, article := range []string{"le ", "la ", "les ", "l'", "un ", "une ", "des ", "the ", "a ", "an "} {
		if strings.HasPrefix(s, article) {
			s = strings.TrimSpace(strings.TrimPrefix(s, article))
			break
		}
	}
	return collapseSpaces(s)
}

// removeAccents décompose puis retire les marques diacritiques.
//
// NFD sépare « é » en « e » + accent aigu ; on écarte ensuite les marques.
// C'est le même principe que le dictionnaire unaccent de PostgreSQL.
func removeAccents(s string) string {
	t := transform.Chain(
		norm.NFD,
		runes.Remove(runes.In(unicode.Mn)),
		norm.NFC,
	)
	result, _, err := transform.String(t, s)
	if err != nil {
		return s
	}
	return result
}
