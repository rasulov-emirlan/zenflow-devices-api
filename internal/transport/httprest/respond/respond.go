// Package respond centralizes JSON response and error writing for the HTTP transport.
package respond

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/deviceprofiles"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/templates"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/logging"
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
// 4xx responses are logged at Warn and 5xx at Error; the fallback log uses the
// request-scoped logger in ctx so request_id / trace_id are attached.
func DomainError(w http.ResponseWriter, log *slog.Logger, err error) {
	DomainErrorCtx(context.Background(), w, log, err)
}

// DomainErrorCtx is the ctx-aware variant; handlers should prefer this so the
// request-scoped logger (with request_id, trace_id, route) is used.
func DomainErrorCtx(ctx context.Context, w http.ResponseWriter, log *slog.Logger, err error) {
	lg := logging.LoggerFromCtx(ctx)
	if lg == slog.Default() && log != nil {
		lg = log
	}
	switch {
	case errors.Is(err, deviceprofiles.ErrInvalidInput):
		lg.WarnContext(ctx, "domain: invalid input", slog.String("err", err.Error()))
		Error(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, deviceprofiles.ErrDuplicateName):
		lg.WarnContext(ctx, "domain: duplicate name")
		Error(w, http.StatusConflict, "duplicate_name", "a device profile with this name already exists")
	case errors.Is(err, deviceprofiles.ErrNotFound), errors.Is(err, templates.ErrNotFound):
		lg.WarnContext(ctx, "domain: not found")
		Error(w, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, deviceprofiles.ErrTemplate):
		lg.WarnContext(ctx, "domain: template error", slog.String("err", err.Error()))
		Error(w, http.StatusBadRequest, "template_error", err.Error())
	default:
		lg.ErrorContext(ctx, "unhandled error", slog.String("err", err.Error()))
		Error(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
}
