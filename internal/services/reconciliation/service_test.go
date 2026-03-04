package reconciliation_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/reconciliation"
	"github.com/vonmutinda/neo/internal/testutil"
)

type ReconciliationSuite struct {
	suite.Suite
	pool        *pgxpool.Pool
	svc         *reconciliation.Service
	receiptRepo repository.TransactionReceiptRepository
	reconRepo   repository.ReconciliationRepository
	auditRepo   repository.AuditRepository
	mockLedger  *testutil.MockLedgerClient
}

func (s *ReconciliationSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.mockLedger = testutil.NewMockLedgerClient()
	s.receiptRepo = repository.NewTransactionReceiptRepository(s.pool)
	s.reconRepo = repository.NewReconciliationRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)

	s.svc = reconciliation.NewService(s.receiptRepo, s.reconRepo, s.auditRepo, s.mockLedger)
}

func (s *ReconciliationSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
}

func (s *ReconciliationSuite) TestRunDailyReconciliation_FileNotFound() {
	err := s.svc.RunDailyReconciliation(context.Background(), "/tmp/nonexistent-clearing-file-12345.csv")
	s.Require().Error(err)
	s.Contains(err.Error(), "opening clearing file")
}

func TestReconciliationSuite(t *testing.T) {
	suite.Run(t, new(ReconciliationSuite))
}
