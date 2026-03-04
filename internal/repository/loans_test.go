package repository_test

import (
	"context"
	"fmt"
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

type LoanSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.LoanRepository
}

func (s *LoanSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewLoanRepository(s.pool)
}

func (s *LoanSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *LoanSuite) TestUpsertCreditProfile_Create() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	profile := &domain.CreditProfile{
		UserID:              userID,
		TrustScore:          650,
		ApprovedLimitCents:  100000,
		AvgMonthlyInflowCents:  50000,
		AvgMonthlyBalanceCents: 25000,
		ActiveDaysPerMonth: 20,
		TotalLoansRepaid:   2,
		LatePaymentsCount:   0,
		CurrentOutstandingCents: 0,
		IsNBEBlacklisted:   false,
		LastCalculatedAt:   time.Now(),
	}
	err := s.repo.UpsertCreditProfile(ctx, profile)
	s.Require().NoError(err)

	got, err := s.repo.GetCreditProfile(ctx, userID)
	s.Require().NoError(err)
	s.Equal(userID, got.UserID)
	s.Equal(650, got.TrustScore)
	s.Equal(int64(100000), got.ApprovedLimitCents)
}

func (s *LoanSuite) TestUpsertCreditProfile_Update() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345679"))

	profile := &domain.CreditProfile{
		UserID:              userID,
		TrustScore:          500,
		ApprovedLimitCents:  50000,
		AvgMonthlyInflowCents:  40000,
		AvgMonthlyBalanceCents: 20000,
		ActiveDaysPerMonth: 15,
		TotalLoansRepaid:   1,
		LatePaymentsCount:   0,
		CurrentOutstandingCents: 0,
		IsNBEBlacklisted:   false,
		LastCalculatedAt:   time.Now(),
	}
	s.Require().NoError(s.repo.UpsertCreditProfile(ctx, profile))

	profile.TrustScore = 750
	profile.ApprovedLimitCents = 150000
	s.Require().NoError(s.repo.UpsertCreditProfile(ctx, profile))

	got, err := s.repo.GetCreditProfile(ctx, userID)
	s.Require().NoError(err)
	s.Equal(750, got.TrustScore)
	s.Equal(int64(150000), got.ApprovedLimitCents)
}

func (s *LoanSuite) TestGetCreditProfile_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetCreditProfile(ctx, uuid.NewString())
	s.ErrorIs(err, domain.ErrNoEligibleProfile)
}

func (s *LoanSuite) TestCreateLoan_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345680"))

	loan := &domain.Loan{
		UserID:               userID,
		PrincipalAmountCents: 100000,
		InterestFeeCents:      5000,
		TotalDueCents:        105000,
		DurationDays:         30,
		DueDate:              time.Now().AddDate(0, 0, 30),
		LedgerLoanAccount:    "loan:test-1",
	}
	err := s.repo.CreateLoan(ctx, loan)
	s.Require().NoError(err)
	s.NotEmpty(loan.ID)

	got, err := s.repo.GetLoan(ctx, loan.ID)
	s.Require().NoError(err)
	s.Equal(loan.ID, got.ID)
	s.Equal(userID, got.UserID)
	s.Equal(int64(100000), got.PrincipalAmountCents)
	s.Equal(int64(105000), got.TotalDueCents)
	s.Equal(domain.LoanActive, got.Status)
}

func (s *LoanSuite) TestGetLoan_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetLoan(ctx, uuid.NewString())
	s.ErrorIs(err, domain.ErrLoanNotFound)
}

func (s *LoanSuite) TestListActiveByUser() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345681"))

	for i := 1; i <= 2; i++ {
		loan := &domain.Loan{
			UserID:               userID,
			PrincipalAmountCents: int64(50000 * i),
			InterestFeeCents:      2500,
			TotalDueCents:        int64(52500 * i),
			DurationDays:         30,
			DueDate:              time.Now().AddDate(0, 0, 30),
			LedgerLoanAccount:    fmt.Sprintf("loan:test-%d", i),
		}
		s.Require().NoError(s.repo.CreateLoan(ctx, loan))
	}

	list, err := s.repo.ListActiveByUser(ctx, userID)
	s.Require().NoError(err)
	s.Len(list, 2)
}

func (s *LoanSuite) TestListAllByUser() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345682"))

	for i := 1; i <= 3; i++ {
		loan := &domain.Loan{
			UserID:               userID,
			PrincipalAmountCents: int64(30000 * i),
			InterestFeeCents:      1500,
			TotalDueCents:        int64(31500 * i),
			DurationDays:         30,
			DueDate:              time.Now().AddDate(0, 0, 30),
			LedgerLoanAccount:    fmt.Sprintf("loan:list-%d", i),
		}
		s.Require().NoError(s.repo.CreateLoan(ctx, loan))
	}

	list, err := s.repo.ListAllByUser(ctx, userID, 2, 0)
	s.Require().NoError(err)
	s.Len(list, 2)
}

