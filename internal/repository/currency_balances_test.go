package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type CurrencyBalanceSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.CurrencyBalanceRepository
}

func (s *CurrencyBalanceSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewCurrencyBalanceRepository(s.pool)
}

func (s *CurrencyBalanceSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *CurrencyBalanceSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440001"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	balance := &domain.CurrencyBalance{
		UserID:       userID,
		CurrencyCode: "ETB",
		IsPrimary:    true,
	}
	err := s.repo.Create(ctx, balance)
	s.Require().NoError(err)
	s.NotEmpty(balance.ID)
	s.NotEmpty(balance.CreatedAt)

	got, err := s.repo.GetByUserAndCurrency(ctx, userID, "ETB")
	s.Require().NoError(err)
	s.Equal(balance.ID, got.ID)
	s.Equal(userID, got.UserID)
	s.Equal("ETB", got.CurrencyCode)
	s.True(got.IsPrimary)
}

func (s *CurrencyBalanceSuite) TestListActiveByUser() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440002"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911111111"))

	etb := &domain.CurrencyBalance{UserID: userID, CurrencyCode: "ETB", IsPrimary: true}
	usd := &domain.CurrencyBalance{UserID: userID, CurrencyCode: "USD", IsPrimary: false}
	s.Require().NoError(s.repo.Create(ctx, etb))
	s.Require().NoError(s.repo.Create(ctx, usd))

	list, err := s.repo.ListActiveByUser(ctx, userID)
	s.Require().NoError(err)
	s.Len(list, 2)
	s.Equal("ETB", list[0].CurrencyCode)
	s.True(list[0].IsPrimary)
	s.Equal("USD", list[1].CurrencyCode)
	s.False(list[1].IsPrimary)
}

func (s *CurrencyBalanceSuite) TestSoftDelete_Success() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440003"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251922222222"))

	// Use non-primary balance: chk_primary_not_deleted forbids soft-deleting primary
	balance := &domain.CurrencyBalance{UserID: userID, CurrencyCode: "USD", IsPrimary: false}
	s.Require().NoError(s.repo.Create(ctx, balance))

	err := s.repo.SoftDelete(ctx, userID, "USD")
	s.Require().NoError(err)

	_, err = s.repo.GetByUserAndCurrency(ctx, userID, "USD")
	s.ErrorIs(err, domain.ErrBalanceNotActive)
}

func (s *CurrencyBalanceSuite) TestSoftDelete_NotActive() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440004"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251933333333"))
	// No ETB balance created - SoftDelete on non-existent
	err := s.repo.SoftDelete(ctx, userID, "ETB")
	s.ErrorIs(err, domain.ErrBalanceNotActive)
}

func (s *CurrencyBalanceSuite) TestReactivate_Success() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440005"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251944444444"))

	// Use non-primary balance: chk_primary_not_deleted forbids soft-deleting primary
	balance := &domain.CurrencyBalance{UserID: userID, CurrencyCode: "USD", IsPrimary: false}
	s.Require().NoError(s.repo.Create(ctx, balance))
	s.Require().NoError(s.repo.SoftDelete(ctx, userID, "USD"))

	reactivated, err := s.repo.Reactivate(ctx, userID, "USD")
	s.Require().NoError(err)
	s.Equal(balance.ID, reactivated.ID)
	s.Nil(reactivated.DeletedAt)

	got, err := s.repo.GetByUserAndCurrency(ctx, userID, "USD")
	s.Require().NoError(err)
	s.Equal(balance.ID, got.ID)
}

func (s *CurrencyBalanceSuite) TestGetSoftDeleted() {
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440006"
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251955555555"))

	// Use non-primary balance: chk_primary_not_deleted forbids soft-deleting primary
	balance := &domain.CurrencyBalance{UserID: userID, CurrencyCode: "USD", IsPrimary: false}
	s.Require().NoError(s.repo.Create(ctx, balance))
	s.Require().NoError(s.repo.SoftDelete(ctx, userID, "USD"))

	got, err := s.repo.GetSoftDeleted(ctx, userID, "USD")
	s.Require().NoError(err)
	s.Equal(balance.ID, got.ID)
	s.Require().NotNil(got.DeletedAt)
}

func TestCurrencyBalanceSuite(t *testing.T) {
	suite.Run(t, new(CurrencyBalanceSuite))
}
