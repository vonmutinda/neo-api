-- ============================================================================
-- Migration 000011: Business Accounts
-- ============================================================================
-- Introduces business entities, database-driven RBAC, batch payments,
-- transaction labeling, tax pots, invoicing, document management,
-- business lending, and transfer approval workflows.
-- ============================================================================

-- ---------------------------------------------------------------------------
-- 1. ENUM TYPES
-- ---------------------------------------------------------------------------

CREATE TYPE business_status AS ENUM ('pending_verification', 'active', 'suspended', 'deactivated');
CREATE TYPE industry_category AS ENUM (
    'retail', 'wholesale', 'manufacturing', 'agriculture', 'technology',
    'healthcare', 'education', 'construction', 'transport', 'hospitality',
    'financial_services', 'import_export', 'professional_services',
    'non_profit', 'other'
);
CREATE TYPE tax_type AS ENUM (
    'vat', 'income_tax', 'withholding_tax', 'pension',
    'excise', 'custom_duty', 'other'
);
CREATE TYPE batch_status AS ENUM ('draft', 'approved', 'processing', 'completed', 'partial', 'failed');
CREATE TYPE batch_item_status AS ENUM ('pending', 'processing', 'completed', 'failed');
CREATE TYPE invoice_status AS ENUM ('draft', 'sent', 'viewed', 'partially_paid', 'paid', 'overdue', 'cancelled');
CREATE TYPE document_type AS ENUM (
    'trade_license', 'tin_certificate', 'memorandum', 'articles_of_association',
    'bank_statement', 'tax_return', 'contract', 'invoice_attachment',
    'receipt', 'id_document', 'other'
);
CREATE TYPE pending_transfer_status AS ENUM ('pending', 'approved', 'rejected', 'expired');

-- NOTE: users.account_type column has been folded into 000001 (CREATE TABLE users)
-- since we are in dev phase and can drop + re-run all migrations.

-- ---------------------------------------------------------------------------
-- 2. BUSINESSES
-- ---------------------------------------------------------------------------

CREATE TABLE businesses (
    id                      UUID              PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id           UUID              NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    name                    TEXT              NOT NULL,
    trade_name              TEXT,
    tin_number              TEXT              UNIQUE NOT NULL,
    trade_license_number    TEXT              UNIQUE NOT NULL,
    industry_category       industry_category NOT NULL,
    industry_sub_category   TEXT,
    registration_date       DATE,
    address                 TEXT,
    city                    TEXT,
    sub_city                TEXT,
    woreda                  TEXT,
    country_code            VARCHAR(7)        NOT NULL,            -- ITU calling code without '+'
    number                  TEXT              NOT NULL,
    email                   TEXT,
    website                 TEXT,
    status                  business_status   NOT NULL DEFAULT 'pending_verification',
    ledger_wallet_id        TEXT              UNIQUE NOT NULL,
    kyb_level               SMALLINT          NOT NULL DEFAULT 0,
    is_frozen               BOOLEAN           NOT NULL DEFAULT FALSE,
    frozen_reason           TEXT,
    frozen_at               TIMESTAMPTZ,
    created_at              TIMESTAMPTZ       NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ       NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_businesses_phone CHECK (number <> '' AND country_code <> '')
);

CREATE INDEX idx_businesses_owner ON businesses (owner_user_id);
CREATE INDEX idx_businesses_tin ON businesses (tin_number);
CREATE INDEX idx_businesses_industry ON businesses (industry_category);
CREATE INDEX idx_businesses_status ON businesses (status);

-- ---------------------------------------------------------------------------
-- 4. DATABASE-DRIVEN RBAC
-- ---------------------------------------------------------------------------

CREATE TABLE business_roles (
    id                              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id                     UUID        REFERENCES businesses(id),
    name                            VARCHAR(50) NOT NULL,
    description                     TEXT,
    is_system                       BOOLEAN     NOT NULL DEFAULT FALSE,
    is_default                      BOOLEAN     NOT NULL DEFAULT FALSE,
    max_transfer_cents              BIGINT,
    daily_transfer_limit_cents      BIGINT,
    requires_approval_above_cents   BIGINT,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (business_id, name)
);

