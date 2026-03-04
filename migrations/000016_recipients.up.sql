-- Add new audit actions for recipients
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'recipient_saved';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'recipient_favorited';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'recipient_archived';

CREATE TYPE recipient_type   AS ENUM ('neo_user', 'bank_account');
CREATE TYPE recipient_status AS ENUM ('active', 'archived');

CREATE TABLE recipients (
    id                      UUID              PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id           UUID              NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type                    recipient_type    NOT NULL,
    display_name            TEXT              NOT NULL,

    -- Neo user fields (type = neo_user)
    neo_user_id             UUID              REFERENCES users(id),
    country_code            VARCHAR(7),
    number                  TEXT,
    username                TEXT,

    -- Bank account fields (type = bank_account)
    institution_code        TEXT,
    bank_name               TEXT,
    swift_bic               VARCHAR(11),
    account_number          TEXT,
    account_number_masked   TEXT,
    bank_country_code       VARCHAR(2)        DEFAULT 'ET',

    -- Regulatory link
    beneficiary_id          UUID              REFERENCES beneficiaries(id),
    is_beneficiary          BOOLEAN           NOT NULL DEFAULT FALSE,

    -- Usage tracking
    is_favorite             BOOLEAN           NOT NULL DEFAULT FALSE,
    last_used_at            TIMESTAMPTZ,
    last_used_currency      VARCHAR(3),
    transfer_count          INT               NOT NULL DEFAULT 0,

    -- State
    status                  recipient_status  NOT NULL DEFAULT 'active',
    created_at              TIMESTAMPTZ       NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ       NOT NULL DEFAULT NOW()
);

-- One active recipient per owner + Neo user identity
CREATE UNIQUE INDEX uq_recipient_neo_user
    ON recipients (owner_user_id, neo_user_id, status)
    WHERE neo_user_id IS NOT NULL;

-- One active recipient per owner + bank account identity
CREATE UNIQUE INDEX uq_recipient_bank
    ON recipients (owner_user_id, institution_code, account_number, status)
    WHERE institution_code IS NOT NULL AND account_number IS NOT NULL;

-- Primary list query: "Show my recipients, favorites first, then recent"
CREATE INDEX idx_recipients_owner_list
    ON recipients (owner_user_id, status, is_favorite DESC, last_used_at DESC NULLS LAST);

-- Lookup by Neo user (for upsert on transfer)
CREATE INDEX idx_recipients_neo_user
    ON recipients (owner_user_id, neo_user_id) WHERE status = 'active';

-- Bank account typeahead
CREATE INDEX idx_recipients_bank_account
    ON recipients (owner_user_id, institution_code, account_number)
    WHERE status = 'active';

-- Full-text search on display name
CREATE INDEX idx_recipients_name_search
    ON recipients USING gin (to_tsvector('simple', display_name));
