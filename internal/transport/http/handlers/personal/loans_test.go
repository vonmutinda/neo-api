package personal_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/gateway/nbe"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/lending"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/vonmutinda/neo/internal/transport/http/handlers/personal"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
)

const testUserID = "550e8400-e29b-41d4-a716-446655440001"

type LoanSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	server   *httptest.Server
	mockLedger *testutil.MockLedgerClient
	loanRepo repository.LoanRepository
}

func (s *LoanSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.mockLedger = testutil.NewMockLedgerClient()

	s.loanRepo = repository.NewLoanRepository(s.pool)
	userRepo := repository.NewUserRepository(s.pool)
	auditRepo := repository.NewAuditRepository(s.pool)
	receiptRepo := repository.NewTransactionReceiptRepository(s.pool)
	nbeClient := nbe.NewStubClient()

	disbursementSvc := lending.NewDisbursementService(s.loanRepo, userRepo, auditRepo, s.mockLedger, nbeClient, receiptRepo)
	loanQuerySvc := lending.NewQueryService(s.loanRepo, userRepo)

	handler := personal.NewLoanHandler(disbursementSvc, loanQuerySvc)

	r := chi.NewRouter()
	r.Use(middleware.Auth(testutil.TestJWTConfig()))
	r.Get("/v1/loans/eligibility", handler.GetEligibility)
	r.Get("/v1/loans", handler.ListHistory)
	r.Post("/v1/loans/apply", handler.Apply)
	r.Get("/v1/loans/{id}", handler.GetLoan)

	s.server = httptest.NewServer(r)
}

func (s *LoanSuite) TearDownSuite() {
	if s.server != nil {
		s.server.Close()
	}
}

