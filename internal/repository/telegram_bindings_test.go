package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type TelegramBindingSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.TelegramLinkTokenRepository
}

func (s *TelegramBindingSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewTelegramLinkTokenRepository(s.pool)
}

func (s *TelegramBindingSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *TelegramBindingSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	token, err := s.repo.Create(ctx, userID, 15*time.Minute)
	s.Require().NoError(err)
	s.NotEmpty(token)
}

func (s *TelegramBindingSuite) TestConsume_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911111111"))

	token, err := s.repo.Create(ctx, userID, 15*time.Minute)
	s.Require().NoError(err)

	consumedUserID, err := s.repo.Consume(ctx, token)
	s.Require().NoError(err)
	s.Equal(userID, consumedUserID)
}

func (s *TelegramBindingSuite) TestConsume_AlreadyConsumed() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251922222222"))

	token, err := s.repo.Create(ctx, userID, 15*time.Minute)
	s.Require().NoError(err)

	_, err = s.repo.Consume(ctx, token)
	s.Require().NoError(err)

	_, err = s.repo.Consume(ctx, token)
	s.ErrorIs(err, domain.ErrNotFound)
}

func TestTelegramBindingSuite(t *testing.T) {
	suite.Run(t, new(TelegramBindingSuite))
}
