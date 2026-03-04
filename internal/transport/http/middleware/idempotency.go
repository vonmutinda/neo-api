package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/httputil"
	"github.com/vonmutinda/neo/pkg/logger"
)

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

const (
	// HeaderIdempotencyKey is the HTTP header clients must set on mutating requests.
	HeaderIdempotencyKey = "Idempotency-Key"

	// HeaderIdempotencyReplayed is set on responses that are replayed from cache.
	HeaderIdempotencyReplayed = "Idempotency-Replayed"
)

// userIDContextKey is the context key for the authenticated user ID.
// Set by the auth middleware upstream.
type userIDContextKey struct{}

// UserIDFromContext extracts the authenticated user ID from the request context.
func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(userIDContextKey{}).(string); ok {
		return v
	}
	return ""
}

// SetUserID returns a new request with the user ID attached to its context.
// This is called by the auth middleware before the idempotency middleware.
func SetUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), userIDContextKey{}, userID)
	return r.WithContext(ctx)
}

// responseRecorder captures the HTTP response written by downstream handlers
// so we can cache it in the idempotency_keys table.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (rec *responseRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *responseRecorder) Write(b []byte) (int, error) {
	rec.body.Write(b)
	return rec.ResponseWriter.Write(b)
}

// Idempotency returns an HTTP middleware that enforces strict idempotency
// on mutating endpoints (POST, PUT, PATCH, DELETE).
//
// The lifecycle:
//
//  1. Extract Idempotency-Key header. Reject if missing.
//  2. Read + buffer the request body, compute SHA-256 hash.
//  3. INSERT ... ON CONFLICT into idempotency_keys (atomic lock).
//  4. Route based on the returned record's status:
//     - completed: replay the cached response immediately.
//     - started + lock not expired: reject (409 -- in-flight).
//     - started + lock expired: re-acquire (crashed handler).
//  5. On first execution: run the downstream handler, capture the response,
//     then UPDATE the idempotency key with the cached result.
func Idempotency(repo repository.IdempotencyRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only enforce on mutating methods.
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			log := logger.FromContext(r.Context())

			// Step 1: Extract and validate the idempotency key.
			idempotencyKey := r.Header.Get(HeaderIdempotencyKey)
			if idempotencyKey == "" {
				httputil.WriteError(w, http.StatusBadRequest, domain.ErrIdempotencyKeyMissing.Error())
				return
			}
			if len(idempotencyKey) > 64 {
				httputil.WriteError(w, http.StatusBadRequest, "idempotency key must be at most 64 characters")
				return
			}

			// Step 2: Extract user ID from auth context. If no user is
			// authenticated (public endpoints like /v1/register), skip
			// idempotency enforcement since the DB schema requires a valid
			// user UUID foreign key.
			userID := UserIDFromContext(r.Context())
			if userID == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Step 3: Buffer the request body so we can hash it AND still pass it downstream.
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				httputil.WriteError(w, http.StatusBadRequest, "failed to read request body")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			// Step 4: Attempt atomic lock acquisition.
			endpoint := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			record, err := repo.AcquireLock(r.Context(), idempotencyKey, userID, endpoint, bodyBytes)
			if err != nil {
				if err == domain.ErrIdempotencyPayloadMismatch {
					log.Warn("idempotency payload mismatch",
						slog.String("key", idempotencyKey),
						slog.String("user_id", userID),
					)
					httputil.WriteError(w, http.StatusUnprocessableEntity, err.Error())
					return
				}
				log.Error("failed to acquire idempotency lock",
					slog.String("key", idempotencyKey),
					slog.String("error", err.Error()),
				)
				httputil.WriteError(w, http.StatusInternalServerError, "idempotency lock failure")
				return
			}

			// Step 5: Route based on the record's current state.
			now := time.Now()

			// Case A: Already completed -- replay the cached response.
			if record.IsCompleted() {
				log.Info("replaying idempotent response",
					slog.String("key", idempotencyKey),
				)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Header().Set(HeaderIdempotencyReplayed, "true")
				code := http.StatusOK
				if record.ResponseCode != nil {
					code = *record.ResponseCode
				}
				w.WriteHeader(code)
				if record.ResponseBody != nil {
					_, _ = w.Write(record.ResponseBody)
				}
				return
			}

			// Case B: In-flight on another goroutine/instance -- reject.
			if record.IsInFlight(now) {
				log.Warn("idempotency key already in flight",
					slog.String("key", idempotencyKey),
					slog.String("user_id", userID),
				)
				httputil.WriteError(w, http.StatusConflict, domain.ErrIdempotencyRequestInFlight.Error())
				return
			}

			// Case C: First execution (or stale lock re-acquisition).
			// Run the actual handler and capture its output.
			recorder := newResponseRecorder(w)
			next.ServeHTTP(recorder, r)

			// Step 6: Cache the response for future replays.
			responseBody := recorder.body.Bytes()
			if len(responseBody) == 0 {
				responseBody = nil
			}

			if err := repo.MarkCompleted(r.Context(), idempotencyKey, recorder.statusCode, json.RawMessage(responseBody)); err != nil {
				// The response has already been sent to the client.
				// We log the failure but cannot change the outcome.
				log.Error("failed to cache idempotency response",
					slog.String("key", idempotencyKey),
					slog.String("error", err.Error()),
				)
			}
		})
	}
}
