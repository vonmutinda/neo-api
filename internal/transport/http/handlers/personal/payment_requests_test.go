package personal_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/payment_requests"
	"github.com/vonmutinda/neo/internal/testutil"
	persh "github.com/vonmutinda/neo/internal/transport/http/handlers/personal"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/phone"
)

type PaymentRequestHandlerSuite struct {
	suite.Suite
	pool        *pgxpool.Pool
	server      *httptest.Server
	prRepo      repository.PaymentRequestRepository
	requesterID string
	payerID     string
}

func (s *PaymentRequestHandlerSuite) SetupSuite() {
	t := s.T()
	s.pool = testutil.MustStartPostgres(t)

	userRepo := repository.NewUserRepository(s.pool)
	s.prRepo = repository.NewPaymentRequestRepository(s.pool)
	auditRepo := repository.NewAuditRepository(s.pool)

	paymentRequestSvc := payment_requests.NewService(s.prRepo, userRepo, nil, auditRepo)

	handler := persh.NewHandlers(
		nil,
		nil, nil, nil, nil, nil,
		userRepo, nil, nil,
		nil, nil, nil, nil, nil,
		nil,
		paymentRequestSvc,
		nil, nil,
	)

	r := chi.NewRouter()
	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.Auth(testutil.TestJWTConfig()))
		r.Post("/payment-requests", handler.PaymentRequests.Create)
		r.Get("/payment-requests/sent", handler.PaymentRequests.ListSent)
		r.Get("/payment-requests/received", handler.PaymentRequests.ListReceived)
		r.Get("/payment-requests/received/count", handler.PaymentRequests.PendingCount)
		r.Get("/payment-requests/{id}", handler.PaymentRequests.Get)
		r.Post("/payment-requests/{id}/decline", handler.PaymentRequests.Decline)
		r.Delete("/payment-requests/{id}", handler.PaymentRequests.Cancel)
		r.Post("/payment-requests/{id}/remind", handler.PaymentRequests.Remind)
	})

	s.server = httptest.NewServer(r)
	t.Cleanup(s.server.Close)
}

func (s *PaymentRequestHandlerSuite) SetupTest() {
	s.requesterID = uuid.NewString()
	s.payerID = uuid.NewString()
}

func (s *PaymentRequestHandlerSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *PaymentRequestHandlerSuite) requesterToken() string {
	return testutil.MustCreateToken(s.T(), s.requesterID)
}

func (s *PaymentRequestHandlerSuite) payerToken() string {
	return testutil.MustCreateToken(s.T(), s.payerID)
}

func (s *PaymentRequestHandlerSuite) seedBothUsers() {
	testutil.SeedUser(s.T(), s.pool, s.requesterID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, s.payerID, phone.MustParse("+251911111111"))
}

// --- POST /v1/payment-requests ---

func (s *PaymentRequestHandlerSuite) TestCreate_Success() {
	s.seedBothUsers()

	body := map[string]any{
		"recipient":    "+251911111111",
		"amountCents":  500_00,
		"currencyCode": "ETB",
		"narration":    "lunch split",
	}
	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/payment-requests", body, s.requesterToken())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusCreated, resp.StatusCode)

	var pr domain.PaymentRequest
	testutil.MustDecodeJSON(s.T(), resp, &pr)
	s.NotEmpty(pr.ID)
	s.Equal(s.requesterID, pr.RequesterID)
	s.Equal(int64(500_00), pr.AmountCents)
	s.Equal(domain.PaymentRequestPending, pr.Status)
}

// --- GET /v1/payment-requests/sent ---

func (s *PaymentRequestHandlerSuite) TestListSent_Empty() {
	s.seedBothUsers()

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/payment-requests/sent", nil, s.requesterToken())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var list []domain.PaymentRequest
	testutil.MustDecodeJSON(s.T(), resp, &list)
	s.Empty(list)
}

// --- GET /v1/payment-requests/received ---

