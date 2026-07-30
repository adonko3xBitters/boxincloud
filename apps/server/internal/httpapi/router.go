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
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/adonko3xBitters/boxincloud/server/internal/accounts"
	"github.com/adonko3xBitters/boxincloud/server/internal/auth"
	"github.com/adonko3xBitters/boxincloud/server/internal/catalog"
	"github.com/adonko3xBitters/boxincloud/server/internal/config"
	"github.com/adonko3xBitters/boxincloud/server/internal/folders"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/handlers"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/middleware"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
	"github.com/adonko3xBitters/boxincloud/server/internal/ingest"
	"github.com/adonko3xBitters/boxincloud/server/internal/library"
	"github.com/adonko3xBitters/boxincloud/server/internal/progress"
	"github.com/adonko3xBitters/boxincloud/server/internal/reader"
)

// Deps rassemble ce dont le routeur a besoin.
//
// Le câblage se fait dans cmd/ : le routeur ne construit rien lui-même, ce qui
// le rend testable avec des doublures.
type Deps struct {
	Config    *config.Config
	Log       *slog.Logger
	DB        handlers.Pinger
	Build     handlers.BuildInfo
	Auth      *auth.Service
	Accounts  *accounts.Service
	Catalog   *catalog.Service
	Folders   *folders.Service
	Libraries *library.Service
	Ingest    *ingest.Service
	Reader    *reader.Service
	Progress  *progress.Service
	Tools     *catalog.Tools
	WebFS     fs.FS // application web embarquée ; nil pour ne rien servir
}

// requestTimeout borne une requête d'API ordinaire.
//
// Une lecture de page peut être longue sur un backend distant, mais aucune
// requête ne doit rester pendante indéfiniment.
const requestTimeout = 30 * time.Second

// relocateTimeout borne le renommage d'une branche.
//
// Dix minutes : de quoi déplacer une branche de plusieurs centaines d'albums sur
// un backend distant, où chaque objet coûte un aller-retour. Le service refuse
// de lui-même au-delà de deux mille albums.
const relocateTimeout = 10 * time.Minute

// uploadTimeout borne un téléversement.
//
// Deux heures : de quoi laisser passer plusieurs giga-octets sur une liaison
// domestique modeste, sans pour autant qu'une connexion oubliée occupe un
// descripteur jusqu'au redémarrage.
const uploadTimeout = 2 * time.Hour

