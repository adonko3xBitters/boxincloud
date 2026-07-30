package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
)

// PingArgs est un job sans effet, dont le seul rôle est de vérifier que la
// chaîne complète fonctionne : insertion en base, prise en charge par un
// worker, exécution, marquage comme terminé.
//
// `boxincloudctl ping-job` l'enfile. C'est le test de fumée du système de jobs,
// et il reste utile après M0 pour diagnostiquer une instance en production.
type PingArgs struct {
	Message string `json:"message"`
}

func (PingArgs) Kind() string { return "ping" }

type PingWorker struct {
	river.WorkerDefaults[PingArgs]
	log *slog.Logger
}

func (w *PingWorker) Work(ctx context.Context, job *river.Job[PingArgs]) error {
	w.log.Info("job ping exécuté",
		slog.Int64("job_id", job.ID),
		slog.String("message", job.Args.Message),
		slog.Duration("latence", time.Since(job.CreatedAt)),
	)
	return nil
}