CREATE TABLE business_role_permissions (
    role_id     UUID            NOT NULL REFERENCES business_roles(id) ON DELETE CASCADE,
    permission  VARCHAR(100)    NOT NULL,
    PRIMARY KEY (role_id, permission)
);

-- Seed system default roles (business_id = NULL, is_system = true)
INSERT INTO business_roles (id, business_id, name, description, is_system, is_default) VALUES
    ('00000000-0000-0000-0000-000000000001', NULL, 'owner',      'Full control of the business',      TRUE, FALSE),
    ('00000000-0000-0000-0000-000000000002', NULL, 'admin',      'Manage members and operations',     TRUE, FALSE),
    ('00000000-0000-0000-0000-000000000003', NULL, 'finance',    'Financial operations',              TRUE, FALSE),
    ('00000000-0000-0000-0000-000000000004', NULL, 'accountant', 'Read-only financial access',        TRUE, FALSE),
    ('00000000-0000-0000-0000-000000000005', NULL, 'viewer',     'Read-only dashboard access',        TRUE, TRUE);

-- Owner: all 25 permissions
INSERT INTO business_role_permissions (role_id, permission) VALUES
    ('00000000-0000-0000-0000-000000000001', 'biz:dashboard:view'),
    ('00000000-0000-0000-0000-000000000001', 'biz:balances:view'),
    ('00000000-0000-0000-0000-000000000001', 'biz:transactions:view'),
    ('00000000-0000-0000-0000-000000000001', 'biz:documents:view'),
    ('00000000-0000-0000-0000-000000000001', 'biz:loans:view'),
    ('00000000-0000-0000-0000-000000000001', 'biz:transfers:initiate:internal'),
    ('00000000-0000-0000-0000-000000000001', 'biz:transfers:initiate:external'),
    ('00000000-0000-0000-0000-000000000001', 'biz:transfers:approve'),
    ('00000000-0000-0000-0000-000000000001', 'biz:convert:initiate'),
    ('00000000-0000-0000-0000-000000000001', 'biz:batch:create'),
    ('00000000-0000-0000-0000-000000000001', 'biz:batch:approve'),
    ('00000000-0000-0000-0000-000000000001', 'biz:batch:execute'),
    ('00000000-0000-0000-0000-000000000001', 'biz:transactions:export'),
    ('00000000-0000-0000-0000-000000000001', 'biz:transactions:label'),
    ('00000000-0000-0000-0000-000000000001', 'biz:invoices:manage'),
    ('00000000-0000-0000-0000-000000000001', 'biz:invoices:view'),
    ('00000000-0000-0000-0000-000000000001', 'biz:pots:manage'),
    ('00000000-0000-0000-0000-000000000001', 'biz:tax_pots:manage'),
    ('00000000-0000-0000-0000-000000000001', 'biz:tax_pots:withdraw'),
    ('00000000-0000-0000-0000-000000000001', 'biz:documents:manage'),
    ('00000000-0000-0000-0000-000000000001', 'biz:members:manage'),
    ('00000000-0000-0000-0000-000000000001', 'biz:roles:manage'),
    ('00000000-0000-0000-0000-000000000001', 'biz:settings:manage'),
    ('00000000-0000-0000-0000-000000000001', 'biz:loans:apply');

