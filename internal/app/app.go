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
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/auth"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/config"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/deviceprofiles"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/seed"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/storage/postgresql"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/transport/httprest"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/logging"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/metrics"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/observability"
)

type App struct {
	cfg            *config.Config
	log            *slog.Logger
	pool           *pgxpool.Pool
	resolver       *auth.Resolver
	deviceProfiles *deviceprofiles.Service
	templates      *templates.Service
	server         *http.Server
	adminServer    *http.Server

	cleanups []func(context.Context) error
}

func New() *App { return &App{} }

func (a *App) addCleanup(fn func(context.Context) error) {
	a.cleanups = append(a.cleanups, fn)
}

// phase runs one init step with startup timing logged. Keeps Init readable
// and makes slow phases obvious at boot without each step repeating the
// duration bookkeeping.
func (a *App) phase(name string, fn func() error) error {
	start := time.Now()
	if err := fn(); err != nil {
		if a.log != nil {
			a.log.Error("startup phase failed",
				slog.String("phase", name),
				slog.Duration("took", time.Since(start)),
				slog.String("err", err.Error()),
			)
		}
		return fmt.Errorf("%s: %w", name, err)
	}
	if a.log != nil {
		a.log.Info("startup phase ok",
			slog.String("phase", name),
			slog.Duration("took", time.Since(start)),
		)
	}
	return nil
}

func (a *App) Init(ctx context.Context) error {
	bootStart := time.Now()
	if err := a.initConfig(ctx); err != nil {
		return fmt.Errorf("init config: %w", err)
	}
	if err := a.initLogger(ctx); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	if err := a.phase("tracing", func() error { return a.initTracing(ctx) }); err != nil {
		return err
	}
	if err := a.phase("db", func() error { return a.initDB(ctx) }); err != nil {
		return err
	}
	if err := a.phase("domains", func() error { return a.initDomains(ctx) }); err != nil {
		return err
	}
	if err := a.phase("http", func() error { return a.initHTTP(ctx) }); err != nil {
		return err
	}
	if err := a.phase("admin", func() error { return a.initAdminHTTP(ctx) }); err != nil {
		return err
	}
	a.log.Info("startup complete", slog.Duration("took", time.Since(bootStart)))
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

// initTracing is a no-op unless TRACING_ENABLED is true. When enabled, a
// shutdown hook is registered so batched spans are flushed on exit.
func (a *App) initTracing(ctx context.Context) error {
	if !a.cfg.TracingEnabled {
		a.log.Info("tracing disabled")
		return nil
	}
	shutdown, err := observability.InitTracer(ctx, a.cfg.ServiceName, a.cfg.OTLPEndpoint, string(a.cfg.Env))
	if err != nil {
		// We treat tracing as best-effort — a collector outage must not break
		// the app. Log and continue.
		a.log.Warn("tracing init failed; continuing without traces",
			slog.String("endpoint", a.cfg.OTLPEndpoint),
			slog.String("err", err.Error()),
		)
		return nil
	}
	a.addCleanup(func(ctx context.Context) error {
		start := time.Now()
		err := shutdown(ctx)
		a.log.Info("tracing shutdown",
			slog.Duration("took", time.Since(start)),
			slog.Any("err", err),
		)
		return err
	})
	a.log.Info("tracing enabled",
		slog.String("endpoint", a.cfg.OTLPEndpoint),
		slog.String("service", a.cfg.ServiceName),
	)
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

func (a *App) initDomains(_ context.Context) error {
	a.resolver = auth.NewResolver(a.cfg.BasicAuthUsers)

	tmplRepo := postgresql.NewTemplatesRepo(a.pool)
	a.templates = templates.NewService(tmplRepo)

	dpRepo := postgresql.NewDeviceProfilesRepo(a.pool)
	a.deviceProfiles = deviceprofiles.NewService(dpRepo, a.templates)
	return nil
}

func (a *App) initHTTP(_ context.Context) error {
	handler := httprest.NewRouter(httprest.Deps{
		Logger:         a.log,
		Auth:           a.resolver,
		DeviceProfiles: a.deviceProfiles,
		Templates:      a.templates,
	})
	a.server = &http.Server{
		Addr:              a.cfg.Addr(),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	a.addCleanup(func(ctx context.Context) error {
		start := time.Now()
		err := a.server.Shutdown(ctx)
		a.log.Info("http shutdown", slog.Duration("took", time.Since(start)), slog.Any("err", err))
		return err
	})
	return nil
}

// initAdminHTTP stands up a separate listener for /metrics and /healthz so
// the admin surface can be firewalled independently of the public API. The
// Prometheus endpoint uses our local Registry (not the default one).
func (a *App) initAdminHTTP(_ context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{
		Registry: metrics.Registry,
	}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	a.adminServer = &http.Server{
		Addr:              a.cfg.MetricsAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	a.addCleanup(func(ctx context.Context) error {
		start := time.Now()
		err := a.adminServer.Shutdown(ctx)
		a.log.Info("admin http shutdown", slog.Duration("took", time.Since(start)), slog.Any("err", err))
		return err
	})
	return nil
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 2)
	go func() {
		a.log.Info("admin listening", slog.String("addr", a.adminServer.Addr))
		if err := a.adminServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("admin: %w", err)
			return
		}
		errCh <- nil
	}()
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
	start := time.Now()
	if a.log != nil {
		a.log.Info("shutdown start")
	}
	var errs []error
	for i := len(a.cleanups) - 1; i >= 0; i-- {
		if err := a.cleanups[i](ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if a.log != nil {
		a.log.Info("shutdown complete", slog.Duration("took", time.Since(start)))
	}
	return errors.Join(errs...)
}
