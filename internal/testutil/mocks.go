package testutil

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/gateway/ethswitch"
	"github.com/vonmutinda/neo/internal/gateway/fayda"
	tgclient "github.com/vonmutinda/neo/internal/gateway/telegram"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/services/convert"
	"github.com/vonmutinda/neo/pkg/cache"
)

// ---- Mock Ledger Client ----

type MockLedgerClient struct {
	mu       sync.Mutex
	Balances map[string]int64
	HoldIDs  int
}

func NewMockLedgerClient() *MockLedgerClient {
	return &MockLedgerClient{Balances: make(map[string]int64)}
}

func (m *MockLedgerClient) EnsureLedgerExists(_ context.Context) error { return nil }

func (m *MockLedgerClient) CreateWallet(_ context.Context, walletID, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Balances[walletID] = 0
	return nil
}

func (m *MockLedgerClient) GetWalletBalance(_ context.Context, walletID, asset string) (*big.Int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if asset != "" {
		if b, ok := m.Balances[walletID+":"+asset]; ok {
			return big.NewInt(b), nil
		}
	}
	bal := m.Balances[walletID]
	return big.NewInt(bal), nil
}

func (m *MockLedgerClient) GetMultiCurrencyBalances(_ context.Context, walletID string, assets []string) (map[string]*big.Int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string]*big.Int, len(assets))
	for _, asset := range assets {
		key := walletID + ":" + asset
		bal, ok := m.Balances[key]
		if !ok {
			bal = m.Balances[walletID]
		}
		result[asset] = big.NewInt(bal)
	}
	return result, nil
}

func (m *MockLedgerClient) CreditWallet(_ context.Context, _, walletID string, amountCents int64, _, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Balances[walletID] += amountCents
	return nil
}

func (m *MockLedgerClient) DebitWallet(_ context.Context, _, walletID string, amountCents int64, _, _ string, _ bool) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Balances[walletID] < amountCents {
		return "", domain.ErrInsufficientFunds
	}
	m.Balances[walletID] -= amountCents
	return "hold-1", nil
}

func (m *MockLedgerClient) HoldFunds(_ context.Context, _, _ string, _ string, amountCents int64, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.HoldIDs++
	return "hold-" + string(rune('0'+m.HoldIDs)), nil
}

func (m *MockLedgerClient) SettleHold(_ context.Context, _, _ string) error { return nil }
func (m *MockLedgerClient) VoidHold(_ context.Context, _, _ string) error   { return nil }

func (m *MockLedgerClient) DisburseLoan(_ context.Context, _, walletID string, principalCents int64, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Balances[walletID] += principalCents
	return "tx-loan-1", nil
}

func (m *MockLedgerClient) CollectLoanRepayment(_ context.Context, _, walletID string, principalCents, feeCents int64, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := principalCents + feeCents
	if m.Balances[walletID] < total {
		return domain.ErrInsufficientFunds
	}
	m.Balances[walletID] -= total
	return nil
}

func (m *MockLedgerClient) CreditFromOverdraft(_ context.Context, _, walletID string, amountCents int64, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Balances[walletID] += amountCents
	return "tx-overdraft-credit-1", nil
}

func (m *MockLedgerClient) DebitToOverdraft(_ context.Context, _, walletID string, amountCents int64, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Balances[walletID] < amountCents {
		return domain.ErrInsufficientFunds
	}
	m.Balances[walletID] -= amountCents
	return nil
}

func (m *MockLedgerClient) ConvertCurrency(_ context.Context, _, walletID string, fromCents int64, fromAsset string, toCents int64, _ string, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Balances[walletID] < fromCents {
		return "", fmt.Errorf("insufficient funds")
	}
	m.Balances[walletID] -= fromCents
	m.Balances[walletID+"_"+fromAsset] -= fromCents
	m.Balances[walletID+"_to"] += toCents
	return "tx-convert-1", nil
}

func (m *MockLedgerClient) GetAccountBalance(_ context.Context, _, _ string) (*big.Int, error) {
	return big.NewInt(0), nil
}

func (m *MockLedgerClient) GetAccountHistory(_ context.Context, _ string, _ int) ([]ledger.Transaction, error) {
	return []ledger.Transaction{}, nil
}

func (m *MockLedgerClient) TransferToPot(_ context.Context, _, walletID, potID string, amountCents int64, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Balances[walletID] < amountCents {
		return domain.ErrInsufficientFunds
	}
	m.Balances[walletID] -= amountCents
	m.Balances["pot:"+potID] += amountCents
	return nil
}

func (m *MockLedgerClient) TransferFromPot(_ context.Context, _, walletID, potID string, amountCents int64, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	potKey := "pot:" + potID
	if m.Balances[potKey] < amountCents {
		return domain.ErrInsufficientFunds
	}
	m.Balances[potKey] -= amountCents
	m.Balances[walletID] += amountCents
	return nil
}

