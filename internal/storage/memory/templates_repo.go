package memory

import (
	"context"
	"sync"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
)

type TemplatesRepo struct {
	mu sync.Mutex
	m  map[string]templates.Template
}

func NewTemplatesRepo(seed []templates.Template) *TemplatesRepo {
	r := &TemplatesRepo{m: map[string]templates.Template{}}
	for _, t := range seed {
		r.m[t.Slug] = t
	}
	return r
}

func (r *TemplatesRepo) Get(_ context.Context, slug string) (templates.Template, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.m[slug]
	if !ok {
		return templates.Template{}, templates.ErrNotFound
	}
	return t, nil
}

func (r *TemplatesRepo) List(_ context.Context) ([]templates.Template, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]templates.Template, 0, len(r.m))
	for _, t := range r.m {
		out = append(out, t)
	}
	return out, nil
}
