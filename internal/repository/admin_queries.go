package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/jackc/pgx/v5"
)

// AdminQueryRepository provides cross-user read queries for the admin layer.
// These are intentionally separate from the per-user repositories to avoid
// bloating existing interfaces with admin-only methods.
type AdminQueryRepository interface {
	// Users
	ListUsers(ctx context.Context, f domain.UserFilter) (*domain.PaginatedResult[domain.User], error)
	CountUsersByKYCLevel(ctx context.Context) (map[domain.KYCLevel]int64, error)

	// Transactions
	ListTransactions(ctx context.Context, f domain.TransactionFilter) (*domain.PaginatedResult[domain.TransactionReceipt], error)
	GetPairedReceipt(ctx context.Context, ledgerTxID string, excludeID string) (*domain.TransactionReceipt, error)
	SumTransactionsByStatusAndType(ctx context.Context, from, to time.Time) ([]TransactionAggregate, error)

	// Loans
	ListLoans(ctx context.Context, f domain.LoanFilter) (*domain.PaginatedResult[domain.Loan], error)
	LoanBookSummary(ctx context.Context) (*LoanBookSummary, error)
	ListCreditProfiles(ctx context.Context, f domain.CreditProfileFilter) (*domain.PaginatedResult[domain.CreditProfile], error)

	// Cards
	ListCards(ctx context.Context, f domain.CardFilter) (*domain.PaginatedResult[domain.Card], error)
	ListCardAuthorizations(ctx context.Context, f domain.CardAuthFilter) (*domain.PaginatedResult[domain.CardAuthorization], error)

	// Audit
	ListAuditEntries(ctx context.Context, f domain.AuditFilter) (*domain.PaginatedResult[domain.AuditEntry], error)

	// Reconciliation
	ListReconExceptions(ctx context.Context, f domain.ExceptionFilter) (*domain.PaginatedResult[domain.ReconException], error)
	ListReconRuns(ctx context.Context, limit, offset int) (*domain.PaginatedResult[domain.ReconRun], error)
	AssignException(ctx context.Context, id, assignedTo string) error
	EscalateException(ctx context.Context, id, notes string) error
	UpdateExceptionStatus(ctx context.Context, id string, status domain.ExceptionStatus) error

	// Businesses
	ListBusinesses(ctx context.Context, f domain.BusinessFilter) (*domain.PaginatedResult[domain.Business], error)

	// Money flow map (session IP per user for geo)
	GetLastSessionIPByUserIDs(ctx context.Context, userIDs []string) (map[string]string, error)

	// Analytics aggregates
	CountUsers(ctx context.Context) (int64, error)
	CountActiveUsers30d(ctx context.Context) (int64, error)
	CountFrozenAccounts(ctx context.Context) (int64, error)
	CountActiveLoans(ctx context.Context) (int64, error)
	SumLoanOutstanding(ctx context.Context) (int64, error)
	CountActiveCards(ctx context.Context) (int64, error)
	CountActivePots(ctx context.Context) (int64, error)
	SumPotBalances(ctx context.Context) (int64, error)
	CountActiveBusinesses(ctx context.Context) (int64, error)
	CountTransactions(ctx context.Context) (int64, error)
}

type TransactionAggregate struct {
	Type       string `json:"type"`
	Status     string `json:"status"`
	Currency   string `json:"currency"`
	Count      int64  `json:"count"`
	TotalCents int64  `json:"totalCents"`
}

type LoanBookSummary struct {
	TotalIssued          int64              `json:"totalLoansIssued"`
	TotalDisbursedCents  int64              `json:"totalDisbursedCents"`
	TotalOutstandingCents int64             `json:"totalOutstandingCents"`
	TotalRepaidCents     int64              `json:"totalRepaidCents"`
	ByStatus             map[string]LoanStatusBucket `json:"byStatus"`
}

type LoanStatusBucket struct {
	Count          int64 `json:"count"`
	OutstandingCents int64 `json:"outstandingCents,omitempty"`
	RepaidCents    int64 `json:"repaidCents,omitempty"`
	WrittenOffCents int64 `json:"writtenOffCents,omitempty"`
}

// --- Implementation ---

type pgAdminQueryRepo struct{ db DBTX }

func NewAdminQueryRepository(db DBTX) AdminQueryRepository {
	return &pgAdminQueryRepo{db: db}
}

// --- Users ---

