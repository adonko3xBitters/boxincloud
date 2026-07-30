// Package miniotest fournit un MinIO jetable aux tests d'intégration.
//
// Deux modes, dans cet ordre de préférence :
//
//  1. **MinIO déjà lancé** — si BOXINCLOUD_TEST_MINIO_ENDPOINT est défini, on
//     l'utilise. C'est le mode confortable en développement local : le MinIO de
//     docker-compose.dev.yml tourne déjà, les tests démarrent instantanément.
//
//  2. **testcontainers** — sinon, un conteneur MinIO est démarré pour la durée
//     du paquet de test. C'est le mode de la CI, et le repli si aucun MinIO
//     n'est disponible.
//
// Dans les deux cas, chaque test obtient son propre bucket : ils ne se marchent
// pas dessus et peuvent tourner en parallèle.
package miniotest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PresignTTL est une durée de validité commode pour les tests d'URL présignées.
const PresignTTL = 5 * time.Minute

// Env décrit un MinIO utilisable et un bucket vide qui lui appartient.
type Env struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
}

// Start fournit un MinIO et un bucket vide, nettoyé en fin de test.
//
// Marque le test comme long : il est ignoré sous `go test -short`.
func Start(t *testing.T) Env {
	t.Helper()

	if testing.Short() {
		t.Skip("test d'intégration ignoré en mode -short")
	}

	base := shared(t)
	bucket := uniqueBucketName(t)

	client, err := minio.New(base.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(base.AccessKey, base.SecretKey, ""),
		Secure:       false,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		t.Fatalf("miniotest : client : %v", err)
	}

	ctx := context.Background()
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatalf("miniotest : création du bucket %s : %v", bucket, err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		removeBucket(cleanupCtx, client, bucket)
	})

	return Env{
		Endpoint:  base.Endpoint,
		AccessKey: base.AccessKey,
		SecretKey: base.SecretKey,
		Bucket:    bucket,
	}
}

// ─── MinIO partagé par le paquet de test ─────────────────────────────────────

var (
	sharedOnce sync.Once
	sharedEnv  Env
	sharedErr  error
)

func shared(t *testing.T) Env {
	t.Helper()

	sharedOnce.Do(func() {
		// Mode 1 : un MinIO déjà lancé.
		if endpoint := os.Getenv("BOXINCLOUD_TEST_MINIO_ENDPOINT"); endpoint != "" {
			sharedEnv = Env{
				Endpoint:  strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://"),
				AccessKey: envOr("BOXINCLOUD_TEST_MINIO_ACCESS_KEY", "boxincloud"),
				SecretKey: envOr("BOXINCLOUD_TEST_MINIO_SECRET_KEY", "boxincloud"),
			}
			return
		}
		// Mode 2 : testcontainers.
		sharedEnv, sharedErr = startContainer()
	})

	if sharedErr != nil {
		t.Skipf("miniotest : aucun MinIO disponible (%v).\n"+
			"Lancez 'make dev-deps' puis exportez BOXINCLOUD_TEST_MINIO_ENDPOINT, "+
			"ou assurez-vous que Docker tourne pour testcontainers.", sharedErr)
	}
	return sharedEnv
}

func startContainer() (Env, error) {
	ctx := context.Background()

	const (
		user = "boxincloud"
		pass = "boxincloud123"
	)

	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:latest",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     user,
			"MINIO_ROOT_PASSWORD": pass,
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").WithStartupTimeout(90 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return Env{}, fmt.Errorf("démarrage du conteneur : %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return Env{}, err
	}
	port, err := container.MappedPort(ctx, "9000")
	if err != nil {
		return Env{}, err
	}

	return Env{
		Endpoint:  fmt.Sprintf("%s:%s", host, port.Port()),
		AccessKey: user,
		SecretKey: pass,
	}, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// removeBucket vide puis supprime un bucket. S3 refuse de supprimer un bucket
// non vide.
func removeBucket(ctx context.Context, client *minio.Client, bucket string) {
	objects := client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true})
	for err := range client.RemoveObjects(ctx, bucket, objects, minio.RemoveObjectsOptions{}) {
		_ = err // best effort : le nettoyage ne doit pas faire échouer un test
	}
	_ = client.RemoveBucket(ctx, bucket)
}

// uniqueBucketName dérive un nom de bucket valide du nom du test.
//
// Contraintes S3 : minuscules, chiffres et tirets, 3 à 63 caractères.
func uniqueBucketName(t *testing.T) string {
	name := strings.ToLower(t.Name())
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}

	// Le nanoseconde suffit à distinguer des sous-tests successifs et permet
	// des exécutions concurrentes sans collision.
	suffix := fmt.Sprintf("-%d", time.Now().UnixNano()%1e10)
	prefix := strings.Trim(b.String(), "-")

	if max := 63 - len(suffix); len(prefix) > max {
		prefix = prefix[:max]
	}
	bucket := strings.Trim(prefix, "-") + suffix

	if len(bucket) < 3 {
		bucket = "test" + bucket
	}
	return bucket
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
