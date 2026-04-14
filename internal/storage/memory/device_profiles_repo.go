// Package memory provides in-memory implementations of the domain repo ports,
// used as a reference implementation and in tests.
package memory

import (
	"context"
	"sync"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/deviceprofiles"
)

type DeviceProfilesRepo struct {
	mu       sync.Mutex
	byID     map[string]deviceprofiles.DeviceProfile
	byUserNm map[string]string
}

func NewDeviceProfilesRepo() *DeviceProfilesRepo {
	return &DeviceProfilesRepo{byID: map[string]deviceprofiles.DeviceProfile{}, byUserNm: map[string]string{}}
}

func key(userID, name string) string { return userID + "\x00" + name }

func (r *DeviceProfilesRepo) Insert(_ context.Context, p deviceprofiles.DeviceProfile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.byUserNm[key(p.UserID, p.Name)]; dup {
		return deviceprofiles.ErrDuplicateName
	}
	r.byID[p.ID] = p
	r.byUserNm[key(p.UserID, p.Name)] = p.ID
	return nil
}

func (r *DeviceProfilesRepo) GetByID(_ context.Context, userID, id string) (deviceprofiles.DeviceProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byID[id]
	if !ok || p.UserID != userID {
		return deviceprofiles.DeviceProfile{}, deviceprofiles.ErrNotFound
	}
	return p, nil
}

func (r *DeviceProfilesRepo) ListByUser(_ context.Context, userID string, page deviceprofiles.Page) ([]deviceprofiles.DeviceProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []deviceprofiles.DeviceProfile{}
	for _, p := range r.byID {
		if p.UserID == userID {
			out = append(out, p)
		}
	}
	if page.Offset >= len(out) {
		return []deviceprofiles.DeviceProfile{}, nil
	}
	end := page.Offset + page.Limit
	if end > len(out) {
		end = len(out)
	}
	return out[page.Offset:end], nil
}

func (r *DeviceProfilesRepo) Update(_ context.Context, p deviceprofiles.DeviceProfile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.byID[p.ID]
	if !ok || existing.UserID != p.UserID {
		return deviceprofiles.ErrNotFound
	}
	if existing.Name != p.Name {
		if _, dup := r.byUserNm[key(p.UserID, p.Name)]; dup {
			return deviceprofiles.ErrDuplicateName
		}
		delete(r.byUserNm, key(existing.UserID, existing.Name))
		r.byUserNm[key(p.UserID, p.Name)] = p.ID
	}
	r.byID[p.ID] = p
	return nil
}

func (r *DeviceProfilesRepo) Delete(_ context.Context, userID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byID[id]
	if !ok || p.UserID != userID {
		return deviceprofiles.ErrNotFound
	}
	delete(r.byID, id)
	delete(r.byUserNm, key(p.UserID, p.Name))
	return nil
}
