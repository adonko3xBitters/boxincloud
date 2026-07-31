package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/adonko3xBitters/boxincloud/server/internal/accounts"
	"github.com/adonko3xBitters/boxincloud/server/internal/app"
	"github.com/adonko3xBitters/boxincloud/server/internal/auth"
	"github.com/adonko3xBitters/boxincloud/server/internal/config"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/gen"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/handlers"
	"github.com/adonko3xBitters/boxincloud/server/internal/library"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/comicfixture"
	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/miniotest"
	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/pgtest"
)

// Ce fichier est le garde-fou du contrat d'API.
//
// Il exerce le serveur réel — vraie base, vrai MinIO, vrais handlers — et
// valide CHAQUE réponse contre api/openapi.yaml. Une divergence entre le
// contrat publié et ce que le serveur renvoie réellement fait échouer la CI.
//
// Sans cela, le contrat dériverait silencieusement : les clients TypeScript et
// Dart sont générés depuis le fichier, pas depuis le code, et une incohérence
// ne se verrait qu'à l'exécution chez l'utilisateur.

// contractHarness assemble un serveur complet et son validateur de contrat.
type contractHarness struct {
	router      http.Handler
	router4Spec routers.Router
	spec        *openapi3.T
	token       string
	comicID     uuid.UUID
	seriesID    uuid.UUID
	libraryID   uuid.UUID

	// adminID est le compte créé à l'installation.
	adminID uuid.UUID

	// userToken ouvre une session sur un compte ORDINAIRE.
	//
	// Sans lui, toute la suite de tests s'exécute en administrateur et ne peut
	// rien prouver sur ce qui est réservé : une route dont on aurait oublié le
	// garde passerait tous les tests.
	userToken string

	// loneComicID désigne un album sans série — le cas que toute jointure sur
	// la table des séries doit supporter.
	loneComicID uuid.UUID

	// indexNow indexe un album en ligne.
	//
	// Les workers sont désactivés dans ce harnais : un album téléversé pendant
	// un test reste donc à l'état « pending », et ses pages ne sont pas
	// servies. Ce raccourci laisse un test vérifier ce qui suit l'indexation —
	// qu'un album déplacé se lit toujours, par exemple.
	indexNow func(ctx context.Context, comicID uuid.UUID) error

	// pool donne un accès direct à la base, pour les tests qui doivent semer
	// des données en masse sans passer par l'API.
	pool *pgxpool.Pool
}

func newContractHarness(t *testing.T) *contractHarness {
	t.Helper()

	pool := pgtest.Start(t)
	minio := miniotest.Start(t)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	cfg := &config.Config{
		Env:       config.EnvProduction, // sans CORS : on teste le comportement livré
		Addr:      ":0",
		SecretKey: bytes.Repeat([]byte{0x17}, 32),
		Cache: config.Cache{
			Dir:     t.TempDir(),
			MaxSize: 0,
		},
		Auth: config.Auth{
			JWTSecret:       bytes.Repeat([]byte{0x17}, 32),
			AccessTokenTTL:  15 * 60 * 1e9,
			RefreshTokenTTL: 24 * 60 * 60 * 1e9,
		},
		Jobs: config.Jobs{Enabled: false},
	}

	core, err := app.BuildCore(context.Background(), cfg, pool, log)
	if err != nil {
		t.Fatalf("BuildCore : %v", err)
	}

	router := httpapi.NewRouter(httpapi.Deps{
		Config:    cfg,
		Log:       log,
		DB:        pool,
		Build:     handlers.BuildInfo{Version: "test", Commit: "test", GoVersion: "test"},
		Auth:      core.Auth,
		Accounts:  core.Accounts,
		Catalog:   core.Catalog,
		Folders:   core.Folders,
		Libraries: core.Libraries,
		Ingest:    core.Ingest,
		Tools:     core.Tools,
		Cache:     core.Cache,
		Reader:    core.Reader,
		Progress:  core.Progress,
		Discovery: core.Discovery,
	})

	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatalf("chargement du contrat : %v", err)
	}
	// Le contrat déclare `/api/v1` comme serveur ; les requêtes de test portent
	// ce préfixe, il ne faut donc pas le neutraliser.
	specRouter, err := gorillamux.NewRouter(spec)
	if err != nil {
		t.Fatalf("routeur de validation : %v", err)
	}

	h := &contractHarness{router: router, router4Spec: specRouter, spec: spec, pool: pool}
	h.indexNow = newDirectIndexer(core)
	h.seed(t, core, minio)
	return h
}

