package admin

import (
	"context"
	"encoding/json"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
)

type CustomerService struct {
	adminRepo repository.AdminQueryRepository
	userRepo  repository.UserRepository
	kycRepo   repository.KYCRepository
	flagRepo  repository.FlagRepository
	auditRepo repository.AuditRepository
	loanRepo  repository.LoanRepository
}

func NewCustomerService(
	adminRepo repository.AdminQueryRepository,
	userRepo repository.UserRepository,
	kycRepo repository.KYCRepository,
	flagRepo repository.FlagRepository,
	auditRepo repository.AuditRepository,
	loanRepo repository.LoanRepository,
) *CustomerService {
	return &CustomerService{
		adminRepo: adminRepo,
		userRepo:  userRepo,
		kycRepo:   kycRepo,
		flagRepo:  flagRepo,
		auditRepo: auditRepo,
		loanRepo:  loanRepo,
	}
}

func (s *CustomerService) List(ctx context.Context, filter domain.UserFilter) (*domain.PaginatedResult[domain.User], error) {
	return s.adminRepo.ListUsers(ctx, filter)
}

type CustomerProfile struct {
	User               domain.User                `json:"user"`
	KYCVerifications   []domain.KYCVerification   `json:"kycVerifications"`
	Flags              []domain.CustomerFlag       `json:"flags"`
	CreditProfile      *domain.CreditProfile       `json:"creditProfile,omitempty"`
	ActiveLoans        int64                        `json:"activeLoans"`
	ActiveCards        int64                        `json:"activeCards"`
	RecentTransactions []domain.TransactionReceipt  `json:"recentTransactions"`
}

func (s *CustomerService) GetProfile(ctx context.Context, userID string) (*CustomerProfile, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	kycList, _ := s.kycRepo.GetByUserID(ctx, userID)
	flags, _ := s.flagRepo.ListByUser(ctx, userID)

	profile := &CustomerProfile{
		User:             *user,
		KYCVerifications: kycList,
		Flags:            flags,
	}

	profile.CreditProfile, _ = s.loanRepo.GetCreditProfile(ctx, userID)

	activeStatus := string(domain.LoanActive)
	loanResult, _ := s.adminRepo.ListLoans(ctx, domain.LoanFilter{
		UserID: &userID,
		Status: &activeStatus,
		Limit:  1,
	})
	if loanResult != nil {
		profile.ActiveLoans = loanResult.Pagination.Total
	}

	cardActive := string(domain.CardStatusActive)
	cardResult, _ := s.adminRepo.ListCards(ctx, domain.CardFilter{
		UserID: &userID,
		Status: &cardActive,
		Limit:  1,
	})
	if cardResult != nil {
		profile.ActiveCards = cardResult.Pagination.Total
	}

	txnResult, _ := s.adminRepo.ListTransactions(ctx, domain.TransactionFilter{
		UserID: &userID,
		Limit:  10,
	})
	if txnResult != nil {
		profile.RecentTransactions = txnResult.Data
	}

	return profile, nil
}

func ptrTo[T any](v T) *T { return &v }

type FreezeRequest struct {
	Reason string `json:"reason" validate:"required"`
}

func (s *CustomerService) Freeze(ctx context.Context, staffID, userID string, req FreezeRequest) error {
	if err := s.userRepo.Freeze(ctx, userID, req.Reason); err != nil {
		return err
	}

	meta, _ := json.Marshal(map[string]string{"reason": req.Reason})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditAdminFreeze,
		ActorType:    "admin",
		ActorID:      &staffID,
		ResourceType: "user",
		ResourceID:   userID,
		Metadata:     meta,
	})
	return nil
}

func (s *CustomerService) Unfreeze(ctx context.Context, staffID, userID string) error {
	if err := s.userRepo.Unfreeze(ctx, userID); err != nil {
		return err
	}

	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditAdminUnfreeze,
		ActorType:    "admin",
		ActorID:      &staffID,
		ResourceType: "user",
		ResourceID:   userID,
	})
	return nil
}

type KYCOverrideRequest struct {
	Level  domain.KYCLevel `json:"level" validate:"required"`
	Reason string          `json:"reason" validate:"required"`
}

func (s *CustomerService) OverrideKYC(ctx context.Context, staffID, userID string, req KYCOverrideRequest) error {
	if err := s.userRepo.UpdateKYCLevel(ctx, userID, req.Level); err != nil {
		return err
	}

	meta, _ := json.Marshal(map[string]any{"reason": req.Reason, "level": req.Level})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditAdminKYCOverride,
		ActorType:    "admin",
		ActorID:      &staffID,
		ResourceType: "user",
		ResourceID:   userID,
		Metadata:     meta,
	})
	return nil
}

type AddNoteRequest struct {
	Note string `json:"note" validate:"required"`
}

func (s *CustomerService) AddNote(ctx context.Context, staffID, userID string, req AddNoteRequest) error {
	meta, _ := json.Marshal(map[string]string{"note": req.Note})
	return s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditAdminNote,
		ActorType:    "admin",
		ActorID:      &staffID,
		ResourceType: "user",
		ResourceID:   userID,
		Metadata:     meta,
	})
}
