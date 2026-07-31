package discovery

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

/*
Enrichissement d'un album importé.

Un album qui arrive d'un catalogue OPDS n'apporte souvent qu'un titre. Le reste
— résumé, langue — se déduit d'un nom de fichier quand on a de la chance, et
manque le reste du temps. Les bases de métadonnées comblent ce trou, une fois,
au moment où l'album entre.

# Pourquoi un seuil ne suffit pas, et ce qui le remplace

Le premier réglage était un seuil de confiance élevé. Il ne pouvait pas marcher,
et l'écrire l'a montré : au moment de l'import, on ne connaît que le TITRE — il
sort du nom de fichier. Un titre identique vaut 0,6 dans le barème, un auteur
commun ajoute le reste, et il n'y a pas d'auteur à comparer. Aucun
enrichissement n'aurait donc jamais eu lieu.

Baisser le seuil à 0,6 aurait rendu la chose fonctionnelle et dangereuse : tout
titre exactement identique aurait été accepté, y compris « Intégrale » ou
« Tome 1 », que des dizaines d'œuvres portent.

Trois garde-fous le remplacent, et il faut être précis sur ce que chacun couvre
— l'écriture des tests a montré qu'un seul d'entre eux portait tout le poids.

**Le titre doit être identique** après normalisation. C'est le plancher, et il
écarte l'essentiel : une base rend toujours quelque chose, et sans cette
exigence on écrirait la fiche du premier résultat venu.

**Le titre doit être distinctif.** C'est le garde-fou qui compte vraiment ici, et
celui qu'on oublie. « Intégrale », « Tome 1 », « Volume 2 » sont des titres
exacts que des dizaines d'œuvres portent, et les noms de fichiers en sont pleins.
Un titre générique n'identifie rien, quel que soit le score.

**Les bases ne doivent pas se contredire.** Utile, mais moins qu'il n'y paraît :
un candidat au titre différent tombe déjà sous le seuil, donc la contradiction
ne survient que lorsqu'un accord d'auteur remonte un titre divergent. Le
mentionner comme protection principale serait se raconter une histoire.

Ce n'est pas de la timidité. Personne ne regarde pendant qu'un job tourne, et
une fiche fausse mais plausible ne se remarque pas — elle se découvre des mois
plus tard, quand on cherche pourquoi le résumé d'un album parle d'autre chose.
Une absence, elle, se voit tout de suite et se corrige d'un clic.

Le rapport est asymétrique : le gain d'un enrichissement automatique réussi est
un résumé qu'on aurait pu saisir soi-même ; le coût d'un échec est une donnée
fausse qu'on ne saura plus distinguer d'une vraie.

# Ce qu'il ne fait pas

Il ne touche ni au titre, ni au numéro de tome. Ces deux champs viennent du nom
de fichier ou du ComicInfo.xml, qui décrivent CETTE édition ; une base
généraliste propose l'œuvre, souvent dans une autre langue et une autre
pagination. Remplacer « Druuna T03 » par « Druuna: Creatura » ferait perdre le
classement de la série pour gagner un titre plus joli.
*/

/*
EnrichThreshold est la confiance minimale d'un candidat retenu.

0,6 correspond à un titre identique après normalisation — accents, casse et
ponctuation repliés. C'est le plancher, pas le critère : la décision se prend
ensuite sur l'accord entre bases.
*/
const EnrichThreshold = 0.6

/*
ComicWriter applique une fiche à un album.

Déclarée au point d'usage : ce paquet ne connaît pas le catalogue. La signature
dit exactement ce qu'elle promet — remplir des trous, jamais écraser — et
c'est l'implémentation SQL qui le garantit, pas une convention.
*/
type ComicWriter interface {
	// Enrich ne remplit que les champs vides et non verrouillés par une saisie
	// manuelle.
	Enrich(ctx context.Context, comicID uuid.UUID, summary, language string) error
}

// SetComicWriter branche l'écriture des métadonnées enrichies.
func (s *Service) SetComicWriter(writer ComicWriter) { s.comics = writer }

