-- ============================================================================
-- Migration 000007: Telegram Command Audit Log
-- ============================================================================
-- Tracks all commands issued through the Telegram bot for audit and rate limiting.
-- The actual user↔telegram binding lives on the users table (telegram_id column).
-- ============================================================================

CREATE TABLE telegram_command_log (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id     BIGINT      NOT NULL,
    user_id         UUID        REFERENCES users(id) ON DELETE SET NULL,

    command         TEXT        NOT NULL,       -- e.g. '/balance', '/start', '/help'
    chat_id         BIGINT      NOT NULL,
    message_id      BIGINT,

    -- Rate limiting metadata
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- For rate limiting: "how many commands has this telegram user sent in the last minute?"
CREATE INDEX idx_tg_cmd_rate ON telegram_command_log (telegram_id, created_at DESC);
