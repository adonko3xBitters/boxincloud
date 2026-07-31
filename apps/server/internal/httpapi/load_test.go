package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

/*
Test de charge : dix mille albums.

Le chiffre n'est pas arbitraire. Une bibliothèque numérisée sérieuse — trente
ans de collection, une intégrale de manga, un fonds partagé en famille —
atteint cet ordre de grandeur, et c'est précisément là que les requêtes écrites
pour cinq albums se mettent à ramper.

Ce que le test mesure : le temps de réponse des trois routes que l'interface
appelle en boucle. Ce qu'il ne mesure pas : la concurrence, l'extraction de
pages, l'indexation. Un test qui mesurerait tout ne mesurerait rien.

Les données sont insérées directement en base, sans passer par le stockage :
dix mille vraies archives coûteraient des gigaoctets et des minutes pour
vérifier des requêtes SQL qui ne les lisent jamais.

Le test n'échoue pas sur un temps absolu — la machine d'intégration continue
est plus lente et plus variable qu'un poste de développement, et un seuil serré
produirait des échecs qui n'apprennent rien. Il échoue sur ce qui est
structurel : une réponse qui dépasse la seconde trahit un balayage complet de
table ou un index manquant, pas une machine chargée.
*/

// budget est le seuil d'échec, choisi large exprès. Voir ci-dessus.
const budget = time.Second

// loadSize est le nombre d'albums synthétiques.
const loadSize = 10_000

