package admin

import (
	"context"
	"encoding/json"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
)

type FlagService struct {
	flagRepo  repository.FlagRepository
	auditRepo repository.AuditRepository
}

func NewFlagService(flagRepo repository.FlagRepository, auditRepo repository.AuditRepository) *FlagService {
	return &FlagService{flagRepo: flagRepo, auditRepo: auditRepo}
}

func (s *FlagService) List(ctx context.Context, filter domain.FlagFilter) (*domain.PaginatedResult[domain.CustomerFlag], error) {
	return s.flagRepo.ListAll(ctx, filter)
}

func (s *FlagService) ListByUser(ctx context.Context, userID string) ([]domain.CustomerFlag, error) {
	return s.flagRepo.ListByUser(ctx, userID)
}

type CreateFlagRequest struct {
	UserID      string              `json:"userId" validate:"required"`
	FlagType    string              `json:"flagType" validate:"required"`
	Severity    domain.FlagSeverity `json:"severity" validate:"required"`
	Description string              `json:"description" validate:"required"`
}

func (s *FlagService) Create(ctx context.Context, staffID string, req CreateFlagRequest) (*domain.CustomerFlag, error) {
	flag := &domain.CustomerFlag{
		UserID:      req.UserID,
		FlagType:    req.FlagType,
		Severity:    req.Severity,
		Description: req.Description,
		CreatedBy:   &staffID,
	}

	if err := s.flagRepo.Create(ctx, flag); err != nil {
		return nil, err
	}

	meta, _ := json.Marshal(map[string]string{
		"flag_type": req.FlagType,
		"severity":  string(req.Severity),
	})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditFlagCreated,
		ActorType:    "admin",
		ActorID:      &staffID,
		ResourceType: "user",
		ResourceID:   req.UserID,
		Metadata:     meta,
	})

	return flag, nil
}

type ResolveFlagRequest struct {
	Note string `json:"note" validate:"required"`
}

func (s *FlagService) Resolve(ctx context.Context, staffID, flagID string, req ResolveFlagRequest) error {
	flag, err := s.flagRepo.GetByID(ctx, flagID)
	if err != nil {
		return err
	}

	if err := s.flagRepo.Resolve(ctx, flagID, staffID, req.Note); err != nil {
		return err
	}

	meta, _ := json.Marshal(map[string]string{"note": req.Note})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditFlagResolved,
		ActorType:    "admin",
		ActorID:      &staffID,
		ResourceType: "user",
		ResourceID:   flag.UserID,
		Metadata:     meta,
	})
	return nil
}
