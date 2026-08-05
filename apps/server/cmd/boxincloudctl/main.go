// Commande boxincloudctl : administration en ligne de commande.
//
// Utile pour les opérations qui n'ont pas leur place dans l'API — diagnostic,
// exploitation — et pour scripter une installation. En M1, c'est aussi
// l'interface qui rend le pipeline démontrable avant qu'il existe une UI.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/adonko3xBitters/boxincloud/server/internal/app"
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

Diagnostic
  ping-db                       Vérifie la connexion à PostgreSQL
  ping-job [message]            Enfile un job de test et vérifie la file
  version                       Affiche la version du binaire

Schéma
  migrate                       Applique les migrations en attente

Comptes
  user list                     Liste les comptes et leur rôle
  user set-password <compte>    Change un mot de passe et révoque les sessions.
                                Le mot de passe est lu sur l'entrée standard,
                                jamais passé en argument.

Stockage
  storage add <nom> <type> [clé=valeur ...]
                                Enregistre un backend après l'avoir testé.
                                Types : s3, local
                                s3    : endpoint= bucket= access_key= secret_key=
                                        [region=] [use_ssl=false] [path_style=true]
                                local : root=
  storage list                  Liste les backends enregistrés
  storage test <nom>            Vérifie qu'un backend répond

Bibliothèques
  library add <nom> <backend> [préfixe]
                                Crée une bibliothèque sur un backend
  library list                  Liste les bibliothèques
  scan <bibliothèque>           Enfile un scan (le serveur doit tourner)
  scan-now <bibliothèque>       Scanne immédiatement, sans passer par la file

Lecture
  page <bibliothèque> <clé> <n> [fichier]
                                Extrait la page n d'une archive et compte les
                                requêtes Range effectuées

eD2k / Kad
  ed2k ping                     Joint le démon aMule déclaré, s'authentifie et
                                mesure l'aller-retour. Dit lequel des quatre
                                points a lâché : adresse, mot de passe, version
                                du protocole ou réseau.

Configuration : variables d'environnement, comme le serveur. Voir .env.example.
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

	switch args[0] {
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

	// Le CLI n'exécute pas de workers : il enfile, le serveur traite. Sauf
	// scan-now, qui exécute en direct pour rendre le pipeline observable sans
	// serveur.
	cfg.Jobs.Enabled = false

	core, err := app.BuildCore(ctx, cfg, pool, log)
	if err != nil {
		return err
	}

	// Le démon aMule journalise le nom et la version de qui s'y connecte :
	// autant qu'il distingue le CLI d'une instance de serveur.
	core.Ed2k.SetVersion(version)

	cmd := &commands{core: core, pool: pool, cfg: cfg, log: log}

	switch args[0] {
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
		return cmd.pingJob(ctx, args[1:])

	case "storage":
		return cmd.storage(ctx, args[1:])

	case "library":
		return cmd.library(ctx, args[1:])

	case "scan":
		return cmd.scan(ctx, args[1:])

	case "scan-now":
		return cmd.scanNow(ctx, args[1:])

	case "page":
		return cmd.page(ctx, args[1:])

	case "user":
		return cmd.user(ctx, args[1:])

	case "ed2k":
		return cmd.ed2k(ctx, args[1:])

	default:
		fmt.Print(usage)
		return errors.New("commande inconnue : " + args[0])
	}
}
