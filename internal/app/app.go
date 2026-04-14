// Package app owns process lifecycle: config, logger, DB, domain services,
// and HTTP server. Init runs each phase in order; phases register cleanups
// that Shutdown runs in LIFO so teardown mirrors setup.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/auth"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/config"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/deviceprofiles"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/seed"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/storage/postgresql"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/transport/httprest"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/logging"
)

type App struct {
	cfg            *config.Config
	log            *slog.Logger
	pool           *pgxpool.Pool
	resolver       *auth.Resolver
	deviceProfiles *deviceprofiles.Service
	templates      *templates.Service
	server         *http.Server

	cleanups []func(context.Context) error
}

func New() *App { return &App{} }

func (a *App) addCleanup(fn func(context.Context) error) {
	a.cleanups = append(a.cleanups, fn)
}

func (a *App) Init(ctx context.Context) error {
	if err := a.initConfig(ctx); err != nil {
		return fmt.Errorf("init config: %w", err)
	}
	if err := a.initLogger(ctx); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	if err := a.initDB(ctx); err != nil {
		return fmt.Errorf("init db: %w", err)
	}
	if err := a.initDomains(ctx); err != nil {
		return fmt.Errorf("init domains: %w", err)
	}
	if err := a.initHTTP(ctx); err != nil {
		return fmt.Errorf("init http: %w", err)
	}
	return nil
}

func (a *App) initConfig(_ context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	a.cfg = cfg
	return nil
}

// Returning error keeps every init step on the same signature so the Init
// pipeline reads uniformly; individual steps may become fallible later.
//
//nolint:unparam
func (a *App) initLogger(_ context.Context) error {
	a.log = logging.New(a.cfg.LogLevel)
	slog.SetDefault(a.log)
	return nil
}

func (a *App) initDB(ctx context.Context) error {
	if err := a.runMigrations(); err != nil {
		return err
	}
	pool, err := postgresql.OpenPool(ctx, a.cfg.DatabaseURL)
	if err != nil {
		return err
	}
	a.pool = pool
	a.addCleanup(func(context.Context) error { pool.Close(); return nil })

	if a.cfg.SeedOnBoot && a.cfg.Env == config.EnvDev {
		// Seed is best-effort idempotent; we run it only in dev, only when
		// explicitly opted in. Config.Load already rejects the prod case.
		seeder := seed.NewTemplateSeeder(pool)
		if err := seeder.Seed(ctx, seed.Options{OnConflict: seed.OnConflictSkip}); err != nil {
			return fmt.Errorf("seed: %w", err)
		}
		a.log.Info("seeded templates on boot")
	}
	return nil
}

// runMigrations applies the configured MigrateMode policy. Kept separate so
// the three branches read cleanly and the initDB body stays small.
func (a *App) runMigrations() error {
	switch a.cfg.MigrateMode {
	case config.MigrateOff:
		a.log.Info("migrations skipped", slog.String("mode", string(a.cfg.MigrateMode)))
		return nil
	case config.MigrateAuto:
		mg, err := postgresql.NewMigrator(a.cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("migrator: %w", err)
		}
		defer func() { _ = mg.Close() }()
		if err := mg.Up(); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		return nil
	case config.MigrateManual:
		mg, err := postgresql.NewMigrator(a.cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("migrator: %w", err)
		}
		defer func() { _ = mg.Close() }()
		pending, err := mg.HasPending()
		if err != nil {
			return fmt.Errorf("check pending: %w", err)
		}
		if pending {
			return errors.New("manual mode: pending migrations detected; run `migrate up`")
		}
		return nil
	default:
		return fmt.Errorf("unknown migrate mode %q", a.cfg.MigrateMode)
	}
}

//nolint:unparam // see initLogger.
func (a *App) initDomains(_ context.Context) error {
	a.resolver = auth.NewResolver(a.cfg.BasicAuthUsers)

	tmplRepo := postgresql.NewTemplatesRepo(a.pool)
	a.templates = templates.NewService(tmplRepo)

	dpRepo := postgresql.NewDeviceProfilesRepo(a.pool)
	a.deviceProfiles = deviceprofiles.NewService(dpRepo, a.templates)
	return nil
}

//nolint:unparam // see initLogger.
func (a *App) initHTTP(_ context.Context) error {
	handler := httprest.NewRouter(httprest.Deps{
		Logger:         a.log,
		Auth:           a.resolver,
		DeviceProfiles: a.deviceProfiles,
		Templates:      a.templates,
	})
	a.server = &http.Server{
		Addr:    a.cfg.Addr(),
		Handler: handler,
	}
	a.addCleanup(func(ctx context.Context) error {
		return a.server.Shutdown(ctx)
	})
	return nil
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		a.log.Info("listening", slog.String("addr", a.server.Addr))
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func (a *App) Shutdown(ctx context.Context) error {
	var errs []error
	for i := len(a.cleanups) - 1; i >= 0; i-- {
		if err := a.cleanups[i](ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
