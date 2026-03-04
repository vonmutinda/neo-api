package repository

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/jackc/pgx/v5"
)

type BusinessLoanRepository interface {
	UpsertCreditProfile(ctx context.Context, cp *domain.BusinessCreditProfile) error
	GetCreditProfile(ctx context.Context, businessID string) (*domain.BusinessCreditProfile, error)

	CreateLoan(ctx context.Context, loan *domain.BusinessLoan) error
	GetLoan(ctx context.Context, id string) (*domain.BusinessLoan, error)
	ListByBusiness(ctx context.Context, businessID string, limit, offset int) ([]domain.BusinessLoan, error)
	UpdateLoanStatus(ctx context.Context, id string, status domain.LoanStatus) error
	IncrementPaid(ctx context.Context, id string, amountCents int64) error

	CreateInstallments(ctx context.Context, installments []domain.BusinessLoanInstallment) error
	ListInstallments(ctx context.Context, loanID string) ([]domain.BusinessLoanInstallment, error)
	MarkInstallmentPaid(ctx context.Context, id, ledgerTx string) error
}

type pgBusinessLoanRepo struct{ db DBTX }

func NewBusinessLoanRepository(db DBTX) BusinessLoanRepository {
	return &pgBusinessLoanRepo{db: db}
}

func (r *pgBusinessLoanRepo) UpsertCreditProfile(ctx context.Context, cp *domain.BusinessCreditProfile) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO business_credit_profiles (
			business_id, trust_score, approved_limit_cents, avg_monthly_revenue_cents,
			avg_monthly_expenses_cents, cash_flow_score, time_in_business_months,
			industry_risk_score, total_loans_repaid, late_payments_count,
			current_outstanding_cents, collateral_value_cents, is_nbe_blacklisted, last_calculated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NOW())
		ON CONFLICT (business_id) DO UPDATE SET
			trust_score=$2, approved_limit_cents=$3, avg_monthly_revenue_cents=$4,
			avg_monthly_expenses_cents=$5, cash_flow_score=$6, time_in_business_months=$7,
			industry_risk_score=$8, total_loans_repaid=$9, late_payments_count=$10,
			current_outstanding_cents=$11, collateral_value_cents=$12, is_nbe_blacklisted=$13,
			last_calculated_at=NOW(), updated_at=NOW()`,
		cp.BusinessID, cp.TrustScore, cp.ApprovedLimitCents, cp.AvgMonthlyRevenueCents,
		cp.AvgMonthlyExpensesCents, cp.CashFlowScore, cp.TimeInBusinessMonths,
		cp.IndustryRiskScore, cp.TotalLoansRepaid, cp.LatePaymentsCount,
		cp.CurrentOutstandingCents, cp.CollateralValueCents, cp.IsNBEBlacklisted)
	if err != nil {
		return fmt.Errorf("upserting business credit profile: %w", err)
	}
	return nil
}

func (r *pgBusinessLoanRepo) GetCreditProfile(ctx context.Context, businessID string) (*domain.BusinessCreditProfile, error) {
	var cp domain.BusinessCreditProfile
	err := r.db.QueryRow(ctx, `
		SELECT business_id, trust_score, approved_limit_cents, avg_monthly_revenue_cents,
			avg_monthly_expenses_cents, cash_flow_score, time_in_business_months,
			industry_risk_score, total_loans_repaid, late_payments_count,
			current_outstanding_cents, collateral_value_cents, is_nbe_blacklisted,
			last_calculated_at, created_at, updated_at
		FROM business_credit_profiles WHERE business_id = $1`, businessID).Scan(
		&cp.BusinessID, &cp.TrustScore, &cp.ApprovedLimitCents, &cp.AvgMonthlyRevenueCents,
		&cp.AvgMonthlyExpensesCents, &cp.CashFlowScore, &cp.TimeInBusinessMonths,
		&cp.IndustryRiskScore, &cp.TotalLoansRepaid, &cp.LatePaymentsCount,
		&cp.CurrentOutstandingCents, &cp.CollateralValueCents, &cp.IsNBEBlacklisted,
		&cp.LastCalculatedAt, &cp.CreatedAt, &cp.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNoEligibleProfile
	}
	if err != nil {
		return nil, fmt.Errorf("getting business credit profile: %w", err)
	}
	return &cp, nil
}

func (r *pgBusinessLoanRepo) CreateLoan(ctx context.Context, loan *domain.BusinessLoan) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO business_loans (business_id, principal_amount_cents, interest_fee_cents,
			total_due_cents, duration_days, due_date, purpose, collateral_description,
			ledger_loan_account, applied_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id, disbursed_at, created_at, updated_at`,
		loan.BusinessID, loan.PrincipalAmountCents, loan.InterestFeeCents,
		loan.TotalDueCents, loan.DurationDays, loan.DueDate, loan.Purpose,
		loan.CollateralDescription, loan.LedgerLoanAccount, loan.AppliedBy,
	).Scan(&loan.ID, &loan.DisbursedAt, &loan.CreatedAt, &loan.UpdatedAt)
}