func (s *LoanSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

// TestGetEligibility_Eligible seeds user + credit profile (score=700, limit=500000) -> 200 with eligibility data
func (s *LoanSuite) TestGetEligibility_Eligible() {
	user := testutil.SeedUser(s.T(), s.pool, testUserID, phone.MustParse("+251912345678"))
	testutil.SeedCreditProfile(s.T(), s.pool, user.ID, 700, 500000)

	req := testutil.NewAuthRequest(s.T(), http.MethodGet, s.server.URL+"/v1/loans/eligibility", nil, testutil.MustCreateToken(s.T(), testUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var data struct {
		IsEligible         bool   `json:"isEligible"`
		TrustScore         int    `json:"trustScore"`
		ApprovedLimitCents int64  `json:"approvedLimitCents"`
		AvailableCents     int64  `json:"availableCents"`
	}
	testutil.MustDecodeJSON(s.T(), resp, &data)
	s.True(data.IsEligible)
	s.Equal(700, data.TrustScore)
	s.Equal(int64(500000), data.ApprovedLimitCents)
	s.Equal(int64(500000), data.AvailableCents)
}

// TestGetEligibility_NoProfile seeds user without profile -> 200 with isEligible false
func (s *LoanSuite) TestGetEligibility_NoProfile() {
	testutil.SeedUser(s.T(), s.pool, testUserID, phone.MustParse("+251912345678"))
	// no credit profile

	req := testutil.NewAuthRequest(s.T(), http.MethodGet, s.server.URL+"/v1/loans/eligibility", nil, testutil.MustCreateToken(s.T(), testUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var data struct {
		IsEligible bool   `json:"isEligible"`
		Reason     string `json:"reason"`
	}
	testutil.MustDecodeJSON(s.T(), resp, &data)
	s.False(data.IsEligible)
	s.NotEmpty(data.Reason)
}

// TestApply_Success seeds user + credit profile, POST -> 201, loan in DB
func (s *LoanSuite) TestApply_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testUserID, phone.MustParse("+251912345678"))
	testutil.SeedCreditProfile(s.T(), s.pool, user.ID, 700, 500000)

	body := map[string]any{
		"principalCents": 100000,
		"durationDays":  30,
	}
	req := testutil.NewAuthRequest(s.T(), http.MethodPost, s.server.URL+"/v1/loans/apply", body, testutil.MustCreateToken(s.T(), testUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	var data struct {
		ID                 string `json:"id"`
		UserID             string `json:"userId"`
		PrincipalAmountCents int64  `json:"principalAmountCents"`
	}
	testutil.MustDecodeJSON(s.T(), resp, &data)
	s.NotEmpty(data.ID)
	s.Equal(testUserID, data.UserID)
	s.Equal(int64(100000), data.PrincipalAmountCents)

	loanCount, err := s.loanRepo.CountByUser(context.Background(), user.ID)
	s.Require().NoError(err)
	s.Equal(1, loanCount)
}

// TestApply_ExceedsLimit seeds profile with limit=50000, request 100000 -> 422
func (s *LoanSuite) TestApply_ExceedsLimit() {
	user := testutil.SeedUser(s.T(), s.pool, testUserID, phone.MustParse("+251912345678"))
	testutil.SeedCreditProfile(s.T(), s.pool, user.ID, 700, 50000)

	body := map[string]any{
		"principalCents": 100000,
		"durationDays":  30,
	}
	req := testutil.NewAuthRequest(s.T(), http.MethodPost, s.server.URL+"/v1/loans/apply", body, testutil.MustCreateToken(s.T(), testUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusUnprocessableEntity, resp.StatusCode)
}

// TestListHistory seeds user + apply a loan, GET /v1/loans -> 200 with array
func (s *LoanSuite) TestListHistory() {
	user := testutil.SeedUser(s.T(), s.pool, testUserID, phone.MustParse("+251912345678"))
	testutil.SeedCreditProfile(s.T(), s.pool, user.ID, 700, 500000)

	// Apply a loan first
	applyBody := map[string]any{
		"principalCents": 100000,
		"durationDays":  30,
	}
	applyReq := testutil.NewAuthRequest(s.T(), http.MethodPost, s.server.URL+"/v1/loans/apply", applyBody, testutil.MustCreateToken(s.T(), testUserID))
	applyResp := testutil.DoRequest(s.T(), applyReq)
	defer applyResp.Body.Close()
	s.Require().Equal(http.StatusCreated, applyResp.StatusCode)

	req := testutil.NewAuthRequest(s.T(), http.MethodGet, s.server.URL+"/v1/loans", nil, testutil.MustCreateToken(s.T(), testUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var data struct {
		Loans      []any `json:"loans"`
		TotalCount int   `json:"totalCount"`
	}
	testutil.MustDecodeJSON(s.T(), resp, &data)
	s.Len(data.Loans, 1)
	s.Equal(1, data.TotalCount)
}

// TestGetLoan_Success applies a loan, GET /v1/loans/{id} -> 200
func (s *LoanSuite) TestGetLoan_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testUserID, phone.MustParse("+251912345678"))
	testutil.SeedCreditProfile(s.T(), s.pool, user.ID, 700, 500000)

	applyBody := map[string]any{
		"principalCents": 100000,
		"durationDays":  30,
	}
	applyReq := testutil.NewAuthRequest(s.T(), http.MethodPost, s.server.URL+"/v1/loans/apply", applyBody, testutil.MustCreateToken(s.T(), testUserID))
	applyResp := testutil.DoRequest(s.T(), applyReq)
	defer applyResp.Body.Close()
	s.Require().Equal(http.StatusCreated, applyResp.StatusCode)

	var loanData struct {
		ID string `json:"id"`
	}
	testutil.MustDecodeJSON(s.T(), applyResp, &loanData)
	loanID := loanData.ID
	s.NotEmpty(loanID)

	req := testutil.NewAuthRequest(s.T(), http.MethodGet, s.server.URL+"/v1/loans/"+loanID, nil, testutil.MustCreateToken(s.T(), testUserID))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var data struct {
		ID                 string `json:"id"`
		PrincipalAmountCents int64  `json:"principalAmountCents"`
	}
	testutil.MustDecodeJSON(s.T(), resp, &data)
	s.Equal(loanID, data.ID)
	s.Equal(int64(100000), data.PrincipalAmountCents)
}

func TestLoanSuite(t *testing.T) {
	suite.Run(t, new(LoanSuite))
}
