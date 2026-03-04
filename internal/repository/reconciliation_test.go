package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
)

type ReconciliationSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.ReconciliationRepository
}

func (s *ReconciliationSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewReconciliationRepository(s.pool)
}

func (s *ReconciliationSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *ReconciliationSuite) TestCreateRun() {
	ctx := context.Background()
	run := &domain.ReconRun{
		RunDate:          time.Now().Truncate(24 * time.Hour),
		ClearingFileName:  "clearing_20250101.csv",
	}
	err := s.repo.CreateRun(ctx, run)
	s.Require().NoError(err)
	s.NotEmpty(run.ID)
	s.NotEmpty(run.StartedAt)
}

func (s *ReconciliationSuite) TestCreateException() {
	ctx := context.Background()
	exc := &domain.ReconException{
		EthSwitchReference:          "ETH-REF-001",
		ErrorType:                   domain.ExceptionMissingInLedger,
		EthSwitchReportedAmountCents: 100000,
		ReconRunDate:                time.Now().Truncate(24 * time.Hour),
	}
	err := s.repo.CreateException(ctx, exc)
	s.Require().NoError(err)
	s.NotEmpty(exc.ID)

	open, err := s.repo.ListOpenExceptions(ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(open)
	s.Equal(exc.ID, open[0].ID)
}

func (s *ReconciliationSuite) TestResolveException() {
	ctx := context.Background()
	exc := &domain.ReconException{
		EthSwitchReference:          "ETH-REF-002",
		ErrorType:                   domain.ExceptionAmountMismatch,
		EthSwitchReportedAmountCents: 50000,
		ReconRunDate:                time.Now().Truncate(24 * time.Hour),
	}
	s.Require().NoError(s.repo.CreateException(ctx, exc))

	err := s.repo.ResolveException(ctx, exc.ID, "Manual adjustment applied", "manual_settle")
	s.Require().NoError(err)

	open, err := s.repo.ListOpenExceptions(ctx)
	s.Require().NoError(err)
	for _, e := range open {
		s.NotEqual(exc.ID, e.ID)
	}
}

func TestReconciliationSuite(t *testing.T) {
	suite.Run(t, new(ReconciliationSuite))
}
