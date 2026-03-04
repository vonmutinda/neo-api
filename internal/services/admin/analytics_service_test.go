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
	"github.com/vonmutinda/neo/pkg/geo"
	"github.com/vonmutinda/neo/pkg/phone"
)

type AdminAnalyticsSuite struct {
	suite.Suite
	pool      *pgxpool.Pool
	adminRepo repository.AdminQueryRepository
	flagRepo  repository.FlagRepository
	svc       *admin.AnalyticsService
}

func (s *AdminAnalyticsSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.adminRepo = repository.NewAdminQueryRepository(s.pool)
	s.flagRepo = repository.NewFlagRepository(s.pool)

	s.svc = admin.NewAnalyticsService(s.adminRepo, s.flagRepo, geo.NoopLookuper{})
}

func (s *AdminAnalyticsSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *AdminAnalyticsSuite) TestOverview_Empty() {
	overview, err := s.svc.Overview(context.Background())
	s.Require().NoError(err)
	s.NotNil(overview)
	s.Equal(int64(0), overview.TotalCustomers)
	s.Equal(int64(0), overview.ActiveCustomers30d)
	s.Equal(int64(0), overview.FrozenAccounts)
	s.Equal(int64(0), overview.ActiveLoans)
	s.Equal(int64(0), overview.TotalLoanOutstanding)
	s.Equal(int64(0), overview.ActiveCards)
	s.Equal(int64(0), overview.ActivePots)
	s.Equal(int64(0), overview.ActiveBusinesses)
	s.Equal(int64(0), overview.TotalTransactions)
	s.Equal(int64(0), overview.OpenFlags)
}

func (s *AdminAnalyticsSuite) TestOverview_WithData() {
	testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251911000001"))
	testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251911000002"))
	testutil.SeedFrozenUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251911000003"), "compliance review")

	overview, err := s.svc.Overview(context.Background())
	s.Require().NoError(err)
	s.NotNil(overview)
	s.GreaterOrEqual(overview.TotalCustomers, int64(3))
	s.GreaterOrEqual(overview.FrozenAccounts, int64(1))
}

func TestAdminAnalyticsSuite(t *testing.T) {
	suite.Run(t, new(AdminAnalyticsSuite))
}
