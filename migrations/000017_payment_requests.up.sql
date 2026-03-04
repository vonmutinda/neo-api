-- Add audit actions for payment requests and loan manual repayment
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'payment_request_created';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'payment_request_paid';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'payment_request_declined';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'payment_request_cancelled';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'payment_request_expired';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'payment_request_reminded';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'loan_manual_repayment';

CREATE TABLE payment_requests (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id            UUID NOT NULL REFERENCES users(id),
    payer_id                UUID REFERENCES users(id),
    payer_country_code      VARCHAR(7) NOT NULL,
    payer_number            TEXT NOT NULL,
    amount_cents            BIGINT NOT NULL CHECK (amount_cents > 0),
    currency_code           VARCHAR(3) NOT NULL DEFAULT 'ETB',
    narration               TEXT NOT NULL DEFAULT '',
    status                  TEXT NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending','paid','declined','cancelled','expired')),
    transaction_id          UUID,
    decline_reason          TEXT,
    reminder_count          INT NOT NULL DEFAULT 0,
    last_reminded_at        TIMESTAMPTZ,
    paid_at                 TIMESTAMPTZ,
    declined_at             TIMESTAMPTZ,
    cancelled_at            TIMESTAMPTZ,
    expires_at              TIMESTAMPTZ NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_requests_requester ON payment_requests (requester_id, status, created_at DESC);
CREATE INDEX idx_payment_requests_payer     ON payment_requests (payer_id, status, created_at DESC);
CREATE INDEX idx_payment_requests_phone     ON payment_requests (payer_country_code, payer_number, status);
CREATE INDEX idx_payment_requests_expires   ON payment_requests (status, expires_at)
    WHERE status = 'pending';
