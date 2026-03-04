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

type BatchPaymentSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.BatchPaymentRepository
}

func (s *BatchPaymentSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewBatchPaymentRepository(s.pool)
}

func (s *BatchPaymentSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *BatchPaymentSuite) seedBusiness(userID, bizID string) {
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
		PhoneNumber: phone.MustParse("+251911100029"),
	}
	bizRepo := repository.NewBusinessRepository(s.pool)
	s.Require().NoError(bizRepo.Create(context.Background(), biz))
}

func (s *BatchPaymentSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	batch := &domain.BatchPayment{
		BusinessID:   bizID,
		Name:         "Payroll Jan 2026",
		TotalCents:   5000000,
		CurrencyCode: "ETB",
		ItemCount:    5,
		Status:       domain.BatchDraft,
		InitiatedBy:  userID,
	}
	err := s.repo.CreateBatch(ctx, batch)
	s.Require().NoError(err)
	s.NotEmpty(batch.ID)

	got, err := s.repo.GetBatch(ctx, batch.ID)
	s.Require().NoError(err)
	s.Equal(batch.ID, got.ID)
	s.Equal(bizID, got.BusinessID)
	s.Equal("Payroll Jan 2026", got.Name)
	s.Equal(int64(5000000), got.TotalCents)
	s.Equal(domain.BatchDraft, got.Status)
}

func (s *BatchPaymentSuite) TestGetBatch_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetBatch(ctx, uuid.NewString())
	s.ErrorIs(err, domain.ErrBatchNotFound)
}

func TestBatchPaymentSuite(t *testing.T) {
	suite.Run(t, new(BatchPaymentSuite))
}
