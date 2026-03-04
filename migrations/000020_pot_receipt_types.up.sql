-- Add receipt types for pot deposit and withdrawal (visible in transactions list).
ALTER TYPE receipt_type ADD VALUE IF NOT EXISTS 'pot_deposit';
ALTER TYPE receipt_type ADD VALUE IF NOT EXISTS 'pot_withdraw';
