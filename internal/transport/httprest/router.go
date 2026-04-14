package httprest

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

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
// middleware. /healthz is public; everything else requires Basic Auth.
func NewRouter(d Deps) http.Handler {
	handlers := &Handlers{deviceProfiles: d.DeviceProfiles, templates: d.Templates, log: d.Logger}

	r := chi.NewRouter()
	r.Use(requestIDMW)
	r.Use(recovererMW(d.Logger))
	r.Use(loggerMW(d.Logger))
	r.Use(basicAuthExcept(d.Auth, "/healthz"))

	gen.HandlerFromMux(handlers, r)
	return r
}
