package httputil

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/vonmutinda/neo/internal/domain"
)

// Envelope is the standard JSON response wrapper for all API responses.
type Envelope struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(Envelope{Data: data})
	}
}

// WriteError writes a JSON error response with the given status code.
func WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Error: message})
}

// HandleError inspects err for domain.AppError or well-known sentinel errors
// and writes an appropriate HTTP status code with a sanitized message. The
// full error is logged server-side but never returned to the client.
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		slog.ErrorContext(r.Context(), appErr.Message, slog.Any("error", appErr.Err))
		WriteError(w, appErr.Code, appErr.Message)
		return
	}

	code, msg := domain.ErrToCode(err)
	if code >= 500 {
		slog.ErrorContext(r.Context(), "internal error", slog.Any("error", err))
	}
	WriteError(w, code, msg)
}

// DecodeJSON decodes a JSON request body into the given destination struct.
// Validates Content-Type and returns an error suitable for returning directly
// to the client.
func DecodeJSON(r *http.Request, dst any) error {
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "application/json") {
		return domain.NewAppError(http.StatusUnsupportedMediaType, "Content-Type must be application/json", nil)
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
