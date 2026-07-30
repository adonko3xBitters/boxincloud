// Package archive lit les archives de bande dessinée à accès aléatoire.
//
// L'objectif : afficher la page 12 d'une archive de 200 Mo posée sur un
// stockage objet distant sans la télécharger. Le format ZIP (donc CBZ) le
// permet nativement — son index est en fin de fichier et chaque entrée est
// compressée indépendamment. C'est la raison pour laquelle CBZ est le format
// recommandé par le projet.
//
// Les formats sans accès aléatoire fiable (CBR, et les PDF complexes) relèvent
// d'une autre stratégie : l'hydratation au premier accès, où un job extrait
// l'intégralité de l'archive vers le cache dérivé.
package archive

import (
	"errors"
	"path"
	"strings"
)

// Format identifie le conteneur d'une archive.
type Format string

const (
	FormatCBZ  Format = "cbz" // ZIP — accès aléatoire natif
	FormatCBR  Format = "cbr" // RAR — hydratation requise
	FormatCB7  Format = "cb7" // 7z  — hydratation requise
	FormatPDF  Format = "pdf"
	FormatEPUB Format = "epub"
)

var (
	// ErrUnsupportedFormat signale une extension que le projet ne traite pas.
	ErrUnsupportedFormat = errors.New("archive : format non supporté")
	// ErrCorrupted signale une archive illisible ou tronquée.
	ErrCorrupted = errors.New("archive : archive corrompue ou illisible")
	// ErrNoPages signale une archive valide mais sans image exploitable.
	ErrNoPages = errors.New("archive : aucune page trouvée")
)

// SupportsRandomAccess indique si le format permet de servir une page par une
// simple requête Range, sans hydratation préalable.
func (f Format) SupportsRandomAccess() bool { return f == FormatCBZ }

// DetectFormat déduit le format de l'extension d'une clé.
//
// On se fie à l'extension plutôt qu'aux octets magiques : les fichiers de BD
// sont nommés par convention, et lire l'en-tête de chaque objet coûterait une
// requête supplémentaire par fichier lors d'un scan.
func DetectFormat(key string) (Format, error) {
	switch strings.ToLower(path.Ext(key)) {
	case ".cbz", ".zip":
		return FormatCBZ, nil
	case ".cbr", ".rar":
		return FormatCBR, nil
	case ".cb7", ".7z":
		return FormatCB7, nil
	case ".pdf":
		return FormatPDF, nil
	case ".epub":
		return FormatEPUB, nil
	default:
		return "", ErrUnsupportedFormat
	}
}

// Compression identifie la méthode de compression d'une entrée ZIP.
type Compression uint16

const (
	CompressionStore   Compression = 0 // aucune
	CompressionDeflate Compression = 8 // le cas courant
)

// Entry décrit une entrée d'archive et où trouver ses octets.
//
// DataOffset et DataSize sont les coordonnées d'accès aléatoire : une fois
// persistées en base, servir la page ne demande plus qu'un seul ReadRange.
// C'est tout l'intérêt de l'indexation.
type Entry struct {
	Name string

	DataOffset int64 // offset absolu des données compressées dans l'archive
	DataSize   int64 // taille compressée
	Size       int64 // taille décompressée

	Compression Compression
}

// Index est le résultat de l'analyse d'une archive.
type Index struct {
	// Pages contient les entrées image, dans l'ordre de lecture.
	Pages []Entry
	// ComicInfo pointe vers ComicInfo.xml s'il est présent.
	ComicInfo *Entry
}

// ─── Reconnaissance des entrées ──────────────────────────────────────────────

var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".jpe": true,
	".png": true, ".gif": true, ".bmp": true,
	".webp": true, ".avif": true, ".jxl": true,
	".tif": true, ".tiff": true,
}

// IsImage indique si une entrée d'archive est une page.
func IsImage(name string) bool {
	return imageExtensions[strings.ToLower(path.Ext(name))]
}

// IsComicInfo reconnaît le fichier de métadonnées ComicInfo.xml, quelle que
// soit sa casse et son emplacement dans l'archive.
func IsComicInfo(name string) bool {
	return strings.EqualFold(path.Base(name), "ComicInfo.xml")
}

// isJunk écarte les entrées qui ne font pas partie du contenu.
//
// Les archives créées sur macOS embarquent systématiquement __MACOSX/ et des
// fichiers ._nom, qui sont des images valides du point de vue de l'extension
// mais du bruit du point de vue du lecteur.
func isJunk(name string) bool {
	if strings.HasSuffix(name, "/") {
		return true // marqueur de répertoire
	}

	base := path.Base(name)
	switch {
	case strings.HasPrefix(base, "._"),
		strings.EqualFold(base, ".DS_Store"),
		strings.EqualFold(base, "Thumbs.db"),
		strings.EqualFold(base, "desktop.ini"):
		return true
	}

	for _, part := range strings.Split(name, "/") {
		if part == "__MACOSX" {
			return true
		}
	}
	return false
}

// ─── Ordre des pages ─────────────────────────────────────────────────────────

// compareNatural ordonne deux noms en traitant les suites de chiffres comme des
// nombres.
//
// Sans cela, « page10.jpg » précéderait « page2.jpg » et l'album se lirait dans
// le désordre. C'est le défaut le plus visible qu'un lecteur puisse avoir.
func compareNatural(a, b string) int {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	i, j := 0, 0

	for i < len(la) && j < len(lb) {
		ca, cb := la[i], lb[j]

		if isDigit(ca) && isDigit(cb) {
			// Compare deux suites de chiffres comme des nombres, en ignorant
			// les zéros de tête : « 007 » et « 7 » sont le même numéro.
			si, ei := i, i
			for ei < len(la) && isDigit(la[ei]) {
				ei++
			}
			sj, ej := j, j
			for ej < len(lb) && isDigit(lb[ej]) {
				ej++
			}

			na := strings.TrimLeft(la[si:ei], "0")
			nb := strings.TrimLeft(lb[sj:ej], "0")

			switch {
			case len(na) != len(nb):
				if len(na) < len(nb) {
					return -1
				}
				return 1
			case na != nb:
				if na < nb {
					return -1
				}
				return 1
			}

			i, j = ei, ej
			continue
		}

		if ca != cb {
			if ca < cb {
				return -1
			}
			return 1
		}
		i++
		j++
	}

	switch {
	case len(la)-i < len(lb)-j:
		return -1
	case len(la)-i > len(lb)-j:
		return 1
	}
	// Départage stable sur la casse d'origine.
	return strings.Compare(a, b)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
