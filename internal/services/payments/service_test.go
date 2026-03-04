package payments_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/overdraft"
	"github.com/vonmutinda/neo/internal/services/payments"
	"github.com/vonmutinda/neo/internal/testutil"
	"github.com/vonmutinda/neo/pkg/phone"
)

// mockOverdraftCover records UseCover calls and simulates crediting the wallet so DebitWallet succeeds.
type mockOverdraftCover struct {
	ledger         *testutil.MockLedgerClient
	called         bool
	shortfallCents int64
	userID         string
	walletID       string
}

func (m *mockOverdraftCover) UseCover(ctx context.Context, userID, walletID, idempotencyKey string, shortfallCents int64, asset string) error {
	m.called = true
	m.shortfallCents = shortfallCents
	m.userID = userID
	m.walletID = walletID
	m.ledger.Balances[walletID] += shortfallCents
	return nil
}

func (m *mockOverdraftCover) AutoRepayOnInflow(context.Context, string, string, string, int64) (int64, error) {
	return 0, nil
}

type PaymentsSuite struct {
	suite.Suite
	pool            *pgxpool.Pool
	userRepo        repository.UserRepository
	receiptRepo     repository.TransactionReceiptRepository
	auditRepo       repository.AuditRepository
	mockLedger      *testutil.MockLedgerClient
	mockEthSwitch   *testutil.MockEthSwitchClient
	svc             *payments.Service
}

func (s *PaymentsSuite) SetupSuite() {
	s.pool = testutil.MustStartPostgres(s.T())

	s.userRepo = repository.NewUserRepository(s.pool)
	s.receiptRepo = repository.NewTransactionReceiptRepository(s.pool)
	s.auditRepo = repository.NewAuditRepository(s.pool)
	s.mockLedger = testutil.NewMockLedgerClient()
	s.mockEthSwitch = testutil.NewMockEthSwitchClient()

	chart := ledger.NewChart("neo")
	s.svc = payments.NewService(
		s.userRepo, s.receiptRepo, s.auditRepo,
		s.mockLedger, s.mockEthSwitch, chart,
		nil, nil, nil, nil, nil,
	)
}

func (s *PaymentsSuite) TearDownTest() {
	testutil.TruncateAll(s.T(), s.pool)
	s.mockLedger.Balances = make(map[string]int64)
	s.mockEthSwitch.ShouldFail = false
}

// helpers ------------------------------------------------------------------

