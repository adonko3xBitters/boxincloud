// Package httpapi assemble le routeur HTTP du serveur.
//
// Chi plutôt que Fiber : Chi est un routeur net/http standard. Le streaming et
// les requêtes Range — le cœur du produit — s'appuient sur le comportement de
// net/http, et tout l'écosystème de middlewares Go reste utilisable. Voir
// docs/01-architecture.md, ADR-001.
package httpapi

import (
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/adonko3xBitters/boxincloud/server/internal/auth"
	"github.com/adonko3xBitters/boxincloud/server/internal/config"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/handlers"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/middleware"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
)

// Deps rassemble ce dont le routeur a besoin.
//
// Le câblage se fait dans cmd/ : le routeur ne construit rien lui-même, ce qui
// le rend testable avec des doublures.
type Deps struct {
	Config *config.Config
	Log    *slog.Logger
	DB     handlers.Pinger
	Build  handlers.BuildInfo
	Auth   *auth.Service
	WebFS  fs.FS // application web embarquée ; nil pour ne rien servir
}

// NewRouter assemble le routeur complet.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	// Pas de chimw.RealIP : il réécrit r.RemoteAddr à partir de
	// X-Forwarded-For / X-Real-IP sans vérifier que ces en-têtes viennent bien
	// d'un proxy de confiance, ce qui les rend usurpables par le client
	// (GHSA-3fxj-6jh8-hvhx). Comme l'adresse servira à limiter le débit sur
	// l'authentification, l'usurpation contournerait la protection.
	// Le support des proxys de confiance viendra avec M3, configuré
	// explicitement.
	r.Use(middleware.Logger(d.Log))
	r.Use(middleware.Recover)
	r.Use(chimw.Compress(5, "application/json", "text/html", "text/css", "application/javascript"))

	// En développement, le web tourne sur son propre port (next dev) : il lui
	// faut du CORS. En production il est servi par ce même binaire, donc
	// même origine, donc aucun CORS nécessaire.
	if d.Config.Env.IsDevelopment() {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"http://localhost:3000", "http://127.0.0.1:3000"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
			AllowCredentials: true,
			MaxAge:           300,
		}))
	}

	health := handlers.NewHealth(d.DB, d.Build)

	// Sondes hors /api : elles ne font pas partie du contrat public et ne
	// doivent pas être versionnées avec lui.
	r.Get("/healthz", health.Live)
	r.Get("/readyz", health.Ready)

	authHandler := handlers.NewAuth(d.Auth)

	r.Route("/api/v1", func(r chi.Router) {
		// Une lecture de page peut être longue sur un backend distant, mais
		// une requête d'API ne doit jamais rester pendante indéfiniment.
		r.Use(chimw.Timeout(30 * time.Second))

		r.Get("/version", health.Version)

		// ── Routes publiques ────────────────────────────────────────────
		r.Route("/auth", func(r chi.Router) {
			r.Get("/status", authHandler.Status)
			r.Post("/setup", authHandler.Setup)
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.Refresh)
			r.Post("/logout", authHandler.Logout)
		})

		// ── Routes authentifiées ────────────────────────────────────────
		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticate(d.Auth))

			r.Get("/me", authHandler.Me)
			r.Get("/me/devices", authHandler.ListDevices)
			r.Post("/me/logout-all", authHandler.LogoutAll)

			// M2, suite : /libraries, /series, /comics, /progress, /sync
		})
	})

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/api/") {
			problem.Write(w, req, problem.NotFound("no route matches this path"))
			return
		}
		serveWeb(d, w, req)
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		problem.Write(w, req, problem.Problem{
			Type:   "https://boxincloud.dev/problems/method-not-allowed",
			Title:  "Method Not Allowed",
			Status: http.StatusMethodNotAllowed,
		})
	})

	return r
}

// serveWeb sert l'application web embarquée.
//
// Repli sur index.html pour toute route inconnue : le web est une SPA, c'est
// son routeur client qui décide quoi afficher.
func serveWeb(d Deps, w http.ResponseWriter, r *http.Request) {
	if d.WebFS == nil {
		problem.Write(w, r, problem.NotFound(
			"the web application is not embedded in this build (run 'make build-web')"))
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	if f, err := d.WebFS.Open(path); err == nil {
		_ = f.Close()
		// Les assets buildés portent un hash dans leur nom : immuables.
		if strings.HasPrefix(path, "_next/") || strings.HasPrefix(path, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.FileServer(http.FS(d.WebFS)).ServeHTTP(w, r)
		return
	}

	// Route inconnue → index.html, sans cache pour que les mises à jour du
	// shell applicatif soient prises en compte immédiatement.
	index, err := fs.ReadFile(d.WebFS, "index.html")
	if err != nil {
		problem.Write(w, r, problem.NotFound("resource not found"))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(index)
}
