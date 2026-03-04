package admin

import (
	"context"
	"encoding/json"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
)

type LoanService struct {
	adminRepo repository.AdminQueryRepository
	loanRepo  repository.LoanRepository
	auditRepo repository.AuditRepository
}

func NewLoanService(
	adminRepo repository.AdminQueryRepository,
	loanRepo repository.LoanRepository,
	auditRepo repository.AuditRepository,
) *LoanService {
	return &LoanService{adminRepo: adminRepo, loanRepo: loanRepo, auditRepo: auditRepo}
}

func (s *LoanService) List(ctx context.Context, filter domain.LoanFilter) (*domain.PaginatedResult[domain.Loan], error) {
	return s.adminRepo.ListLoans(ctx, filter)
}

func (s *LoanService) GetByID(ctx context.Context, id string) (*domain.Loan, error) {
	return s.loanRepo.GetLoan(ctx, id)
}

func (s *LoanService) Summary(ctx context.Context) (*repository.LoanBookSummary, error) {
	return s.adminRepo.LoanBookSummary(ctx)
}

func (s *LoanService) ListCreditProfiles(ctx context.Context, filter domain.CreditProfileFilter) (*domain.PaginatedResult[domain.CreditProfile], error) {
	return s.adminRepo.ListCreditProfiles(ctx, filter)
}

func (s *LoanService) GetCreditProfile(ctx context.Context, userID string) (*domain.CreditProfile, error) {
	return s.loanRepo.GetCreditProfile(ctx, userID)
}

type WriteOffRequest struct {
	Reason          string `json:"reason" validate:"required"`
	ReferenceTicket string `json:"referenceTicket"`
}

func (s *LoanService) WriteOff(ctx context.Context, staffID, loanID string, req WriteOffRequest) error {
	loan, err := s.loanRepo.GetLoan(ctx, loanID)
	if err != nil {
		return err
	}

	if loan.Status != domain.LoanDefaulted {
		return domain.ErrInvalidInput
	}

	if err := s.loanRepo.UpdateLoanStatus(ctx, loanID, domain.LoanWrittenOff, loan.DaysPastDue); err != nil {
		return err
	}

	meta, _ := json.Marshal(map[string]string{
		"reason":           req.Reason,
		"reference_ticket": req.ReferenceTicket,
	})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditAdminLoanWriteOff,
		ActorType:    "admin",
		ActorID:      &staffID,
		ResourceType: "loan",
		ResourceID:   loanID,
		Metadata:     meta,
	})
	return nil
}

type CreditOverrideRequest struct {
	ApprovedLimitCents int64  `json:"approvedLimitCents" validate:"required,gt=0"`
	Reason             string `json:"reason" validate:"required"`
}

func (s *LoanService) OverrideCredit(ctx context.Context, staffID, userID string, req CreditOverrideRequest) error {
	profile, err := s.loanRepo.GetCreditProfile(ctx, userID)
	if err != nil {
		return err
	}

	profile.ApprovedLimitCents = req.ApprovedLimitCents
	if err := s.loanRepo.UpsertCreditProfile(ctx, profile); err != nil {
		return err
	}

	meta, _ := json.Marshal(map[string]any{
		"reason":              req.Reason,
		"approved_limit_cents": req.ApprovedLimitCents,
	})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditAdminCreditOverride,
		ActorType:    "admin",
		ActorID:      &staffID,
		ResourceType: "credit_profile",
		ResourceID:   userID,
		Metadata:     meta,
	})
	return nil
}
