package deviceprofiles

import "context"

// Repo is the port implemented by storage adapters. Implementations must
// translate backend-specific errors to the domain errors in errors.go so
// upstream layers can match with errors.Is.
type Repo interface {
	Insert(ctx context.Context, p DeviceProfile) error
	GetByID(ctx context.Context, userID, id string) (DeviceProfile, error)
	ListByUser(ctx context.Context, userID string, page Page) ([]DeviceProfile, error)
	Update(ctx context.Context, p DeviceProfile) error
	Delete(ctx context.Context, userID, id string) error
}
