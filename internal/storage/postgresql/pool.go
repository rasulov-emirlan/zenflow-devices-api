// Package postgresql is the pgx-backed storage adapter for the domain repos.
package postgresql

import (
	"context"
	"embed"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/multitracer"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/pgxtags"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// OpenPool builds a pgx pool with observability attached: OTLP traces via
// otelpgx and Prometheus metrics via the local metrics tracer. The tracers
// are combined via multitracer so both fire for every query.
func OpenPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.ConnConfig.Tracer = multitracer.New(
		NewMetricsTracer(),
		otelpgx.NewTracer(),
	)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("new pool: %w", err)
	}
	if err := pool.Ping(pgxtags.With(ctx, "ping", "internal")); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// Migrate runs all pending up migrations. Thin convenience wrapper around
// Migrator for callers (e.g. tests) that want a one-shot apply.
func Migrate(dsn string) error {
	mg, err := NewMigrator(dsn)
	if err != nil {
		return err
	}
	defer func() { _ = mg.Close() }()
	return mg.Up()
}

// trimScheme strips the postgres:// prefix so callers can re-prefix with pgx5://,
// which is the scheme registered by golang-migrate's pgx/v5 driver.
func trimScheme(dsn string) string {
	for _, p := range []string{"postgres://", "postgresql://"} {
		if len(dsn) >= len(p) && dsn[:len(p)] == p {
			return dsn[len(p):]
		}
	}
	return dsn
}