func (r *pgAdminQueryRepo) ListUsers(ctx context.Context, f domain.UserFilter) (*domain.PaginatedResult[domain.User], error) {
	limit, offset := domain.NormalizePagination(f.Limit, f.Offset)
	where, args := buildUserWhere(f)

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting users: %w", err)
	}

	orderBy := " ORDER BY created_at DESC"
	if f.SortBy != "" {
		col := sanitizeUserSortCol(f.SortBy)
		dir := "DESC"
		if strings.EqualFold(f.SortOrder, "asc") {
			dir = "ASC"
		}
		orderBy = fmt.Sprintf(" ORDER BY %s %s", col, dir)
	}

	query := `SELECT id, country_code, number, fayda_id_number, first_name, middle_name, last_name,
		date_of_birth, gender, fayda_photo_url, kyc_level, is_frozen, frozen_reason, frozen_at,
		ledger_wallet_id, telegram_id, telegram_username, created_at, updated_at, account_type,
		spend_waterfall_order
		FROM users` + where + orderBy + fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(
			&u.ID, &u.PhoneNumber.CountryCode, &u.PhoneNumber.Number, &u.FaydaIDNumber,
			&u.FirstName, &u.MiddleName, &u.LastName, &u.DateOfBirth, &u.Gender, &u.FaydaPhotoURL,
			&u.KYCLevel, &u.IsFrozen, &u.FrozenReason, &u.FrozenAt,
			&u.LedgerWalletID, &u.TelegramID, &u.TelegramUsername,
			&u.CreatedAt, &u.UpdatedAt, &u.AccountType, &u.SpendWaterfallOrder,
		); err != nil {
			return nil, fmt.Errorf("scanning user: %w", err)
		}
		users = append(users, u)
	}
	return domain.NewPaginatedResult(users, total, limit, offset), rows.Err()
}

func (r *pgAdminQueryRepo) CountUsersByKYCLevel(ctx context.Context) (map[domain.KYCLevel]int64, error) {
	rows, err := r.db.Query(ctx, `SELECT kyc_level, COUNT(*) FROM users GROUP BY kyc_level`)
	if err != nil {
		return nil, fmt.Errorf("counting users by kyc: %w", err)
	}
	defer rows.Close()

	result := make(map[domain.KYCLevel]int64)
	for rows.Next() {
		var level domain.KYCLevel
		var count int64
		if err := rows.Scan(&level, &count); err != nil {
			return nil, err
		}
		result[level] = count
	}
	return result, rows.Err()
}

// --- Transactions ---