// seed installe un jeu de données minimal : un compte, un backend, une
// bibliothèque et un album indexé.
func (h *contractHarness) seed(t *testing.T, core *app.Core, minio miniotest.Env) {
	t.Helper()
	ctx := context.Background()

	admin, err := core.Auth.Setup(ctx, "contract", "contract@example.test", "un mot de passe solide")
	if err != nil {
		t.Fatalf("création du compte : %v", err)
	}
	h.adminID = admin.ID

	tokens, err := core.Auth.Login(ctx, auth.LoginParams{
		Username:   "contract",
		Password:   "un mot de passe solide",
		DeviceName: "Contract",
		Platform:   "web",
	})
	if err != nil {
		t.Fatalf("connexion : %v", err)
	}
	h.token = tokens.AccessToken

	if _, err := core.Accounts.Create(ctx, accounts.CreateParams{
		Username: "ordinaire",
		Email:    "ordinaire@example.test",
		Password: "un autre mot de passe solide",
		Role:     "user",
	}); err != nil {
		t.Fatalf("compte ordinaire : %v", err)
	}

	userTokens, err := core.Auth.Login(ctx, auth.LoginParams{
		Username:   "ordinaire",
		Password:   "un autre mot de passe solide",
		DeviceName: "Contract",
		Platform:   "web",
	})
	if err != nil {
		t.Fatalf("connexion du compte ordinaire : %v", err)
	}
	h.userToken = userTokens.AccessToken

	backend, err := core.Libraries.CreateBackend(ctx, library.CreateBackendParams{
		Name: "minio-contract",
		Kind: storage.KindS3,
		Config: map[string]string{
			"endpoint": minio.Endpoint, "bucket": minio.Bucket,
			"use_ssl": "false", "path_style": "true",
		},
		Secrets: map[string]string{
			"access_key": minio.AccessKey, "secret_key": minio.SecretKey,
		},
	})
	if err != nil {
		t.Fatalf("backend : %v", err)
	}

	lib, err := core.Libraries.CreateLibrary(ctx, library.CreateLibraryParams{
		Name: "Contract", BackendID: backend.ID, RootPrefix: "bd/",
	})
	if err != nil {
		t.Fatalf("bibliothèque : %v", err)
	}
	h.libraryID = lib.ID

	built := comicfixture.BuildCBZ(t, comicfixture.Options{
		Pages: 6, ComicInfo: comicfixture.SampleComicInfo,
	})
	provider, err := core.Libraries.ProviderForLibrary(ctx, lib)
	if err != nil {
		t.Fatal(err)
	}
	key := "bd/Les Aventures de Tintin - T11 - Le Secret de la Licorne.cbz"
	if err := provider.Write(ctx, key, bytes.NewReader(built.Data), int64(len(built.Data)), ""); err != nil {
		t.Fatalf("téléversement : %v", err)
	}

	// Indexation en direct : le contrat porte sur l'API, pas sur
	// l'ordonnancement de la file de jobs.
	runner := newDirectRunner(core)

	// Un second album SANS série, et sans ComicInfo.xml pour lui en inventer une.
	//
	// Les one-shots sont un cas courant — Persepolis, Maus, une intégrale — et
	// pourtant un cas limite pour toute requête qui joint la table des séries.
	// Sans un exemplaire dans le jeu de test, une jointure mal formée passerait
	// toute la suite au vert et ne casserait que chez l'utilisateur.
	standalone := comicfixture.BuildCBZ(t, comicfixture.Options{Pages: 4})
	loneKey := "bd/Persepolis.cbz"
	if err := provider.Write(ctx, loneKey, bytes.NewReader(standalone.Data),
		int64(len(standalone.Data)), ""); err != nil {
		t.Fatalf("téléversement du one-shot : %v", err)
	}

	if _, err := runner(ctx, lib.ID); err != nil {
		t.Fatalf("indexation : %v", err)
	}

	comics, err := core.Queries.ListComicsByLibrary(ctx, listParams(lib.ID))
	if err != nil || len(comics) == 0 {
		t.Fatalf("aucun album indexé : %v", err)
	}

	for _, c := range comics {
		if c.SeriesID.Valid {
			h.comicID = c.ID
			h.seriesID = c.SeriesID.UUID
		} else {
			h.loneComicID = c.ID
		}
	}

	if h.comicID == uuid.Nil {
		t.Fatal("aucun album rattaché à une série")
	}
	if h.loneComicID == uuid.Nil {
		t.Fatal("aucun album sans série : le cas limite ne serait pas couvert")
	}
}

