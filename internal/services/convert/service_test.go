package convert_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/convert"
	"github.com/vonmutinda/neo/internal/services/overdraft"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/money"
	"github.com/vonmutinda/neo/pkg/phone"
)

type ConvertSuite struct {
	suite.Suite
	pool             *pgxpool.Pool
	svc              *convert.Service
	userRepo         repository.UserRepository
	receiptRepo      repository.TransactionReceiptRepository
	mockLedger       *testutil.MockLedgerClient
	mockRateProvider *testutil.MockRateProvider
}

func (s *ConvertSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.userRepo = repository.NewUserRepository(s.pool)
	s.receiptRepo = repository.NewTransactionReceiptRepository(s.pool)
	s.mockLedger = testutil.NewMockLedgerClient()
	s.mockRateProvider = testutil.NewMockRateProvider()
	chart := ledger.NewChart("neo")

	s.svc = convert.NewService(s.userRepo, s.mockLedger, chart, s.mockRateProvider, nil, s.receiptRepo, nil)
}

func (s *ConvertSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
	s.mockLedger.Balances = make(map[string]int64)
	s.mockRateProvider.Rates = make(map[string]convert.Rate)
}

func (s *ConvertSuite) TestConvert_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	user := testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, userID, "ETB", true)
	testutil.SeedCurrencyBalance(s.T(), s.pool, userID, "USD", false)

	s.mockRateProvider.Rates["ETB_USD"] = convert.Rate{
		From:      "ETB",
		To:        "USD",
		Mid:       0.019,
		Bid:       0.018,
		Ask:       0.02,
		Spread:    5.26,
		Timestamp: time.Now(),
		Source:    "static",
	}

	fromAsset := money.FormatAsset("ETB")
	s.mockLedger.Balances[user.LedgerWalletID+":"+fromAsset] = 1_000_000
	s.mockLedger.Balances[user.LedgerWalletID] = 1_000_000

	resp, err := s.svc.Convert(ctx, userID, &domain.ConvertRequest{
		FromCurrency: "ETB",
		ToCurrency:   "USD",
		AmountCents:  100_000,
	})
	s.Require().NoError(err)
	s.Equal("ETB", resp.FromCurrency)
	s.Equal("USD", resp.ToCurrency)
	s.Equal(int64(100_000), resp.FromAmountCents)
	s.Greater(resp.ToAmountCents, int64(0))
	s.NotEmpty(resp.TransactionID)
}

func (s *ConvertSuite) TestConvert_InsufficientFunds() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345679"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, userID, "ETB", true)
	testutil.SeedCurrencyBalance(s.T(), s.pool, userID, "USD", false)

	s.mockRateProvider.Rates["ETB_USD"] = convert.Rate{
		From:      "ETB",
		To:        "USD",
		Mid:       0.019,
		Bid:       0.018,
		Ask:       0.02,
		Spread:    5.26,
		Timestamp: time.Now(),
		Source:    "static",
	}

	walletID := "wallet:" + userID
	fromAsset := money.FormatAsset("ETB")
	s.mockLedger.Balances[walletID+":"+fromAsset] = 0
	s.mockLedger.Balances[walletID] = 0

	_, err := s.svc.Convert(ctx, userID, &domain.ConvertRequest{
		FromCurrency: "ETB",
		ToCurrency:   "USD",
		AmountCents:  100_000,
	})
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrInsufficientFunds)
}

func (s *ConvertSuite) TestGetRate_Success() {
	s.mockRateProvider.Rates["ETB_USD"] = convert.Rate{
		From:      "ETB",
		To:        "USD",
		Mid:       0.019,
		Bid:       0.018,
		Ask:       0.02,
		Spread:    5.26,
		Timestamp: time.Now(),
		Source:    "static",
	}

	rate, err := s.svc.GetRate(context.Background(), "ETB", "USD")
	s.Require().NoError(err)
	s.Equal("ETB", rate.From)
	s.Equal("USD", rate.To)
	s.InDelta(0.019, rate.Mid, 0.001)
	s.InDelta(0.018, rate.Bid, 0.001)
	s.InDelta(0.02, rate.Ask, 0.001)
}

func (s *ConvertSuite) TestGetRate_InvalidCurrency() {
	_, err := s.svc.GetRate(context.Background(), "ETB", "INVALID")
	s.Require().Error(err)
}

