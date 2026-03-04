package overdraft_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/overdraft"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type OverdraftServiceSuite struct {
	suite.Suite
	pool         *pgxpool.Pool
	overdraftRepo repository.OverdraftRepository
	loanRepo     repository.LoanRepository
	userRepo     repository.UserRepository
	auditRepo    repository.AuditRepository
	svc          *overdraft.Service
}

func (s *OverdraftServiceSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.overdraftRepo = repository.NewOverdraftRepository(s.pool)
	s.loanRepo = repository.NewLoanRepository(s.pool)
	s.userRepo = repository.NewUserRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)
	s.svc = overdraft.NewService(
		s.overdraftRepo,
		s.loanRepo,
		s.userRepo,
		testutil.NewMockLedgerClient(),
		s.auditRepo,
	)
}

func (s *OverdraftServiceSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *OverdraftServiceSuite) TestEligibility_NoProfile() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911111111"))

	limit, eligible, err := s.svc.Eligibility(ctx, userID)
	s.Require().NoError(err)
	s.False(eligible)
	s.Equal(int64(0), limit)
}

func (s *OverdraftServiceSuite) TestEligibility_Eligible() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911222222"))
	testutil.SeedCreditProfile(s.T(), s.pool, userID, 700, 5000000) // 10% = 500,000 cents = 5,000 ETB

	limit, eligible, err := s.svc.Eligibility(ctx, userID)
	s.Require().NoError(err)
	s.True(eligible)
	s.Equal(int64(500000), limit)
}

func (s *OverdraftServiceSuite) TestEligibility_CappedAt50kETB() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911333333"))
	testutil.SeedCreditProfile(s.T(), s.pool, userID, 750, 100000000) // 10% = 10M cents, cap 5M

	limit, eligible, err := s.svc.Eligibility(ctx, userID)
	s.Require().NoError(err)
	s.True(eligible)
	s.Equal(int64(5000000), limit)
}

func (s *OverdraftServiceSuite) TestOptIn_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911444444"))
	testutil.SeedCreditProfile(s.T(), s.pool, userID, 700, 1000000)

	o, err := s.svc.OptIn(ctx, userID)
	s.Require().NoError(err)
	s.Require().NotNil(o)
	s.Equal(userID, o.UserID)
	s.Equal(int64(100000), o.LimitCents)
	s.Equal(domain.OverdraftActive, o.Status)
}

func (s *OverdraftServiceSuite) TestOptIn_NotEligible() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911555555"))

	_, err := s.svc.OptIn(ctx, userID)
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrOverdraftNotEligible)
}

func (s *OverdraftServiceSuite) TestOptOut_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911666666"))
	testutil.SeedCreditProfile(s.T(), s.pool, userID, 700, 1000000)
	_, err := s.svc.OptIn(ctx, userID)
	s.Require().NoError(err)

	err = s.svc.OptOut(ctx, userID)
	s.Require().NoError(err)

	o, _ := s.overdraftRepo.GetByUser(ctx, userID)
	s.Require().NotNil(o)
	s.Equal(domain.OverdraftInactive, o.Status)
}

func (s *OverdraftServiceSuite) TestOptOut_InUse() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911777777"))
	testutil.SeedCreditProfile(s.T(), s.pool, userID, 700, 1000000)
	_, err := s.svc.OptIn(ctx, userID)
	s.Require().NoError(err)
	_, err = s.pool.Exec(ctx, `UPDATE overdrafts SET used_cents = 1000, status = 'used' WHERE user_id = $1`, userID)
	s.Require().NoError(err)

	err = s.svc.OptOut(ctx, userID)
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrOverdraftInUse)
}

func (s *OverdraftServiceSuite) TestGetStatus_Inactive() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911888888"))
	testutil.SeedCreditProfile(s.T(), s.pool, userID, 700, 1000000)

	resp, err := s.svc.GetStatus(ctx, userID)
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(domain.OverdraftInactive, resp.Status)
	s.Equal(int64(100000), resp.LimitCents)
	s.NotEmpty(resp.FeeSummary)
}

func (s *OverdraftServiceSuite) TestGetStatus_Active() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911999999"))
	testutil.SeedCreditProfile(s.T(), s.pool, userID, 700, 1000000)
	_, err := s.svc.OptIn(ctx, userID)
	s.Require().NoError(err)

	resp, err := s.svc.GetStatus(ctx, userID)
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(domain.OverdraftActive, resp.Status)
	s.Equal(int64(100000), resp.LimitCents)
	s.Equal(int64(100000), resp.AvailableCents)
}

// TestAutoRepayOnInflow_ReturnsRepayCents verifies that AutoRepayOnInflow returns the amount repaid.
func (s *OverdraftServiceSuite) TestAutoRepayOnInflow_ReturnsRepayCents() {
	ctx := context.Background()
	userID := uuid.NewString()
	user := testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251911000001"))
	testutil.SeedCreditProfile(s.T(), s.pool, userID, 700, 1000000)
	_, err := s.svc.OptIn(ctx, userID)
	s.Require().NoError(err)
	_, err = s.pool.Exec(ctx, `UPDATE overdrafts SET used_cents = 3000, status = 'used' WHERE user_id = $1`, userID)
	s.Require().NoError(err)

	mockLedger := testutil.NewMockLedgerClient()
	mockLedger.Balances[user.LedgerWalletID] = 10000
	svc := overdraft.NewService(s.overdraftRepo, s.loanRepo, s.userRepo, mockLedger, s.auditRepo)

	repayCents, err := svc.AutoRepayOnInflow(ctx, userID, user.LedgerWalletID, "ik-repay", 10000)
	s.Require().NoError(err)
	s.Equal(int64(3000), repayCents)
}

func TestOverdraftServiceSuite(t *testing.T) {
	suite.Run(t, new(OverdraftServiceSuite))
}
