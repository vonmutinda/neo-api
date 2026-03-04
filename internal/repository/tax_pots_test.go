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

type TaxPotSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.TaxPotRepository
}

func (s *TaxPotSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewTaxPotRepository(s.pool)
}

func (s *TaxPotSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *TaxPotSuite) seedBusinessAndPot(userID, bizID string) string {
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))
	potID := testutil.SeedPot(s.T(), s.pool, userID, "Tax Pot", "ETB")

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
		PhoneNumber: phone.MustParse("+251911100022"),
	}
	bizRepo := repository.NewBusinessRepository(s.pool)
	s.Require().NoError(bizRepo.Create(context.Background(), biz))
	return potID
}

func (s *TaxPotSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	potID := s.seedBusinessAndPot(userID, bizID)

	tp := &domain.TaxPot{
		BusinessID: bizID,
		PotID:      potID,
		TaxType:    domain.TaxVAT,
		Notes:      strPtr("VAT withholding"),
	}
	err := s.repo.Create(ctx, tp)
	s.Require().NoError(err)
	s.NotEmpty(tp.ID)

	got, err := s.repo.GetByID(ctx, tp.ID)
	s.Require().NoError(err)
	s.Equal(tp.ID, got.ID)
	s.Equal(bizID, got.BusinessID)
	s.Equal(potID, got.PotID)
	s.Equal(domain.TaxVAT, got.TaxType)
}

func (s *TaxPotSuite) TestGetByID_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetByID(ctx, uuid.NewString())
	s.ErrorIs(err, domain.ErrTaxPotNotFound)
}

func TestTaxPotSuite(t *testing.T) {
	suite.Run(t, new(TaxPotSuite))
}
