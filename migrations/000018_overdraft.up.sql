CREATE TYPE overdraft_status AS ENUM ('inactive', 'active', 'used', 'suspended');

CREATE TABLE overdrafts (
    id                      UUID              PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                 UUID              NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    limit_cents             BIGINT            NOT NULL DEFAULT 0 CHECK (limit_cents >= 0),
    used_cents              BIGINT            NOT NULL DEFAULT 0 CHECK (used_cents >= 0),
    available_cents         BIGINT            GENERATED ALWAYS AS (limit_cents - used_cents) STORED,
    daily_fee_basis_points  SMALLINT          NOT NULL DEFAULT 15,
    interest_free_days      SMALLINT          NOT NULL DEFAULT 7,
    accrued_fee_cents       BIGINT            NOT NULL DEFAULT 0 CHECK (accrued_fee_cents >= 0),
    status                  overdraft_status  NOT NULL DEFAULT 'inactive',
    overdrawn_since         TIMESTAMPTZ,
    last_fee_accrual_at     TIMESTAMPTZ,
    opted_in_at             TIMESTAMPTZ,
    created_at              TIMESTAMPTZ       NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ       NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_overdrafts_user    ON overdrafts (user_id);
CREATE INDEX idx_overdrafts_used    ON overdrafts (status) WHERE status = 'used';
