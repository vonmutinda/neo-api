package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type TransferTotalsSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.TransferTotalsRepository
}

func (s *TransferTotalsSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewTransferTotalsRepository(s.pool)
}

func (s *TransferTotalsSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *TransferTotalsSuite) TestIncrement_AndGetDaily() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	err := s.repo.Increment(ctx, userID, "ETB", "outbound", 100000)
	s.Require().NoError(err)

	total, err := s.repo.GetDailyTotal(ctx, userID, "ETB", "outbound")
	s.Require().NoError(err)
	s.Equal(int64(100000), total)

	err = s.repo.Increment(ctx, userID, "ETB", "outbound", 50000)
	s.Require().NoError(err)

	total, err = s.repo.GetDailyTotal(ctx, userID, "ETB", "outbound")
	s.Require().NoError(err)
	s.Equal(int64(150000), total)
}

func (s *TransferTotalsSuite) TestIncrement_AndGetMonthly() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911111111"))

	err := s.repo.Increment(ctx, userID, "ETB", "outbound", 200000)
	s.Require().NoError(err)

	total, err := s.repo.GetMonthlyTotal(ctx, userID, "ETB", "outbound")
	s.Require().NoError(err)
	s.Equal(int64(200000), total)
}

func TestTransferTotalsSuite(t *testing.T) {
	suite.Run(t, new(TransferTotalsSuite))
}
