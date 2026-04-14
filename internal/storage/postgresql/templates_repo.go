package postgresql

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/pgxtags"
)

const tableTemplates = "templates"

type TemplatesRepo struct {
	pool *pgxpool.Pool
}

func NewTemplatesRepo(pool *pgxpool.Pool) *TemplatesRepo { return &TemplatesRepo{pool: pool} }

const templateColumns = `slug, name, device_type, window_width, window_height,
	user_agent, country_code, custom_headers`

func (r *TemplatesRepo) Get(ctx context.Context, slug string) (templates.Template, error) {
	ctx = pgxtags.With(ctx, "select", tableTemplates)
	row := r.pool.QueryRow(ctx,
		`SELECT `+templateColumns+` FROM templates WHERE slug = $1`, slug)
	t, err := scanTemplate(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return templates.Template{}, templates.ErrNotFound
	}
	return t, err
}

func (r *TemplatesRepo) List(ctx context.Context) ([]templates.Template, error) {
	ctx = pgxtags.With(ctx, "select", tableTemplates)
	rows, err := r.pool.Query(ctx,
		`SELECT `+templateColumns+` FROM templates ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []templates.Template{}
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func scanTemplate(row rowScanner) (templates.Template, error) {
	var (
		t          templates.Template
		headersRaw []byte
	)
	if err := row.Scan(
		&t.Slug, &t.Name, &t.DeviceType, &t.WindowWidth, &t.WindowHeight,
		&t.UserAgent, &t.CountryCode, &headersRaw,
	); err != nil {
		return templates.Template{}, err
	}
	if len(headersRaw) > 0 {
		var tmp []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal(headersRaw, &tmp); err != nil {
			return templates.Template{}, err
		}
		t.CustomHeaders = make([]templates.Header, len(tmp))
		for i, h := range tmp {
			t.CustomHeaders[i] = templates.Header{Key: h.Key, Value: h.Value}
		}
	}
	return t, nil
}