-- Admin: all except biz:roles:manage
INSERT INTO business_role_permissions (role_id, permission) VALUES
    ('00000000-0000-0000-0000-000000000002', 'biz:dashboard:view'),
    ('00000000-0000-0000-0000-000000000002', 'biz:balances:view'),
    ('00000000-0000-0000-0000-000000000002', 'biz:transactions:view'),
    ('00000000-0000-0000-0000-000000000002', 'biz:documents:view'),
    ('00000000-0000-0000-0000-000000000002', 'biz:loans:view'),
    ('00000000-0000-0000-0000-000000000002', 'biz:transfers:initiate:internal'),
    ('00000000-0000-0000-0000-000000000002', 'biz:transfers:initiate:external'),
    ('00000000-0000-0000-0000-000000000002', 'biz:transfers:approve'),
    ('00000000-0000-0000-0000-000000000002', 'biz:convert:initiate'),
    ('00000000-0000-0000-0000-000000000002', 'biz:batch:create'),
    ('00000000-0000-0000-0000-000000000002', 'biz:batch:approve'),
    ('00000000-0000-0000-0000-000000000002', 'biz:batch:execute'),
    ('00000000-0000-0000-0000-000000000002', 'biz:transactions:export'),
    ('00000000-0000-0000-0000-000000000002', 'biz:transactions:label'),
    ('00000000-0000-0000-0000-000000000002', 'biz:invoices:manage'),
    ('00000000-0000-0000-0000-000000000002', 'biz:invoices:view'),
    ('00000000-0000-0000-0000-000000000002', 'biz:pots:manage'),
    ('00000000-0000-0000-0000-000000000002', 'biz:tax_pots:manage'),
    ('00000000-0000-0000-0000-000000000002', 'biz:tax_pots:withdraw'),
    ('00000000-0000-0000-0000-000000000002', 'biz:documents:manage'),
    ('00000000-0000-0000-0000-000000000002', 'biz:members:manage'),
    ('00000000-0000-0000-0000-000000000002', 'biz:settings:manage'),
    ('00000000-0000-0000-0000-000000000002', 'biz:loans:apply');

-- Finance: transfers, batch, convert, invoices, export
INSERT INTO business_role_permissions (role_id, permission) VALUES
    ('00000000-0000-0000-0000-000000000003', 'biz:dashboard:view'),
    ('00000000-0000-0000-0000-000000000003', 'biz:balances:view'),
    ('00000000-0000-0000-0000-000000000003', 'biz:transactions:view'),
    ('00000000-0000-0000-0000-000000000003', 'biz:transfers:initiate:internal'),
    ('00000000-0000-0000-0000-000000000003', 'biz:transfers:initiate:external'),
    ('00000000-0000-0000-0000-000000000003', 'biz:convert:initiate'),
    ('00000000-0000-0000-0000-000000000003', 'biz:batch:create'),
    ('00000000-0000-0000-0000-000000000003', 'biz:batch:execute'),
    ('00000000-0000-0000-0000-000000000003', 'biz:transactions:export'),
    ('00000000-0000-0000-0000-000000000003', 'biz:invoices:manage'),
    ('00000000-0000-0000-0000-000000000003', 'biz:invoices:view');

-- Accountant: read-only + export, label, tax pots, view invoices
INSERT INTO business_role_permissions (role_id, permission) VALUES
    ('00000000-0000-0000-0000-000000000004', 'biz:dashboard:view'),
    ('00000000-0000-0000-0000-000000000004', 'biz:balances:view'),
    ('00000000-0000-0000-0000-000000000004', 'biz:transactions:view'),
    ('00000000-0000-0000-0000-000000000004', 'biz:documents:view'),
    ('00000000-0000-0000-0000-000000000004', 'biz:loans:view'),
    ('00000000-0000-0000-0000-000000000004', 'biz:transactions:export'),
    ('00000000-0000-0000-0000-000000000004', 'biz:transactions:label'),
    ('00000000-0000-0000-0000-000000000004', 'biz:tax_pots:manage'),
    ('00000000-0000-0000-0000-000000000004', 'biz:invoices:view');

-- Viewer: read-only dashboard and balances
INSERT INTO business_role_permissions (role_id, permission) VALUES
    ('00000000-0000-0000-0000-000000000005', 'biz:dashboard:view'),
    ('00000000-0000-0000-0000-000000000005', 'biz:balances:view'),
    ('00000000-0000-0000-0000-000000000005', 'biz:transactions:view'),
    ('00000000-0000-0000-0000-000000000005', 'biz:documents:view'),
    ('00000000-0000-0000-0000-000000000005', 'biz:loans:view');

-- ---------------------------------------------------------------------------
-- 5. BUSINESS MEMBERS
-- ---------------------------------------------------------------------------

CREATE TABLE business_members (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id     UUID            NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    user_id         UUID            NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    role_id         UUID            NOT NULL REFERENCES business_roles(id),
    title           TEXT,
    invited_by      UUID            NOT NULL REFERENCES users(id),
    joined_at       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    is_active       BOOLEAN         NOT NULL DEFAULT TRUE,
    removed_at      TIMESTAMPTZ,
    removed_by      UUID            REFERENCES users(id),
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_business_member UNIQUE (business_id, user_id)
);

