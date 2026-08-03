// Package app assemble et pilote le cycle de vie du serveur.
//
// Le câblage des dépendances est centralisé ici : c'est le seul endroit du
// projet où les modules se connaissent. Ils ne s'importent jamais entre eux —
// voir CONTRIBUTING.md, règle 2.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime"
	"syscall"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/config"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/handlers"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/db"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/jobs"
	"github.com/adonko3xBitters/boxincloud/server/web"
)

// App regroupe les composants du serveur et leur cycle de vie.
type App struct {
	cfg  *config.Config
	log  *slog.Logger
	pool *db.Pool
	core *Core
	http *http.Server
}

// Build construit l'application : connexion à la base, migrations, jobs,
// routeur HTTP. Ne démarre rien — c'est le rôle de Run.
func Build(ctx context.Context, cfg *config.Config, log *slog.Logger, build handlers.BuildInfo) (*App, error) {
	pool, err := db.Connect(ctx, cfg.Database, log)
	if err != nil {
		return nil, err
	}

	closeOnError := func(err error) (*App, error) {
		pool.Close()
		return nil, err
	}

	if cfg.Database.AutoMigrate {
		if err := db.Migrate(ctx, pool, log); err != nil {
			return closeOnError(err)
		}
		if err := jobs.Migrate(ctx, pool, log); err != nil {
			return closeOnError(err)
		}
	}

	core, err := BuildCore(ctx, cfg, pool, log)
	if err != nil {
		return closeOnError(err)
	}

	webFS, err := web.FS()
	if err != nil {
		log.Warn("application web non embarquée", slog.Any("err", err))
		webFS = nil
	}

	router := httpapi.NewRouter(httpapi.Deps{
		Config:    cfg,
		Log:       log,
		DB:        pool,
		Build:     build,
		Auth:      core.Auth,
		Accounts:  core.Accounts,
		Catalog:   core.Catalog,
		Folders:   core.Folders,
		Libraries: core.Libraries,
		Ingest:    core.Ingest,
		Reader:    core.Reader,
		Progress:  core.Progress,
		Tools:     core.Tools,
		Cache:     core.Cache,
		WebFS:     webFS,
	})

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: router,
		// Pas de WriteTimeout : servir une page depuis un backend distant
		// peut légitimement être long. Le contrôle du temps se fait par
		// middleware sur /api, et par le contexte de la requête.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	return &App{cfg: cfg, log: log, pool: pool, core: core, http: srv}, nil
}

// Run démarre le serveur et bloque jusqu'à l'annulation du contexte, puis
// procède à un arrêt propre.
func (a *App) Run(ctx context.Context) error {
	if err := a.core.Jobs.Start(ctx); err != nil {
		return err
	}

	if a.cfg.Jobs.Enabled {
		a.log.Info("workers démarrés", slog.Int("max_workers", a.cfg.Jobs.MaxWorkers))
	} else {
		a.log.Info("workers désactivés (BOXINCLOUD_JOBS_ENABLED=false)")
	}

	errCh := make(chan error, 1)
	go func() {
		a.log.Info("serveur à l'écoute",
			slog.String("addr", a.cfg.Addr),
			slog.String("env", string(a.cfg.Env)),
			slog.String("go", runtime.Version()),
		)
		if err := a.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- listenFailure(a.cfg.Addr, err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		a.log.Info("arrêt demandé, fin des requêtes en cours…")
		return a.shutdown()
	}
}

// shutdown ferme les composants dans l'ordre inverse de leur ouverture.
func (a *App) shutdown() error {
	// Contexte propre : celui de Run est déjà annulé.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var errs []error

	if err := a.http.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("arrêt du serveur HTTP : %w", err))
	}

	// Après le serveur HTTP : un job en cours peut avoir été déclenché par une
	// requête qui vient tout juste de se terminer.
	if err := a.core.Jobs.Stop(ctx); err != nil {
		errs = append(errs, fmt.Errorf("arrêt des workers : %w", err))
	}

	a.pool.Close()
	a.log.Info("arrêt terminé")

	return errors.Join(errs...)
}

/*
listenFailure explique un démarrage impossible, au lieu de le constater.

« listen tcp :8070: bind: address already in use » est exact et inutile : il ne
dit ni qui occupe le port, ni comment le savoir, ni qu'on peut simplement en
changer. C'est le premier message que rencontre quelqu'un qui relance une
instance sans avoir arrêté la précédente — le cas le plus banal du
développement, et celui qui coûte le plus de minutes à qui découvre le projet.

Le port est extrait de l'adresse écoutée plutôt que réécrit en dur : `Addr` peut
valoir « :8080 », « 127.0.0.1:8080 » ou « [::1]:8080 », et une commande qui ne
correspond pas à la configuration réelle vaut moins que pas de commande.
*/
func listenFailure(addr string, err error) error {
	if !errors.Is(err, syscall.EADDRINUSE) {
		return fmt.Errorf("serveur HTTP : %w", err)
	}

	port := addr
	if _, p, splitErr := net.SplitHostPort(addr); splitErr == nil && p != "" {
		port = p
	}

	return fmt.Errorf(`le port %s est déjà occupé.

Une autre instance tourne probablement — c'est le cas le plus fréquent après un
redémarrage. Pour voir laquelle :

    lsof -nP -iTCP:%s -sTCP:LISTEN

Puis arrêtez-la, ou écoutez ailleurs :

    BOXINCLOUD_ADDR=:8081

(%w)`, port, port, err)
}
