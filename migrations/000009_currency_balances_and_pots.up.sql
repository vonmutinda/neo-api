-- ============================================================================
-- Migration 000009: Currency Infrastructure
-- ============================================================================
-- Currency balances, account details, pots, and supported currency registry.
-- ============================================================================

-- Sequence for generating 10-digit account numbers.
CREATE SEQUENCE account_number_seq START WITH 1000000001 INCREMENT BY 1;

-- ---------------------------------------------------------------------------
-- 1. CURRENCY BALANCES
-- ---------------------------------------------------------------------------
-- Tracks which currencies a user has activated. The Formance ledger handles
-- the actual balance; this table is the source of truth for "which currencies
-- does this user have."
--
-- Soft-delete: deleted_at IS NOT NULL means deactivated. The partial unique
-- index allows re-activation after soft-delete.

CREATE TABLE currency_balances (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    currency_code   VARCHAR(3)  NOT NULL,
    is_primary      BOOLEAN     NOT NULL DEFAULT FALSE,
    fx_source       VARCHAR(30),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,

    CONSTRAINT chk_primary_not_deleted CHECK (NOT (is_primary AND deleted_at IS NOT NULL))
);

CREATE UNIQUE INDEX uq_user_currency_active
    ON currency_balances (user_id, currency_code)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_currency_balances_user_active
    ON currency_balances (user_id)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- 2. ACCOUNT DETAILS
-- ---------------------------------------------------------------------------
-- Per-currency IBAN and banking details. 1:1 with currency_balances for
-- currencies that support account details (ETB, USD, EUR, GBP).

CREATE TABLE account_details (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    currency_balance_id UUID        NOT NULL REFERENCES currency_balances(id) ON DELETE RESTRICT,
    iban                VARCHAR(34) NOT NULL,
    account_number      VARCHAR(20) NOT NULL,
    bank_name           TEXT        NOT NULL DEFAULT 'Neo Bank Ethiopia',
    swift_code          VARCHAR(11) NOT NULL DEFAULT 'NEOBETET',
    routing_number      VARCHAR(20),
    sort_code           VARCHAR(10),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_currency_balance_details UNIQUE (currency_balance_id)
);

CREATE UNIQUE INDEX idx_account_details_iban ON account_details (iban);
CREATE UNIQUE INDEX idx_account_details_account_number ON account_details (account_number);

-- ---------------------------------------------------------------------------
-- 3. POTS (Savings Jars)
-- ---------------------------------------------------------------------------
-- Sub-wallets for organizing money by purpose. Each pot maps to a Formance
-- account: {prefix}:wallets:{walletID}:pot:{potID}

CREATE TABLE pots (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    name            VARCHAR(50) NOT NULL,
    currency_code   VARCHAR(3)  NOT NULL,
    target_cents    BIGINT,
    emoji           VARCHAR(10),
    is_archived     BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at     TIMESTAMPTZ
);

CREATE UNIQUE INDEX uq_user_pot_name_active
    ON pots (user_id, name)
    WHERE NOT is_archived;

CREATE INDEX idx_pots_user_active
    ON pots (user_id)
    WHERE NOT is_archived;

-- ---------------------------------------------------------------------------
-- 4. SUPPORTED CURRENCIES
-- ---------------------------------------------------------------------------
-- Database-backed currency registry. Replaces the hardcoded Go variables in
-- pkg/money/money.go. Admin-manageable via the staff dashboard.

CREATE TABLE supported_currencies (
    code                VARCHAR(3)  PRIMARY KEY,
    name                TEXT        NOT NULL,
    symbol              VARCHAR(10) NOT NULL,
    flag                VARCHAR(5)  NOT NULL DEFAULT '',
    exponent            INT         NOT NULL DEFAULT 2,
    has_account_details BOOLEAN     NOT NULL DEFAULT FALSE,
    is_active           BOOLEAN     NOT NULL DEFAULT TRUE,
    sort_order          INT         NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_supported_currencies_active
    ON supported_currencies (is_active, sort_order);

INSERT INTO supported_currencies (code, name, symbol, flag, exponent, has_account_details, is_active, sort_order) VALUES
    ('ETB', 'Ethiopian Birr', 'Br', 'ET', 2, TRUE,  TRUE, 1),
    ('USD', 'US Dollar',      '$',  'US', 2, TRUE,  TRUE, 2),
    ('EUR', 'Euro',           '€',  'EU', 2, TRUE,  TRUE, 3);
