package middleware

import (
	"context"
	"net/http"
	"regexp"

	"github.com/google/uuid"
)

var validRequestID = regexp.MustCompile(`^[a-zA-Z0-9\-]{1,64}$`)

const (
	// HeaderXRequestID is the standard header for request tracing.
	HeaderXRequestID = "X-Request-ID"
)

type requestIDKey struct{}

// RequestID is middleware that reads or generates an X-Request-ID header
// and propagates it through the context and response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(HeaderXRequestID)
		if reqID == "" || !validRequestID.MatchString(reqID) {
			reqID = uuid.NewString()
		}

		// Set on response so clients can correlate
		w.Header().Set(HeaderXRequestID, reqID)

		// Propagate through context
		ctx := context.WithValue(r.Context(), requestIDKey{}, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext extracts the X-Request-ID from the context.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}
