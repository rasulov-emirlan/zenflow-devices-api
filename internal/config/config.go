package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port           string
	DatabaseURL    string
	LogLevel       string
	BasicAuthUsers map[string]string // username -> bcrypt hash
}

func Load() (*Config, error) {
	cfg := &Config{
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
