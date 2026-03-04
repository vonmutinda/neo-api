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
	"github.com/vonmutinda/neo/internal/services/payments"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
	persh "github.com/vonmutinda/neo/internal/transport/http/handlers/personal"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
)

const (
	transferUserID1 = "550e8400-e29b-41d4-a716-446655440001"
	transferUserID2 = "550e8400-e29b-41d4-a716-446655440002"
)

type TransferSuite struct {
	suite.Suite
	pool         *pgxpool.Pool
	server       *httptest.Server
	mockLedger   *testutil.MockLedgerClient
	mockEthSwitch *testutil.MockEthSwitchClient
	receiptRepo  repository.TransactionReceiptRepository
}

func (s *TransferSuite) SetupSuite() {
	t := s.T()
	s.pool = testutil.MustStartPostgres(t)

	userRepo := repository.NewUserRepository(s.pool)
	s.receiptRepo = repository.NewTransactionReceiptRepository(s.pool)
	auditRepo := repository.NewAuditRepository(s.pool)
	s.mockLedger = testutil.NewMockLedgerClient()
	s.mockEthSwitch = testutil.NewMockEthSwitchClient()
	chart := ledger.NewChart("neo")

	paymentsSvc := payments.NewService(userRepo, s.receiptRepo, auditRepo, s.mockLedger, s.mockEthSwitch, chart, nil, nil, nil, nil, nil)

	handler := persh.NewHandlers(
		nil,
		nil, paymentsSvc, nil, nil, nil,
		userRepo, nil, nil,
		nil, nil, nil, nil, nil,
		nil,
		nil,
		nil, nil,
	)

	r := chi.NewRouter()
	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.Auth(testutil.TestJWTConfig()))
		r.Post("/transfers/outbound", handler.Transfers.Outbound)
		r.Post("/transfers/inbound", handler.Transfers.Inbound)
	})

	s.server = httptest.NewServer(r)
	t.Cleanup(s.server.Close)
}

func (s *TransferSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *TransferSuite) TestOutbound_Success() {
	user := testutil.SeedUser(s.T(), s.pool, transferUserID1, phone.MustParse("+251912345678"))
	s.mockLedger.Balances[user.LedgerWalletID] = 500000

	body := map[string]interface{}{
		"amountCents":     100000,
		"currency":        "ETB",
		"destPhone":       "+251911111111",
		"destInstitution": "CBE",
		"narration":       "test payment",
	}
	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/transfers/outbound", body, testutil.MustCreateToken(s.T(), transferUserID1))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	list, err := s.receiptRepo.ListByUserID(context.Background(), transferUserID1, 100, 0)
	s.Require().NoError(err)
	s.Len(list, 1)
}

func (s *TransferSuite) TestOutbound_FrozenUser() {
	_ = testutil.SeedFrozenUser(s.T(), s.pool, transferUserID1, phone.MustParse("+251912345678"), "fraud")
	s.mockLedger.Balances["wallet:"+transferUserID1] = 500000

	body := map[string]interface{}{
		"amountCents":     100000,
		"currency":        "ETB",
		"destPhone":       "+251911111111",
		"destInstitution": "CBE",
		"narration":       "test payment",
	}
	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/transfers/outbound", body, testutil.MustCreateToken(s.T(), transferUserID1))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusForbidden, resp.StatusCode)
}

func (s *TransferSuite) TestOutbound_MissingAuth() {
	body := map[string]interface{}{
		"amountCents":     100000,
		"currency":        "ETB",
		"destPhone":       "+251911111111",
		"destInstitution": "CBE",
		"narration":       "test payment",
	}
	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/transfers/outbound", body, "")
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusUnauthorized, resp.StatusCode)
}

func (s *TransferSuite) TestInbound_Success() {
	sender := testutil.SeedUser(s.T(), s.pool, transferUserID1, phone.MustParse("+251912345678"))
	recipient := testutil.SeedUser(s.T(), s.pool, transferUserID2, phone.MustParse("+251911111111"))
	s.mockLedger.Balances[sender.LedgerWalletID] = 500000

	body := map[string]interface{}{
		"recipientPhone": recipient.PhoneNumber,
		"amountCents":    100000,
		"currency":       "ETB",
		"narration":      "lunch money",
	}
	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/transfers/inbound", body, testutil.MustCreateToken(s.T(), transferUserID1))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	senderList, err := s.receiptRepo.ListByUserID(context.Background(), transferUserID1, 100, 0)
	s.Require().NoError(err)
	recipientList, err := s.receiptRepo.ListByUserID(context.Background(), transferUserID2, 100, 0)
	s.Require().NoError(err)
	s.Equal(2, len(senderList)+len(recipientList))
}

func (s *TransferSuite) TestInbound_SelfTransfer() {
	user := testutil.SeedUser(s.T(), s.pool, transferUserID1, phone.MustParse("+251912345678"))
	s.mockLedger.Balances[user.LedgerWalletID] = 500000

	body := map[string]interface{}{
		"recipientPhone": user.PhoneNumber,
		"amountCents":    100000,
		"currency":       "ETB",
		"narration":      "self",
	}
	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/transfers/inbound", body, testutil.MustCreateToken(s.T(), transferUserID1))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *TransferSuite) TestInbound_InsufficientFunds() {
	sender := testutil.SeedUser(s.T(), s.pool, transferUserID1, phone.MustParse("+251912345678"))
	recipient := testutil.SeedUser(s.T(), s.pool, transferUserID2, phone.MustParse("+251911111111"))
	s.mockLedger.Balances[sender.LedgerWalletID] = 0

	body := map[string]interface{}{
		"recipientPhone": recipient.PhoneNumber,
		"amountCents":    100000,
		"currency":       "ETB",
		"narration":      "lunch",
	}
	req := testutil.NewAuthRequest(s.T(), "POST", s.server.URL+"/v1/transfers/inbound", body, testutil.MustCreateToken(s.T(), transferUserID1))
	resp := testutil.DoRequest(s.T(), req)
	defer resp.Body.Close()

	s.Equal(http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestTransferSuite(t *testing.T) {
	suite.Run(t, new(TransferSuite))
}
