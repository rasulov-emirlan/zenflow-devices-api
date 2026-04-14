// Package respond centralizes JSON response and error writing for the HTTP transport.
package respond

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/deviceprofiles"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
)

const maxBodyBytes = 1 << 20

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, errorBody{Error: code, Message: message})
}

// DecodeBody parses a strict JSON body into dst. Returns an error writable via BadRequest.
func DecodeBody(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("empty request body")
		}
		return err
	}
	if dec.More() {
		return errors.New("body must contain a single JSON object")
	}
	return nil
}

// DomainError maps domain errors to HTTP responses. Unknown errors become 500.
func DomainError(w http.ResponseWriter, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, deviceprofiles.ErrInvalidInput):
		Error(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, deviceprofiles.ErrDuplicateName):
		Error(w, http.StatusConflict, "duplicate_name", "a device profile with this name already exists")
	case errors.Is(err, deviceprofiles.ErrNotFound), errors.Is(err, templates.ErrNotFound):
		Error(w, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, deviceprofiles.ErrTemplate):
		Error(w, http.StatusBadRequest, "template_error", err.Error())
	default:
		if log != nil {
			log.Error("unhandled error", slog.String("err", err.Error()))
		}
		Error(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
}
