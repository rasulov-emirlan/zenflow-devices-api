package config

import "testing"

func TestParseUsers(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    map[string]string
		wantErr bool
	}{
		{"empty", "", map[string]string{}, false},
		{"single", "alice:$2a$10$xyz", map[string]string{"alice": "$2a$10$xyz"}, false},
		{"two", "alice:h1,bob:h2", map[string]string{"alice": "h1", "bob": "h2"}, false},
		{"bcrypt with colons", "alice:$2a$10$abc:def", map[string]string{"alice": "$2a$10$abc:def"}, false},
		{"missing hash", "alice:", nil, true},
		{"missing user", ":hash", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseUsers(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tc.want))
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("got[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestParseEnv(t *testing.T) {
	for _, in := range []string{"dev", "staging", "prod"} {
		if _, err := parseEnv(in); err != nil {
			t.Errorf("parseEnv(%q) unexpected err: %v", in, err)
		}
	}
	if _, err := parseEnv("qa"); err == nil {
		t.Error("parseEnv(qa) want err, got nil")
	}
}

func TestParseMigrateMode(t *testing.T) {
	for _, in := range []string{"auto", "manual", "off"} {
		if _, err := parseMigrateMode(in); err != nil {
			t.Errorf("parseMigrateMode(%q) unexpected err: %v", in, err)
		}
	}
	if _, err := parseMigrateMode("yolo"); err == nil {
		t.Error("parseMigrateMode(yolo) want err, got nil")
	}
}

func TestDefaultMigrateMode(t *testing.T) {
	if got := defaultMigrateMode(EnvProd); got != string(MigrateOff) {
		t.Errorf("prod default = %q, want off", got)
	}
	if got := defaultMigrateMode(EnvDev); got != string(MigrateAuto) {
		t.Errorf("dev default = %q, want auto", got)
	}
}

func TestLoadValidation(t *testing.T) {
	// minimal valid env for Load
	base := map[string]string{
		"DATABASE_URL":     "postgres://x",
		"BASIC_AUTH_USERS": "a:h",
	}
	set := func(t *testing.T, m map[string]string) {
		t.Helper()
		for k, v := range m {
			t.Setenv(k, v)
		}
	}

	t.Run("default dev auto", func(t *testing.T) {
		set(t, base)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load err: %v", err)
		}
		if cfg.Env != EnvDev || cfg.MigrateMode != MigrateAuto || cfg.SeedOnBoot {
			t.Errorf("unexpected cfg: %+v", cfg)
		}
	})

	t.Run("prod default off", func(t *testing.T) {
		set(t, base)
		t.Setenv("APP_ENV", "prod")
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load err: %v", err)
		}
		if cfg.MigrateMode != MigrateOff {
			t.Errorf("prod default mode = %q, want off", cfg.MigrateMode)
		}
	})

	t.Run("prod auto rejected", func(t *testing.T) {
		set(t, base)
		t.Setenv("APP_ENV", "prod")
		t.Setenv("MIGRATE_MODE", "auto")
		if _, err := Load(); err == nil {
			t.Fatal("want err, got nil")
		}
	})

	t.Run("prod seed rejected", func(t *testing.T) {
		set(t, base)
		t.Setenv("APP_ENV", "prod")
		t.Setenv("SEED_ON_BOOT", "true")
		if _, err := Load(); err == nil {
			t.Fatal("want err, got nil")
		}
	})

	t.Run("invalid env", func(t *testing.T) {
		set(t, base)
		t.Setenv("APP_ENV", "qa")
		if _, err := Load(); err == nil {
			t.Fatal("want err, got nil")
		}
	})
}
