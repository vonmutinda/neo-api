package business

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/permissions"
	"github.com/vonmutinda/neo/pkg/phone"
)

// DefaultBalanceCreator creates the default ETB currency balance for a business.
type DefaultBalanceCreator interface {
	CreateDefaultBalance(ctx context.Context, userID string) (*domain.CurrencyBalance, *domain.AccountDetails, error)
}

// Service handles business entity lifecycle, member management, and role CRUD.
type Service struct {
	bizRepo    repository.BusinessRepository
	memberRepo repository.BusinessMemberRepository
	roleRepo   repository.BusinessRoleRepository
	userRepo   repository.UserRepository
	auditRepo  repository.AuditRepository
	ledger     ledger.Client
	permSvc    *permissions.Service
}

func NewService(
	bizRepo repository.BusinessRepository,
	memberRepo repository.BusinessMemberRepository,
	roleRepo repository.BusinessRoleRepository,
	userRepo repository.UserRepository,
	auditRepo repository.AuditRepository,
	ledgerClient ledger.Client,
	permSvc *permissions.Service,
) *Service {
	return &Service{
		bizRepo:    bizRepo,
		memberRepo: memberRepo,
		roleRepo:   roleRepo,
		userRepo:   userRepo,
		auditRepo:  auditRepo,
		ledger:     ledgerClient,
		permSvc:    permSvc,
	}
}

// RegisterBusiness creates a new business for the authenticated user.
func (s *Service) RegisterBusiness(ctx context.Context, ownerUserID string, req *RegisterRequest) (*domain.Business, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	if _, err := s.userRepo.GetByID(ctx, ownerUserID); err != nil {
		return nil, fmt.Errorf("looking up owner: %w", err)
	}

	walletID := uuid.NewString()
	bizID := uuid.NewString()

	tradeName := strPtr(req.TradeName)
	email := strPtr(req.Email)
	city := strPtr(req.City)
	subCity := strPtr(req.SubCity)
	industrySub := strPtr(req.IndustrySubCategory)

	biz := &domain.Business{
		ID:                  bizID,
		OwnerUserID:         ownerUserID,
		Name:                req.Name,
		TradeName:           tradeName,
		TINNumber:           req.TINNumber,
		TradeLicenseNumber:  req.TradeLicenseNumber,
		IndustryCategory:    domain.IndustryCategory(req.IndustryCategory),
		IndustrySubCategory: industrySub,
		PhoneNumber:         req.PhoneNumber,
		Email:               email,
		City:                city,
		SubCity:             subCity,
		Status:              domain.BusinessStatusPendingVerification,
		LedgerWalletID:      "biz-wallet:" + walletID,
		KYBLevel:            0,
	}

	if err := s.ledger.CreateWallet(ctx, walletID, req.Name); err != nil {
		return nil, fmt.Errorf("provisioning business wallet: %w", err)
	}

	if err := s.bizRepo.Create(ctx, biz); err != nil {
		return nil, fmt.Errorf("creating business: %w", err)
	}

	ownerRole, err := s.roleRepo.GetSystemRoleByName(ctx, "owner")
	if err != nil {
		return nil, fmt.Errorf("loading owner role: %w", err)
	}

	member := &domain.BusinessMember{
		BusinessID: bizID,
		UserID:     ownerUserID,
		RoleID:     ownerRole.ID,
		InvitedBy:  ownerUserID,
		IsActive:   true,
	}
	if err := s.memberRepo.Create(ctx, member); err != nil {
		return nil, fmt.Errorf("creating owner membership: %w", err)
	}

	meta, _ := json.Marshal(map[string]string{"business_id": bizID})
	_ = s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditBusinessRegistered,
		ActorType:    "user",
		ActorID:      &ownerUserID,
		ResourceType: "business",
		ResourceID:   bizID,
		Metadata:     meta,
	})

	return biz, nil
}

// GetBusiness returns a business by ID.
func (s *Service) GetBusiness(ctx context.Context, id string) (*domain.Business, error) {
	return s.bizRepo.GetByID(ctx, id)
}

