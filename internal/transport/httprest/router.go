package httprest

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/auth"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/deviceprofiles"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/transport/httprest/gen"
)

type Deps struct {
	Logger         *slog.Logger
	Auth           *auth.Resolver
	DeviceProfiles *deviceprofiles.Service
	Templates      *templates.Service
}

// NewRouter wires the generated OpenAPI server into a chi router with project
// middleware. /healthz and /metrics (if mounted here) bypass Basic Auth. The
// router is wrapped with otelhttp so inbound requests create server spans and
// W3C traceparent is honored.
func NewRouter(d Deps) http.Handler {
	handlers := &Handlers{deviceProfiles: d.DeviceProfiles, templates: d.Templates, log: d.Logger}

	r := chi.NewRouter()
	r.Use(requestIDMW)
	// metricsMW and injectLoggerMW must run after chi has resolved the route
	// so RoutePattern() and span context are available.
	r.Use(metricsMW())
	r.Use(injectLoggerMW(d.Logger))
	r.Use(recovererMW(d.Logger))
	r.Use(accessLogMW())
	r.Use(basicAuthExcept(d.Auth, "/healthz", "/metrics"))

	gen.HandlerFromMux(handlers, r)

	// Name spans after the matched route so traces group usefully.
	return otelhttp.NewHandler(r, "http.server",
		otelhttp.WithSpanNameFormatter(func(_ string, req *http.Request) string {
			if rc := chi.RouteContext(req.Context()); rc != nil {
				if p := rc.RoutePattern(); p != "" {
					return req.Method + " " + p
				}
			}
			return req.Method + " " + req.URL.Path
		}),
	)
}
