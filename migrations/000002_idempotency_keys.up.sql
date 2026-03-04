-- ============================================================================
-- Migration 000002: Idempotency Keys
-- ============================================================================
-- Strict idempotency enforcement for all mutating API endpoints.
-- Uses INSERT ... ON CONFLICT to atomically lock requests and prevent
-- double-spending on network retries.
--
-- Lifecycle:
--   1. Client sends request with Idempotency-Key header (UUID).
--   2. Middleware INSERTs with status='started'. ON CONFLICT returns existing row.
--   3. If status='completed', replay cached response immediately.
--   4. If status='started' and row is recent, reject (409 Conflict -- in-flight).
--   5. On handler completion, UPDATE status='completed' + cache response.
--   6. Background job purges keys older than 48 hours.
-- ============================================================================

CREATE TYPE idempotency_status AS ENUM ('started', 'completed', 'failed');

CREATE TABLE idempotency_keys (
    key             UUID                PRIMARY KEY,               -- Client-generated UUID
    user_id         UUID                NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    -- SHA-256 hash of the canonical request payload.
    -- If a user retries the same key but changes the amount from 50 to 500, we reject it.
    request_hash    TEXT                NOT NULL,

    -- HTTP endpoint this key was used for (aids debugging)
    endpoint        TEXT                NOT NULL,                  -- e.g. 'POST /v1/transfers/outbound'

    status          idempotency_status  NOT NULL DEFAULT 'started',

    -- Cached HTTP response for safe replays
    response_code   SMALLINT,
    response_body   JSONB,

    created_at      TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ         NOT NULL DEFAULT NOW(),

    -- Lock expiry: if a request crashes mid-flight, the lock becomes stale.
    -- Background cleanup treats 'started' keys older than this as failed.
    lock_expires_at TIMESTAMPTZ         NOT NULL DEFAULT (NOW() + INTERVAL '5 minutes')
);

-- For background cleanup job (purge keys older than 48h)
CREATE INDEX idx_idempotency_created    ON idempotency_keys (created_at);
-- For detecting stale locks
CREATE INDEX idx_idempotency_stale      ON idempotency_keys (status, lock_expires_at)
    WHERE status = 'started';
-- For user-scoped queries
CREATE INDEX idx_idempotency_user       ON idempotency_keys (user_id);