CREATE INDEX idx_business_members_user ON business_members (user_id) WHERE is_active;
CREATE INDEX idx_business_members_business ON business_members (business_id) WHERE is_active;
CREATE INDEX idx_business_members_role ON business_members (role_id);

-- ---------------------------------------------------------------------------
-- 6. PENDING TRANSFERS (Approval Workflow)
-- ---------------------------------------------------------------------------

CREATE TABLE pending_transfers (
    id                  UUID                    PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id         UUID                    NOT NULL REFERENCES businesses(id),
    initiated_by        UUID                    NOT NULL REFERENCES users(id),
    transfer_type       VARCHAR(20)             NOT NULL,
    amount_cents        BIGINT                  NOT NULL CHECK (amount_cents > 0),
    currency_code       VARCHAR(3)              NOT NULL DEFAULT 'ETB',
    recipient_info      JSONB                   NOT NULL,
    status              pending_transfer_status NOT NULL DEFAULT 'pending',
    reason              TEXT,
    approved_by         UUID                    REFERENCES users(id),
    approved_at         TIMESTAMPTZ,
    rejected_by         UUID                    REFERENCES users(id),
    rejected_at         TIMESTAMPTZ,
    expires_at          TIMESTAMPTZ             NOT NULL,
    created_at          TIMESTAMPTZ             NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ             NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pending_transfers_business ON pending_transfers (business_id, status) WHERE status = 'pending';
CREATE INDEX idx_pending_transfers_initiator ON pending_transfers (initiated_by, created_at DESC);
CREATE INDEX idx_pending_transfers_expiry ON pending_transfers (expires_at) WHERE status = 'pending';

-- ---------------------------------------------------------------------------
-- 7. TRANSACTION CATEGORIES AND LABELS
-- ---------------------------------------------------------------------------

CREATE TABLE transaction_categories (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id     UUID        REFERENCES businesses(id),
    name            TEXT        NOT NULL,
    color           VARCHAR(7),
    icon            VARCHAR(20),
    is_system       BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed system-defined categories
INSERT INTO transaction_categories (id, business_id, name, color, icon, is_system) VALUES
    (gen_random_uuid(), NULL, 'Salary',                '#4CAF50', 'banknote',   TRUE),
    (gen_random_uuid(), NULL, 'Rent',                  '#FF9800', 'home',       TRUE),
    (gen_random_uuid(), NULL, 'Utilities',             '#2196F3', 'zap',        TRUE),
    (gen_random_uuid(), NULL, 'Supplies',              '#9C27B0', 'package',    TRUE),
    (gen_random_uuid(), NULL, 'Travel',                '#00BCD4', 'plane',      TRUE),
    (gen_random_uuid(), NULL, 'Marketing',             '#E91E63', 'megaphone',  TRUE),
    (gen_random_uuid(), NULL, 'Professional Services', '#795548', 'briefcase',  TRUE),
    (gen_random_uuid(), NULL, 'Taxes',                 '#F44336', 'receipt',    TRUE),
    (gen_random_uuid(), NULL, 'Insurance',             '#607D8B', 'shield',     TRUE),
    (gen_random_uuid(), NULL, 'Loan Repayment',        '#FF5722', 'credit-card',TRUE),
    (gen_random_uuid(), NULL, 'Revenue',               '#8BC34A', 'trending-up',TRUE),
    (gen_random_uuid(), NULL, 'FX Conversion',         '#3F51B5', 'repeat',     TRUE),
    (gen_random_uuid(), NULL, 'Transfer',              '#009688', 'send',       TRUE),
    (gen_random_uuid(), NULL, 'Other',                 '#9E9E9E', 'tag',        TRUE);

CREATE TABLE transaction_labels (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id      UUID        NOT NULL REFERENCES transaction_receipts(id),
    category_id         UUID        REFERENCES transaction_categories(id),
    custom_label        TEXT,
    notes               TEXT,
    tagged_by           UUID        NOT NULL REFERENCES users(id),
    tax_deductible      BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tx_labels_transaction ON transaction_labels (transaction_id);
CREATE INDEX idx_tx_labels_category ON transaction_labels (category_id);
CREATE INDEX idx_tx_labels_tax ON transaction_labels (tax_deductible) WHERE tax_deductible;

-- ---------------------------------------------------------------------------
-- 8. TAX POTS
-- ---------------------------------------------------------------------------

CREATE TABLE tax_pots (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id         UUID        NOT NULL REFERENCES businesses(id),
    pot_id              UUID        NOT NULL REFERENCES pots(id),
    tax_type            tax_type    NOT NULL,
    auto_sweep_percent  NUMERIC(5,2),
    due_date            DATE,
    notes               TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_business_tax_type UNIQUE (business_id, tax_type)
);

-- ---------------------------------------------------------------------------
-- 9. BATCH PAYMENTS
-- ---------------------------------------------------------------------------

CREATE TABLE batch_payments (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id     UUID            NOT NULL REFERENCES businesses(id),
    name            TEXT            NOT NULL,
    total_cents     BIGINT          NOT NULL,
    currency_code   VARCHAR(3)      NOT NULL DEFAULT 'ETB',
    item_count      INT             NOT NULL,
    status          batch_status    NOT NULL DEFAULT 'draft',
    initiated_by    UUID            NOT NULL REFERENCES users(id),
    approved_by     UUID            REFERENCES users(id),
    approved_at     TIMESTAMPTZ,
    processed_at    TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE TABLE batch_payment_items (
    id                  UUID                PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id            UUID                NOT NULL REFERENCES batch_payments(id),
    recipient_name      TEXT                NOT NULL,
    recipient_phone     TEXT,
    recipient_country_code VARCHAR(7),                             -- ITU calling code (nullable)
    recipient_bank      TEXT,
    recipient_account   TEXT,
    amount_cents        BIGINT              NOT NULL CHECK (amount_cents > 0),
    narration           TEXT,
    category_id         UUID                REFERENCES transaction_categories(id),
    status              batch_item_status   NOT NULL DEFAULT 'pending',
    transaction_id      UUID                REFERENCES transaction_receipts(id),
    error_message       TEXT,
    created_at          TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ         NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_batch_items_recipient_phone CHECK (
        (recipient_phone IS NULL AND recipient_country_code IS NULL) OR
        (recipient_phone IS NOT NULL AND recipient_country_code IS NOT NULL)
    )
);

CREATE INDEX idx_batch_items_batch ON batch_payment_items (batch_id);

-- ---------------------------------------------------------------------------
-- 10. INVOICES
-- ---------------------------------------------------------------------------

CREATE TABLE invoices (
    id                  UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id         UUID            NOT NULL REFERENCES businesses(id),
    invoice_number      TEXT            UNIQUE NOT NULL,
    customer_name       TEXT            NOT NULL,
    customer_phone      TEXT,
    customer_country_code VARCHAR(7),                              -- ITU calling code (nullable)
    customer_email      TEXT,
    customer_user_id    UUID            REFERENCES users(id),
    currency_code       VARCHAR(3)      NOT NULL DEFAULT 'ETB',
    subtotal_cents      BIGINT          NOT NULL,
    tax_cents           BIGINT          NOT NULL DEFAULT 0,
    total_cents         BIGINT          NOT NULL,
    paid_cents          BIGINT          NOT NULL DEFAULT 0,
    status              invoice_status  NOT NULL DEFAULT 'draft',
    issue_date          DATE            NOT NULL,
    due_date            DATE            NOT NULL,
    notes               TEXT,
    payment_link        TEXT,
    created_by          UUID            NOT NULL REFERENCES users(id),
    created_at          TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_invoices_customer_phone CHECK (
        (customer_phone IS NULL AND customer_country_code IS NULL) OR
        (customer_phone IS NOT NULL AND customer_country_code IS NOT NULL)
    )
);

CREATE TABLE invoice_line_items (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id      UUID        NOT NULL REFERENCES invoices(id),
    description     TEXT        NOT NULL,
    quantity        NUMERIC(10,2)  NOT NULL DEFAULT 1,
    unit_price_cents BIGINT     NOT NULL,
    total_cents     BIGINT      NOT NULL,
    category_id     UUID        REFERENCES transaction_categories(id),
    sort_order      INT         NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invoices_business ON invoices (business_id, created_at DESC);
CREATE INDEX idx_invoices_status ON invoices (status) WHERE status NOT IN ('paid', 'cancelled');
CREATE INDEX idx_invoices_customer ON invoices (customer_user_id) WHERE customer_user_id IS NOT NULL;
CREATE INDEX idx_invoice_items_invoice ON invoice_line_items (invoice_id);

-- ---------------------------------------------------------------------------
-- 11. BUSINESS DOCUMENTS
-- ---------------------------------------------------------------------------

CREATE TABLE business_documents (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id     UUID            NOT NULL REFERENCES businesses(id),
    name            TEXT            NOT NULL,
    document_type   document_type   NOT NULL,
    file_key        TEXT            NOT NULL,
    file_size_bytes BIGINT          NOT NULL,
    mime_type       TEXT            NOT NULL,
    uploaded_by     UUID            NOT NULL REFERENCES users(id),
    description     TEXT,
    tags            TEXT[],
    is_archived     BOOLEAN         NOT NULL DEFAULT FALSE,
    expires_at      DATE,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_documents_business ON business_documents (business_id) WHERE NOT is_archived;
CREATE INDEX idx_documents_type ON business_documents (business_id, document_type);
CREATE INDEX idx_documents_tags ON business_documents USING GIN (tags);

-- ---------------------------------------------------------------------------
-- 12. BUSINESS LENDING
-- ---------------------------------------------------------------------------

CREATE TABLE business_credit_profiles (
    business_id                 UUID    PRIMARY KEY REFERENCES businesses(id),
    trust_score                 INT     NOT NULL DEFAULT 300 CHECK (trust_score >= 300 AND trust_score <= 1000),
    approved_limit_cents        BIGINT  NOT NULL DEFAULT 0,
    avg_monthly_revenue_cents   BIGINT  NOT NULL DEFAULT 0,
    avg_monthly_expenses_cents  BIGINT  NOT NULL DEFAULT 0,
    cash_flow_score             INT     NOT NULL DEFAULT 0,
    time_in_business_months     INT     NOT NULL DEFAULT 0,
    industry_risk_score         INT     NOT NULL DEFAULT 50,
    total_loans_repaid          INT     NOT NULL DEFAULT 0,
    late_payments_count         INT     NOT NULL DEFAULT 0,
    current_outstanding_cents   BIGINT  NOT NULL DEFAULT 0,
    collateral_value_cents      BIGINT  NOT NULL DEFAULT 0,
    is_nbe_blacklisted          BOOLEAN NOT NULL DEFAULT FALSE,
    last_calculated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE business_loans (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id             UUID        NOT NULL REFERENCES businesses(id),
    principal_amount_cents  BIGINT      NOT NULL CHECK (principal_amount_cents > 0),
    interest_fee_cents      BIGINT      NOT NULL CHECK (interest_fee_cents >= 0),
    total_due_cents         BIGINT      NOT NULL,
    total_paid_cents        BIGINT      NOT NULL DEFAULT 0,
    duration_days           INT         NOT NULL CHECK (duration_days > 0),
    disbursed_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    due_date                TIMESTAMPTZ NOT NULL,
    status                  loan_status NOT NULL DEFAULT 'active',
    days_past_due           INT         NOT NULL DEFAULT 0,
    purpose                 TEXT,
    collateral_description  TEXT,
    ledger_loan_account     TEXT        NOT NULL,
    ledger_disbursement_tx  TEXT,
    applied_by              UUID        NOT NULL REFERENCES users(id),
    approved_by             UUID        REFERENCES users(id),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE business_loan_installments (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    loan_id             UUID        NOT NULL REFERENCES business_loans(id),
    installment_number  SMALLINT    NOT NULL,
    amount_due_cents    BIGINT      NOT NULL,
    amount_paid_cents   BIGINT      NOT NULL DEFAULT 0,
    due_date            TIMESTAMPTZ NOT NULL,
    is_paid             BOOLEAN     NOT NULL DEFAULT FALSE,
    paid_at             TIMESTAMPTZ,
    ledger_repayment_tx TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (loan_id, installment_number)
);
