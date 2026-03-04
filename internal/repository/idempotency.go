package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

// IdempotencyRepository handles all database operations for the idempotency_keys table.
type IdempotencyRepository interface {
	// AcquireLock atomically inserts a new idempotency key or returns the existing one.
	// Uses INSERT ... ON CONFLICT to guarantee atomicity.
	AcquireLock(ctx context.Context, key, userID, endpoint string, payload []byte) (*domain.IdempotencyRecord, error)

	// MarkCompleted transitions a started key to completed and caches the response.
	MarkCompleted(ctx context.Context, key string, responseCode int, responseBody json.RawMessage) error

	// MarkFailed transitions a started key to failed.
	MarkFailed(ctx context.Context, key string) error

	// PurgeExpired deletes idempotency keys older than the given retention period.
	PurgeExpired(ctx context.Context, olderThan time.Duration) (int64, error)
}

// pgIdempotencyRepo is the PostgreSQL implementation of IdempotencyRepository.
type pgIdempotencyRepo struct {
	db DBTX
}

// NewIdempotencyRepository creates a new PostgreSQL-backed idempotency repository.
func NewIdempotencyRepository(db DBTX) IdempotencyRepository {
	return &pgIdempotencyRepo{db: db}
}

// HashPayload computes a deterministic SHA-256 hash of the request payload.
// This is used to detect parameter tampering on retries: same key but different body.
func HashPayload(payload []byte) string {
	h := sha256.Sum256(payload)
	return hex.EncodeToString(h[:])
}

func (r *pgIdempotencyRepo) AcquireLock(ctx context.Context, key, userID, endpoint string, payload []byte) (*domain.IdempotencyRecord, error) {
	requestHash := HashPayload(payload)

	// INSERT ... ON CONFLICT is the core of the atomic idempotency lock.
	//
	// If the key does not exist, it is inserted with status='started'.
	// If it does exist, the ON CONFLICT clause performs a no-op update
	// (just bumps updated_at) so RETURNING can fetch the existing row.
	//
	// This is a single atomic SQL statement -- no TOCTOU race.
	query := `
		INSERT INTO idempotency_keys (key, user_id, request_hash, endpoint, status, lock_expires_at)
		VALUES ($1, $2, $3, $4, 'started', NOW() + INTERVAL '5 minutes')
		ON CONFLICT (key) DO UPDATE
			SET updated_at = NOW()
		RETURNING
			key, user_id, request_hash, endpoint, status,
			response_code, response_body,
			lock_expires_at, created_at, updated_at
	`

	var rec domain.IdempotencyRecord
	err := r.db.QueryRow(ctx, query, key, userID, requestHash, endpoint).Scan(
		&rec.Key,
		&rec.UserID,
		&rec.RequestHash,
		&rec.Endpoint,
		&rec.Status,
		&rec.ResponseCode,
		&rec.ResponseBody,
		&rec.LockExpiresAt,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("acquiring idempotency lock: %w", err)
	}

	// Security check: detect payload tampering.
	// If someone reuses the same idempotency key but changes the request body
	// (e.g., changing transfer amount from 50 to 500), we must reject it.
	if rec.RequestHash != requestHash {
		return nil, domain.ErrIdempotencyPayloadMismatch
	}

	return &rec, nil
}

func (r *pgIdempotencyRepo) MarkCompleted(ctx context.Context, key string, responseCode int, responseBody json.RawMessage) error {
	query := `
		UPDATE idempotency_keys
		SET status = 'completed',
			response_code = $2,
			response_body = $3,
			updated_at = NOW()
		WHERE key = $1 AND status = 'started'
	`

	tag, err := r.db.Exec(ctx, query, key, responseCode, responseBody)
	if err != nil {
		return fmt.Errorf("marking idempotency key completed: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("idempotency key %s: not in 'started' state or does not exist", key)
	}

	return nil
}

func (r *pgIdempotencyRepo) MarkFailed(ctx context.Context, key string) error {
	query := `
		UPDATE idempotency_keys
		SET status = 'failed',
			updated_at = NOW()
		WHERE key = $1 AND status = 'started'
	`

	tag, err := r.db.Exec(ctx, query, key)
	if err != nil {
		return fmt.Errorf("marking idempotency key failed: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("idempotency key %s: not in 'started' state or does not exist", key)
	}

	return nil
}

func (r *pgIdempotencyRepo) PurgeExpired(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		DELETE FROM idempotency_keys
		WHERE created_at < NOW() - $1::interval
	`

	tag, err := r.db.Exec(ctx, query, olderThan.String())
	if err != nil {
		return 0, fmt.Errorf("purging expired idempotency keys: %w", err)
	}

	return tag.RowsAffected(), nil
}

// Verify interface compliance at compile time.
var _ IdempotencyRepository = (*pgIdempotencyRepo)(nil)

// Ensure pgx.Row is used (suppress unused import in edge cases).
var _ pgx.Row = (pgx.Row)(nil)
