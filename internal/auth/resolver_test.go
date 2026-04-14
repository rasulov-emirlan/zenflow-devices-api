package auth

import (
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestResolverVerify(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	r := NewResolver(map[string]string{"alice": string(hash)})

	if uid, err := r.Verify("alice", "secret"); err != nil || uid != "alice" {
		t.Fatalf("valid creds: uid=%q err=%v", uid, err)
	}
	if _, err := r.Verify("alice", "wrong"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("wrong password: err=%v", err)
	}
	if _, err := r.Verify("ghost", "secret"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unknown user: err=%v", err)
	}
}
