package indexer

import (
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// ComicInfo reprend les champs de ComicInfo.xml, le format de métadonnées de
// fait des fichiers CBZ.
//
// Seuls les champs exploités par boxincloud sont modélisés ; le XML complet est
// conservé tel quel dans comics.metadata, pour ne rien perdre de ce qu'un futur
// jalon pourrait vouloir utiliser.
type ComicInfo struct {
	XMLName xml.Name `xml:"ComicInfo"`

	Title     string `xml:"Title"`
	Series    string `xml:"Series"`
	Number    string `xml:"Number"`
	Volume    string `xml:"Volume"`
	Summary   string `xml:"Summary"`
	Year      string `xml:"Year"`
	Month     string `xml:"Month"`
	Day       string `xml:"Day"`
	Writer    string `xml:"Writer"`
	Penciller string `xml:"Penciller"`
	Publisher string `xml:"Publisher"`
	Genre     string `xml:"Genre"`
	Language  string `xml:"LanguageISO"`
	PageCount string `xml:"PageCount"`
	AgeRating string `xml:"AgeRating"`
}

// ParseComicInfo lit un ComicInfo.xml.
func ParseComicInfo(r io.Reader) (*ComicInfo, []byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, 1<<20)) // 1 Mio : largement suffisant
	if err != nil {
		return nil, nil, fmt.Errorf("lecture de ComicInfo.xml : %w", err)
	}

	var info ComicInfo
	if err := xml.Unmarshal(raw, &info); err != nil {
		return nil, raw, fmt.Errorf("ComicInfo.xml illisible : %w", err)
	}
	return &info, raw, nil
}

// Metadata est le résultat normalisé de l'extraction, quelle qu'en soit la
// source.
type Metadata struct {
	Title      string
	Series     string
	Number     string
	NumberSort *float64
	Volume     *int16
	Summary    string
	Year       int
	Month      int
	Day        int
	Language   string
	AgeRating  *int16
}

// ToMetadata normalise un ComicInfo.
func (c *ComicInfo) ToMetadata() Metadata {
	m := Metadata{
		Title:    strings.TrimSpace(c.Title),
		Series:   strings.TrimSpace(c.Series),
		Number:   strings.TrimSpace(c.Number),
		Summary:  strings.TrimSpace(c.Summary),
		Language: strings.TrimSpace(c.Language),
	}

	if v := parseNumberSort(m.Number); v != nil {
		m.NumberSort = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(c.Volume)); err == nil {
		vol := toInt16(v)
		m.Volume = &vol
	}
	m.Year, _ = strconv.Atoi(strings.TrimSpace(c.Year))
	m.Month, _ = strconv.Atoi(strings.TrimSpace(c.Month))
	m.Day, _ = strconv.Atoi(strings.TrimSpace(c.Day))

	if r := ageRatingToNumber(c.AgeRating); r != nil {
		m.AgeRating = r
	}
	return m
}

// ─── Analyse du nom de fichier ───────────────────────────────────────────────

// Motifs reconnus, du plus spécifique au plus général. L'ordre compte : un
// « T01 » explicite doit primer sur un nombre isolé.
var filenamePatterns = []*regexp.Regexp{
	// « Série - T01 - Titre », « Série T01 », « Série - Tome 3 »
	regexp.MustCompile(`^(?P<series>.+?)[\s_-]+[Tt](?:ome)?[\s_.]?(?P<number>\d+(?:\.\d+)?)(?:[\s_-]+(?P<title>.+))?$`),
	// « Série #12 - Titre »
	regexp.MustCompile(`^(?P<series>.+?)[\s_-]+#(?P<number>\d+(?:\.\d+)?)(?:[\s_-]+(?P<title>.+))?$`),
	// « Série v03 »
	regexp.MustCompile(`^(?P<series>.+?)[\s_-]+[Vv](?:ol)?[\s_.]?(?P<number>\d+(?:\.\d+)?)(?:[\s_-]+(?P<title>.+))?$`),
	// « Série 012 - Titre » : un nombre isolé en dernier recours
	regexp.MustCompile(`^(?P<series>.+?)[\s_-]+(?P<number>\d{1,4}(?:\.\d+)?)(?:[\s_-]+(?P<title>.+))?$`),
}

// Éléments de nommage sans valeur éditoriale, fréquents dans les collections
// numériques.
var noiseTokens = regexp.MustCompile(`(?i)[\s_-]*[\[(](?:` +
	`\d{3,4}p|fr|french|vf|scan|webrip|hd|c2c|digital|nerd|empire|minutemen|dcp|team[^\])]*` +
	`)[\])]`)

// ParseFilename déduit série, numéro et titre du nom de fichier.
//
// Repli utilisé quand l'archive n'a pas de ComicInfo.xml — c'est-à-dire la
// majorité des collections constituées à la main. La déduction est
// délibérément prudente : mieux vaut une série absente qu'une série fausse,
// qu'un utilisateur devra corriger album par album.
func ParseFilename(key string) Metadata {
	base := path.Base(key)
	name := strings.TrimSuffix(base, path.Ext(base))

	name = noiseTokens.ReplaceAllString(name, "")
	name = strings.TrimSpace(strings.NewReplacer("_", " ", ".", " ").Replace(name))
	name = collapseSpaces(name)

	m := Metadata{Title: name}

	for _, re := range filenamePatterns {
		match := re.FindStringSubmatch(name)
		if match == nil {
			continue
		}

		groups := make(map[string]string)
		for i, key := range re.SubexpNames() {
			if key != "" && i < len(match) {
				groups[key] = strings.TrimSpace(match[i])
			}
		}

		series := collapseSpaces(strings.Trim(groups["series"], " -_"))
		if series == "" {
			continue
		}

		m.Series = series
		m.Number = groups["number"]
		m.NumberSort = parseNumberSort(m.Number)

		if title := collapseSpaces(strings.Trim(groups["title"], " -_")); title != "" {
			m.Title = title
		} else if m.Number != "" {
			// Sans titre propre, « Série - T01 » se lit mieux que le nom brut.
			m.Title = series + " - " + m.Number
		}
		break
	}

	return m
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// parseNumberSort dérive une valeur triable d'un numéro textuel.
//
// Les numéros réels sont sales : « 3 », « 3.5 », « HS », « Tome 2 ». On extrait
// le premier nombre s'il y en a un, et on renonce sinon — un tri approximatif
// vaut mieux qu'un tri faux.
func parseNumberSort(s string) *float64 {
	m := regexp.MustCompile(`\d+(?:\.\d+)?`).FindString(s)
	if m == "" {
		return nil
	}
	v, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return nil
	}
	return &v
}

// ageRatingToNumber traduit les mentions ComicInfo en âge minimum.
func ageRatingToNumber(s string) *int16 {
	var age int16
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "everyone", "all ages", "g":
		age = 0
	case "everyone 10+", "pg":
		age = 10
	case "teen", "t":
		age = 13
	case "mature 17+", "ma15+", "m":
		age = 17
	case "adults only 18+", "r18+", "x18+":
		age = 18
	default:
		return nil
	}
	return &age
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
