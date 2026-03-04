package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
)

type AdminQueriesSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.AdminQueryRepository
}

func (s *AdminQueriesSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewAdminQueryRepository(s.pool)
}

func (s *AdminQueriesSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *AdminQueriesSuite) TestCountUsers() {
	ctx := context.Background()
	count, err := s.repo.CountUsers(ctx)
	s.Require().NoError(err)
	s.GreaterOrEqual(count, int64(0))
}

func TestAdminQueriesSuite(t *testing.T) {
	suite.Run(t, new(AdminQueriesSuite))
}
