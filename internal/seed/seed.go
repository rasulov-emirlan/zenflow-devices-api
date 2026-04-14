// Package seed populates reference data (e.g. the template catalog) into
// the database. Seeds are separated from schema migrations so that data can
// evolve independently of DDL and so that environments can opt in/out.
package seed

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/pgxtags"
)

// OnConflict selects how existing rows are handled when seeding.
type OnConflict string

const (
	OnConflictSkip   OnConflict = "skip"
	OnConflictUpdate OnConflict = "update"
	OnConflictFail   OnConflict = "fail"
)

// ParseOnConflict accepts a CLI string form and returns the enum.
func ParseOnConflict(s string) (OnConflict, error) {
	switch OnConflict(s) {
	case OnConflictSkip, OnConflictUpdate, OnConflictFail:
		return OnConflict(s), nil
	default:
		return "", fmt.Errorf("invalid on-conflict %q (want skip|update|fail)", s)
	}
}

// Options is the knob bag accepted by a Seeder.
type Options struct {
	OnConflict OnConflict
}

// Seeder is implemented by any source of reference data. Implementations must
// be idempotent under OnConflictSkip and OnConflictUpdate.
type Seeder interface {
	Seed(ctx context.Context, opts Options) error
}

//go:embed data/templates.json
var templatesJSON []byte

type header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type templateRow struct {
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	DeviceType    string   `json:"device_type"`
	WindowWidth   int      `json:"window_width"`
	WindowHeight  int      `json:"window_height"`
	UserAgent     string   `json:"user_agent"`
	CountryCode   string   `json:"country_code"`
	CustomHeaders []header `json:"custom_headers"`
}

// TemplateSeeder writes the embedded template catalog into the `templates`
// table. It is safe to run repeatedly.
type TemplateSeeder struct {
	pool *pgxpool.Pool
}

// NewTemplateSeeder wires the seeder to a live pgx pool.
func NewTemplateSeeder(pool *pgxpool.Pool) *TemplateSeeder {
	return &TemplateSeeder{pool: pool}
}

// Seed applies the embedded template JSON to the DB respecting opts.OnConflict.
func (s *TemplateSeeder) Seed(ctx context.Context, opts Options) error {
	if opts.OnConflict == "" {
		opts.OnConflict = OnConflictSkip
	}
	var rows []templateRow
	if err := json.Unmarshal(templatesJSON, &rows); err != nil {
		return fmt.Errorf("decode templates.json: %w", err)
	}
	if len(rows) == 0 {
		return errors.New("templates.json is empty")
	}

	ctx = pgxtags.With(ctx, "upsert", "templates")
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const base = `INSERT INTO templates
	  (slug, name, device_type, window_width, window_height, user_agent, country_code, custom_headers)
	  VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb)`
	var tail string
	switch opts.OnConflict {
	case OnConflictSkip:
		tail = ` ON CONFLICT (slug) DO NOTHING`
	case OnConflictUpdate:
		tail = ` ON CONFLICT (slug) DO UPDATE SET
		  name = EXCLUDED.name,
		  device_type = EXCLUDED.device_type,
		  window_width = EXCLUDED.window_width,
		  window_height = EXCLUDED.window_height,
		  user_agent = EXCLUDED.user_agent,
		  country_code = EXCLUDED.country_code,
		  custom_headers = EXCLUDED.custom_headers`
	case OnConflictFail:
		tail = ``
	}

	for _, r := range rows {
		headersJSON, err := json.Marshal(r.CustomHeaders)
		if err != nil {
			return fmt.Errorf("marshal headers for %q: %w", r.Slug, err)
		}
		if _, err := tx.Exec(ctx, base+tail,
			r.Slug, r.Name, r.DeviceType, r.WindowWidth, r.WindowHeight,
			r.UserAgent, r.CountryCode, string(headersJSON),
		); err != nil {
			return fmt.Errorf("insert %q: %w", r.Slug, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// FromSource returns a Seeder for the named source. Unknown names return an error.
func FromSource(name string, pool *pgxpool.Pool) (Seeder, error) {
	switch name {
	case "templates":
		return NewTemplateSeeder(pool), nil
	default:
		return nil, fmt.Errorf("unknown seed source %q", name)
	}
}
