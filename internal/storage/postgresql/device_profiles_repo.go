package postgresql

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/deviceprofiles"
)

type DeviceProfilesRepo struct {
	pool *pgxpool.Pool
}

func NewDeviceProfilesRepo(pool *pgxpool.Pool) *DeviceProfilesRepo {
	return &DeviceProfilesRepo{pool: pool}
}

const deviceProfileColumns = `id, user_id, name, device_type, window_width, window_height,
	user_agent, country_code, custom_headers, extra, template_slug, created_at, updated_at`

func (r *DeviceProfilesRepo) Insert(ctx context.Context, p deviceprofiles.DeviceProfile) error {
	headersJSON, err := json.Marshal(headersToJSON(p.CustomHeaders))
	if err != nil {
		return err
	}
	extraJSON, err := marshalExtra(p.Extra)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO device_profiles (`+deviceProfileColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		p.ID, p.UserID, p.Name, string(p.DeviceType), p.WindowWidth, p.WindowHeight,
		p.UserAgent, p.CountryCode, headersJSON, extraJSON, p.TemplateSlug, p.CreatedAt, p.UpdatedAt,
	)
	return translateDeviceProfilesErr(err)
}

func (r *DeviceProfilesRepo) GetByID(ctx context.Context, userID, id string) (deviceprofiles.DeviceProfile, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+deviceProfileColumns+` FROM device_profiles WHERE id = $1 AND user_id = $2`, id, userID)
	p, err := scanDeviceProfile(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return deviceprofiles.DeviceProfile{}, deviceprofiles.ErrNotFound
	}
	return p, err
}

func (r *DeviceProfilesRepo) ListByUser(ctx context.Context, userID string, page deviceprofiles.Page) ([]deviceprofiles.DeviceProfile, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+deviceProfileColumns+` FROM device_profiles
		 WHERE user_id = $1
		 ORDER BY created_at DESC, id
		 LIMIT $2 OFFSET $3`, userID, page.Limit, page.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []deviceprofiles.DeviceProfile{}
	for rows.Next() {
		p, err := scanDeviceProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *DeviceProfilesRepo) Update(ctx context.Context, p deviceprofiles.DeviceProfile) error {
	headersJSON, err := json.Marshal(headersToJSON(p.CustomHeaders))
	if err != nil {
		return err
	}
	extraJSON, err := marshalExtra(p.Extra)
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE device_profiles SET
		  name = $3, device_type = $4, window_width = $5, window_height = $6,
		  user_agent = $7, country_code = $8, custom_headers = $9, extra = $10,
		  updated_at = $11
		WHERE id = $1 AND user_id = $2`,
		p.ID, p.UserID, p.Name, string(p.DeviceType), p.WindowWidth, p.WindowHeight,
		p.UserAgent, p.CountryCode, headersJSON, extraJSON, p.UpdatedAt,
	)
	if err != nil {
		return translateDeviceProfilesErr(err)
	}
	if tag.RowsAffected() == 0 {
		return deviceprofiles.ErrNotFound
	}
	return nil
}

func (r *DeviceProfilesRepo) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM device_profiles WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return deviceprofiles.ErrNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

type headerJSON struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func headersToJSON(hs []deviceprofiles.Header) []headerJSON {
	out := make([]headerJSON, len(hs))
	for i, h := range hs {
		out[i] = headerJSON{Key: h.Key, Value: h.Value}
	}
	return out
}

func headersFromJSON(raw []byte) ([]deviceprofiles.Header, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var tmp []headerJSON
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return nil, err
	}
	out := make([]deviceprofiles.Header, len(tmp))
	for i, h := range tmp {
		out[i] = deviceprofiles.Header{Key: h.Key, Value: h.Value}
	}
	return out, nil
}

func marshalExtra(m map[string]any) (any, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func scanDeviceProfile(row rowScanner) (deviceprofiles.DeviceProfile, error) {
	var (
		p            deviceprofiles.DeviceProfile
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
		return deviceprofiles.DeviceProfile{}, err
	}
	p.DeviceType = deviceprofiles.DeviceType(deviceType)
	headers, err := headersFromJSON(headersRaw)
	if err != nil {
		return deviceprofiles.DeviceProfile{}, err
	}
	p.CustomHeaders = headers
	if len(extraRaw) > 0 {
		var extra map[string]any
		if err := json.Unmarshal(extraRaw, &extra); err != nil {
			return deviceprofiles.DeviceProfile{}, err
		}
		p.Extra = extra
	}
	p.TemplateSlug = templateSlug
	return p, nil
}
