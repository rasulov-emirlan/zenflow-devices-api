package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Env is the deployment environment. It gates destructive operations and
// defaults for migration/seed behavior.
type Env string

const (
	EnvDev     Env = "dev"
	EnvStaging Env = "staging"
	EnvProd    Env = "prod"
)

// MigrateMode controls what initDB does with pending migrations at boot.
type MigrateMode string

const (
	MigrateAuto   MigrateMode = "auto"   // apply pending migrations up
	MigrateManual MigrateMode = "manual" // fail fast if any pending
	MigrateOff    MigrateMode = "off"    // skip, trust external tooling
)

type Config struct {
	Env            Env
	MigrateMode    MigrateMode
	SeedOnBoot     bool
	Port           string
	DatabaseURL    string
	LogLevel       string
	BasicAuthUsers map[string]string // username -> bcrypt hash
}

func Load() (*Config, error) {
	envStr := strings.ToLower(getenv("APP_ENV", string(EnvDev)))
	env, err := parseEnv(envStr)
	if err != nil {
		return nil, err
	}

	modeStr := strings.ToLower(getenv("MIGRATE_MODE", defaultMigrateMode(env)))
	mode, err := parseMigrateMode(modeStr)
	if err != nil {
		return nil, err
	}
	if env == EnvProd && mode == MigrateAuto {
		// Auto migrations in production would let a rollback-worthy schema
		// change ship silently with the app. Force the operator to opt in.
		return nil, errors.New("MIGRATE_MODE=auto is not allowed when APP_ENV=prod")
	}

	seedOnBoot, err := parseBool(getenv("SEED_ON_BOOT", "false"))
	if err != nil {
		return nil, fmt.Errorf("parse SEED_ON_BOOT: %w", err)
	}
	if seedOnBoot && env == EnvProd {
		return nil, errors.New("SEED_ON_BOOT=true is not allowed when APP_ENV=prod")
	}

	cfg := &Config{
		Env:         env,
		MigrateMode: mode,
		SeedOnBoot:  seedOnBoot,
		Port:        getenv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		LogLevel:    getenv("LOG_LEVEL", "info"),
	}
	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	users, err := parseUsers(os.Getenv("BASIC_AUTH_USERS"))
	if err != nil {
		return nil, fmt.Errorf("parse BASIC_AUTH_USERS: %w", err)
	}
	if len(users) == 0 {
		return nil, errors.New("BASIC_AUTH_USERS is required (at least one user)")
	}
	cfg.BasicAuthUsers = users
	return cfg, nil
}

func (c *Config) Addr() string { return ":" + c.Port }

func getenv(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func parseEnv(s string) (Env, error) {
	switch Env(s) {
	case EnvDev, EnvStaging, EnvProd:
		return Env(s), nil
	default:
		return "", fmt.Errorf("invalid APP_ENV %q (want dev|staging|prod)", s)
	}
}

func parseMigrateMode(s string) (MigrateMode, error) {
	switch MigrateMode(s) {
	case MigrateAuto, MigrateManual, MigrateOff:
		return MigrateMode(s), nil
	default:
		return "", fmt.Errorf("invalid MIGRATE_MODE %q (want auto|manual|off)", s)
	}
}

func defaultMigrateMode(env Env) string {
	if env == EnvProd {
		return string(MigrateOff)
	}
	return string(MigrateAuto)
}

func parseBool(s string) (bool, error) {
	if s == "" {
		return false, nil
	}
	return strconv.ParseBool(s)
}

// parseUsers parses "user1:hash1,user2:hash2".
// bcrypt hashes contain ':' only after "$2a$10$" — we split on the FIRST colon.
func parseUsers(raw string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return out, nil
	}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		idx := strings.Index(pair, ":")
		if idx <= 0 || idx == len(pair)-1 {
			return nil, fmt.Errorf("invalid user:hash pair %q", pair)
		}
		name := strings.TrimSpace(pair[:idx])
		hash := strings.TrimSpace(pair[idx+1:])
		out[name] = hash
	}
	return out, nil
}
