package lending_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/gateway/nbe"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/lending"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type DisbursementSuite struct {
	suite.Suite
	pool       *pgxpool.Pool
	svc        *lending.DisbursementService
	mockLedger *testutil.MockLedgerClient
	loanRepo   repository.LoanRepository
	auditRepo  repository.AuditRepository
}

func (s *DisbursementSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.mockLedger = testutil.NewMockLedgerClient()
	s.loanRepo = repository.NewLoanRepository(s.pool)
	userRepo := repository.NewUserRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)
	receiptRepo := repository.NewTransactionReceiptRepository(s.pool)
	nbeClient := nbe.NewStubClient()

	s.svc = lending.NewDisbursementService(s.loanRepo, userRepo, s.auditRepo, s.mockLedger, nbeClient, receiptRepo)
}

func (s *DisbursementSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *DisbursementSuite) TestDisburseLoan_Success() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCreditProfile(s.T(), s.pool, user.ID, 700, 500000)

	loan, err := s.svc.DisburseLoan(context.Background(), user.ID, &lending.LoanApplyRequest{
		PrincipalCents: 100000,
		DurationDays:   30,
	})
	s.Require().NoError(err)
	s.NotEmpty(loan.ID)
	s.Equal(user.ID, loan.UserID)
	s.Equal(int64(100000), loan.PrincipalAmountCents)

	ctx := context.Background()
	loanCount, err := s.loanRepo.CountByUser(ctx, user.ID)
	s.Require().NoError(err)
	s.Equal(1, loanCount)

	installments, err := s.loanRepo.ListInstallmentsByLoan(ctx, loan.ID)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(installments), 1)

	entries, err := s.auditRepo.ListByResource(ctx, "loan", loan.ID, 10)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(entries), 1)

	// Loan disbursement should create a transaction receipt
	receiptRepo := repository.NewTransactionReceiptRepository(s.pool)
	recs, err := receiptRepo.ListByUserID(ctx, user.ID, 10, 0)
	s.Require().NoError(err)
	var disbReceipt *domain.TransactionReceipt
	for i := range recs {
		if recs[i].Type == domain.ReceiptLoanDisbursement {
			disbReceipt = &recs[i]
			break
		}
	}
	s.Require().NotNil(disbReceipt, "user should have a loan_disbursement receipt")
	s.Equal(int64(100000), disbReceipt.AmountCents)
	s.Equal("ETB", disbReceipt.Currency)
	s.Equal(domain.ReceiptCompleted, disbReceipt.Status)
}

func (s *DisbursementSuite) TestDisburseLoan_FrozenUser() {
	user := testutil.SeedFrozenUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"), "AML hold")
	testutil.SeedCreditProfile(s.T(), s.pool, user.ID, 700, 500000)

	_, err := s.svc.DisburseLoan(context.Background(), user.ID, &lending.LoanApplyRequest{
		PrincipalCents: 100000,
		DurationDays:   30,
	})
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrUserFrozen)
}

func (s *DisbursementSuite) TestDisburseLoan_NoProfile() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	// no credit profile

	_, err := s.svc.DisburseLoan(context.Background(), user.ID, &lending.LoanApplyRequest{
		PrincipalCents: 100000,
		DurationDays:   30,
	})
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrNoEligibleProfile)
}

func (s *DisbursementSuite) TestDisburseLoan_ExceedsLimit() {
	user := testutil.SeedUser(s.T(), s.pool, testutil.TestUserID, phone.MustParse("+251912345678"))
	testutil.SeedCreditProfile(s.T(), s.pool, user.ID, 700, 50000)

	_, err := s.svc.DisburseLoan(context.Background(), user.ID, &lending.LoanApplyRequest{
		PrincipalCents: 100000,
		DurationDays:   30,
	})
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrLoanLimitExceeded)
}

func TestDisbursementSuite(t *testing.T) {
	suite.Run(t, new(DisbursementSuite))
}
