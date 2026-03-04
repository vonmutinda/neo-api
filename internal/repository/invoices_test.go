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

type InvoiceSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.InvoiceRepository
}

func (s *InvoiceSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewInvoiceRepository(s.pool)
}

func (s *InvoiceSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *InvoiceSuite) seedBusiness(userID, bizID string) {
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
		PhoneNumber: phone.MustParse("+251911100024"),
	}
	bizRepo := repository.NewBusinessRepository(s.pool)
	s.Require().NoError(bizRepo.Create(context.Background(), biz))
}

func (s *InvoiceSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	inv := &domain.Invoice{
		BusinessID:    bizID,
		InvoiceNumber: "INV-2026-00001",
		CustomerName:  "Acme Customer",
		CurrencyCode:  "ETB",
		SubtotalCents: 100000,
		TaxCents:      15000,
		TotalCents:    115000,
		Status:        domain.InvoiceDraft,
		IssueDate:     "2026-02-22",
		DueDate:       "2026-03-22",
		CreatedBy:     userID,
	}
	err := s.repo.Create(ctx, inv)
	s.Require().NoError(err)
	s.NotEmpty(inv.ID)

	got, err := s.repo.GetByID(ctx, inv.ID)
	s.Require().NoError(err)
	s.Equal(inv.ID, got.ID)
	s.Equal(bizID, got.BusinessID)
	s.Equal("INV-2026-00001", got.InvoiceNumber)
	s.Equal("Acme Customer", got.CustomerName)
	s.Equal(int64(115000), got.TotalCents)
}

func (s *InvoiceSuite) TestGetByID_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetByID(ctx, uuid.NewString())
	s.ErrorIs(err, domain.ErrInvoiceNotFound)
}

func TestInvoiceSuite(t *testing.T) {
	suite.Run(t, new(InvoiceSuite))
}
