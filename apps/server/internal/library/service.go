// Package library administre les backends de stockage et les bibliothèques.
//
// C'est ici que la configuration persistée en base devient un
// storage.Provider utilisable. Une instance peut ainsi servir une bibliothèque
// depuis un MinIO local et une autre depuis un Backblaze B2, sans redémarrage
// ni fichier de configuration.
package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/crypto"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage/local"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage/s3"
)

var (
	ErrBackendNotFound = errors.New("library : backend de stockage introuvable")
	ErrLibraryNotFound = errors.New("library : bibliothèque introuvable")
	ErrInvalidConfig   = errors.New("library : configuration de backend invalide")
)

// Backend est la vue applicative d'un backend de stockage.
//
// Secrets n'y figure pas : les identifiants ne sortent jamais de la base
// autrement que pour construire un provider.
type Backend struct {
	ID        uuid.UUID
	Name      string
	Kind      storage.Kind
	Config    map[string]string
	IsDefault bool
	ReadOnly  bool
	Status    string
}

// Library est la vue applicative d'une bibliothèque.
type Library struct {
	ID         uuid.UUID
	BackendID  uuid.UUID
	Name       string
	Kind       string
	RootPrefix string
	ComicCount int32
}

// Repository est ce dont le service a besoin de la base.
//
// Déclarée au point d'usage : le module ne dépend pas du paquet de données
// généré, ce qui le rend testable avec une doublure.
type Repository interface {
	CreateBackend(ctx context.Context, b Backend, secretsEnc []byte) (Backend, error)
	GetBackend(ctx context.Context, id uuid.UUID) (Backend, []byte, error)
	GetBackendByName(ctx context.Context, name string) (Backend, []byte, error)
	ListBackends(ctx context.Context) ([]Backend, error)
	SetBackendStatus(ctx context.Context, id uuid.UUID, status, detail string) error

	CreateLibrary(ctx context.Context, l Library) (Library, error)
	GetLibrary(ctx context.Context, id uuid.UUID) (Library, error)
	GetLibraryByName(ctx context.Context, name string) (Library, error)
	ListLibraries(ctx context.Context) ([]Library, error)
}

// Service construit et met en cache les providers de stockage.
type Service struct {
	repo   Repository
	admin  AdminRepository
	sealer *crypto.Sealer
	log    *slog.Logger

	// Les providers sont réutilisés : chacun tient un pool de connexions HTTP,
	// qu'il serait absurde de recréer à chaque lecture de page.
	mu        sync.RWMutex
	providers map[uuid.UUID]storage.Provider
}

func NewService(repo Repository, sealer *crypto.Sealer, log *slog.Logger) *Service {
	return &Service{
		repo:      repo,
		sealer:    sealer,
		log:       log,
		providers: make(map[uuid.UUID]storage.Provider),
	}
}

// ─── Backends ────────────────────────────────────────────────────────────────

// CreateBackendParams décrit un backend à enregistrer.
type CreateBackendParams struct {
	Name      string
	Kind      storage.Kind
	Config    map[string]string
	Secrets   map[string]string
	IsDefault bool
	ReadOnly  bool
}

// CreateBackend enregistre un backend après avoir vérifié qu'il fonctionne.
//
// La vérification précède l'enregistrement : un backend injoignable ou aux
// identifiants erronés ne doit pas entrer en base, où il produirait des scans
// en échec sans cause évidente.
func (s *Service) CreateBackend(ctx context.Context, p CreateBackendParams) (Backend, error) {
	if p.Name == "" {
		return Backend{}, fmt.Errorf("%w : le nom est obligatoire", ErrInvalidConfig)
	}

	provider, err := buildProvider(p.Kind, p.Config, p.Secrets, p.ReadOnly)
	if err != nil {
		return Backend{}, err
	}
	if err := provider.Ping(ctx); err != nil {
		return Backend{}, fmt.Errorf("le backend ne répond pas : %w", err)
	}

	secretsEnc, err := s.sealSecrets(p.Secrets)
	if err != nil {
		return Backend{}, err
	}

	backend := Backend{
		ID:        uuid.Must(uuid.NewV7()),
		Name:      p.Name,
		Kind:      p.Kind,
		Config:    p.Config,
		IsDefault: p.IsDefault,
		ReadOnly:  p.ReadOnly,
		Status:    "ok",
	}

	created, err := s.repo.CreateBackend(ctx, backend, secretsEnc)
	if err != nil {
		return Backend{}, err
	}

	s.mu.Lock()
	s.providers[created.ID] = provider
	s.mu.Unlock()

	return created, nil
}

