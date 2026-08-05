// Package jobs expose la file de jobs asynchrones du serveur.
//
// L'implémentation repose sur River, qui stocke ses jobs dans PostgreSQL.
// Deux conséquences importantes :
//
//   - aucun service supplémentaire à déployer — la stack reste PostgreSQL + un
//     binaire ;
//   - les jobs peuvent être enfilés dans la même transaction que l'écriture
//     métier qui les motive, ce qui rend impossible le job orphelin (enfilé
//     alors que la transaction a été annulée) ou perdu (transaction validée
//     mais job jamais enfilé).
//
// C'est cette seconde propriété qui compte le plus pour l'indexation : un comic
// inséré en base a nécessairement son job d'indexation, et réciproquement.
package jobs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/adonko3xBitters/boxincloud/server/internal/config"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/db"
)

// Client encapsule le client River de l'application.
type Client struct {
	river *river.Client[pgx.Tx]
	log   *slog.Logger

	// processing dit si ce client exécute des jobs, ou s'il se contente d'en
	// enfiler. Le distinguer est nécessaire parce que River refuse de démarrer
	// un client sans queue — voir Start.
	processing bool
}

// Migrate applique les migrations propres à River.
//
// Séparé des migrations applicatives : River gère son propre schéma et sa
// propre table de version.
func Migrate(ctx context.Context, pool *db.Pool, log *slog.Logger) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("initialisation du migrateur River : %w", err)
	}

	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("migrations River : %w", err)
	}

	if len(res.Versions) == 0 {
		log.Info("schéma River à jour")
	} else {
		log.Info("migrations River appliquées", slog.Int("count", len(res.Versions)))
	}
	return nil
}

// New construit le client de jobs et enregistre les workers.
//
// register reçoit le registre pour y déclarer les workers métier. Le câblage se
// fait ainsi dans cmd/, et le paquet jobs ne dépend d'aucun module métier — un
// worker d'indexation n'a pas à être connu de l'infrastructure qui l'exécute.
func New(pool *db.Pool, cfg config.Jobs, log *slog.Logger, register func(*river.Workers)) (*Client, error) {
	workers := river.NewWorkers()
	RegisterBuiltins(workers, log)
	if register != nil {
		register(workers)
	}

	riverCfg := &river.Config{
		Logger:  log,
		Workers: workers,
	}

	// Sans queue configurée, le client peut enfiler des jobs mais n'en exécute
	// aucun. Utile pour boxincloudctl, ou pour un déploiement où l'API et les
	// workers seraient séparés.
	if cfg.Enabled {
		riverCfg.Queues = map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: cfg.MaxWorkers},
		}
	}

	rc, err := river.NewClient(riverpgxv5.New(pool), riverCfg)
	if err != nil {
		return nil, fmt.Errorf("création du client River : %w", err)
	}

	return &Client{river: rc, log: log, processing: cfg.Enabled}, nil
}

// RegisterBuiltins enregistre les workers d'infrastructure, indépendants du
// métier.
func RegisterBuiltins(workers *river.Workers, log *slog.Logger) {
	river.AddWorker(workers, &PingWorker{log: log})
}

/*
Start démarre les workers. Sans effet si les jobs sont désactivés.

Ce garde manquait, et son absence rendait BOXINCLOUD_JOBS_ENABLED=false
INUTILISABLE : River refuse de démarrer un client sans queue configurée, si bien
que le serveur s'arrêtait au démarrage sur « Queues and Workers must be
configured for a client to start working ».

Le message accusait la configuration de River, que personne n'écrit à la main,
et non la variable d'environnement qui l'avait produite. Le mode était pourtant
documenté deux fonctions plus haut comme prévu et utile — pour boxincloudctl,
ou pour séparer l'API des workers.

C'est ici que le garde doit vivre, et pas chez l'appelant : seul ce paquet sait
qu'une queue est nécessaire au démarrage, et le lui faire savoir depuis app
diffuserait un détail de River dans le cycle de vie du serveur.
*/
func (c *Client) Start(ctx context.Context) error {
	if !c.processing {
		return nil
	}
	if err := c.river.Start(ctx); err != nil {
		return fmt.Errorf("démarrage des workers : %w", err)
	}
	return nil
}

// Stop attend la fin des jobs en cours puis arrête les workers.
//
// Symétrique de Start : arrêter ce qui n'a jamais démarré n'est pas une erreur,
// et le contraire obligerait chaque appelant à retenir s'il avait démarré.
func (c *Client) Stop(ctx context.Context) error {
	if !c.processing {
		return nil
	}
	return c.river.Stop(ctx)
}

// Insert enfile un job hors transaction.
//
// Pour l'enfiler dans une transaction métier — le cas à privilégier — utiliser
// InsertTx.
func (c *Client) Insert(ctx context.Context, args river.JobArgs) error {
	_, err := c.river.Insert(ctx, args, nil)
	return err
}

// InsertTx enfile un job dans une transaction existante.
//
// Le job n'existe que si la transaction est validée : c'est la garantie
// d'atomicité entre l'écriture métier et le traitement qu'elle déclenche.
func (c *Client) InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs) error {
	_, err := c.river.InsertTx(ctx, tx, args, nil)
	return err
}