// NewRouter assemble le routeur complet.
//
// Une dépendance oubliée fait échouer la construction, pas la requête : sans ce
// contrôle, un service non câblé se manifeste par des 500 opaques sur les
// routes concernées, et rien ne dit lequel manque.
func NewRouter(d Deps) http.Handler {
	for name, wired := range map[string]bool{
		"Config":    d.Config != nil,
		"Log":       d.Log != nil,
		"DB":        d.DB != nil,
		"Auth":      d.Auth != nil,
		"Accounts":  d.Accounts != nil,
		"Catalog":   d.Catalog != nil,
		"Folders":   d.Folders != nil,
		"Libraries": d.Libraries != nil,
		"Ingest":    d.Ingest != nil,
		"Tools":     d.Tools != nil,
		"Reader":    d.Reader != nil,
		"Progress":  d.Progress != nil,
	} {
		if !wired {
			panic("httpapi : dépendance " + name + " non câblée dans NewRouter")
		}
	}

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
		r.Get("/version", health.Version)

		// ── Routes publiques ────────────────────────────────────────────
		r.Route("/auth", func(r chi.Router) {
			r.Use(chimw.Timeout(requestTimeout))

			r.Get("/status", authHandler.Status)
			r.Post("/setup", authHandler.Setup)
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.Refresh)
			r.Post("/logout", authHandler.Logout)
		})

		// ── Routes authentifiées ────────────────────────────────────────
		r.Group(func(r chi.Router) {
			r.Use(chimw.Timeout(requestTimeout))
			r.Use(middleware.Authenticate(d.Auth))

			r.Get("/me", authHandler.Me)
			r.Get("/me/devices", authHandler.ListDevices)
			r.Post("/me/logout-all", authHandler.LogoutAll)

			catalogHandler := handlers.NewCatalog(d.Catalog)
			readerHandler := handlers.NewReader(d.Reader, d.Catalog)
			progressHandler := handlers.NewProgress(d.Progress, d.Catalog)

			r.Get("/home", catalogHandler.Home)
			r.Get("/search", catalogHandler.Search)
			r.Get("/libraries", catalogHandler.ListLibraries)

			r.Get("/series", catalogHandler.ListSeries)
			r.Get("/series/{seriesID}", catalogHandler.GetSeries)

			r.Route("/comics", func(r chi.Router) {
				r.Get("/", catalogHandler.ListComics)

				r.Route("/{comicID}", func(r chi.Router) {
					r.Get("/", catalogHandler.GetComic)
					r.Get("/manifest", readerHandler.Manifest)

					r.Get("/progress", progressHandler.Get)
					r.Put("/progress", progressHandler.Update)
					r.Delete("/progress", progressHandler.Delete)
				})
			})

			r.Get("/continue-reading", progressHandler.ContinueReading)
			r.Get("/sync", progressHandler.SyncPull)

			// ── Outils de gestion ───────────────────────────────────
			toolsHandler := handlers.NewTools(d.Tools)

			foldersHandler := handlers.NewFolders(d.Folders, d.Catalog)

			r.Get("/folders", foldersHandler.List)
			r.Post("/folders", foldersHandler.Create)
			r.Put("/folders/lock", foldersHandler.SetLock)
			r.Post("/folders/unlock", foldersHandler.Unlock)
			r.Delete("/libraries/{libraryID}/folders/unlock", foldersHandler.Relock)

			// ── Partage ─────────────────────────────────────────────
			sharingHandler := handlers.NewSharing(d.Folders, d.Catalog, readerHandler)

			r.Get("/libraries/{libraryID}/folders/access", sharingHandler.ListFolderGrants)
			r.Post("/folders/access", sharingHandler.GrantFolder)
			r.Delete("/libraries/{libraryID}/folders/access/{userID}", sharingHandler.RevokeFolderGrant)

			r.Get("/share-links", sharingHandler.ListShares)
			r.Post("/share-links", sharingHandler.CreateShare)
			r.Delete("/share-links/{shareID}", sharingHandler.RevokeShare)
			r.Get("/me/marks", toolsHandler.UserMarks)
			r.Post("/comics/bulk", toolsHandler.Bulk)
			r.Put("/comics/{comicID}/favorite", toolsHandler.SetFavorite)
			r.Put("/comics/{comicID}/rating", toolsHandler.SetRating)
			r.Patch("/comics/{comicID}", toolsHandler.EditComic)

			// ── Administration et ingestion ─────────────────────────
			//
			// Ce qui n'existait qu'en ligne de commande : déclarer un
			// backend, créer une bibliothèque, y déposer un fichier,
			// relancer un parcours. Sans ces routes, remplir boxincloud
			// demandait un accès shell au serveur.
			adminHandler := handlers.NewAdmin(d.Libraries, d.Catalog, d.Ingest)

			r.Get("/storage-backends", adminHandler.ListBackends)
			r.Post("/storage-backends", adminHandler.CreateBackend)
			r.Post("/storage-backends/{backendID}/test", adminHandler.TestBackend)

			r.Post("/libraries", adminHandler.CreateLibrary)
			r.Post("/libraries/{libraryID}/scan", adminHandler.Scan)

			r.Delete("/comics/{comicID}", adminHandler.DeleteComic)
			r.Delete("/libraries/{libraryID}/folders", foldersHandler.Delete)
			r.Put("/comics/{comicID}/folder", adminHandler.MoveComic)
			r.Post("/comics/manage", adminHandler.BulkManage)

			// ── Comptes ─────────────────────────────────────────────
			accountsHandler := handlers.NewAccounts(d.Accounts)

			r.Route("/accounts", func(r chi.Router) {
				r.Get("/", accountsHandler.List)
				r.Post("/", accountsHandler.Create)
				r.Patch("/{userID}", accountsHandler.Update)
				r.Delete("/{userID}", accountsHandler.Delete)
				r.Get("/{userID}/library-access", accountsHandler.ListGrants)
			})

			r.Get("/libraries/{libraryID}/access", accountsHandler.ListLibraryGrants)
			r.Post("/libraries/{libraryID}/access", accountsHandler.Grant)
			r.Delete("/libraries/{libraryID}/access/{userID}", accountsHandler.Revoke)
		})

		// ── Opérations longues sur l'arborescence ───────────────────────
		//
		// Renommer un dossier renomme chacun des objets qu'il contient : sur un
		// backend distant, une branche de plusieurs centaines d'albums dépasse
		// largement le délai des requêtes ordinaires.
		r.Group(func(r chi.Router) {
			r.Use(chimw.Timeout(relocateTimeout))
			r.Use(middleware.Authenticate(d.Auth))

			foldersHandler := handlers.NewFolders(d.Folders, d.Catalog)
			r.Put("/folders/path", foldersHandler.Relocate)
		})

		// ── Téléversement ───────────────────────────────────────────────
		//
		// Isolé parce qu'il ne peut pas partager le délai des autres routes :
		// une intégrale de plusieurs centaines de méga-octets sur une liaison
		// domestique dépasse largement trente secondes, et la couper à
		// mi-parcours laisserait un objet tronqué dans le bucket.
		//
		// L'absence de délai n'est pas une absence de garde-fou : la taille est
		// bornée à la lecture, et une connexion muette est fermée par
		// IdleTimeout côté serveur.
		r.Group(func(r chi.Router) {
			r.Use(chimw.Timeout(uploadTimeout))
			r.Use(middleware.Authenticate(d.Auth))

			adminHandler := handlers.NewAdmin(d.Libraries, d.Catalog, d.Ingest)
			r.Post("/libraries/{libraryID}/upload", adminHandler.Upload)
		})

		/*
			── Accès public par lien de partage ────────────────────────────

			Les SEULES routes du serveur qui ne demandent aucun compte.

			Elles sont volontairement regroupées et nommées : la surface non
			authentifiée doit se lire d'un seul tenant plutôt que se découvrir
			en parcourant le routeur.

			Leur portée est celle du lien, revérifiée à chaque requête. Aucune
			ne prend d'identifiant de bibliothèque, aucune ne liste autre chose
			que ce que le lien désigne, et toutes sont en lecture seule.
		*/
		r.Group(func(r chi.Router) {
			r.Use(chimw.Timeout(requestTimeout))

			readerHandler := handlers.NewReader(d.Reader, d.Catalog)
			sharingHandler := handlers.NewSharing(d.Folders, d.Catalog, readerHandler)

			r.Route("/share/{token}", func(r chi.Router) {
				r.Get("/", sharingHandler.GetShared)
				r.Get("/comics/{comicID}/manifest", sharingHandler.SharedManifest)
				r.Get("/comics/{comicID}/pages/{index}", sharingHandler.SharedPage)
				r.Get("/comics/{comicID}/cover", sharingHandler.SharedCover)
			})
		})

		// ── Routes acceptant le jeton en paramètre d'URL ────────────────
		//
		// Une balise <img> ne peut pas porter d'en-tête Authorization, et
		// sendBeacon — seul mécanisme qu'un navigateur exécute de façon fiable
		// à la fermeture d'un onglet — non plus.
		//
		// Ces routes sont isolées : un jeton dans une URL fuit par l'en-tête
		// Referer, les journaux de proxy et l'historique du navigateur. On
		// l'accepte là où il n'y a pas d'alternative, nulle part ailleurs.
		r.Group(func(r chi.Router) {
			r.Use(chimw.Timeout(requestTimeout))
			r.Use(middleware.AuthenticateAllowingQueryToken(d.Auth))

			readerHandler := handlers.NewReader(d.Reader, d.Catalog)
			progressHandler := handlers.NewProgress(d.Progress, d.Catalog)

			r.Get("/comics/{comicID}/cover", readerHandler.Cover)
			r.Get("/comics/{comicID}/pages/{index}", readerHandler.Page)
			r.Post("/sync", progressHandler.SyncPush)
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
// Trois résolutions successives, dans cet ordre :
//
//  1. le fichier tel quel — les assets `_next/…` ;
//  2. le fichier suffixé de `.html` — l'export statique de Next produit
//     `library.html` pour la route `/library`, et sans cette étape un
//     rafraîchissement sur une route interne servirait le mauvais document ;
//  3. index.html — le routeur client décide alors quoi afficher.
func serveWeb(d Deps, w http.ResponseWriter, r *http.Request) {
	if d.WebFS == nil {
		problem.Write(w, r, problem.NotFound(
			"the web application is not embedded in this build (run 'make build-web')"))
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" || name == "index.html" {
		serveHTML(d.WebFS, "index.html", w, r)
		return
	}

	if exists(d.WebFS, name) {
		// Les assets buildés portent un hash dans leur nom : immuables.
		if strings.HasPrefix(name, "_next/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.FileServer(http.FS(d.WebFS)).ServeHTTP(w, r)
		return
	}

	if candidate := name + ".html"; exists(d.WebFS, candidate) {
		serveHTML(d.WebFS, candidate, w, r)
		return
	}

	// Un chemin portant une extension désigne un fichier, pas une route de la
	// SPA. Lui servir index.html renverrait du HTML avec un code 200 à une
	// balise <img> ou <script> — ce qui masque l'absence réelle du fichier et
	// rend le diagnostic pénible. On répond 404, comme il se doit.
	if path.Ext(name) != "" {
		problem.Write(w, r, problem.NotFound("resource not found"))
		return
	}

	serveHTML(d.WebFS, "index.html", w, r)
}

/*
exists indique si un chemin désigne un FICHIER du bundle embarqué.

Les répertoires sont exclus délibérément. L'export statique de Next produit à la
fois `partage.html` et un répertoire `partage/` ; laisser passer le second ferait
rediriger `/partage` vers `/partage/` par le serveur de fichiers, alors que le
document attendu est le premier. La redirection fonctionnait, mais elle salissait
une adresse faite pour être copiée et transmise.

fs.ValidPath est vérifié explicitement : embed.FS rejetterait déjà un chemin
contenant « .. » ou une barre de tête, mais s'en remettre à ce comportement
laisserait la garantie implicite. Ici le contrat est écrit — seul un chemin
relatif propre peut atteindre le système de fichiers.
*/
func exists(fsys fs.FS, name string) bool {
	if !fs.ValidPath(name) {
		return false
	}
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	return err == nil && !info.IsDir()
}

// serveHTML sert un document, sans cache.
//
// Le HTML référence des assets dont le nom change à chaque build : le mettre
// en cache ferait charger d'anciens scripts après une mise à jour du serveur.
func serveHTML(fsys fs.FS, name string, w http.ResponseWriter, r *http.Request) {
	// Le nom vient du chemin de l'URL : on ne lit que ce qui est effectivement
	// embarqué dans le binaire. Un chemin absent, ou sortant du bundle, ne peut
	// donc pas atteindre le disque — le contenu servi est celui que nous avons
	// nous-mêmes compilé, jamais une donnée fournie par le client.
	if !exists(fsys, name) {
		problem.Write(w, r, problem.NotFound(
			"the web application is not embedded in this build (run 'make build-web')"))
		return
	}

	body, err := fs.ReadFile(fsys, name)
	if err != nil {
		problem.Write(w, r, problem.NotFound(
			"the web application is not embedded in this build (run 'make build-web')"))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	// #nosec G705 -- l'analyse de flux voit un chemin issu de l'URL et conclut
	// à une injection. Elle ne peut pas suivre la garantie : `body` provient
	// d'un embed.FS compilé dans le binaire, et `exists` a déjà écarté tout
	// chemin invalide. Les octets servis sont ceux que nous avons produits au
	// build, jamais une donnée fournie par le client.
	_, _ = w.Write(body)
}