// ProviderFor retourne le provider d'un backend, en le construisant au besoin.
func (s *Service) ProviderFor(ctx context.Context, backendID uuid.UUID) (storage.Provider, error) {
	s.mu.RLock()
	p, ok := s.providers[backendID]
	s.mu.RUnlock()
	if ok {
		return p, nil
	}

	backend, secretsEnc, err := s.repo.GetBackend(ctx, backendID)
	if err != nil {
		return nil, err
	}

	secrets, err := s.openSecrets(secretsEnc)
	if err != nil {
		return nil, err
	}

	provider, err := buildProvider(backend.Kind, backend.Config, secrets, backend.ReadOnly)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	// Une construction concurrente a pu gagner la course : on garde la sienne
	// pour ne pas multiplier les pools de connexions.
	if existing, ok := s.providers[backendID]; ok {
		s.mu.Unlock()
		return existing, nil
	}
	s.providers[backendID] = provider
	s.mu.Unlock()

	return provider, nil
}

// TestBackend vérifie qu'un backend répond et met à jour son état.
func (s *Service) TestBackend(ctx context.Context, backendID uuid.UUID) error {
	provider, err := s.ProviderFor(ctx, backendID)
	if err != nil {
		return err
	}

	if err := provider.Ping(ctx); err != nil {
		_ = s.repo.SetBackendStatus(ctx, backendID, "error", err.Error())
		return err
	}
	return s.repo.SetBackendStatus(ctx, backendID, "ok", "")
}

// Invalidate oublie le provider d'un backend, pour qu'il soit reconstruit à la
// prochaine utilisation. À appeler après modification de sa configuration.
func (s *Service) Invalidate(backendID uuid.UUID) {
	s.mu.Lock()
	delete(s.providers, backendID)
	s.mu.Unlock()
}

func (s *Service) ListBackends(ctx context.Context) ([]Backend, error) {
	return s.repo.ListBackends(ctx)
}

func (s *Service) GetBackendByName(ctx context.Context, name string) (Backend, error) {
	b, _, err := s.repo.GetBackendByName(ctx, name)
	return b, err
}

// ─── Bibliothèques ───────────────────────────────────────────────────────────

type CreateLibraryParams struct {
	Name       string
	BackendID  uuid.UUID
	Kind       string
	RootPrefix string
}

// CreateLibrary enregistre une bibliothèque après avoir vérifié que son backend
// est accessible.
func (s *Service) CreateLibrary(ctx context.Context, p CreateLibraryParams) (Library, error) {
	if p.Name == "" {
		return Library{}, fmt.Errorf("%w : le nom est obligatoire", ErrInvalidConfig)
	}
	if p.Kind == "" {
		p.Kind = "comic"
	}

	provider, err := s.ProviderFor(ctx, p.BackendID)
	if err != nil {
		return Library{}, err
	}
	if err := provider.Ping(ctx); err != nil {
		return Library{}, fmt.Errorf("le backend de cette bibliothèque ne répond pas : %w", err)
	}

	return s.repo.CreateLibrary(ctx, Library{
		ID:         uuid.Must(uuid.NewV7()),
		BackendID:  p.BackendID,
		Name:       p.Name,
		Kind:       p.Kind,
		RootPrefix: p.RootPrefix,
	})
}