func (r *pgAdminQueryRepo) ListTransactions(ctx context.Context, f domain.TransactionFilter) (*domain.PaginatedResult[domain.TransactionReceipt], error) {
	limit, offset := domain.NormalizePagination(f.Limit, f.Offset)
	where, args := buildTxnWhere(f)

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM transaction_receipts`+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting transactions: %w", err)
	}

	orderBy := " ORDER BY created_at DESC"
	if f.SortBy != "" {
		col := sanitizeTxnSortCol(f.SortBy)
		dir := "DESC"
		if strings.EqualFold(f.SortOrder, "asc") {
			dir = "ASC"
		}
		orderBy = fmt.Sprintf(" ORDER BY %s %s", col, dir)
	}

	query := `SELECT id, user_id, ledger_transaction_id, ethswitch_reference, idempotency_key,
		type, status, amount_cents, currency, counterparty_name, counterparty_phone, counterparty_country_code,
		counterparty_institution, narration, purpose, beneficiary_id, created_at, updated_at
		FROM transaction_receipts` + where + orderBy + fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing transactions: %w", err)
	}
	defer rows.Close()

	var txns []domain.TransactionReceipt
	for rows.Next() {
		var t domain.TransactionReceipt
		var cpPhone *string
		var cpCC *string
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.LedgerTransactionID, &t.EthSwitchReference, &t.IdempotencyKey,
			&t.Type, &t.Status, &t.AmountCents, &t.Currency, &t.CounterpartyName, &cpPhone, &cpCC,
			&t.CounterpartyInstitution, &t.Narration, &t.Purpose, &t.BeneficiaryID, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning transaction: %w", err)
		}
		if cpPhone != nil {
			p, _ := phone.Parse(*cpPhone)
			t.CounterpartyPhone = &p
		}
		txns = append(txns, t)
	}
	return domain.NewPaginatedResult(txns, total, limit, offset), rows.Err()
}

func (r *pgAdminQueryRepo) GetPairedReceipt(ctx context.Context, ledgerTxID string, excludeID string) (*domain.TransactionReceipt, error) {
	query := `SELECT id, user_id, ledger_transaction_id, ethswitch_reference, idempotency_key,
		type, status, amount_cents, currency, counterparty_name, counterparty_phone, counterparty_country_code,
		counterparty_institution, narration, purpose, beneficiary_id, created_at, updated_at
		FROM transaction_receipts
		WHERE ledger_transaction_id = $1 AND id != $2
		LIMIT 1`

	var t domain.TransactionReceipt
	var cpPhone *string
	var cpCC *string
	err := r.db.QueryRow(ctx, query, ledgerTxID, excludeID).Scan(
		&t.ID, &t.UserID, &t.LedgerTransactionID, &t.EthSwitchReference, &t.IdempotencyKey,
		&t.Type, &t.Status, &t.AmountCents, &t.Currency, &t.CounterpartyName, &cpPhone, &cpCC,
		&t.CounterpartyInstitution, &t.Narration, &t.Purpose, &t.BeneficiaryID, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting paired receipt: %w", err)
	}
	if cpPhone != nil {
		p, _ := phone.Parse(*cpPhone)
		t.CounterpartyPhone = &p
	}
	return &t, nil
}

func (r *pgAdminQueryRepo) SumTransactionsByStatusAndType(ctx context.Context, from, to time.Time) ([]TransactionAggregate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT type, status, currency, COUNT(*), COALESCE(SUM(amount_cents), 0)
		FROM transaction_receipts
		WHERE created_at >= $1 AND created_at < $2
		GROUP BY type, status, currency
		ORDER BY type, status`, from, to)
	if err != nil {
		return nil, fmt.Errorf("aggregating transactions: %w", err)
	}
	defer rows.Close()

	var result []TransactionAggregate
	for rows.Next() {
		var a TransactionAggregate
		if err := rows.Scan(&a.Type, &a.Status, &a.Currency, &a.Count, &a.TotalCents); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// --- Loans ---

func (r *pgAdminQueryRepo) ListLoans(ctx context.Context, f domain.LoanFilter) (*domain.PaginatedResult[domain.Loan], error) {
	limit, offset := domain.NormalizePagination(f.Limit, f.Offset)
	where, args := buildLoanWhere(f)

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM loans`+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting loans: %w", err)
	}

	orderBy := " ORDER BY created_at DESC"
	if f.SortBy != "" {
		col := sanitizeLoanSortCol(f.SortBy)
		dir := "DESC"
		if strings.EqualFold(f.SortOrder, "asc") {
			dir = "ASC"
		}
		orderBy = fmt.Sprintf(" ORDER BY %s %s", col, dir)
	}

	query := `SELECT id, user_id, principal_amount_cents, interest_fee_cents, total_due_cents,
		total_paid_cents, duration_days, disbursed_at, due_date, status, days_past_due,
		ledger_loan_account, ledger_disbursement_tx, idempotency_key, created_at, updated_at
		FROM loans` + where + orderBy + fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing loans: %w", err)
	}
	defer rows.Close()

	var loans []domain.Loan
	for rows.Next() {
		var l domain.Loan
		if err := rows.Scan(
			&l.ID, &l.UserID, &l.PrincipalAmountCents, &l.InterestFeeCents, &l.TotalDueCents,
			&l.TotalPaidCents, &l.DurationDays, &l.DisbursedAt, &l.DueDate, &l.Status, &l.DaysPastDue,
			&l.LedgerLoanAccount, &l.LedgerDisbursementTx, &l.IdempotencyKey, &l.CreatedAt, &l.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning loan: %w", err)
		}
		loans = append(loans, l)
	}
	return domain.NewPaginatedResult(loans, total, limit, offset), rows.Err()
}

func (r *pgAdminQueryRepo) LoanBookSummary(ctx context.Context) (*LoanBookSummary, error) {
	rows, err := r.db.Query(ctx, `
		SELECT status,
			COUNT(*),
			COALESCE(SUM(principal_amount_cents), 0),
			COALESCE(SUM(total_due_cents - total_paid_cents), 0),
			COALESCE(SUM(total_paid_cents), 0)
		FROM loans GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("loan book summary: %w", err)
	}
	defer rows.Close()

	summary := &LoanBookSummary{ByStatus: make(map[string]LoanStatusBucket)}
	for rows.Next() {
		var status string
		var count, disbursed, outstanding, repaid int64
		if err := rows.Scan(&status, &count, &disbursed, &outstanding, &repaid); err != nil {
			return nil, err
		}
		summary.TotalIssued += count
		summary.TotalDisbursedCents += disbursed
		summary.TotalOutstandingCents += outstanding
		summary.TotalRepaidCents += repaid
		summary.ByStatus[status] = LoanStatusBucket{
			Count:            count,
			OutstandingCents: outstanding,
			RepaidCents:      repaid,
		}
	}
	return summary, rows.Err()
}

