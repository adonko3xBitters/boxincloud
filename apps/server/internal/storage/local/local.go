// Package local implémente storage.Provider sur un système de fichiers.
//
// Sert trois usages : le développement sans MinIO, le chemin de migration
// depuis Komga ou Kavita, et les installations qui n'ont qu'un disque.
//
// Le système de fichiers n'est pas le modèle de référence du projet — il en est
// un cas particulier. Les clés restent des chemins en slash, indépendants du
// séparateur de la plateforme.
package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
)

// Provider expose un répertoire comme backend de stockage.
type Provider struct {
	root     string
	readOnly bool
}

// Options configure un backend local.
type Options struct {
	// Root est le répertoire racine. Toutes les clés y sont relatives.
	Root string
	// ReadOnly refuse les écritures. Utile pour une collection existante que
	// l'on veut indexer sans risque de modification.
	ReadOnly bool
}

var _ storage.Provider = (*Provider)(nil)

// New construit un provider local et vérifie que la racine est utilisable.
func New(opts Options) (*Provider, error) {
	if opts.Root == "" {
		return nil, errors.New("storage/local : le répertoire racine est obligatoire")
	}

	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("storage/local : chemin racine invalide : %w", err)
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("storage/local : racine inaccessible : %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("storage/local : %s n'est pas un répertoire", root)
	}

	return &Provider{root: root, readOnly: opts.ReadOnly}, nil
}

func (p *Provider) Kind() storage.Kind { return storage.KindLocal }

func (p *Provider) Ping(ctx context.Context) error {
	if _, err := os.Stat(p.root); err != nil {
		return fmt.Errorf("storage/local : %w", err)
	}
	return nil
}

// resolve traduit une clé en chemin absolu, en refusant toute sortie de la
// racine.
//
// La traversée de chemin est le risque principal de ce provider : une clé
// venue d'une archive ou d'une saisie utilisateur ne doit jamais pouvoir
// atteindre /etc/passwd.
func (p *Provider) resolve(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("%w : clé vide", storage.ErrNotFound)
	}
	// Les clés sont toujours en slash, quelle que soit la plateforme.
	clean := filepath.Clean(filepath.FromSlash("/" + key))
	full := filepath.Join(p.root, clean)

	// Ceinture et bretelles : après Clean et Join, le chemin doit rester sous
	// la racine.
	if full != p.root && !strings.HasPrefix(full, p.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w : la clé sort du répertoire racine", storage.ErrPermissionDenied)
	}
	return full, nil
}

// keyOf est l'inverse de resolve : chemin absolu → clé en slash.
func (p *Provider) keyOf(path string) (string, error) {
	rel, err := filepath.Rel(p.root, path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func (p *Provider) List(ctx context.Context, prefix string, fn func(storage.ObjectInfo) error) error {
	start := p.root
	if prefix != "" {
		resolved, err := p.resolve(prefix)
		if err != nil {
			return err
		}
		// Un préfixe peut désigner un répertoire ou un début de nom. On part
		// du répertoire parent existant le plus proche et on filtre ensuite.
		if info, err := os.Stat(resolved); err == nil && info.IsDir() {
			start = resolved
		} else {
			start = filepath.Dir(resolved)
		}
	}

	if _, err := os.Stat(start); errors.Is(err, fs.ErrNotExist) {
		return nil // préfixe sans correspondance : liste vide, pas une erreur
	}

	return filepath.WalkDir(start, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Un répertoire illisible ne doit pas interrompre tout le scan
			// d'une bibliothèque.
			if errors.Is(err, fs.ErrPermission) {
				return nil
			}
			return err
		}

		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		key, err := p.keyOf(path)
		if err != nil {
			return err
		}
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil // supprimé pendant le parcours
			}
			return err
		}

		return fn(storage.ObjectInfo{
			Key:         key,
			Size:        info.Size(),
			ModTime:     info.ModTime(),
			ETag:        localETag(info),
			ContentType: contentTypeOf(key),
		})
	})
}

