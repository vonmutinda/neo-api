package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
)

type RemittanceProviderSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.RemittanceProviderRepository
}

func (s *RemittanceProviderSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewRemittanceProviderRepository(s.pool)
}

func (s *RemittanceProviderSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *RemittanceProviderSuite) TestListActive() {
	ctx := context.Background()
	// Table may be empty; ListActive should return without error
	list, err := s.repo.ListActive(ctx)
	s.Require().NoError(err)
	s.Empty(list) // empty table yields empty or nil slice
}

func TestRemittanceProviderSuite(t *testing.T) {
	suite.Run(t, new(RemittanceProviderSuite))
}
