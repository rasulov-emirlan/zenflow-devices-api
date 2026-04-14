// Package auth provides transport-agnostic credential verification.
package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

var ErrUnauthorized = errors.New("unauthorized")

type Resolver struct {
	users map[string]string // username -> bcrypt hash
}

func NewResolver(users map[string]string) *Resolver {
	return &Resolver{users: users}
}

// Verify returns the resolved user ID on success. The user ID is distinct
// from the username to allow future decoupling (e.g. stable IDs across renames).
func (r *Resolver) Verify(username, password string) (userID string, err error) {
	hash, ok := r.users[username]
	if !ok {
		// Run bcrypt anyway to avoid leaking user existence via timing.
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinvalidinvalidi"), []byte(password))
		return "", ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", ErrUnauthorized
	}
	return username, nil
}
