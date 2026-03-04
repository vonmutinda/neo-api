package personal_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/pots"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/vonmutinda/neo/internal/transport/http/handlers/personal"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
)

const potTestUserID = "550e8400-e29b-41d4-a716-446655440002"

type PotSuite struct {
	suite.Suite
	pool       *pgxpool.Pool
	server     *httptest.Server
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

	potsSvc := pots.NewService(s.potRepo, balanceRepo, userRepo, s.mockLedger, chart, receiptRepo)
	handler := personal.NewPotHandler(potsSvc)

	r := chi.NewRouter()
	r.Use(middleware.Auth(testutil.TestJWTConfig()))
	r.Post("/v1/pots", handler.Create)
	r.Get("/v1/pots", handler.List)
	r.Get("/v1/pots/{id}", handler.Get)
	r.Patch("/v1/pots/{id}", handler.Update)
	r.Delete("/v1/pots/{id}", handler.Delete)
	r.Post("/v1/pots/{id}/add", handler.Add)
	r.Post("/v1/pots/{id}/withdraw", handler.Withdraw)

	s.server = httptest.NewServer(r)
}

func (s *PotSuite) TearDownSuite() {
	if s.server != nil {
		s.server.Close()
	}
}

func (s *PotSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

// TestCreate_Success seeds user + ETB balance, POST {"name":"Emergency","currencyCode":"ETB"} -> 201, pot in DB
func (s *PotSuite) TestCreate_Success() {
	user := testutil.SeedUser(s.T(), s.pool, potTestUserID, phone.MustParse("+251912345679"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)

	body := map[string]any{
		"name":         "Emergency",
		"currencyCode": "ETB",
	}
	req := testutil.NewAuthRequest(s.T(), http.MethodPost, s.server.URL+"/v1/pots", body, testutil.MustCreateToken(s.T(), potTestUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	var data struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		CurrencyCode string `json:"currencyCode"`
	}
	testutil.MustDecodeJSON(s.T(), resp, &data)
	s.NotEmpty(data.ID)
	s.Equal("Emergency", data.Name)
	s.Equal("ETB", data.CurrencyCode)

	list, err := s.potRepo.ListActiveByUser(context.Background(), user.ID)
	s.Require().NoError(err)
	s.Len(list, 1)
}

// TestCreate_CurrencyNotActive seeds user with only ETB, POST with USD -> 422
func (s *PotSuite) TestCreate_CurrencyNotActive() {
	user := testutil.SeedUser(s.T(), s.pool, potTestUserID, phone.MustParse("+251912345679"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	// user has no USD balance

	body := map[string]any{
		"name":         "Vacation",
		"currencyCode": "USD",
	}
	req := testutil.NewAuthRequest(s.T(), http.MethodPost, s.server.URL+"/v1/pots", body, testutil.MustCreateToken(s.T(), potTestUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusUnprocessableEntity, resp.StatusCode)
}

// TestList_ReturnsPots seeds user + 2 pots, GET -> 200 with array
func (s *PotSuite) TestList_ReturnsPots() {
	user := testutil.SeedUser(s.T(), s.pool, potTestUserID, phone.MustParse("+251912345679"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	testutil.SeedPot(s.T(), s.pool, user.ID, "Pot1", "ETB")
	testutil.SeedPot(s.T(), s.pool, user.ID, "Pot2", "ETB")

	req := testutil.NewAuthRequest(s.T(), http.MethodGet, s.server.URL+"/v1/pots", nil, testutil.MustCreateToken(s.T(), potTestUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var data []map[string]any
	testutil.MustDecodeJSON(s.T(), resp, &data)
	s.Len(data, 2)
}

// TestGet_Success seeds pot, GET /v1/pots/{id} -> 200
func (s *PotSuite) TestGet_Success() {
	user := testutil.SeedUser(s.T(), s.pool, potTestUserID, phone.MustParse("+251912345679"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	potID := testutil.SeedPot(s.T(), s.pool, user.ID, "Savings", "ETB")

	req := testutil.NewAuthRequest(s.T(), http.MethodGet, s.server.URL+"/v1/pots/"+potID, nil, testutil.MustCreateToken(s.T(), potTestUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var data struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	testutil.MustDecodeJSON(s.T(), resp, &data)
	s.Equal(potID, data.ID)
	s.Equal("Savings", data.Name)
}

// TestAdd_Success seeds pot, set mock balance to 500000, POST /v1/pots/{id}/add {"amountCents":10000} -> 200
func (s *PotSuite) TestAdd_Success() {
	user := testutil.SeedUser(s.T(), s.pool, potTestUserID, phone.MustParse("+251912345679"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	potID := testutil.SeedPot(s.T(), s.pool, user.ID, "Savings", "ETB")

	s.mockLedger.Balances["wallet:"+user.ID] = 500000

	body := map[string]any{"amountCents": 10000}
	req := testutil.NewAuthRequest(s.T(), http.MethodPost, s.server.URL+"/v1/pots/"+potID+"/add", body, testutil.MustCreateToken(s.T(), potTestUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)
}

// TestWithdraw_InsufficientFunds seeds pot, set mock pot balance to 0, POST withdraw -> 422
func (s *PotSuite) TestWithdraw_InsufficientFunds() {
	user := testutil.SeedUser(s.T(), s.pool, potTestUserID, phone.MustParse("+251912345679"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	potID := testutil.SeedPot(s.T(), s.pool, user.ID, "Savings", "ETB")

	s.mockLedger.Balances["pot:"+potID] = 0

	body := map[string]any{"amountCents": 10000}
	req := testutil.NewAuthRequest(s.T(), http.MethodPost, s.server.URL+"/v1/pots/"+potID+"/withdraw", body, testutil.MustCreateToken(s.T(), potTestUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusUnprocessableEntity, resp.StatusCode)
}

// TestDelete_Success seeds pot, set mock pot balance to 0, DELETE /v1/pots/{id} -> 204
func (s *PotSuite) TestDelete_Success() {
	user := testutil.SeedUser(s.T(), s.pool, potTestUserID, phone.MustParse("+251912345679"))
	testutil.SeedCurrencyBalance(s.T(), s.pool, user.ID, "ETB", true)
	potID := testutil.SeedPot(s.T(), s.pool, user.ID, "Savings", "ETB")

	s.mockLedger.Balances["pot:"+potID] = 0

	req := testutil.NewAuthRequest(s.T(), http.MethodDelete, s.server.URL+"/v1/pots/"+potID, nil, testutil.MustCreateToken(s.T(), potTestUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)

	pot, err := s.potRepo.GetByID(context.Background(), potID)
	s.Require().NoError(err)
	s.True(pot.IsArchived)
}

func TestPotSuite(t *testing.T) {
	suite.Run(t, new(PotSuite))
}
