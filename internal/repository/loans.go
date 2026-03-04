package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type LoanRepository interface {
	// Credit Profiles
	UpsertCreditProfile(ctx context.Context, p *domain.CreditProfile) error
	GetCreditProfile(ctx context.Context, userID string) (*domain.CreditProfile, error)

	// Loans
	CreateLoan(ctx context.Context, loan *domain.Loan) error
	GetLoan(ctx context.Context, id string) (*domain.Loan, error)
	ListActiveByUser(ctx context.Context, userID string) ([]domain.Loan, error)
	ListAllByUser(ctx context.Context, userID string, limit, offset int) ([]domain.Loan, error)
	CountByUser(ctx context.Context, userID string) (int, error)
	UpdateLoanStatus(ctx context.Context, id string, status domain.LoanStatus, daysPastDue int) error
	IncrementPaid(ctx context.Context, id string, amountCents int64) error

	// Installments
	CreateInstallments(ctx context.Context, installments []domain.LoanInstallment) error
	ListInstallmentsByLoan(ctx context.Context, loanID string) ([]domain.LoanInstallment, error)
	ListDueUnpaidInstallments(ctx context.Context) ([]domain.LoanInstallment, error)
	MarkInstallmentPaid(ctx context.Context, id string, ledgerTxID string) error
	CountLatePayments(ctx context.Context, userID string) (int, error)
}

type pgLoanRepo struct{ db DBTX }

func NewLoanRepository(db DBTX) LoanRepository { return &pgLoanRepo{db: db} }

func (r *pgLoanRepo) UpsertCreditProfile(ctx context.Context, p *domain.CreditProfile) error {
	query := `
		INSERT INTO credit_profiles (
			user_id, trust_score, approved_limit_cents,
			avg_monthly_inflow_cents, avg_monthly_balance_cents, active_days_per_month,
			total_loans_repaid, late_payments_count, current_outstanding_cents,
			is_nbe_blacklisted, blacklist_checked_at, last_calculated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (user_id) DO UPDATE SET
			trust_score = EXCLUDED.trust_score,
			approved_limit_cents = EXCLUDED.approved_limit_cents,
			avg_monthly_inflow_cents = EXCLUDED.avg_monthly_inflow_cents,
			avg_monthly_balance_cents = EXCLUDED.avg_monthly_balance_cents,
			active_days_per_month = EXCLUDED.active_days_per_month,
			total_loans_repaid = EXCLUDED.total_loans_repaid,
			late_payments_count = EXCLUDED.late_payments_count,
			current_outstanding_cents = EXCLUDED.current_outstanding_cents,
			is_nbe_blacklisted = EXCLUDED.is_nbe_blacklisted,
			blacklist_checked_at = EXCLUDED.blacklist_checked_at,
			last_calculated_at = EXCLUDED.last_calculated_at,
			updated_at = NOW()`
	_, err := r.db.Exec(ctx, query,
		p.UserID, p.TrustScore, p.ApprovedLimitCents,
		p.AvgMonthlyInflowCents, p.AvgMonthlyBalanceCents, p.ActiveDaysPerMonth,
		p.TotalLoansRepaid, p.LatePaymentsCount, p.CurrentOutstandingCents,
		p.IsNBEBlacklisted, p.BlacklistCheckedAt, p.LastCalculatedAt)
	if err != nil {
		return fmt.Errorf("upserting credit profile: %w", err)
	}
	return nil
}

func (r *pgLoanRepo) GetCreditProfile(ctx context.Context, userID string) (*domain.CreditProfile, error) {
	var p domain.CreditProfile
	err := r.db.QueryRow(ctx, `SELECT * FROM credit_profiles WHERE user_id = $1`, userID).Scan(
		&p.UserID, &p.TrustScore, &p.ApprovedLimitCents,
		&p.AvgMonthlyInflowCents, &p.AvgMonthlyBalanceCents, &p.ActiveDaysPerMonth,
		&p.TotalLoansRepaid, &p.LatePaymentsCount, &p.CurrentOutstandingCents,
		&p.IsNBEBlacklisted, &p.BlacklistCheckedAt, &p.LastCalculatedAt,
		&p.CreatedAt, &p.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNoEligibleProfile
	}
	if err != nil {
		return nil, fmt.Errorf("getting credit profile: %w", err)
	}
	return &p, nil
}

