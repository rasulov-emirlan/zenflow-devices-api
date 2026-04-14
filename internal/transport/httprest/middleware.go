// Package httprest is the chi-based HTTP transport adapter. Handlers translate
// to/from domain types via DTOs defined alongside each handler.
package httprest

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/auth"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/transport/httprest/respond"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/logging"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/metrics"
)

type ctxKey int

const (
	ctxUserID ctxKey = iota
	ctxReqID
)

const authRealm = `Basic realm="zenflow-devices-api", charset="UTF-8"`

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
					logging.LoggerFromCtx(r.Context()).ErrorContext(r.Context(), "panic in handler",
						slog.Any("panic", rec),
						slog.String("stack", string(debug.Stack())),
					)
					respond.Error(w, http.StatusInternalServerError, "internal_error", "internal error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.status = http.StatusOK
		s.wroteHeader = true
	}
	return s.ResponseWriter.Write(b)
}

// injectLoggerMW enriches ctx with a request-scoped logger carrying stable
// attrs (request_id, method, route, user_id, trace_id, span_id). It runs
// inside chi so RouteContext is populated by the time handlers log.
func injectLoggerMW(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attrs := []any{
				slog.String("request_id", RequestID(r.Context())),
				slog.String("method", r.Method),
			}
			if rc := chi.RouteContext(r.Context()); rc != nil {
				if p := rc.RoutePattern(); p != "" {
					attrs = append(attrs, slog.String("route", p))
				}
			}
			if uid := UserID(r.Context()); uid != "" {
				attrs = append(attrs, slog.String("user_id", uid))
			}
			if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
				attrs = append(attrs,
					slog.String("trace_id", sc.TraceID().String()),
					slog.String("span_id", sc.SpanID().String()),
				)
			}
			lg := base.With(attrs...)
			ctx := logging.WithLogger(r.Context(), lg)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// accessLogMW emits one structured access log per request using the ctx logger
// (which already carries request_id, route, trace_id, etc).
func accessLogMW() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			logging.LoggerFromCtx(r.Context()).InfoContext(r.Context(), "http",
				slog.Int("status", rec.status),
				slog.Duration("took", time.Since(start)),
			)
		})
	}
}

// metricsMW records Prometheus counters/histograms + an in-flight gauge.
// It must be mounted inside chi so RoutePattern() is populated; we keep a
// low-cardinality "route" label by falling back to "unknown" for unmatched
// requests (404s for random paths).
func metricsMW() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			metrics.HTTPRequestsInFlight.Inc()
			defer metrics.HTTPRequestsInFlight.Dec()

			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			route := "unknown"
			if rc := chi.RouteContext(r.Context()); rc != nil {
				if p := rc.RoutePattern(); p != "" {
					route = p
				}
			}
			class := metrics.StatusClass(rec.status)
			metrics.HTTPRequestsTotal.WithLabelValues(r.Method, route, class).Inc()
			metrics.HTTPRequestDuration.WithLabelValues(r.Method, route, class).Observe(time.Since(start).Seconds())
		})
	}
}

// basicAuthExcept applies Basic Auth to every request except those whose path
// is in the skip set (e.g. /healthz, /metrics).
func basicAuthExcept(resolver *auth.Resolver, skip ...string) func(http.Handler) http.Handler {
	skipSet := make(map[string]struct{}, len(skip))
	for _, p := range skip {
		skipSet[p] = struct{}{}
	}
	authMW := basicAuthMW(resolver)
	return func(next http.Handler) http.Handler {
		authed := authMW(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := skipSet[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}
			authed.ServeHTTP(w, r)
		})
	}
}

func basicAuthMW(resolver *auth.Resolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", authRealm)
				respond.Error(w, http.StatusUnauthorized, "unauthorized", "credentials required")
				return
			}
			uid, err := resolver.Verify(user, pass)
			if err != nil {
				w.Header().Set("WWW-Authenticate", authRealm)
				respond.Error(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserID, uid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
