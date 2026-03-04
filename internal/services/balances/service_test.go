package balances_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/balances"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type BalanceSuite struct {
	suite.Suite
	pool         *pgxpool.Pool
	svc          *balances.Service
	mockLedger   *testutil.MockLedgerClient
	balanceRepo  repository.CurrencyBalanceRepository
	userRepo     repository.UserRepository
}

func (s *BalanceSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.mockLedger = testutil.NewMockLedgerClient()
	s.balanceRepo = repository.NewCurrencyBalanceRepository(s.pool)
	accountRepo := repository.NewAccountDetailsRepository(s.pool)
	s.userRepo = repository.NewUserRepository(s.pool)

	s.svc = balances.NewService(s.balanceRepo, accountRepo, s.userRepo, s.mockLedger, nil)
}

func (s *BalanceSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *BalanceSuite) TestCreateCurrencyBalance_ETB() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)

	result, err := s.svc.CreateCurrencyBalance(context.Background(), user.ID, &balances.CreateBalanceRequest{
		CurrencyCode: "USD",
	})
	s.Require().NoError(err)
	s.Equal("USD", result.CurrencyCode)
	s.False(result.IsPrimary)
	s.NotEmpty(result.ID)

	ctx := context.Background()
	bal, err := s.balanceRepo.GetByUserAndCurrency(ctx, user.ID, "USD")
	s.Require().NoError(err)
	s.NotNil(bal)
}

func (s *BalanceSuite) TestCreateCurrencyBalance_KYCInsufficient() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)

	s.Require().NoError(s.userRepo.UpdateKYCLevel(context.Background(), user.ID, domain.KYCBasic))

	_, err := s.svc.CreateCurrencyBalance(context.Background(), user.ID, &balances.CreateBalanceRequest{
		CurrencyCode: "USD",
	})
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrKYCInsufficientForFX)
}

func (s *BalanceSuite) TestCreateCurrencyBalance_AlreadyActive() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "USD", false)

	_, err := s.svc.CreateCurrencyBalance(context.Background(), user.ID, &balances.CreateBalanceRequest{
		CurrencyCode: "USD",
	})
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrBalanceAlreadyActive)
}

func (s *BalanceSuite) TestDeleteCurrencyBalance_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "USD", false)

	s.mockLedger.Balances[user.LedgerWalletID] = 0

	s.Require().NoError(s.svc.DeleteCurrencyBalance(context.Background(), user.ID, "USD"))

	bal, err := s.balanceRepo.GetSoftDeleted(context.Background(), user.ID, "USD")
	s.Require().NoError(err)
	s.NotNil(bal)
}

func (s *BalanceSuite) TestDeleteCurrencyBalance_Primary() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)

	err := s.svc.DeleteCurrencyBalance(context.Background(), user.ID, "ETB")
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrCannotDeletePrimary)
}

func (s *BalanceSuite) TestDeleteCurrencyBalance_NotEmpty() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "USD", false)

	s.mockLedger.Balances[user.LedgerWalletID] = 100

	err := s.svc.DeleteCurrencyBalance(context.Background(), user.ID, "USD")
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrBalanceNotEmpty)
}

func (s *BalanceSuite) TestListActiveCurrencyBalances() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "USD", false)

	s.mockLedger.Balances[user.LedgerWalletID] = 500000

	list, err := s.svc.ListActiveCurrencyBalances(context.Background(), user.ID)
	s.Require().NoError(err)
	s.Len(list, 2)
}

func TestBalanceSuite(t *testing.T) {
	suite.Run(t, new(BalanceSuite))
}
