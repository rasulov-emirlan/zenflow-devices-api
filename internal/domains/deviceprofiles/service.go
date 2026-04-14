package deviceprofiles

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/logging"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/metrics"
)

// TemplateLookup is the narrow seam into the templates domain — the device
// profile service only needs Get, so it depends on an interface rather than
// the concrete service, which keeps testing and future splits simple.
type TemplateLookup interface {
	Get(ctx context.Context, slug string) (templates.Template, error)
}

type Service struct {
	repo      Repo
	templates TemplateLookup
	now       func() time.Time
	newID     func() string
}

func NewService(repo Repo, tmpl TemplateLookup) *Service {
	return &Service{
		repo:      repo,
		templates: tmpl,
		now:       time.Now,
		newID:     func() string { return uuid.NewString() },
	}
}

func (s *Service) Create(ctx context.Context, userID string, in Input) (DeviceProfile, error) {
	lg := logging.LoggerFromCtx(ctx)
	if userID == "" {
		metrics.DeviceProfilesValidationErrorsTotal.WithLabelValues("user_id").Inc()
		return DeviceProfile{}, fmt.Errorf("%w: user_id required", ErrInvalidInput)
	}
	if in.TemplateSlug != nil {
		t, err := s.templates.Get(ctx, *in.TemplateSlug)
		if err != nil {
			outcome := "error"
			if errors.Is(err, templates.ErrNotFound) {
				outcome = "miss"
			}
			metrics.TemplateLookupsTotal.WithLabelValues(outcome).Inc()
			lg.WarnContext(ctx, "device profile: template lookup failed",
				slog.String("slug", *in.TemplateSlug),
				slog.String("err", err.Error()),
			)
			return DeviceProfile{}, fmt.Errorf("%w: %w", ErrTemplate, err)
		}
		metrics.TemplateLookupsTotal.WithLabelValues("hit").Inc()
		applyTemplate(&in, t)
	}
	p := DeviceProfile{
		ID:            s.newID(),
		UserID:        userID,
		Name:          in.Name,
		DeviceType:    in.DeviceType,
		WindowWidth:   in.WindowWidth,
		WindowHeight:  in.WindowHeight,
		UserAgent:     in.UserAgent,
		CountryCode:   in.CountryCode,
		CustomHeaders: in.CustomHeaders,
		Extra:         in.Extra,
		TemplateSlug:  in.TemplateSlug,
		CreatedAt:     s.now(),
		UpdatedAt:     s.now(),
	}
	if err := p.Validate(); err != nil {
		metrics.DeviceProfilesValidationErrorsTotal.WithLabelValues(validationField(err)).Inc()
		lg.WarnContext(ctx, "device profile: validation failed", slog.String("err", err.Error()))
		return DeviceProfile{}, err
	}
	if err := s.repo.Insert(ctx, p); err != nil {
		if !errors.Is(err, ErrDuplicateName) {
			lg.ErrorContext(ctx, "device profile: insert failed", slog.String("err", err.Error()))
		}
		return DeviceProfile{}, err
	}
	metrics.DeviceProfilesCreatedTotal.Inc()
	lg.DebugContext(ctx, "device profile: created", slog.String("id", p.ID))
	return p, nil
}

func (s *Service) Get(ctx context.Context, userID, id string) (DeviceProfile, error) {
	p, err := s.repo.GetByID(ctx, userID, id)
	if err != nil && !errors.Is(err, ErrNotFound) {
		logging.LoggerFromCtx(ctx).ErrorContext(ctx, "device profile: get failed",
			slog.String("id", id), slog.String("err", err.Error()))
	}
	return p, err
}

func (s *Service) List(ctx context.Context, userID string, page Page) ([]DeviceProfile, error) {
	out, err := s.repo.ListByUser(ctx, userID, page.Normalize())
	if err != nil {
		logging.LoggerFromCtx(ctx).ErrorContext(ctx, "device profile: list failed", slog.String("err", err.Error()))
	}
	return out, err
}

