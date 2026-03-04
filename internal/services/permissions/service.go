package permissions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/cache"
)

type Service struct {
	roleRepo   repository.BusinessRoleRepository
	memberRepo repository.BusinessMemberRepository
	cache      cache.Cache
	ttl        time.Duration
}

func NewService(
	roleRepo repository.BusinessRoleRepository,
	memberRepo repository.BusinessMemberRepository,
	cacheTTL time.Duration,
	c cache.Cache,
) *Service {
	if cacheTTL == 0 {
		cacheTTL = 5 * time.Minute
	}
	return &Service{
		roleRepo:   roleRepo,
		memberRepo: memberRepo,
		cache:      c,
		ttl:        cacheTTL,
	}
}

func (s *Service) HasPermission(ctx context.Context, memberUserID, businessID string, perm domain.BusinessPermission) (bool, error) {
	role, err := s.getMemberRole(ctx, memberUserID, businessID)
	if err != nil {
		return false, err
	}
	return role.HasPermission(perm), nil
}

func (s *Service) CheckTransferAllowed(ctx context.Context, memberUserID, businessID string, amountCents int64, transferType string) error {
	role, err := s.getMemberRole(ctx, memberUserID, businessID)
	if err != nil {
		return err
	}

	var needed domain.BusinessPermission
	switch transferType {
	case "internal":
		needed = domain.BPermTransferInternal
	case "external":
		needed = domain.BPermTransferExternal
	default:
		needed = domain.BPermTransferInternal
	}
	if !role.HasPermission(needed) {
		return domain.ErrForbidden
	}

	if role.MaxTransferCents != nil && amountCents > *role.MaxTransferCents {
		return domain.ErrTransferExceedsLimit
	}

	if role.DailyTransferLimitCents != nil {
		todayTotal, err := s.memberRepo.SumTransfersTodayByMember(ctx, memberUserID, businessID)
		if err != nil {
			return fmt.Errorf("checking daily limit: %w", err)
		}
		if todayTotal+amountCents > *role.DailyTransferLimitCents {
			return domain.ErrDailyLimitExceeded
		}
	}

	if role.RequiresApprovalAboveCents != nil && amountCents > *role.RequiresApprovalAboveCents {
		return domain.ErrApprovalRequired
	}

	return nil
}

func (s *Service) GetMemberPermissions(ctx context.Context, memberUserID, businessID string) ([]domain.BusinessPermission, error) {
	role, err := s.getMemberRole(ctx, memberUserID, businessID)
	if err != nil {
		return nil, err
	}
	return role.Permissions, nil
}

func (s *Service) InvalidateCache(businessID string) {
	_ = s.cache.DeleteByPrefix(context.Background(), "neo:roles:"+businessID+":")
}

func (s *Service) getMemberRole(ctx context.Context, memberUserID, businessID string) (*domain.BusinessRole, error) {
	member, err := s.memberRepo.GetByBusinessAndUser(ctx, businessID, memberUserID)
	if err != nil {
		return nil, err
	}
	if !member.IsActive {
		return nil, domain.ErrMemberNotActive
	}

	cacheKey := "neo:roles:" + businessID + ":" + member.RoleID
	if data, ok := s.cache.Get(ctx, cacheKey); ok {
		var role domain.BusinessRole
		if json.Unmarshal(data, &role) == nil {
			return &role, nil
		}
	}

	role, err := s.roleRepo.GetByID(ctx, member.RoleID)
	if err != nil {
		return nil, fmt.Errorf("loading role %s: %w", member.RoleID, err)
	}

	if data, err := json.Marshal(role); err == nil {
		_ = s.cache.Set(ctx, cacheKey, data, s.ttl)
	}
	return role, nil
}
