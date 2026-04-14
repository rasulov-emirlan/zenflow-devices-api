// Package httprest is the chi-based HTTP transport adapter.
package httprest

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/auth"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/httpx"
)

type ctxKey int

const (
	ctxUserID ctxKey = iota
	ctxReqID
)

const authRealm = `Basic realm="zenflow-devices-api", charset="UTF-8"`

// UserID returns the authenticated user id from the request context.
// Panics if used outside a handler chain that ran basic-auth middleware.
func UserID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxUserID).(string); ok {
		return v
	}
	return ""
}

func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxReqID).(string); ok {
		return v
	}
	return ""
}

func requestIDMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), ctxReqID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func recovererMW(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.ErrorContext(r.Context(), "panic in handler",
						slog.Any("panic", rec),
						slog.String("stack", string(debug.Stack())),
						slog.String("request_id", RequestID(r.Context())),
					)
					httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func loggerMW(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.InfoContext(r.Context(), "http",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Duration("took", time.Since(start)),
				slog.String("request_id", RequestID(r.Context())),
			)
		})
	}
}

func basicAuthMW(resolver *auth.Resolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", authRealm)
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "credentials required")
				return
			}
			uid, err := resolver.Verify(user, pass)
			if err != nil {
				w.Header().Set("WWW-Authenticate", authRealm)
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserID, uid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
