// Package pgtest fournit un PostgreSQL migré aux tests d'intégration.
//
// Même principe que miniotest : on réutilise le PostgreSQL de
// docker-compose.dev.yml quand BOXINCLOUD_TEST_DATABASE_URL est défini, sinon
// on démarre un conteneur via testcontainers.
//
// Chaque test obtient sa propre base, créée puis détruite : les tests peuvent
// s'exécuter en parallèle sans se marcher dessus, et un test qui échoue ne
// laisse pas de données parasites au suivant.
package pgtest

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/jobs"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/db"
)

// Start fournit un pool sur une base vide et migrée, nettoyée en fin de test.
func Start(t *testing.T) *pgxpool.Pool {
	t.Helper()

	if testing.Short() {
		t.Skip("test d'intégration ignoré en mode -short")
	}

	adminURL := shared(t)
	dbName := uniqueDBName(t)

	ctx := context.Background()

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("pgtest : connexion administrateur : %v", err)
	}
	// CREATE DATABASE n'accepte pas de paramètre lié ; le nom est dérivé du nom
	// du test et filtré caractère par caractère par uniqueDBName.
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("pgtest : création de la base %s : %v", dbName, err)
	}
	_ = admin.Close(ctx)

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		conn, err := pgx.Connect(cleanupCtx, adminURL)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close(cleanupCtx) }()
		_, _ = conn.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
	})

	pool, err := pgxpool.New(ctx, replaceDatabase(adminURL, dbName))
	if err != nil {
		t.Fatalf("pgtest : pool : %v", err)
	}
	t.Cleanup(pool.Close)

	// Les tests tournent sur le schéma réel, migrations comprises : une
	// migration cassée est détectée ici, pas en production.
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := db.Migrate(ctx, pool, log); err != nil {
		t.Fatalf("pgtest : migrations : %v", err)
	}

	// River tient son propre schéma, appliqué séparément au démarrage du
	// serveur. Sans lui, tout code qui enfile un job échoue en test alors qu'il
	// fonctionne en production — un écart qui fait douter du code plutôt que du
	// banc d'essai.
	if err := jobs.Migrate(ctx, pool, log); err != nil {
		t.Fatalf("pgtest : migrations River : %v", err)
	}

	return pool
}

// ─── PostgreSQL partagé par le paquet de test ────────────────────────────────

var (
	sharedOnce sync.Once
	sharedURL  string
	sharedErr  error
)

func shared(t *testing.T) string {
	t.Helper()

	sharedOnce.Do(func() {
		if url := os.Getenv("BOXINCLOUD_TEST_DATABASE_URL"); url != "" {
			sharedURL = url
			return
		}
		sharedURL, sharedErr = startContainer()
	})

	if sharedErr != nil {
		t.Skipf("pgtest : aucun PostgreSQL disponible (%v).\n"+
			"Lancez 'make dev-deps' puis exportez BOXINCLOUD_TEST_DATABASE_URL, "+
			"ou assurez-vous que Docker tourne pour testcontainers.", sharedErr)
	}
	return sharedURL
}

func startContainer() (string, error) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:17-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "boxincloud",
			"POSTGRES_PASSWORD": "boxincloud",
			"POSTGRES_DB":       "boxincloud",
		},
		// fsync désactivé : ces bases sont jetables, la durabilité n'a aucun
		// intérêt ici et coûte cher sur un test qui crée des dizaines de bases.
		Cmd: []string{"postgres", "-c", "fsync=off", "-c", "synchronous_commit=off"},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(90 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return "", fmt.Errorf("démarrage du conteneur : %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return "", err
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("postgres://boxincloud:boxincloud@%s:%s/boxincloud?sslmode=disable",
		host, port.Port()), nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// uniqueDBName dérive un nom de base valide du nom du test.
func uniqueDBName(t *testing.T) string {
	var b strings.Builder
	b.WriteString("test_")

	for _, r := range strings.ToLower(t.Name()) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}

	name := b.String()
	if len(name) > 40 {
		name = name[:40]
	}
	return fmt.Sprintf("%s_%d", name, time.Now().UnixNano()%1e9)
}

// replaceDatabase remplace le nom de base dans une URL de connexion.
func replaceDatabase(url, dbName string) string {
	base, query, hasQuery := strings.Cut(url, "?")

	idx := strings.LastIndex(base, "/")
	if idx < 0 {
		return url
	}
	out := base[:idx+1] + dbName

	if hasQuery {
		return out + "?" + query
	}
	return out
}