func (p *Provider) Stat(ctx context.Context, key string) (storage.ObjectInfo, error) {
	path, err := p.resolve(key)
	if err != nil {
		return storage.ObjectInfo{}, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return storage.ObjectInfo{}, translateErr(err)
	}
	if info.IsDir() {
		return storage.ObjectInfo{}, fmt.Errorf("%w : %s est un répertoire", storage.ErrNotFound, key)
	}

	return storage.ObjectInfo{
		Key:         key,
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		ETag:        localETag(info),
		ContentType: contentTypeOf(key),
	}, nil
}

func (p *Provider) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	path, err := p.resolve(key)
	if err != nil {
		return nil, err
	}

	// #nosec G304 -- le chemin sort de resolve(), qui refuse toute clé sortant
	// de la racine (test : TestPathTraversalIsBlocked).
	f, err := os.Open(path)
	if err != nil {
		return nil, translateErr(err)
	}
	return f, nil
}

// ReadRange lit length octets à partir de offset.
//
// Sur un fichier local, c'est un simple io.SectionReader — pas de coût réseau,
// mais la même sémantique que sur S3, ce qui permet au reste du code d'ignorer
// le backend.
func (p *Provider) ReadRange(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	if offset < 0 {
		return nil, fmt.Errorf("%w : offset négatif (%d)", storage.ErrInvalidRange, offset)
	}

	path, err := p.resolve(key)
	if err != nil {
		return nil, err
	}

	// #nosec G304 -- voir Open : resolve() borne le chemin à la racine.
	f, err := os.Open(path)
	if err != nil {
		return nil, translateErr(err)
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, translateErr(err)
	}

	if offset >= info.Size() && info.Size() > 0 {
		_ = f.Close()
		return nil, fmt.Errorf("%w : offset %d au-delà de la taille %d", storage.ErrInvalidRange, offset, info.Size())
	}

	remaining := info.Size() - offset
	if length < 0 || length > remaining {
		length = remaining
	}

	return &sectionReadCloser{
		SectionReader: io.NewSectionReader(f, offset, length),
		closer:        f,
	}, nil
}

func (p *Provider) Write(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	if p.readOnly {
		return storage.ErrReadOnly
	}

	path, err := p.resolve(key)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("storage/local : création du répertoire : %w", err)
	}

	// Écriture dans un fichier temporaire puis renommage atomique : une
	// écriture interrompue ne laisse jamais un objet tronqué visible. C'est
	// important pour le cache dérivé, lu en concurrence de son écriture.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("storage/local : fichier temporaire : %w", err)
	}
	tmpName := tmp.Name()

	defer func() {
		// Sans effet si le renommage a réussi.
		_ = os.Remove(tmpName)
	}()

	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("storage/local : écriture : %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("storage/local : fermeture : %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("storage/local : renommage : %w", err)
	}
	return nil
}

func (p *Provider) Delete(ctx context.Context, key string) error {
	if p.readOnly {
		return storage.ErrReadOnly
	}

	path, err := p.resolve(key)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return translateErr(err)
	}
	return nil // supprimer un objet absent n'est pas une erreur
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// sectionReadCloser ferme le fichier sous-jacent en même temps que la section.
type sectionReadCloser struct {
	*io.SectionReader
	closer io.Closer
}

func (s *sectionReadCloser) Close() error { return s.closer.Close() }

// localETag fabrique un identifiant de version à partir de la taille et de la
// date de modification.
//
// Ce n'est pas un hash du contenu — comme les ETag S3 composites, on le traite
// comme une chaîne opaque dont seule l'égalité a un sens.
func localETag(info os.FileInfo) string {
	return fmt.Sprintf("%d-%d", info.Size(), info.ModTime().UnixNano())
}

func contentTypeOf(key string) string {
	if ct := mime.TypeByExtension(filepath.Ext(key)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

func translateErr(err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w : %w", storage.ErrNotFound, err)
	case errors.Is(err, fs.ErrPermission):
		return fmt.Errorf("%w : %w", storage.ErrPermissionDenied, err)
	default:
		return fmt.Errorf("storage/local : %w", err)
	}
}
