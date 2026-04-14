CREATE TABLE IF NOT EXISTS templates (
  slug           TEXT PRIMARY KEY,
  name           TEXT NOT NULL,
  device_type    TEXT NOT NULL CHECK (device_type IN ('desktop','mobile')),
  window_width   INT  NOT NULL CHECK (window_width  BETWEEN 1 AND 10000),
  window_height  INT  NOT NULL CHECK (window_height BETWEEN 1 AND 10000),
  user_agent     TEXT NOT NULL,
  country_code   CHAR(2) NOT NULL,
  custom_headers JSONB NOT NULL DEFAULT '[]'::jsonb
);

CREATE TABLE IF NOT EXISTS device_profiles (
  id             UUID PRIMARY KEY,
  user_id        TEXT NOT NULL,
  name           TEXT NOT NULL,
  device_type    TEXT NOT NULL CHECK (device_type IN ('desktop','mobile')),
  window_width   INT  NOT NULL CHECK (window_width  BETWEEN 1 AND 10000),
  window_height  INT  NOT NULL CHECK (window_height BETWEEN 1 AND 10000),
  user_agent     TEXT NOT NULL,
  country_code   CHAR(2) NOT NULL,
  custom_headers JSONB NOT NULL DEFAULT '[]'::jsonb,
  extra          JSONB,
  template_slug  TEXT REFERENCES templates(slug) ON DELETE SET NULL,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, name)
);

CREATE INDEX IF NOT EXISTS device_profiles_user_idx ON device_profiles(user_id);
