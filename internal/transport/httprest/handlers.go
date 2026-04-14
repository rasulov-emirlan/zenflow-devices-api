package httprest

import (
	"log/slog"
	"net/http"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/profiles"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/transport/httprest/gen"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/transport/httprest/respond"
)

// Handlers implements gen.ServerInterface.
type Handlers struct {
	profiles  *profiles.Service
	templates *templates.Service
	log       *slog.Logger
}

var _ gen.ServerInterface = (*Handlers)(nil)

const timeFormat = "2006-01-02T15:04:05Z"

// GetHealth implements GET /healthz.
func (h *Handlers) GetHealth(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, gen.HealthResponse{Status: "ok"})
}

// ListProfiles implements GET /profiles.
func (h *Handlers) ListProfiles(w http.ResponseWriter, r *http.Request, params gen.ListProfilesParams) {
	page := profiles.Page{Limit: 50, Offset: 0}
	if params.Limit != nil {
		page.Limit = *params.Limit
	}
	if params.Offset != nil {
		page.Offset = *params.Offset
	}
	items, err := h.profiles.List(r.Context(), UserID(r.Context()), page)
	if err != nil {
		respond.DomainError(w, h.log, err)
		return
	}
	out := make([]gen.Profile, len(items))
	for i, p := range items {
		out[i] = toGenProfile(p)
	}
	respond.JSON(w, http.StatusOK, gen.ProfileList{Items: out})
}

// CreateProfile implements POST /profiles.
func (h *Handlers) CreateProfile(w http.ResponseWriter, r *http.Request) {
	var req gen.CreateProfileRequest
	if err := respond.DecodeBody(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	in := profiles.Input{
		Name:          req.Name,
		DeviceType:    profiles.DeviceType(req.DeviceType),
		WindowWidth:   req.WindowWidth,
		WindowHeight:  req.WindowHeight,
		UserAgent:     req.UserAgent,
		CountryCode:   req.CountryCode,
		CustomHeaders: fromGenHeaders(req.CustomHeaders),
		TemplateSlug:  req.TemplateSlug,
	}
	if req.Extra != nil {
		in.Extra = *req.Extra
	}
	p, err := h.profiles.Create(r.Context(), UserID(r.Context()), in)
	if err != nil {
		respond.DomainError(w, h.log, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toGenProfile(p))
}

// GetProfile implements GET /profiles/{id}.
func (h *Handlers) GetProfile(w http.ResponseWriter, r *http.Request, id gen.ProfileID) {
	p, err := h.profiles.Get(r.Context(), UserID(r.Context()), id)
	if err != nil {
		respond.DomainError(w, h.log, err)
		return
	}
	respond.JSON(w, http.StatusOK, toGenProfile(p))
}

// PatchProfile implements PATCH /profiles/{id}.
func (h *Handlers) PatchProfile(w http.ResponseWriter, r *http.Request, id gen.ProfileID) {
	var req gen.PatchProfileRequest
	if err := respond.DecodeBody(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_body", err.Error())
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
		hs := fromGenHeaders(req.CustomHeaders)
		patch.CustomHeaders = &hs
	}
	p, err := h.profiles.Patch(r.Context(), UserID(r.Context()), id, patch)
	if err != nil {
		respond.DomainError(w, h.log, err)
		return
	}
	respond.JSON(w, http.StatusOK, toGenProfile(p))
}

// DeleteProfile implements DELETE /profiles/{id}.
func (h *Handlers) DeleteProfile(w http.ResponseWriter, r *http.Request, id gen.ProfileID) {
	if err := h.profiles.Delete(r.Context(), UserID(r.Context()), id); err != nil {
		respond.DomainError(w, h.log, err)
		return
	}
	respond.NoContent(w)
}

// ListTemplates implements GET /templates.
func (h *Handlers) ListTemplates(w http.ResponseWriter, r *http.Request) {
	items, err := h.templates.List(r.Context())
	if err != nil {
		respond.DomainError(w, h.log, err)
		return
	}
	out := make([]gen.Template, len(items))
	for i, t := range items {
		out[i] = toGenTemplate(t)
	}
	respond.JSON(w, http.StatusOK, gen.TemplateList{Items: out})
}

// GetTemplate implements GET /templates/{slug}.
func (h *Handlers) GetTemplate(w http.ResponseWriter, r *http.Request, slug string) {
	t, err := h.templates.Get(r.Context(), slug)
	if err != nil {
		respond.DomainError(w, h.log, err)
		return
	}
	respond.JSON(w, http.StatusOK, toGenTemplate(t))
}

func toGenProfile(p profiles.Profile) gen.Profile {
	headers := toGenHeaders(p.CustomHeaders)
	out := gen.Profile{
		Id:            p.ID,
		UserId:        p.UserID,
		Name:          p.Name,
		DeviceType:    gen.DeviceType(p.DeviceType),
		WindowWidth:   p.WindowWidth,
		WindowHeight:  p.WindowHeight,
		UserAgent:     p.UserAgent,
		CountryCode:   p.CountryCode,
		CustomHeaders: headers,
		TemplateSlug:  p.TemplateSlug,
		CreatedAt:     p.CreatedAt.UTC(),
		UpdatedAt:     p.UpdatedAt.UTC(),
	}
	if len(p.Extra) > 0 {
		extra := p.Extra
		out.Extra = &extra
	}
	return out
}

func toGenTemplate(t templates.Template) gen.Template {
	return gen.Template{
		Slug:          t.Slug,
		Name:          t.Name,
		DeviceType:    t.DeviceType,
		WindowWidth:   t.WindowWidth,
		WindowHeight:  t.WindowHeight,
		UserAgent:     t.UserAgent,
		CountryCode:   t.CountryCode,
		CustomHeaders: toGenHeadersFromTemplates(t.CustomHeaders),
	}
}

func toGenHeaders(in []profiles.Header) []gen.Header {
	out := make([]gen.Header, len(in))
	for i, h := range in {
		out[i] = gen.Header{Key: h.Key, Value: h.Value}
	}
	return out
}

func toGenHeadersFromTemplates(in []templates.Header) []gen.Header {
	out := make([]gen.Header, len(in))
	for i, h := range in {
		out[i] = gen.Header{Key: h.Key, Value: h.Value}
	}
	return out
}

func fromGenHeaders(in *[]gen.Header) []profiles.Header {
	if in == nil {
		return nil
	}
	out := make([]profiles.Header, len(*in))
	for i, h := range *in {
		out[i] = profiles.Header{Key: h.Key, Value: h.Value}
	}
	return out
}
