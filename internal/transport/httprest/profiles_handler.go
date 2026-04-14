package httprest

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/profiles"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/httpx"
)

type profilesHandler struct {
	svc *profiles.Service
	log *slog.Logger
}

// --- DTOs (transport-owned) ---

type headerDTO struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type profileResponse struct {
	ID            string         `json:"id"`
	UserID        string         `json:"user_id"`
	Name          string         `json:"name"`
	DeviceType    string         `json:"device_type"`
	WindowWidth   int            `json:"window_width"`
	WindowHeight  int            `json:"window_height"`
	UserAgent     string         `json:"user_agent"`
	CountryCode   string         `json:"country_code"`
	CustomHeaders []headerDTO    `json:"custom_headers"`
	Extra         map[string]any `json:"extra,omitempty"`
	TemplateSlug  *string        `json:"template_slug,omitempty"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
}

type createProfileRequest struct {
	Name          string         `json:"name"`
	DeviceType    string         `json:"device_type"`
	WindowWidth   int            `json:"window_width"`
	WindowHeight  int            `json:"window_height"`
	UserAgent     string         `json:"user_agent"`
	CountryCode   string         `json:"country_code"`
	CustomHeaders []headerDTO    `json:"custom_headers"`
	Extra         map[string]any `json:"extra"`
	TemplateSlug  *string        `json:"template_slug"`
}

type patchProfileRequest struct {
	Name          *string         `json:"name"`
	DeviceType    *string         `json:"device_type"`
	WindowWidth   *int            `json:"window_width"`
	WindowHeight  *int            `json:"window_height"`
	UserAgent     *string         `json:"user_agent"`
	CountryCode   *string         `json:"country_code"`
	CustomHeaders *[]headerDTO    `json:"custom_headers"`
	Extra         *map[string]any `json:"extra"`
}

func toProfileResponse(p profiles.Profile) profileResponse {
	headers := make([]headerDTO, len(p.CustomHeaders))
	for i, h := range p.CustomHeaders {
		headers[i] = headerDTO{Key: h.Key, Value: h.Value}
	}
	return profileResponse{
		ID: p.ID, UserID: p.UserID, Name: p.Name,
		DeviceType:    string(p.DeviceType),
		WindowWidth:   p.WindowWidth,
		WindowHeight:  p.WindowHeight,
		UserAgent:     p.UserAgent,
		CountryCode:   p.CountryCode,
		CustomHeaders: headers,
		Extra:         p.Extra,
		TemplateSlug:  p.TemplateSlug,
		CreatedAt:     p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     p.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func fromHeaderDTOs(in []headerDTO) []profiles.Header {
	out := make([]profiles.Header, len(in))
	for i, h := range in {
		out[i] = profiles.Header{Key: h.Key, Value: h.Value}
	}
	return out
}

// --- handlers ---

func (h *profilesHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createProfileRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	in := profiles.Input{
		Name:          req.Name,
		DeviceType:    profiles.DeviceType(req.DeviceType),
		WindowWidth:   req.WindowWidth,
		WindowHeight:  req.WindowHeight,
		UserAgent:     req.UserAgent,
		CountryCode:   req.CountryCode,
		CustomHeaders: fromHeaderDTOs(req.CustomHeaders),
		Extra:         req.Extra,
		TemplateSlug:  req.TemplateSlug,
	}
	p, err := h.svc.Create(r.Context(), UserID(r.Context()), in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toProfileResponse(p))
}

func (h *profilesHandler) list(w http.ResponseWriter, r *http.Request) {
	page := profiles.Page{Limit: parseIntOr(r.URL.Query().Get("limit"), 50), Offset: parseIntOr(r.URL.Query().Get("offset"), 0)}
	items, err := h.svc.List(r.Context(), UserID(r.Context()), page)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]profileResponse, len(items))
	for i, p := range items {
		out[i] = toProfileResponse(p)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *profilesHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.svc.Get(r.Context(), UserID(r.Context()), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toProfileResponse(p))
}

func (h *profilesHandler) patch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req patchProfileRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	patch := profiles.Patch{
		Name:         req.Name,
		WindowWidth:  req.WindowWidth,
		WindowHeight: req.WindowHeight,
		UserAgent:    req.UserAgent,
		CountryCode:  req.CountryCode,
		Extra:        req.Extra,
	}
	if req.DeviceType != nil {
		dt := profiles.DeviceType(*req.DeviceType)
		patch.DeviceType = &dt
	}
	if req.CustomHeaders != nil {
		hs := fromHeaderDTOs(*req.CustomHeaders)
		patch.CustomHeaders = &hs
	}
	p, err := h.svc.Patch(r.Context(), UserID(r.Context()), id, patch)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toProfileResponse(p))
}

func (h *profilesHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), UserID(r.Context()), id); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseIntOr(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
