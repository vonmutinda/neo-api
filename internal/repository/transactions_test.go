package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type TransactionReceiptSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	repo     repository.TransactionReceiptRepository
}

func (s *TransactionReceiptSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())
	s.repo = repository.NewTransactionReceiptRepository(s.pool)
}

func (s *TransactionReceiptSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *TransactionReceiptSuite) TestCreate_Success() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345678"))

	ref := "ES-123"
	rec := &domain.TransactionReceipt{
		UserID:              userID,
		LedgerTransactionID: "ledger-tx-1",
		EthSwitchReference:  &ref,
		Type:                domain.ReceiptEthSwitchOut,
		Status:              domain.ReceiptPending,
		AmountCents:         100000,
		Currency:            "ETB",
	}
	err := s.repo.Create(ctx, rec)
	s.Require().NoError(err)
	s.NotEmpty(rec.ID)

	got, err := s.repo.GetByID(ctx, rec.ID)
	s.Require().NoError(err)
	s.Equal(rec.ID, got.ID)
	s.Equal(userID, got.UserID)
	s.Equal("ledger-tx-1", got.LedgerTransactionID)
	s.Require().NotNil(got.EthSwitchReference)
	s.Equal("ES-123", *got.EthSwitchReference)
	s.Equal(domain.ReceiptEthSwitchOut, got.Type)
	s.Equal(domain.ReceiptPending, got.Status)
	s.Equal(int64(100000), got.AmountCents)
	s.Equal("ETB", got.Currency)
}

func (s *TransactionReceiptSuite) TestGetByID_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetByID(ctx, uuid.NewString())
	s.ErrorIs(err, domain.ErrNotFound)
}

func (s *TransactionReceiptSuite) TestGetByEthSwitchReference() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345679"))

	ref := "ES-456"
	rec := &domain.TransactionReceipt{
		UserID:              userID,
		LedgerTransactionID: "ledger-tx-2",
		EthSwitchReference:  &ref,
		Type:                domain.ReceiptEthSwitchOut,
		Status:              domain.ReceiptPending,
		AmountCents:         50000,
		Currency:            "ETB",
	}
	s.Require().NoError(s.repo.Create(ctx, rec))

	got, err := s.repo.GetByEthSwitchReference(ctx, "ES-456")
	s.Require().NoError(err)
	s.Equal(rec.ID, got.ID)
	s.Equal("ES-456", *got.EthSwitchReference)
}

func (s *TransactionReceiptSuite) TestGetByEthSwitchReference_NotFound() {
	ctx := context.Background()
	_, err := s.repo.GetByEthSwitchReference(ctx, "ES-NONEXISTENT")
	s.ErrorIs(err, domain.ErrNotFound)
}

func (s *TransactionReceiptSuite) TestListByUserID() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345680"))

	for i := 1; i <= 3; i++ {
		rec := &domain.TransactionReceipt{
			UserID:              userID,
			LedgerTransactionID: fmt.Sprintf("ledger-tx-%d", i),
			Type:                domain.ReceiptEthSwitchOut,
			Status:              domain.ReceiptPending,
			AmountCents:         int64(i * 10000),
			Currency:            "ETB",
		}
		s.Require().NoError(s.repo.Create(ctx, rec))
	}

	list, err := s.repo.ListByUserID(ctx, userID, 2, 0)
	s.Require().NoError(err)
	s.Len(list, 2)
}

func (s *TransactionReceiptSuite) TestUpdateStatus() {
	ctx := context.Background()
	userID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, userID, phone.MustParse("+251912345681"))

	rec := &domain.TransactionReceipt{
		UserID:              userID,
		LedgerTransactionID: "ledger-tx-pending",
		Type:                domain.ReceiptEthSwitchOut,
		Status:              domain.ReceiptPending,
		AmountCents:         100000,
		Currency:            "ETB",
	}
	s.Require().NoError(s.repo.Create(ctx, rec))

	err := s.repo.UpdateStatus(ctx, rec.ID, domain.ReceiptCompleted)
	s.Require().NoError(err)

	got, err := s.repo.GetByID(ctx, rec.ID)
	s.Require().NoError(err)
	s.Equal(domain.ReceiptCompleted, got.Status)
}

func TestTransactionReceiptSuite(t *testing.T) {
	suite.Run(t, new(TransactionReceiptSuite))
}