// call valide la requête contre le contrat, l'exécute, puis valide la réponse.
//
// L'ordre compte : le handler consomme le corps de la requête, donc la
// validation d'entrée doit passer avant, sur son propre lecteur.
func (h *contractHarness) call(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return h.callWith(t, method, path, body, true)
}

// callWith exécute l'appel en choisissant de valider ou non la requête.
//
// Ne pas la valider sert un cas précis : envoyer délibérément un corps que le
// contrat interdit, pour vérifier que le serveur le refuse de lui-même. Le
// contrat ne protège que les clients qui le respectent ; le serveur doit tenir
// aussi face à ceux qui ne le font pas.
func (h *contractHarness) callWith(
	t *testing.T, method, path string, body any, validateRequest bool,
) *httptest.ResponseRecorder {
	t.Helper()

	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}

	newRequest := func() *http.Request {
		var reader io.Reader
		if raw != nil {
			reader = bytes.NewReader(raw)
		}
		req := httptest.NewRequest(method, path, reader)
		if raw != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if h.token != "" {
			req.Header.Set("Authorization", "Bearer "+h.token)
		}
		return req
	}

	validationReq := newRequest()

	route, pathParams, err := h.router4Spec.FindRoute(validationReq)
	if err != nil {
		t.Fatalf("%s %s : route absente du contrat OpenAPI (%v)\n"+
			"Toute route servie doit être déclarée dans api/openapi.yaml.",
			method, path, err)
	}

	reqInput := &openapi3filter.RequestValidationInput{
		Request:    validationReq,
		PathParams: pathParams,
		Route:      route,
		Options: &openapi3filter.Options{
			// L'authentification est vérifiée par le serveur, pas par le
			// validateur : celui-ci n'a pas à rejouer la vérification du jeton.
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
	}

	if validateRequest {
		if err := openapi3filter.ValidateRequest(context.Background(), reqInput); err != nil {
			t.Errorf("%s %s : requête non conforme au contrat : %v", method, path, err)
		}
	}

	// Requête neuve pour le serveur : celle de validation a été consommée.
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, newRequest())

	responseInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: reqInput,
		Status:                 rec.Code,
		Header:                 rec.Header(),
		Options:                &openapi3filter.Options{IncludeResponseStatus: true},
	}
	responseInput.SetBodyBytes(rec.Body.Bytes())

	if err := openapi3filter.ValidateResponse(context.Background(), responseInput); err != nil {
		t.Errorf("%s %s → %d : réponse non conforme au contrat :\n  %v\n  corps : %s",
			method, path, rec.Code, err, truncate(rec.Body.String(), 400))
	}

	return rec
}

// ─── Tests ───────────────────────────────────────────────────────────────────

// TestIntegrationContract exerce toute la surface de l'API et valide chaque
// réponse contre le contrat publié.
func TestIntegrationContract(t *testing.T) {
	h := newContractHarness(t)

	t.Run("version", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/version", nil, http.StatusOK)
	})

	t.Run("authStatus", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/auth/status", nil, http.StatusOK)
	})

	t.Run("me", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/me", nil, http.StatusOK)
	})

	t.Run("devices", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/me/devices", nil, http.StatusOK)
	})

	t.Run("libraries", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/libraries", nil, http.StatusOK)
	})

	t.Run("home", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/home", nil, http.StatusOK)
	})

	t.Run("comics", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/comics?limit=10", nil, http.StatusOK)
	})

	t.Run("comicDetail", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/comics/"+h.comicID.String(), nil, http.StatusOK)
	})

	t.Run("series", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/series", nil, http.StatusOK)
	})

	t.Run("seriesDetail", func(t *testing.T) {
		if h.seriesID == uuid.Nil {
			t.Skip("aucune série rattachée")
		}
		h.expect(t, http.MethodGet, "/api/v1/series/"+h.seriesID.String(), nil, http.StatusOK)
	})

	t.Run("search", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/search?q=licorne", nil, http.StatusOK)
	})

	t.Run("manifest", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/comics/"+h.comicID.String()+"/manifest", nil, http.StatusOK)
	})

	t.Run("progressInitial", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/comics/"+h.comicID.String()+"/progress", nil, http.StatusOK)
	})

	t.Run("progressUpdate", func(t *testing.T) {
		h.expect(t, http.MethodPut, "/api/v1/comics/"+h.comicID.String()+"/progress",
			map[string]any{"page": 3, "pageCount": 6}, http.StatusOK)
	})

	t.Run("continueReading", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/continue-reading", nil, http.StatusOK)
	})

	t.Run("syncPull", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/sync", nil, http.StatusOK)
	})

	t.Run("syncPush", func(t *testing.T) {
		h.expect(t, http.MethodPost, "/api/v1/sync", map[string]any{
			"updates": []map[string]any{
				{"comicId": h.comicID.String(), "page": 5, "pageCount": 6, "status": "read"},
			},
		}, http.StatusOK)
	})

	t.Run("progressDelete", func(t *testing.T) {
		h.expect(t, http.MethodDelete, "/api/v1/comics/"+h.comicID.String()+"/progress",
			nil, http.StatusNoContent)
	})
}

