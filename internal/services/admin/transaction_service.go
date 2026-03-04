package admin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
)

type TransactionService struct {
	adminRepo   repository.AdminQueryRepository
	receiptRepo repository.TransactionReceiptRepository
	auditRepo   repository.AuditRepository
}

func NewTransactionService(
	adminRepo repository.AdminQueryRepository,
	receiptRepo repository.TransactionReceiptRepository,
	auditRepo repository.AuditRepository,
) *TransactionService {
	return &TransactionService{
		adminRepo:   adminRepo,
		receiptRepo: receiptRepo,
		auditRepo:   auditRepo,
	}
}

// AdminTransactionView extends TransactionReceipt with optional conversion pair data.
type AdminTransactionView struct {
	domain.TransactionReceipt
	ConvertedCurrency    *string `json:"convertedCurrency,omitempty"`
	ConvertedAmountCents *int64  `json:"convertedAmountCents,omitempty"`
}

func (s *TransactionService) List(ctx context.Context, filter domain.TransactionFilter) (*domain.PaginatedResult[AdminTransactionView], error) {
	result, err := s.adminRepo.ListTransactions(ctx, filter)
	if err != nil {
		return nil, err
	}

	views := make([]AdminTransactionView, 0, len(result.Data))
	var skipped int64
	for _, tx := range result.Data {
		if tx.Type == domain.ReceiptConvertIn {
			if _, err := s.adminRepo.GetPairedReceipt(ctx, tx.LedgerTransactionID, tx.ID); err == nil {
				skipped++
				continue
			}
		}
		v := AdminTransactionView{TransactionReceipt: tx}
		if tx.Type == domain.ReceiptConvertOut {
			if paired, err := s.adminRepo.GetPairedReceipt(ctx, tx.LedgerTransactionID, tx.ID); err == nil {
				v.ConvertedCurrency = &paired.Currency
				v.ConvertedAmountCents = &paired.AmountCents
			}
		}
		views = append(views, v)
	}

	adjustedTotal := result.Pagination.Total - skipped
	return domain.NewPaginatedResult(views, adjustedTotal, int(result.Pagination.Limit), int(result.Pagination.Offset)), nil
}

func (s *TransactionService) GetByID(ctx context.Context, id string) (*domain.TransactionReceipt, error) {
	return s.receiptRepo.GetByID(ctx, id)
}

// ConversionView merges a convert_out/convert_in pair into a single view.
type ConversionView struct {
	ID               string               `json:"id"`
	UserID           string               `json:"userId"`
	FromCurrency     string               `json:"fromCurrency"`
	ToCurrency       string               `json:"toCurrency"`
	FromAmountCents  int64                `json:"fromAmountCents"`
	ToAmountCents    int64                `json:"toAmountCents"`
	Status           domain.ReceiptStatus `json:"status"`
	Narration        *string              `json:"narration,omitempty"`
	LedgerTxID       string               `json:"ledgerTransactionId"`
	CreatedAt        time.Time            `json:"createdAt"`
}

// GetConversion returns a merged conversion view for a convert_out or convert_in receipt.
func (s *TransactionService) GetConversion(ctx context.Context, id string) (*ConversionView, error) {
	receipt, err := s.receiptRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if receipt.Type != domain.ReceiptConvertOut && receipt.Type != domain.ReceiptConvertIn {
		return nil, domain.ErrInvalidInput
	}

	paired, err := s.adminRepo.GetPairedReceipt(ctx, receipt.LedgerTransactionID, receipt.ID)
	if err != nil {
		return nil, err
	}

	out, in := receipt, paired
	if receipt.Type == domain.ReceiptConvertIn {
		out, in = paired, receipt
	}

	return &ConversionView{
		ID:              out.ID,
		UserID:          out.UserID,
		FromCurrency:    out.Currency,
		ToCurrency:      in.Currency,
		FromAmountCents: out.AmountCents,
		ToAmountCents:   in.AmountCents,
		Status:          out.Status,
		Narration:       out.Narration,
		LedgerTxID:      out.LedgerTransactionID,
		CreatedAt:       out.CreatedAt,
	}, nil
}

func (s *TransactionService) SumByStatusAndType(ctx context.Context, from, to time.Time) ([]repository.TransactionAggregate, error) {
	return s.adminRepo.SumTransactionsByStatusAndType(ctx, from, to)
}

type ReverseRequest struct {
	Reason          string `json:"reason" validate:"required"`
	ReferenceTicket string `json:"referenceTicket"`
}

func (s *TransactionService) Reverse(ctx context.Context, staffID, txnID string, req ReverseRequest) error {
	txn, err := s.receiptRepo.GetByID(ctx, txnID)
	if err != nil {
		return err
	}

	if txn.Status != domain.ReceiptCompleted {
		return domain.ErrInvalidInput
	}

	if err := s.receiptRepo.UpdateStatus(ctx, txnID, domain.ReceiptReversed); err != nil {
		return err
	}

	meta, _ := json.Marshal(map[string]string{
		"reason":           req.Reason,
		"reference_ticket": req.ReferenceTicket,
	})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditAdminTxnReversed,
		ActorType:    "admin",
		ActorID:      &staffID,
		ResourceType: "transaction",
		ResourceID:   txnID,
		Metadata:     meta,
	})
	return nil
}
