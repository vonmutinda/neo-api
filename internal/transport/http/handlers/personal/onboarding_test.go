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
	"github.com/vonmutinda/neo/internal/services/onboarding"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
	persh "github.com/vonmutinda/neo/internal/transport/http/handlers/personal"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
)

const (
	userID1 = "550e8400-e29b-41d4-a716-446655440001"
)

type OnboardingSuite struct {
	suite.Suite
	pool        *pgxpool.Pool
	server      *httptest.Server
	userRepo    repository.UserRepository
	balanceRepo repository.CurrencyBalanceRepository
}

func (s *OnboardingSuite) SetupSuite() {
	t := s.T()
	s.pool = testutil.MustStartPostgres(t)

	s.userRepo = repository.NewUserRepository(s.pool)
	kycRepo := repository.NewKYCRepository(s.pool)
	auditRepo := repository.NewAuditRepository(s.pool)
	s.balanceRepo = repository.NewCurrencyBalanceRepository(s.pool)
	accountRepo := repository.NewAccountDetailsRepository(s.pool)

	mockLedger := testutil.NewMockLedgerClient()
	mockFayda := testutil.NewMockFaydaClient()

	balancesSvc := balances.NewService(s.balanceRepo, accountRepo, s.userRepo, mockLedger, nil)
	onboardingSvc := onboarding.NewService(s.userRepo, kycRepo, auditRepo, mockFayda, mockLedger, balancesSvc)

	handler := persh.NewHandlers(
		nil,
		onboardingSvc, nil, nil, nil, nil,
		s.userRepo, nil, s.balanceRepo,
		balancesSvc, nil, nil, nil, nil,
		nil,
		nil,
		nil, nil,
	)

	r := chi.NewRouter()
	r.Post("/v1/register", handler.Onboarding.Register)
	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.Auth(testutil.TestJWTConfig()))
		r.Post("/kyc/otp", handler.Onboarding.RequestOTP)
		r.Post("/kyc/verify", handler.Onboarding.VerifyOTP)
	})

	s.server = httptest.NewServer(r)
	t.Cleanup(s.server.Close)
}

func (s *OnboardingSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *OnboardingSuite) TestRegister_Success() {
	body := map[string]string{"phoneNumber": "+251912345678"}
	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/register", body, "")
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusCreated, resp.StatusCode)

	ctx := context.Background()
	user, err := s.userRepo.GetByPhone(ctx, phone.MustParse("+251912345678"))
	s.Require().NoError(err)
	s.NotNil(user)

	balances, err := s.balanceRepo.ListActiveByUser(ctx, user.ID)
	s.Require().NoError(err)
	var etbPrimary bool
	for _, b := range balances {
		if b.CurrencyCode == "ETB" && b.IsPrimary {
			etbPrimary = true
			break
		}
	}
	s.True(etbPrimary, "expected ETB primary balance")
}

func (s *OnboardingSuite) TestRegister_MissingPhone() {
	body := map[string]string{}
	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/register", body, "")
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *OnboardingSuite) TestRequestOTP_Success() {
	_ = testutil.SeedUser(s.T(), s.pool, userID1, phone.MustParse("+251912345678"))

	body := map[string]string{"faydaFin": "123456789012"}
	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/kyc/otp", body, testutil.MustCreateToken(s.T(), userID1))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)
}

func (s *OnboardingSuite) TestVerifyOTP_Success() {
	_ = testutil.SeedUser(s.T(), s.pool, userID1, phone.MustParse("+251912345678"))

	body := map[string]interface{}{
		"faydaFin":      "123456789012",
		"otp":           "123456",
		"transactionId": "tx-1",
	}
	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/kyc/verify", body, testutil.MustCreateToken(s.T(), userID1))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	user, err := s.userRepo.GetByID(context.Background(), userID1)
	s.Require().NoError(err)
	s.Equal(domain.KYCLevel(2), user.KYCLevel)
}

func TestOnboardingSuite(t *testing.T) {
	suite.Run(t, new(OnboardingSuite))
}