// convertCreditingLedger credits walletID with toCents on ConvertCurrency so DebitToOverdraft can succeed.
type convertCreditingLedger struct {
	*testutil.MockLedgerClient
}

func (c *convertCreditingLedger) ConvertCurrency(ctx context.Context, ik, walletID string, fromCents int64, fromAsset string, toCents int64, toAsset, fxAccount string) (string, error) {
	txID, err := c.MockLedgerClient.ConvertCurrency(ctx, ik, walletID, fromCents, fromAsset, toCents, toAsset, fxAccount)
	if err != nil {
		return txID, err
	}
	c.MockLedgerClient.Balances[walletID] += toCents
	return txID, nil
}

// TestConvert_ToETB_AutoRepayOnInflow uses real overdraft repo and service; user has overdraft used, convert to ETB, assert overdraft reduced.
func (s *ConvertSuite) TestConvert_ToETB_AutoRepayOnInflow() {
	ctx := context.Background()
	userID := uuid.NewString()
	user := testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345670"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, userID, "ETB", true)
	testutil.SeedCurrencyBalance(s.T(), s.pool, userID, "USD", false)

	testutil.SeedCreditProfile(s.T(), s.pool, user.ID, 700, 1000000)
	overdraftRepo := repository.NewOverdraftRepository(s.pool)
	loanRepo := repository.NewLoanRepository(s.pool)
	auditRepo := repository.NewAuditRepository(s.pool)
	ledgerWithCredit := &convertCreditingLedger{MockLedgerClient: s.mockLedger}
	overdraftSvc := overdraft.NewService(overdraftRepo, loanRepo, s.userRepo, ledgerWithCredit, auditRepo)
	_, err := overdraftSvc.OptIn(ctx, user.ID)
	s.Require().NoError(err)
	_, err = s.pool.Exec(ctx, `UPDATE overdrafts SET used_cents = 2000, status = 'used' WHERE user_id = $1`, user.ID)
	s.Require().NoError(err)

	s.mockRateProvider.Rates["USD_ETB"] = convert.Rate{
		From:      "USD",
		To:        "ETB",
		Mid:       52.0,
		Bid:       51.0,
		Ask:       53.0,
		Spread:    0,
		Timestamp: time.Now(),
		Source:    "static",
	}
	fromAsset := money.FormatAsset("USD")
	s.mockLedger.Balances[user.LedgerWalletID] = 100_000
	s.mockLedger.Balances[user.LedgerWalletID+":"+fromAsset] = 100_000

	svc := convert.NewService(s.userRepo, ledgerWithCredit, ledger.NewChart("neo"), s.mockRateProvider, nil, s.receiptRepo, overdraftSvc)

	resp, err := svc.Convert(ctx, userID, &domain.ConvertRequest{
		FromCurrency: "USD",
		ToCurrency:   "ETB",
		AmountCents:  10_000,
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal("ETB", resp.ToCurrency)
	s.Greater(resp.ToAmountCents, int64(0))

	o, err := overdraftRepo.GetByUser(ctx, user.ID)
	s.Require().NoError(err)
	s.Require().NotNil(o)
	s.Equal(int64(0), o.UsedCents, "overdraft used_cents should be fully repaid after convert to ETB")
	s.Equal(domain.OverdraftActive, o.Status)

	// Convert-in receipt should have overdraft metadata
	recs, err := s.receiptRepo.ListByUserID(ctx, userID, 10, 0)
	s.Require().NoError(err)
	var convertIn *domain.TransactionReceipt
	for i := range recs {
		if recs[i].Type == domain.ReceiptConvertIn {
			convertIn = &recs[i]
			break
		}
	}
	s.Require().NotNil(convertIn, "user should have a convert_in receipt")
	s.Require().NotNil(convertIn.Metadata, "convert_in receipt should have overdraft metadata")
	var meta domain.InflowOverdraftMetadata
	err = json.Unmarshal(*convertIn.Metadata, &meta)
	s.Require().NoError(err)
	s.Equal(resp.ToAmountCents, meta.TotalInflowCents)
	s.Equal(int64(2000), meta.OverdraftRepaymentCents)
	s.Equal(resp.ToAmountCents-2000, meta.NetInflowCents)
}

func TestConvertSuite(t *testing.T) {
	suite.Run(t, new(ConvertSuite))
}
