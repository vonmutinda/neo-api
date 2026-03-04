package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type PaymentRequestRepoSuite struct {
	suite.Suite
	pool *pgxpool.Pool
	repo repository.PaymentRequestRepository
}

func (s *PaymentRequestRepoSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewPaymentRequestRepository(s.pool)
}

func (s *PaymentRequestRepoSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *PaymentRequestRepoSuite) seedUsers() (string, string) {
	requesterID := uuid.NewString()
	payerID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, requesterID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, payerID, phone.MustParse("+251911111111"))
	return requesterID, payerID
}

// --- Create ---

func (s *PaymentRequestRepoSuite) TestCreate_Success() {
	ctx := context.Background()
	requesterID, payerID := s.seedUsers()
	payerPhone := phone.MustParse("+251911111111")

	pr := &domain.PaymentRequest{
		RequesterID:  requesterID,
		PayerID:      &payerID,
		PayerPhone:   payerPhone,
		AmountCents:  500_00,
		CurrencyCode: "ETB",
		Narration:    "lunch split",
		ExpiresAt:    time.Now().AddDate(0, 0, 30),
	}
	err := s.repo.Create(ctx, pr)
	s.Require().NoError(err)
	s.NotEmpty(pr.ID)
	s.Equal(domain.PaymentRequestPending, pr.Status)
	s.False(pr.CreatedAt.IsZero())
	s.False(pr.UpdatedAt.IsZero())
}

// --- GetByID ---

func (s *PaymentRequestRepoSuite) TestGetByID_Success() {
	ctx := context.Background()
	requesterID, payerID := s.seedUsers()
	payerPhone := phone.MustParse("+251911111111")
	id := testutil.SeedPaymentRequest(s.T(), s.pool, requesterID, payerID, payerPhone, 1000_00, "ETB")

	got, err := s.repo.GetByID(ctx, id)
	s.Require().NoError(err)
	s.Equal(id, got.ID)
	s.Equal(requesterID, got.RequesterID)
	s.Require().NotNil(got.PayerID)
	s.Equal(payerID, *got.PayerID)
	s.Equal(int64(1000_00), got.AmountCents)
	s.Equal("ETB", got.CurrencyCode)
	s.Equal(domain.PaymentRequestPending, got.Status)
}

func (s *PaymentRequestRepoSuite) TestGetByID_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetByID(ctx, uuid.NewString())
	s.ErrorIs(err, domain.ErrPaymentRequestNotFound)
}

// --- Pay ---

func (s *PaymentRequestRepoSuite) TestPay_Success() {
	ctx := context.Background()
	requesterID, payerID := s.seedUsers()
	payerPhone := phone.MustParse("+251911111111")
	id := testutil.SeedPaymentRequest(s.T(), s.pool, requesterID, payerID, payerPhone, 500_00, "ETB")

	txID := uuid.NewString()
	err := s.repo.Pay(ctx, id, txID)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, id)
	s.Require().NoError(err)
	s.Equal(domain.PaymentRequestPaid, got.Status)
	s.Require().NotNil(got.TransactionID)
	s.Equal(txID, *got.TransactionID)
	s.NotNil(got.PaidAt)
}

// --- Decline ---

func (s *PaymentRequestRepoSuite) TestDecline_Success() {
	ctx := context.Background()
	requesterID, payerID := s.seedUsers()
	payerPhone := phone.MustParse("+251911111111")
	id := testutil.SeedPaymentRequest(s.T(), s.pool, requesterID, payerID, payerPhone, 500_00, "ETB")

	err := s.repo.Decline(ctx, id, "not now")
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, id)
	s.Require().NoError(err)
	s.Equal(domain.PaymentRequestDeclined, got.Status)
	s.Require().NotNil(got.DeclineReason)
	s.Equal("not now", *got.DeclineReason)
	s.NotNil(got.DeclinedAt)
}

// --- Cancel ---

func (s *PaymentRequestRepoSuite) TestCancel_Success() {
	ctx := context.Background()
	requesterID, payerID := s.seedUsers()
	payerPhone := phone.MustParse("+251911111111")
	id := testutil.SeedPaymentRequest(s.T(), s.pool, requesterID, payerID, payerPhone, 500_00, "ETB")

	err := s.repo.Cancel(ctx, id)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, id)
	s.Require().NoError(err)
	s.Equal(domain.PaymentRequestCancelled, got.Status)
	s.NotNil(got.CancelledAt)
}

