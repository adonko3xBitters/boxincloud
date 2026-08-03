// Package config charge et valide la configuration du serveur depuis
// l'environnement.
//
// Principe : une seule source (les variables d'environnement, 12-factor), une
// validation stricte au démarrage, et un message d'erreur qui dit quoi faire.
// Un serveur mal configuré doit refuser de démarrer immédiatement plutôt que
// d'échouer plus tard sur une requête.
package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment distingue les comportements de développement de ceux de production.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"
)

func (e Environment) IsDevelopment() bool { return e == EnvDevelopment }

// Config regroupe l'intégralité de la configuration du serveur.
//
// Les backends de stockage et les bibliothèques ne figurent volontairement pas
// ici : ils sont administrés depuis l'interface et stockés en base, pour
// permettre d'en ajouter sans redémarrer.
type Config struct {
	Env       Environment
	Addr      string
	PublicURL string

	LogLevel  slog.Level
	LogFormat string // "json" | "text"

	Database Database
	Jobs     Jobs
	Auth     Auth
	Cache    Cache
	Upload   Upload

	// SecretKey chiffre les identifiants des backends de stockage en base.
	SecretKey []byte
}

type Database struct {
	URL         string
	MaxConns    int32
	MinConns    int32
	AutoMigrate bool
}

type Jobs struct {
	Enabled    bool
	MaxWorkers int
}

