package admin_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/repository"
	admin "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type AdminLoanSuite struct {
	suite.Suite
	pool      *pgxpool.Pool
	adminRepo repository.AdminQueryRepository
	loanRepo  repository.LoanRepository
	auditRepo repository.AuditRepository
	svc       *admin.LoanService
}

func (s *AdminLoanSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.adminRepo = repository.NewAdminQueryRepository(s.pool)
	s.loanRepo = repository.NewLoanRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)

	s.svc = admin.NewLoanService(s.adminRepo, s.loanRepo, s.auditRepo)
}

func (s *AdminLoanSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *AdminLoanSuite) TestGetByID_NotFound() {
	_, err := s.svc.GetByID(context.Background(), uuid.NewString())
	s.Require().Error(err)
}

func (s *AdminLoanSuite) TestGetCreditProfile_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCreditProfile(s.T(), s.pool, user.ID, 750, 5000000)

	profile, err := s.svc.GetCreditProfile(context.Background(), user.ID)
	s.Require().NoError(err)
	s.Equal(750, profile.TrustScore)
	s.Equal(int64(5000000), profile.ApprovedLimitCents)
}

func (s *AdminLoanSuite) TestOverrideCredit_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCreditProfile(s.T(), s.pool, user.ID, 750, 5000000)
	staffID := uuid.NewString()

	err := s.svc.OverrideCredit(context.Background(), staffID, user.ID, admin.CreditOverrideRequest{
		ApprovedLimitCents: 10000000,
		Reason:             "manual review approved higher limit",
	})
	s.Require().NoError(err)

	var limitCents int64
	err = s.pool.QueryRow(context.Background(),
		"SELECT approved_limit_cents FROM credit_profiles WHERE user_id=$1", user.ID,
	).Scan(&limitCents)
	s.Require().NoError(err)
	s.Equal(int64(10000000), limitCents)
}

func TestAdminLoanSuite(t *testing.T) {
	suite.Run(t, new(AdminLoanSuite))
}