func (r *pgBusinessLoanRepo) GetLoan(ctx context.Context, id string) (*domain.BusinessLoan, error) {
	var l domain.BusinessLoan
	err := r.db.QueryRow(ctx, `
		SELECT id, business_id, principal_amount_cents, interest_fee_cents, total_due_cents,
			total_paid_cents, duration_days, disbursed_at, due_date, status, days_past_due,
			purpose, collateral_description, ledger_loan_account, ledger_disbursement_tx,
			applied_by, approved_by, created_at, updated_at
		FROM business_loans WHERE id = $1`, id).Scan(
		&l.ID, &l.BusinessID, &l.PrincipalAmountCents, &l.InterestFeeCents, &l.TotalDueCents,
		&l.TotalPaidCents, &l.DurationDays, &l.DisbursedAt, &l.DueDate, &l.Status, &l.DaysPastDue,
		&l.Purpose, &l.CollateralDescription, &l.LedgerLoanAccount, &l.LedgerDisbursementTx,
		&l.AppliedBy, &l.ApprovedBy, &l.CreatedAt, &l.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrLoanNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting business loan: %w", err)
	}
	return &l, nil
}

func (r *pgBusinessLoanRepo) ListByBusiness(ctx context.Context, businessID string, limit, offset int) ([]domain.BusinessLoan, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, business_id, principal_amount_cents, interest_fee_cents, total_due_cents,
			total_paid_cents, duration_days, disbursed_at, due_date, status, days_past_due,
			purpose, collateral_description, ledger_loan_account, ledger_disbursement_tx,
			applied_by, approved_by, created_at, updated_at
		FROM business_loans WHERE business_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, businessID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing business loans: %w", err)
	}
	defer rows.Close()
	var result []domain.BusinessLoan
	for rows.Next() {
		var l domain.BusinessLoan
		if err := rows.Scan(
			&l.ID, &l.BusinessID, &l.PrincipalAmountCents, &l.InterestFeeCents, &l.TotalDueCents,
			&l.TotalPaidCents, &l.DurationDays, &l.DisbursedAt, &l.DueDate, &l.Status, &l.DaysPastDue,
			&l.Purpose, &l.CollateralDescription, &l.LedgerLoanAccount, &l.LedgerDisbursementTx,
			&l.AppliedBy, &l.ApprovedBy, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning business loan: %w", err)
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

func (r *pgBusinessLoanRepo) UpdateLoanStatus(ctx context.Context, id string, status domain.LoanStatus) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE business_loans SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	if err != nil {
		return fmt.Errorf("updating loan status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrLoanNotFound
	}
	return nil
}

func (r *pgBusinessLoanRepo) IncrementPaid(ctx context.Context, id string, amountCents int64) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE business_loans SET total_paid_cents = total_paid_cents + $2, updated_at=NOW() WHERE id=$1`,
		id, amountCents)
	if err != nil {
		return fmt.Errorf("incrementing paid: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrLoanNotFound
	}
	return nil
}

func (r *pgBusinessLoanRepo) CreateInstallments(ctx context.Context, installments []domain.BusinessLoanInstallment) error {
	for i := range installments {
		inst := &installments[i]
		if err := r.db.QueryRow(ctx,
			`INSERT INTO business_loan_installments (loan_id, installment_number, amount_due_cents, due_date)
			VALUES ($1,$2,$3,$4) RETURNING id, created_at, updated_at`,
			inst.LoanID, inst.InstallmentNumber, inst.AmountDueCents, inst.DueDate,
		).Scan(&inst.ID, &inst.CreatedAt, &inst.UpdatedAt); err != nil {
			return fmt.Errorf("creating installment %d: %w", inst.InstallmentNumber, err)
		}
	}
	return nil
}

func (r *pgBusinessLoanRepo) ListInstallments(ctx context.Context, loanID string) ([]domain.BusinessLoanInstallment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, loan_id, installment_number, amount_due_cents, amount_paid_cents,
			due_date, is_paid, paid_at, ledger_repayment_tx, created_at, updated_at
		FROM business_loan_installments WHERE loan_id = $1 ORDER BY installment_number`, loanID)
	if err != nil {
		return nil, fmt.Errorf("listing installments: %w", err)
	}
	defer rows.Close()
	var result []domain.BusinessLoanInstallment
	for rows.Next() {
		var inst domain.BusinessLoanInstallment
		if err := rows.Scan(&inst.ID, &inst.LoanID, &inst.InstallmentNumber,
			&inst.AmountDueCents, &inst.AmountPaidCents, &inst.DueDate, &inst.IsPaid,
			&inst.PaidAt, &inst.LedgerRepaymentTx, &inst.CreatedAt, &inst.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning installment: %w", err)
		}
		result = append(result, inst)
	}
	return result, rows.Err()
}

func (r *pgBusinessLoanRepo) MarkInstallmentPaid(ctx context.Context, id, ledgerTx string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE business_loan_installments SET is_paid=TRUE, paid_at=NOW(), ledger_repayment_tx=$2, updated_at=NOW()
		WHERE id=$1`, id, ledgerTx)
	if err != nil {
		return fmt.Errorf("marking installment paid: %w", err)
	}
	return nil
}

var _ BusinessLoanRepository = (*pgBusinessLoanRepo)(nil)
