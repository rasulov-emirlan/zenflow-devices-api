// Package logging provides the app's slog logger and context helpers.
//
// Callers carry a request-scoped logger in ctx (set by middleware or by the
// app bootstrap) and retrieve it via LoggerFromCtx. The returned logger is
// never nil: it falls back to slog.Default() so library code can log safely
// without worrying about plumbing.
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type ctxKey struct{}

// New builds a JSON slog logger at the requested level. Unknown levels fall
// back to info so a typo in config never silences logs.
func New(level string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

// WithLogger returns a new context carrying lg. A nil logger is a no-op.
func WithLogger(ctx context.Context, lg *slog.Logger) context.Context {
	if lg == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, lg)
}

// LoggerFromCtx returns the context logger, or slog.Default() if absent.
func LoggerFromCtx(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if lg, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && lg != nil {
		return lg
	}
	return slog.Default()
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
