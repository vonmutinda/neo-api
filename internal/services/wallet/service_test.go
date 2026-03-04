package wallet_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/convert"
	"github.com/vonmutinda/neo/internal/services/wallet"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type WalletServiceSuite struct {
	suite.Suite
	pool        *pgxpool.Pool
	userRepo    repository.UserRepository
	receiptRepo repository.TransactionReceiptRepository
	balanceRepo repository.CurrencyBalanceRepository
	mockLedger  *testutil.MockLedgerClient
	svc         *wallet.Service
}

func (s *WalletServiceSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.userRepo = repository.NewUserRepository(s.pool)
	s.receiptRepo = repository.NewTransactionReceiptRepository(s.pool)
	s.balanceRepo = repository.NewCurrencyBalanceRepository(s.pool)
	s.mockLedger = testutil.NewMockLedgerClient()

	chart := ledger.NewChart("neo")
	rateProvider := convert.NewStaticRateProvider(1.5)

	s.svc = wallet.NewService(
		s.userRepo, s.receiptRepo, s.balanceRepo,
		s.mockLedger, chart, rateProvider,
	)
}

func (s *WalletServiceSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
	s.mockLedger.Balances = make(map[string]int64)
}

func (s *WalletServiceSuite) seedReceipt(userID, ledgerTxID string, receiptType domain.ReceiptType, amountCents int64, currency string, narration *string) string {
	ik := uuid.NewString()
	var id string
	err := s.pool.QueryRow(context.Background(),
		`INSERT INTO transaction_receipts (user_id, type, status, amount_cents, currency, ledger_transaction_id, idempotency_key, narration)
		 VALUES ($1, $2, 'completed', $3, $4, $5, $6, $7) RETURNING id`,
		userID, receiptType, amountCents, currency, ledgerTxID, ik, narration,
	).Scan(&id)
	s.Require().NoError(err)
	return id
}

func (s *WalletServiceSuite) seedConversionPair(userID string) (outID, inID string) {
	ledgerTxID := "ledger:" + uuid.NewString()
	narration := "Converted 500.00 ETB to 10.00 USD"
	outID = s.seedReceipt(userID, ledgerTxID, domain.ReceiptConvertOut, 50000, "ETB", &narration)
	inID = s.seedReceipt(userID, ledgerTxID, domain.ReceiptConvertIn, 1000, "USD", &narration)
	return
}

// ---------------------------------------------------------------------------
// GetBalance
// ---------------------------------------------------------------------------

func (s *WalletServiceSuite) TestGetBalance() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345678"))
	s.mockLedger.Balances[user.LedgerWalletID+":ETB/2"] = 150000

	view, err := s.svc.GetBalance(context.Background(), user.ID, "ETB")
	s.Require().NoError(err)
	s.Equal(int64(150000), view.BalanceCents)
	s.Equal("ETB", view.Currency)
	s.Equal("Br", view.Symbol)
	s.Equal(user.LedgerWalletID, view.WalletID)
}

func (s *WalletServiceSuite) TestGetBalance_DefaultCurrency() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345679"))
	s.mockLedger.Balances[user.LedgerWalletID] = 200000

	view, err := s.svc.GetBalance(context.Background(), user.ID, "")
	s.Require().NoError(err)
	s.Equal("ETB", view.Currency)
}

func (s *WalletServiceSuite) TestGetBalance_InvalidCurrency() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345680"))
	_ = user

	_, err := s.svc.GetBalance(context.Background(), user.ID, "INVALID")
	s.Require().Error(err)
}

// ---------------------------------------------------------------------------
// GetSummary
// ---------------------------------------------------------------------------

func (s *WalletServiceSuite) TestGetSummary() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345681"))
	s.mockLedger.Balances[user.LedgerWalletID] = 100000

	summary, err := s.svc.GetSummary(context.Background(), user.ID)
	s.Require().NoError(err)
	s.Equal(user.LedgerWalletID, summary.WalletID)
	s.Equal("ETB", summary.PrimaryCurrency)
	s.NotEmpty(summary.Balances)
}

// ---------------------------------------------------------------------------
// ListTransactions
// ---------------------------------------------------------------------------

func (s *WalletServiceSuite) TestListTransactions_Basic() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345682"))
	for i := 0; i < 3; i++ {
		s.seedReceipt(user.ID, "ledger:"+uuid.NewString(), domain.ReceiptP2PSend, 10000, "ETB", nil)
	}

	views, err := s.svc.ListTransactions(context.Background(), user.ID, nil, 10, 0)
	s.Require().NoError(err)
	s.Len(views, 3)
}

func (s *WalletServiceSuite) TestListTransactions_DeduplicatesConversions() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345683"))
	s.seedReceipt(user.ID, "ledger:"+uuid.NewString(), domain.ReceiptP2PSend, 10000, "ETB", nil)
	s.seedConversionPair(user.ID)

	views, err := s.svc.ListTransactions(context.Background(), user.ID, nil, 10, 0)
	s.Require().NoError(err)

	s.Len(views, 2, "should have p2p_send + convert_out, not convert_in")

	var types []domain.ReceiptType
	for _, v := range views {
		types = append(types, v.Type)
	}
	s.Contains(types, domain.ReceiptP2PSend)
	s.Contains(types, domain.ReceiptConvertOut)
	s.NotContains(types, domain.ReceiptConvertIn)

	for _, v := range views {
		if v.Type == domain.ReceiptConvertOut {
			s.NotNil(v.ConvertedCurrency)
			s.Equal("USD", *v.ConvertedCurrency)
			s.NotNil(v.ConvertedAmountCents)
			s.Equal(int64(1000), *v.ConvertedAmountCents)
		}
	}
}

func (s *WalletServiceSuite) TestListTransactions_ConvertInAloneKept() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345684"))
	narration := "Converted 500.00 ETB to 10.00 USD"
	s.seedReceipt(user.ID, "ledger:"+uuid.NewString(), domain.ReceiptConvertIn, 1000, "USD", &narration)

	usd := "USD"
	views, err := s.svc.ListTransactions(context.Background(), user.ID, &usd, 10, 0)
	s.Require().NoError(err)
	s.Len(views, 1, "convert_in should be kept when no paired convert_out in result")
	s.Equal(domain.ReceiptConvertIn, views[0].Type)
}

func (s *WalletServiceSuite) TestListTransactions_CurrencyFilter() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345685"))
	s.seedReceipt(user.ID, "ledger:"+uuid.NewString(), domain.ReceiptP2PSend, 10000, "ETB", nil)
	s.seedReceipt(user.ID, "ledger:"+uuid.NewString(), domain.ReceiptP2PReceive, 500, "USD", nil)

	usd := "USD"
	views, err := s.svc.ListTransactions(context.Background(), user.ID, &usd, 10, 0)
	s.Require().NoError(err)
	s.Len(views, 1)
	s.Equal("USD", views[0].Currency)
}

func TestWalletServiceSuite(t *testing.T) {
	suite.Run(t, new(WalletServiceSuite))
}
