package httprest

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/auth"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/profiles"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/httpx"
)

type Deps struct {
	Logger    *slog.Logger
	Auth      *auth.Resolver
	Profiles  *profiles.Service
	Templates *templates.Service
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(requestIDMW)
	r.Use(recovererMW(d.Logger))
	r.Use(loggerMW(d.Logger))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	prof := &profilesHandler{svc: d.Profiles, log: d.Logger}
	tmpl := &templatesHandler{svc: d.Templates, log: d.Logger}

	r.Group(func(r chi.Router) {
		r.Use(basicAuthMW(d.Auth))

		r.Route("/profiles", func(r chi.Router) {
			r.Post("/", prof.create)
			r.Get("/", prof.list)
			r.Get("/{id}", prof.get)
			r.Patch("/{id}", prof.patch)
			r.Delete("/{id}", prof.delete)
		})

		r.Route("/templates", func(r chi.Router) {
			r.Get("/", tmpl.list)
			r.Get("/{slug}", tmpl.get)
		})
	})

	return r
}
