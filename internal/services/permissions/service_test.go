package permissions_test

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

type PermissionsSuite struct {
	suite.Suite
	pool       *pgxpool.Pool
	svc        *permissions.Service
	bizSvc     *business.Service
	memberRepo repository.BusinessMemberRepository
	roleRepo   repository.BusinessRoleRepository
	userRepo   repository.UserRepository
	mockCache  *testutil.MockCache
}

func (s *PermissionsSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.memberRepo = repository.NewBusinessMemberRepository(s.pool)
	s.roleRepo = repository.NewBusinessRoleRepository(s.pool)
	s.userRepo = repository.NewUserRepository(s.pool)
	s.mockCache = testutil.NewMockCache()

	s.svc = permissions.NewService(s.roleRepo, s.memberRepo, 0, s.mockCache)

	bizRepo := repository.NewBusinessRepository(s.pool)
	auditRepo := repository.NewAuditRepository(s.pool)
	mockLedger := testutil.NewMockLedgerClient()
	s.bizSvc = business.NewService(bizRepo, s.memberRepo, s.roleRepo, s.userRepo, auditRepo, mockLedger, s.svc)
}

func (s *PermissionsSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
	testutil.SeedBusinessSystemRoles(s.T(), s.pool)
	s.mockCache.Data = make(map[string][]byte)
}

func (s *PermissionsSuite) TestHasPermission_Granted() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))

	biz, err := s.bizSvc.RegisterBusiness(ctx, ownerID, &business.RegisterRequest{
		Name:               "Perm Corp",
		TINNumber:          "123456789",
		TradeLicenseNumber: "TL-12345",
		IndustryCategory:   "retail",
		PhoneNumber:        phone.MustParse("+251911000001"),
	})
	s.Require().NoError(err)

	ok, err := s.svc.HasPermission(ctx, ownerID, biz.ID, domain.BPermViewDashboard)
	s.Require().NoError(err)
	s.True(ok)

	ok, err = s.svc.HasPermission(ctx, ownerID, biz.ID, domain.BPermManageMembers)
	s.Require().NoError(err)
	s.True(ok)
}

func (s *PermissionsSuite) TestHasPermission_NotMember() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	outsiderID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, outsiderID, phone.MustParse("+251922334455"))

	biz, err := s.bizSvc.RegisterBusiness(ctx, ownerID, &business.RegisterRequest{
		Name:               "Closed Corp",
		TINNumber:          "987654321",
		TradeLicenseNumber: "TL-99999",
		IndustryCategory:   "technology",
		PhoneNumber:        phone.MustParse("+251911000002"),
	})
	s.Require().NoError(err)

	_, err = s.svc.HasPermission(ctx, outsiderID, biz.ID, domain.BPermViewDashboard)
	s.Require().Error(err)
}

func (s *PermissionsSuite) TestGetMemberPermissions_Owner() {
	ctx := context.Background()
	ownerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, ownerID, phone.MustParse("+251912345678"))

	biz, err := s.bizSvc.RegisterBusiness(ctx, ownerID, &business.RegisterRequest{
		Name:               "Full Perms Corp",
		TINNumber:          "555666777",
		TradeLicenseNumber: "TL-77777",
		IndustryCategory:   "healthcare",
		PhoneNumber:        phone.MustParse("+251911000003"),
	})
	s.Require().NoError(err)

	perms, err := s.svc.GetMemberPermissions(ctx, ownerID, biz.ID)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(perms), 20, "owner role should have all permissions")
}

func TestPermissionsSuite(t *testing.T) {
	suite.Run(t, new(PermissionsSuite))
}
