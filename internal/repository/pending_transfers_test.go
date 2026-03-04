package repository_test

import (
	"context"
	"encoding/json"
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

type PendingTransferSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.PendingTransferRepository
}

func (s *PendingTransferSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewPendingTransferRepository(s.pool)
}

func (s *PendingTransferSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *PendingTransferSuite) seedBusiness(userID, bizID string) {
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
		PhoneNumber: phone.MustParse("+251911100023"),
	}
	bizRepo := repository.NewBusinessRepository(s.pool)
	s.Require().NoError(bizRepo.Create(context.Background(), biz))
}

func (s *PendingTransferSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	bizID := uuid.NewString()
	s.seedBusiness(userID, bizID)

	recipientInfo := json.RawMessage(`{"phone":"+251911111111"}`)
	pt := &domain.PendingTransfer{
		BusinessID:    bizID,
		InitiatedBy:   userID,
		TransferType:  "p2p",
		AmountCents:    100000,
		CurrencyCode:  "ETB",
		RecipientInfo: recipientInfo,
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}
	err := s.repo.Create(ctx, pt)
	s.Require().NoError(err)
	s.NotEmpty(pt.ID)
	s.Equal(domain.PendingTransferPending, pt.Status)

	got, err := s.repo.GetByID(ctx, pt.ID)
	s.Require().NoError(err)
	s.Equal(pt.ID, got.ID)
	s.Equal(bizID, got.BusinessID)
	s.Equal(userID, got.InitiatedBy)
	s.Equal(int64(100000), got.AmountCents)
}

func (s *PendingTransferSuite) TestGetByID_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetByID(ctx, uuid.NewString())
	s.ErrorIs(err, domain.ErrPendingTransferNotFound)
}

func TestPendingTransferSuite(t *testing.T) {
	suite.Run(t, new(PendingTransferSuite))
}
