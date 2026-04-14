package httprest

import (
	"errors"
	"net/http"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/profiles"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/httpx"
)

// writeDomainError maps domain errors to HTTP responses.
// Unknown errors become 500 with a generic message — full error goes to logs.
func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, profiles.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, profiles.ErrDuplicateName):
		httpx.WriteError(w, http.StatusConflict, "duplicate_name", "a profile with this name already exists")
	case errors.Is(err, profiles.ErrNotFound), errors.Is(err, templates.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, profiles.ErrTemplate):
		httpx.WriteError(w, http.StatusBadRequest, "template_error", err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
}