// TestIntegrationContractStandalone couvre l'album sans série de bout en bout.
//
// Le détail, le manifeste, une page et la couverture : tout ce qu'il faut pour
// ouvrir un one-shot dans le lecteur. Une jointure sur la table des séries qui
// ne tolère pas l'absence de série casse ici, et nulle part ailleurs.
func TestIntegrationContractStandalone(t *testing.T) {
	h := newContractHarness(t)
	lone := "/api/v1/comics/" + h.loneComicID.String()

	t.Run("detail", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, lone, nil, http.StatusOK)

		var payload struct {
			SeriesName string `json:"seriesName"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.SeriesName != "" {
			t.Errorf("seriesName = %q, attendu vide pour un album sans série", payload.SeriesName)
		}
	})

	t.Run("manifest", func(t *testing.T) {
		h.expect(t, http.MethodGet, lone+"/manifest", nil, http.StatusOK)
	})

	t.Run("page", func(t *testing.T) {
		h.expect(t, http.MethodGet, lone+"/pages/0", nil, http.StatusOK)
	})

	t.Run("cover", func(t *testing.T) {
		h.expect(t, http.MethodGet, lone+"/cover", nil, http.StatusOK)
	})

	// La liste et la recherche joignent elles aussi la table des séries : un
	// one-shot ne doit pas les faire échouer, ni disparaître du résultat.
	t.Run("listIncludesStandalone", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/comics?limit=50", nil, http.StatusOK)

		var payload struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}

		found := false
		for _, item := range payload.Items {
			if item.ID == h.loneComicID.String() {
				found = true
			}
		}
		if !found {
			t.Error("l'album sans série est absent de la liste")
		}
	})

	t.Run("search", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/search?q=persepolis", nil, http.StatusOK)
	})

	// L'album à série doit continuer de rapporter son nom : la jointure ne doit
	// pas avoir été neutralisée en réglant le cas du one-shot.
	t.Run("seriesNameStillReported", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/comics/"+h.comicID.String(), nil, http.StatusOK)

		var payload struct {
			SeriesName string `json:"seriesName"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.SeriesName == "" {
			t.Error("seriesName vide alors que l'album appartient à une série")
		}
	})
}

// Les réponses d'erreur font partie du contrat au même titre que les autres :
// c'est sur elles que les clients branchent leur comportement.
func TestIntegrationContractErrors(t *testing.T) {
	h := newContractHarness(t)

	t.Run("setupDéjàEffectué", func(t *testing.T) {
		h.expect(t, http.MethodPost, "/api/v1/auth/setup", map[string]any{
			"username": "intrus", "password": "un autre mot de passe",
		}, http.StatusForbidden)
	})

	t.Run("albumInexistant", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/comics/"+uuid.Must(uuid.NewV7()).String(),
			nil, http.StatusNotFound)
	})

	t.Run("curseurMalformé", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/comics?cursor=pas-un-curseur",
			nil, http.StatusUnprocessableEntity)
	})

	t.Run("sansJeton", func(t *testing.T) {
		saved := h.token
		h.token = ""
		defer func() { h.token = saved }()

		h.expect(t, http.MethodGet, "/api/v1/me", nil, http.StatusUnauthorized)
	})

	t.Run("jetonInvalide", func(t *testing.T) {
		saved := h.token
		h.token = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.signature-bidon"
		defer func() { h.token = saved }()

		h.expect(t, http.MethodGet, "/api/v1/me", nil, http.StatusUnauthorized)
	})

	t.Run("pageHorsLimites", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/comics/"+h.comicID.String()+"/pages/999",
			nil, http.StatusUnprocessableEntity)
	})
}

