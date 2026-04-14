// Package memory provides in-memory repo implementations used for tests
// and as a reference implementation of the domain ports.
package memory

import (
	"context"
	"sync"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/profiles"
)

type ProfilesRepo struct {
	mu       sync.Mutex
	byID     map[string]profiles.Profile
	byUserNm map[string]string
}

func NewProfilesRepo() *ProfilesRepo {
	return &ProfilesRepo{byID: map[string]profiles.Profile{}, byUserNm: map[string]string{}}
}

func key(userID, name string) string { return userID + "\x00" + name }

func (r *ProfilesRepo) Insert(_ context.Context, p profiles.Profile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.byUserNm[key(p.UserID, p.Name)]; dup {
		return profiles.ErrDuplicateName
	}
	r.byID[p.ID] = p
	r.byUserNm[key(p.UserID, p.Name)] = p.ID
	return nil
}

func (r *ProfilesRepo) GetByID(_ context.Context, userID, id string) (profiles.Profile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byID[id]
	if !ok || p.UserID != userID {
		return profiles.Profile{}, profiles.ErrNotFound
	}
	return p, nil
}

func (r *ProfilesRepo) ListByUser(_ context.Context, userID string, page profiles.Page) ([]profiles.Profile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []profiles.Profile{}
	for _, p := range r.byID {
		if p.UserID == userID {
			out = append(out, p)
		}
	}
	// naive pagination
	if page.Offset >= len(out) {
		return []profiles.Profile{}, nil
	}
	end := page.Offset + page.Limit
	if end > len(out) {
		end = len(out)
	}
	return out[page.Offset:end], nil
}

func (r *ProfilesRepo) Update(_ context.Context, p profiles.Profile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.byID[p.ID]
	if !ok || existing.UserID != p.UserID {
		return profiles.ErrNotFound
	}
	if existing.Name != p.Name {
		if _, dup := r.byUserNm[key(p.UserID, p.Name)]; dup {
			return profiles.ErrDuplicateName
		}
		delete(r.byUserNm, key(existing.UserID, existing.Name))
		r.byUserNm[key(p.UserID, p.Name)] = p.ID
	}
	r.byID[p.ID] = p
	return nil
}

func (r *ProfilesRepo) Delete(_ context.Context, userID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byID[id]
	if !ok || p.UserID != userID {
		return profiles.ErrNotFound
	}
	delete(r.byID, id)
	delete(r.byUserNm, key(p.UserID, p.Name))
	return nil
}
