package cardauth_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	cardauth "github.com/vonmutinda/neo/internal/services/card_auth"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type CardAuthSuite struct {
	suite.Suite
	pool       *pgxpool.Pool
	authRepo   repository.CardAuthorizationRepository
	mockLedger *testutil.MockLedgerClient
	mockRates  *testutil.MockRateProvider
	svc        *cardauth.Service
	userID     string
	cardID     string
}

func (s *CardAuthSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	cards := repository.NewCardRepository(s.pool)
	s.authRepo = repository.NewCardAuthorizationRepository(s.pool)
	users := repository.NewUserRepository(s.pool)
	balances := repository.NewCurrencyBalanceRepository(s.pool)
	audit := repository.NewAuditRepository(s.pool)
	s.mockLedger = testutil.NewMockLedgerClient()
	chart := ledger.NewChart("test:")
	s.mockRates = testutil.NewMockRateProvider()
	iso4217 := cardauth.StaticISO4217Resolver{"840": "USD", "978": "EUR", "230": "ETB"}

	s.svc = cardauth.NewService(cards, s.authRepo, users, balances, audit, s.mockLedger, chart, s.mockRates, nil, iso4217)

	_ = cards
}

func (s *CardAuthSuite) SetupTest() {
	s.userID = uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, s.userID, phone.MustParse("+251912345678"))
	s.cardID = testutil.SeedCard(s.T(), s.pool, s.userID, domain.CardStatusActive)
}

func (s *CardAuthSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
	s.mockLedger.Balances = make(map[string]int64)
}

func (s *CardAuthSuite) seedApprovedAuth(rrn, holdID string) string {
	s.T().Helper()
	respCode := "00"
	return testutil.SeedCardAuthorization(s.T(), s.pool,
		s.cardID, rrn, "123456", "ETB", 5000,
		domain.AuthApproved, &respCode, &holdID)
}

func (s *CardAuthSuite) seedAuthWithStatus(rrn string, status domain.AuthStatus, holdID *string) string {
	s.T().Helper()
	var respCode *string
	if status == domain.AuthApproved {
		rc := "00"
		respCode = &rc
	} else if status == domain.AuthDeclined {
		rc := "05"
		respCode = &rc
	}
	return testutil.SeedCardAuthorization(s.T(), s.pool,
		s.cardID, rrn, "123456", "ETB", 3000,
		status, respCode, holdID)
}

// --- SettleAuthorization ---

func (s *CardAuthSuite) TestSettleAuthorization_Success() {
	ctx := context.Background()
	s.seedApprovedAuth("RRN001", "hold-abc")

	err := s.svc.SettleAuthorization(ctx, "RRN001", 4800)
	s.Require().NoError(err)

	auth, err := s.authRepo.GetByRRN(ctx, "RRN001")
	s.Require().NoError(err)
	s.Equal(domain.AuthCleared, auth.Status)
	s.Require().NotNil(auth.SettlementAmountCents)
	s.Equal(int64(4800), *auth.SettlementAmountCents)
	s.NotNil(auth.SettledAt)
}

func (s *CardAuthSuite) TestSettleAuthorization_AuthNotFound() {
	ctx := context.Background()

	err := s.svc.SettleAuthorization(ctx, "NONEXISTENT", 1000)
	s.Require().Error(err)
	s.True(strings.Contains(err.Error(), "looking up auth by RRN"))
}

func (s *CardAuthSuite) TestSettleAuthorization_WrongStatus_Declined() {
	ctx := context.Background()
	s.seedAuthWithStatus("RRN-DECLINED", domain.AuthDeclined, nil)

	err := s.svc.SettleAuthorization(ctx, "RRN-DECLINED", 3000)
	s.Require().Error(err)
	s.True(strings.Contains(err.Error(), "cannot settle"))
}

func (s *CardAuthSuite) TestSettleAuthorization_WrongStatus_Reversed() {
	ctx := context.Background()
	holdID := "hold-rev"
	s.seedAuthWithStatus("RRN-REVERSED", domain.AuthReversed, &holdID)

	err := s.svc.SettleAuthorization(ctx, "RRN-REVERSED", 2000)
	s.Require().Error(err)
	s.True(strings.Contains(err.Error(), "cannot settle"))
}

func (s *CardAuthSuite) TestSettleAuthorization_WrongStatus_Cleared() {
	ctx := context.Background()
	holdID := "hold-clr"
	s.seedAuthWithStatus("RRN-CLEARED", domain.AuthCleared, &holdID)

	err := s.svc.SettleAuthorization(ctx, "RRN-CLEARED", 1500)
	s.Require().Error(err)
	s.True(strings.Contains(err.Error(), "cannot settle"))
}

