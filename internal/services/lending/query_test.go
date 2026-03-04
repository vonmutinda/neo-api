package lending_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/lending"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type LendingQuerySuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	loanRepo repository.LoanRepository
	userRepo repository.UserRepository
	svc      *lending.QueryService
}

func (s *LendingQuerySuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.loanRepo = repository.NewLoanRepository(s.pool)
	s.userRepo = repository.NewUserRepository(s.pool)
	s.svc = lending.NewQueryService(s.loanRepo, s.userRepo)
}

func (s *LendingQuerySuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *LendingQuerySuite) seedUser() string {
	id := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, id, phone.MustParse("+251912345678"))
	return id
}

// --- GetEligibility Tests ---

func (s *LendingQuerySuite) TestGetEligibility_NoProfile() {
	userID := s.seedUser()

	elig, err := s.svc.GetEligibility(context.Background(), userID)
	s.Require().NoError(err)
	s.False(elig.IsEligible)
	s.NotEmpty(elig.Reason)
}

func (s *LendingQuerySuite) TestGetEligibility_EligibleUser() {
	userID := s.seedUser()
	testutil.SeedCreditProfile(s.T(), s.pool, userID, 750, 5000000)

	elig, err := s.svc.GetEligibility(context.Background(), userID)
	s.Require().NoError(err)
	s.True(elig.IsEligible)
	s.False(elig.HasActiveLoan)
	s.Equal(750, elig.TrustScore)
	s.Equal(int64(5000000), elig.AvailableCents)
	s.Empty(elig.Reason)
}

func (s *LendingQuerySuite) TestGetEligibility_LowTrustScore() {
	userID := s.seedUser()
	testutil.SeedCreditProfile(s.T(), s.pool, userID, 400, 0)

	elig, err := s.svc.GetEligibility(context.Background(), userID)
	s.Require().NoError(err)
	s.False(elig.IsEligible)
	s.NotEmpty(elig.Reason)
}

func (s *LendingQuerySuite) TestGetEligibility_MaxedOutLimit() {
	userID := s.seedUser()
	testutil.SeedCreditProfile(s.T(), s.pool, userID, 800, 5000000)
	_, err := s.pool.Exec(context.Background(),
		`UPDATE credit_profiles SET current_outstanding_cents = 5000000 WHERE user_id = $1`, userID)
	s.Require().NoError(err)

	elig, err := s.svc.GetEligibility(context.Background(), userID)
	s.Require().NoError(err)
	s.False(elig.IsEligible)
	s.Equal(int64(0), elig.AvailableCents)
}

func (s *LendingQuerySuite) TestGetEligibility_NBEBlacklisted() {
	userID := s.seedUser()
	testutil.SeedCreditProfile(s.T(), s.pool, userID, 800, 5000000)
	_, err := s.pool.Exec(context.Background(),
		`UPDATE credit_profiles SET is_nbe_blacklisted = true WHERE user_id = $1`, userID)
	s.Require().NoError(err)

	elig, err := s.svc.GetEligibility(context.Background(), userID)
	s.Require().NoError(err)
	s.False(elig.IsEligible)
	s.True(elig.IsNBEBlacklisted)
}

func (s *LendingQuerySuite) TestGetEligibility_UserNotFound() {
	_, err := s.svc.GetEligibility(context.Background(), uuid.NewString())
	s.Error(err)
}

// --- ListHistory Tests ---

func (s *LendingQuerySuite) TestListHistory_Empty() {
	userID := s.seedUser()

	page, err := s.svc.ListHistory(context.Background(), userID, 20, 0)
	s.Require().NoError(err)
	s.Len(page.Loans, 0)
	s.Equal(0, page.TotalCount)
}

func (s *LendingQuerySuite) TestListHistory_WithLoans() {
	userID := s.seedUser()

	testutil.SeedLoan(s.T(), s.pool, userID, 1000000, 500000, domain.LoanActive)
	testutil.SeedLoan(s.T(), s.pool, userID, 2000000, 2100000, domain.LoanRepaid)
	testutil.SeedLoan(s.T(), s.pool, userID, 500000, 0, domain.LoanActive)

	page, err := s.svc.ListHistory(context.Background(), userID, 20, 0)
	s.Require().NoError(err)
	s.Len(page.Loans, 3)
	s.Equal(3, page.TotalCount)

	s.Equal(2, page.Stats.ActiveLoansCount)
	s.Equal(1, page.Stats.CompletedLoansCount)

	expectedBorrowed := int64(1000000 + 2000000 + 500000)
	s.Equal(expectedBorrowed, page.Stats.TotalBorrowedCents)
}

