package deviceprofiles

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
)

type fakeRepo struct {
	byID     map[string]DeviceProfile
	byUserNm map[string]string // userID+"\x00"+name -> id
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byID: map[string]DeviceProfile{}, byUserNm: map[string]string{}}
}

func key(userID, name string) string { return userID + "\x00" + name }

func (f *fakeRepo) Insert(_ context.Context, p DeviceProfile) error {
	if _, dup := f.byUserNm[key(p.UserID, p.Name)]; dup {
		return ErrDuplicateName
	}
	f.byID[p.ID] = p
	f.byUserNm[key(p.UserID, p.Name)] = p.ID
	return nil
}

func (f *fakeRepo) GetByID(_ context.Context, userID, id string) (DeviceProfile, error) {
	p, ok := f.byID[id]
	if !ok || p.UserID != userID {
		return DeviceProfile{}, ErrNotFound
	}
	return p, nil
}

func (f *fakeRepo) ListByUser(_ context.Context, userID string, page Page) ([]DeviceProfile, error) {
	out := []DeviceProfile{}
	for _, p := range f.byID {
		if p.UserID == userID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeRepo) Update(_ context.Context, p DeviceProfile) error {
	existing, ok := f.byID[p.ID]
	if !ok || existing.UserID != p.UserID {
		return ErrNotFound
	}
	if existing.Name != p.Name {
		if _, dup := f.byUserNm[key(p.UserID, p.Name)]; dup {
			return ErrDuplicateName
		}
		delete(f.byUserNm, key(existing.UserID, existing.Name))
		f.byUserNm[key(p.UserID, p.Name)] = p.ID
	}
	f.byID[p.ID] = p
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, userID, id string) error {
	p, ok := f.byID[id]
	if !ok || p.UserID != userID {
		return ErrNotFound
	}
	delete(f.byID, id)
	delete(f.byUserNm, key(p.UserID, p.Name))
	return nil
}

type fakeTemplates struct{ m map[string]templates.Template }

func (f *fakeTemplates) Get(_ context.Context, slug string) (templates.Template, error) {
	t, ok := f.m[slug]
	if !ok {
		return templates.Template{}, templates.ErrNotFound
	}
	return t, nil
}

func newService() (*Service, *fakeRepo) {
	repo := newFakeRepo()
	tmpl := &fakeTemplates{m: map[string]templates.Template{
		"iphone-us": {
			Slug: "iphone-us", Name: "iphone", DeviceType: "mobile",
			WindowWidth: 390, WindowHeight: 844,
			UserAgent: "Mozilla/5.0 iPhone", CountryCode: "US",
		},
	}}
	svc := NewService(repo, tmpl)
	svc.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	return svc, repo
}

func validInput() Input {
	return Input{
		Name:         "my-profile",
		DeviceType:   DeviceDesktop,
		WindowWidth:  1920,
		WindowHeight: 1080,
		UserAgent:    "Mozilla/5.0",
		CountryCode:  "US",
	}
}

func TestCreateHappy(t *testing.T) {
	svc, _ := newService()
	p, err := svc.Create(context.Background(), "alice", validInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == "" || p.UserID != "alice" {
		t.Fatalf("unexpected device profile: %+v", p)
	}
}

func TestCreateRequiresUser(t *testing.T) {
	svc, _ := newService()
	_, err := svc.Create(context.Background(), "", validInput())
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestCreateValidation(t *testing.T) {
	svc, _ := newService()
	in := validInput()
	in.CountryCode = "usa"
	_, err := svc.Create(context.Background(), "alice", in)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestCreateDuplicateName(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()
	if _, err := svc.Create(ctx, "alice", validInput()); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Create(ctx, "alice", validInput())
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("want ErrDuplicateName, got %v", err)
	}
}

func TestCreateFromTemplate(t *testing.T) {
	svc, _ := newService()
	slug := "iphone-us"
	p, err := svc.Create(context.Background(), "alice", Input{
		Name:         "my-iphone",
		TemplateSlug: &slug,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.DeviceType != DeviceMobile || p.WindowWidth != 390 || p.CountryCode != "US" {
		t.Fatalf("template not applied: %+v", p)
	}
	if !strings.Contains(p.UserAgent, "iPhone") {
		t.Fatalf("UA: %s", p.UserAgent)
	}
}

func TestCreateTemplateOverride(t *testing.T) {
	svc, _ := newService()
	slug := "iphone-us"
	p, err := svc.Create(context.Background(), "alice", Input{
		Name:         "my-iphone-de",
		CountryCode:  "DE",
		TemplateSlug: &slug,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.CountryCode != "DE" {
		t.Fatalf("override lost: got %q", p.CountryCode)
	}
}

func TestCreateUnknownTemplate(t *testing.T) {
	svc, _ := newService()
	slug := "nope"
	_, err := svc.Create(context.Background(), "alice", Input{Name: "x", TemplateSlug: &slug})
	if !errors.Is(err, ErrTemplate) {
		t.Fatalf("want ErrTemplate, got %v", err)
	}
}

func TestGetEnforcesOwnership(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()
	p, _ := svc.Create(ctx, "alice", validInput())
	_, err := svc.Get(ctx, "bob", p.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPatchUpdates(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()
	p, _ := svc.Create(ctx, "alice", validInput())
	w := 800
	updated, err := svc.Patch(ctx, "alice", p.ID, Patch{WindowWidth: &w})
	if err != nil {
		t.Fatalf("Patch: %v", err)
	}
	if updated.WindowWidth != 800 {
		t.Fatalf("width = %d", updated.WindowWidth)
	}
}

func TestPatchValidation(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()
	p, _ := svc.Create(ctx, "alice", validInput())
	bad := "usa"
	_, err := svc.Patch(ctx, "alice", p.ID, Patch{CountryCode: &bad})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestDeleteOwnership(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()
	p, _ := svc.Create(ctx, "alice", validInput())
	if err := svc.Delete(ctx, "bob", p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := svc.Delete(ctx, "alice", p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}
