package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type BusinessMemberSuite struct {
	suite.Suite
	pool       *pgxpool.Pool
	memberRepo repository.BusinessMemberRepository
	bizRepo    repository.BusinessRepository
}

func (s *BusinessMemberSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.memberRepo = repository.NewBusinessMemberRepository(s.pool)
	s.bizRepo = repository.NewBusinessRepository(s.pool)
}

func (s *BusinessMemberSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *BusinessMemberSuite) seedBusiness(userID, bizID string) *domain.Business {
	p := phone.MustParse("+251911100011")
	testutil.SeedUser(s.T(), s.pool, userID, p)
	tradeName := "Test Corp Trading"
	biz := &domain.Business{
		ID:                 bizID,
		OwnerUserID:        userID,
		Name:               "Test Corp",
		TradeName:          &tradeName,
		TINNumber:          uuid.NewString()[:10],
		TradeLicenseNumber: "TL-" + uuid.NewString()[:6],
		IndustryCategory:   domain.IndustryRetail,
		Status:             domain.BusinessStatusActive,
		LedgerWalletID:     "wallet:biz-" + uuid.NewString(),
		PhoneNumber: phone.MustParse("+251911100026"),
	}
	s.Require().NoError(s.bizRepo.Create(context.Background(), biz))
	return biz
}

func (s *BusinessMemberSuite) seedRole(bizID, name string) string {
	roleRepo := repository.NewBusinessRoleRepository(s.pool)
	role := &domain.BusinessRole{
		BusinessID: &bizID,
		Name:       name,
		IsSystem:   false,
	}
	s.Require().NoError(roleRepo.Create(context.Background(), role))
	return role.ID
}

func (s *BusinessMemberSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)
	roleID := s.seedRole(bizID, "Custom Role")

	member := &domain.BusinessMember{
		BusinessID: bizID,
		UserID:     userID,
		RoleID:     roleID,
		InvitedBy:  userID,
	}
	err := s.memberRepo.Create(ctx, member)
	s.Require().NoError(err)
	s.NotEmpty(member.ID)
	s.NotEmpty(member.JoinedAt)

	got, err := s.memberRepo.GetByID(ctx, member.ID)
	s.Require().NoError(err)
	s.Equal(member.ID, got.ID)
	s.Equal(bizID, got.BusinessID)
	s.Equal(userID, got.UserID)
	s.Equal(roleID, got.RoleID)
	s.True(got.IsActive)
}

func (s *BusinessMemberSuite) TestGetByBusinessAndUser() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)
	roleID := s.seedRole(bizID, "Custom Role")

	member := &domain.BusinessMember{
		BusinessID: bizID,
		UserID:     userID,
		RoleID:     roleID,
		InvitedBy:  userID,
	}
	s.Require().NoError(s.memberRepo.Create(ctx, member))

	got, err := s.memberRepo.GetByBusinessAndUser(ctx, bizID, userID)
	s.Require().NoError(err)
	s.Equal(member.ID, got.ID)
	s.Equal(bizID, got.BusinessID)
	s.Equal(userID, got.UserID)
}

func (s *BusinessMemberSuite) TestListByBusiness() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	memberUserID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(ownerID, bizID)
	testutil.SeedUser(s.T(), s.pool, memberUserID, phone.MustParse("+251922222222"))
	roleID := s.seedRole(bizID, "Custom Role")

	m1 := &domain.BusinessMember{
		BusinessID: bizID,
		UserID:     ownerID,
		RoleID:     roleID,
		InvitedBy:  ownerID,
	}
	m2 := &domain.BusinessMember{
		BusinessID: bizID,
		UserID:     memberUserID,
		RoleID:     roleID,
		InvitedBy:  ownerID,
	}
	s.Require().NoError(s.memberRepo.Create(ctx, m1))
	s.Require().NoError(s.memberRepo.Create(ctx, m2))

	list, err := s.memberRepo.ListByBusiness(ctx, bizID)
	s.Require().NoError(err)
	s.Len(list, 2)
}

func (s *BusinessMemberSuite) TestUpdateRole() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)
	roleID1 := s.seedRole(bizID, "Role One")
	roleID2 := s.seedRole(bizID, "Role Two")

	member := &domain.BusinessMember{
		BusinessID: bizID,
		UserID:     userID,
		RoleID:     roleID1,
		InvitedBy:  userID,
	}
	s.Require().NoError(s.memberRepo.Create(ctx, member))

	err := s.memberRepo.UpdateRole(ctx, member.ID, roleID2)
	s.Require().NoError(err)

	got, err := s.memberRepo.GetByID(ctx, member.ID)
	s.Require().NoError(err)
	s.Equal(roleID2, got.RoleID)
}

func (s *BusinessMemberSuite) TestRemove() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)
	roleID := s.seedRole(bizID, "Custom Role")

	member := &domain.BusinessMember{
		BusinessID: bizID,
		UserID:     userID,
		RoleID:     roleID,
		InvitedBy:  userID,
	}
	s.Require().NoError(s.memberRepo.Create(ctx, member))

	err := s.memberRepo.Remove(ctx, member.ID, userID)
	s.Require().NoError(err)

	// GetByBusinessAndUser filters by is_active, so removed member should not be found
	_, err = s.memberRepo.GetByBusinessAndUser(ctx, bizID, userID)
	s.ErrorIs(err, domain.ErrMemberNotFound)

	// ListByBusiness also filters by is_active
	list, err := s.memberRepo.ListByBusiness(ctx, bizID)
	s.Require().NoError(err)
	s.Len(list, 0)
}

func TestBusinessMemberSuite(t *testing.T) {
	suite.Run(t, new(BusinessMemberSuite))
}
