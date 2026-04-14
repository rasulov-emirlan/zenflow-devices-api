package deviceprofiles

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
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
	if userID == "" {
		return DeviceProfile{}, fmt.Errorf("%w: user_id required", ErrInvalidInput)
	}
	if in.TemplateSlug != nil {
		t, err := s.templates.Get(ctx, *in.TemplateSlug)
		if err != nil {
			return DeviceProfile{}, fmt.Errorf("%w: %w", ErrTemplate, err)
		}
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
		return DeviceProfile{}, err
	}
	if err := s.repo.Insert(ctx, p); err != nil {
		return DeviceProfile{}, err
	}
	return p, nil
}

func (s *Service) Get(ctx context.Context, userID, id string) (DeviceProfile, error) {
	return s.repo.GetByID(ctx, userID, id)
}

func (s *Service) List(ctx context.Context, userID string, page Page) ([]DeviceProfile, error) {
	return s.repo.ListByUser(ctx, userID, page.Normalize())
}

func (s *Service) Patch(ctx context.Context, userID, id string, patch Patch) (DeviceProfile, error) {
	current, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return DeviceProfile{}, err
	}
	applyPatch(&current, patch)
	current.UpdatedAt = s.now()
	if err := current.Validate(); err != nil {
		return DeviceProfile{}, err
	}
	if err := s.repo.Update(ctx, current); err != nil {
		return DeviceProfile{}, err
	}
	return current, nil
}

func (s *Service) Delete(ctx context.Context, userID, id string) error {
	return s.repo.Delete(ctx, userID, id)
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
