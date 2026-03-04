-- ============================================================================
-- Migration 000015: Fees & Pricing
-- ============================================================================
-- Fee schedules, remittance providers, and remittance transfers.
-- ============================================================================

-- ---------------------------------------------------------------------------
-- 1. FEE SCHEDULES
-- ---------------------------------------------------------------------------
-- Bank-configurable pricing rules. Supports flat fees, percent fees (in basis
-- points), min/max clamping, per-currency and per-channel scoping, and
-- effective date ranges for scheduled price changes.

CREATE TABLE fee_schedules (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL,
    fee_type          TEXT NOT NULL
                          CHECK (fee_type IN ('fx_spread','transfer_flat','transfer_percent','corridor_markup')),
    transaction_type  TEXT NOT NULL
                          CHECK (transaction_type IN ('p2p','ethswitch_out','fx_conversion','card_international','international_transfer')),
    currency          TEXT,
    channel           TEXT,
    flat_amount_cents BIGINT NOT NULL DEFAULT 0 CHECK (flat_amount_cents >= 0),
    percent_bps       INT NOT NULL DEFAULT 0 CHECK (percent_bps >= 0),
    min_fee_cents     BIGINT NOT NULL DEFAULT 0 CHECK (min_fee_cents >= 0),
    max_fee_cents     BIGINT NOT NULL DEFAULT 0 CHECK (max_fee_cents >= 0),
    is_active         BOOLEAN NOT NULL DEFAULT true,
    effective_from    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fee_schedules_lookup
    ON fee_schedules (transaction_type, is_active, effective_from)
    WHERE is_active = true;

CREATE INDEX idx_fee_schedules_currency
    ON fee_schedules (transaction_type, currency, is_active)
    WHERE is_active = true;

-- ---------------------------------------------------------------------------
-- 2. REMITTANCE PROVIDERS & TRANSFERS
-- ---------------------------------------------------------------------------
-- External payment partners for international transfers (Wise, Terrapay, etc.)
-- and a ledger of transfers executed through them.

CREATE TABLE remittance_providers (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    corridors       JSONB NOT NULL DEFAULT '[]',
    api_base_url    TEXT NOT NULL,
    webhook_secret  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE remittance_transfers (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id               UUID NOT NULL REFERENCES users(id),
    provider_id           TEXT NOT NULL REFERENCES remittance_providers(id),
    provider_transfer_id  TEXT,
    quote_id              TEXT NOT NULL,
    source_currency       TEXT NOT NULL,
    target_currency       TEXT NOT NULL,
    source_amount_cents   BIGINT NOT NULL CHECK (source_amount_cents > 0),
    target_amount_cents   BIGINT NOT NULL CHECK (target_amount_cents > 0),
    exchange_rate         DOUBLE PRECISION NOT NULL CHECK (exchange_rate > 0),
    our_fee_cents         BIGINT NOT NULL DEFAULT 0,
    provider_fee_cents    BIGINT NOT NULL DEFAULT 0,
    total_fee_cents       BIGINT NOT NULL DEFAULT 0,
    status                TEXT NOT NULL DEFAULT 'pending'
                              CHECK (status IN ('pending','processing','funds_converted','sent','delivered','failed','cancelled','refunded')),
    recipient_name        TEXT NOT NULL,
    recipient_country     TEXT NOT NULL,
    hold_id               TEXT,
    failure_reason        TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_remittance_transfers_user
    ON remittance_transfers (user_id, created_at DESC);

CREATE INDEX idx_remittance_transfers_status
    ON remittance_transfers (status, updated_at)
    WHERE status NOT IN ('delivered','failed','cancelled','refunded');
