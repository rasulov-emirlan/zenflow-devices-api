package httprest

import (
	"log/slog"
	"net/http"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/deviceprofiles"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/transport/httprest/gen"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/transport/httprest/respond"
)

// Handlers implements gen.ServerInterface.
type Handlers struct {
	deviceProfiles *deviceprofiles.Service
	templates      *templates.Service
	log            *slog.Logger
}

var _ gen.ServerInterface = (*Handlers)(nil)

// GetHealth implements GET /healthz.
func (h *Handlers) GetHealth(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, gen.HealthResponse{Status: "ok"})
}

// ListDeviceProfiles implements GET /device-profiles.
func (h *Handlers) ListDeviceProfiles(w http.ResponseWriter, r *http.Request, params gen.ListDeviceProfilesParams) {
	page := deviceprofiles.Page{Limit: 50, Offset: 0}
	if params.Limit != nil {
		page.Limit = *params.Limit
	}
	if params.Offset != nil {
		page.Offset = *params.Offset
	}
	items, err := h.deviceProfiles.List(r.Context(), UserID(r.Context()), page)
	if err != nil {
		respond.DomainErrorCtx(r.Context(), w, h.log, err)
		return
	}
	out := make([]gen.DeviceProfile, len(items))
	for i, p := range items {
		out[i] = toGenDeviceProfile(p)
	}
	respond.JSON(w, http.StatusOK, gen.DeviceProfileList{Items: out})
}

// CreateDeviceProfile implements POST /device-profiles.
func (h *Handlers) CreateDeviceProfile(w http.ResponseWriter, r *http.Request) {
	var req gen.CreateDeviceProfileRequest
	if err := respond.DecodeBody(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	in := deviceprofiles.Input{
		Name:          req.Name,
		DeviceType:    deviceprofiles.DeviceType(req.DeviceType),
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
	p, err := h.deviceProfiles.Create(r.Context(), UserID(r.Context()), in)
	if err != nil {
		respond.DomainErrorCtx(r.Context(), w, h.log, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toGenDeviceProfile(p))
}

// GetDeviceProfile implements GET /device-profiles/{id}.
func (h *Handlers) GetDeviceProfile(w http.ResponseWriter, r *http.Request, id gen.DeviceProfileID) {
	p, err := h.deviceProfiles.Get(r.Context(), UserID(r.Context()), id)
	if err != nil {
		respond.DomainErrorCtx(r.Context(), w, h.log, err)
		return
	}
	respond.JSON(w, http.StatusOK, toGenDeviceProfile(p))
}

// PatchDeviceProfile implements PATCH /device-profiles/{id}.
func (h *Handlers) PatchDeviceProfile(w http.ResponseWriter, r *http.Request, id gen.DeviceProfileID) {
	var req gen.PatchDeviceProfileRequest
	if err := respond.DecodeBody(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	patch := deviceprofiles.Patch{
		Name:         req.Name,
		WindowWidth:  req.WindowWidth,
		WindowHeight: req.WindowHeight,
		UserAgent:    req.UserAgent,
		CountryCode:  req.CountryCode,
		Extra:        req.Extra,
	}
	if req.DeviceType != nil {
		dt := deviceprofiles.DeviceType(*req.DeviceType)
		patch.DeviceType = &dt
	}
	if req.CustomHeaders != nil {
		hs := fromGenHeaders(req.CustomHeaders)
		patch.CustomHeaders = &hs
	}
	p, err := h.deviceProfiles.Patch(r.Context(), UserID(r.Context()), id, patch)
	if err != nil {
		respond.DomainErrorCtx(r.Context(), w, h.log, err)
		return
	}
	respond.JSON(w, http.StatusOK, toGenDeviceProfile(p))
}

// DeleteDeviceProfile implements DELETE /device-profiles/{id}.
func (h *Handlers) DeleteDeviceProfile(w http.ResponseWriter, r *http.Request, id gen.DeviceProfileID) {
	if err := h.deviceProfiles.Delete(r.Context(), UserID(r.Context()), id); err != nil {
		respond.DomainErrorCtx(r.Context(), w, h.log, err)
		return
	}
	respond.NoContent(w)
}

// ListTemplates implements GET /templates.
func (h *Handlers) ListTemplates(w http.ResponseWriter, r *http.Request) {
	items, err := h.templates.List(r.Context())
	if err != nil {
		respond.DomainErrorCtx(r.Context(), w, h.log, err)
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
		respond.DomainErrorCtx(r.Context(), w, h.log, err)
		return
	}
	respond.JSON(w, http.StatusOK, toGenTemplate(t))
}

func toGenDeviceProfile(p deviceprofiles.DeviceProfile) gen.DeviceProfile {
	headers := toGenHeaders(p.CustomHeaders)
	out := gen.DeviceProfile{
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

func toGenHeaders(in []deviceprofiles.Header) []gen.Header {
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

func fromGenHeaders(in *[]gen.Header) []deviceprofiles.Header {
	if in == nil {
		return nil
	}
	out := make([]deviceprofiles.Header, len(*in))
	for i, h := range *in {
		out[i] = deviceprofiles.Header{Key: h.Key, Value: h.Value}
	}
	return out
}
