// Package s3 implémente storage.Provider sur un stockage objet compatible S3.
//
// Couvre MinIO, AWS S3, Backblaze B2, Cloudflare R2, Wasabi et Garage. C'est le
// backend qui porte la promesse du projet : servir une page de BD sans
// télécharger l'archive, grâce aux requêtes HTTP Range.
package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
)

// Provider expose un bucket S3 comme backend de stockage.
type Provider struct {
	client   *minio.Client
	bucket   string
	readOnly bool
}

// Options configure un backend S3.
type Options struct {
	// Endpoint sans schéma : "s3.amazonaws.com", "minio:9000",
	// "s3.eu-central-003.backblazeb2.com".
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	UseSSL    bool

	// PathStyle force les URL de la forme endpoint/bucket/key au lieu de
	// bucket.endpoint/key. Nécessaire pour MinIO et la plupart des
	// installations auto-hébergées.
	PathStyle bool

	ReadOnly bool
}

var (
	_ storage.Provider  = (*Provider)(nil)
	_ storage.Presigner = (*Provider)(nil)
)

// New construit un provider S3. Ne contacte pas le serveur — utiliser Ping
// pour valider les identifiants.
func New(opts Options) (*Provider, error) {
	if opts.Endpoint == "" {
		return nil, errors.New("storage/s3 : endpoint obligatoire")
	}
	if opts.Bucket == "" {
		return nil, errors.New("storage/s3 : bucket obligatoire")
	}

	// Une erreur fréquente en configuration : coller le schéma dans
	// l'endpoint. On le tolère en le retirant, plutôt que d'échouer avec un
	// message obscur du SDK.
	endpoint := opts.Endpoint
	switch {
	case strings.HasPrefix(endpoint, "https://"):
		endpoint = strings.TrimPrefix(endpoint, "https://")
		opts.UseSSL = true
	case strings.HasPrefix(endpoint, "http://"):
		endpoint = strings.TrimPrefix(endpoint, "http://")
	}
	endpoint = strings.TrimSuffix(endpoint, "/")

	cfg := &minio.Options{
		Creds:  credentials.NewStaticV4(opts.AccessKey, opts.SecretKey, ""),
		Secure: opts.UseSSL,
		Region: opts.Region,
	}
	if opts.PathStyle {
		cfg.BucketLookup = minio.BucketLookupPath
	}

	client, err := minio.New(endpoint, cfg)
	if err != nil {
		return nil, fmt.Errorf("storage/s3 : %w", err)
	}

	return &Provider{client: client, bucket: opts.Bucket, readOnly: opts.ReadOnly}, nil
}

func (p *Provider) Kind() storage.Kind { return storage.KindS3 }

// Ping vérifie que le bucket existe et que les identifiants y donnent accès.
func (p *Provider) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	exists, err := p.client.BucketExists(ctx, p.bucket)
	if err != nil {
		return fmt.Errorf("storage/s3 : %w", translateErr(err))
	}
	if !exists {
		return fmt.Errorf("storage/s3 : le bucket %q n'existe pas ou n'est pas accessible avec ces identifiants", p.bucket)
	}
	return nil
}