func (s *PaymentsSuite) countAuditActions(action domain.AuditAction) int {
	var n int
	err := s.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE action = $1`, string(action),
	).Scan(&n)
	s.Require().NoError(err)
	return n
}

// --- Outbound Transfer Tests ---

func (s *PaymentsSuite) TestProcessOutboundTransfer_Success() {
	ctx := context.Background()
	senderID := uuid.NewString()
	user := testutil.SeedUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"))

	s.mockLedger.Balances[user.LedgerWalletID] = 500000

	err := s.svc.ProcessOutboundTransfer(ctx, user.ID, &payments.OutboundTransferRequest{
		AmountCents: 100000, Currency: "ETB", DestPhone: phone.MustParse("+251911111111"), DestInstitution: "CBE", Narration: "test payment",
	})
	s.Require().NoError(err)

	receipts, err := s.receiptRepo.ListByUserID(ctx, user.ID, 10, 0)
	s.Require().NoError(err)
	s.Len(receipts, 1)

	s.Greater(s.countAuditActions(domain.AuditTransferInitiated), 0, "expected transfer_initiated audit entry")
	s.Greater(s.countAuditActions(domain.AuditTransferSettled), 0, "expected transfer_settled audit entry")
}

func (s *PaymentsSuite) TestProcessOutboundTransfer_USD() {
	ctx := context.Background()
	senderID := uuid.NewString()
	user := testutil.SeedUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"))

	s.mockLedger.Balances[user.LedgerWalletID] = 50000

	err := s.svc.ProcessOutboundTransfer(ctx, user.ID, &payments.OutboundTransferRequest{
		AmountCents: 10000, Currency: "USD", DestPhone: phone.MustParse("+251911111111"), DestInstitution: "CBE", Narration: "usd transfer",
	})
	s.Require().NoError(err)

	receipts, err := s.receiptRepo.ListByUserID(ctx, user.ID, 10, 0)
	s.Require().NoError(err)
	s.Len(receipts, 1)
	s.Equal("USD", receipts[0].Currency)
}

func (s *PaymentsSuite) TestProcessOutboundTransfer_InvalidCurrency() {
	ctx := context.Background()
	senderID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"))

	err := s.svc.ProcessOutboundTransfer(ctx, senderID, &payments.OutboundTransferRequest{
		AmountCents: 100000, Currency: "GBP", DestPhone: phone.MustParse("+251911111111"), DestInstitution: "CBE", Narration: "test",
	})
	s.Error(err)
}

func (s *PaymentsSuite) TestProcessOutboundTransfer_UserFrozen() {
	ctx := context.Background()
	senderID := uuid.NewString()
	user := testutil.SeedFrozenUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"), "AML hold")

	err := s.svc.ProcessOutboundTransfer(ctx, user.ID, &payments.OutboundTransferRequest{
		AmountCents: 100000, Currency: "ETB", DestPhone: phone.MustParse("+251911111111"), DestInstitution: "CBE", Narration: "test",
	})
	s.True(errors.Is(err, domain.ErrUserFrozen))
}

func (s *PaymentsSuite) TestProcessOutboundTransfer_EthSwitchFailure_VoidsHold() {
	ctx := context.Background()
	senderID := uuid.NewString()
	user := testutil.SeedUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"))

	s.mockLedger.Balances[user.LedgerWalletID] = 500000
	s.mockEthSwitch.ShouldFail = true

	err := s.svc.ProcessOutboundTransfer(ctx, user.ID, &payments.OutboundTransferRequest{
		AmountCents: 100000, Currency: "ETB", DestPhone: phone.MustParse("+251911111111"), DestInstitution: "CBE", Narration: "test",
	})
	s.Error(err)

	s.Greater(s.countAuditActions(domain.AuditTransferFailed), 0, "expected transfer_failed audit entry")
}

func (s *PaymentsSuite) TestProcessOutboundTransfer_InvalidAmount() {
	ctx := context.Background()
	senderID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"))

	err := s.svc.ProcessOutboundTransfer(ctx, senderID, &payments.OutboundTransferRequest{
		AmountCents: 0, Currency: "ETB", DestPhone: phone.MustParse("+251911111111"), DestInstitution: "CBE", Narration: "test",
	})
	s.Error(err)

	err = s.svc.ProcessOutboundTransfer(ctx, senderID, &payments.OutboundTransferRequest{
		AmountCents: -100, Currency: "ETB", DestPhone: phone.MustParse("+251911111111"), DestInstitution: "CBE", Narration: "test",
	})
	s.Error(err)
}

// --- Inbound P2P Transfer Tests ---

func (s *PaymentsSuite) TestProcessInboundTransfer_Success() {
	ctx := context.Background()
	senderID := uuid.NewString()
	recipientID := uuid.NewString()
	sender := testutil.SeedUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"))
	recipient := testutil.SeedUser(s.T(), s.pool, recipientID, phone.MustParse("+251911111111"))

	s.mockLedger.Balances[sender.LedgerWalletID] = 500000

	err := s.svc.ProcessInboundTransfer(ctx, sender.ID, &payments.InboundTransferRequest{
		RecipientPhone: recipient.PhoneNumber, AmountCents: 100000, Currency: "ETB", Narration: "lunch money",
	})
	s.Require().NoError(err)

	senderReceipts, err := s.receiptRepo.ListByUserID(ctx, sender.ID, 10, 0)
	s.Require().NoError(err)
	recipientReceipts, err := s.receiptRepo.ListByUserID(ctx, recipient.ID, 10, 0)
	s.Require().NoError(err)

	s.Len(senderReceipts, 1)
	s.Len(recipientReceipts, 1)
	s.Equal(domain.ReceiptP2PSend, senderReceipts[0].Type)
	s.Equal(domain.ReceiptP2PReceive, recipientReceipts[0].Type)

	s.Greater(s.countAuditActions(domain.AuditP2PTransfer), 0, "expected p2p_transfer audit entry")
}

func (s *PaymentsSuite) TestProcessInboundTransfer_SenderFrozen() {
	ctx := context.Background()
	senderID := uuid.NewString()
	recipientID := uuid.NewString()
	sender := testutil.SeedFrozenUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"), "AML hold")
	testutil.SeedUser(s.T(), s.pool, recipientID, phone.MustParse("+251911111111"))

	err := s.svc.ProcessInboundTransfer(ctx, sender.ID, &payments.InboundTransferRequest{
		RecipientPhone: phone.MustParse("+251911111111"), AmountCents: 100000, Currency: "ETB", Narration: "test",
	})
	s.True(errors.Is(err, domain.ErrUserFrozen))
}

func (s *PaymentsSuite) TestProcessInboundTransfer_RecipientFrozen() {
	ctx := context.Background()
	senderID := uuid.NewString()
	recipientID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"))
	recipient := testutil.SeedFrozenUser(s.T(), s.pool, recipientID, phone.MustParse("+251911111111"), "AML hold")

	err := s.svc.ProcessInboundTransfer(ctx, senderID, &payments.InboundTransferRequest{
		RecipientPhone: recipient.PhoneNumber, AmountCents: 100000, Currency: "ETB", Narration: "test",
	})
	s.True(errors.Is(err, domain.ErrUserFrozen))
}

func (s *PaymentsSuite) TestProcessInboundTransfer_RecipientNotFound() {
	ctx := context.Background()
	senderID := uuid.NewString()
	testutil.SeedUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"))

	err := s.svc.ProcessInboundTransfer(ctx, senderID, &payments.InboundTransferRequest{
		RecipientPhone: phone.MustParse("+251900000000"), AmountCents: 100000, Currency: "ETB", Narration: "test",
	})
	s.True(errors.Is(err, domain.ErrUserNotFound))
}

func (s *PaymentsSuite) TestProcessInboundTransfer_SelfTransfer() {
	ctx := context.Background()
	senderID := uuid.NewString()
	sender := testutil.SeedUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"))

	err := s.svc.ProcessInboundTransfer(ctx, sender.ID, &payments.InboundTransferRequest{
		RecipientPhone: sender.PhoneNumber, AmountCents: 100000, Currency: "ETB", Narration: "test",
	})
	s.True(errors.Is(err, domain.ErrInvalidInput))
}

func (s *PaymentsSuite) TestProcessInboundTransfer_InsufficientFunds() {
	ctx := context.Background()
	senderID := uuid.NewString()
	recipientID := uuid.NewString()
	sender := testutil.SeedUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, recipientID, phone.MustParse("+251911111111"))

	s.mockLedger.Balances[sender.LedgerWalletID] = 50

	err := s.svc.ProcessInboundTransfer(ctx, sender.ID, &payments.InboundTransferRequest{
		RecipientPhone: phone.MustParse("+251911111111"), AmountCents: 100000, Currency: "ETB", Narration: "test",
	})
	s.True(errors.Is(err, domain.ErrInsufficientFunds))
}

// failingOverdraftCover implements OverdraftCover and returns a fixed error from UseCover.
type failingOverdraftCover struct{ err error }

func (f *failingOverdraftCover) UseCover(context.Context, string, string, string, int64, string) error { return f.err }
func (f *failingOverdraftCover) AutoRepayOnInflow(context.Context, string, string, string, int64) (int64, error) {
	return 0, nil
}

// TestProcessInboundTransfer_ETBInsufficientBalance_OverdraftCover verifies that when balance is insufficient
// for an ETB transfer and overdraft is available, UseCover is called and the transfer succeeds.
func (s *PaymentsSuite) TestProcessInboundTransfer_ETBInsufficientBalance_OverdraftCover() {
	ctx := context.Background()
	senderID := uuid.NewString()
	recipientID := uuid.NewString()
	sender := testutil.SeedUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"))
	recipient := testutil.SeedUser(s.T(), s.pool, recipientID, phone.MustParse("+251911111111"))

	s.mockLedger.Balances[sender.LedgerWalletID] = 50
	transferAmount := int64(100000)
	shortfall := transferAmount - 50

	mockOverdraft := &mockOverdraftCover{ledger: s.mockLedger}
	chart := ledger.NewChart("neo")
	svcWithOverdraft := payments.NewService(
		s.userRepo, s.receiptRepo, s.auditRepo,
		s.mockLedger, s.mockEthSwitch, chart,
		nil, nil, nil, nil, mockOverdraft,
	)

	err := svcWithOverdraft.ProcessInboundTransfer(ctx, sender.ID, &payments.InboundTransferRequest{
		RecipientPhone: recipient.PhoneNumber, AmountCents: transferAmount, Currency: "ETB", Narration: "overdraft test",
	})
	s.Require().NoError(err)

	s.True(mockOverdraft.called, "UseCover should have been called")
	s.Equal(sender.ID, mockOverdraft.userID)
	s.Equal(sender.LedgerWalletID, mockOverdraft.walletID)
	s.Equal(shortfall, mockOverdraft.shortfallCents)

	receipts, err := s.receiptRepo.ListByUserID(ctx, sender.ID, 10, 0)
	s.Require().NoError(err)
	s.Len(receipts, 1)
	s.Equal(transferAmount, receipts[0].AmountCents)
	s.Equal("ETB", receipts[0].Currency)
}

// TestProcessInboundTransfer_OverdraftLimitExceeded_ReturnsSpecificError verifies that when overdraft
// returns ErrOverdraftLimitExceeded, the payment service returns that error (not generic ErrInsufficientFunds).
func (s *PaymentsSuite) TestProcessInboundTransfer_OverdraftLimitExceeded_ReturnsSpecificError() {
	ctx := context.Background()
	senderID := uuid.NewString()
	sender := testutil.SeedUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, uuid.NewString(), phone.MustParse("+251911111111"))

	s.mockLedger.Balances[sender.LedgerWalletID] = 50
	failingOverdraft := &failingOverdraftCover{err: domain.ErrOverdraftLimitExceeded}
	chart := ledger.NewChart("neo")
	svc := payments.NewService(
		s.userRepo, s.receiptRepo, s.auditRepo,
		s.mockLedger, s.mockEthSwitch, chart,
		nil, nil, nil, nil, failingOverdraft,
	)

	err := svc.ProcessInboundTransfer(ctx, sender.ID, &payments.InboundTransferRequest{
		RecipientPhone: phone.MustParse("+251911111111"), AmountCents: 100000, Currency: "ETB", Narration: "test",
	})
	s.Require().Error(err)
	s.True(errors.Is(err, domain.ErrOverdraftLimitExceeded), "should return overdraft limit exceeded, got %v", err)
	s.False(errors.Is(err, domain.ErrInsufficientFunds), "should not be generic insufficient funds")
}

func (s *PaymentsSuite) TestProcessInboundTransfer_MultiCurrency_USD() {
	ctx := context.Background()
	senderID := uuid.NewString()
	recipientID := uuid.NewString()
	sender := testutil.SeedUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"))
	testutil.SeedUser(s.T(), s.pool, recipientID, phone.MustParse("+251911111111"))

	s.mockLedger.Balances[sender.LedgerWalletID] = 50000

	err := s.svc.ProcessInboundTransfer(ctx, sender.ID, &payments.InboundTransferRequest{
		RecipientPhone: phone.MustParse("+251911111111"), AmountCents: 5000, Currency: "USD", Narration: "dollar transfer",
	})
	s.Require().NoError(err)

	receipts, err := s.receiptRepo.ListByUserID(ctx, sender.ID, 10, 0)
	s.Require().NoError(err)
	s.Len(receipts, 1)
	s.Equal("USD", receipts[0].Currency)
}

// creditingLedger credits the destination wallet on DebitWallet so recipient has balance for AutoRepayOnInflow.
type creditingLedger struct {
	*testutil.MockLedgerClient
	chart         *ledger.Chart
	destWalletID  string
}

func (c *creditingLedger) DebitWallet(ctx context.Context, ik, walletID string, amountCents int64, asset, destination string, pending bool) (string, error) {
	holdID, err := c.MockLedgerClient.DebitWallet(ctx, ik, walletID, amountCents, asset, destination, pending)
	if err != nil {
		return holdID, err
	}
	if c.destWalletID != "" && destination == c.chart.MainAccount(c.destWalletID) {
		c.MockLedgerClient.Balances[c.destWalletID] += amountCents
	}
	return holdID, nil
}

// TestProcessInboundTransfer_ETB_AutoRepayOnInflow uses real repos and overdraft service; asserts overdraft repo state after transfer.
func (s *PaymentsSuite) TestProcessInboundTransfer_ETB_AutoRepayOnInflow() {
	ctx := context.Background()
	senderID := uuid.NewString()
	recipientID := uuid.NewString()
	sender := testutil.SeedUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"))
	recipient := testutil.SeedUser(s.T(), s.pool, recipientID, phone.MustParse("+251911111111"))

	testutil.SeedCreditProfile(s.T(), s.pool, recipient.ID, 700, 1000000)
	overdraftRepo := repository.NewOverdraftRepository(s.pool)
	loanRepo := repository.NewLoanRepository(s.pool)
	overdraftSvc := overdraft.NewService(overdraftRepo, loanRepo, s.userRepo, s.mockLedger, s.auditRepo)
	_, err := overdraftSvc.OptIn(ctx, recipient.ID)
	s.Require().NoError(err)
	_, err = s.pool.Exec(ctx, `UPDATE overdrafts SET used_cents = 5000, status = 'used' WHERE user_id = $1`, recipient.ID)
	s.Require().NoError(err)

	chart := ledger.NewChart("neo")
	ledgerWithCredit := &creditingLedger{MockLedgerClient: s.mockLedger, chart: chart, destWalletID: recipient.LedgerWalletID}
	s.mockLedger.Balances[sender.LedgerWalletID] = 10000
	overdraftSvcReal := overdraft.NewService(overdraftRepo, loanRepo, s.userRepo, ledgerWithCredit, s.auditRepo)
	svc := payments.NewService(s.userRepo, s.receiptRepo, s.auditRepo, ledgerWithCredit, s.mockEthSwitch, chart, nil, nil, nil, nil, overdraftSvcReal)

	err = svc.ProcessInboundTransfer(ctx, sender.ID, &payments.InboundTransferRequest{
		RecipientPhone: recipient.PhoneNumber, AmountCents: 10000, Currency: "ETB", Narration: "repay test",
	})
	s.Require().NoError(err)

	o, err := overdraftRepo.GetByUser(ctx, recipient.ID)
	s.Require().NoError(err)
	s.Require().NotNil(o)
	s.Equal(int64(0), o.UsedCents, "overdraft used_cents should be fully repaid")
	s.Equal(domain.OverdraftActive, o.Status)

	// Recipient's receive receipt should have overdraft metadata (10000 in, 5000 repaid, 5000 net)
	recs, err := s.receiptRepo.ListByUserID(ctx, recipient.ID, 10, 0)
	s.Require().NoError(err)
	var recvReceipt *domain.TransactionReceipt
	for i := range recs {
		if recs[i].Type == domain.ReceiptP2PReceive {
			recvReceipt = &recs[i]
			break
		}
	}
	s.Require().NotNil(recvReceipt, "recipient should have a p2p_receive receipt")
	s.Require().NotNil(recvReceipt.Metadata, "receipt should have overdraft metadata")
	var meta domain.InflowOverdraftMetadata
	err = json.Unmarshal(*recvReceipt.Metadata, &meta)
	s.Require().NoError(err)
	s.Equal(int64(10000), meta.TotalInflowCents)
	s.Equal(int64(5000), meta.OverdraftRepaymentCents)
	s.Equal(int64(5000), meta.NetInflowCents)
}

// TestProcessBatchTransfer_ETB_AutoRepayOnInflow uses real repos and overdraft service; one recipient has overdraft, assert it is reduced.
func (s *PaymentsSuite) TestProcessBatchTransfer_ETB_AutoRepayOnInflow() {
	ctx := context.Background()
	senderID := uuid.NewString()
	recipient1ID := uuid.NewString()
	recipient2ID := uuid.NewString()
	sender := testutil.SeedUser(s.T(), s.pool, senderID, phone.MustParse("+251912345678"))
	recipient1 := testutil.SeedUser(s.T(), s.pool, recipient1ID, phone.MustParse("+251911111111"))
	recipient2 := testutil.SeedUser(s.T(), s.pool, recipient2ID, phone.MustParse("+251922222222"))

	testutil.SeedCreditProfile(s.T(), s.pool, recipient1.ID, 700, 1000000)
	overdraftRepo := repository.NewOverdraftRepository(s.pool)
	loanRepo := repository.NewLoanRepository(s.pool)
	overdraftSvc := overdraft.NewService(overdraftRepo, loanRepo, s.userRepo, s.mockLedger, s.auditRepo)
	_, err := overdraftSvc.OptIn(ctx, recipient1.ID)
	s.Require().NoError(err)
	_, err = s.pool.Exec(ctx, `UPDATE overdrafts SET used_cents = 3000, status = 'used' WHERE user_id = $1`, recipient1.ID)
	s.Require().NoError(err)

	chart := ledger.NewChart("neo")
	dests := []string{recipient1.LedgerWalletID, recipient2.LedgerWalletID}
	creditingBatchLedger := &batchCreditingLedger{MockLedgerClient: s.mockLedger, chart: chart, destWalletIDs: dests}
	s.mockLedger.Balances[sender.LedgerWalletID] = 10000
	overdraftSvcReal := overdraft.NewService(overdraftRepo, loanRepo, s.userRepo, creditingBatchLedger, s.auditRepo)
	svc := payments.NewService(s.userRepo, s.receiptRepo, s.auditRepo, creditingBatchLedger, s.mockEthSwitch, chart, nil, nil, nil, nil, overdraftSvcReal)

	_, err = svc.ProcessBatchTransfer(ctx, sender.ID, &payments.BatchTransferRequest{
		Currency: "ETB",
		Items: []payments.BatchTransferItem{
			{Recipient: recipient1.PhoneNumber.E164(), AmountCents: 5000, Narration: "r1"},
			{Recipient: recipient2.PhoneNumber.E164(), AmountCents: 5000, Narration: "r2"},
		},
	})
	s.Require().NoError(err)

	o1, err := overdraftRepo.GetByUser(ctx, recipient1.ID)
	s.Require().NoError(err)
	s.Require().NotNil(o1)
	s.Equal(int64(0), o1.UsedCents, "recipient1 overdraft should be fully repaid")

	// Recipient1's receive receipt should have overdraft metadata (5000 in, 3000 repaid, 2000 net)
	recs, err := s.receiptRepo.ListByUserID(ctx, recipient1.ID, 10, 0)
	s.Require().NoError(err)
	var recvReceipt *domain.TransactionReceipt
	for i := range recs {
		if recs[i].Type == domain.ReceiptP2PReceive {
			recvReceipt = &recs[i]
			break
		}
	}
	s.Require().NotNil(recvReceipt, "recipient1 should have a p2p_receive receipt")
	s.Require().NotNil(recvReceipt.Metadata, "receipt should have overdraft metadata")
	var meta domain.InflowOverdraftMetadata
	err = json.Unmarshal(*recvReceipt.Metadata, &meta)
	s.Require().NoError(err)
	s.Equal(int64(5000), meta.TotalInflowCents)
	s.Equal(int64(3000), meta.OverdraftRepaymentCents)
	s.Equal(int64(2000), meta.NetInflowCents)
}

type batchCreditingLedger struct {
	*testutil.MockLedgerClient
	chart         *ledger.Chart
	destWalletIDs []string
}

func (b *batchCreditingLedger) DebitWallet(ctx context.Context, ik, walletID string, amountCents int64, asset, destination string, pending bool) (string, error) {
	holdID, err := b.MockLedgerClient.DebitWallet(ctx, ik, walletID, amountCents, asset, destination, pending)
	if err != nil {
		return holdID, err
	}
	for _, destID := range b.destWalletIDs {
		if destID != "" && destination == b.chart.MainAccount(destID) {
			b.MockLedgerClient.Balances[destID] += amountCents
			break
		}
	}
	return holdID, nil
}

func TestPaymentsSuite(t *testing.T) {
	suite.Run(t, new(PaymentsSuite))
}
