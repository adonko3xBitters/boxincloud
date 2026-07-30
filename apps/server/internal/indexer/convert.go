package indexer

import (
	"math"
	"strings"

	"github.com/adonko3xBitters/boxincloud/server/internal/archive"
)

// detectComicFormat reconnaît une archive de BD à sa clé.
//
// Retourne un booléen plutôt qu'une erreur : lors d'un scan, un objet qui n'est
// pas une archive est le cas nominal, pas un échec. Distinguer les deux évite
// de traiter le silence comme une erreur avalée.
func detectComicFormat(key string) (archive.Format, bool) {
	format, err := archive.DetectFormat(key)
	if err != nil {
		return "", false
	}
	return format, true
}

// Conversions bornées vers les types de la base.
//
// Les valeurs viennent de sources non fiables — nom de fichier, ComicInfo.xml
// fourni par l'utilisateur, nombre d'entrées d'une archive. Une conversion
// naïve produirait un entier négatif sur une valeur aberrante, et l'insertion
// échouerait avec une erreur incompréhensible plutôt qu'avec une donnée
// simplement bornée.

func toInt16(v int) int16 {
	switch {
	case v < math.MinInt16:
		return math.MinInt16
	case v > math.MaxInt16:
		return math.MaxInt16
	default:
		return int16(v)
	}
}

func toInt32(v int) int32 {
	switch {
	case v < math.MinInt32:
		return math.MinInt32
	case v > math.MaxInt32:
		return math.MaxInt32
	default:
		return int32(v)
	}
}

func uint16ToInt16(v uint16) int16 {
	if v > math.MaxInt16 {
		return math.MaxInt16
	}
	return int16(v)
}

// FolderOf extrait le dossier d'une clé d'objet, relatif au préfixe de la
// bibliothèque.
//
// « bd/Tintin/T11.cbz » avec le préfixe « bd/ » donne « Tintin ». Un album à la
// racine donne une chaîne vide.
func FolderOf(objectKey, rootPrefix string) string {
	rest := strings.TrimPrefix(objectKey, rootPrefix)
	rest = strings.TrimPrefix(rest, "/")

	idx := strings.LastIndex(rest, "/")
	if idx < 0 {
		return ""
	}
	return rest[:idx]
}