func (s *LendingQuerySuite) TestListHistory_Pagination() {
	userID := s.seedUser()

	for i := 0; i < 5; i++ {
		testutil.SeedLoan(s.T(), s.pool, userID, 100000, 0, domain.LoanActive)
	}

	page1, err := s.svc.ListHistory(context.Background(), userID, 2, 0)
	s.Require().NoError(err)
	s.Len(page1.Loans, 2)
	s.Equal(5, page1.TotalCount)

	page2, err := s.svc.ListHistory(context.Background(), userID, 2, 2)
	s.Require().NoError(err)
	s.Len(page2.Loans, 2)

	page3, err := s.svc.ListHistory(context.Background(), userID, 2, 4)
	s.Require().NoError(err)
	s.Len(page3.Loans, 1)
}

// --- GetLoanDetail Tests ---

func (s *LendingQuerySuite) TestGetLoanDetail_Success() {
	userID := s.seedUser()
	loanID := testutil.SeedLoan(s.T(), s.pool, userID, 1000000, 300000, domain.LoanActive)

	testutil.SeedLoanInstallment(s.T(), s.pool, loanID, 1, 525000, true)
	testutil.SeedLoanInstallment(s.T(), s.pool, loanID, 2, 525000, false)

	detail, err := s.svc.GetLoanDetail(context.Background(), userID, loanID)
	s.Require().NoError(err)
	s.Equal(loanID, detail.ID)
	s.Len(detail.Installments, 2)
	s.Equal(detail.TotalDueCents-detail.TotalPaidCents, detail.RemainingCents)
}

func (s *LendingQuerySuite) TestGetLoanDetail_OwnershipCheck() {
	userID := s.seedUser()
	otherID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, otherID, phone.MustParse("+251900000000"))

	loanID := testutil.SeedLoan(s.T(), s.pool, otherID, 1000000, 0, domain.LoanActive)

	_, err := s.svc.GetLoanDetail(context.Background(), userID, loanID)
	s.Error(err)
}

func (s *LendingQuerySuite) TestGetLoanDetail_NotFound() {
	userID := s.seedUser()

	_, err := s.svc.GetLoanDetail(context.Background(), userID, uuid.NewString())
	s.Error(err)
}

// --- HasActiveLoan Tests ---

func (s *LendingQuerySuite) TestGetEligibility_HasActiveLoan() {
	userID := s.seedUser()
	testutil.SeedCreditProfile(s.T(), s.pool, userID, 800, 5000000)
	_, err := s.pool.Exec(context.Background(),
		`UPDATE credit_profiles SET current_outstanding_cents = 1000000 WHERE user_id = $1`, userID)
	s.Require().NoError(err)

	elig, err := s.svc.GetEligibility(context.Background(), userID)
	s.Require().NoError(err)
	s.False(elig.IsEligible)
	s.True(elig.HasActiveLoan)
	s.Contains(elig.Reason, "outstanding loan")
}

// --- Credit Score Breakdown Tests ---

func (s *LendingQuerySuite) TestGetCreditScore_NoProfile() {
	userID := s.seedUser()

	breakdown, err := s.svc.GetCreditScore(context.Background(), userID)
	s.Require().NoError(err)
	s.Equal(300, breakdown.TrustScore)
	s.Equal(1000, breakdown.MaxScore)
	s.Equal(300, breakdown.BasePoints)
	s.NotEmpty(breakdown.Tips)
}

func (s *LendingQuerySuite) TestGetCreditScore_WithProfile() {
	userID := s.seedUser()
	testutil.SeedCreditProfile(s.T(), s.pool, userID, 750, 5000000)

	breakdown, err := s.svc.GetCreditScore(context.Background(), userID)
	s.Require().NoError(err)
	s.Equal(750, breakdown.TrustScore)
	s.Equal(1000, breakdown.MaxScore)
	s.Equal(300, breakdown.BasePoints)
	s.GreaterOrEqual(breakdown.CashFlowPoints, 0)
	s.GreaterOrEqual(breakdown.StabilityPoints, 0)
	s.LessOrEqual(breakdown.PenaltyPoints, 0)
	s.NotEmpty(breakdown.Tips)
}

func (s *LendingQuerySuite) TestGetCreditScore_WithLatePayments() {
	userID := s.seedUser()
	testutil.SeedCreditProfile(s.T(), s.pool, userID, 700, 3000000)
	_, err := s.pool.Exec(context.Background(),
		`UPDATE credit_profiles SET late_payments_count = 2 WHERE user_id = $1`, userID)
	s.Require().NoError(err)

	breakdown, err := s.svc.GetCreditScore(context.Background(), userID)
	s.Require().NoError(err)
	s.Equal(-100, breakdown.PenaltyPoints)

	hasLateTip := false
	for _, tip := range breakdown.Tips {
		if tip == "Avoid late loan payments -- each one reduces your score by 50 points." {
			hasLateTip = true
		}
	}
	s.True(hasLateTip)
}

func TestLendingQuerySuite(t *testing.T) {
	suite.Run(t, new(LendingQuerySuite))
}