func (s *LoanSuite) TestCountByUser() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345683"))

	loan1 := &domain.Loan{
		UserID:               userID,
		PrincipalAmountCents: 100000,
		InterestFeeCents:      5000,
		TotalDueCents:        105000,
		DurationDays:         30,
		DueDate:              time.Now().AddDate(0, 0, 30),
		LedgerLoanAccount:    "loan:count-1",
	}
	loan2 := &domain.Loan{
		UserID:               userID,
		PrincipalAmountCents: 50000,
		InterestFeeCents:      2500,
		TotalDueCents:        52500,
		DurationDays:         30,
		DueDate:              time.Now().AddDate(0, 0, 30),
		LedgerLoanAccount:    "loan:count-2",
	}
	s.Require().NoError(s.repo.CreateLoan(ctx, loan1))
	s.Require().NoError(s.repo.CreateLoan(ctx, loan2))

	count, err := s.repo.CountByUser(ctx, userID)
	s.Require().NoError(err)
	s.Equal(2, count)
}

func (s *LoanSuite) TestUpdateLoanStatus() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345684"))

	loan := &domain.Loan{
		UserID:               userID,
		PrincipalAmountCents: 100000,
		InterestFeeCents:      5000,
		TotalDueCents:        105000,
		DurationDays:         30,
		DueDate:              time.Now().AddDate(0, 0, 30),
		LedgerLoanAccount:    "loan:status-test",
	}
	s.Require().NoError(s.repo.CreateLoan(ctx, loan))

	err := s.repo.UpdateLoanStatus(ctx, loan.ID, domain.LoanInArrears, 5)
	s.Require().NoError(err)

	got, err := s.repo.GetLoan(ctx, loan.ID)
	s.Require().NoError(err)
	s.Equal(domain.LoanInArrears, got.Status)
	s.Equal(5, got.DaysPastDue)
}

func (s *LoanSuite) TestIncrementPaid() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345685"))

	loan := &domain.Loan{
		UserID:               userID,
		PrincipalAmountCents: 100000,
		InterestFeeCents:      5000,
		TotalDueCents:        105000,
		DurationDays:         30,
		DueDate:              time.Now().AddDate(0, 0, 30),
		LedgerLoanAccount:    "loan:increment-test",
	}
	s.Require().NoError(s.repo.CreateLoan(ctx, loan))

	err := s.repo.IncrementPaid(ctx, loan.ID, 50000)
	s.Require().NoError(err)

	got, err := s.repo.GetLoan(ctx, loan.ID)
	s.Require().NoError(err)
	s.Equal(int64(50000), got.TotalPaidCents)
}

func (s *LoanSuite) TestCreateInstallments() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345686"))

	loan := &domain.Loan{
		UserID:               userID,
		PrincipalAmountCents: 105000,
		InterestFeeCents:      0,
		TotalDueCents:        105000,
		DurationDays:         90,
		DueDate:               time.Now().AddDate(0, 3, 0),
		LedgerLoanAccount:    "loan:installments-test",
	}
	s.Require().NoError(s.repo.CreateLoan(ctx, loan))

	installments := []domain.LoanInstallment{
		{LoanID: loan.ID, InstallmentNumber: 1, AmountDueCents: 35000, DueDate: time.Now().AddDate(0, 1, 0)},
		{LoanID: loan.ID, InstallmentNumber: 2, AmountDueCents: 35000, DueDate: time.Now().AddDate(0, 2, 0)},
		{LoanID: loan.ID, InstallmentNumber: 3, AmountDueCents: 35000, DueDate: time.Now().AddDate(0, 3, 0)},
	}
	err := s.repo.CreateInstallments(ctx, installments)
	s.Require().NoError(err)

	list, err := s.repo.ListInstallmentsByLoan(ctx, loan.ID)
	s.Require().NoError(err)
	s.Len(list, 3)
	s.Equal(1, list[0].InstallmentNumber)
	s.Equal(2, list[1].InstallmentNumber)
	s.Equal(3, list[2].InstallmentNumber)
}

func (s *LoanSuite) TestMarkInstallmentPaid() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345687"))

	loan := &domain.Loan{
		UserID:               userID,
		PrincipalAmountCents: 35000,
		InterestFeeCents:      0,
		TotalDueCents:        35000,
		DurationDays:         30,
		DueDate:              time.Now().AddDate(0, 1, 0),
		LedgerLoanAccount:    "loan:mark-paid-test",
	}
	s.Require().NoError(s.repo.CreateLoan(ctx, loan))

	installments := []domain.LoanInstallment{
		{LoanID: loan.ID, InstallmentNumber: 1, AmountDueCents: 35000, DueDate: time.Now().AddDate(0, 1, 0)},
	}
	s.Require().NoError(s.repo.CreateInstallments(ctx, installments))

	list, err := s.repo.ListInstallmentsByLoan(ctx, loan.ID)
	s.Require().NoError(err)
	s.Require().Len(list, 1)
	instID := list[0].ID

	err = s.repo.MarkInstallmentPaid(ctx, instID, "ledger-repay-tx-1")
	s.Require().NoError(err)

	list, err = s.repo.ListInstallmentsByLoan(ctx, loan.ID)
	s.Require().NoError(err)
	s.Require().Len(list, 1)
	s.True(list[0].IsPaid)
}

func TestLoanSuite(t *testing.T) {
	suite.Run(t, new(LoanSuite))
}
