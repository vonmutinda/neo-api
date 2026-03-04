package admin_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	admin "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/internal/testutil"
)

type AdminReconSuite struct {
	suite.Suite
	pool      *pgxpool.Pool
	adminRepo repository.AdminQueryRepository
	auditRepo repository.AuditRepository
	svc       *admin.ReconService
}

func (s *AdminReconSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.adminRepo = repository.NewAdminQueryRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)

	s.svc = admin.NewReconService(s.adminRepo, s.auditRepo)
}

func (s *AdminReconSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *AdminReconSuite) TestListRuns() {
	result, err := s.svc.ListRuns(context.Background(), 10, 0)
	s.Require().NoError(err)
	s.NotNil(result)
}

func (s *AdminReconSuite) TestListExceptions() {
	result, err := s.svc.ListExceptions(context.Background(), domain.ExceptionFilter{
		Limit:  10,
		Offset: 0,
	})
	s.Require().NoError(err)
	s.NotNil(result)
}

func TestAdminReconSuite(t *testing.T) {
	suite.Run(t, new(AdminReconSuite))
}
