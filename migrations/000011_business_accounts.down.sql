DROP TABLE IF EXISTS business_loan_installments;
DROP TABLE IF EXISTS business_loans;
DROP TABLE IF EXISTS business_credit_profiles;
DROP TABLE IF EXISTS business_documents;
DROP TABLE IF EXISTS invoice_line_items;
DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS batch_payment_items;
DROP TABLE IF EXISTS batch_payments;
DROP TABLE IF EXISTS tax_pots;
DROP TABLE IF EXISTS transaction_labels;
DROP TABLE IF EXISTS transaction_categories;
DROP TABLE IF EXISTS pending_transfers;
DROP TABLE IF EXISTS business_members;
DROP TABLE IF EXISTS business_role_permissions;
DROP TABLE IF EXISTS business_roles;
DROP TABLE IF EXISTS businesses;

-- NOTE: users.account_type column is now part of 000001 and dropped with the table.

DROP TYPE IF EXISTS pending_transfer_status;
DROP TYPE IF EXISTS document_type;
DROP TYPE IF EXISTS invoice_status;
DROP TYPE IF EXISTS batch_item_status;
DROP TYPE IF EXISTS batch_status;
DROP TYPE IF EXISTS tax_type;
DROP TYPE IF EXISTS industry_category;
DROP TYPE IF EXISTS business_status;
