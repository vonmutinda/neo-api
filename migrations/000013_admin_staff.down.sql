-- Reverse migration 000013: Admin Staff, Customer Flags, System Config

DROP TABLE IF EXISTS system_config;
DROP TABLE IF EXISTS customer_flags;
DROP TABLE IF EXISTS staff;

DROP TYPE IF EXISTS flag_severity;
DROP TYPE IF EXISTS staff_role;
