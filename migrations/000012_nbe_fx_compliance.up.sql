-- ============================================================================
-- Migration 000012: NBE FX Compliance — Regulatory Rules Engine
-- ============================================================================
-- Implements the configurable regulatory engine for FXD/01/2024 (amended FXD/04/2026).
-- Creates tables for rules, transfer tracking, beneficiaries, and alters
-- existing tables for FX source, spend waterfall, card FX tracking, and audit.

-- 1. REGULATORY RULES
CREATE TABLE regulatory_rules (
    id              UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    key             VARCHAR(100)  NOT NULL,
    description     TEXT          NOT NULL,
    value_type      VARCHAR(20)   NOT NULL,
    value           TEXT          NOT NULL,
    scope           VARCHAR(20)   NOT NULL DEFAULT 'global',
    scope_value     VARCHAR(50)   NOT NULL DEFAULT '',
    nbe_reference   VARCHAR(100)  NOT NULL DEFAULT '',
    effective_from  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    effective_to    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    UNIQUE (key, scope, scope_value, effective_from)
);

CREATE INDEX idx_regulatory_rules_key ON regulatory_rules (key);
CREATE INDEX idx_regulatory_rules_effective ON regulatory_rules (key, effective_from, effective_to);

-- 2. SEED RULES (FXD/01/2024)

-- Clause 8: Outbound remittance cap for FX account holders
INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('outbound_remittance_monthly_cap',
        'Maximum outbound remittance per month for FX account holders',
        'amount_cents', '300000', 'account_type', 'fx_holder',
        'FXD/01/2024 Clause 8');

-- Clause 15: Advance payment for medical/education
INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('advance_payment_medical_education_cap',
        'Max advance payment for medical/education without visa/ticket',
        'amount_cents', '2000000', 'global', '',
        'FXD/01/2024 Clause 15');

-- Clause 5: Minimum FX account opening balance removed
INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('fx_account_min_opening_balance',
        'Minimum balance to open FX savings account (removed by NBE)',
        'amount_cents', '0', 'global', '',
        'FXD/01/2024 Clause 5');

-- Clause 2: International card spend
INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('card_international_spend_enabled',
        'Allow card spend at international (non-ETB) merchants',
        'bool', 'true', 'global', '',
        'FXD/01/2024 Clause 2');

-- Clause 2 + 7: Auto-conversion during card authorization
INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('card_auto_conversion_enabled',
        'Allow automatic ETB-to-FX conversion during card authorization',
        'bool', 'true', 'global', '',
        'FXD/01/2024 Clause 2 + Clause 7');

-- Clause 8: Monthly cap on international card spend
INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('card_international_monthly_cap',
        'Maximum monthly international card spend (in USD cents)',
        'amount_cents', '300000', 'global', '',
        'FXD/01/2024 Clause 8');

-- Clause 3: Family payments from FX account
INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('fx_family_payment_enabled',
        'Whether FX holders can pay for spouse/children education, medical, travel',
        'bool', 'true', 'global', '',
        'FXD/01/2024 Clause 3');

-- Clause 6: Outbound investment
INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('outbound_investment_enabled',
        'Whether outbound investment is allowed (requires NBE case-by-case approval)',
        'bool', 'false', 'global', '',
        'FXD/01/2024 Clause 6');

-- Clause 6: Outbound investment requires NBE approval
INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('outbound_investment_requires_nbe_approval',
        'Whether outbound investment requires NBE case-by-case approval',
        'bool', 'true', 'global', '',
        'FXD/01/2024 Clause 6');

-- FX conversion spread
INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('fx_conversion_spread_percent',
        'Spread applied on top of mid-market rate for FX conversions',
        'percent', '1.5', 'global', '',
        'internal_policy');

-- FX conversion enabled
INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('fx_conversion_enabled',
        'Whether in-app FX conversion is enabled',
        'bool', 'true', 'global', '',
        'FXD/01/2024 Clause 7');

-- Clause 8: Document threshold for outbound remittance
INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('outbound_remittance_doc_threshold',
        'Amount above which supporting documents are required for outbound remittance',
        'amount_cents', '100000', 'global', '',
        'FXD/01/2024 Clause 8');

-- KYC-based daily transfer limits (internal policy)
INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('daily_transfer_limit',
        'Maximum daily transfer amount for KYC Basic users',
        'amount_cents', '7500000', 'kyc_level', '1',
        'internal_policy');

INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('daily_transfer_limit',
        'Maximum daily transfer amount for KYC Verified users',
        'amount_cents', '15000000', 'kyc_level', '2',
        'internal_policy');

INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('daily_transfer_limit',
        'Maximum daily transfer amount for KYC Enhanced users',
        'amount_cents', '100000000', 'kyc_level', '3',
        'internal_policy');

-- Rate staleness threshold
INSERT INTO regulatory_rules (key, description, value_type, value, scope, scope_value, nbe_reference)
VALUES ('fx_rate_staleness_threshold',
        'Maximum age of exchange rate data before conversions are rejected (Go duration)',
        'duration', '24h', 'global', '',
        'internal_policy');

-- 3. TRANSFER DAILY TOTALS (rolling-window counter)
CREATE TABLE transfer_daily_totals (
    user_id      UUID        NOT NULL,
    currency     VARCHAR(3)  NOT NULL,
    direction    VARCHAR(10) NOT NULL,
    date         DATE        NOT NULL DEFAULT CURRENT_DATE,
    total_cents  BIGINT      NOT NULL DEFAULT 0,
    tx_count     INT         NOT NULL DEFAULT 0,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, currency, direction, date)
);

-- 4. BENEFICIARIES (family payments, Clause 3)
CREATE TABLE beneficiaries (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID         NOT NULL REFERENCES users(id),
    full_name    VARCHAR(200) NOT NULL,
    relationship VARCHAR(20)  NOT NULL,
    document_url TEXT,
    is_verified  BOOLEAN      NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX idx_beneficiaries_user ON beneficiaries (user_id) WHERE deleted_at IS NULL;

-- 5. NOTE: Columns previously added via ALTER TABLE have been folded into
-- their original CREATE TABLE migrations (000001, 000003, 000004, 000008,
-- 000009) since we are in dev phase and can drop + re-run all migrations.