func (r *pgLoanRepo) CreateLoan(ctx context.Context, loan *domain.Loan) error {
	query := `
		INSERT INTO loans (user_id, principal_amount_cents, interest_fee_cents, total_due_cents,
			duration_days, due_date, ledger_loan_account, ledger_disbursement_tx, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, disbursed_at, created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		loan.UserID, loan.PrincipalAmountCents, loan.InterestFeeCents, loan.TotalDueCents,
		loan.DurationDays, loan.DueDate, loan.LedgerLoanAccount, loan.LedgerDisbursementTx, loan.IdempotencyKey,
	).Scan(&loan.ID, &loan.DisbursedAt, &loan.CreatedAt, &loan.UpdatedAt)
}

func (r *pgLoanRepo) GetLoan(ctx context.Context, id string) (*domain.Loan, error) {
	var l domain.Loan
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, principal_amount_cents, interest_fee_cents, total_due_cents, total_paid_cents,
			duration_days, disbursed_at, due_date, status, days_past_due,
			ledger_loan_account, ledger_disbursement_tx, idempotency_key, created_at, updated_at
		FROM loans WHERE id = $1`, id).Scan(
		&l.ID, &l.UserID, &l.PrincipalAmountCents, &l.InterestFeeCents, &l.TotalDueCents, &l.TotalPaidCents,
		&l.DurationDays, &l.DisbursedAt, &l.DueDate, &l.Status, &l.DaysPastDue,
		&l.LedgerLoanAccount, &l.LedgerDisbursementTx, &l.IdempotencyKey, &l.CreatedAt, &l.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrLoanNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting loan: %w", err)
	}
	return &l, nil
}

func (r *pgLoanRepo) ListActiveByUser(ctx context.Context, userID string) ([]domain.Loan, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, principal_amount_cents, interest_fee_cents, total_due_cents, total_paid_cents,
			duration_days, disbursed_at, due_date, status, days_past_due,
			ledger_loan_account, ledger_disbursement_tx, idempotency_key, created_at, updated_at
		FROM loans WHERE user_id = $1 AND status IN ('active','in_arrears') ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing active loans: %w", err)
	}
	defer rows.Close()
	var loans []domain.Loan
	for rows.Next() {
		var l domain.Loan
		if err := rows.Scan(&l.ID, &l.UserID, &l.PrincipalAmountCents, &l.InterestFeeCents, &l.TotalDueCents, &l.TotalPaidCents,
			&l.DurationDays, &l.DisbursedAt, &l.DueDate, &l.Status, &l.DaysPastDue,
			&l.LedgerLoanAccount, &l.LedgerDisbursementTx, &l.IdempotencyKey, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning loan: %w", err)
		}
		loans = append(loans, l)
	}
	return loans, rows.Err()
}

func (r *pgLoanRepo) ListAllByUser(ctx context.Context, userID string, limit, offset int) ([]domain.Loan, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, principal_amount_cents, interest_fee_cents, total_due_cents, total_paid_cents,
			duration_days, disbursed_at, due_date, status, days_past_due,
			ledger_loan_account, ledger_disbursement_tx, idempotency_key, created_at, updated_at
		FROM loans WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing all loans for user: %w", err)
	}
	defer rows.Close()
	var loans []domain.Loan
	for rows.Next() {
		var l domain.Loan
		if err := rows.Scan(&l.ID, &l.UserID, &l.PrincipalAmountCents, &l.InterestFeeCents, &l.TotalDueCents, &l.TotalPaidCents,
			&l.DurationDays, &l.DisbursedAt, &l.DueDate, &l.Status, &l.DaysPastDue,
			&l.LedgerLoanAccount, &l.LedgerDisbursementTx, &l.IdempotencyKey, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning loan: %w", err)
		}
		loans = append(loans, l)
	}
	return loans, rows.Err()
}

