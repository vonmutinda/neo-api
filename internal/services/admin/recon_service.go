package admin

import (
	"context"
	"encoding/json"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
)

type ReconService struct {
	adminRepo repository.AdminQueryRepository
	auditRepo repository.AuditRepository
}

func NewReconService(adminRepo repository.AdminQueryRepository, auditRepo repository.AuditRepository) *ReconService {
	return &ReconService{adminRepo: adminRepo, auditRepo: auditRepo}
}

func (s *ReconService) ListRuns(ctx context.Context, limit, offset int) (*domain.PaginatedResult[domain.ReconRun], error) {
	return s.adminRepo.ListReconRuns(ctx, limit, offset)
}

func (s *ReconService) ListExceptions(ctx context.Context, filter domain.ExceptionFilter) (*domain.PaginatedResult[domain.ReconException], error) {
	return s.adminRepo.ListReconExceptions(ctx, filter)
}

func (s *ReconService) Assign(ctx context.Context, staffID, exceptionID, assignedTo string) error {
	if err := s.adminRepo.AssignException(ctx, exceptionID, assignedTo); err != nil {
		return err
	}
	meta, _ := json.Marshal(map[string]string{"assigned_to": assignedTo})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action: domain.AuditReconAssigned, ActorType: "admin",
		ActorID: &staffID, ResourceType: "recon_exception", ResourceID: exceptionID,
		Metadata: meta,
	})
	return nil
}

func (s *ReconService) Investigate(ctx context.Context, staffID, exceptionID string) error {
	if err := s.adminRepo.UpdateExceptionStatus(ctx, exceptionID, domain.ExceptionInvestigating); err != nil {
		return err
	}
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action: domain.AuditReconInvestigating, ActorType: "admin",
		ActorID: &staffID, ResourceType: "recon_exception", ResourceID: exceptionID,
	})
	return nil
}

type ResolveExceptionRequest struct {
	Notes  string `json:"notes" validate:"required"`
	Action string `json:"action" validate:"required"`
}

func (s *ReconService) Resolve(ctx context.Context, staffID, exceptionID string, req ResolveExceptionRequest) error {
	if err := s.adminRepo.UpdateExceptionStatus(ctx, exceptionID, domain.ExceptionResolved); err != nil {
		return err
	}
	meta, _ := json.Marshal(map[string]string{"notes": req.Notes, "action": req.Action})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action: domain.AuditReconExceptionResolved, ActorType: "admin",
		ActorID: &staffID, ResourceType: "recon_exception", ResourceID: exceptionID,
		Metadata: meta,
	})
	return nil
}

func (s *ReconService) Escalate(ctx context.Context, staffID, exceptionID, notes string) error {
	if err := s.adminRepo.EscalateException(ctx, exceptionID, notes); err != nil {
		return err
	}
	meta, _ := json.Marshal(map[string]string{"notes": notes})
	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action: domain.AuditReconEscalated, ActorType: "admin",
		ActorID: &staffID, ResourceType: "recon_exception", ResourceID: exceptionID,
		Metadata: meta,
	})
	return nil
}