func TestIntegrationLoadTenThousand(t *testing.T) {
	if testing.Short() {
		t.Skip("test de charge : ignoré en mode court")
	}

	h := newContractHarness(t)
	seedComics(t, h, loadSize)

	measure := func(t *testing.T, name, path string) time.Duration {
		t.Helper()

		// Une première requête à blanc : elle paie les caches de plan de
		// PostgreSQL, que les suivantes réutiliseraient de toute façon en
		// production. La mesurer donnerait un chiffre qu'aucun utilisateur ne
		// vit jamais.
		h.expect(t, http.MethodGet, path, nil, http.StatusOK)

		const runs = 5
		var worst time.Duration

		for range runs {
			start := time.Now()
			h.expect(t, http.MethodGet, path, nil, http.StatusOK)
			if elapsed := time.Since(start); elapsed > worst {
				worst = elapsed
			}
		}

		// Le pire des cinq, pas la moyenne : une moyenne noie exactement la
		// requête lente qu'on cherche.
		t.Logf("%-28s %6.1f ms (pire de %d)", name, float64(worst.Microseconds())/1000, runs)
		if worst > budget {
			t.Errorf("%s : %v, au-delà du budget de %v", name, worst, budget)
		}
		return worst
	}

	t.Run("première page du catalogue", func(t *testing.T) {
		measure(t, "GET /comics?limit=100", "/api/v1/comics?limit=100")
	})

	t.Run("tri par titre", func(t *testing.T) {
		// Le tri par titre est celui qui n'a pas d'index évident : c'est là
		// qu'un balayage complet se manifeste en premier.
		measure(t, "GET /comics?sort=title", "/api/v1/comics?limit=100&sort=title")
	})

	t.Run("filtre par statut de lecture", func(t *testing.T) {
		// Jointure sur reading_progress, plus le cas « unread » qui doit
		// couvrir l'ABSENCE de ligne — le piège classique d'un LEFT JOIN.
		measure(t, "GET /comics?readStatus=unread",
			"/api/v1/comics?limit=100&readStatus=unread")
	})

	t.Run("recherche", func(t *testing.T) {
		// Trigrammes sur dix mille titres : la requête la plus coûteuse de
		// l'API, et celle qu'on tape lettre par lettre.
		measure(t, "GET /search?q=album", "/api/v1/search?q=album")
	})

	t.Run("liste des séries", func(t *testing.T) {
		measure(t, "GET /series", "/api/v1/series?limit=100")
	})

	t.Run("accueil", func(t *testing.T) {
		// Plusieurs étagères en une réponse : c'est la première chose que voit
		// quelqu'un qui ouvre l'application.
		measure(t, "GET /home", "/api/v1/home")
	})

	t.Run("pagination complète", func(t *testing.T) {
		// Le curseur doit rester constant en coût. Un OFFSET dégraderait page
		// après page, ce qui ne se voit qu'en allant au bout — et personne ne
		// va au bout à la main.
		var cursor string
		var pages, seen int
		var worst time.Duration

		for {
			path := "/api/v1/comics?limit=200"
			if cursor != "" {
				path += "&cursor=" + cursor
			}

			start := time.Now()
			rec := h.expect(t, http.MethodGet, path, nil, http.StatusOK)
			if elapsed := time.Since(start); elapsed > worst {
				worst = elapsed
			}

			var payload struct {
				Items      []json.RawMessage `json:"items"`
				NextCursor string            `json:"nextCursor"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}

			seen += len(payload.Items)
			pages++
			cursor = payload.NextCursor

			if cursor == "" || pages > 100 {
				break
			}
		}

		t.Logf("%-28s %6.1f ms (pire), %d pages, %d albums",
			"pagination complète", float64(worst.Microseconds())/1000, pages, seen)

		if seen < loadSize {
			t.Errorf("albums parcourus = %d, attendu au moins %d — la pagination perd des lignes",
				seen, loadSize)
		}
		if worst > budget {
			t.Errorf("page la plus lente : %v, au-delà du budget de %v", worst, budget)
		}
	})
}

/*
seedComics insère des albums synthétiques par COPY.

Dix mille INSERT individuels prendraient plusieurs minutes ; `CopyFrom` les
passe en une seconde. Le contenu importe peu — ces lignes ne sont jamais lues
depuis le stockage — mais leur DISTRIBUTION compte : des titres tous identiques
rendraient les index artificiellement efficaces, et le test mesurerait alors la
mise en cache plutôt que les requêtes.
*/
func seedComics(t *testing.T, h *contractHarness, count int) {
	t.Helper()

	ctx := context.Background()
	start := time.Now()

	// Graine fixe : deux exécutions doivent produire les mêmes données, sans
	// quoi une régression de performance se confondrait avec un tirage
	// malheureux.
	rng := rand.New(rand.NewSource(20260731))

	// Cent séries pour dix mille albums : une centaine de tomes chacune, ce
	// qui est la forme réelle d'un fonds de manga.
	const seriesCount = 100
	seriesIDs := make([]uuid.UUID, seriesCount)

	seriesRows := make([][]any, 0, seriesCount)
	for i := range seriesCount {
		seriesIDs[i] = uuid.Must(uuid.NewV7())
		name := fmt.Sprintf("Série %03d", i)
		seriesRows = append(seriesRows, []any{
			seriesIDs[i], h.libraryID, name, strings.ToLower(name), 0,
		})
	}

	if _, err := h.pool.CopyFrom(ctx,
		pgx.Identifier{"series"},
		[]string{"id", "library_id", "name", "sort_name", "comic_count"},
		pgx.CopyFromRows(seriesRows),
	); err != nil {
		t.Fatalf("insertion des séries : %v", err)
	}

	adjectives := []string{
		"Rouge", "Noir", "Perdu", "Dernier", "Grand", "Silencieux",
		"Oublié", "Nouveau", "Vieux", "Étrange",
	}
	nouns := []string{
		"Voyage", "Secret", "Combat", "Retour", "Album", "Chemin",
		"Royaume", "Rivage", "Serment", "Passage",
	}

	comicRows := make([][]any, 0, count)
	for i := range count {
		seriesID := seriesIDs[i%seriesCount]
		title := fmt.Sprintf("%s %s %d",
			adjectives[rng.Intn(len(adjectives))],
			nouns[rng.Intn(len(nouns))],
			i)

		// Une arborescence, pas un dossier unique : le filtrage par préfixe
		// doit travailler sur des chemins qui se ressemblent.
		folder := fmt.Sprintf("Série %03d", i%seriesCount)

		comicRows = append(comicRows, []any{
			uuid.Must(uuid.NewV7()),
			h.libraryID,
			seriesID,
			fmt.Sprintf("charge/%06d.cbz", i),
			int64(2_000_000 + rng.Intn(50_000_000)),
			"cbz",
			title,
			fmt.Sprintf("%d", i%100+1),
			float64(i%100 + 1),
			30 + rng.Intn(80),
			"ready",
			folder,
			time.Now().UTC(),
		})
	}

	if _, err := h.pool.CopyFrom(ctx,
		pgx.Identifier{"comics"},
		[]string{
			"id", "library_id", "series_id", "object_key", "file_size", "format",
			"title", "number", "number_sort", "page_count", "state", "folder_path",
			"indexed_at",
		},
		pgx.CopyFromRows(comicRows),
	); err != nil {
		t.Fatalf("insertion des albums : %v", err)
	}

	// ANALYZE avant de mesurer : sans statistiques à jour, le planificateur
	// choisit ses plans sur une table qu'il croit vide, et le test mesurerait
	// une situation qui ne se produit jamais en pratique.
	if _, err := h.pool.Exec(ctx, "ANALYZE comics, series"); err != nil {
		t.Fatalf("ANALYZE : %v", err)
	}

	t.Logf("%d albums et %d séries insérés en %v",
		count, seriesCount, time.Since(start).Round(time.Millisecond))
}
