package admin_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	admin "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type AdminCustomerSuite struct {
	suite.Suite
	pool      *pgxpool.Pool
	adminRepo repository.AdminQueryRepository
	userRepo  repository.UserRepository
	kycRepo   repository.KYCRepository
	flagRepo  repository.FlagRepository
	auditRepo repository.AuditRepository
	loanRepo  repository.LoanRepository
	svc       *admin.CustomerService
}

func (s *AdminCustomerSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.adminRepo = repository.NewAdminQueryRepository(s.pool)
	s.userRepo = repository.NewUserRepository(s.pool)
	s.kycRepo = repository.NewKYCRepository(s.pool)
	s.flagRepo = repository.NewFlagRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)
	s.loanRepo = repository.NewLoanRepository(s.pool)
	s.svc = admin.NewCustomerService(s.adminRepo, s.userRepo, s.kycRepo, s.flagRepo, s.auditRepo, s.loanRepo)
}

func (s *AdminCustomerSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *AdminCustomerSuite) TestList_ReturnsUsers() {
	testutil.SeedUser(s.T(), s.pool, uuid.New().String(), phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, uuid.New().String(), phone.MustParse("+251912345679"))

	result, err := s.svc.List(context.Background(), domain.UserFilter{Limit: 10})
	s.Require().NoError(err)
	s.GreaterOrEqual(int(result.Pagination.Total), 2)
}

func (s *AdminCustomerSuite) TestGetProfile_Success() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.New().String(), phone.MustParse("+251912345678"))

	profile, err := s.svc.GetProfile(context.Background(), user.ID)
	s.Require().NoError(err)
	s.Equal(user.ID, profile.User.ID)
}

func (s *AdminCustomerSuite) TestGetProfile_NotFound() {
	_, err := s.svc.GetProfile(context.Background(), uuid.New().String())
	s.Require().Error(err)
}

func (s *AdminCustomerSuite) TestFreeze_Success() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.New().String(), phone.MustParse("+251912345678"))
	staffID := uuid.New().String()
	testutil.SeedStaff(s.T(), s.pool, staffID, "admin@neo.com", "password123", domain.RoleSuperAdmin)

	err := s.svc.Freeze(context.Background(), staffID, user.ID, admin.FreezeRequest{
		Reason: "suspicious activity",
	})
	s.Require().NoError(err)

	var isFrozen bool
	err = s.pool.QueryRow(context.Background(),
		`SELECT is_frozen FROM users WHERE id = $1`, user.ID).Scan(&isFrozen)
	s.Require().NoError(err)
	s.True(isFrozen)
}

func (s *AdminCustomerSuite) TestUnfreeze_Success() {
	user := testutil.SeedFrozenUser(s.T(), s.pool, uuid.New().String(), phone.MustParse("+251912345678"), "test freeze")
	staffID := uuid.New().String()
	testutil.SeedStaff(s.T(), s.pool, staffID, "admin@neo.com", "password123", domain.RoleSuperAdmin)

	err := s.svc.Unfreeze(context.Background(), staffID, user.ID)
	s.Require().NoError(err)

	var isFrozen bool
	err = s.pool.QueryRow(context.Background(),
		`SELECT is_frozen FROM users WHERE id = $1`, user.ID).Scan(&isFrozen)
	s.Require().NoError(err)
	s.False(isFrozen)
}

func (s *AdminCustomerSuite) TestOverrideKYC_Success() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.New().String(), phone.MustParse("+251912345678"))
	staffID := uuid.New().String()
	testutil.SeedStaff(s.T(), s.pool, staffID, "admin@neo.com", "password123", domain.RoleSuperAdmin)

	err := s.svc.OverrideKYC(context.Background(), staffID, user.ID, admin.KYCOverrideRequest{
		Level:  domain.KYCBasic,
		Reason: "manual override",
	})
	s.Require().NoError(err)

	var kycLevel int
	err = s.pool.QueryRow(context.Background(),
		`SELECT kyc_level FROM users WHERE id = $1`, user.ID).Scan(&kycLevel)
	s.Require().NoError(err)
	s.Equal(int(domain.KYCBasic), kycLevel)
}

func (s *AdminCustomerSuite) TestAddNote_Success() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.New().String(), phone.MustParse("+251912345678"))
	staffID := uuid.New().String()
	testutil.SeedStaff(s.T(), s.pool, staffID, "admin@neo.com", "password123", domain.RoleSuperAdmin)

	err := s.svc.AddNote(context.Background(), staffID, user.ID, admin.AddNoteRequest{
		Note: "Customer called about account",
	})
	s.Require().NoError(err)

	entries, err := s.auditRepo.ListByResource(context.Background(), "user", user.ID, 10)
	s.Require().NoError(err)
	s.NotEmpty(entries)
	s.Equal(domain.AuditAdminNote, entries[0].Action)
}

func TestAdminCustomerSuite(t *testing.T) {
	suite.Run(t, new(AdminCustomerSuite))
}
