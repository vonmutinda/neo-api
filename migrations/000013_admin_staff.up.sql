-- ============================================================================
-- Migration 000013: Admin Staff, Customer Flags, System Config
-- ============================================================================
-- Introduces the bank operations admin layer: staff accounts with role-based
-- access control, customer flagging for risk management, and a key-value
-- system configuration table for feature flags and operational limits.
-- ============================================================================

-- ---------------------------------------------------------------------------
-- 1. ENUM TYPES
-- ---------------------------------------------------------------------------

CREATE TYPE staff_role AS ENUM (
    'super_admin',
    'customer_support',
    'customer_support_lead',
    'compliance_officer',
    'lending_officer',
    'reconciliation_analyst',
    'card_operations',
    'treasury',
    'auditor'
);

CREATE TYPE flag_severity AS ENUM ('info', 'warning', 'critical');

-- ---------------------------------------------------------------------------
-- 2. STAFF TABLE
-- ---------------------------------------------------------------------------

CREATE TABLE staff (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT         UNIQUE NOT NULL,
    full_name       TEXT         NOT NULL,
    role            staff_role   NOT NULL,
    department      TEXT         NOT NULL,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    password_hash   TEXT         NOT NULL,
    last_login_at   TIMESTAMPTZ,
    created_by      UUID         REFERENCES staff(id),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deactivated_at  TIMESTAMPTZ
);

CREATE INDEX idx_staff_email ON staff (email) WHERE is_active;
CREATE INDEX idx_staff_role ON staff (role) WHERE is_active;

-- ---------------------------------------------------------------------------
-- 3. CUSTOMER FLAGS TABLE
-- ---------------------------------------------------------------------------

CREATE TABLE customer_flags (
    id              UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID          NOT NULL REFERENCES users(id),
    flag_type       TEXT          NOT NULL,
    severity        flag_severity NOT NULL,
    description     TEXT          NOT NULL,
    created_by      UUID          REFERENCES staff(id),
    is_resolved     BOOLEAN       NOT NULL DEFAULT FALSE,
    resolved_by     UUID          REFERENCES staff(id),
    resolved_at     TIMESTAMPTZ,
    resolution_note TEXT,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_customer_flags_user ON customer_flags (user_id) WHERE NOT is_resolved;
CREATE INDEX idx_customer_flags_open ON customer_flags (severity, created_at DESC) WHERE NOT is_resolved;
CREATE INDEX idx_customer_flags_type ON customer_flags (flag_type) WHERE NOT is_resolved;

-- ---------------------------------------------------------------------------
-- 4. SYSTEM CONFIG TABLE
-- ---------------------------------------------------------------------------

CREATE TABLE system_config (
    key         TEXT        PRIMARY KEY,
    value       JSONB       NOT NULL,
    description TEXT,
    updated_by  UUID        REFERENCES staff(id),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default feature flags
INSERT INTO system_config (key, value, description) VALUES
    ('registrations_enabled',       'true',  'Kill switch for new user sign-ups'),
    ('ethswitch_enabled',           'true',  'Enable/disable external EthSwitch transfers'),
    ('lending_enabled',             'true',  'Enable/disable new loan disbursements'),
    ('p2p_enabled',                 'true',  'Enable/disable P2P transfers'),
    ('business_registration_enabled', 'true', 'Enable/disable new business registration'),
    ('batch_payments_enabled',      'true',  'Enable/disable batch payment processing'),
    ('fx_conversion_enabled',       'true',  'Enable/disable FX currency conversion');

-- Seed default system limits
INSERT INTO system_config (key, value, description) VALUES
    ('max_single_transfer_cents',     '50000000',  'Maximum single transfer amount in cents (500,000 ETB)'),
    ('daily_transfer_limit_cents_kyc1', '7500000', 'Daily transfer limit for KYC level 1 in cents'),
    ('daily_transfer_limit_cents_kyc2', '15000000', 'Daily transfer limit for KYC level 2 in cents'),
    ('daily_transfer_limit_cents_kyc3', '100000000', 'Daily transfer limit for KYC level 3 in cents'),
    ('max_loan_amount_cents',         '50000000',  'Maximum loan principal in cents (500,000 ETB)');
