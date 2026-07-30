// Commande boxincloud : le serveur.
//
// Un binaire unique qui sert l'API, l'application web embarquée et exécute les
// workers d'indexation. Les workers peuvent être désactivés
// (BOXINCLOUD_JOBS_ENABLED=false) pour séparer API et traitement dans un
// déploiement plus large, mais l'installation par défaut n'exige qu'un process.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/adonko3xBitters/boxincloud/server/internal/app"
	"github.com/adonko3xBitters/boxincloud/server/internal/config"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/handlers"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/logging"
)

// Injectés à la compilation via -ldflags. Voir le Makefile.
var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\nboxincloud : %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logging.New(cfg.LogLevel, cfg.LogFormat)

	build := handlers.BuildInfo{
		Version:   version,
		Commit:    commit,
		GoVersion: runtime.Version(),
	}
	log.Info("boxincloud démarre",
		"version", build.Version,
		"commit", build.Commit,
	)

	// Annulé sur SIGINT/SIGTERM, ce qui déclenche l'arrêt propre.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.Build(ctx, cfg, log, build)
	if err != nil {
		return err
	}

	return a.Run(ctx)
}
