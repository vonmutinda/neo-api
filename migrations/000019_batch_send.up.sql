ALTER TYPE receipt_type ADD VALUE IF NOT EXISTS 'batch_send';
ALTER TABLE transaction_receipts ADD COLUMN IF NOT EXISTS metadata JSONB;