// Les images ne sont pas du JSON : leur conformité porte sur le type de
// contenu, les en-têtes de cache et le comportement conditionnel.
func TestIntegrationContractImages(t *testing.T) {
	h := newContractHarness(t)

	t.Run("page", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/comics/"+h.comicID.String()+"/pages/0",
			nil, http.StatusOK)

		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
			t.Errorf("Content-Type = %q, attendu une image", ct)
		}
		if rec.Header().Get("ETag") == "" {
			t.Error("ETag absent : la mise en cache côté client en dépend")
		}
		if !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
			t.Error("Cache-Control devrait marquer la variante immuable")
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Error("nosniff absent : une archive pourrait contenir un faux .jpg")
		}
		if rec.Body.Len() == 0 {
			t.Error("corps vide")
		}
	})

	t.Run("cover", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/comics/"+h.comicID.String()+"/cover?width=320",
			nil, http.StatusOK)
		if rec.Body.Len() == 0 {
			t.Error("couverture vide")
		}
	})

	// Une requête conditionnelle doit répondre 304 sans corps : c'est ce qui
	// supprime tout le trafic d'images sur un album relu.
	t.Run("notModified", func(t *testing.T) {
		path := "/api/v1/comics/" + h.comicID.String() + "/pages/0"

		first := httptest.NewRequest(http.MethodGet, path, nil)
		first.Header.Set("Authorization", "Bearer "+h.token)
		rec1 := httptest.NewRecorder()
		h.router.ServeHTTP(rec1, first)

		etag := rec1.Header().Get("ETag")
		if etag == "" {
			t.Fatal("pas d'ETag sur la première réponse")
		}

		second := httptest.NewRequest(http.MethodGet, path, nil)
		second.Header.Set("Authorization", "Bearer "+h.token)
		second.Header.Set("If-None-Match", etag)
		rec2 := httptest.NewRecorder()
		h.router.ServeHTTP(rec2, second)

		if rec2.Code != http.StatusNotModified {
			t.Errorf("statut %d, attendu 304", rec2.Code)
		}
		if rec2.Body.Len() != 0 {
			t.Errorf("%d octets transférés sur un 304, attendu 0", rec2.Body.Len())
		}
	})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (h *contractHarness) expect(t *testing.T, method, path string, body any, want int) *httptest.ResponseRecorder {
	t.Helper()

	rec := h.call(t, method, path, body)
	if rec.Code != want {
		t.Errorf("%s %s : statut %d, attendu %d — corps : %s",
			method, path, rec.Code, want, truncate(rec.Body.String(), 300))
	}
	return rec
}

