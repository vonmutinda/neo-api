-- ============================================================================
-- Migration 000004: Cards & Card Authorizations
-- ============================================================================
-- Card metadata and security toggles live in Postgres.
-- Actual card funds (holds, settlements) live in Formance.
--
-- PCI DSS Compliance:
--   - We NEVER store the raw PAN (Primary Account Number).
--   - We store only a tokenized reference and the masked last four digits.
--   - The token-to-PAN mapping lives in the card processor's HSM vault.
-- ============================================================================

CREATE TYPE card_type   AS ENUM ('physical', 'virtual', 'ephemeral');
CREATE TYPE card_status AS ENUM ('active', 'frozen', 'cancelled', 'expired', 'pending_activation');

CREATE TABLE cards (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    -- Tokenized card data (PCI DSS compliant -- no raw PAN ever stored)
    tokenized_pan       TEXT        UNIQUE NOT NULL,               -- Token from card processor HSM
    last_four           VARCHAR(4)  NOT NULL,                      -- For UI display: "•••• 4532"
    expiry_month        SMALLINT    NOT NULL CHECK (expiry_month BETWEEN 1 AND 12),
    expiry_year         SMALLINT    NOT NULL CHECK (expiry_year >= 2024),

    type                card_type   NOT NULL,
    status              card_status NOT NULL DEFAULT 'pending_activation',

    -- Granular security toggles (managed by user via Next.js app)
    allow_online        BOOLEAN     NOT NULL DEFAULT TRUE,
    allow_contactless   BOOLEAN     NOT NULL DEFAULT TRUE,
    allow_atm           BOOLEAN     NOT NULL DEFAULT TRUE,
    allow_international BOOLEAN     NOT NULL DEFAULT FALSE,        -- Default off for Ethiopian domestic cards

    -- Spending limits (enforced at authorization time)
    daily_limit_cents   BIGINT      NOT NULL DEFAULT 1000000,      -- 10,000 ETB default
    monthly_limit_cents BIGINT      NOT NULL DEFAULT 15000000,     -- 150,000 ETB default
    per_txn_limit_cents BIGINT      NOT NULL DEFAULT 500000,       -- 5,000 ETB per transaction default

    -- Formance link
    ledger_card_account TEXT,                                      -- e.g. '@wallet:<user_id>:card' in Formance

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cards_user_id   ON cards (user_id);
CREATE INDEX idx_cards_token     ON cards (tokenized_pan);
CREATE INDEX idx_cards_status    ON cards (status) WHERE status = 'active';

-- ---------------------------------------------------------------------------
-- CARD AUTHORIZATIONS (ISO 8583 Dual-Message System)
-- ---------------------------------------------------------------------------
-- Card networks use a two-step process:
--   1. Authorization (Hold): Merchant requests approval → we hold funds in Formance
--   2. Clearing (Settlement): Merchant presents final amount → we settle the hold
--
-- This table tracks the full lifecycle of each authorization.

CREATE TYPE auth_status AS ENUM (
    'approved',         -- Authorization approved, hold created in Formance
    'declined',         -- Authorization declined (insufficient funds, frozen card, etc.)
    'cleared',          -- Settlement received, hold confirmed in Formance
    'reversed',         -- Full reversal (merchant cancelled)
    'partially_cleared',-- Settlement amount differs from auth amount
    'expired'           -- Auth expired (no settlement within clearing window)
);

CREATE TABLE card_authorizations (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id                     UUID        NOT NULL REFERENCES cards(id) ON DELETE RESTRICT,

    -- ISO 8583 Trace Data (critical for dispute resolution)
    retrieval_reference_number  TEXT        NOT NULL,               -- DE 37: unique per authorization
    stan                        TEXT        NOT NULL,               -- DE 11: System Trace Audit Number
    auth_code                   TEXT,                               -- DE 38: Authorization ID Response (6-char)
    merchant_name               TEXT,
    merchant_id                 TEXT,                               -- DE 42: Card Acceptor ID
    merchant_category_code      TEXT,                               -- DE 18: MCC (e.g. 5411 = Grocery)
    terminal_id                 TEXT,                               -- DE 41: Card Acceptor Terminal ID
    acquiring_institution       TEXT,                               -- DE 32: Acquiring Institution ID

    -- Financials
    auth_amount_cents           BIGINT      NOT NULL,               -- Amount requested at authorization
    settlement_amount_cents     BIGINT,                             -- Final cleared amount (may differ)
    currency                    VARCHAR(3)  NOT NULL DEFAULT 'ETB',

    status                      auth_status NOT NULL DEFAULT 'approved',

    -- FX tracking (super card auto-conversion)
    merchant_currency           VARCHAR(3),
    fx_rate_applied             NUMERIC(18,8),
    fx_from_currency            VARCHAR(3),
    fx_from_amount_cents        BIGINT,

    -- Decline reason (if status = 'declined')
    decline_reason              TEXT,                               -- e.g. 'insufficient_funds', 'card_frozen'
    response_code               VARCHAR(3),                         -- DE 39: ISO 8583 response code

    -- Formance Ledger link
    ledger_hold_id              TEXT,                               -- Hold ID in Formance (for settle/void)

    -- Timestamps
    authorized_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at                  TIMESTAMPTZ,
    reversed_at                 TIMESTAMPTZ,
    expires_at                  TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '7 days'), -- Auth hold window

    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Primary query: "Show me authorizations for this card"
CREATE INDEX idx_auth_card_id       ON card_authorizations (card_id, authorized_at DESC);
-- For settlement matching
CREATE INDEX idx_auth_rrn           ON card_authorizations (retrieval_reference_number);
-- For expiry cleanup
CREATE INDEX idx_auth_expires       ON card_authorizations (expires_at)
    WHERE status = 'approved';
-- For Formance hold reconciliation
CREATE INDEX idx_auth_ledger_hold   ON card_authorizations (ledger_hold_id)
    WHERE ledger_hold_id IS NOT NULL;
