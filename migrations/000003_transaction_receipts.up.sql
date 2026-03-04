-- ============================================================================
-- Migration 000003: Transaction Receipts
-- ============================================================================
-- UI-only table for rendering transaction history in the Next.js app.
-- This is NOT the source of truth for financial data (that's Formance).
-- Think of this as a denormalized "read model" optimized for fast UI queries.
-- ============================================================================

CREATE TYPE receipt_type AS ENUM (
    'p2p_send',             -- Internal wallet-to-wallet
    'p2p_receive',          -- Internal wallet-to-wallet (inbound)
    'ethswitch_out',        -- Outbound via EthioPay-IPS
    'ethswitch_in',         -- Inbound via EthioPay-IPS
    'card_purchase',        -- Card POS/online purchase
    'card_atm',             -- Card ATM withdrawal
    'loan_disbursement',    -- Loan credited to wallet
    'loan_repayment',       -- Loan installment debited
    'fee',                  -- Platform fees
    'convert_out',          -- FX conversion debit (source currency)
    'convert_in'            -- FX conversion credit (target currency)
);

CREATE TYPE receipt_status AS ENUM ('pending', 'completed', 'failed', 'reversed');

CREATE TABLE transaction_receipts (
    id                      UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                 UUID            NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    -- Link to the authoritative source of truth
    ledger_transaction_id   TEXT            NOT NULL,              -- Formance transaction ID
    ethswitch_reference     TEXT,                                  -- EthSwitch global trace ID (if routed via IPS)
    idempotency_key         UUID,                                 -- Links back to idempotency_keys.key

    -- Display data
    type                    receipt_type    NOT NULL,
    status                  receipt_status  NOT NULL DEFAULT 'pending',
    amount_cents            BIGINT          NOT NULL,              -- Always ETB in smallest unit (cents)
    currency                VARCHAR(3)      NOT NULL DEFAULT 'ETB',

    -- Counterparty info (for UI display)
    counterparty_name       TEXT,                                  -- e.g. "Abebe Bikila" or "Telebirr"
    counterparty_phone      TEXT,                                  -- E.164 format
    counterparty_country_code VARCHAR(7),                          -- ITU calling code (nullable)
    counterparty_institution TEXT,                                 -- e.g. "TELEBIRR", "CBE"
    narration               TEXT,                                  -- User-provided note

    -- FX compliance (NBE Clause 8/15)
    purpose                 VARCHAR(30),
    beneficiary_id          UUID,

    -- Fee tracking
    fee_cents               BIGINT          NOT NULL DEFAULT 0,
    fee_breakdown           JSONB,

    created_at              TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_receipts_counterparty_phone CHECK (
        (counterparty_phone IS NULL AND counterparty_country_code IS NULL) OR
        (counterparty_phone IS NOT NULL AND counterparty_country_code IS NOT NULL)
    )
);

-- Primary query: "Show me my recent transactions"
CREATE INDEX idx_receipts_user_created  ON transaction_receipts (user_id, created_at DESC);
-- For reconciliation lookups
CREATE INDEX idx_receipts_ethswitch_ref ON transaction_receipts (ethswitch_reference)
    WHERE ethswitch_reference IS NOT NULL;
-- For linking back from idempotency
CREATE INDEX idx_receipts_idempotency   ON transaction_receipts (idempotency_key)
    WHERE idempotency_key IS NOT NULL;
-- For status-based filtering
CREATE INDEX idx_receipts_status        ON transaction_receipts (status) WHERE status = 'pending';
