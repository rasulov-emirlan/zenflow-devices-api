package profiles

import "errors"

var (
	ErrNotFound      = errors.New("profile not found")
	ErrDuplicateName = errors.New("profile name already exists")
	ErrInvalidInput  = errors.New("invalid input")
	ErrTemplate      = errors.New("template error")
)