func (r *pgAdminQueryRepo) ListCreditProfiles(ctx context.Context, f domain.CreditProfileFilter) (*domain.PaginatedResult[domain.CreditProfile], error) {
	limit, offset := domain.NormalizePagination(f.Limit, f.Offset)

	var conditions []string
	var args []any
	idx := 1
	if f.MinTrustScore != nil {
		conditions = append(conditions, fmt.Sprintf("trust_score >= $%d", idx))
		args = append(args, *f.MinTrustScore)
		idx++
	}
	if f.MaxTrustScore != nil {
		conditions = append(conditions, fmt.Sprintf("trust_score <= $%d", idx))
		args = append(args, *f.MaxTrustScore)
		idx++
	}
	if f.IsBlacklisted != nil {
		conditions = append(conditions, fmt.Sprintf("is_nbe_blacklisted = $%d", idx))
		args = append(args, *f.IsBlacklisted)
		idx++
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM credit_profiles`+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting credit profiles: %w", err)
	}

	query := `SELECT user_id, trust_score, approved_limit_cents, avg_monthly_inflow_cents,
		avg_monthly_balance_cents, active_days_per_month, total_loans_repaid, late_payments_count,
		current_outstanding_cents, is_nbe_blacklisted, blacklist_checked_at, last_calculated_at,
		created_at, updated_at
		FROM credit_profiles` + where + fmt.Sprintf(" ORDER BY trust_score DESC LIMIT %d OFFSET %d", limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing credit profiles: %w", err)
	}
	defer rows.Close()

	var profiles []domain.CreditProfile
	for rows.Next() {
		var p domain.CreditProfile
		if err := rows.Scan(
			&p.UserID, &p.TrustScore, &p.ApprovedLimitCents, &p.AvgMonthlyInflowCents,
			&p.AvgMonthlyBalanceCents, &p.ActiveDaysPerMonth, &p.TotalLoansRepaid, &p.LatePaymentsCount,
			&p.CurrentOutstandingCents, &p.IsNBEBlacklisted, &p.BlacklistCheckedAt, &p.LastCalculatedAt,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning credit profile: %w", err)
		}
		profiles = append(profiles, p)
	}
	return domain.NewPaginatedResult(profiles, total, limit, offset), rows.Err()
}

// --- Cards ---

func (r *pgAdminQueryRepo) ListCards(ctx context.Context, f domain.CardFilter) (*domain.PaginatedResult[domain.Card], error) {
	limit, offset := domain.NormalizePagination(f.Limit, f.Offset)
	where, args := buildCardWhere(f)

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM cards`+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting cards: %w", err)
	}

	query := `SELECT id, user_id, tokenized_pan, last_four, expiry_month, expiry_year,
		type, status, allow_online, allow_contactless, allow_atm, allow_international,
		daily_limit_cents, monthly_limit_cents, per_txn_limit_cents, ledger_card_account,
		created_at, updated_at
		FROM cards` + where + " ORDER BY created_at DESC" + fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing cards: %w", err)
	}
	defer rows.Close()

	var cards []domain.Card
	for rows.Next() {
		var c domain.Card
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.TokenizedPAN, &c.LastFour, &c.ExpiryMonth, &c.ExpiryYear,
			&c.Type, &c.Status, &c.AllowOnline, &c.AllowContactless, &c.AllowATM, &c.AllowInternational,
			&c.DailyLimitCents, &c.MonthlyLimitCents, &c.PerTxnLimitCents, &c.LedgerCardAccount,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning card: %w", err)
		}
		cards = append(cards, c)
	}
	return domain.NewPaginatedResult(cards, total, limit, offset), rows.Err()
}

func (r *pgAdminQueryRepo) ListCardAuthorizations(ctx context.Context, f domain.CardAuthFilter) (*domain.PaginatedResult[domain.CardAuthorization], error) {
	limit, offset := domain.NormalizePagination(f.Limit, f.Offset)
	where, args := buildCardAuthWhere(f)

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM card_authorizations`+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting card auths: %w", err)
	}

	query := `SELECT id, card_id, retrieval_reference_number, stan, auth_code,
		merchant_name, merchant_id, merchant_category_code, terminal_id, acquiring_institution,
		auth_amount_cents, settlement_amount_cents, currency, status,
		decline_reason, response_code, ledger_hold_id,
		authorized_at, settled_at, reversed_at, expires_at, created_at, updated_at
		FROM card_authorizations` + where + " ORDER BY created_at DESC" + fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing card auths: %w", err)
	}
	defer rows.Close()

	var auths []domain.CardAuthorization
	for rows.Next() {
		var a domain.CardAuthorization
		if err := rows.Scan(
			&a.ID, &a.CardID, &a.RetrievalReferenceNumber, &a.STAN, &a.AuthCode,
			&a.MerchantName, &a.MerchantID, &a.MerchantCategoryCode, &a.TerminalID, &a.AcquiringInstitution,
			&a.AuthAmountCents, &a.SettlementAmountCents, &a.Currency, &a.Status,
			&a.DeclineReason, &a.ResponseCode, &a.LedgerHoldID,
			&a.AuthorizedAt, &a.SettledAt, &a.ReversedAt, &a.ExpiresAt, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning card auth: %w", err)
		}
		auths = append(auths, a)
	}
	return domain.NewPaginatedResult(auths, total, limit, offset), rows.Err()
}

// --- Audit ---

func (r *pgAdminQueryRepo) ListAuditEntries(ctx context.Context, f domain.AuditFilter) (*domain.PaginatedResult[domain.AuditEntry], error) {
	limit, offset := domain.NormalizePagination(f.Limit, f.Offset)
	where, args := buildAuditWhere(f)

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log`+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting audit entries: %w", err)
	}

	dir := "DESC"
	if strings.EqualFold(f.SortOrder, "asc") {
		dir = "ASC"
	}

	query := `SELECT id, action, actor_type, actor_id, resource_type, resource_id, metadata,
		ip_address, user_agent, regulatory_rule_key, regulatory_action, nbe_reference, created_at
		FROM audit_log` + where + fmt.Sprintf(" ORDER BY created_at %s LIMIT %d OFFSET %d", dir, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing audit entries: %w", err)
	}
	defer rows.Close()

	var entries []domain.AuditEntry
	for rows.Next() {
		var e domain.AuditEntry
		if err := rows.Scan(
			&e.ID, &e.Action, &e.ActorType, &e.ActorID, &e.ResourceType, &e.ResourceID, &e.Metadata,
			&e.IPAddress, &e.UserAgent, &e.RegulatoryRuleKey, &e.RegulatoryAction, &e.NBEReference, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning audit entry: %w", err)
		}
		entries = append(entries, e)
	}
	return domain.NewPaginatedResult(entries, total, limit, offset), rows.Err()
}

