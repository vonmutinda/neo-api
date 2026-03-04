package domain

import (
	"encoding/json"
	"time"
)

// IdempotencyStatus represents the lifecycle state of an idempotent request.
type IdempotencyStatus string

const (
	IdempotencyStarted   IdempotencyStatus = "started"
	IdempotencyCompleted IdempotencyStatus = "completed"
	IdempotencyFailed    IdempotencyStatus = "failed"
)

// IdempotencyRecord represents a single row in the idempotency_keys table.
// It tracks whether a given request has been seen before and, if completed,
// caches the response for safe replay on retries.
type IdempotencyRecord struct {
	Key           string            `json:"key"`
	UserID        string            `json:"userId"`
	RequestHash   string            `json:"requestHash"`
	Endpoint      string            `json:"endpoint"`
	Status        IdempotencyStatus `json:"status"`
	ResponseCode  *int              `json:"responseCode,omitempty"`
	ResponseBody  json.RawMessage   `json:"responseBody,omitempty"`
	LockExpiresAt time.Time         `json:"lockExpiresAt"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

// IsCompleted returns true if this request has already been fully processed.
func (r *IdempotencyRecord) IsCompleted() bool {
	return r.Status == IdempotencyCompleted
}

// IsFailed returns true if this request previously failed.
func (r *IdempotencyRecord) IsFailed() bool {
	return r.Status == IdempotencyFailed
}

// IsInFlight returns true if another goroutine/instance is currently processing
// this request (status=started and the lock has not yet expired).
func (r *IdempotencyRecord) IsInFlight(now time.Time) bool {
	return r.Status == IdempotencyStarted && now.Before(r.LockExpiresAt)
}

// IsLockExpired returns true if the lock has gone stale (the original handler
// crashed or timed out without completing).
func (r *IdempotencyRecord) IsLockExpired(now time.Time) bool {
	return r.Status == IdempotencyStarted && !now.Before(r.LockExpiresAt)
}