// expectRejected envoie un corps que le contrat interdit et exige que le
// serveur le refuse. La réponse, elle, reste validée : une erreur doit être
// conforme au contrat même quand la requête ne l'était pas.
func (h *contractHarness) expectRejected(t *testing.T, method, path string, body any, want int) {
	t.Helper()

	rec := h.callWith(t, method, path, body, false)
	if rec.Code != want {
		t.Errorf("%s %s : statut %d, attendu %d — le serveur aurait dû refuser ce corps ; corps : %s",
			method, path, rec.Code, want, truncate(rec.Body.String(), 300))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Les filtres et le tri font partie du contrat : un paramètre non déclaré
// serait rejeté ici, avant que le client web ne l'envoie en production.
func TestIntegrationContractFilters(t *testing.T) {
	h := newContractHarness(t)

	for _, query := range []string{
		"sort=recent",
		"sort=title",
		"sort=released",
		"readStatus=unread",
		"readStatus=in_progress",
		"readStatus=read",
		"readStatus=&sort=title&limit=10",
		"folder=",
		"folder=bd",
		"favorites=true",
	} {
		t.Run(query, func(t *testing.T) {
			h.expect(t, http.MethodGet, "/api/v1/comics?"+query, nil, http.StatusOK)
		})
	}
}

// TestIntegrationContractTools couvre les outils de bibliothèque : dossiers,
// favoris, notes, édition et actions en lot.
//
// Les sous-tests s'enchaînent volontairement dans l'ordre où l'utilisateur les
// déclenche — poser une note puis la retirer, mettre en favori puis vérifier
// que /me/marks le rapporte. Un endpoint validé isolément ne prouverait pas que
// ce qu'il écrit ressort ailleurs sous la forme annoncée.
func TestIntegrationContractTools(t *testing.T) {
	h := newContractHarness(t)
	comic := "/api/v1/comics/" + h.comicID.String()

	t.Run("folders", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/folders", nil, http.StatusOK)
	})

	t.Run("foldersByLibrary", func(t *testing.T) {
		h.expect(t, http.MethodGet,
			"/api/v1/folders?libraryId="+h.libraryID.String(), nil, http.StatusOK)
	})

	t.Run("marksEmpty", func(t *testing.T) {
		h.expect(t, http.MethodGet, "/api/v1/me/marks", nil, http.StatusOK)
	})

	t.Run("setFavorite", func(t *testing.T) {
		h.expect(t, http.MethodPut, comic+"/favorite",
			map[string]any{"favorite": true}, http.StatusOK)
	})

	t.Run("setRating", func(t *testing.T) {
		h.expect(t, http.MethodPut, comic+"/rating",
			map[string]any{"rating": 4}, http.StatusOK)
	})

	// Les écritures précédentes doivent ressortir ici : c'est la seule requête
	// dont dépend l'affichage des favoris et des notes dans une grille.
	t.Run("marksPopulated", func(t *testing.T) {
		rec := h.expect(t, http.MethodGet, "/api/v1/me/marks", nil, http.StatusOK)

		var payload struct {
			Favorites []string       `json:"favorites"`
			Ratings   map[string]int `json:"ratings"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Favorites) != 1 || payload.Favorites[0] != h.comicID.String() {
			t.Errorf("favoris = %v, attendu [%s]", payload.Favorites, h.comicID)
		}
		if payload.Ratings[h.comicID.String()] != 4 {
			t.Errorf("note = %d, attendu 4", payload.Ratings[h.comicID.String()])
		}
	})

	t.Run("clearRating", func(t *testing.T) {
		h.expect(t, http.MethodPut, comic+"/rating",
			map[string]any{"rating": 0}, http.StatusOK)
	})

	t.Run("ratingOutOfRange", func(t *testing.T) {
		h.expectRejected(t, http.MethodPut, comic+"/rating",
			map[string]any{"rating": 9}, http.StatusUnprocessableEntity)
	})

	t.Run("unsetFavorite", func(t *testing.T) {
		h.expect(t, http.MethodPut, comic+"/favorite",
			map[string]any{"favorite": false}, http.StatusOK)
	})

	t.Run("editComic", func(t *testing.T) {
		rec := h.expect(t, http.MethodPatch, comic,
			map[string]any{"title": "Le Secret de la Licorne"}, http.StatusOK)

		var payload struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Title != "Le Secret de la Licorne" {
			t.Errorf("titre = %q après édition", payload.Title)
		}
	})

	t.Run("bulkRead", func(t *testing.T) {
		rec := h.expect(t, http.MethodPost, "/api/v1/comics/bulk", map[string]any{
			"action": "read",
			"ids":    []string{h.comicID.String()},
		}, http.StatusOK)

		var payload struct {
			Affected int64 `json:"affected"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Affected != 1 {
			t.Errorf("affected = %d, attendu 1", payload.Affected)
		}
	})

	// Un identifiant inconnu ne doit pas faire échouer le lot : il est filtré
	// en amont de l'écriture, avec les albums hors des bibliothèques visibles.
	t.Run("bulkIgnoresUnknownIDs", func(t *testing.T) {
		rec := h.expect(t, http.MethodPost, "/api/v1/comics/bulk", map[string]any{
			"action": "unread",
			"ids":    []string{h.comicID.String(), uuid.NewString()},
		}, http.StatusOK)

		var payload struct {
			Affected int64 `json:"affected"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Affected != 1 {
			t.Errorf("affected = %d : l'identifiant inconnu aurait dû être ignoré", payload.Affected)
		}
	})

	t.Run("bulkRejectsUnknownAction", func(t *testing.T) {
		h.expectRejected(t, http.MethodPost, "/api/v1/comics/bulk", map[string]any{
			"action": "incinerate",
			"ids":    []string{h.comicID.String()},
		}, http.StatusUnprocessableEntity)
	})
}
