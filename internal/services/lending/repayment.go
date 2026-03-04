package lending

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	nlog "github.com/vonmutinda/neo/pkg/logger"
	"github.com/vonmutinda/neo/pkg/money"
	"github.com/google/uuid"
)

// RepaymentService handles auto-sweep loan repayments.
// Run by the lending-worker cron job daily.
type RepaymentService struct {
	loans    repository.LoanRepository
	users    repository.UserRepository
	audit    repository.AuditRepository
	ledger   ledger.Client
	receipts repository.TransactionReceiptRepository
}

func NewRepaymentService(
	loans repository.LoanRepository,
	users repository.UserRepository,
	audit repository.AuditRepository,
	ledgerClient ledger.Client,
	receipts repository.TransactionReceiptRepository,
) *RepaymentService {
	return &RepaymentService{
		loans:    loans,
		users:    users,
		audit:    audit,
		ledger:   ledgerClient,
		receipts: receipts,
	}
}

// AutoSweep finds all unpaid installments due today and attempts to collect
// repayment from each borrower's wallet.
func (s *RepaymentService) AutoSweep(ctx context.Context) error {
	log := nlog.FromContext(ctx)

	installments, err := s.loans.ListDueUnpaidInstallments(ctx)
	if err != nil {
		return fmt.Errorf("listing due installments: %w", err)
	}

	log.Info("auto-sweep starting", slog.Int("installments", len(installments)))

	for _, inst := range installments {
		if err := s.processInstallment(ctx, inst); err != nil {
			log.Error("failed to process installment",
				slog.String("installment_id", inst.ID),
				slog.String("loan_id", inst.LoanID),
				slog.String("error", err.Error()),
			)
			continue
		}
	}

	return nil
}

func (s *RepaymentService) processInstallment(ctx context.Context, inst domain.LoanInstallment) error {
	loan, err := s.loans.GetLoan(ctx, inst.LoanID)
	if err != nil {
		return fmt.Errorf("fetching loan: %w", err)
	}

	user, err := s.users.GetByID(ctx, loan.UserID)
	if err != nil {
		return fmt.Errorf("fetching borrower: %w", err)
	}

	asset := money.FormatAsset()
	ik := uuid.NewString()

	// Split: principal portion goes to loan_capital, fee portion to interest.
	// For simplicity, we apportion based on the loan's principal/fee ratio.
	ratio := float64(loan.PrincipalAmountCents) / float64(loan.TotalDueCents)
	principalPortion := int64(float64(inst.AmountDueCents) * ratio)
	feePortion := inst.AmountDueCents - principalPortion

	if err := s.ledger.CollectLoanRepayment(ctx, ik, user.LedgerWalletID, principalPortion, feePortion, asset); err != nil {
		return fmt.Errorf("collecting repayment via ledger: %w", err)
	}

	narration := "Loan repayment"
	_ = s.receipts.Create(ctx, &domain.TransactionReceipt{
		UserID:              loan.UserID,
		LedgerTransactionID: ik,
		IdempotencyKey:      &ik,
		Type:                domain.ReceiptLoanRepayment,
		Status:              domain.ReceiptCompleted,
		AmountCents:         inst.AmountDueCents,
		Currency:            "ETB",
		FeeCents:            feePortion,
		Narration:           &narration,
	})

	if err := s.loans.MarkInstallmentPaid(ctx, inst.ID, ik); err != nil {
		return fmt.Errorf("marking installment paid: %w", err)
	}

	if err := s.loans.IncrementPaid(ctx, loan.ID, inst.AmountDueCents); err != nil {
		return fmt.Errorf("incrementing loan paid total: %w", err)
	}

	userID := loan.UserID
	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditLoanRepayment,
		ActorType:    "cron",
		ActorID:      &userID,
		ResourceType: "loan",
		ResourceID:   loan.ID,
	})

	return nil
}
