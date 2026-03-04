package beneficiary

import (
	"context"
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
)

type Service struct {
	beneficiaries repository.BeneficiaryRepository
	audit         repository.AuditRepository
}

func NewService(
	beneficiaries repository.BeneficiaryRepository,
	audit repository.AuditRepository,
) *Service {
	return &Service{beneficiaries: beneficiaries, audit: audit}
}

func (s *Service) Create(ctx context.Context, userID string, req *CreateBeneficiaryRequest) (*domain.Beneficiary, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	b := &domain.Beneficiary{
		UserID:       userID,
		FullName:     req.FullName,
		Relationship: domain.BeneficiaryRelType(req.Relationship),
		IsVerified:   false,
	}
	if req.DocumentURL != "" {
		b.DocumentURL = &req.DocumentURL
	}

	if err := s.beneficiaries.Create(ctx, b); err != nil {
		return nil, fmt.Errorf("creating beneficiary: %w", err)
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditBeneficiaryAdded,
		ActorType:    "user",
		ActorID:      &userID,
		ResourceType: "beneficiary",
		ResourceID:   b.ID,
	})

	return b, nil
}

func (s *Service) List(ctx context.Context, userID string) ([]domain.Beneficiary, error) {
	return s.beneficiaries.ListByUser(ctx, userID)
}

func (s *Service) Delete(ctx context.Context, id, userID string) error {
	if err := s.beneficiaries.SoftDelete(ctx, id, userID); err != nil {
		return err
	}

	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditBeneficiaryRemoved,
		ActorType:    "user",
		ActorID:      &userID,
		ResourceType: "beneficiary",
		ResourceID:   id,
	})

	return nil
}
