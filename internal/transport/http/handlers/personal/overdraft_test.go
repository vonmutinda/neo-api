package personal_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/overdraft"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/vonmutinda/neo/internal/transport/http/handlers/personal"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
)

const overdraftTestUserID = "550e8400-e29b-41d4-a716-446655440002"

type OverdraftSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	server   *httptest.Server
	overdraftRepo repository.OverdraftRepository
}

func (s *OverdraftSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	overdraftRepo := repository.NewOverdraftRepository(s.pool)
	loanRepo := repository.NewLoanRepository(s.pool)
	userRepo := repository.NewUserRepository(s.pool)
	auditRepo := repository.NewAuditRepository(s.pool)
	s.overdraftRepo = overdraftRepo

	svc := overdraft.NewService(overdraftRepo, loanRepo, userRepo, testutil.NewMockLedgerClient(), auditRepo)
	handler := personal.NewOverdraftHandler(svc)

	r := chi.NewRouter()
	r.Use(middleware.Auth(testutil.TestJWTConfig()))
	r.Get("/v1/overdraft", handler.Get)
	r.Post("/v1/overdraft/opt-in", handler.OptIn)
	r.Post("/v1/overdraft/opt-out", handler.OptOut)
	r.Post("/v1/overdraft/repay", handler.Repay)

	s.server = httptest.NewServer(r)
}

func (s *OverdraftSuite) TearDownSuite() {
	if s.server != nil {
		s.server.Close()
	}
}

func (s *OverdraftSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *OverdraftSuite) TestGet_Inactive() {
	testutil.SeedUser(s.T(), s.pool, overdraftTestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCreditProfile(s.T(), s.pool, overdraftTestUserID, 700, 1000000)

	req := testutil.NewAuthRequest(s.T(), http.MethodGet, s.server.URL+"/v1/overdraft", nil, testutil.MustCreateToken(s.T(), overdraftTestUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var data struct {
		Status       string `json:"status"`
		LimitCents   int64  `json:"limitCents"`
		FeeSummary   string `json:"feeSummary"`
	}
	testutil.MustDecodeJSON(s.T(), resp, &data)
	s.Equal("inactive", data.Status)
	s.Equal(int64(100000), data.LimitCents)
	s.NotEmpty(data.FeeSummary)
}

func (s *OverdraftSuite) TestOptIn_Success() {
	testutil.SeedUser(s.T(), s.pool, overdraftTestUserID, phone.MustParse("+251912345679"))
	testutil.SeedCreditProfile(s.T(), s.pool, overdraftTestUserID, 700, 1000000)

	req := testutil.NewAuthRequest(s.T(), http.MethodPost, s.server.URL+"/v1/overdraft/opt-in", nil, testutil.MustCreateToken(s.T(), overdraftTestUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var data struct {
		ID         string `json:"id"`
		Status     string `json:"status"`
		LimitCents int64  `json:"limitCents"`
	}
	testutil.MustDecodeJSON(s.T(), resp, &data)
	s.NotEmpty(data.ID)
	s.Equal("active", data.Status)
	s.Equal(int64(100000), data.LimitCents)

	o, err := s.overdraftRepo.GetByUser(context.Background(), overdraftTestUserID)
	s.Require().NoError(err)
	s.Require().NotNil(o)
	s.Equal("active", string(o.Status))
}

func (s *OverdraftSuite) TestOptOut_Success() {
	testutil.SeedUser(s.T(), s.pool, overdraftTestUserID, phone.MustParse("+251912345680"))
	testutil.SeedCreditProfile(s.T(), s.pool, overdraftTestUserID, 700, 1000000)

	optInReq := testutil.NewAuthRequest(s.T(), http.MethodPost, s.server.URL+"/v1/overdraft/opt-in", nil, testutil.MustCreateToken(s.T(), overdraftTestUserID))
	optInResp := testutil.DoRequest(s.T(), optInReq)
	optInResp.Body.Close()
	s.Require().Equal(http.StatusOK, optInResp.StatusCode)

	req := testutil.NewAuthRequest(s.T(), http.MethodPost, s.server.URL+"/v1/overdraft/opt-out", nil, testutil.MustCreateToken(s.T(), overdraftTestUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	o, err := s.overdraftRepo.GetByUser(context.Background(), overdraftTestUserID)
	s.Require().NoError(err)
	s.Require().NotNil(o)
	s.Equal("inactive", string(o.Status))
}

func TestOverdraftSuite(t *testing.T) {
	suite.Run(t, new(OverdraftSuite))
}
