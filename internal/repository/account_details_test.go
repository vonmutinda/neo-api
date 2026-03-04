package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type AccountDetailsSuite struct {
	suite.Suite
	pool        *pgxpool.Pool
	detailsRepo repository.AccountDetailsRepository
	balanceRepo repository.CurrencyBalanceRepository
}

func (s *AccountDetailsSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.detailsRepo = repository.NewAccountDetailsRepository(s.pool)
	s.balanceRepo = repository.NewCurrencyBalanceRepository(s.pool)
}

func (s *AccountDetailsSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *AccountDetailsSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	balanceID := testutil.SeedCurrencyBalance(s.T(), s.pool, userID, "ETB", true)

	details := &domain.AccountDetails{
		CurrencyBalanceID: balanceID,
		IBAN:              "ET00NEO0000000001ETB",
		AccountNumber:     "0000000001",
		BankName:          "Neo Bank Ethiopia",
		SwiftCode:         "NEOBETET",
	}
	err := s.detailsRepo.Create(ctx, details)
	s.Require().NoError(err)
	s.NotEmpty(details.ID)
}

func (s *AccountDetailsSuite) TestGetByCurrencyBalanceID() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345679"))

	balanceID := testutil.SeedCurrencyBalance(s.T(), s.pool, userID, "ETB", true)

	details := &domain.AccountDetails{
		CurrencyBalanceID: balanceID,
		IBAN:              "ET00NEO0000000002ETB",
		AccountNumber:     "0000000002",
		BankName:          "Neo Bank Ethiopia",
		SwiftCode:         "NEOBETET",
	}
	s.Require().NoError(s.detailsRepo.Create(ctx, details))

	got, err := s.detailsRepo.GetByCurrencyBalanceID(ctx, balanceID)
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Equal(details.ID, got.ID)
	s.Equal(balanceID, got.CurrencyBalanceID)
	s.Equal("ET00NEO0000000002ETB", got.IBAN)
	s.Equal("0000000002", got.AccountNumber)
	s.Equal("Neo Bank Ethiopia", got.BankName)
	s.Equal("NEOBETET", got.SwiftCode)
}

func (s *AccountDetailsSuite) TestGetByCurrencyBalanceID_NotFound() {
	ctx := context.Background()
	got, err := s.detailsRepo.GetByCurrencyBalanceID(ctx, uuid.NewString())
	s.Require().NoError(err)
	s.Nil(got)
}

func (s *AccountDetailsSuite) TestListByUser() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345680"))

	balance1ID := testutil.SeedCurrencyBalance(s.T(), s.pool, userID, "ETB", true)
	balance2ID := testutil.SeedCurrencyBalance(s.T(), s.pool, userID, "USD", false)

	details1 := &domain.AccountDetails{
		CurrencyBalanceID: balance1ID,
		IBAN:              "ET00NEO0000000003ETB",
		AccountNumber:     "0000000003",
		BankName:          "Neo Bank Ethiopia",
		SwiftCode:         "NEOBETET",
	}
	details2 := &domain.AccountDetails{
		CurrencyBalanceID: balance2ID,
		IBAN:              "ET00NEO0000000004USD",
		AccountNumber:     "0000000004",
		BankName:          "Neo Bank Ethiopia",
		SwiftCode:         "NEOBETET",
	}
	s.Require().NoError(s.detailsRepo.Create(ctx, details1))
	s.Require().NoError(s.detailsRepo.Create(ctx, details2))

	list, err := s.detailsRepo.ListByUser(ctx, userID)
	s.Require().NoError(err)
	s.Len(list, 2)
}

func (s *AccountDetailsSuite) TestNextAccountNumber() {
	ctx := context.Background()

	first, err := s.detailsRepo.NextAccountNumber(ctx)
	s.Require().NoError(err)

	second, err := s.detailsRepo.NextAccountNumber(ctx)
	s.Require().NoError(err)

	s.Greater(second, first)
}

func TestAccountDetailsSuite(t *testing.T) {
	suite.Run(t, new(AccountDetailsSuite))
}
