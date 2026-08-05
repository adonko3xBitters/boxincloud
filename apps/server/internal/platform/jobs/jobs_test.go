package jobs_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/adonko3xBitters/boxincloud/server/internal/config"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/jobs"
	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/pgtest"
)

/*
TestIntegrationClientSansQueueDemarre couvre le défaut que ce paquet a porté
jusqu'ici.

BOXINCLOUD_JOBS_ENABLED=false est documenté comme un mode prévu : enfiler sans
exécuter, pour le CLI ou pour un déploiement où l'API et les workers seraient
séparés. Il était en réalité INUTILISABLE — River refuse de démarrer un client
sans queue, et le serveur s'arrêtait au démarrage sur un message qui accusait la
configuration de River plutôt que la variable qui l'avait produite.

Le test démarre ET arrête, parce que le défaut symétrique existe aussi : un Stop
sur un client jamais démarré n'a pas à échouer.
*/
func TestIntegrationClientSansQueueDemarre(t *testing.T) {
	pool := pgtest.Start(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	if err := jobs.Migrate(ctx, pool, log); err != nil {
		t.Fatalf("migrations River : %v", err)
	}

	client, err := jobs.New(pool, config.Jobs{Enabled: false}, log, nil)
	if err != nil {
		t.Fatalf("New : %v", err)
	}

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start avec les jobs désactivés : %v\n"+
			"Ce mode est documenté comme prévu ; il doit démarrer sans rien exécuter.", err)
	}
	if err := client.Stop(ctx); err != nil {
		t.Errorf("Stop d'un client jamais démarré : %v", err)
	}
}

// TestIntegrationClientAvecQueueDemarre est le contrepoids.
//
// Un garde posé trop large empêcherait les workers de démarrer pour de bon, et
// le test précédent passerait quand même — brillamment.
func TestIntegrationClientAvecQueueDemarre(t *testing.T) {
	pool := pgtest.Start(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	if err := jobs.Migrate(ctx, pool, log); err != nil {
		t.Fatalf("migrations River : %v", err)
	}

	client, err := jobs.New(pool, config.Jobs{Enabled: true, MaxWorkers: 2}, log, nil)
	if err != nil {
		t.Fatalf("New : %v", err)
	}

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start avec les jobs activés : %v", err)
	}
	t.Cleanup(func() { _ = client.Stop(context.Background()) })

	// Un job enfilé doit être accepté : c'est ce qui distingue un client qui
	// tourne d'un client qui a seulement dit oui à Start.
	if err := client.Insert(ctx, jobs.PingArgs{Message: "essai"}); err != nil {
		t.Errorf("Insert : %v", err)
	}
}
