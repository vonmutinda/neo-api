package admin_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	admin "github.com/vonmutinda/neo/internal/services/admin"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

type AdminTransactionSuite struct {
	suite.Suite
	pool        *pgxpool.Pool
	adminRepo   repository.AdminQueryRepository
	receiptRepo repository.TransactionReceiptRepository
	auditRepo   repository.AuditRepository
	svc         *admin.TransactionService
}

func (s *AdminTransactionSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.adminRepo = repository.NewAdminQueryRepository(s.pool)
	s.receiptRepo = repository.NewTransactionReceiptRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)

	s.svc = admin.NewTransactionService(s.adminRepo, s.receiptRepo, s.auditRepo)
}

func (s *AdminTransactionSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *AdminTransactionSuite) seedReceipt(userID string, status domain.ReceiptStatus) string {
	var id string
	err := s.pool.QueryRow(context.Background(),
		`INSERT INTO transaction_receipts (user_id, type, status, amount_cents, currency, ledger_transaction_id)
		 VALUES ($1, 'p2p_send', $2, 10000, 'ETB', $3) RETURNING id`,
		userID, status, "ledger:"+uuid.NewString(),
	).Scan(&id)
	s.Require().NoError(err)
	return id
}

func (s *AdminTransactionSuite) TestGetByID_Success() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345678"))
	receiptID := s.seedReceipt(user.ID, domain.ReceiptCompleted)

	receipt, err := s.svc.GetByID(context.Background(), receiptID)
	s.Require().NoError(err)
	s.Equal(receiptID, receipt.ID)
	s.Equal(user.ID, receipt.UserID)
	s.Equal(domain.ReceiptCompleted, receipt.Status)
}

func (s *AdminTransactionSuite) TestGetByID_NotFound() {
	_, err := s.svc.GetByID(context.Background(), uuid.NewString())
	s.Require().Error(err)
}

func (s *AdminTransactionSuite) TestReverse_Success() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345678"))
	receiptID := s.seedReceipt(user.ID, domain.ReceiptCompleted)

	staffID := uuid.NewString()
	err := s.svc.Reverse(context.Background(), staffID, receiptID, admin.ReverseRequest{Reason: "test"})
	s.Require().NoError(err)

	receipt, err := s.receiptRepo.GetByID(context.Background(), receiptID)
	s.Require().NoError(err)
	s.Equal(domain.ReceiptReversed, receipt.Status)
}

func (s *AdminTransactionSuite) TestReverse_NotCompleted() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345678"))
	receiptID := s.seedReceipt(user.ID, domain.ReceiptPending)

	staffID := uuid.NewString()
	err := s.svc.Reverse(context.Background(), staffID, receiptID, admin.ReverseRequest{Reason: "test"})
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrInvalidInput)
}

func (s *AdminTransactionSuite) seedConversionPair(userID string) (outID, inID string) {
	ledgerTxID := "ledger:" + uuid.NewString()
	ik := uuid.NewString()

	err := s.pool.QueryRow(context.Background(),
		`INSERT INTO transaction_receipts (user_id, type, status, amount_cents, currency, ledger_transaction_id, idempotency_key, narration)
		 VALUES ($1, 'convert_out', 'completed', 50000, 'ETB', $2, $3, 'Converted 500.00 ETB to 10.00 USD') RETURNING id`,
		userID, ledgerTxID, ik,
	).Scan(&outID)
	s.Require().NoError(err)

	err = s.pool.QueryRow(context.Background(),
		`INSERT INTO transaction_receipts (user_id, type, status, amount_cents, currency, ledger_transaction_id, idempotency_key, narration)
		 VALUES ($1, 'convert_in', 'completed', 1000, 'USD', $2, $3, 'Converted 500.00 ETB to 10.00 USD') RETURNING id`,
		userID, ledgerTxID, ik,
	).Scan(&inID)
	s.Require().NoError(err)
	return
}

func (s *AdminTransactionSuite) TestGetConversion_FromConvertOut() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345678"))
	outID, _ := s.seedConversionPair(user.ID)

	view, err := s.svc.GetConversion(context.Background(), outID)
	s.Require().NoError(err)
	s.Equal("ETB", view.FromCurrency)
	s.Equal("USD", view.ToCurrency)
	s.Equal(int64(50000), view.FromAmountCents)
	s.Equal(int64(1000), view.ToAmountCents)
}

func (s *AdminTransactionSuite) TestGetConversion_FromConvertIn() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345678"))
	_, inID := s.seedConversionPair(user.ID)

	view, err := s.svc.GetConversion(context.Background(), inID)
	s.Require().NoError(err)
	s.Equal("ETB", view.FromCurrency)
	s.Equal("USD", view.ToCurrency)
	s.Equal(int64(50000), view.FromAmountCents)
	s.Equal(int64(1000), view.ToAmountCents)
}

func (s *AdminTransactionSuite) TestGetConversion_NonConvertReceipt() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345678"))
	receiptID := s.seedReceipt(user.ID, domain.ReceiptCompleted)

	_, err := s.svc.GetConversion(context.Background(), receiptID)
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrInvalidInput)
}

func (s *AdminTransactionSuite) TestList_EnrichesConvertOut() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345678"))
	s.seedConversionPair(user.ID)

	convertOut := string(domain.ReceiptConvertOut)
	result, err := s.svc.List(context.Background(), domain.TransactionFilter{
		UserID: &user.ID,
		Type:   &convertOut,
		Limit:  10,
	})
	s.Require().NoError(err)
	s.Require().Len(result.Data, 1)
	s.NotNil(result.Data[0].ConvertedCurrency)
	s.Equal("USD", *result.Data[0].ConvertedCurrency)
	s.NotNil(result.Data[0].ConvertedAmountCents)
	s.Equal(int64(1000), *result.Data[0].ConvertedAmountCents)
}

func (s *AdminTransactionSuite) TestList_HidesConvertIn() {
	user := testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251912345678"))
	s.seedReceipt(user.ID, domain.ReceiptCompleted)
	s.seedConversionPair(user.ID)

	result, err := s.svc.List(context.Background(), domain.TransactionFilter{
		UserID: &user.ID,
		Limit:  10,
	})
	s.Require().NoError(err)

	var types []domain.ReceiptType
	for _, v := range result.Data {
		types = append(types, v.Type)
	}
	s.Contains(types, domain.ReceiptP2PSend)
	s.Contains(types, domain.ReceiptConvertOut)
	s.NotContains(types, domain.ReceiptConvertIn)
	s.Equal(int64(2), result.Pagination.Total)
}

func TestAdminTransactionSuite(t *testing.T) {
	suite.Run(t, new(AdminTransactionSuite))
}