// UpdateBusiness updates mutable business fields.
func (s *Service) UpdateBusiness(ctx context.Context, bizID string, req *UpdateBusinessRequest) (*domain.Business, error) {
	biz, err := s.bizRepo.GetByID(ctx, bizID)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		biz.Name = req.Name
	}
	if req.TradeName != "" {
		biz.TradeName = &req.TradeName
	}
	if req.IndustryCategory != "" {
		biz.IndustryCategory = domain.IndustryCategory(req.IndustryCategory)
	}
	if req.IndustrySubCategory != "" {
		biz.IndustrySubCategory = &req.IndustrySubCategory
	}
	if req.Address != "" {
		biz.Address = &req.Address
	}
	if req.City != "" {
		biz.City = &req.City
	}
	if req.SubCity != "" {
		biz.SubCity = &req.SubCity
	}
	if req.Woreda != "" {
		biz.Woreda = &req.Woreda
	}
	if req.PhoneNumber != (phone.PhoneNumber{}) {
		biz.PhoneNumber = req.PhoneNumber
	}
	if req.Email != "" {
		biz.Email = &req.Email
	}
	if req.Website != "" {
		biz.Website = &req.Website
	}

	if err := s.bizRepo.Update(ctx, biz); err != nil {
		return nil, err
	}

	meta, _ := json.Marshal(map[string]string{"business_id": bizID})
	_ = s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditBusinessUpdated,
		ActorType:    "user",
		ResourceType: "business",
		ResourceID:   bizID,
		Metadata:     meta,
	})

	return biz, nil
}

// ListMyBusinesses returns all businesses where the user is a member.
func (s *Service) ListMyBusinesses(ctx context.Context, userID string) ([]domain.Business, error) {
	return s.bizRepo.ListByMember(ctx, userID)
}

// --- Member Management ---

func (s *Service) InviteMember(ctx context.Context, bizID, inviterUserID string, req *InviteMemberRequest) (*domain.BusinessMember, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByPhone(ctx, req.PhoneNumber)
	if err != nil {
		return nil, fmt.Errorf("looking up user by phone: %w", err)
	}

	if _, err := s.roleRepo.GetByID(ctx, req.RoleID); err != nil {
		return nil, err
	}

	existing, err := s.memberRepo.GetByBusinessAndUser(ctx, bizID, user.ID)
	if err == nil && existing.IsActive {
		return nil, domain.ErrAlreadyMember
	}

	member := &domain.BusinessMember{
		BusinessID: bizID,
		UserID:     user.ID,
		RoleID:     req.RoleID,
		Title:      req.Title,
		InvitedBy:  inviterUserID,
		IsActive:   true,
	}
	if err := s.memberRepo.Create(ctx, member); err != nil {
		return nil, fmt.Errorf("creating member: %w", err)
	}

	meta, _ := json.Marshal(map[string]string{
		"business_id":    bizID,
		"member_user_id": user.ID,
		"role_id":        req.RoleID,
	})
	_ = s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditBusinessMemberInvited,
		ActorType:    "user",
		ActorID:      &inviterUserID,
		ResourceType: "business",
		ResourceID:   bizID,
		Metadata:     meta,
	})

	return member, nil
}

func (s *Service) UpdateMemberRole(ctx context.Context, bizID, memberID, actorUserID string, req *UpdateMemberRoleRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}

	member, err := s.memberRepo.GetByID(ctx, memberID)
	if err != nil {
		return err
	}
	if member.BusinessID != bizID {
		return domain.ErrMemberNotFound
	}

	if member.RoleID == domain.SystemRoleOwnerID {
		return domain.ErrCannotRemoveOwner
	}

	if _, err := s.roleRepo.GetByID(ctx, req.RoleID); err != nil {
		return err
	}

	if err := s.memberRepo.UpdateRole(ctx, memberID, req.RoleID); err != nil {
		return err
	}

	s.permSvc.InvalidateCache(bizID)

	meta, _ := json.Marshal(map[string]string{
		"business_id": bizID,
		"member_id":   memberID,
		"old_role_id": member.RoleID,
		"new_role_id": req.RoleID,
	})
	_ = s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditBusinessMemberRoleChanged,
		ActorType:    "user",
		ActorID:      &actorUserID,
		ResourceType: "business",
		ResourceID:   bizID,
		Metadata:     meta,
	})

	return nil
}

func (s *Service) RemoveMember(ctx context.Context, bizID, memberID, actorUserID string) error {
	member, err := s.memberRepo.GetByID(ctx, memberID)
	if err != nil {
		return err
	}
	if member.BusinessID != bizID {
		return domain.ErrMemberNotFound
	}

	if member.RoleID == domain.SystemRoleOwnerID {
		return domain.ErrCannotRemoveOwner
	}

	if err := s.memberRepo.Remove(ctx, memberID, actorUserID); err != nil {
		return err
	}

	meta, _ := json.Marshal(map[string]string{
		"business_id":    bizID,
		"member_id":      memberID,
		"member_user_id": member.UserID,
	})
	_ = s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditBusinessMemberRemoved,
		ActorType:    "user",
		ActorID:      &actorUserID,
		ResourceType: "business",
		ResourceID:   bizID,
		Metadata:     meta,
	})

	return nil
}

func (s *Service) ListMembers(ctx context.Context, bizID string) ([]domain.BusinessMember, error) {
	members, err := s.memberRepo.ListByBusiness(ctx, bizID)
	if err != nil {
		return nil, err
	}
	for i := range members {
		role, roleErr := s.roleRepo.GetByID(ctx, members[i].RoleID)
		if roleErr == nil {
			members[i].Role = role
		}
	}
	return members, nil
}

