//go:build integration

// Run with: go test -tags=integration ./test/...
// Requires Docker.
package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/bcrypt"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/auth"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/profiles"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/storage/postgresql"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/transport/httprest"
)

func TestProfilesEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	pgC, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("zenflow"),
		tcpostgres.WithUsername("zenflow"),
		tcpostgres.WithPassword("zenflow"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pgC.Terminate(context.Background()) })

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}

	if err := postgresql.Migrate(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := postgresql.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	resolver := auth.NewResolver(map[string]string{"alice": string(hash), "bob": string(hash)})

	tmplSvc := templates.NewService(postgresql.NewTemplatesRepo(pool))
	profSvc := profiles.NewService(postgresql.NewProfilesRepo(pool), tmplSvc)

	handler := httprest.NewRouter(httprest.Deps{
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Auth:      resolver,
		Profiles:  profSvc,
		Templates: tmplSvc,
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	body := map[string]any{
		"name":          "my-desktop",
		"device_type":   "desktop",
		"window_width":  1920,
		"window_height": 1080,
		"user_agent":    "Mozilla/5.0 (X11; Linux)",
		"country_code":  "US",
		"custom_headers": []map[string]string{
			{"key": "Accept-Language", "value": "en-US"},
		},
	}

	t.Run("401 without auth", func(t *testing.T) {
		resp := do(t, srv.URL, http.MethodPost, "/profiles", body, "", "")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d", resp.StatusCode)
		}
	})

	var createdID string
	t.Run("201 happy path", func(t *testing.T) {
		resp := do(t, srv.URL, http.MethodPost, "/profiles", body, "alice", "secret")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, body=%s", resp.StatusCode, readBody(resp))
		}
		var got map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got["user_id"] != "alice" || got["name"] != "my-desktop" {
			t.Fatalf("unexpected body: %+v", got)
		}
		createdID, _ = got["id"].(string)
		if createdID == "" {
			t.Fatal("empty id")
		}
	})

	t.Run("409 duplicate name", func(t *testing.T) {
		resp := do(t, srv.URL, http.MethodPost, "/profiles", body, "alice", "secret")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("status = %d", resp.StatusCode)
		}
	})

	t.Run("400 invalid", func(t *testing.T) {
		bad := map[string]any{
			"name": "x", "device_type": "desktop",
			"window_width": 1920, "window_height": 1080,
			"user_agent": "ua", "country_code": "usa",
		}
		resp := do(t, srv.URL, http.MethodPost, "/profiles", bad, "alice", "secret")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d", resp.StatusCode)
		}
	})

	t.Run("404 cross-user isolation", func(t *testing.T) {
		resp := do(t, srv.URL, http.MethodGet, "/profiles/"+createdID, nil, "bob", "secret")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d", resp.StatusCode)
		}
	})

	t.Run("templates seeded", func(t *testing.T) {
		resp := do(t, srv.URL, http.MethodGet, "/templates", nil, "alice", "secret")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		var got struct{ Items []map[string]any }
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if len(got.Items) == 0 {
			t.Fatal("no templates seeded")
		}
	})

	t.Run("create from template", func(t *testing.T) {
		slug := "mobile-iphone-us"
		req := map[string]any{
			"name":          "phone-profile",
			"template_slug": slug,
		}
		resp := do(t, srv.URL, http.MethodPost, "/profiles", req, "alice", "secret")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, body=%s", resp.StatusCode, readBody(resp))
		}
		var got map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&got)
		if got["device_type"] != "mobile" || got["country_code"] != "US" {
			t.Fatalf("template not applied: %+v", got)
		}
	})
}

func do(t *testing.T, baseURL, method, path string, body any, user, pass string) *http.Response {
	t.Helper()
	var buf io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		buf = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, baseURL+path, buf)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// _ reference to fmt to keep import set stable if we add debug prints.
var _ = fmt.Sprintf