type Auth struct {
	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type Cache struct {
	Dir     string
	MaxSize int64 // octets
}

// Upload borne ce qu'un client peut envoyer.
type Upload struct {
	// MaxSize est la taille maximale d'un fichier téléversé, en octets.
	//
	// Une intégrale numérisée dépasse couramment les 500 Mo ; le défaut est
	// donc large. Zéro lève la limite, ce qui n'est raisonnable que sur un
	// serveur dont on maîtrise tous les comptes.
	MaxSize int64
}

// Load lit la configuration depuis l'environnement et la valide.
func Load() (*Config, error) {
	var errs []error
	fail := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	cfg := &Config{
		Env:       Environment(envString("BOXINCLOUD_ENV", string(EnvDevelopment))),
		Addr:      envString("BOXINCLOUD_ADDR", ":8080"),
		PublicURL: envString("BOXINCLOUD_PUBLIC_URL", "http://localhost:8080"),
		LogFormat: envString("BOXINCLOUD_LOG_FORMAT", "text"),
	}

	if cfg.Env != EnvDevelopment && cfg.Env != EnvProduction {
		fail("BOXINCLOUD_ENV doit valoir 'development' ou 'production' (reçu %q)", cfg.Env)
	}

	level, err := parseLogLevel(envString("BOXINCLOUD_LOG_LEVEL", "info"))
	if err != nil {
		fail("BOXINCLOUD_LOG_LEVEL : %w", err)
	}
	cfg.LogLevel = level

	if cfg.LogFormat != "json" && cfg.LogFormat != "text" {
		fail("BOXINCLOUD_LOG_FORMAT doit valoir 'json' ou 'text' (reçu %q)", cfg.LogFormat)
	}

	// ── Base de données ──────────────────────────────────────────────────
	cfg.Database = Database{
		URL:         os.Getenv("DATABASE_URL"),
		MaxConns:    envInt32("BOXINCLOUD_DB_MAX_CONNS", 20),
		MinConns:    envInt32("BOXINCLOUD_DB_MIN_CONNS", 2),
		AutoMigrate: envBool("BOXINCLOUD_DB_AUTO_MIGRATE", true),
	}
	if cfg.Database.URL == "" {
		fail("DATABASE_URL est obligatoire (ex. postgres://user:pass@localhost:5432/boxincloud?sslmode=disable)")
	}
	if cfg.Database.MinConns > cfg.Database.MaxConns {
		fail("BOXINCLOUD_DB_MIN_CONNS (%d) ne peut dépasser BOXINCLOUD_DB_MAX_CONNS (%d)",
			cfg.Database.MinConns, cfg.Database.MaxConns)
	}

	// ── Clé de chiffrement ───────────────────────────────────────────────
	if rawKey := os.Getenv("BOXINCLOUD_SECRET_KEY"); rawKey == "" {
		fail("BOXINCLOUD_SECRET_KEY est obligatoire — générer avec : openssl rand -hex 32")
	} else {
		key, err := hex.DecodeString(rawKey)
		switch {
		case err != nil:
			fail("BOXINCLOUD_SECRET_KEY doit être en hexadécimal — générer avec : openssl rand -hex 32")
		case len(key) != 32:
			fail("BOXINCLOUD_SECRET_KEY doit faire 32 octets (64 caractères hex), reçu %d octets", len(key))
		default:
			cfg.SecretKey = key
		}
	}

	// ── Jobs ─────────────────────────────────────────────────────────────
	cfg.Jobs = Jobs{
		Enabled:    envBool("BOXINCLOUD_JOBS_ENABLED", true),
		MaxWorkers: envInt("BOXINCLOUD_JOBS_MAX_WORKERS", 8),
	}
	if cfg.Jobs.Enabled && cfg.Jobs.MaxWorkers < 1 {
		fail("BOXINCLOUD_JOBS_MAX_WORKERS doit valoir au moins 1")
	}

	// ── Authentification ─────────────────────────────────────────────────
	cfg.Auth = Auth{
		AccessTokenTTL:  envDuration("BOXINCLOUD_ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: envDuration("BOXINCLOUD_REFRESH_TOKEN_TTL", 30*24*time.Hour),
	}
	if raw := os.Getenv("BOXINCLOUD_JWT_SECRET"); raw != "" {
		secret, err := hex.DecodeString(raw)
		if err != nil {
			fail("BOXINCLOUD_JWT_SECRET doit être en hexadécimal")
		} else {
			cfg.Auth.JWTSecret = secret
		}
	} else {
		// Dérivée de la clé maître : une variable de moins à configurer.
		cfg.Auth.JWTSecret = cfg.SecretKey
	}

	// ── Téléversement ────────────────────────────────────────────────────
	uploadSize, err := parseByteSize(envString("BOXINCLOUD_UPLOAD_MAX_SIZE", "2GB"))
	if err != nil {
		fail("BOXINCLOUD_UPLOAD_MAX_SIZE : %w", err)
	}
	cfg.Upload = Upload{MaxSize: uploadSize}

	// ── Cache dérivé ─────────────────────────────────────────────────────
	size, err := parseByteSize(envString("BOXINCLOUD_CACHE_MAX_SIZE", "10GB"))
	if err != nil {
		fail("BOXINCLOUD_CACHE_MAX_SIZE : %w", err)
	}
	cfg.Cache = Cache{
		Dir:     envString("BOXINCLOUD_CACHE_DIR", "./data/cache"),
		MaxSize: size,
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("configuration invalide :\n%w%s", errors.Join(errs...), hintEnv())
	}
	return cfg, nil
}

/*
hintEnv rappelle comment charger .env, quand un .env existe justement.

La configuration vient de l'ENVIRONNEMENT, délibérément : en production, c'est
Docker ou systemd qui la fournit, et lire un fichier caché à côté du binaire
serait une surprise. Ce choix a un coût en développement, et ce coût s'est
manifesté six fois de suite pendant une seule session de mise au point — le
binaire ignore le `.env` posé à côté de lui, et rien ne le dit.

Le rappel n'apparaît QUE si un `.env` existe : ailleurs il désignerait un
fichier absent, ce qui égarerait au lieu d'aider. Le message reste donc muet en
production, où il n'aurait aucun sens.
*/
func hintEnv() string {
	candidates := []string{".env", "../../.env"}

	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		return "\n\nUn fichier .env existe, mais les binaires ne le lisent pas :" +
			" la configuration vient de l'environnement.\n" +
			"En développement, passez par make, qui le charge pour vous :\n\n" +
			"    make run                        # le serveur\n" +
			"    make ctl ARGS=\"user list\"       # boxincloudctl\n"
	}
	return ""
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// envInt32 borne la valeur à l'intervalle d'un int32 : pgxpool attend un int32
// pour la taille du pool, et une conversion naïve depuis int tronquerait
// silencieusement une valeur aberrante.
func envInt32(key string, def int32) int32 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return def
	}
	return int32(n)
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func parseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("niveau inconnu %q (attendu : debug, info, warn, error)", s)
	}
}

// parseByteSize accepte "10GB", "512MB", "1024" (octets). Insensible à la casse.
func parseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0, errors.New("valeur vide")
	}

	units := []struct {
		suffix string
		factor int64
	}{
		{"TB", 1 << 40},
		{"GB", 1 << 30},
		{"MB", 1 << 20},
		{"KB", 1 << 10},
		{"B", 1},
	}

	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			num := strings.TrimSpace(strings.TrimSuffix(s, u.suffix))
			f, err := strconv.ParseFloat(num, 64)
			if err != nil {
				return 0, fmt.Errorf("nombre invalide %q", num)
			}
			if f < 0 {
				return 0, errors.New("la taille ne peut être négative")
			}
			return int64(f * float64(u.factor)), nil
		}
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("format invalide %q (attendu : 10GB, 512MB, ou un nombre d'octets)", s)
	}
	return n, nil
}