func (s *Service) Patch(ctx context.Context, userID, id string, patch Patch) (DeviceProfile, error) {
	lg := logging.LoggerFromCtx(ctx)
	current, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			lg.ErrorContext(ctx, "device profile: patch load failed", slog.String("id", id), slog.String("err", err.Error()))
		}
		return DeviceProfile{}, err
	}
	applyPatch(&current, patch)
	current.UpdatedAt = s.now()
	if err := current.Validate(); err != nil {
		metrics.DeviceProfilesValidationErrorsTotal.WithLabelValues(validationField(err)).Inc()
		lg.WarnContext(ctx, "device profile: patch validation failed", slog.String("err", err.Error()))
		return DeviceProfile{}, err
	}
	if err := s.repo.Update(ctx, current); err != nil {
		if !errors.Is(err, ErrDuplicateName) && !errors.Is(err, ErrNotFound) {
			lg.ErrorContext(ctx, "device profile: update failed", slog.String("err", err.Error()))
		}
		return DeviceProfile{}, err
	}
	lg.DebugContext(ctx, "device profile: patched", slog.String("id", id))
	return current, nil
}

func (s *Service) Delete(ctx context.Context, userID, id string) error {
	err := s.repo.Delete(ctx, userID, id)
	if err != nil && !errors.Is(err, ErrNotFound) {
		logging.LoggerFromCtx(ctx).ErrorContext(ctx, "device profile: delete failed",
			slog.String("id", id), slog.String("err", err.Error()))
	}
	return err
}

// validationField extracts a best-effort field name from a validation error
// for metric labeling. The domain's Validate messages are not machine-parsed
// here so we fall back to "other" if no known keyword is present; this keeps
// the label set bounded.
func validationField(err error) string {
	if err == nil {
		return "other"
	}
	msg := err.Error()
	for _, f := range []string{"name", "device_type", "window_width", "window_height", "user_agent", "country_code", "user_id"} {
		if containsWord(msg, f) {
			return f
		}
	}
	return "other"
}

func containsWord(s, w string) bool {
	// Small helper to avoid importing strings just for Contains in a hot path.
	n, m := len(s), len(w)
	if m == 0 || n < m {
		return false
	}
	for i := 0; i+m <= n; i++ {
		if s[i:i+m] == w {
			return true
		}
	}
	return false
}

// applyTemplate fills zero-valued Input fields from the template so caller
// overrides always win.
func applyTemplate(in *Input, t templates.Template) {
	if in.Name == "" {
		in.Name = t.Name
	}
	if in.DeviceType == "" {
		in.DeviceType = DeviceType(t.DeviceType)
	}
	if in.WindowWidth == 0 {
		in.WindowWidth = t.WindowWidth
	}
	if in.WindowHeight == 0 {
		in.WindowHeight = t.WindowHeight
	}
	if in.UserAgent == "" {
		in.UserAgent = t.UserAgent
	}
	if in.CountryCode == "" {
		in.CountryCode = t.CountryCode
	}
	if in.CustomHeaders == nil {
		cloned := make([]Header, len(t.CustomHeaders))
		for i, h := range t.CustomHeaders {
			cloned[i] = Header{Key: h.Key, Value: h.Value}
		}
		in.CustomHeaders = cloned
	}
}

func applyPatch(p *DeviceProfile, patch Patch) {
	if patch.Name != nil {
		p.Name = *patch.Name
	}
	if patch.DeviceType != nil {
		p.DeviceType = *patch.DeviceType
	}
	if patch.WindowWidth != nil {
		p.WindowWidth = *patch.WindowWidth
	}
	if patch.WindowHeight != nil {
		p.WindowHeight = *patch.WindowHeight
	}
	if patch.UserAgent != nil {
		p.UserAgent = *patch.UserAgent
	}
	if patch.CountryCode != nil {
		p.CountryCode = *patch.CountryCode
	}
	if patch.CustomHeaders != nil {
		p.CustomHeaders = *patch.CustomHeaders
	}
	if patch.Extra != nil {
		p.Extra = *patch.Extra
	}
}