func (r *pgLoanRepo) CountByUser(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM loans WHERE user_id = $1`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting loans for user: %w", err)
	}
	return count, nil
}

func (r *pgLoanRepo) UpdateLoanStatus(ctx context.Context, id string, status domain.LoanStatus, daysPastDue int) error {
	_, err := r.db.Exec(ctx, `UPDATE loans SET status=$2, days_past_due=$3, updated_at=NOW() WHERE id=$1`, id, status, daysPastDue)
	return err
}

func (r *pgLoanRepo) IncrementPaid(ctx context.Context, id string, amountCents int64) error {
	_, err := r.db.Exec(ctx, `UPDATE loans SET total_paid_cents = total_paid_cents + $2, updated_at=NOW() WHERE id=$1`, id, amountCents)
	return err
}

func (r *pgLoanRepo) CreateInstallments(ctx context.Context, installments []domain.LoanInstallment) error {
	if len(installments) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, inst := range installments {
		batch.Queue(`INSERT INTO loan_installments (loan_id, installment_number, amount_due_cents, due_date)
			VALUES ($1,$2,$3,$4)`, inst.LoanID, inst.InstallmentNumber, inst.AmountDueCents, inst.DueDate)
	}
	br := r.db.SendBatch(ctx, batch)
	defer br.Close()
	for i := range installments {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("creating installment %d: %w", i+1, err)
		}
	}
	return nil
}

func (r *pgLoanRepo) ListInstallmentsByLoan(ctx context.Context, loanID string) ([]domain.LoanInstallment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, loan_id, installment_number, amount_due_cents, amount_paid_cents, due_date, is_paid, paid_at, ledger_repayment_tx, created_at, updated_at
		FROM loan_installments WHERE loan_id = $1 ORDER BY installment_number`, loanID)
	if err != nil {
		return nil, fmt.Errorf("listing installments: %w", err)
	}
	defer rows.Close()
	var result []domain.LoanInstallment
	for rows.Next() {
		var i domain.LoanInstallment
		if err := rows.Scan(&i.ID, &i.LoanID, &i.InstallmentNumber, &i.AmountDueCents, &i.AmountPaidCents,
			&i.DueDate, &i.IsPaid, &i.PaidAt, &i.LedgerRepaymentTx, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning installment: %w", err)
		}
		result = append(result, i)
	}
	return result, rows.Err()
}

func (r *pgLoanRepo) ListDueUnpaidInstallments(ctx context.Context) ([]domain.LoanInstallment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, loan_id, installment_number, amount_due_cents, amount_paid_cents, due_date, is_paid, paid_at, ledger_repayment_tx, created_at, updated_at
		FROM loan_installments WHERE is_paid = FALSE AND due_date <= NOW() ORDER BY due_date`)
	if err != nil {
		return nil, fmt.Errorf("listing due installments: %w", err)
	}
	defer rows.Close()
	var result []domain.LoanInstallment
	for rows.Next() {
		var i domain.LoanInstallment
		if err := rows.Scan(&i.ID, &i.LoanID, &i.InstallmentNumber, &i.AmountDueCents, &i.AmountPaidCents,
			&i.DueDate, &i.IsPaid, &i.PaidAt, &i.LedgerRepaymentTx, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning due installment: %w", err)
		}
		result = append(result, i)
	}
	return result, rows.Err()
}

func (r *pgLoanRepo) MarkInstallmentPaid(ctx context.Context, id string, ledgerTxID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE loan_installments SET is_paid=TRUE, paid_at=NOW(), amount_paid_cents=amount_due_cents, ledger_repayment_tx=$2, updated_at=NOW() WHERE id=$1`,
		id, ledgerTxID)
	return err
}

func (r *pgLoanRepo) CountLatePayments(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM loan_installments li
		JOIN loans l ON l.id = li.loan_id
		WHERE l.user_id = $1 AND li.is_paid = FALSE AND li.due_date < NOW()`, userID).Scan(&count)
	return count, err
}

var _ LoanRepository = (*pgLoanRepo)(nil)
