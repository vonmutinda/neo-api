package personal_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/balances"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
	persh "github.com/vonmutinda/neo/internal/transport/http/handlers/personal"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
)

const (
	balanceUserID = "550e8400-e29b-41d4-a716-446655440001"
)

type BalanceSuite struct {
	suite.Suite
	pool         *pgxpool.Pool
	server       *httptest.Server
	mockLedger   *testutil.MockLedgerClient
	balanceRepo  repository.CurrencyBalanceRepository
	userRepo     repository.UserRepository
}

func (s *BalanceSuite) SetupSuite() {
	t := s.T()
	s.pool = testutil.MustStartPostgres(t)

	s.userRepo = repository.NewUserRepository(s.pool)
	s.balanceRepo = repository.NewCurrencyBalanceRepository(s.pool)
	accountRepo := repository.NewAccountDetailsRepository(s.pool)
	s.mockLedger = testutil.NewMockLedgerClient()

	balancesSvc := balances.NewService(s.balanceRepo, accountRepo, s.userRepo, s.mockLedger, nil)

	handler := persh.NewHandlers(
		nil,
		nil, nil, nil, nil, nil,
		s.userRepo, nil, s.balanceRepo,
		balancesSvc, nil, nil, nil, nil,
		nil,
		nil,
		nil, nil,
	)

	r := chi.NewRouter()
	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.Auth(testutil.TestJWTConfig()))
		r.Post("/balances", handler.Balances.Create)
		r.Get("/balances", handler.Balances.List)
		r.Delete("/balances/{code}", handler.Balances.Delete)
	})

	s.server = httptest.NewServer(r)
	t.Cleanup(s.server.Close)
}

func (s *BalanceSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *BalanceSuite) TestCreate_Success() {
	_ = testutil.SeedUser(s.T(), s.pool, balanceUserID, phone.MustParse("+251912345678"))
	_ = testutil.SeedCurrencyBalance(s.T(), s.pool, balanceUserID, "ETB", true)

	body := map[string]string{"currencyCode": "USD"}
	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/balances", body, testutil.MustCreateToken(s.T(), balanceUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusCreated, resp.StatusCode)

	bal, err := s.balanceRepo.GetByUserAndCurrency(context.Background(), balanceUserID, "USD")
	s.Require().NoError(err)
	s.NotNil(bal)
}

func (s *BalanceSuite) TestCreate_KYCInsufficient() {
	_ = testutil.SeedUser(s.T(), s.pool, balanceUserID, phone.MustParse("+251912345678"))
	_ = testutil.SeedCurrencyBalance(s.T(), s.pool, balanceUserID, "ETB", true)
	s.Require().NoError(s.userRepo.UpdateKYCLevel(context.Background(), balanceUserID, domain.KYCBasic))

	body := map[string]string{"currencyCode": "USD"}
	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/balances", body, testutil.MustCreateToken(s.T(), balanceUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusUnprocessableEntity, resp.StatusCode)
}

func (s *BalanceSuite) TestList_ReturnsAll() {
	_ = testutil.SeedUser(s.T(), s.pool, balanceUserID, phone.MustParse("+251912345678"))
	_ = testutil.SeedCurrencyBalance(s.T(), s.pool, balanceUserID, "ETB", true)
	_ = testutil.SeedCurrencyBalance(s.T(), s.pool, balanceUserID, "USD", false)

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/balances", nil, testutil.MustCreateToken(s.T(), balanceUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var list []map[string]interface{}
	testutil.MustDecodeJSON(s.T(), resp, &list)
	s.Require().Len(list, 2)
}

func (s *BalanceSuite) TestDelete_Success() {
	user := testutil.SeedUser(s.T(), s.pool, balanceUserID, phone.MustParse("+251912345678"))
	_ = testutil.SeedCurrencyBalance(s.T(), s.pool, balanceUserID, "ETB", true)
	_ = testutil.SeedCurrencyBalance(s.T(), s.pool, balanceUserID, "USD", false)

	s.mockLedger.Balances[user.LedgerWalletID] = 0

	req := testutil.NewAuthRequest(s.T(), "DELETE", s.server.URL+"/v1/balances/USD", nil, testutil.MustCreateToken(s.T(), balanceUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusNoContent, resp.StatusCode)

	bal, err := s.balanceRepo.GetSoftDeleted(context.Background(), balanceUserID, "USD")
	s.Require().NoError(err)
	s.NotNil(bal)
}

func TestBalanceSuite(t *testing.T) {
	suite.Run(t, new(BalanceSuite))
}
