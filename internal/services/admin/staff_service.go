package admin

import (
	"context"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
)

type StaffService struct {
	staffRepo repository.StaffRepository
	auditRepo repository.AuditRepository
}

func NewStaffService(staffRepo repository.StaffRepository, auditRepo repository.AuditRepository) *StaffService {
	return &StaffService{staffRepo: staffRepo, auditRepo: auditRepo}
}

func (s *StaffService) List(ctx context.Context, filter domain.StaffFilter) (*domain.PaginatedResult[domain.Staff], error) {
	return s.staffRepo.ListAll(ctx, filter)
}

func (s *StaffService) GetByID(ctx context.Context, id string) (*domain.Staff, error) {
	return s.staffRepo.GetByID(ctx, id)
}

type UpdateStaffRequest struct {
	FullName   *string           `json:"fullName,omitempty"`
	Role       *domain.StaffRole `json:"role,omitempty"`
	Department *string           `json:"department,omitempty"`
}

func (s *StaffService) Update(ctx context.Context, actorID, staffID string, req UpdateStaffRequest) (*domain.Staff, error) {
	staff, err := s.staffRepo.GetByID(ctx, staffID)
	if err != nil {
		return nil, err
	}

	if req.FullName != nil {
		staff.FullName = *req.FullName
	}
	if req.Role != nil {
		staff.Role = *req.Role
	}
	if req.Department != nil {
		staff.Department = *req.Department
	}

	if err := s.staffRepo.Update(ctx, staff); err != nil {
		return nil, err
	}

	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditStaffUpdated,
		ActorType:    "admin",
		ActorID:      &actorID,
		ResourceType: "staff",
		ResourceID:   staffID,
	})

	return staff, nil
}

func (s *StaffService) Deactivate(ctx context.Context, actorID, staffID string) error {
	if err := s.staffRepo.Deactivate(ctx, staffID); err != nil {
		return err
	}

	s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditStaffDeactivated,
		ActorType:    "admin",
		ActorID:      &actorID,
		ResourceType: "staff",
		ResourceID:   staffID,
	})

	return nil
}