func (s *CardAuthSuite) TestSettleAuthorization_NoLedgerHoldID() {
	ctx := context.Background()
	respCode := "00"
	testutil.SeedCardAuthorization(s.T(), s.pool,
		s.cardID, "RRN-NOHOLD", "444444", "ETB", 1000,
		domain.AuthApproved, &respCode, nil)

	err := s.svc.SettleAuthorization(ctx, "RRN-NOHOLD", 1000)
	s.Require().Error(err)
	s.True(strings.Contains(err.Error(), "no ledger hold ID"))
}

func (s *CardAuthSuite) TestSettleAuthorization_PartialAmount() {
	ctx := context.Background()
	s.seedApprovedAuth("RRN-PARTIAL", "hold-partial")

	err := s.svc.SettleAuthorization(ctx, "RRN-PARTIAL", 2500)
	s.Require().NoError(err)

	auth, err := s.authRepo.GetByRRN(ctx, "RRN-PARTIAL")
	s.Require().NoError(err)
	s.Equal(domain.AuthCleared, auth.Status)
	s.Require().NotNil(auth.SettlementAmountCents)
	s.Equal(int64(2500), *auth.SettlementAmountCents)
}

// --- ReverseAuthorization ---

func (s *CardAuthSuite) TestReverseAuthorization_Success() {
	ctx := context.Background()
	s.seedApprovedAuth("RRN-REV", "hold-rev-ok")

	err := s.svc.ReverseAuthorization(ctx, "RRN-REV")
	s.Require().NoError(err)

	auth, err := s.authRepo.GetByRRN(ctx, "RRN-REV")
	s.Require().NoError(err)
	s.Equal(domain.AuthReversed, auth.Status)
	s.NotNil(auth.ReversedAt)
}

func (s *CardAuthSuite) TestReverseAuthorization_AuthNotFound() {
	ctx := context.Background()

	err := s.svc.ReverseAuthorization(ctx, "NONEXISTENT")
	s.Require().Error(err)
	s.True(strings.Contains(err.Error(), "looking up auth by RRN"))
}

func (s *CardAuthSuite) TestReverseAuthorization_WrongStatus_Declined() {
	ctx := context.Background()
	s.seedAuthWithStatus("RRN-REV-DECL", domain.AuthDeclined, nil)

	err := s.svc.ReverseAuthorization(ctx, "RRN-REV-DECL")
	s.Require().Error(err)
	s.True(strings.Contains(err.Error(), "cannot reverse"))
}

func (s *CardAuthSuite) TestReverseAuthorization_WrongStatus_Cleared() {
	ctx := context.Background()
	holdID := "hold-clr2"
	s.seedAuthWithStatus("RRN-REV-CLR", domain.AuthCleared, &holdID)

	err := s.svc.ReverseAuthorization(ctx, "RRN-REV-CLR")
	s.Require().Error(err)
	s.True(strings.Contains(err.Error(), "cannot reverse"))
}

func (s *CardAuthSuite) TestReverseAuthorization_WrongStatus_AlreadyReversed() {
	ctx := context.Background()
	holdID := "hold-rev2"
	s.seedAuthWithStatus("RRN-REV-REV", domain.AuthReversed, &holdID)

	err := s.svc.ReverseAuthorization(ctx, "RRN-REV-REV")
	s.Require().Error(err)
	s.True(strings.Contains(err.Error(), "cannot reverse"))
}

func (s *CardAuthSuite) TestReverseAuthorization_NoLedgerHoldID() {
	ctx := context.Background()
	respCode := "00"
	testutil.SeedCardAuthorization(s.T(), s.pool,
		s.cardID, "RRN-REV-NOHOLD", "888888", "ETB", 500,
		domain.AuthApproved, &respCode, nil)

	err := s.svc.ReverseAuthorization(ctx, "RRN-REV-NOHOLD")
	s.Require().Error(err)
	s.True(strings.Contains(err.Error(), "no ledger hold ID"))
}

// --- Settle then Reverse should fail ---

func (s *CardAuthSuite) TestSettleThenReverse_Fails() {
	ctx := context.Background()
	s.seedApprovedAuth("RRN-SEQ", "hold-seq")

	err := s.svc.SettleAuthorization(ctx, "RRN-SEQ", 5000)
	s.Require().NoError(err)

	err = s.svc.ReverseAuthorization(ctx, "RRN-SEQ")
	s.Require().Error(err)
	s.True(strings.Contains(err.Error(), "cannot reverse"))
}

// --- Reverse then Settle should fail ---

func (s *CardAuthSuite) TestReverseThenSettle_Fails() {
	ctx := context.Background()
	s.seedApprovedAuth("RRN-SEQ2", "hold-seq2")

	err := s.svc.ReverseAuthorization(ctx, "RRN-SEQ2")
	s.Require().NoError(err)

	err = s.svc.SettleAuthorization(ctx, "RRN-SEQ2", 5000)
	s.Require().Error(err)
	s.True(strings.Contains(err.Error(), "cannot settle"))
}

func TestCardAuthSuite(t *testing.T) {
	suite.Run(t, new(CardAuthSuite))
}
