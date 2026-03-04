-- ============================================================================
-- Migration 000005: Lending (Credit Profiles, Loans, Installments)
-- ============================================================================
-- Micro-credit system for Ethiopian MSMEs and individual working capital.
--
-- Trust Score Model:
--   Base score: 300 (floor). Max: 1000.
--   Pillar 1: Cash Flow Velocity     (max +400 pts) -- from Formance tx history
--   Pillar 2: Account Stability      (max +200 pts) -- average balance retention
--   Pillar 3: Repayment History      (penalty: -50 per late payment)
--
-- Loan Lifecycle:
--   1. User applies → system checks CreditProfile.approved_limit_cents
--   2. NBE blacklist check (Fayda ID against credit registry)
--   3. Disbursement via Formance: @system:loan_capital → @wallet:<user_id>
--   4. Installments created in loan_installments table
--   5. Auto-sweep worker attempts repayment on due dates
--   6. Late → in_arrears. 90+ days → defaulted (reported to NBE CRB)
-- ============================================================================

-- ---------------------------------------------------------------------------
-- 1. CREDIT PROFILES (One per user, recalculated weekly)
-- ---------------------------------------------------------------------------

CREATE TABLE credit_profiles (
    user_id                 UUID    PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,

    -- Trust Score (300-1000 range)
    trust_score             INT     NOT NULL DEFAULT 300
                                    CHECK (trust_score >= 300 AND trust_score <= 1000),

    -- How much they can borrow right now (0 means ineligible)
    approved_limit_cents    BIGINT  NOT NULL DEFAULT 0
                                    CHECK (approved_limit_cents >= 0),

    -- Raw algorithm input data (updated by lending-worker cron)
    avg_monthly_inflow_cents    BIGINT      NOT NULL DEFAULT 0,
    avg_monthly_balance_cents   BIGINT      NOT NULL DEFAULT 0,
    active_days_per_month       SMALLINT    NOT NULL DEFAULT 0
                                            CHECK (active_days_per_month >= 0 AND active_days_per_month <= 31),
    total_loans_repaid          INT         NOT NULL DEFAULT 0,
    late_payments_count         INT         NOT NULL DEFAULT 0,
    current_outstanding_cents   BIGINT      NOT NULL DEFAULT 0,    -- Sum of all active loan balances

    -- NBE blacklist status (checked before every disbursement)
    is_nbe_blacklisted          BOOLEAN     NOT NULL DEFAULT FALSE,
    blacklist_checked_at        TIMESTAMPTZ,

    last_calculated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- 2. LOANS
-- ---------------------------------------------------------------------------

CREATE TYPE loan_status AS ENUM ('active', 'in_arrears', 'defaulted', 'repaid', 'written_off');

CREATE TABLE loans (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                 UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    -- Financial terms
    principal_amount_cents  BIGINT      NOT NULL CHECK (principal_amount_cents > 0),
    interest_fee_cents      BIGINT      NOT NULL CHECK (interest_fee_cents >= 0),
    total_due_cents         BIGINT      NOT NULL CHECK (total_due_cents > 0),
    total_paid_cents        BIGINT      NOT NULL DEFAULT 0 CHECK (total_paid_cents >= 0),

    -- Duration
    duration_days           SMALLINT    NOT NULL CHECK (duration_days > 0),  -- 30, 60, or 90 day terms
    disbursed_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    due_date                TIMESTAMPTZ NOT NULL,

    -- Status
    status                  loan_status NOT NULL DEFAULT 'active',
    days_past_due           INT         NOT NULL DEFAULT 0,

    -- Formance Ledger links
    ledger_loan_account     TEXT        NOT NULL,                  -- e.g. '@loan:<loan_id>' in Formance
    ledger_disbursement_tx  TEXT,                                  -- Formance tx ID for the disbursement
    idempotency_key         UUID,                                  -- Links to idempotency_keys

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_loans_user_id      ON loans (user_id);
CREATE INDEX idx_loans_status       ON loans (status) WHERE status IN ('active', 'in_arrears');
CREATE INDEX idx_loans_due_date     ON loans (due_date) WHERE status = 'active';

-- ---------------------------------------------------------------------------
-- 3. LOAN INSTALLMENTS (Repayment Schedule)
-- ---------------------------------------------------------------------------

CREATE TABLE loan_installments (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    loan_id             UUID        NOT NULL REFERENCES loans(id) ON DELETE RESTRICT,

    -- Schedule
    installment_number  SMALLINT    NOT NULL,                      -- 1, 2, 3... for ordered display
    amount_due_cents    BIGINT      NOT NULL CHECK (amount_due_cents > 0),
    amount_paid_cents   BIGINT      NOT NULL DEFAULT 0 CHECK (amount_paid_cents >= 0),
    due_date            TIMESTAMPTZ NOT NULL,

    -- Status
    is_paid             BOOLEAN     NOT NULL DEFAULT FALSE,
    paid_at             TIMESTAMPTZ,

    -- Formance link
    ledger_repayment_tx TEXT,                                      -- Formance tx ID for this payment

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (loan_id, installment_number)
);

CREATE INDEX idx_installments_loan      ON loan_installments (loan_id);
CREATE INDEX idx_installments_due       ON loan_installments (due_date)
    WHERE is_paid = FALSE;
-- For the auto-sweep worker: "find all unpaid installments due today"
CREATE INDEX idx_installments_sweep     ON loan_installments (due_date, is_paid)
    WHERE is_paid = FALSE;
