// Package postgresql — migrator wraps golang-migrate with a small,
// purpose-built API that the app bootstrap and the CLI both call into.
package postgresql

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Migrator drives up/down/version operations over the embedded migrations FS.
// It owns a single *migrate.Migrate instance; callers must Close it.
type Migrator struct {
	m *migrate.Migrate
}

// NewMigrator constructs a Migrator bound to the given DSN. The DSN may be a
// plain postgres:// URL; it is rewritten to the pgx5:// scheme internally.
func NewMigrator(dsn string) (*Migrator, error) {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrate source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, "pgx5://"+trimScheme(dsn))
	if err != nil {
		return nil, fmt.Errorf("migrate init: %w", err)
	}
	return &Migrator{m: m}, nil
}

// Up applies all pending migrations. ErrNoChange is treated as success.
func (mg *Migrator) Up() error {
	if err := mg.m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// Down rolls back `steps` migrations. If steps <= 0 returns an error; callers
// must be explicit about rollback scope to avoid accidental full teardowns.
func (mg *Migrator) Down(steps int) error {
	if steps <= 0 {
		return fmt.Errorf("down: steps must be > 0, got %d", steps)
	}
	if err := mg.m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

// Version returns current schema version and whether the schema is dirty.
// A zero version with ErrNilVersion means no migrations have been applied yet.
func (mg *Migrator) Version() (version uint, dirty bool, err error) {
	v, d, err := mg.m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("version: %w", err)
	}
	return v, d, nil
}

// Force marks the DB at version v and clears the dirty flag. Destructive;
// used only to recover from a failed migration.
func (mg *Migrator) Force(v int) error {
	if err := mg.m.Force(v); err != nil {
		return fmt.Errorf("force: %w", err)
	}
	return nil
}

// HasPending reports whether there are migrations above the current version.
func (mg *Migrator) HasPending() (bool, error) {
	current, dirty, err := mg.Version()
	if err != nil {
		return false, err
	}
	if dirty {
		return true, nil
	}
	// Probe: attempt a +0 no-op isn't supported; instead list source versions
	// via the embedded FS. We re-open the source to enumerate.
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return false, fmt.Errorf("pending: source: %w", err)
	}
	defer func() { _ = src.Close() }()
	first, err := src.First()
	if err != nil {
		return false, nil
	}
	latest := first
	for {
		next, err := src.Next(latest)
		if err != nil {
			break
		}
		latest = next
	}
	return latest > current, nil
}

// Close releases source + database handles held by the underlying migrator.
func (mg *Migrator) Close() error {
	srcErr, dbErr := mg.m.Close()
	return errors.Join(srcErr, dbErr)
}
