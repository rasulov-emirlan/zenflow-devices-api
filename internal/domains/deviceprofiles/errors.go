package deviceprofiles

import "errors"

var (
	ErrNotFound      = errors.New("device profile not found")
	ErrDuplicateName = errors.New("device profile name already exists")
	ErrInvalidInput  = errors.New("invalid input")
	ErrTemplate      = errors.New("template error")
)
