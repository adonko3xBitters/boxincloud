// Commande boxincloudctl : administration en ligne de commande.
//
// Utile pour les opérations qui n'ont pas leur place dans l'API — migrations
// manuelles, diagnostic, tâches d'exploitation — et pour scripter une
// installation.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/adonko3xBitters/boxincloud/server/internal/config"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/db"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/jobs"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/logging"
)

var (
	version = "dev"
	commit  = "unknown"
)

const usage = `boxincloudctl — administration de boxincloud

Usage :
  boxincloudctl <commande> [arguments]

Commandes :
  migrate        Applique les migrations en attente (schéma applicatif + River)
  ping-db        Vérifie la connexion à PostgreSQL
  ping-job       Enfile un job de test et vérifie la file
  version        Affiche la version du binaire

Configuration : variables d'environnement, comme le serveur.
Voir .env.example.
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "\nboxincloudctl : %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}

	cmd := args[0]

	switch cmd {
	case "version", "-v", "--version":
		fmt.Printf("boxincloudctl %s (%s, %s)\n", version, commit, runtime.Version())
		return nil
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.LogLevel, cfg.LogFormat)
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.Database, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch cmd {
	case "migrate":
		if err := db.Migrate(ctx, pool, log); err != nil {
			return err
		}
		return jobs.Migrate(ctx, pool, log)

	case "ping-db":
		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("PostgreSQL injoignable : %w", err)
		}
		fmt.Println("PostgreSQL : ok")
		return nil

	case "ping-job":
		message := "ping"
		if len(args) > 1 {
			message = strings.Join(args[1:], " ")
		}

		// Client sans queue : on enfile sans exécuter. C'est le serveur qui
		// prendra le job, ce qui valide la chaîne complète.
		client, err := jobs.New(pool, config.Jobs{Enabled: false}, log)
		if err != nil {
			return err
		}
		if err := client.Insert(ctx, jobs.PingArgs{Message: message}); err != nil {
			return fmt.Errorf("insertion du job : %w", err)
		}
		fmt.Printf("Job 'ping' enfilé (%q).\n", message)
		fmt.Println("Vérifiez les logs du serveur : il doit apparaître sous une seconde.")
		return nil

	default:
		fmt.Print(usage)
		return errors.New("commande inconnue : " + cmd)
	}
}