func (m *MockLedgerClient) GetPotBalance(_ context.Context, _, potID, _ string) (*big.Int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return big.NewInt(m.Balances["pot:"+potID]), nil
}

func (m *MockLedgerClient) DebitWalletWithFee(_ context.Context, _ string, walletID string,
	principalCents int64, feeCents int64, asset string,
	_ string, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := walletID + ":" + asset
	m.Balances[key] -= principalCents + feeCents
	return "", nil
}

var _ ledger.Client = (*MockLedgerClient)(nil)

// ---- Mock Fayda Client ----

type MockFaydaClient struct {
	ShouldFail bool
}

func NewMockFaydaClient() *MockFaydaClient { return &MockFaydaClient{} }

func (m *MockFaydaClient) RequestOTP(_ context.Context, _, _ string) error {
	if m.ShouldFail {
		return domain.ErrFaydaUnavailable
	}
	return nil
}

func (m *MockFaydaClient) VerifyAndFetchKYC(_ context.Context, _, _, _ string) (*fayda.KYCResponse, error) {
	if m.ShouldFail {
		return nil, domain.ErrFaydaAuthFailed
	}
	resp := &fayda.KYCResponse{Status: "y", TransactionID: "tx-123"}
	resp.Identity.FullName = "Abebe Bikila"
	resp.Identity.DOB = "1990-01-01"
	resp.Identity.Gender = "Male"
	return resp, nil
}

var _ fayda.Client = (*MockFaydaClient)(nil)

// ---- Mock EthSwitch Client ----

type MockEthSwitchClient struct {
	ShouldFail bool
}

func NewMockEthSwitchClient() *MockEthSwitchClient { return &MockEthSwitchClient{} }

func (m *MockEthSwitchClient) InitiateTransfer(_ context.Context, req ethswitch.TransferRequest) (*ethswitch.TransferResponse, error) {
	if m.ShouldFail {
		return nil, domain.ErrEthSwitchTimeout
	}
	return &ethswitch.TransferResponse{
		EthSwitchReference: "ES-" + req.IdempotencyKey,
		Status:             "SUCCESS",
	}, nil
}

func (m *MockEthSwitchClient) CheckTransactionStatus(_ context.Context, ik string) (*ethswitch.StatusCheckResponse, error) {
	return &ethswitch.StatusCheckResponse{
		EthSwitchReference: "ES-" + ik,
		Status:             "SUCCESS",
	}, nil
}

var _ ethswitch.Client = (*MockEthSwitchClient)(nil)

// ---- Mock Telegram Client ----

type MockTelegramClient struct {
	mu       sync.Mutex
	Messages []SentMessage
}

type SentMessage struct {
	ChatID int64
	Text   string
}

func NewMockTelegramClient() *MockTelegramClient { return &MockTelegramClient{} }

func (m *MockTelegramClient) SendMessage(_ context.Context, chatID int64, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages = append(m.Messages, SentMessage{ChatID: chatID, Text: text})
	return nil
}

func (m *MockTelegramClient) SetWebhook(_ context.Context, _ string) error { return nil }

func (m *MockTelegramClient) LastMessage() *SentMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Messages) == 0 {
		return nil
	}
	return &m.Messages[len(m.Messages)-1]
}

var _ tgclient.Client = (*MockTelegramClient)(nil)

// ---- Mock Rate Provider ----

type MockRateProvider struct {
	mu    sync.Mutex
	Rates map[string]convert.Rate
}

func NewMockRateProvider() *MockRateProvider {
	return &MockRateProvider{Rates: make(map[string]convert.Rate)}
}

func (m *MockRateProvider) GetRate(_ context.Context, from, to string) (convert.Rate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := from + "_" + to
	if r, ok := m.Rates[key]; ok {
		return r, nil
	}
	return convert.Rate{}, domain.ErrInvalidCurrency
}

var _ convert.RateProvider = (*MockRateProvider)(nil)

// ---- Mock Cache ----

type MockCache struct {
	mu       sync.Mutex
	Data     map[string][]byte
	Counters map[string]int64
}

func NewMockCache() *MockCache {
	return &MockCache{
		Data:     make(map[string][]byte),
		Counters: make(map[string]int64),
	}
}

func (m *MockCache) Get(_ context.Context, key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.Data[key]
	return v, ok
}

func (m *MockCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Data[key] = value
	return nil
}

func (m *MockCache) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Data, key)
	return nil
}

func (m *MockCache) DeleteByPrefix(_ context.Context, prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.Data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(m.Data, k)
		}
	}
	return nil
}

func (m *MockCache) Increment(_ context.Context, key string, _ time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Counters[key]++
	return m.Counters[key], nil
}

func (m *MockCache) Ping(_ context.Context) error { return nil }
func (m *MockCache) Close() error                 { return nil }

var _ cache.Cache = (*MockCache)(nil)
