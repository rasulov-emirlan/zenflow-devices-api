package profiles

import "context"

// Repo is the port implemented by storage adapters.
// Implementations must translate backend-specific errors to the domain
// errors declared in errors.go (ErrNotFound, ErrDuplicateName, ...).
type Repo interface {
	Insert(ctx context.Context, p Profile) error
	GetByID(ctx context.Context, userID, id string) (Profile, error)
	ListByUser(ctx context.Context, userID string, page Page) ([]Profile, error)
	Update(ctx context.Context, p Profile) error
	Delete(ctx context.Context, userID, id string) error
}
