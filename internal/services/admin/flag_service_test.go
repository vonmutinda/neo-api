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

type AdminFlagSuite struct {
	suite.Suite
	pool      *pgxpool.Pool
	flagRepo  repository.FlagRepository
	auditRepo repository.AuditRepository
	svc       *admin.FlagService
}

func (s *AdminFlagSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.flagRepo = repository.NewFlagRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)

	s.svc = admin.NewFlagService(s.flagRepo, s.auditRepo)
}

func (s *AdminFlagSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *AdminFlagSuite) TestCreate_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	staff := testutil.SeedStaff(s.T(), s.pool, uuid.NewString(), "flag-admin@neo.com", "pass1234", domain.RoleSuperAdmin)

	flag, err := s.svc.Create(context.Background(), staff.ID, admin.CreateFlagRequest{
		UserID:      user.ID,
		FlagType:    "suspicious_activity",
		Severity:    domain.FlagSeverityWarning,
		Description: "test flag description",
	})
	s.Require().NoError(err)
	s.NotEmpty(flag.ID)
	s.Equal(user.ID, flag.UserID)
	s.Equal("suspicious_activity", flag.FlagType)
	s.Equal(domain.FlagSeverityWarning, flag.Severity)
}

func (s *AdminFlagSuite) TestListByUser() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedFlag(s.T(), s.pool, user.ID, "suspicious_activity", domain.FlagSeverityWarning)
	testutil.SeedFlag(s.T(), s.pool, user.ID, "identity_mismatch", domain.FlagSeverityCritical)

	flags, err := s.svc.ListByUser(context.Background(), user.ID)
	s.Require().NoError(err)
	s.Len(flags, 2)
}

func (s *AdminFlagSuite) TestResolve_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	flagID := testutil.SeedFlag(s.T(), s.pool, user.ID, "suspicious_activity", domain.FlagSeverityWarning)
	staff := testutil.SeedStaff(s.T(), s.pool, uuid.NewString(), "resolve-admin@neo.com", "pass1234", domain.RoleSuperAdmin)

	err := s.svc.Resolve(context.Background(), staff.ID, flagID, admin.ResolveFlagRequest{
		Note: "investigated and cleared",
	})
	s.Require().NoError(err)

	var isResolved bool
	err = s.pool.QueryRow(context.Background(),
		"SELECT is_resolved FROM customer_flags WHERE id=$1", flagID,
	).Scan(&isResolved)
	s.Require().NoError(err)
	s.True(isResolved)
}

func TestAdminFlagSuite(t *testing.T) {
	suite.Run(t, new(AdminFlagSuite))
}
