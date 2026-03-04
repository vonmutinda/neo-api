package testutil

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/pkg/password"
	"github.com/vonmutinda/neo/pkg/phone"
)

// MustStartPostgres spins up an ephemeral Postgres 16 container, runs all
// migrations, and returns a connected pool. The container is terminated
// automatically when the test completes.
func MustStartPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pg, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("neobank_test"),
		postgres.WithUsername("neobank"),
		postgres.WithPassword("neobank_test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	host, err := pg.Host(ctx)
	require.NoError(t, err)
	port, err := pg.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)

	t.Setenv("NEO_DATABASE_HOST", host)
	t.Setenv("NEO_DATABASE_PORT", port.Port())
	t.Setenv("NEO_DATABASE_USERNAME", "neobank")
	t.Setenv("NEO_DATABASE_PASSWORD", "neobank_test")
	t.Setenv("NEO_DATABASE_DATABASE", "neobank_test")
	t.Setenv("NEO_DATABASE_DISABLE_TLS", "true")

	connStr := fmt.Sprintf(
		"pgx5://neobank:neobank_test@%s:%s/neobank_test?sslmode=disable",
		host, port.Port(),
	)

	poolConnStr := fmt.Sprintf(
		"postgres://neobank:neobank_test@%s:%s/neobank_test?sslmode=disable",
		host, port.Port(),
	)
	pool, err := pgxpool.New(ctx, poolConnStr)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	RunMigrations(t, connStr)
	return pool
}

// RunMigrations applies all pending migrations using golang-migrate.
func RunMigrations(t *testing.T, connStr string) {
	t.Helper()
	migrationsDir := "file://" + findMigrationsDir()

	m, err := migrate.New(migrationsDir, connStr)
	require.NoError(t, err)
	defer m.Close()

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		require.NoError(t, err)
	}
}

