-- ============================================================================
-- Migration 000008: System Audit Log
-- ============================================================================
-- NBE requires immutable, append-only audit logs for all sensitive operations.
-- This covers user state changes, compliance actions, and admin overrides.
--
-- This table is INSERT-only. No UPDATEs or DELETEs are permitted.
-- Enforced via application-layer policy and Postgres RLS if needed.
-- ============================================================================

CREATE TYPE audit_action AS ENUM (
    -- User lifecycle
    'user_created',
    'user_frozen',
    'user_unfrozen',
    'user_deleted',

    -- KYC
    'kyc_otp_requested',
    'kyc_verified',
    'kyc_failed',
    'kyc_level_upgraded',

    -- Financial
    'transfer_initiated',
    'transfer_settled',
    'transfer_voided',
    'transfer_failed',

    -- Cards
    'card_issued',
    'card_frozen',
    'card_unfrozen',
    'card_cancelled',
    'card_limit_changed',

    -- Card Authorizations
    'card_auth_approved',
    'card_auth_declined',
    'card_auth_settled',
    'card_auth_reversed',

    -- Lending
    'loan_disbursed',
    'loan_repayment',
    'loan_defaulted',
    'loan_written_off',
    'credit_score_updated',

    -- Reconciliation
    'recon_exception_opened',
    'recon_exception_resolved',

    -- Telegram
    'telegram_bound',
    'telegram_unbound',

    -- P2P
    'p2p_transfer',

    -- Business entity lifecycle
    'business_registered',
    'business_updated',
    'business_verified',
    'business_suspended',
    'business_deactivated',

    -- RBAC role management
    'business_role_created',
    'business_role_updated',
    'business_role_deleted',

    -- Member management
    'business_member_invited',
    'business_member_role_changed',
    'business_member_removed',

    -- Transfer approval workflow
    'business_transfer_approval_requested',
    'business_transfer_approved',
    'business_transfer_rejected',
    'business_transfer_expired',

    -- Batch payments
    'business_batch_created',
    'business_batch_approved',
    'business_batch_processed',

    -- Business lending
    'business_loan_applied',
    'business_loan_disbursed',

    -- FX Rates
    'fx_rate_updated',
    'fx_rate_manual_override',

    -- Regulatory / FX Compliance
    'transfer_blocked',
    'transfer_pending_review',
    'rule_evaluated',
    'rule_updated',
    'fx_conversion',
    'beneficiary_added',
    'beneficiary_removed',

    -- Admin operations
    'admin_note',
    'admin_freeze',
    'admin_unfreeze',
    'admin_kyc_override',
    'admin_loan_writeoff',
    'admin_credit_override',
    'admin_card_issued',
    'admin_card_freeze',
    'admin_card_cancel',
    'admin_txn_reversed',
    'flag_created',
    'flag_resolved',
    'staff_created',
    'staff_deactivated',
    'staff_updated',
    'config_changed',
    'bulk_freeze',
    'recon_assigned',
    'recon_escalated',
    'recon_investigating',

    -- Fees & Pricing
    'fee_schedule_created',
    'fee_schedule_updated',
    'fee_schedule_disabled',
    'fee_collected',
    'remittance_initiated',
    'remittance_completed',
    'remittance_failed',

    -- Legacy
    'admin_override'
);

CREATE TABLE audit_log (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    action          audit_action    NOT NULL,

    -- Who performed the action
    actor_type      TEXT            NOT NULL,       -- 'user', 'system', 'admin', 'cron'
    actor_id        TEXT,                           -- user UUID, admin email, or cron job name

    -- What was affected
    resource_type   TEXT            NOT NULL,       -- 'user', 'card', 'loan', 'transfer', etc.
    resource_id     TEXT            NOT NULL,       -- UUID of the affected resource

    -- Context
    metadata        JSONB,                          -- Arbitrary context (old_value, new_value, reason, etc.)
    ip_address      INET,                           -- Client IP (NULL for system/cron actions)
    user_agent      TEXT,                           -- HTTP User-Agent header

    -- Regulatory compliance fields (NBE FX Directive)
    regulatory_rule_key VARCHAR(100),
    regulatory_action   VARCHAR(20),
    nbe_reference       VARCHAR(100),

    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

-- Primary query: "Show me all actions on this user"
CREATE INDEX idx_audit_resource     ON audit_log (resource_type, resource_id, created_at DESC);
-- For compliance: "Show me all actions by this admin"
CREATE INDEX idx_audit_actor        ON audit_log (actor_type, actor_id, created_at DESC);
-- For time-range queries
CREATE INDEX idx_audit_created      ON audit_log (created_at DESC);
-- For filtering by action type
CREATE INDEX idx_audit_action       ON audit_log (action, created_at DESC);
