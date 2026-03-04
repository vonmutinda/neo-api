package personal_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/convert"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/vonmutinda/neo/internal/transport/http/handlers/personal"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
)

const convertTestUserID = "550e8400-e29b-41d4-a716-446655440003"

type ConvertSuite struct {
	suite.Suite
	pool       *pgxpool.Pool
	server    *httptest.Server
	mockLedger *testutil.MockLedgerClient
}

func (s *ConvertSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.mockLedger = testutil.NewMockLedgerClient()

	chart := ledger.NewChart("neo")
	userRepo := repository.NewUserRepository(s.pool)
	receiptRepo := repository.NewTransactionReceiptRepository(s.pool)
	rateProvider := convert.NewStaticRateProvider(1.5)

	convertSvc := convert.NewService(userRepo, s.mockLedger, chart, rateProvider, nil, receiptRepo, nil)
	handler := personal.NewConvertHandler(convertSvc)

	r := chi.NewRouter()
	r.Use(middleware.Auth(testutil.TestJWTConfig()))
	r.Post("/v1/convert", handler.Convert)
	r.Get("/v1/convert/rate", handler.GetRate)

	s.server = httptest.NewServer(r)
}

func (s *ConvertSuite) TearDownSuite() {
	if s.server != nil {
		s.server.Close()
	}
}

func (s *ConvertSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

// TestConvert_Success seeds user, set mock balance to 5000000 (50000.00 ETB), POST -> 200
func (s *ConvertSuite) TestConvert_Success() {
	user := testutil.SeedUser(s.T(), s.pool, convertTestUserID, phone.MustParse("+251912345680"))

	s.mockLedger.Balances["wallet:"+user.ID] = 5000000 // 50000.00 ETB

	body := map[string]any{
		"fromCurrency": "ETB",
		"toCurrency":  "USD",
		"amountCents": 100000,
	}
	req := testutil.NewAuthRequest(s.T(), http.MethodPost, s.server.URL+"/v1/convert", body, testutil.MustCreateToken(s.T(), convertTestUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var data struct {
		FromCurrency    string  `json:"fromCurrency"`
		ToCurrency      string  `json:"toCurrency"`
		FromAmountCents int64   `json:"fromAmountCents"`
		ToAmountCents   int64   `json:"toAmountCents"`
		TransactionID   string  `json:"transactionId"`
	}
	testutil.MustDecodeJSON(s.T(), resp, &data)
	s.Equal("ETB", data.FromCurrency)
	s.Equal("USD", data.ToCurrency)
	s.Equal(int64(100000), data.FromAmountCents)
	s.NotEmpty(data.TransactionID)
}

// TestConvert_SameCurrency POST ETB->ETB -> 400
func (s *ConvertSuite) TestConvert_SameCurrency() {
	testutil.SeedUser(s.T(), s.pool, convertTestUserID, phone.MustParse("+251912345680"))

	body := map[string]any{
		"fromCurrency": "ETB",
		"toCurrency":  "ETB",
		"amountCents": 100000,
	}
	req := testutil.NewAuthRequest(s.T(), http.MethodPost, s.server.URL+"/v1/convert", body, testutil.MustCreateToken(s.T(), convertTestUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestConvert_InsufficientFunds set mock balance to 0 -> 422
func (s *ConvertSuite) TestConvert_InsufficientFunds() {
	user := testutil.SeedUser(s.T(), s.pool, convertTestUserID, phone.MustParse("+251912345680"))

	s.mockLedger.Balances["wallet:"+user.ID] = 0

	body := map[string]any{
		"fromCurrency": "ETB",
		"toCurrency":  "USD",
		"amountCents": 100000,
	}
	req := testutil.NewAuthRequest(s.T(), http.MethodPost, s.server.URL+"/v1/convert", body, testutil.MustCreateToken(s.T(), convertTestUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusUnprocessableEntity, resp.StatusCode)
}

// TestGetRate_Success GET /v1/convert/rate?from=ETB&to=USD -> 200
func (s *ConvertSuite) TestGetRate_Success() {
	req := testutil.NewAuthRequest(s.T(), http.MethodGet, s.server.URL+"/v1/convert/rate?from=ETB&to=USD", nil, testutil.MustCreateToken(s.T(), convertTestUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var data struct {
		From string  `json:"from"`
		To   string  `json:"to"`
		Mid  float64 `json:"mid"`
	}
	testutil.MustDecodeJSON(s.T(), resp, &data)
	s.Equal("ETB", data.From)
	s.Equal("USD", data.To)
	s.Greater(data.Mid, 0.0)
}

func TestConvertSuite(t *testing.T) {
	suite.Run(t, new(ConvertSuite))
}