func (r *pgAdminQueryRepo) GetLastSessionIPByUserIDs(ctx context.Context, userIDs []string) (map[string]string, error) {
	if len(userIDs) == 0 {
		return map[string]string{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (user_id) user_id::text, NULLIF(TRIM(ip_address), '') AS ip_address
		FROM sessions
		WHERE user_id = ANY($1::uuid[]) AND ip_address IS NOT NULL AND TRIM(ip_address) != ''
		ORDER BY user_id, created_at DESC`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("getting last session IP by user: %w", err)
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var userID, ip string
		if err := rows.Scan(&userID, &ip); err != nil {
			return nil, fmt.Errorf("scanning session IP: %w", err)
		}
		if ip != "" {
			out[userID] = ip
		}
	}
	return out, rows.Err()
}

// --- Reconciliation ---

func (r *pgAdminQueryRepo) ListReconExceptions(ctx context.Context, f domain.ExceptionFilter) (*domain.PaginatedResult[domain.ReconException], error) {
	limit, offset := domain.NormalizePagination(f.Limit, f.Offset)
	where, args := buildExceptionWhere(f)

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM reconciliation_exceptions`+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting recon exceptions: %w", err)
	}

	query := `SELECT * FROM reconciliation_exceptions` + where +
		" ORDER BY created_at DESC" + fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing recon exceptions: %w", err)
	}
	defer rows.Close()

	var exceptions []domain.ReconException
	for rows.Next() {
		e, err := scanReconException(rows)
		if err != nil {
			return nil, err
		}
		exceptions = append(exceptions, *e)
	}
	return domain.NewPaginatedResult(exceptions, total, limit, offset), rows.Err()
}

