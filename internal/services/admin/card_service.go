package admin

import (
	"context"
	"encoding/json"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
)

type CardService struct {
	adminRepo repository.AdminQueryRepository
	cardRepo  repository.CardRepository
	auditRepo repository.AuditRepository
}

func NewCardService(
	adminRepo repository.AdminQueryRepository,
	cardRepo repository.CardRepository,
	auditRepo repository.AuditRepository,
) *CardService {
	return &CardService{adminRepo: adminRepo, cardRepo: cardRepo, auditRepo: auditRepo}
}

func (s *CardService) List(ctx context.Context, filter domain.CardFilter) (*domain.PaginatedResult[domain.Card], error) {
	return s.adminRepo.ListCards(ctx, filter)
}

func (s *CardService) GetByID(ctx context.Context, id string) (*domain.Card, error) {
	return s.cardRepo.GetByID(ctx, id)
}

func (s *CardService) ListAuthorizations(ctx context.Context, filter domain.CardAuthFilter) (*domain.PaginatedResult[domain.CardAuthorization], error) {
	return s.adminRepo.ListCardAuthorizations(ctx, filter)
}

func (s *CardService) Freeze(ctx context.Context, staffID, cardID string) error {
	if err := s.cardRepo.UpdateStatus(ctx, cardID, domain.CardStatusFrozen); err != nil {
		return err
	}
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action: domain.AuditAdminCardFreeze, ActorType: "admin",
		ActorID: &staffID, ResourceType: "card", ResourceID: cardID,
	})
	return nil
}

func (s *CardService) Cancel(ctx context.Context, staffID, cardID, reason string) error {
	if err := s.cardRepo.UpdateStatus(ctx, cardID, domain.CardStatusCancelled); err != nil {
		return err
	}
	meta, _ := json.Marshal(map[string]string{"reason": reason})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action: domain.AuditAdminCardCancel, ActorType: "admin",
		ActorID: &staffID, ResourceType: "card", ResourceID: cardID,
		Metadata: meta,
	})
	return nil
}

func (s *CardService) Unfreeze(ctx context.Context, staffID, cardID string) error {
	if err := s.cardRepo.UpdateStatus(ctx, cardID, domain.CardStatusActive); err != nil {
		return err
	}
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action: domain.AuditCardUnfrozen, ActorType: "admin",
		ActorID: &staffID, ResourceType: "card", ResourceID: cardID,
	})
	return nil
}

func (s *CardService) UpdateLimits(ctx context.Context, staffID, cardID string, daily, monthly, perTxn int64) error {
	if err := s.cardRepo.UpdateLimits(ctx, cardID, daily, monthly, perTxn); err != nil {
		return err
	}
	meta, _ := json.Marshal(map[string]int64{
		"daily_limit_cents": daily, "monthly_limit_cents": monthly, "per_txn_limit_cents": perTxn,
	})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action: domain.AuditCardLimitChanged, ActorType: "admin",
		ActorID: &staffID, ResourceType: "card", ResourceID: cardID,
		Metadata: meta,
	})
	return nil
}
