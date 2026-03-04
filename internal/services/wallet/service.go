package wallet

import (
	"context"
	"fmt"
	"strings"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/services/convert"
	"github.com/vonmutinda/neo/pkg/money"
)

type Service struct {
	users    repository.UserRepository
	receipts repository.TransactionReceiptRepository
	balances repository.CurrencyBalanceRepository
	ledger   ledger.Client
	chart    *ledger.Chart
	rates    convert.RateProvider
}

func NewService(
	users repository.UserRepository,
	receipts repository.TransactionReceiptRepository,
	balances repository.CurrencyBalanceRepository,
	ledgerClient ledger.Client,
	chart *ledger.Chart,
	rates convert.RateProvider,
) *Service {
	return &Service{
		users:    users,
		receipts: receipts,
		balances: balances,
		ledger:   ledgerClient,
		chart:    chart,
		rates:    rates,
	}
}

type BalanceView struct {
	WalletID     string `json:"walletId"`
	Currency     string `json:"currency"`
	Symbol       string `json:"symbol"`
	BalanceCents int64  `json:"balanceCents"`
	Display      string `json:"display"`
}

func (s *Service) GetBalance(ctx context.Context, userID, currencyCode string) (*BalanceView, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	if currencyCode == "" {
		currencyCode = money.CurrencyETB
	}
	currencyCode = strings.ToUpper(currencyCode)

	cur, err := money.LookupCurrency(currencyCode)
	if err != nil {
		return nil, err
	}

	asset := money.FormatAsset(cur.Code)
	balance, err := s.ledger.GetWalletBalance(ctx, user.LedgerWalletID, asset)
	if err != nil {
		return nil, fmt.Errorf("getting balance: %w", err)
	}

	return &BalanceView{
		WalletID:     user.LedgerWalletID,
		Currency:     cur.Code,
		Symbol:       cur.Symbol,
		BalanceCents: balance.Int64(),
		Display:      money.Display(balance.Int64(), cur.Code),
	}, nil
}

func (s *Service) GetSummary(ctx context.Context, userID string) (*money.AccountSummary, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	var assets []string
	var activeCurrencies []money.Currency

	activeBalances, _ := s.balances.ListActiveByUser(ctx, userID)
	if len(activeBalances) > 0 {
		for _, ab := range activeBalances {
			cur, err := money.LookupCurrency(ab.CurrencyCode)
			if err != nil {
				continue
			}
			activeCurrencies = append(activeCurrencies, cur)
			assets = append(assets, money.FormatAsset(ab.CurrencyCode))
		}
	} else {
		assets = money.AllAssets()
		activeCurrencies = money.SupportedCurrencies
	}

	balanceMap, err := s.ledger.GetMultiCurrencyBalances(ctx, user.LedgerWalletID, assets)
	if err != nil {
		return nil, fmt.Errorf("getting balances: %w", err)
	}

	balanceByCurrency := make(map[string]int64, len(balanceMap))
	for asset, bal := range balanceMap {
		code := strings.Split(asset, "/")[0]
		balanceByCurrency[code] = bal.Int64()
	}

	rates := make(map[string]float64, len(activeCurrencies))
	for _, cur := range activeCurrencies {
		if cur.Code == money.CurrencyETB {
			continue
		}
		rate, err := s.rates.GetRate(ctx, cur.Code, money.CurrencyETB)
		if err == nil {
			rates[cur.Code] = rate.Mid
		}
	}

	summary := money.BuildAccountSummary(user.LedgerWalletID, balanceByCurrency, rates, activeCurrencies)
	return &summary, nil
}

// TransactionView extends TransactionReceipt with optional conversion pair data
// so the UI can render FX conversions as a single row.
type TransactionView struct {
	domain.TransactionReceipt
	ConvertedCurrency    *string `json:"convertedCurrency,omitempty"`
	ConvertedAmountCents *int64  `json:"convertedAmountCents,omitempty"`
}

func (s *Service) ListTransactions(ctx context.Context, userID string, currency *string, limit, offset int) ([]TransactionView, error) {
	receipts, err := s.receipts.ListByUserIDFiltered(ctx, userID, currency, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing receipts: %w", err)
	}

	// Pass 1: index convert_out by ledger transaction ID and build a set of
	// convert_in ledger IDs that have a paired convert_out.
	convertOutByLedgerTx := make(map[string]int)     // ledgerTxID -> index in receipts
	pairedConvertIns := make(map[string]struct{})     // ledgerTxIDs where convert_in should be skipped

	for i, tx := range receipts {
		if tx.Type == domain.ReceiptConvertOut {
			convertOutByLedgerTx[tx.LedgerTransactionID] = i
		}
	}
	for _, tx := range receipts {
		if tx.Type == domain.ReceiptConvertIn {
			if _, hasPair := convertOutByLedgerTx[tx.LedgerTransactionID]; hasPair {
				pairedConvertIns[tx.LedgerTransactionID] = struct{}{}
			}
		}
	}

	// Pass 2: build output, skipping paired convert_in and enriching convert_out.
	views := make([]TransactionView, 0, len(receipts))
	for _, tx := range receipts {
		if tx.Type == domain.ReceiptConvertIn {
			if _, skip := pairedConvertIns[tx.LedgerTransactionID]; skip {
				continue
			}
		}

		v := TransactionView{TransactionReceipt: tx}

		if tx.Type == domain.ReceiptConvertOut {
			for _, other := range receipts {
				if other.Type == domain.ReceiptConvertIn && other.LedgerTransactionID == tx.LedgerTransactionID {
					cur := other.Currency
					amt := other.AmountCents
					v.ConvertedCurrency = &cur
					v.ConvertedAmountCents = &amt
					// Attach convert_in metadata (from/to + overdraft breakdown) so UI can show FX label and overdraft
					v.TransactionReceipt.Metadata = other.Metadata
					break
				}
			}
		}

		views = append(views, v)
	}

	return views, nil
}
