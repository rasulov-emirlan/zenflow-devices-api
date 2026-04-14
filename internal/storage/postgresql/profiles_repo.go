package postgresql

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/profiles"
)

type ProfilesRepo struct {
	pool *pgxpool.Pool
}

func NewProfilesRepo(pool *pgxpool.Pool) *ProfilesRepo { return &ProfilesRepo{pool: pool} }

const profileColumns = `id, user_id, name, device_type, window_width, window_height,
	user_agent, country_code, custom_headers, extra, template_slug, created_at, updated_at`

func (r *ProfilesRepo) Insert(ctx context.Context, p profiles.Profile) error {
	headersJSON, err := json.Marshal(headersToJSON(p.CustomHeaders))
	if err != nil {
		return err
	}
	extraJSON, err := marshalExtra(p.Extra)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO profiles (`+profileColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		p.ID, p.UserID, p.Name, string(p.DeviceType), p.WindowWidth, p.WindowHeight,
		p.UserAgent, p.CountryCode, headersJSON, extraJSON, p.TemplateSlug, p.CreatedAt, p.UpdatedAt,
	)
	return translateProfilesErr(err)
}

func (r *ProfilesRepo) GetByID(ctx context.Context, userID, id string) (profiles.Profile, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+profileColumns+` FROM profiles WHERE id = $1 AND user_id = $2`, id, userID)
	p, err := scanProfile(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return profiles.Profile{}, profiles.ErrNotFound
	}
	return p, err
}

func (r *ProfilesRepo) ListByUser(ctx context.Context, userID string, page profiles.Page) ([]profiles.Profile, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+profileColumns+` FROM profiles
		 WHERE user_id = $1
		 ORDER BY created_at DESC, id
		 LIMIT $2 OFFSET $3`, userID, page.Limit, page.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []profiles.Profile{}
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *ProfilesRepo) Update(ctx context.Context, p profiles.Profile) error {
	headersJSON, err := json.Marshal(headersToJSON(p.CustomHeaders))
	if err != nil {
		return err
	}
	extraJSON, err := marshalExtra(p.Extra)
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE profiles SET
		  name = $3, device_type = $4, window_width = $5, window_height = $6,
		  user_agent = $7, country_code = $8, custom_headers = $9, extra = $10,
		  updated_at = $11
		WHERE id = $1 AND user_id = $2`,
		p.ID, p.UserID, p.Name, string(p.DeviceType), p.WindowWidth, p.WindowHeight,
		p.UserAgent, p.CountryCode, headersJSON, extraJSON, p.UpdatedAt,
	)
	if err != nil {
		return translateProfilesErr(err)
	}
	if tag.RowsAffected() == 0 {
		return profiles.ErrNotFound
	}
	return nil
}

func (r *ProfilesRepo) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM profiles WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return profiles.ErrNotFound
	}
	return nil
}

// --- scanning + JSON helpers ---

type rowScanner interface {
	Scan(dest ...any) error
}

type headerJSON struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func headersToJSON(hs []profiles.Header) []headerJSON {
	out := make([]headerJSON, len(hs))
	for i, h := range hs {
		out[i] = headerJSON{Key: h.Key, Value: h.Value}
	}
	return out
}

func headersFromJSON(raw []byte) ([]profiles.Header, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var tmp []headerJSON
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return nil, err
	}
	out := make([]profiles.Header, len(tmp))
	for i, h := range tmp {
		out[i] = profiles.Header{Key: h.Key, Value: h.Value}
	}
	return out, nil
}

func marshalExtra(m map[string]any) (any, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func scanProfile(row rowScanner) (profiles.Profile, error) {
	var (
		p            profiles.Profile
		deviceType   string
		headersRaw   []byte
		extraRaw     []byte
		templateSlug *string
	)
	if err := row.Scan(
		&p.ID, &p.UserID, &p.Name, &deviceType, &p.WindowWidth, &p.WindowHeight,
		&p.UserAgent, &p.CountryCode, &headersRaw, &extraRaw, &templateSlug,
		&p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return profiles.Profile{}, err
	}
	p.DeviceType = profiles.DeviceType(deviceType)
	headers, err := headersFromJSON(headersRaw)
	if err != nil {
		return profiles.Profile{}, err
	}
	p.CustomHeaders = headers
	if len(extraRaw) > 0 {
		var extra map[string]any
		if err := json.Unmarshal(extraRaw, &extra); err != nil {
			return profiles.Profile{}, err
		}
		p.Extra = extra
	}
	p.TemplateSlug = templateSlug
	return p, nil
}
