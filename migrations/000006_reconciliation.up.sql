-- ============================================================================
-- Migration 000006: Reconciliation Exceptions
-- ============================================================================
-- The EOD (End-of-Day) reconciliation engine performs a 3-way match:
--   1. EthSwitch clearing CSV  (what the network says happened)
--   2. Postgres receipts       (what our system recorded)
--   3. Formance ledger         (what our ledger actually moved)
--
-- Any discrepancy creates an exception row here for FinOps investigation.
-- ============================================================================

CREATE TYPE exception_type   AS ENUM (
    'missing_in_ledger',       -- EthSwitch has it, we don't
    'missing_in_ethswitch',    -- We have it, EthSwitch doesn't (rare but critical)
    'amount_mismatch',         -- Amounts differ between systems
    'status_mismatch',         -- Status differs (e.g., we voided but EthSwitch settled)
    'duplicate_reference',     -- Same EthSwitch reference appears multiple times
    'orphaned_hold'            -- Transit hold with no matching settlement or void
);

CREATE TYPE exception_status AS ENUM ('open', 'investigating', 'resolved', 'escalated');

CREATE TABLE reconciliation_exceptions (
    id                              UUID             PRIMARY KEY DEFAULT gen_random_uuid(),

    -- The external reference that triggered this exception
    ethswitch_reference             TEXT             NOT NULL,
    idempotency_key                 UUID,

    -- Exception details
    error_type                      exception_type   NOT NULL,
    ethswitch_reported_amount_cents BIGINT           NOT NULL,
    ledger_reported_amount_cents    BIGINT,                        -- NULL if missing_in_ledger
    postgres_reported_amount_cents  BIGINT,                        -- NULL if not in our receipts
    amount_difference_cents         BIGINT,                        -- Computed: |ethswitch - ledger|

    -- Resolution workflow
    status                          exception_status NOT NULL DEFAULT 'open',
    assigned_to                     TEXT,                           -- FinOps team member
    resolution_notes                TEXT,
    resolution_action               TEXT,                           -- e.g. 'manual_settle', 'void_and_refund'

    -- Reconciliation run metadata
    recon_run_date                  DATE             NOT NULL,      -- Which EOD run found this
    clearing_file_name              TEXT,                           -- Original CSV filename

    created_at                      TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    resolved_at                     TIMESTAMPTZ,
    updated_at                      TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_exceptions_status      ON reconciliation_exceptions (status);
CREATE INDEX idx_exceptions_run_date    ON reconciliation_exceptions (recon_run_date);
CREATE INDEX idx_exceptions_ethref      ON reconciliation_exceptions (ethswitch_reference);
CREATE INDEX idx_exceptions_open        ON reconciliation_exceptions (status, created_at)
    WHERE status IN ('open', 'investigating');

-- ---------------------------------------------------------------------------
-- RECONCILIATION RUN LOG
-- ---------------------------------------------------------------------------
-- One row per EOD reconciliation execution. Tracks success/failure and stats.

CREATE TABLE reconciliation_runs (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    run_date            DATE        NOT NULL UNIQUE,               -- One run per day
    clearing_file_name  TEXT        NOT NULL,

    -- Statistics
    total_records       INT         NOT NULL DEFAULT 0,
    matched_count       INT         NOT NULL DEFAULT 0,
    exception_count     INT         NOT NULL DEFAULT 0,

    -- Execution metadata
    started_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at         TIMESTAMPTZ,
    status              TEXT        NOT NULL DEFAULT 'running',     -- 'running', 'completed', 'failed'
    error_message       TEXT,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_recon_runs_date ON reconciliation_runs (run_date);
