package httprest

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/httpx"
)

type templatesHandler struct {
	svc *templates.Service
	log *slog.Logger
}

type templateResponse struct {
	Slug          string      `json:"slug"`
	Name          string      `json:"name"`
	DeviceType    string      `json:"device_type"`
	WindowWidth   int         `json:"window_width"`
	WindowHeight  int         `json:"window_height"`
	UserAgent     string      `json:"user_agent"`
	CountryCode   string      `json:"country_code"`
	CustomHeaders []headerDTO `json:"custom_headers"`
}

func toTemplateResponse(t templates.Template) templateResponse {
	hs := make([]headerDTO, len(t.CustomHeaders))
	for i, h := range t.CustomHeaders {
		hs[i] = headerDTO{Key: h.Key, Value: h.Value}
	}
	return templateResponse{
		Slug: t.Slug, Name: t.Name, DeviceType: t.DeviceType,
		WindowWidth: t.WindowWidth, WindowHeight: t.WindowHeight,
		UserAgent: t.UserAgent, CountryCode: t.CountryCode,
		CustomHeaders: hs,
	}
}

func (h *templatesHandler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]templateResponse, len(items))
	for i, t := range items {
		out[i] = toTemplateResponse(t)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *templatesHandler) get(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	t, err := h.svc.Get(r.Context(), slug)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toTemplateResponse(t))
}