/*
enrichImported complète un album fraîchement importé.

Jamais fatal. Un import réussi dont l'enrichissement échoue reste un import
réussi : l'album est là, lisible, et son résumé manquera — ce qui est
exactement l'état dans lequel il serait arrivé sans cette étape.
*/
func (s *Service) enrichImported(ctx context.Context, comicID uuid.UUID, title string) {
	if s.comics == nil || s.registry == nil || comicID == uuid.Nil {
		return
	}

	work := Work{Title: title}
	if tooGeneric(work.Title) {
		// Rien à identifier : un titre générique rend un candidat exact et
		// pourtant sans rapport.
		s.log.Debug("titre trop générique pour un enrichissement automatique",
			slog.String("comic", comicID.String()), slog.String("titre", title))
		return
	}

	described, err := s.Describe(ctx, work)
	if err != nil {
		s.log.Info("enrichissement impossible",
			slog.String("comic", comicID.String()), slog.Any("err", err))
		return
	}

	best, ok := corroborated(described.Candidates, EnrichThreshold)
	if !ok {
		// Soit rien d'assez sûr, soit les bases se contredisent. Dans les deux
		// cas mieux vaut un album incomplet qu'un album décrit par la fiche
		// d'un autre.
		s.log.Debug("aucune fiche corroborée",
			slog.String("comic", comicID.String()), slog.String("titre", title))
		return
	}

	if best.Summary == "" && best.Language == "" {
		return
	}

	if err := s.comics.Enrich(ctx, comicID, best.Summary, best.Language); err != nil {
		s.log.Warn("métadonnées enrichies non enregistrées",
			slog.String("comic", comicID.String()), slog.Any("err", err))
		return
	}

	s.log.Info("album enrichi",
		slog.String("comic", comicID.String()),
		slog.String("source", best.ProviderName),
		slog.Float64("confiance", best.Confidence))
}

/*
corroborated retient une fiche seulement si les bases sont d'accord.

Deux cas l'acceptent, et un seul les résume : il ne doit pas exister de
désaccord.

  - une seule base a répondu au-dessus du seuil — personne ne la contredit ;
  - plusieurs ont répondu et désignent la même œuvre — elles se confirment.

Le désaccord, lui, est décisif : deux bases indépendantes qui proposent des
œuvres différentes pour le même titre disent précisément que ce titre est
ambigu. Aucun score ne produit cette information, parce qu'il est calculé sur
les mêmes données des deux côtés.

La comparaison porte sur le titre normalisé, pas sur l'identité de l'œuvre :
deux bases ne partagent aucun identifiant, et le titre est le seul terrain
commun. C'est grossier, et c'est le bon niveau de grossièreté — on cherche à
détecter une contradiction, pas à prouver une égalité.
*/
func corroborated(candidates []Description, min float64) (Description, bool) {
	var retained []Description
	for _, candidate := range candidates {
		if candidate.Confidence >= min {
			retained = append(retained, candidate)
		}
	}
	if len(retained) == 0 {
		return Description{}, false
	}

	reference := normalizeTitle(retained[0].Title)
	for _, candidate := range retained[1:] {
		if normalizeTitle(candidate.Title) != reference {
			return Description{}, false
		}
	}

	/*
		La plus complète l'emporte parmi celles qui s'accordent.

		Les bases ne remplissent pas les mêmes champs : Google Books a souvent
		le résumé, Open Library la langue. Prendre la première rendrait le
		résultat dépendant de l'ordre de tri, alors qu'on peut prendre la plus
		utile sans rien risquer — elles décrivent la même œuvre.
	*/
	best := retained[0]
	for _, candidate := range retained[1:] {
		if completeness(candidate) > completeness(best) {
			best = candidate
		}
	}
	return best, true
}

/*
tooGeneric écarte les titres qui n'identifient rien.

La liste est courte et vise ce qu'on trouve réellement dans les noms de fichiers
de bande dessinée, une fois le tome et la série retirés par l'analyse. Elle
n'est pas exhaustive et n'a pas à l'être : elle couvre les cas fréquents, et le
reste est rattrapé par les champs qu'on accepte d'écrire — un résumé et une
langue, tous deux visibles et corrigeables d'un clic.

La borne de longueur attrape ce que la liste manque : un titre de trois lettres
ou moins ne distingue rien, et un titre purement numérique non plus.
*/
func tooGeneric(title string) bool {
	normalized := normalizeTitle(title)
	if len([]rune(normalized)) < 4 {
		return true
	}

	// Purement numérique : « 01 », « 2024 ».
	digitsOnly := true
	for _, r := range normalized {
		if r < '0' || r > '9' {
			digitsOnly = false
			break
		}
	}
	if digitsOnly {
		return true
	}

	switch normalized {
	case "integrale", "integrales", "tome", "volume", "album", "recueil",
		"one shot", "oneshot", "hors serie", "special", "annuel",
		"chapitre", "partie", "premiere partie", "seconde partie",
		"complete", "complet", "collection", "serie", "coffret",
		"omnibus", "anthologie", "compilation", "inedit", "inedits":
		return true
	}

	// « tome 3 », « volume 2 », « partie 1 » : un préfixe générique suivi d'un
	// nombre ne dit rien de plus que le préfixe seul.
	for _, prefix := range []string{"tome ", "volume ", "partie ", "chapitre ", "album "} {
		if len(normalized) > len(prefix) && normalized[:len(prefix)] == prefix {
			rest := normalized[len(prefix):]
			numeric := true
			for _, r := range rest {
				if r < '0' || r > '9' {
					numeric = false
					break
				}
			}
			if numeric {
				return true
			}
		}
	}

	return false
}

// completeness compte les champs que l'enrichissement sait appliquer.
func completeness(d Description) int {
	score := 0
	if d.Summary != "" {
		score++
	}
	if d.Language != "" {
		score++
	}
	return score
}
