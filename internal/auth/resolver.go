// Package auth provides transport-agnostic credential verification.
// The resolver knows nothing about HTTP — transport adapters call Verify
// with a username/password pair and receive the authenticated user ID.
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

// Verify returns the user ID on success. The user ID currently equals the
// username — kept as a distinct return so callers don't couple to that.
func (r *Resolver) Verify(username, password string) (userID string, err error) {
	hash, ok := r.users[username]
	if !ok {
		// Still run bcrypt to avoid user-enumeration via timing.
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinvalidinvalidi"), []byte(password))
		return "", ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", ErrUnauthorized
	}
	return username, nil
}
