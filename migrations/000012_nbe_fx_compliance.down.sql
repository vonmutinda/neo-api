-- Reverse migration 000012: NBE FX Compliance
-- NOTE: ALTER TABLE DROP COLUMN statements removed -- columns are now part of
-- their original CREATE TABLE migrations and dropped when those tables are dropped.

DROP TABLE IF EXISTS beneficiaries;
DROP TABLE IF EXISTS transfer_daily_totals;
DROP TABLE IF EXISTS regulatory_rules;
