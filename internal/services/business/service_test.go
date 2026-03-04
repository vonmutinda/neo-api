package business_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/business"
	"github.com/vonmutinda/neo/internal/services/permissions"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type BusinessSuite struct {
	suite.Suite
	pool       *pgxpool.Pool
	svc        *business.Service
	bizRepo    repository.BusinessRepository
	memberRepo repository.BusinessMemberRepository
	roleRepo   repository.BusinessRoleRepository
	auditRepo  repository.AuditRepository
	userRepo   repository.UserRepository
}

func (s *BusinessSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.bizRepo = repository.NewBusinessRepository(s.pool)
	s.memberRepo = repository.NewBusinessMemberRepository(s.pool)
	s.roleRepo = repository.NewBusinessRoleRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)
	s.userRepo = repository.NewUserRepository(s.pool)

	mockLedger := testutil.NewMockLedgerClient()
	mockCache := testutil.NewMockCache()
	permSvc := permissions.NewService(s.roleRepo, s.memberRepo, 0, mockCache)

	s.svc = business.NewService(s.bizRepo, s.memberRepo, s.roleRepo, s.userRepo, s.auditRepo, mockLedger, permSvc)
}

func (s *BusinessSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
	testutil.SeedBusinessSystemRoles(s.T(), s.pool)
}

func (s *BusinessSuite) TestRegisterBusiness_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911111111"))

	biz, err := s.svc.RegisterBusiness(ctx, userID, &business.RegisterRequest{
		Name:               "Test Biz",
		TINNumber:          "123456789",
		TradeLicenseNumber: "TL-12345",
		IndustryCategory:   "retail",
		PhoneNumber:        phone.MustParse("+251911000001"),
	})
	s.Require().NoError(err)
	s.NotEmpty(biz.ID)
	s.Equal("Test Biz", biz.Name)
	s.Equal(domain.BusinessStatusPendingVerification, biz.Status)

	fetched, err := s.bizRepo.GetByID(ctx, biz.ID)
	s.Require().NoError(err)
	s.Equal(biz.ID, fetched.ID)

	members, err := s.memberRepo.ListByBusiness(ctx, biz.ID)
	s.Require().NoError(err)
	s.Require().Len(members, 1)
	s.Equal(userID, members[0].UserID)
	s.Equal(domain.SystemRoleOwnerID, members[0].RoleID)
	s.True(members[0].IsActive)
}

func (s *BusinessSuite) TestInviteMember_Success() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	inviteeID := uuid.NewString()
	inviteePhone := phone.MustParse("+251922334455")

	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, inviteeID, inviteePhone)

	biz, err := s.svc.RegisterBusiness(ctx, ownerID, &business.RegisterRequest{
		Name:               "Invite Corp",
		TINNumber:          "987654321",
		TradeLicenseNumber: "TL-99999",
		IndustryCategory:   "technology",
		PhoneNumber:        phone.MustParse("+251911000002"),
	})
	s.Require().NoError(err)

	roles, err := s.svc.ListRoles(ctx, biz.ID)
	s.Require().NoError(err)
	var viewerRoleID string
	for _, r := range roles {
		if r.Name == "viewer" {
			viewerRoleID = r.ID
			break
		}
	}
	s.Require().NotEmpty(viewerRoleID, "viewer system role must exist")

	member, err := s.svc.InviteMember(ctx, biz.ID, ownerID, &business.InviteMemberRequest{
		PhoneNumber: inviteePhone,
		RoleID:      viewerRoleID,
	})
	s.Require().NoError(err)
	s.NotEmpty(member.ID)
	s.Equal(inviteeID, member.UserID)
	s.Equal(viewerRoleID, member.RoleID)
	s.True(member.IsActive)

	allMembers, err := s.memberRepo.ListByBusiness(ctx, biz.ID)
	s.Require().NoError(err)
	s.Len(allMembers, 2)
}

func (s *BusinessSuite) TestRemoveMember_Success() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	memberUserID := uuid.NewString()
	memberPhone := phone.MustParse("+251933445566")

	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, memberUserID, memberPhone)

	biz, err := s.svc.RegisterBusiness(ctx, ownerID, &business.RegisterRequest{
		Name:               "Remove Corp",
		TINNumber:          "111222333",
		TradeLicenseNumber: "TL-55555",
		IndustryCategory:   "agriculture",
		PhoneNumber:        phone.MustParse("+251911000003"),
	})
	s.Require().NoError(err)

	roles, err := s.svc.ListRoles(ctx, biz.ID)
	s.Require().NoError(err)
	var viewerRoleID string
	for _, r := range roles {
		if r.Name == "viewer" {
			viewerRoleID = r.ID
			break
		}
	}
	s.Require().NotEmpty(viewerRoleID)

	member, err := s.svc.InviteMember(ctx, biz.ID, ownerID, &business.InviteMemberRequest{
		PhoneNumber: memberPhone,
		RoleID:      viewerRoleID,
	})
	s.Require().NoError(err)

	err = s.svc.RemoveMember(ctx, biz.ID, member.ID, ownerID)
	s.Require().NoError(err)

	remaining, err := s.memberRepo.ListByBusiness(ctx, biz.ID)
	s.Require().NoError(err)
	s.Len(remaining, 1, "only the owner should remain active")
	s.Equal(ownerID, remaining[0].UserID)
}

func TestBusinessSuite(t *testing.T) {
	suite.Run(t, new(BusinessSuite))
}