func (s *PaymentRequestHandlerSuite) TestListReceived_WithRequests() {
	s.seedBothUsers()
	payerPhone := phone.MustParse("+251911111111")
	testutil.SeedPaymentRequest(s.T(), s.pool, s.requesterID, s.payerID, payerPhone, 500_00, "ETB")

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/payment-requests/received", nil, s.payerToken())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var list []domain.PaymentRequest
	testutil.MustDecodeJSON(s.T(), resp, &list)
	s.Len(list, 1)
	s.Equal(int64(500_00), list[0].AmountCents)
}

// --- GET /v1/payment-requests/{id} ---

func (s *PaymentRequestHandlerSuite) TestGet_Success() {
	s.seedBothUsers()
	payerPhone := phone.MustParse("+251911111111")
	id := testutil.SeedPaymentRequest(s.T(), s.pool, s.requesterID, s.payerID, payerPhone, 500_00, "ETB")

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/payment-requests/"+id, nil, s.requesterToken())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var pr domain.PaymentRequest
	testutil.MustDecodeJSON(s.T(), resp, &pr)
	s.Equal(id, pr.ID)
}

func (s *PaymentRequestHandlerSuite) TestGet_NotFound() {
	s.seedBothUsers()

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/payment-requests/"+uuid.NewString(), nil, s.requesterToken())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusNotFound, resp.StatusCode)
}

// --- POST /v1/payment-requests/{id}/decline ---

func (s *PaymentRequestHandlerSuite) TestDecline_Success() {
	s.seedBothUsers()
	payerPhone := phone.MustParse("+251911111111")
	id := testutil.SeedPaymentRequest(s.T(), s.pool, s.requesterID, s.payerID, payerPhone, 500_00, "ETB")

	body := map[string]string{"reason": "not now"}
	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/payment-requests/"+id+"/decline", body, s.payerToken())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusNoContent, resp.StatusCode)

	got, err := s.prRepo.GetByID(context.Background(), id)
	s.Require().NoError(err)
	s.Equal(domain.PaymentRequestDeclined, got.Status)
}

// --- DELETE /v1/payment-requests/{id} ---

func (s *PaymentRequestHandlerSuite) TestCancel_Success() {
	s.seedBothUsers()
	payerPhone := phone.MustParse("+251911111111")
	id := testutil.SeedPaymentRequest(s.T(), s.pool, s.requesterID, s.payerID, payerPhone, 500_00, "ETB")

	req := testutil.NewAuthRequest(s.T(), "DELETE", s.server.URL+"/v1/payment-requests/"+id, nil, s.requesterToken())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusNoContent, resp.StatusCode)

	got, err := s.prRepo.GetByID(context.Background(), id)
	s.Require().NoError(err)
	s.Equal(domain.PaymentRequestCancelled, got.Status)
}

// --- GET /v1/payment-requests/received/count ---

func (s *PaymentRequestHandlerSuite) TestPendingCount() {
	s.seedBothUsers()
	payerPhone := phone.MustParse("+251911111111")
	testutil.SeedPaymentRequest(s.T(), s.pool, s.requesterID, s.payerID, payerPhone, 100_00, "ETB")
	testutil.SeedPaymentRequest(s.T(), s.pool, s.requesterID, s.payerID, payerPhone, 200_00, "ETB")

	req := testutil.NewAuthRequest(s.T(), "GET", s.server.URL+"/v1/payment-requests/received/count", nil, s.payerToken())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var result map[string]int
	testutil.MustDecodeJSON(s.T(), resp, &result)
	s.Equal(2, result["count"])
}

// --- POST /v1/payment-requests/{id}/remind ---

func (s *PaymentRequestHandlerSuite) TestRemind_Success() {
	s.seedBothUsers()
	payerPhone := phone.MustParse("+251911111111")
	id := testutil.SeedPaymentRequest(s.T(), s.pool, s.requesterID, s.payerID, payerPhone, 500_00, "ETB")

	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/payment-requests/"+id+"/remind", nil, s.requesterToken())
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusNoContent, resp.StatusCode)

	got, err := s.prRepo.GetByID(context.Background(), id)
	s.Require().NoError(err)
	s.Equal(1, got.ReminderCount)
}

func TestPaymentRequestHandlerSuite(t *testing.T) {
	suite.Run(t, new(PaymentRequestHandlerSuite))
}