// --- Role Management ---

func (s *Service) CreateRole(ctx context.Context, bizID, actorUserID string, req *CreateRoleRequest) (*domain.BusinessRole, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	perms := make([]domain.BusinessPermission, len(req.Permissions))
	for i, p := range req.Permissions {
		perms[i] = domain.BusinessPermission(p)
	}

	desc := strPtr(req.Description)
	role := &domain.BusinessRole{
		BusinessID:                 &bizID,
		Name:                       req.Name,
		Description:                desc,
		MaxTransferCents:           req.MaxTransferCents,
		DailyTransferLimitCents:    req.DailyTransferLimitCents,
		RequiresApprovalAboveCents: req.RequiresApprovalAboveCents,
	}

	if err := s.roleRepo.Create(ctx, role); err != nil {
		return nil, fmt.Errorf("creating role: %w", err)
	}

	if err := s.roleRepo.SetPermissions(ctx, role.ID, perms); err != nil {
		return nil, fmt.Errorf("setting permissions: %w", err)
	}
	role.Permissions = perms

	s.permSvc.InvalidateCache(bizID)

	meta, _ := json.Marshal(map[string]string{
		"business_id": bizID,
		"role_id":     role.ID,
		"role_name":   req.Name,
	})
	_ = s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditBusinessRoleCreated,
		ActorType:    "user",
		ActorID:      &actorUserID,
		ResourceType: "business",
		ResourceID:   bizID,
		Metadata:     meta,
	})

	return role, nil
}

func (s *Service) UpdateRole(ctx context.Context, bizID, roleID, actorUserID string, req *UpdateRoleRequest) (*domain.BusinessRole, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return nil, err
	}
	if role.IsSystem {
		return nil, domain.ErrSystemRoleImmutable
	}

	if req.Name != "" {
		role.Name = req.Name
	}
	if req.Description != "" {
		role.Description = &req.Description
	}
	if req.MaxTransferCents != nil {
		role.MaxTransferCents = req.MaxTransferCents
	}
	if req.DailyTransferLimitCents != nil {
		role.DailyTransferLimitCents = req.DailyTransferLimitCents
	}
	if req.RequiresApprovalAboveCents != nil {
		role.RequiresApprovalAboveCents = req.RequiresApprovalAboveCents
	}

	if err := s.roleRepo.Update(ctx, role); err != nil {
		return nil, err
	}

	if len(req.Permissions) > 0 {
		perms := make([]domain.BusinessPermission, len(req.Permissions))
		for i, p := range req.Permissions {
			perms[i] = domain.BusinessPermission(p)
		}
		if err := s.roleRepo.SetPermissions(ctx, roleID, perms); err != nil {
			return nil, fmt.Errorf("updating permissions: %w", err)
		}
		role.Permissions = perms
	}

	s.permSvc.InvalidateCache(bizID)

	meta, _ := json.Marshal(map[string]string{
		"business_id": bizID,
		"role_id":     roleID,
	})
	_ = s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditBusinessRoleUpdated,
		ActorType:    "user",
		ActorID:      &actorUserID,
		ResourceType: "business",
		ResourceID:   bizID,
		Metadata:     meta,
	})

	return role, nil
}

func (s *Service) DeleteRole(ctx context.Context, bizID, roleID, actorUserID string) error {
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return err
	}
	if role.IsSystem {
		return domain.ErrSystemRoleImmutable
	}

	count, err := s.roleRepo.CountMembersByRole(ctx, roleID)
	if err != nil {
		return err
	}
	if count > 0 {
		return domain.ErrRoleInUse
	}

	if err := s.roleRepo.Delete(ctx, roleID); err != nil {
		return err
	}

	s.permSvc.InvalidateCache(bizID)

	meta, _ := json.Marshal(map[string]string{
		"business_id": bizID,
		"role_id":     roleID,
		"role_name":   role.Name,
	})
	_ = s.auditRepo.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditBusinessRoleDeleted,
		ActorType:    "user",
		ActorID:      &actorUserID,
		ResourceType: "business",
		ResourceID:   bizID,
		Metadata:     meta,
	})

	return nil
}

func (s *Service) ListRoles(ctx context.Context, bizID string) ([]domain.BusinessRole, error) {
	return s.roleRepo.ListByBusiness(ctx, bizID)
}

func (s *Service) GetRole(ctx context.Context, roleID string) (*domain.BusinessRole, error) {
	return s.roleRepo.GetByID(ctx, roleID)
}

func (s *Service) GetMyPermissions(ctx context.Context, userID, bizID string) ([]domain.BusinessPermission, error) {
	return s.permSvc.GetMemberPermissions(ctx, userID, bizID)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
