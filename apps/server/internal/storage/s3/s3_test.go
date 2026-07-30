package s3_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage/s3"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage/storagetest"
	"github.com/adonko3xBitters/boxincloud/server/internal/testsupport/miniotest"
)

// Le provider S3 doit passer exactement la même suite que le provider local.
//
// Ces tests tournent contre un vrai MinIO, jamais contre un mock : un mock ne
// reproduit ni le comportement des requêtes Range, ni les codes d'erreur, ni la
// sémantique des ETag — c'est-à-dire précisément ce qui nous intéresse ici.
func TestIntegrationConformance(t *testing.T) {
	env := miniotest.Start(t)

	p, err := s3.New(s3.Options{
		Endpoint:  env.Endpoint,
		Bucket:    env.Bucket,
		AccessKey: env.AccessKey,
		SecretKey: env.SecretKey,
		UseSSL:    false,
		PathStyle: true,
	})
	if err != nil {
		t.Fatalf("New : %v", err)
	}

	storagetest.RunSuite(t, p)
}

func TestIntegrationPingRejectsMissingBucket(t *testing.T) {
	env := miniotest.Start(t)

	p, err := s3.New(s3.Options{
		Endpoint:  env.Endpoint,
		Bucket:    "bucket-qui-nexiste-pas",
		AccessKey: env.AccessKey,
		SecretKey: env.SecretKey,
		PathStyle: true,
	})
	if err != nil {
		t.Fatalf("New : %v", err)
	}

	if err := p.Ping(context.Background()); err == nil {
		t.Fatal("Ping devrait échouer sur un bucket inexistant")
	}
}

func TestIntegrationPingRejectsBadCredentials(t *testing.T) {
	env := miniotest.Start(t)

	p, err := s3.New(s3.Options{
		Endpoint:  env.Endpoint,
		Bucket:    env.Bucket,
		AccessKey: "mauvaise-cle",
		SecretKey: "mauvais-secret",
		PathStyle: true,
	})
	if err != nil {
		t.Fatalf("New : %v", err)
	}

	if err := p.Ping(context.Background()); err == nil {
		t.Fatal("Ping devrait échouer avec de mauvais identifiants")
	}
}

// Le serveur peut rediriger vers une URL présignée plutôt que de relayer les
// octets — ce qui retire entièrement le trafic des pages du serveur.
func TestIntegrationPresignedURL(t *testing.T) {
	env := miniotest.Start(t)

	p, err := s3.New(s3.Options{
		Endpoint:  env.Endpoint,
		Bucket:    env.Bucket,
		AccessKey: env.AccessKey,
		SecretKey: env.SecretKey,
		PathStyle: true,
	})
	if err != nil {
		t.Fatalf("New : %v", err)
	}

	ctx := context.Background()
	key := "presign/page.jpg"
	if err := p.Write(ctx, key, strings.NewReader("contenu"), 7, "image/jpeg"); err != nil {
		t.Fatalf("Write : %v", err)
	}

	presigner, ok := storage.Provider(p).(storage.Presigner)
	if !ok {
		t.Fatal("le provider S3 devrait implémenter storage.Presigner")
	}

	url, err := presigner.PresignedURL(ctx, key, miniotest.PresignTTL)
	if err != nil {
		t.Fatalf("PresignedURL : %v", err)
	}
	if !strings.Contains(url, key) || !strings.Contains(url, "X-Amz-Signature") {
		t.Errorf("URL présignée inattendue : %s", url)
	}
}

// Une erreur du SDK doit être traduite vers les erreurs de storage : les
// appelants ne doivent jamais distinguer une NoSuchKey S3 d'un ENOENT local.
func TestIntegrationErrorTranslation(t *testing.T) {
	env := miniotest.Start(t)

	p, err := s3.New(s3.Options{
		Endpoint:  env.Endpoint,
		Bucket:    env.Bucket,
		AccessKey: env.AccessKey,
		SecretKey: env.SecretKey,
		PathStyle: true,
	})
	if err != nil {
		t.Fatalf("New : %v", err)
	}

	if _, err := p.Stat(context.Background(), "absent/nulle-part.cbz"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Stat sur clé absente : attendu ErrNotFound, obtenu %v", err)
	}
}

func TestNewValidatesOptions(t *testing.T) {
	if _, err := s3.New(s3.Options{Bucket: "x"}); err == nil {
		t.Error("un endpoint vide devrait être refusé")
	}
	if _, err := s3.New(s3.Options{Endpoint: "localhost:9000"}); err == nil {
		t.Error("un bucket vide devrait être refusé")
	}
}

// Coller le schéma dans l'endpoint est une erreur de configuration fréquente :
// on la tolère plutôt que d'échouer avec un message obscur du SDK.
func TestNewToleratesSchemeInEndpoint(t *testing.T) {
	for _, endpoint := range []string{
		"https://s3.eu-central-003.backblazeb2.com",
		"http://localhost:9000",
		"localhost:9000/",
	} {
		if _, err := s3.New(s3.Options{Endpoint: endpoint, Bucket: "comics"}); err != nil {
			t.Errorf("endpoint %q refusé : %v", endpoint, err)
		}
	}
}
