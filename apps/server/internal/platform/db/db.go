// Package db gère la connexion à PostgreSQL et l'application des migrations.
package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/adonko3xBitters/boxincloud/server/internal/config"
)

// Pool est le pool de connexions PostgreSQL de l'application.
type Pool = pgxpool.Pool

// Connect ouvre le pool de connexions et vérifie que la base répond.
func Connect(ctx context.Context, cfg config.Database, log *slog.Logger) (*Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL invalide : %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("création du pool : %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("PostgreSQL injoignable : %w", err)
	}

	log.Info("PostgreSQL connecté",
		slog.String("host", poolCfg.ConnConfig.Host),
		slog.String("database", poolCfg.ConnConfig.Database),
		slog.Int("max_conns", int(cfg.MaxConns)),
	)
	return pool, nil
}
