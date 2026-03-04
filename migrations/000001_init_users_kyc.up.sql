-- ============================================================================
-- Migration 000001: Users & KYC
-- Ethiopian Neobank -- PostgreSQL State Database
-- ============================================================================
-- This table stores identity and state. It does NOT store financial balances.
-- All authoritative balance data lives in Formance (double-entry ledger).
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ---------------------------------------------------------------------------
-- 1. USERS
-- ---------------------------------------------------------------------------
-- Core identity table. One row per customer. The ledger_wallet_id column is
-- the 1:1 link to the user's primary Formance account (@wallet:<user_id>).

CREATE TABLE users (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    country_code      VARCHAR(7)  NOT NULL,                        -- ITU calling code without '+' (e.g. "251", "1", "44")
    number            TEXT        NOT NULL,                         -- national subscriber number (e.g. "960598761")
    username          VARCHAR(30) UNIQUE,                          -- optional user-chosen handle
    password_hash     TEXT,                                        -- bcrypt hash (NULL until password set)
    fayda_id_number   TEXT        UNIQUE,                          -- Ethiopian National ID (12-digit FIN)

    -- Demographics (populated after Fayda eKYC verification)
    first_name        TEXT,
    middle_name       TEXT,
    last_name         TEXT,
    date_of_birth     DATE,
    gender            VARCHAR(10),
    fayda_photo_url   TEXT,                                        -- S3/R2 URL to the Fayda-sourced portrait

    -- KYC & Compliance
    kyc_level         SMALLINT    NOT NULL DEFAULT 1,              -- 1: Basic (75,000 ETB), 2: Verified (150,000 ETB), 3: Enhanced
    is_frozen         BOOLEAN     NOT NULL DEFAULT FALSE,          -- AML/Compliance lock (blocks all transactions)
    frozen_reason     TEXT,                                        -- Human-readable reason for freeze
    frozen_at         TIMESTAMPTZ,

    -- Formance Ledger Link
    ledger_wallet_id  TEXT        UNIQUE NOT NULL,                 -- e.g. 'wallet:<uuid>' — maps to Formance account

    -- Telegram Binding (nullable until user links via /start <token>)
    telegram_id       BIGINT      UNIQUE,
    telegram_username TEXT,

    -- Account type (personal or business, added for business accounts feature)
    account_type      VARCHAR(20) NOT NULL DEFAULT 'personal',

    -- Spend waterfall preference (super card auto-conversion drain order)
    spend_waterfall_order JSONB   NOT NULL DEFAULT '["merchant_currency","ETB"]',

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_users_phone CHECK (number <> '' AND country_code <> ''),
    CONSTRAINT chk_users_credentials CHECK (password_hash IS NULL OR password_hash <> ''),
    CONSTRAINT uq_users_phone UNIQUE (country_code, number)
);

CREATE INDEX idx_users_phone        ON users (country_code, number);
CREATE UNIQUE INDEX idx_users_username ON users (username) WHERE username IS NOT NULL;
CREATE INDEX idx_users_fayda        ON users (fayda_id_number) WHERE fayda_id_number IS NOT NULL;
CREATE INDEX idx_users_telegram     ON users (telegram_id)     WHERE telegram_id IS NOT NULL;
CREATE INDEX idx_users_kyc_level    ON users (kyc_level);
CREATE INDEX idx_users_is_frozen    ON users (is_frozen)       WHERE is_frozen = TRUE;

-- ---------------------------------------------------------------------------
-- 2. KYC VERIFICATIONS (Audit Trail for NBE Inspectors)
-- ---------------------------------------------------------------------------
-- Every successful Fayda verification creates an immutable row here.
-- NBE requires proof that the user consented to biometric verification.

CREATE TYPE kyc_verification_status AS ENUM ('pending', 'verified', 'failed', 'expired');

CREATE TABLE kyc_verifications (
    id                    UUID                    PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id               UUID                    NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    fayda_fin             TEXT                    NOT NULL,        -- The 12-digit FIN used for this verification
    fayda_transaction_id  TEXT                    NOT NULL,        -- Fayda's transaction ID (proof of consent)

    status                kyc_verification_status NOT NULL DEFAULT 'pending',
    verified_at           TIMESTAMPTZ,
    fayda_expiry_date     DATE,                                   -- ID expiry (Fayda IDs expire after 8 years)

    -- Raw Fayda response (encrypted at rest via column-level encryption or app-level)
    -- Stored for regulatory audits; NOT used for runtime decisions.
    raw_response_hash     TEXT,                                   -- SHA-256 of the encrypted response blob

    created_at            TIMESTAMPTZ             NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_kyc_user_id   ON kyc_verifications (user_id);
CREATE INDEX idx_kyc_status    ON kyc_verifications (status);
CREATE INDEX idx_kyc_fayda_fin ON kyc_verifications (fayda_fin);

-- ---------------------------------------------------------------------------
-- 3. TELEGRAM DEEP-LINK TOKENS
-- ---------------------------------------------------------------------------
-- Short-lived tokens used for /start <token> binding flow.
-- Created when user requests Telegram linking from the app; consumed once.

CREATE TABLE telegram_link_tokens (
    token       TEXT        PRIMARY KEY,                           -- Cryptographically random token
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    consumed    BOOLEAN     NOT NULL DEFAULT FALSE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tg_token_user    ON telegram_link_tokens (user_id);
CREATE INDEX idx_tg_token_expires ON telegram_link_tokens (expires_at) WHERE consumed = FALSE;

-- ---------------------------------------------------------------------------
-- 4. SESSIONS
-- ---------------------------------------------------------------------------
-- Tracks active refresh tokens for user authentication.
-- Each login creates a session; refresh rotates it; logout revokes it.

CREATE TABLE sessions (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token   TEXT        UNIQUE NOT NULL,
    user_agent      TEXT,
    ip_address      TEXT,
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user    ON sessions (user_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_sessions_refresh ON sessions (refresh_token) WHERE revoked_at IS NULL;
CREATE INDEX idx_sessions_expiry  ON sessions (expires_at) WHERE revoked_at IS NULL;