func (r *pgAdminQueryRepo) ListReconRuns(ctx context.Context, limit, offset int) (*domain.PaginatedResult[domain.ReconRun], error) {
	limit, offset = domain.NormalizePagination(limit, offset)

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM reconciliation_runs`).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting recon runs: %w", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, run_date, clearing_file_name, total_records, matched_count, exception_count,
		started_at, finished_at, status, error_message, created_at
		FROM reconciliation_runs ORDER BY run_date DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing recon runs: %w", err)
	}
	defer rows.Close()

	var runs []domain.ReconRun
	for rows.Next() {
		var run domain.ReconRun
		if err := rows.Scan(
			&run.ID, &run.RunDate, &run.ClearingFileName, &run.TotalRecords,
			&run.MatchedCount, &run.ExceptionCount, &run.StartedAt, &run.FinishedAt,
			&run.Status, &run.ErrorMessage, &run.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning recon run: %w", err)
		}
		runs = append(runs, run)
	}
	return domain.NewPaginatedResult(runs, total, limit, offset), rows.Err()
}

func (r *pgAdminQueryRepo) AssignException(ctx context.Context, id, assignedTo string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE reconciliation_exceptions SET assigned_to = $2, status = 'investigating', updated_at = NOW() WHERE id = $1`,
		id, assignedTo)
	if err != nil {
		return fmt.Errorf("assigning exception: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *pgAdminQueryRepo) EscalateException(ctx context.Context, id, notes string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE reconciliation_exceptions SET status = 'escalated', resolution_notes = $2, updated_at = NOW() WHERE id = $1`,
		id, notes)
	if err != nil {
		return fmt.Errorf("escalating exception: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *pgAdminQueryRepo) UpdateExceptionStatus(ctx context.Context, id string, status domain.ExceptionStatus) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE reconciliation_exceptions SET status = $2, updated_at = NOW() WHERE id = $1`,
		id, status)
	if err != nil {
		return fmt.Errorf("updating exception status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// --- Businesses ---

func (r *pgAdminQueryRepo) ListBusinesses(ctx context.Context, f domain.BusinessFilter) (*domain.PaginatedResult[domain.Business], error) {
	limit, offset := domain.NormalizePagination(f.Limit, f.Offset)
	where, args := buildBusinessWhere(f)

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM businesses`+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting businesses: %w", err)
	}

	query := `SELECT * FROM businesses` + where +
		" ORDER BY created_at DESC" + fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing businesses: %w", err)
	}
	defer rows.Close()

	var businesses []domain.Business
	for rows.Next() {
		b, err := scanBusiness(rows)
		if err != nil {
			return nil, err
		}
		businesses = append(businesses, *b)
	}
	return domain.NewPaginatedResult(businesses, total, limit, offset), rows.Err()
}

// --- Analytics Aggregates ---

func (r *pgAdminQueryRepo) CountUsers(ctx context.Context) (int64, error) {
	return r.countQuery(ctx, `SELECT COUNT(*) FROM users`)
}

func (r *pgAdminQueryRepo) CountActiveUsers30d(ctx context.Context) (int64, error) {
	return r.countQuery(ctx, `SELECT COUNT(DISTINCT user_id) FROM transaction_receipts WHERE created_at >= NOW() - INTERVAL '30 days'`)
}

func (r *pgAdminQueryRepo) CountFrozenAccounts(ctx context.Context) (int64, error) {
	return r.countQuery(ctx, `SELECT COUNT(*) FROM users WHERE is_frozen`)
}

func (r *pgAdminQueryRepo) CountActiveLoans(ctx context.Context) (int64, error) {
	return r.countQuery(ctx, `SELECT COUNT(*) FROM loans WHERE status IN ('active', 'in_arrears')`)
}

func (r *pgAdminQueryRepo) SumLoanOutstanding(ctx context.Context) (int64, error) {
	return r.countQuery(ctx, `SELECT COALESCE(SUM(total_due_cents - total_paid_cents), 0) FROM loans WHERE status IN ('active', 'in_arrears', 'defaulted')`)
}

func (r *pgAdminQueryRepo) CountActiveCards(ctx context.Context) (int64, error) {
	return r.countQuery(ctx, `SELECT COUNT(*) FROM cards WHERE status = 'active'`)
}

func (r *pgAdminQueryRepo) CountActivePots(ctx context.Context) (int64, error) {
	return r.countQuery(ctx, `SELECT COUNT(*) FROM pots WHERE NOT is_archived`)
}

func (r *pgAdminQueryRepo) SumPotBalances(ctx context.Context) (int64, error) {
	return 0, nil
}

func (r *pgAdminQueryRepo) CountActiveBusinesses(ctx context.Context) (int64, error) {
	return r.countQuery(ctx, `SELECT COUNT(*) FROM businesses WHERE status = 'active'`)
}

func (r *pgAdminQueryRepo) CountTransactions(ctx context.Context) (int64, error) {
	return r.countQuery(ctx, `SELECT COUNT(*) FROM transaction_receipts`)
}

func (r *pgAdminQueryRepo) countQuery(ctx context.Context, query string) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, query).Scan(&count)
	return count, err
}

// --- Filter Builders ---

func buildUserWhere(f domain.UserFilter) (string, []any) {
	var conds []string
	var args []any
	idx := 1

	if f.Search != nil && *f.Search != "" {
		conds = append(conds, fmt.Sprintf("(number ILIKE $%d OR first_name ILIKE $%d OR last_name ILIKE $%d OR fayda_id_number ILIKE $%d)", idx, idx, idx, idx))
		args = append(args, "%"+*f.Search+"%")
		idx++
	}
	if f.KYCLevel != nil {
		conds = append(conds, fmt.Sprintf("kyc_level = $%d", idx))
		args = append(args, *f.KYCLevel)
		idx++
	}
	if f.IsFrozen != nil {
		conds = append(conds, fmt.Sprintf("is_frozen = $%d", idx))
		args = append(args, *f.IsFrozen)
		idx++
	}
	if f.AccountType != nil {
		conds = append(conds, fmt.Sprintf("account_type = $%d", idx))
		args = append(args, *f.AccountType)
		idx++
	}
	if f.CreatedFrom != nil {
		conds = append(conds, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *f.CreatedFrom)
		idx++
	}
	if f.CreatedTo != nil {
		conds = append(conds, fmt.Sprintf("created_at < $%d", idx))
		args = append(args, *f.CreatedTo)
		idx++
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func buildTxnWhere(f domain.TransactionFilter) (string, []any) {
	var conds []string
	var args []any
	idx := 1

	if f.Search != nil && *f.Search != "" {
		conds = append(conds, fmt.Sprintf("(counterparty_name ILIKE $%d OR counterparty_phone ILIKE $%d OR narration ILIKE $%d OR ethswitch_reference ILIKE $%d)", idx, idx, idx, idx))
		args = append(args, "%"+*f.Search+"%")
		idx++
	}
	if f.UserID != nil {
		conds = append(conds, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, *f.UserID)
		idx++
	}
	if f.Type != nil {
		conds = append(conds, fmt.Sprintf("type = $%d", idx))
		args = append(args, *f.Type)
		idx++
	}
	if f.Status != nil {
		conds = append(conds, fmt.Sprintf("status = $%d", idx))
		args = append(args, *f.Status)
		idx++
	}
	if f.Currency != nil {
		conds = append(conds, fmt.Sprintf("currency = $%d", idx))
		args = append(args, *f.Currency)
		idx++
	}
	if f.MinAmountCents != nil {
		conds = append(conds, fmt.Sprintf("amount_cents >= $%d", idx))
		args = append(args, *f.MinAmountCents)
		idx++
	}
	if f.MaxAmountCents != nil {
		conds = append(conds, fmt.Sprintf("amount_cents <= $%d", idx))
		args = append(args, *f.MaxAmountCents)
		idx++
	}
	if f.CreatedFrom != nil {
		conds = append(conds, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *f.CreatedFrom)
		idx++
	}
	if f.CreatedTo != nil {
		conds = append(conds, fmt.Sprintf("created_at < $%d", idx))
		args = append(args, *f.CreatedTo)
		idx++
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func buildLoanWhere(f domain.LoanFilter) (string, []any) {
	var conds []string
	var args []any
	idx := 1

	if f.UserID != nil {
		conds = append(conds, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, *f.UserID)
		idx++
	}
	if f.Status != nil {
		conds = append(conds, fmt.Sprintf("status = $%d", idx))
		args = append(args, *f.Status)
		idx++
	}
	if f.MinAmount != nil {
		conds = append(conds, fmt.Sprintf("principal_amount_cents >= $%d", idx))
		args = append(args, *f.MinAmount)
		idx++
	}
	if f.MaxAmount != nil {
		conds = append(conds, fmt.Sprintf("principal_amount_cents <= $%d", idx))
		args = append(args, *f.MaxAmount)
		idx++
	}
	if f.CreatedFrom != nil {
		conds = append(conds, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *f.CreatedFrom)
		idx++
	}
	if f.CreatedTo != nil {
		conds = append(conds, fmt.Sprintf("created_at < $%d", idx))
		args = append(args, *f.CreatedTo)
		idx++
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func buildCardWhere(f domain.CardFilter) (string, []any) {
	var conds []string
	var args []any
	idx := 1

	if f.UserID != nil {
		conds = append(conds, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, *f.UserID)
		idx++
	}
	if f.Type != nil {
		conds = append(conds, fmt.Sprintf("type = $%d", idx))
		args = append(args, *f.Type)
		idx++
	}
	if f.Status != nil {
		conds = append(conds, fmt.Sprintf("status = $%d", idx))
		args = append(args, *f.Status)
		idx++
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func buildCardAuthWhere(f domain.CardAuthFilter) (string, []any) {
	var conds []string
	var args []any
	idx := 1

	if f.CardID != nil {
		conds = append(conds, fmt.Sprintf("card_id = $%d", idx))
		args = append(args, *f.CardID)
		idx++
	}
	if f.UserID != nil {
		conds = append(conds, fmt.Sprintf("card_id IN (SELECT id FROM cards WHERE user_id = $%d)", idx))
		args = append(args, *f.UserID)
		idx++
	}
	if f.Status != nil {
		conds = append(conds, fmt.Sprintf("status = $%d", idx))
		args = append(args, *f.Status)
		idx++
	}
	if f.MCC != nil {
		conds = append(conds, fmt.Sprintf("merchant_category_code = $%d", idx))
		args = append(args, *f.MCC)
		idx++
	}
	if f.CreatedFrom != nil {
		conds = append(conds, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *f.CreatedFrom)
		idx++
	}
	if f.CreatedTo != nil {
		conds = append(conds, fmt.Sprintf("created_at < $%d", idx))
		args = append(args, *f.CreatedTo)
		idx++
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func buildAuditWhere(f domain.AuditFilter) (string, []any) {
	var conds []string
	var args []any
	idx := 1

	if f.Search != nil && *f.Search != "" {
		conds = append(conds, fmt.Sprintf("metadata::text ILIKE $%d", idx))
		args = append(args, "%"+*f.Search+"%")
		idx++
	}
	if f.Action != nil {
		conds = append(conds, fmt.Sprintf("action = $%d", idx))
		args = append(args, *f.Action)
		idx++
	}
	if f.ActorType != nil {
		conds = append(conds, fmt.Sprintf("actor_type = $%d", idx))
		args = append(args, *f.ActorType)
		idx++
	}
	if f.ActorID != nil {
		conds = append(conds, fmt.Sprintf("actor_id = $%d", idx))
		args = append(args, *f.ActorID)
		idx++
	}
	if f.ResourceType != nil {
		conds = append(conds, fmt.Sprintf("resource_type = $%d", idx))
		args = append(args, *f.ResourceType)
		idx++
	}
	if f.ResourceID != nil {
		conds = append(conds, fmt.Sprintf("resource_id = $%d", idx))
		args = append(args, *f.ResourceID)
		idx++
	}
	if f.CreatedFrom != nil {
		conds = append(conds, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *f.CreatedFrom)
		idx++
	}
	if f.CreatedTo != nil {
		conds = append(conds, fmt.Sprintf("created_at < $%d", idx))
		args = append(args, *f.CreatedTo)
		idx++
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func buildExceptionWhere(f domain.ExceptionFilter) (string, []any) {
	var conds []string
	var args []any
	idx := 1

	if f.Status != nil {
		conds = append(conds, fmt.Sprintf("status = $%d", idx))
		args = append(args, *f.Status)
		idx++
	}
	if f.ErrorType != nil {
		conds = append(conds, fmt.Sprintf("error_type = $%d", idx))
		args = append(args, *f.ErrorType)
		idx++
	}
	if f.AssignedTo != nil {
		conds = append(conds, fmt.Sprintf("assigned_to = $%d", idx))
		args = append(args, *f.AssignedTo)
		idx++
	}
	if f.CreatedFrom != nil {
		conds = append(conds, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *f.CreatedFrom)
		idx++
	}
	if f.CreatedTo != nil {
		conds = append(conds, fmt.Sprintf("created_at < $%d", idx))
		args = append(args, *f.CreatedTo)
		idx++
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func buildBusinessWhere(f domain.BusinessFilter) (string, []any) {
	var conds []string
	var args []any
	idx := 1

	if f.Search != nil && *f.Search != "" {
		conds = append(conds, fmt.Sprintf("(name ILIKE $%d OR trade_name ILIKE $%d OR tin_number ILIKE $%d)", idx, idx, idx))
		args = append(args, "%"+*f.Search+"%")
		idx++
	}
	if f.Status != nil {
		conds = append(conds, fmt.Sprintf("status = $%d", idx))
		args = append(args, *f.Status)
		idx++
	}
	if f.Industry != nil {
		conds = append(conds, fmt.Sprintf("industry_category = $%d", idx))
		args = append(args, *f.Industry)
		idx++
	}
	if f.IsFrozen != nil {
		conds = append(conds, fmt.Sprintf("is_frozen = $%d", idx))
		args = append(args, *f.IsFrozen)
		idx++
	}
	if f.CreatedFrom != nil {
		conds = append(conds, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *f.CreatedFrom)
		idx++
	}
	if f.CreatedTo != nil {
		conds = append(conds, fmt.Sprintf("created_at < $%d", idx))
		args = append(args, *f.CreatedTo)
		idx++
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// --- Sort Column Sanitizers ---

func sanitizeUserSortCol(col string) string {
	switch col {
	case "created_at", "number", "first_name", "last_name", "kyc_level":
		return col
	default:
		return "created_at"
	}
}

func sanitizeTxnSortCol(col string) string {
	switch col {
	case "created_at", "amount_cents", "type", "status":
		return col
	default:
		return "created_at"
	}
}

func sanitizeLoanSortCol(col string) string {
	switch col {
	case "created_at", "principal_amount_cents", "status", "due_date":
		return col
	default:
		return "created_at"
	}
}

// scanReconException scans a reconciliation_exceptions row.
func scanReconException(rows pgx.Rows) (*domain.ReconException, error) {
	var e domain.ReconException
	err := rows.Scan(
		&e.ID, &e.EthSwitchReference, &e.IdempotencyKey, &e.ErrorType,
		&e.EthSwitchReportedAmountCents, &e.LedgerReportedAmountCents,
		&e.PostgresReportedAmountCents, &e.AmountDifferenceCents,
		&e.Status, &e.AssignedTo, &e.ResolutionNotes, &e.ResolutionAction,
		&e.ReconRunDate, &e.ClearingFileName, &e.CreatedAt, &e.ResolvedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning recon exception: %w", err)
	}
	return &e, nil
}

// scanBusiness scans a businesses row.
func scanBusiness(rows pgx.Rows) (*domain.Business, error) {
	var b domain.Business
	err := rows.Scan(
		&b.ID, &b.OwnerUserID, &b.Name, &b.TradeName, &b.TINNumber, &b.TradeLicenseNumber,
		&b.IndustryCategory, &b.IndustrySubCategory, &b.RegistrationDate,
		&b.Address, &b.City, &b.SubCity, &b.Woreda,
		&b.PhoneNumber, &b.PhoneNumber.CountryCode, &b.Email, &b.Website,
		&b.Status, &b.LedgerWalletID, &b.KYBLevel, &b.IsFrozen, &b.FrozenReason, &b.FrozenAt,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning business: %w", err)
	}
	return &b, nil
}

var _ AdminQueryRepository = (*pgAdminQueryRepo)(nil)