// TruncateAll truncates all application tables in dependency order.
// Call in TearDownTest to reset the database to a clean state between tests.
func TruncateAll(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		TRUNCATE
			sessions,
			telegram_link_tokens,
			invoice_line_items,
			invoices,
			batch_payment_items,
			batch_payments,
			transaction_labels,
			transaction_categories,
			pending_transfers,
			business_documents,
			business_loan_installments,
			business_loans,
			business_credit_profiles,
			tax_pots,
			business_members,
			business_role_permissions,
			business_roles,
			businesses,
			remittance_transfers,
			fee_schedules,
			remittance_providers,
			transfer_daily_totals,
			regulatory_rules,
			payment_requests,
			overdrafts,
			recipients,
			beneficiaries,
			pots,
			account_details,
			currency_balances,
			card_authorizations,
			cards,
			loan_installments,
			loans,
			credit_profiles,
			transaction_receipts,
			idempotency_keys,
			reconciliation_exceptions,
			reconciliation_runs,
			customer_flags,
			audit_log,
			kyc_verifications,
			staff,
			system_config,
			fx_rates,
			users
		CASCADE
	`)
	require.NoError(t, err)
}

// TestUserID is a valid UUID for integration tests. Use when seeding users
// into real Postgres (users.id is UUID type).
const TestUserID = "11111111-1111-1111-1111-111111111111"

// SeedUser inserts a user with sensible defaults.
// id must be a valid UUID (e.g. TestUserID or uuid.NewString()).
func SeedUser(t *testing.T, pool *pgxpool.Pool, id string, p phone.PhoneNumber) *domain.User {
	t.Helper()
	u := &domain.User{
		ID:             id,
		PhoneNumber:    p,
		KYCLevel:       domain.KYCVerified,
		LedgerWalletID: "wallet:" + id,
		AccountType:    domain.AccountTypePersonal,
	}
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, country_code, number, username, password_hash, ledger_wallet_id, kyc_level, account_type)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		u.ID, u.PhoneNumber.CountryCode, u.PhoneNumber.Number, u.Username, nil, u.LedgerWalletID, u.KYCLevel, u.AccountType,
	)
	require.NoError(t, err)
	return u
}

// SeedFrozenUser inserts a frozen user.
func SeedFrozenUser(t *testing.T, pool *pgxpool.Pool, id string, p phone.PhoneNumber, reason string) *domain.User {
	t.Helper()
	u := SeedUser(t, pool, id, p)
	_, err := pool.Exec(context.Background(),
		`UPDATE users SET is_frozen = true, frozen_reason = $2 WHERE id = $1`,
		id, reason,
	)
	require.NoError(t, err)
	u.IsFrozen = true
	u.FrozenReason = &reason
	return u
}

// SeedCurrencyBalance inserts an active currency balance for a user.
func SeedCurrencyBalance(t *testing.T, pool *pgxpool.Pool, userID, currency string, isPrimary bool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO currency_balances (user_id, currency_code, is_primary)
		 VALUES ($1, $2, $3) RETURNING id`,
		userID, currency, isPrimary,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// SeedCreditProfile inserts a credit profile for a user.
func SeedCreditProfile(t *testing.T, pool *pgxpool.Pool, userID string, trustScore int, limitCents int64) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO credit_profiles (user_id, trust_score, approved_limit_cents, last_calculated_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET trust_score = $2, approved_limit_cents = $3`,
		userID, trustScore, limitCents,
	)
	require.NoError(t, err)
}

// SeedBeneficiary inserts a beneficiary for a user.
func SeedBeneficiary(t *testing.T, pool *pgxpool.Pool, userID, name string, rel domain.BeneficiaryRelType) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO beneficiaries (user_id, full_name, relationship)
		 VALUES ($1, $2, $3) RETURNING id`,
		userID, name, string(rel),
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// SeedRecipient inserts a neo_user recipient for a user and returns the recipient ID.
func SeedRecipient(t *testing.T, pool *pgxpool.Pool, ownerUserID, neoUserID, displayName string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO recipients (owner_user_id, type, display_name, neo_user_id, status)
		 VALUES ($1, 'neo_user', $2, $3, 'active') RETURNING id`,
		ownerUserID, displayName, neoUserID,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// SeedBankRecipient inserts a bank_account recipient for a user and returns the recipient ID.
func SeedBankRecipient(t *testing.T, pool *pgxpool.Pool, ownerUserID, displayName, institutionCode, accountNumber string) string {
	t.Helper()
	bankInfo := domain.LookupBank(institutionCode)
	var bankName *string
	if bankInfo != nil {
		bankName = &bankInfo.Name
	}
	masked := "****" + accountNumber[len(accountNumber)-4:]
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO recipients (owner_user_id, type, display_name, institution_code, bank_name, account_number, account_number_masked, status)
		 VALUES ($1, 'bank_account', $2, $3, $4, $5, $6, 'active') RETURNING id`,
		ownerUserID, displayName, institutionCode, bankName, accountNumber, masked,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// SeedPaymentRequest inserts a pending payment request and returns the ID.
func SeedPaymentRequest(t *testing.T, pool *pgxpool.Pool, requesterID, payerID string, payerPhone phone.PhoneNumber, amountCents int64, currency string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO payment_requests (requester_id, payer_id, payer_country_code, payer_number,
		 amount_cents, currency_code, narration, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, 'test request', NOW() + INTERVAL '30 days')
		 RETURNING id`,
		requesterID, payerID, payerPhone.CountryCode, payerPhone.Number, amountCents, currency,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// SeedPot inserts a pot for a user.
func SeedPot(t *testing.T, pool *pgxpool.Pool, userID, name, currency string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO pots (user_id, name, currency_code)
		 VALUES ($1, $2, $3) RETURNING id`,
		userID, name, currency,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// SeedStaff inserts a staff member with a PBKDF2-hashed password.
func SeedStaff(t *testing.T, pool *pgxpool.Pool, id, email, pw string, role domain.StaffRole) *domain.Staff {
	t.Helper()
	hash := password.GeneratePasswordHash(pw)
	s := &domain.Staff{
		ID:           id,
		Email:        email,
		FullName:     "Test Staff",
		Role:         role,
		Department:   "engineering",
		PasswordHash: hash,
		IsActive:     true,
	}
	_, err := pool.Exec(context.Background(),
		`INSERT INTO staff (id, email, full_name, role, department, password_hash, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		s.ID, s.Email, s.FullName, s.Role, s.Department, s.PasswordHash, s.IsActive,
	)
	require.NoError(t, err)
	return s
}

// SeedCard inserts a card for a user.
func SeedCard(t *testing.T, pool *pgxpool.Pool, userID string, status domain.CardStatus) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO cards (user_id, tokenized_pan, last_four, expiry_month, expiry_year, type, status,
		 allow_online, allow_contactless, allow_atm, allow_international,
		 daily_limit_cents, monthly_limit_cents, per_txn_limit_cents)
		 VALUES ($1, $2, '1234', 12, 2030, 'virtual', $3,
		 true, true, true, true, 100000000, 500000000, 50000000)
		 RETURNING id`,
		userID, "token-"+userID, status,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// SeedSystemConfig inserts a system config key-value pair.
func SeedSystemConfig(t *testing.T, pool *pgxpool.Pool, key string, value []byte) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO system_config (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = $2`,
		key, value,
	)
	require.NoError(t, err)
}

// SeedFlag inserts a customer flag.
func SeedFlag(t *testing.T, pool *pgxpool.Pool, userID, flagType string, severity domain.FlagSeverity) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO customer_flags (user_id, flag_type, severity, description)
		 VALUES ($1, $2, $3, 'test flag') RETURNING id`,
		userID, flagType, severity,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// SeedBusinessSystemRoles re-inserts the system business roles and their
// permissions that are normally seeded by migration 000011. Call after
// TruncateAll when tests depend on system roles (e.g. business registration).
func SeedBusinessSystemRoles(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO business_roles (id, business_id, name, description, is_system, is_default) VALUES
			('00000000-0000-0000-0000-000000000001', NULL, 'owner',      'Full control',   TRUE, FALSE),
			('00000000-0000-0000-0000-000000000002', NULL, 'admin',      'Manage ops',      TRUE, FALSE),
			('00000000-0000-0000-0000-000000000003', NULL, 'finance',    'Finance ops',     TRUE, FALSE),
			('00000000-0000-0000-0000-000000000004', NULL, 'accountant', 'Read-only fin',   TRUE, FALSE),
			('00000000-0000-0000-0000-000000000005', NULL, 'viewer',     'Read-only dash',  TRUE, TRUE)
		ON CONFLICT (id) DO NOTHING`)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO business_role_permissions (role_id, permission)
		SELECT '00000000-0000-0000-0000-000000000001', unnest(ARRAY[
			'biz:dashboard:view','biz:balances:view','biz:transactions:view','biz:documents:view',
			'biz:loans:view','biz:transfers:initiate:internal','biz:transfers:initiate:external',
			'biz:transfers:approve','biz:convert:initiate','biz:batch:create','biz:batch:approve',
			'biz:batch:execute','biz:transactions:export','biz:transactions:label',
			'biz:invoices:manage','biz:invoices:view','biz:pots:manage','biz:tax_pots:manage',
			'biz:tax_pots:withdraw','biz:documents:manage','biz:members:manage',
			'biz:roles:manage','biz:settings:manage','biz:loans:apply'])
		ON CONFLICT DO NOTHING`)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO business_role_permissions (role_id, permission)
		SELECT '00000000-0000-0000-0000-000000000005', unnest(ARRAY[
			'biz:dashboard:view','biz:balances:view','biz:transactions:view'])
		ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
}

// SeedTelegramLinkToken inserts a telegram link token for a user and returns the token string.
func SeedTelegramLinkToken(t *testing.T, pool *pgxpool.Pool, userID, token string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO telegram_link_tokens (token, user_id, expires_at)
		 VALUES ($1, $2, NOW() + INTERVAL '1 hour')`,
		token, userID,
	)
	require.NoError(t, err)
}

// SeedCardAuthorization inserts a card authorization record.
func SeedCardAuthorization(t *testing.T, pool *pgxpool.Pool, cardID, rrn, stan, currency string, amountCents int64, status domain.AuthStatus, responseCode, ledgerHoldID *string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO card_authorizations (card_id, retrieval_reference_number, stan, auth_amount_cents, currency, status, response_code, ledger_hold_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		cardID, rrn, stan, amountCents, currency, status, responseCode, ledgerHoldID,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// SeedLoan inserts a loan record and returns the loan ID.
func SeedLoan(t *testing.T, pool *pgxpool.Pool, userID string, principalCents, paidCents int64, status domain.LoanStatus) string {
	t.Helper()
	feeCents := int64(float64(principalCents) * 0.05)
	totalDueCents := principalCents + feeCents
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO loans (user_id, principal_amount_cents, interest_fee_cents, total_due_cents, total_paid_cents,
		 duration_days, due_date, status, ledger_loan_account)
		 VALUES ($1, $2, $3, $4, $5, 30, NOW() + INTERVAL '30 days', $6, 'loan:test-' || gen_random_uuid())
		 RETURNING id`,
		userID, principalCents, feeCents, totalDueCents, paidCents, status,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// SeedLoanInstallment inserts a loan installment record.
func SeedLoanInstallment(t *testing.T, pool *pgxpool.Pool, loanID string, num int, amountCents int64, isPaid bool) string {
	t.Helper()
	var id string
	dueDate := time.Now().AddDate(0, num, 0)
	err := pool.QueryRow(context.Background(),
		`INSERT INTO loan_installments (loan_id, installment_number, amount_due_cents, due_date, is_paid)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		loanID, num, amountCents, dueDate, isPaid,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func findMigrationsDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
}
