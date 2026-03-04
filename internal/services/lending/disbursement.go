package lending

import (
	"context"
	"fmt"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/gateway/nbe"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/money"
	"github.com/google/uuid"
)

// DisbursementService handles loan application, blacklist checks, and disbursement.
type DisbursementService struct {
	loans     repository.LoanRepository
	users     repository.UserRepository
	audit     repository.AuditRepository
	ledger    ledger.Client
	nbeClient nbe.Client
	receipts  repository.TransactionReceiptRepository
}

func NewDisbursementService(
	loans repository.LoanRepository,
	users repository.UserRepository,
	audit repository.AuditRepository,
	ledgerClient ledger.Client,
	nbeClient nbe.Client,
	receipts repository.TransactionReceiptRepository,
) *DisbursementService {
	return &DisbursementService{
		loans:     loans,
		users:     users,
		audit:     audit,
		ledger:    ledgerClient,
		nbeClient: nbeClient,
		receipts:  receipts,
	}
}

// DisburseLoan validates the user's credit profile, checks the NBE blacklist,
// and moves funds from @system:loan_capital to the user's wallet.
func (s *DisbursementService) DisburseLoan(ctx context.Context, userID string, req *LoanApplyRequest) (*domain.Loan, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	principalCents := req.PrincipalCents
	durationDays := req.DurationDays

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}
	if user.IsFrozen {
		return nil, domain.ErrUserFrozen
	}

	// Check credit profile
	profile, err := s.loans.GetCreditProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching credit profile: %w", err)
	}

	if !profile.IsEligibleForLoan() {
		return nil, domain.ErrNoEligibleProfile
	}
	if principalCents > profile.ApprovedLimitCents {
		return nil, domain.ErrLoanLimitExceeded
	}

	activeLoans, err := s.loans.ListActiveByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("checking active loans: %w", err)
	}
	if len(activeLoans) > 0 {
		return nil, domain.ErrActiveLoanExists
	}

	// NBE blacklist check
	if user.FaydaIDNumber != nil {
		isBlacklisted, err := s.nbeClient.IsBlacklisted(ctx, *user.FaydaIDNumber)
		if err != nil {
			return nil, fmt.Errorf("checking NBE blacklist: %w", err)
		}
		if isBlacklisted {
			return nil, domain.ErrNBEBlacklisted
		}
	}

	// Calculate facilitation fee (5% flat for Ethiopian micro-credit)
	feeCents := int64(float64(principalCents) * 0.05)
	totalDueCents := principalCents + feeCents

	asset := money.FormatAsset()
	ik := uuid.NewString()

	// Disburse via Formance: @system:loan_capital → @wallet:user_id
	txID, err := s.ledger.DisburseLoan(ctx, ik, user.LedgerWalletID, principalCents, asset)
	if err != nil {
		return nil, fmt.Errorf("disbursing loan via ledger: %w", err)
	}

	dueDate := time.Now().AddDate(0, 0, durationDays)
	loan := &domain.Loan{
		UserID:               userID,
		PrincipalAmountCents: principalCents,
		InterestFeeCents:     feeCents,
		TotalDueCents:        totalDueCents,
		DurationDays:         durationDays,
		DueDate:              dueDate,
		LedgerLoanAccount:    "loan:" + ik,
		LedgerDisbursementTx: &txID,
		IdempotencyKey:       &ik,
	}

	if err := s.loans.CreateLoan(ctx, loan); err != nil {
		return nil, fmt.Errorf("recording loan in postgres: %w", err)
	}

	// Create installments (equal monthly payments)
	months := durationDays / 30
	if months < 1 {
		months = 1
	}
	installmentAmount := totalDueCents / int64(months)
	var installments []domain.LoanInstallment
	for i := 1; i <= months; i++ {
		amt := installmentAmount
		if i == months {
			amt = totalDueCents - installmentAmount*int64(months-1)
		}
		installments = append(installments, domain.LoanInstallment{
			LoanID:            loan.ID,
			InstallmentNumber: i,
			AmountDueCents:    amt,
			DueDate:           time.Now().AddDate(0, i, 0),
		})
	}
	if err := s.loans.CreateInstallments(ctx, installments); err != nil {
		return nil, fmt.Errorf("creating installments: %w", err)
	}

	_ = s.receipts.Create(ctx, &domain.TransactionReceipt{
		UserID:              userID,
		LedgerTransactionID: txID,
		IdempotencyKey:      &ik,
		Type:                domain.ReceiptLoanDisbursement,
		Status:              domain.ReceiptCompleted,
		AmountCents:         principalCents,
		Currency:            money.CurrencyETB,
		Narration:           strPtr("Loan disbursed"),
	})

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditLoanDisbursed,
		ActorType:    "user",
		ActorID:      &userID,
		ResourceType: "loan",
		ResourceID:   loan.ID,
	})

	return loan, nil
}

// ManualRepay allows a user to repay part or all of their outstanding loan.
func (s *DisbursementService) ManualRepay(ctx context.Context, userID, loanID string, req *LoanRepayRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}

	loan, err := s.loans.GetLoan(ctx, loanID)
	if err != nil {
		return err
	}
	if loan.UserID != userID {
		return domain.ErrForbidden
	}
	if loan.Status != domain.LoanActive && loan.Status != domain.LoanInArrears {
		return domain.ErrLoanNotFound
	}

	remaining := loan.RemainingCents()
	amountCents := req.AmountCents
	if amountCents > remaining {
		amountCents = remaining
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("looking up user: %w", err)
	}

	asset := money.FormatAsset()
	ik := uuid.NewString()

	ratio := float64(loan.PrincipalAmountCents) / float64(loan.TotalDueCents)
	principalPortion := int64(float64(amountCents) * ratio)
	feePortion := amountCents - principalPortion

	if err := s.ledger.CollectLoanRepayment(ctx, ik, user.LedgerWalletID, principalPortion, feePortion, asset); err != nil {
		return fmt.Errorf("collecting repayment via ledger: %w", err)
	}

	_ = s.receipts.Create(ctx, &domain.TransactionReceipt{
		UserID:              userID,
		LedgerTransactionID: ik,
		IdempotencyKey:      &ik,
		Type:                domain.ReceiptLoanRepayment,
		Status:              domain.ReceiptCompleted,
		AmountCents:         amountCents,
		Currency:            money.CurrencyETB,
		FeeCents:            feePortion,
		Narration:           strPtr("Loan repayment"),
	})

	if err := s.loans.IncrementPaid(ctx, loanID, amountCents); err != nil {
		return fmt.Errorf("incrementing loan paid total: %w", err)
	}

	installments, err := s.loans.ListInstallmentsByLoan(ctx, loanID)
	if err != nil {
		return fmt.Errorf("listing installments: %w", err)
	}
	var covered int64
	for _, inst := range installments {
		if inst.IsPaid {
			continue
		}
		if covered+inst.AmountDueCents <= amountCents {
			_ = s.loans.MarkInstallmentPaid(ctx, inst.ID, ik)
			covered += inst.AmountDueCents
		} else {
			break
		}
	}

	if loan.TotalPaidCents+amountCents >= loan.TotalDueCents {
		_ = s.loans.UpdateLoanStatus(ctx, loanID, domain.LoanRepaid, 0)
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditLoanManualRepayment,
		ActorType:    "user",
		ActorID:      &userID,
		ResourceType: "loan",
		ResourceID:   loanID,
	})

	return nil
}

func strPtr(s string) *string { return &s }
