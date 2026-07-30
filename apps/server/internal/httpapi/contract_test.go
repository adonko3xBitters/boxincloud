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
		Config:   cfg,
		Log:      log,
		DB:       pool,
		Build:    handlers.BuildInfo{Version: "test", Commit: "test", GoVersion: "test"},
		Auth:     core.Auth,
		Catalog:  core.Catalog,
		Reader:   core.Reader,
		Progress: core.Progress,
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

	h := &contractHarness{router: router, router4Spec: specRouter, spec: spec}
	h.seed(t, core, minio)
	return h
}

// seed installe un jeu de données minimal : un compte, un backend, une
// bibliothèque et un album indexé.
func (h *contractHarness) seed(t *testing.T, core *app.Core, minio miniotest.Env) {
	t.Helper()
	ctx := context.Background()

	if _, err := core.Auth.Setup(ctx, "contract", "contract@example.test", "un mot de passe solide"); err != nil {
		t.Fatalf("création du compte : %v", err)
	}

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
	if _, err := runner(ctx, lib.ID); err != nil {
		t.Fatalf("indexation : %v", err)
	}

	comics, err := core.Queries.ListComicsByLibrary(ctx, listParams(lib.ID))
	if err != nil || len(comics) == 0 {
		t.Fatalf("aucun album indexé : %v", err)
	}
	h.comicID = comics[0].ID
	if comics[0].SeriesID.Valid {
		h.seriesID = comics[0].SeriesID.UUID
	}
}

// call valide la requête contre le contrat, l'exécute, puis valide la réponse.
//
// L'ordre compte : le handler consomme le corps de la requête, donc la
// validation d'entrée doit passer avant, sur son propre lecteur.
func (h *contractHarness) call(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
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

	if err := openapi3filter.ValidateRequest(context.Background(), reqInput); err != nil {
		t.Errorf("%s %s : requête non conforme au contrat : %v", method, path, err)
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
