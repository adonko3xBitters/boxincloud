package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/adonko3xBitters/boxincloud/server/migrations"
)

// Migrate applique les migrations en attente.
//
// Les migrations sont embarquées dans le binaire : déployer une nouvelle
// version suffit, il n'y a pas d'étape manuelle ni de fichier à copier.
func Migrate(ctx context.Context, pool *Pool, log *slog.Logger) error {
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(gooseLogger{log})

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("dialecte goose : %w", err)
	}

	// goose travaille sur database/sql ; on emprunte le pool pgx existant
	// plutôt que d'ouvrir une seconde connexion.
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()

	before, err := goose.GetDBVersionContext(ctx, sqlDB)
	if err != nil {
		return fmt.Errorf("lecture de la version du schéma : %w", err)
	}

	if err := goose.UpContext(ctx, sqlDB, "."); err != nil {
		return fmt.Errorf("application des migrations : %w", err)
	}

	after, err := goose.GetDBVersionContext(ctx, sqlDB)
	if err != nil {
		return fmt.Errorf("relecture de la version du schéma : %w", err)
	}

	if before == after {
		log.Info("schéma à jour", slog.Int64("version", after))
	} else {
		log.Info("migrations appliquées",
			slog.Int64("de", before),
			slog.Int64("à", after),
		)
	}
	return nil
}

// gooseLogger branche la sortie de goose sur slog.
type gooseLogger struct{ log *slog.Logger }

func (g gooseLogger) Printf(format string, v ...any) {
	g.log.Debug(fmt.Sprintf(format, v...))
}

func (g gooseLogger) Fatalf(format string, v ...any) {
	g.log.Error(fmt.Sprintf(format, v...))
}
