package pots_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/pots"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type PotSuite struct {
	suite.Suite
	pool       *pgxpool.Pool
	svc        *pots.Service
	mockLedger *testutil.MockLedgerClient
	potRepo    repository.PotRepository
}

func (s *PotSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.mockLedger = testutil.NewMockLedgerClient()
	chart := ledger.NewChart("neo")
	s.potRepo = repository.NewPotRepository(s.pool)
	balanceRepo := repository.NewCurrencyBalanceRepository(s.pool)
	userRepo := repository.NewUserRepository(s.pool)

	receiptRepo := repository.NewTransactionReceiptRepository(s.pool)
	s.svc = pots.NewService(s.potRepo, balanceRepo, userRepo, s.mockLedger, chart, receiptRepo)
}

func (s *PotSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *PotSuite) TestCreatePot_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)

	pot, err := s.svc.CreatePot(context.Background(), user.ID, &pots.CreatePotRequest{
		Name:         "Vacation",
		CurrencyCode: "ETB",
	})
	s.Require().NoError(err)
	s.Equal("Vacation", pot.Name)
	s.Equal("ETB", pot.CurrencyCode)
	s.NotEmpty(pot.ID)

	list, err := s.potRepo.ListActiveByUser(context.Background(), user.ID)
	s.Require().NoError(err)
	s.Len(list, 1)
}

func (s *PotSuite) TestCreatePot_CurrencyNotActive() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	// user has no USD balance

	_, err := s.svc.CreatePot(context.Background(), user.ID, &pots.CreatePotRequest{
		Name:         "Vacation",
		CurrencyCode: "USD",
	})
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrCurrencyNotActive)
}

func (s *PotSuite) TestAddToPot_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	potID := testutil.SeedPot(s.T(), s.pool, user.ID, "Savings", "ETB")

	s.mockLedger.Balances[user.LedgerWalletID] = 500000

	_, err := s.svc.AddToPot(context.Background(), user.ID, potID, &pots.PotTransferRequest{
		AmountCents: 100000,
	})
	s.Require().NoError(err)
}

func (s *PotSuite) TestAddToPot_CreatesReceipt() {
	ctx := context.Background()
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	potID := testutil.SeedPot(s.T(), s.pool, user.ID, "Savings", "ETB")
	s.mockLedger.Balances[user.LedgerWalletID] = 500000

	_, err := s.svc.AddToPot(ctx, user.ID, potID, &pots.PotTransferRequest{AmountCents: 50000})
	s.Require().NoError(err)

	receiptRepo := repository.NewTransactionReceiptRepository(s.pool)
	recs, err := receiptRepo.ListByUserID(ctx, user.ID, 10, 0)
	s.Require().NoError(err)
	var depositReceipt *domain.TransactionReceipt
	for i := range recs {
		if recs[i].Type == domain.ReceiptPotDeposit {
			depositReceipt = &recs[i]
			break
		}
	}
	s.Require().NotNil(depositReceipt, "user should have a pot_deposit receipt")
	s.Equal(int64(50000), depositReceipt.AmountCents)
	s.Equal("ETB", depositReceipt.Currency)
	s.Require().NotNil(depositReceipt.Metadata)
}

func (s *PotSuite) TestAddToPot_InsufficientFunds() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	potID := testutil.SeedPot(s.T(), s.pool, user.ID, "Savings", "ETB")

	s.mockLedger.Balances[user.LedgerWalletID] = 0

	_, err := s.svc.AddToPot(context.Background(), user.ID, potID, &pots.PotTransferRequest{
		AmountCents: 100000,
	})
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrInsufficientFunds)
}

func (s *PotSuite) TestWithdrawFromPot_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	potID := testutil.SeedPot(s.T(), s.pool, user.ID, "Savings", "ETB")

	s.mockLedger.Balances["pot:"+potID] = 50000

	_, err := s.svc.WithdrawFromPot(context.Background(), user.ID, potID, &pots.PotTransferRequest{
		AmountCents: 25000,
	})
	s.Require().NoError(err)
}

func (s *PotSuite) TestWithdrawFromPot_CreatesReceipt() {
	ctx := context.Background()
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	potID := testutil.SeedPot(s.T(), s.pool, user.ID, "Savings", "ETB")
	s.mockLedger.Balances["pot:"+potID] = 50000

	_, err := s.svc.WithdrawFromPot(ctx, user.ID, potID, &pots.PotTransferRequest{AmountCents: 20000})
	s.Require().NoError(err)

	receiptRepo := repository.NewTransactionReceiptRepository(s.pool)
	recs, err := receiptRepo.ListByUserID(ctx, user.ID, 10, 0)
	s.Require().NoError(err)
	var withdrawReceipt *domain.TransactionReceipt
	for i := range recs {
		if recs[i].Type == domain.ReceiptPotWithdraw {
			withdrawReceipt = &recs[i]
			break
		}
	}
	s.Require().NotNil(withdrawReceipt, "user should have a pot_withdraw receipt")
	s.Equal(int64(20000), withdrawReceipt.AmountCents)
	s.Equal("ETB", withdrawReceipt.Currency)
	s.Require().NotNil(withdrawReceipt.Metadata)
}

func (s *PotSuite) TestArchivePot_Empty() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	potID := testutil.SeedPot(s.T(), s.pool, user.ID, "Savings", "ETB")

	s.mockLedger.Balances["pot:"+potID] = 0

	returnedCents, currency, err := s.svc.ArchivePot(context.Background(), user.ID, potID)
	s.Require().NoError(err)
	s.Equal(int64(0), returnedCents)
	s.Equal("ETB", currency)

	pot, err := s.potRepo.GetByID(context.Background(), potID)
	s.Require().NoError(err)
	s.True(pot.IsArchived)
}

func (s *PotSuite) TestListPots() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	testutil.SeedPot(s.T(), s.pool, user.ID, "Pot1", "ETB")
	testutil.SeedPot(s.T(), s.pool, user.ID, "Pot2", "ETB")

	list, err := s.svc.ListPots(context.Background(), user.ID)
	s.Require().NoError(err)
	s.Len(list, 2)
}

func TestPotSuite(t *testing.T) {
	suite.Run(t, new(PotSuite))
}
