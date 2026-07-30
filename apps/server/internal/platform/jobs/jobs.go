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
// Les workers métier (indexation, vignettes) seront ajoutés ici au fil des
// jalons. Register centralise l'enregistrement pour que le câblage reste
// visible en un seul endroit.
func New(pool *db.Pool, cfg config.Jobs, log *slog.Logger) (*Client, error) {
	workers := river.NewWorkers()
	Register(workers, log)

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

	return &Client{river: rc, log: log}, nil
}

// Register enregistre l'ensemble des workers connus.
func Register(workers *river.Workers, log *slog.Logger) {
	river.AddWorker(workers, &PingWorker{log: log})
	// M1 ajoutera ici : ScanLibraryWorker, IngestComicWorker,
	// BuildPageIndexWorker, GenerateCoverWorker.
}

// Start démarre les workers. Sans effet si les jobs sont désactivés.
func (c *Client) Start(ctx context.Context) error {
	if err := c.river.Start(ctx); err != nil {
		return fmt.Errorf("démarrage des workers : %w", err)
	}
	return nil
}

// Stop attend la fin des jobs en cours puis arrête les workers.
func (c *Client) Stop(ctx context.Context) error {
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
