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

type BusinessLoanSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.BusinessLoanRepository
}

func (s *BusinessLoanSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewBusinessLoanRepository(s.pool)
}

func (s *BusinessLoanSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *BusinessLoanSuite) seedBusinessAndProfile(userID, bizID string) {
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))
	tradeName := "Test Corp"
	biz := &domain.Business{
		ID:                 bizID,
		OwnerUserID:        userID,
		Name:               "Test Corp",
		TradeName:          &tradeName,
		TINNumber:          "TIN-" + uuid.NewString()[:8],
		TradeLicenseNumber: "TL-" + uuid.NewString()[:8],
		IndustryCategory:   domain.IndustryRetail,
		Status:             domain.BusinessStatusActive,
		LedgerWalletID:     "wallet:biz-" + uuid.NewString(),
		PhoneNumber: phone.MustParse("+251911100027"),
	}
	bizRepo := repository.NewBusinessRepository(s.pool)
	s.Require().NoError(bizRepo.Create(context.Background(), biz))

	cp := &domain.BusinessCreditProfile{
		BusinessID:             bizID,
		TrustScore:              750,
		ApprovedLimitCents:      10000000,
		AvgMonthlyRevenueCents:  5000000,
		AvgMonthlyExpensesCents: 3000000,
		CashFlowScore:           80,
		TimeInBusinessMonths:    24,
		IndustryRiskScore:       50,
		TotalLoansRepaid:        2,
		LatePaymentsCount:       0,
		CurrentOutstandingCents: 0,
		CollateralValueCents:    2000000,
		IsNBEBlacklisted:        false,
	}
	s.Require().NoError(s.repo.UpsertCreditProfile(context.Background(), cp))
}

func (s *BusinessLoanSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusinessAndProfile(userID, bizID)

	dueDate := time.Now().Add(90 * 24 * time.Hour)
	loan := &domain.BusinessLoan{
		BusinessID:           bizID,
		PrincipalAmountCents: 1000000,
		InterestFeeCents:     50000,
		TotalDueCents:        1050000,
		DurationDays:         90,
		DueDate:              dueDate,
		LedgerLoanAccount:    "loan:biz-" + uuid.NewString(),
		AppliedBy:            userID,
	}
	err := s.repo.CreateLoan(ctx, loan)
	s.Require().NoError(err)
	s.NotEmpty(loan.ID)

	got, err := s.repo.GetLoan(ctx, loan.ID)
	s.Require().NoError(err)
	s.Equal(loan.ID, got.ID)
	s.Equal(bizID, got.BusinessID)
	s.Equal(int64(1000000), got.PrincipalAmountCents)
	s.Equal(int64(1050000), got.TotalDueCents)
	s.Equal(userID, got.AppliedBy)
}

func (s *BusinessLoanSuite) TestGetLoan_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetLoan(ctx, uuid.NewString())
	s.ErrorIs(err, domain.ErrLoanNotFound)
}

func TestBusinessLoanSuite(t *testing.T) {
	suite.Run(t, new(BusinessLoanSuite))
}
