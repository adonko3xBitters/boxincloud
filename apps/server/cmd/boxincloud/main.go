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

/*
Deux sous-commandes, et c'est assez.

`serve` est le défaut : l'invoquer sans argument doit démarrer le serveur,
parce que c'est ce que fait ce binaire quatre-vingt-dix-neuf fois sur cent.

`version` existe pour une raison précise : elle doit répondre SANS
configuration. C'est ce qui permet de vérifier qu'une image publiée démarre —
un contrôle qui, sans elle, exigerait une base de données rien que pour savoir
si le binaire s'exécute.
*/
func main() {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "version", "--version", "-v":
		fmt.Printf("boxincloud %s (%s, %s)\n", version, commit, runtime.Version())
		return

	case "serve":
		if err := run(); err != nil {
			fmt.Fprintf(os.Stderr, "\nboxincloud : %v\n", err)
			os.Exit(1)
		}

	case "help", "--help", "-h":
		usage(os.Stdout)

	default:
		fmt.Fprintf(os.Stderr, "boxincloud : commande inconnue « %s »\n\n", command)
		usage(os.Stderr)
		os.Exit(2)
	}
}

func usage(w *os.File) {
	// Le retour est ignoré délibérément : si écrire l'aide sur stdout ou stderr
	// échoue, il n'existe plus aucun canal pour le signaler.
	_, _ = fmt.Fprint(w, `boxincloud — serveur de bibliothèque de bandes dessinées

Usage :
  boxincloud [commande]

Commandes :
  serve      Démarre le serveur (défaut)
  version    Affiche la version, sans lire la configuration
  help       Affiche ce message

La configuration passe par l'environnement. Voir .env.example.
`)
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