func (s *Service) GetLibrary(ctx context.Context, id uuid.UUID) (Library, error) {
	return s.repo.GetLibrary(ctx, id)
}

func (s *Service) GetLibraryByName(ctx context.Context, name string) (Library, error) {
	return s.repo.GetLibraryByName(ctx, name)
}

func (s *Service) ListLibraries(ctx context.Context) ([]Library, error) {
	return s.repo.ListLibraries(ctx)
}

// ProviderForLibrary est le raccourci utilisé par l'indexeur et le lecteur.
func (s *Service) ProviderForLibrary(ctx context.Context, lib Library) (storage.Provider, error) {
	return s.ProviderFor(ctx, lib.BackendID)
}

// ─── Construction des providers ──────────────────────────────────────────────

// buildProvider traduit une configuration persistée en provider concret.
//
// Point d'extension : ajouter un backend consiste à ajouter un cas ici et une
// implémentation dans internal/storage. Aucun autre module n'est concerné.
func buildProvider(kind storage.Kind, config, secrets map[string]string, readOnly bool) (storage.Provider, error) {
	switch kind {
	case storage.KindS3:
		bucket := config["bucket"]
		if bucket == "" {
			return nil, fmt.Errorf("%w : 'bucket' est obligatoire pour un backend S3", ErrInvalidConfig)
		}
		endpoint := config["endpoint"]
		if endpoint == "" {
			return nil, fmt.Errorf("%w : 'endpoint' est obligatoire pour un backend S3", ErrInvalidConfig)
		}

		return s3.New(s3.Options{
			Endpoint:  endpoint,
			Bucket:    bucket,
			Region:    config["region"],
			AccessKey: secrets["access_key"],
			SecretKey: secrets["secret_key"],
			UseSSL:    parseBool(config["use_ssl"], true),
			// Les installations auto-hébergées — MinIO en tête — exigent le
			// style « path ». C'est le défaut le plus sûr pour ce public.
			PathStyle: parseBool(config["path_style"], true),
			ReadOnly:  readOnly,
		})

	case storage.KindLocal:
		root := config["root"]
		if root == "" {
			return nil, fmt.Errorf("%w : 'root' est obligatoire pour un backend local", ErrInvalidConfig)
		}
		return local.New(local.Options{Root: root, ReadOnly: readOnly})

	case storage.KindWebDAV:
		return nil, fmt.Errorf("%w : le backend WebDAV n'est pas encore implémenté", ErrInvalidConfig)

	default:
		return nil, fmt.Errorf("%w : type de backend inconnu %q", ErrInvalidConfig, kind)
	}
}

func parseBool(s string, def bool) bool {
	if s == "" {
		return def
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return def
	}
	return b
}

// ─── Secrets ─────────────────────────────────────────────────────────────────

func (s *Service) sealSecrets(secrets map[string]string) ([]byte, error) {
	if len(secrets) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(secrets)
	if err != nil {
		return nil, fmt.Errorf("library : sérialisation des identifiants : %w", err)
	}
	sealed, err := s.sealer.Seal(raw)
	if err != nil {
		return nil, fmt.Errorf("library : chiffrement des identifiants : %w", err)
	}
	return sealed, nil
}

func (s *Service) openSecrets(sealed []byte) (map[string]string, error) {
	if len(sealed) == 0 {
		return map[string]string{}, nil
	}
	raw, err := s.sealer.Open(sealed)
	if err != nil {
		return nil, fmt.Errorf("library : déchiffrement des identifiants impossible — "+
			"BOXINCLOUD_SECRET_KEY a-t-elle changé ? : %w", err)
	}
	var secrets map[string]string
	if err := json.Unmarshal(raw, &secrets); err != nil {
		return nil, fmt.Errorf("library : identifiants illisibles : %w", err)
	}
	return secrets, nil
}