// List énumère les objets sous un préfixe.
//
// Le parcours est en flux : une bibliothèque peut contenir des dizaines de
// milliers d'objets, et minio-go pagine de lui-même.
func (p *Provider) List(ctx context.Context, prefix string, fn func(storage.ObjectInfo) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // libère la goroutine interne de minio-go si fn interrompt

	objects := p.client.ListObjects(ctx, p.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for obj := range objects {
		if obj.Err != nil {
			return fmt.Errorf("storage/s3 : listage : %w", translateErr(obj.Err))
		}
		// Les marqueurs de répertoire des consoles S3 : clé finissant par "/"
		// et taille nulle. Ce ne sont pas des objets à indexer.
		if strings.HasSuffix(obj.Key, "/") && obj.Size == 0 {
			continue
		}

		if err := fn(storage.ObjectInfo{
			Key:         obj.Key,
			Size:        obj.Size,
			ModTime:     obj.LastModified,
			ETag:        strings.Trim(obj.ETag, `"`),
			ContentType: obj.ContentType,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (p *Provider) Stat(ctx context.Context, key string) (storage.ObjectInfo, error) {
	info, err := p.client.StatObject(ctx, p.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return storage.ObjectInfo{}, translateErr(err)
	}

	return storage.ObjectInfo{
		Key:         key,
		Size:        info.Size,
		ModTime:     info.LastModified,
		ETag:        strings.Trim(info.ETag, `"`),
		ContentType: info.ContentType,
	}, nil
}

func (p *Provider) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := p.client.GetObject(ctx, p.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, translateErr(err)
	}
	return prime(obj)
}

// ReadRange lit length octets à partir de offset via une requête HTTP Range.
//
// ★ C'est la méthode qui fait exister boxincloud. Elle permet de lire l'index
// d'une archive de 200 Mo, puis d'en extraire une page précise, avec au total
// quelques kilo-octets transférés.
func (p *Provider) ReadRange(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	if offset < 0 {
		return nil, fmt.Errorf("%w : offset négatif (%d)", storage.ErrInvalidRange, offset)
	}

	opts := minio.GetObjectOptions{}

	if length < 0 {
		// De offset jusqu'à la fin. SetRange avec un end nul signifierait
		// « zéro octet » : il faut passer par l'en-tête directement.
		if offset > 0 {
			opts.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}
	} else {
		if length == 0 {
			return io.NopCloser(strings.NewReader("")), nil
		}
		// SetRange est inclusif sur les deux bornes.
		if err := opts.SetRange(offset, offset+length-1); err != nil {
			return nil, fmt.Errorf("%w : %w", storage.ErrInvalidRange, err)
		}
	}

	obj, err := p.client.GetObject(ctx, p.bucket, key, opts)
	if err != nil {
		return nil, translateErr(err)
	}
	return prime(obj)
}

// prime déclenche la requête HTTP et remonte immédiatement les erreurs.
//
// minio.GetObject est paresseux : sans cela, une clé absente ou une plage
// invalide n'échouerait qu'à la première lecture, loin de l'appel fautif.
//
// On ne peut PAS utiliser obj.Stat() pour cela : sur un objet ouvert avec une
// plage, Stat réinitialise l'offset interne et la lecture rend l'objet entier
// — ce qui viderait de son sens toute l'architecture du projet. On amorce donc
// en lisant le premier octet, que l'on remet devant le flux. Coût : une seule
// requête HTTP, celle que l'on voulait de toute façon.
func prime(obj io.ReadCloser) (io.ReadCloser, error) {
	var first [1]byte

	n, err := obj.Read(first[:])
	if err != nil && !errors.Is(err, io.EOF) {
		_ = obj.Close()
		return nil, translateErr(err)
	}
	if n == 0 {
		_ = obj.Close()
		return io.NopCloser(bytes.NewReader(nil)), nil
	}

	return &primedReader{
		reader: io.MultiReader(bytes.NewReader(first[:n]), obj),
		closer: obj,
	}, nil
}

// primedReader recolle l'octet d'amorçage devant le reste du flux.
type primedReader struct {
	reader io.Reader
	closer io.Closer
}

func (r *primedReader) Read(p []byte) (int, error) { return r.reader.Read(p) }
func (r *primedReader) Close() error               { return r.closer.Close() }

func (p *Provider) Write(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	if p.readOnly {
		return storage.ErrReadOnly
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// size < 0 : minio-go bascule sur un envoi multipart en bufferisant.
	_, err := p.client.PutObject(ctx, p.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("storage/s3 : écriture de %q : %w", key, translateErr(err))
	}
	return nil
}

func (p *Provider) Delete(ctx context.Context, key string) error {
	if p.readOnly {
		return storage.ErrReadOnly
	}

	err := p.client.RemoveObject(ctx, p.bucket, key, minio.RemoveObjectOptions{})
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("storage/s3 : suppression de %q : %w", key, translateErr(err))
	}
	return nil
}

// PresignedURL produit une URL d'accès direct temporaire.
//
// Quand le backend le permet, le serveur y redirige le client au lieu de
// relayer les octets. Sur une bibliothèque servie à plusieurs personnes, cela
// retire entièrement le trafic des pages du serveur.
func (p *Provider) PresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := p.client.PresignedGetObject(ctx, p.bucket, key, ttl, nil)
	if err != nil {
		return "", fmt.Errorf("storage/s3 : présignature de %q : %w", key, translateErr(err))
	}
	return u.String(), nil
}

// ─── Traduction des erreurs ──────────────────────────────────────────────────

// translateErr convertit les erreurs du SDK vers celles de storage.
//
// Les appelants ne doivent jamais avoir à distinguer une NoSuchKey S3 d'un
// ENOENT local : c'est toute la raison d'être de l'abstraction.
func translateErr(err error) error {
	if err == nil {
		return nil
	}

	var resp minio.ErrorResponse
	if errors.As(err, &resp) {
		switch {
		case resp.StatusCode == http.StatusNotFound,
			resp.Code == "NoSuchKey", resp.Code == "NoSuchBucket":
			return fmt.Errorf("%w : %s", storage.ErrNotFound, resp.Key)

		case resp.StatusCode == http.StatusForbidden,
			resp.Code == "AccessDenied", resp.Code == "InvalidAccessKeyId",
			resp.Code == "SignatureDoesNotMatch":
			return fmt.Errorf("%w : %s", storage.ErrPermissionDenied, resp.Message)

		case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable,
			resp.Code == "InvalidRange":
			return fmt.Errorf("%w : %s", storage.ErrInvalidRange, resp.Message)
		}
	}
	return err
}

func isNotFound(err error) bool {
	return errors.Is(translateErr(err), storage.ErrNotFound)
}