// --- IncrementReminder ---

func (s *PaymentRequestRepoSuite) TestIncrementReminder_Success() {
	ctx := context.Background()
	requesterID, payerID := s.seedUsers()
	payerPhone := phone.MustParse("+251911111111")
	id := testutil.SeedPaymentRequest(s.T(), s.pool, requesterID, payerID, payerPhone, 500_00, "ETB")

	for i := 0; i < 3; i++ {
		s.Require().NoError(s.repo.IncrementReminder(ctx, id))
	}

	got, err := s.repo.GetByID(ctx, id)
	s.Require().NoError(err)
	s.Equal(3, got.ReminderCount)
	s.NotNil(got.LastRemindedAt)

	err = s.repo.IncrementReminder(ctx, id)
	s.ErrorIs(err, domain.ErrReminderLimitReached)
}

// --- ListByRequester ---

func (s *PaymentRequestRepoSuite) TestListByRequester() {
	ctx := context.Background()
	requesterID, payerID := s.seedUsers()
	payerPhone := phone.MustParse("+251911111111")

	testutil.SeedPaymentRequest(s.T(), s.pool, requesterID, payerID, payerPhone, 100_00, "ETB")
	testutil.SeedPaymentRequest(s.T(), s.pool, requesterID, payerID, payerPhone, 200_00, "ETB")

	list, err := s.repo.ListByRequester(ctx, requesterID, 10, 0)
	s.Require().NoError(err)
	s.Len(list, 2)
}

// --- ListByPayer ---

func (s *PaymentRequestRepoSuite) TestListByPayer() {
	ctx := context.Background()
	requesterID, payerID := s.seedUsers()
	payerPhone := phone.MustParse("+251911111111")

	testutil.SeedPaymentRequest(s.T(), s.pool, requesterID, payerID, payerPhone, 100_00, "ETB")
	testutil.SeedPaymentRequest(s.T(), s.pool, requesterID, payerID, payerPhone, 200_00, "ETB")

	list, err := s.repo.ListByPayer(ctx, payerID, 10, 0)
	s.Require().NoError(err)
	s.Len(list, 2)
}

// --- CountPendingByPayer ---

func (s *PaymentRequestRepoSuite) TestCountPendingByPayer() {
	ctx := context.Background()
	requesterID, payerID := s.seedUsers()
	payerPhone := phone.MustParse("+251911111111")

	testutil.SeedPaymentRequest(s.T(), s.pool, requesterID, payerID, payerPhone, 100_00, "ETB")
	testutil.SeedPaymentRequest(s.T(), s.pool, requesterID, payerID, payerPhone, 200_00, "ETB")
	paidID := testutil.SeedPaymentRequest(s.T(), s.pool, requesterID, payerID, payerPhone, 300_00, "ETB")
	s.Require().NoError(s.repo.Pay(ctx, paidID, uuid.NewString()))

	count, err := s.repo.CountPendingByPayer(ctx, payerID)
	s.Require().NoError(err)
	s.Equal(2, count)
}

// --- ExpirePending ---

func (s *PaymentRequestRepoSuite) TestExpirePending() {
	ctx := context.Background()
	requesterID, payerID := s.seedUsers()
	payerPhone := phone.MustParse("+251911111111")

	testutil.SeedPaymentRequest(s.T(), s.pool, requesterID, payerID, payerPhone, 100_00, "ETB")

	_, err := s.pool.Exec(ctx,
		`UPDATE payment_requests SET expires_at = NOW() - INTERVAL '1 hour' WHERE requester_id = $1`,
		requesterID)
	s.Require().NoError(err)

	n, err := s.repo.ExpirePending(ctx)
	s.Require().NoError(err)
	s.Equal(int64(1), n)

	got, err := s.repo.GetByID(ctx, func() string {
		var id string
		s.Require().NoError(s.pool.QueryRow(ctx,
			`SELECT id FROM payment_requests WHERE requester_id = $1`, requesterID).Scan(&id))
		return id
	}())
	s.Require().NoError(err)
	s.Equal(domain.PaymentRequestExpired, got.Status)
}

func TestPaymentRequestRepoSuite(t *testing.T) {
	suite.Run(t, new(PaymentRequestRepoSuite))
}
