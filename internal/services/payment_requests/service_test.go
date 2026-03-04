package payment_requests_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/payment_requests"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type PaymentRequestServiceSuite struct {
	suite.Suite
	pool      *pgxpool.Pool
	svc       *payment_requests.Service
	prRepo    repository.PaymentRequestRepository
	auditRepo repository.AuditRepository
}

func (s *PaymentRequestServiceSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.prRepo = repository.NewPaymentRequestRepository(s.pool)
	userRepo := repository.NewUserRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)
	s.svc = payment_requests.NewService(s.prRepo, userRepo, nil, s.auditRepo)
}

func (s *PaymentRequestServiceSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *PaymentRequestServiceSuite) countAuditActions(action domain.AuditAction) int {
	var n int
	err := s.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE action = $1`, string(action),
	).Scan(&n)
	s.Require().NoError(err)
	return n
}

func (s *PaymentRequestServiceSuite) seedUsers() (string, string) {
	requesterID := uuid.NewString()
	payerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, requesterID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, payerID, phone.MustParse("+251911111111"))
	return requesterID, payerID
}

func (s *PaymentRequestServiceSuite) createRequest(requesterID string, recipientPhone string) *domain.PaymentRequest {
	pr, err := s.svc.Create(context.Background(), requesterID, &payment_requests.CreatePaymentRequestForm{
		Recipient:    recipientPhone,
		AmountCents:  500_00,
		CurrencyCode: "ETB",
		Narration:    "test request",
	})
	s.Require().NoError(err)
	return pr
}

// --- Create ---

func (s *PaymentRequestServiceSuite) TestCreate_Success() {
	ctx := context.Background()
	requesterID, _ := s.seedUsers()

	pr, err := s.svc.Create(ctx, requesterID, &payment_requests.CreatePaymentRequestForm{
		Recipient:    "+251911111111",
		AmountCents:  500_00,
		CurrencyCode: "ETB",
		Narration:    "lunch split",
	})
	s.Require().NoError(err)
	s.NotEmpty(pr.ID)
	s.Equal(requesterID, pr.RequesterID)
	s.Equal(int64(500_00), pr.AmountCents)
	s.Equal(domain.PaymentRequestPending, pr.Status)

	s.GreaterOrEqual(s.countAuditActions(domain.AuditPaymentRequestCreated), 1)
}

func (s *PaymentRequestServiceSuite) TestCreate_SelfRequest() {
	requesterID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, requesterID, phone.MustParse("+251912345678"))

	_, err := s.svc.Create(context.Background(), requesterID, &payment_requests.CreatePaymentRequestForm{
		Recipient:    "+251912345678",
		AmountCents:  500_00,
		CurrencyCode: "ETB",
		Narration:    "self request",
	})
	s.ErrorIs(err, domain.ErrSelfRequest)
}

// --- Decline ---

func (s *PaymentRequestServiceSuite) TestDecline_Success() {
	requesterID, payerID := s.seedUsers()
	pr := s.createRequest(requesterID, "+251911111111")

	err := s.svc.Decline(context.Background(), payerID, pr.ID, &payment_requests.DeclineForm{Reason: "not now"})
	s.Require().NoError(err)

	got, err := s.prRepo.GetByID(context.Background(), pr.ID)
	s.Require().NoError(err)
	s.Equal(domain.PaymentRequestDeclined, got.Status)

	s.GreaterOrEqual(s.countAuditActions(domain.AuditPaymentRequestDeclined), 1)
}

// --- Cancel ---

func (s *PaymentRequestServiceSuite) TestCancel_Success() {
	requesterID, _ := s.seedUsers()
	pr := s.createRequest(requesterID, "+251911111111")

	err := s.svc.Cancel(context.Background(), requesterID, pr.ID)
	s.Require().NoError(err)

	got, err := s.prRepo.GetByID(context.Background(), pr.ID)
	s.Require().NoError(err)
	s.Equal(domain.PaymentRequestCancelled, got.Status)

	s.GreaterOrEqual(s.countAuditActions(domain.AuditPaymentRequestCancelled), 1)
}

// --- Remind ---

func (s *PaymentRequestServiceSuite) TestRemind_Success() {
	requesterID, _ := s.seedUsers()
	pr := s.createRequest(requesterID, "+251911111111")

	err := s.svc.Remind(context.Background(), requesterID, pr.ID)
	s.Require().NoError(err)

	got, err := s.prRepo.GetByID(context.Background(), pr.ID)
	s.Require().NoError(err)
	s.Equal(1, got.ReminderCount)

	s.GreaterOrEqual(s.countAuditActions(domain.AuditPaymentRequestReminded), 1)
}

func (s *PaymentRequestServiceSuite) TestRemind_LimitReached() {
	requesterID, _ := s.seedUsers()
	pr := s.createRequest(requesterID, "+251911111111")

	for i := 0; i < 3; i++ {
		s.Require().NoError(s.svc.Remind(context.Background(), requesterID, pr.ID))
	}

	err := s.svc.Remind(context.Background(), requesterID, pr.ID)
	s.ErrorIs(err, domain.ErrReminderLimitReached)
}

// --- Get ---

func (s *PaymentRequestServiceSuite) TestGet_Success() {
	requesterID, _ := s.seedUsers()
	pr := s.createRequest(requesterID, "+251911111111")

	got, err := s.svc.Get(context.Background(), requesterID, pr.ID)
	s.Require().NoError(err)
	s.Equal(pr.ID, got.ID)
}

func (s *PaymentRequestServiceSuite) TestGet_NotVisible() {
	requesterID, _ := s.seedUsers()
	pr := s.createRequest(requesterID, "+251911111111")

	unrelatedID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, unrelatedID, phone.MustParse("+251922222222"))

	_, err := s.svc.Get(context.Background(), unrelatedID, pr.ID)
	s.ErrorIs(err, domain.ErrPaymentRequestNotFound)
}

// --- PendingCount ---

func (s *PaymentRequestServiceSuite) TestPendingCount() {
	requesterID, payerID := s.seedUsers()

	s.createRequest(requesterID, "+251911111111")
	s.createRequest(requesterID, "+251911111111")

	count, err := s.svc.PendingCount(context.Background(), payerID)
	s.Require().NoError(err)
	s.Equal(2, count)
}

// --- ExpireStale ---

func (s *PaymentRequestServiceSuite) TestExpireStale() {
	ctx := context.Background()
	requesterID, payerID := s.seedUsers()
	payerPhone := phone.MustParse("+251911111111")
	testutil.SeedPaymentRequest(s.T(), s.pool, requesterID, payerID, payerPhone, 500_00, "ETB")

	_, err := s.pool.Exec(ctx,
		`UPDATE payment_requests SET expires_at = NOW() - INTERVAL '1 hour' WHERE requester_id = $1`,
		requesterID)
	s.Require().NoError(err)

	n, err := s.svc.ExpireStale(ctx)
	s.Require().NoError(err)
	s.Equal(int64(1), n)

	s.GreaterOrEqual(s.countAuditActions(domain.AuditPaymentRequestExpired), 1)
}

func TestPaymentRequestServiceSuite(t *testing.T) {
	suite.Run(t, new(PaymentRequestServiceSuite))
}
