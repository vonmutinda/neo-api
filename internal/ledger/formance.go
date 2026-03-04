package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vonmutinda/neo/internal/config"
	sdk "github.com/formancehq/formance-sdk-go/v3"
	"github.com/formancehq/formance-sdk-go/v3/pkg/models/operations"
	"github.com/formancehq/formance-sdk-go/v3/pkg/models/sdkerrors"
	"github.com/formancehq/formance-sdk-go/v3/pkg/models/shared"
	"github.com/formancehq/go-libs/v3/pointer"
	"github.com/formancehq/go-libs/v3/query"
	"github.com/google/uuid"
)

// FormanceClient implements the Client interface using the Formance Go SDK.
// It communicates with Formance CE's Ledger v2 API.
type FormanceClient struct {
	sdk        *sdk.Formance
	ledgerName string
	chart      *Chart
	baseURL    string
	httpClient *http.Client
}

// NewFormanceClient creates a new Formance-backed ledger client.
// If httpClient is nil, a default client with a 15-second timeout is used.
func NewFormanceClient(cfg *config.Formance) *FormanceClient {
	return &FormanceClient{
		sdk:        initFormanceSDK(cfg),
		ledgerName: cfg.LedgerName,
		chart:      NewChart(cfg.AccountPrefix),
		baseURL:    cfg.URL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func initFormanceSDK(cfg *config.Formance) *sdk.Formance {
	if cfg.URL == "" {
		return nil
	}
	// The Formance Go SDK v3 targets the full Formance Stack and prepends
	// /api/ledger to all ledger paths. The standalone Formance Ledger CE
	// serves directly at /v2/..., so we use a custom transport that strips
	// the /api/ledger prefix before forwarding the request.
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &stripPrefixTransport{
			prefix: "/api/ledger",
			base:   http.DefaultTransport,
		},
	}
	return sdk.New(
		sdk.WithServerURL(cfg.URL),
		sdk.WithClient(client),
	)
}

type stripPrefixTransport struct {
	prefix string
	base   http.RoundTripper
}

func (t *stripPrefixTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.HasPrefix(req.URL.Path, t.prefix) {
		req2 := req.Clone(req.Context())
		req2.URL = new(url.URL)
		*req2.URL = *req.URL
		req2.URL.Path = strings.TrimPrefix(req.URL.Path, t.prefix)
		if req2.URL.RawPath != "" {
			req2.URL.RawPath = strings.TrimPrefix(req2.URL.RawPath, t.prefix)
		}
		return t.base.RoundTrip(req2)
	}
	return t.base.RoundTrip(req)
}

// --- Lifecycle ---

// EnsureLedgerExists uses direct HTTP against the standalone Formance Ledger
// v2 API (which serves at /v2/{ledger}), bypassing the SDK which targets the
// full Formance Stack path (/api/ledger/v2/{ledger}).
func (f *FormanceClient) EnsureLedgerExists(ctx context.Context) error {
	infoURL := fmt.Sprintf("%s/v2/%s/_info", f.baseURL, f.ledgerName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, infoURL, nil)
	if err != nil {
		return fmt.Errorf("building ledger info request: %w", err)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("checking ledger existence: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	createURL := fmt.Sprintf("%s/v2/%s", f.baseURL, f.ledgerName)
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, createURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("building create ledger request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("creating ledger: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("creating ledger: unexpected status %d", resp.StatusCode)
	}

	return nil
}

// --- Wallet Operations ---

func (f *FormanceClient) CreateWallet(ctx context.Context, walletID, name string) error {
	metadata := map[string]string{
		"wallets/spec/type": "wallets.primary",
		"wallets/name":      name,
		"wallets/id":        walletID,
		"wallets/balances":  "true",
	}

	_, err := f.sdk.Ledger.V2.AddMetadataToAccount(ctx, operations.V2AddMetadataToAccountRequest{
		RequestBody:    metadata,
		Address:        f.chart.MainAccount(walletID),
		Ledger:         f.ledgerName,
		IdempotencyKey: pointer.For("create-wallet-" + walletID),
	})
	if err != nil {
		return fmt.Errorf("creating wallet in ledger: %w", err)
	}

	return nil
}

func (f *FormanceClient) GetWalletBalance(ctx context.Context, walletID, asset string) (*big.Int, error) {
	resp, err := f.sdk.Ledger.V2.GetAccount(ctx, operations.V2GetAccountRequest{
		Address: f.chart.MainAccount(walletID),
		Ledger:  f.ledgerName,
		Expand:  pointer.For("volumes"),
	})
	if err != nil {
		if isFormanceNotFound(err) {
			return big.NewInt(0), nil
		}
		return nil, fmt.Errorf("getting wallet balance: %w", err)
	}

	volumes := resp.V2AccountResponse.Data.Volumes
	if volumes == nil {
		return big.NewInt(0), nil
	}

	vol, ok := volumes[asset]
	if !ok {
		return big.NewInt(0), nil
	}

	balance := new(big.Int).Sub(vol.Input, vol.Output)
	return balance, nil
}

func (f *FormanceClient) GetMultiCurrencyBalances(ctx context.Context, walletID string, assets []string) (map[string]*big.Int, error) {
	resp, err := f.sdk.Ledger.V2.GetAccount(ctx, operations.V2GetAccountRequest{
		Address: f.chart.MainAccount(walletID),
		Ledger:  f.ledgerName,
		Expand:  pointer.For("volumes"),
	})
	if err != nil {
		// Account not yet created in Formance — return zero for all currencies.
		if isFormanceNotFound(err) {
			result := make(map[string]*big.Int, len(assets))
			for _, asset := range assets {
				result[asset] = big.NewInt(0)
			}
			return result, nil
		}
		return nil, fmt.Errorf("getting multi-currency balances: %w", err)
	}

	result := make(map[string]*big.Int, len(assets))
	volumes := resp.V2AccountResponse.Data.Volumes

	for _, asset := range assets {
		if volumes == nil {
			result[asset] = big.NewInt(0)
			continue
		}
		vol, ok := volumes[asset]
		if !ok {
			result[asset] = big.NewInt(0)
			continue
		}
		result[asset] = new(big.Int).Sub(vol.Input, vol.Output)
	}

	return result, nil
}

// --- Credit & Debit ---

func (f *FormanceClient) CreditWallet(ctx context.Context, ik string, walletID string, amountCents int64, asset string, source string) error {
	if source == "" {
		source = f.chart.World()
	}

	script := BuildCreditWalletScript(source)
	amountStr := fmt.Sprintf("%s %d", asset, amountCents)

	_, err := f.createTransaction(ctx, ik, shared.V2PostTransaction{
		Script: &shared.V2PostTransactionScript{
			Plain: script,
			Vars: map[string]string{
				"destination": f.chart.MainAccount(walletID),
				"amount":      amountStr,
			},
		},
		Metadata: map[string]string{
			"wallets/transaction": "true",
			"neo/type":            "credit",
			"neo/wallet_id":       walletID,
		},
	})
	if err != nil {
		return fmt.Errorf("crediting wallet: %w", err)
	}

	return nil
}

func (f *FormanceClient) DebitWallet(ctx context.Context, ik string, walletID string, amountCents int64, asset string, destination string, pending bool) (string, error) {
	if destination == "" {
		destination = f.chart.World()
	}

	sources := []string{f.chart.MainAccount(walletID)}

	var holdID string
	var metadata map[string]map[string]string

	if pending {
		holdID = uuid.NewString()
		holdAccount := f.chart.HoldAccount(holdID)
		destination = holdAccount

		metadata = map[string]map[string]string{
			holdAccount: {
				"wallets/spec/type":       "wallets.hold",
				"wallets/holds/wallet_id": walletID,
				"wallets/holds/id":        holdID,
				"wallets/holds/asset":     asset,
			},
		}
	}

	script := BuildDebitWalletScript(metadata, sources...)
	amountStr := fmt.Sprintf("%s %d", asset, amountCents)

	_, err := f.createTransaction(ctx, ik, shared.V2PostTransaction{
		Script: &shared.V2PostTransactionScript{
			Plain: script,
			Vars: map[string]string{
				"destination": destination,
				"amount":      amountStr,
			},
		},
		Metadata: map[string]string{
			"wallets/transaction": "true",
			"neo/type":            "debit",
			"neo/wallet_id":       walletID,
		},
	})
	if err != nil {
		return "", fmt.Errorf("debiting wallet: %w", err)
	}

	return holdID, nil
}

// --- Hold Operations ---

func (f *FormanceClient) HoldFunds(ctx context.Context, ik string, walletID, transitAccount string, amountCents int64, asset string) (string, error) {
	holdID := uuid.NewString()
	holdAccount := f.chart.HoldAccount(holdID)

	sources := []string{f.chart.MainAccount(walletID)}
	script := BuildDebitWalletScript(map[string]map[string]string{
		holdAccount: {
			"wallets/spec/type":       "wallets.hold",
			"wallets/holds/wallet_id": walletID,
			"wallets/holds/id":        holdID,
			"wallets/holds/asset":     asset,
			"neo/transit_account":     transitAccount,
		},
	}, sources...)

	amountStr := fmt.Sprintf("%s %d", asset, amountCents)

	_, err := f.createTransaction(ctx, ik, shared.V2PostTransaction{
		Script: &shared.V2PostTransactionScript{
			Plain: script,
			Vars: map[string]string{
				"destination": holdAccount,
				"amount":      amountStr,
			},
		},
		Metadata: map[string]string{
			"wallets/transaction": "true",
			"neo/type":            "hold",
			"neo/wallet_id":       walletID,
			"neo/hold_id":         holdID,
			"neo/transit_account": transitAccount,
		},
	})
	if err != nil {
		return "", fmt.Errorf("holding funds: %w", err)
	}

	return holdID, nil
}

func (f *FormanceClient) SettleHold(ctx context.Context, ik string, holdID string) error {
	holdAccount := f.chart.HoldAccount(holdID)

	// Fetch the hold account to find the asset and destination.
	account, err := f.getAccount(ctx, holdAccount)
	if err != nil {
		return fmt.Errorf("fetching hold account for settlement: %w", err)
	}

	asset := account.Metadata["wallets/holds/asset"]
	transitAccount := account.Metadata["neo/transit_account"]
	if transitAccount == "" {
		transitAccount = f.chart.World()
	}

	remaining := f.accountBalance(account, asset)
	if remaining.Cmp(big.NewInt(0)) <= 0 {
		return fmt.Errorf("hold %s is already closed (zero remaining)", holdID)
	}

	script := BuildConfirmHoldScript(true, asset)

	walletID := account.Metadata["wallets/holds/wallet_id"]
	_, err = f.createTransaction(ctx, ik, shared.V2PostTransaction{
		Script: &shared.V2PostTransactionScript{
			Plain: script,
			Vars: map[string]string{
				"hold":             holdAccount,
				"amount":           fmt.Sprintf("%s %s", asset, remaining.String()),
				"dest":             transitAccount,
				"void_destination": f.chart.MainAccount(walletID),
			},
		},
		Metadata: map[string]string{
			"wallets/transaction": "true",
			"neo/type":            "settle_hold",
			"neo/hold_id":         holdID,
		},
	})
	if err != nil {
		return fmt.Errorf("settling hold: %w", err)
	}

	return nil
}

func (f *FormanceClient) VoidHold(ctx context.Context, ik string, holdID string) error {
	holdAccount := f.chart.HoldAccount(holdID)

	// Find the original transaction that created this hold.
	txs, err := f.listTransactions(ctx, ListTxQuery{
		Destination: holdAccount,
	})
	if err != nil {
		return fmt.Errorf("finding original hold transaction: %w", err)
	}
	if len(txs) != 1 {
		return fmt.Errorf("expected 1 transaction for hold %s, got %d", holdID, len(txs))
	}

	account, err := f.getAccount(ctx, holdAccount)
	if err != nil {
		return fmt.Errorf("fetching hold account for void: %w", err)
	}

	asset := account.Metadata["wallets/holds/asset"]
	remaining := f.accountBalance(account, asset)
	if remaining.Cmp(big.NewInt(0)) <= 0 {
		return fmt.Errorf("hold %s is already closed (zero remaining)", holdID)
	}

	// Build postings from the original transaction to reverse.
	var postings []Posting
	for _, p := range txs[0].Postings {
		postings = append(postings, Posting{
			Source:      p.Source,
			Destination: p.Destination,
			Amount:      p.Amount.Int64(),
			Asset:       p.Asset,
		})
	}

	script := BuildCancelHoldScript(asset, postings...)

	_, err = f.createTransaction(ctx, ik, shared.V2PostTransaction{
		Script: &shared.V2PostTransactionScript{
			Plain: script,
			Vars: map[string]string{
				"hold": holdAccount,
			},
		},
		Metadata: map[string]string{
			"wallets/transaction": "true",
			"neo/type":            "void_hold",
			"neo/hold_id":         holdID,
		},
	})
	if err != nil {
		return fmt.Errorf("voiding hold: %w", err)
	}

	return nil
}

// --- Loan Operations ---

func (f *FormanceClient) DisburseLoan(ctx context.Context, ik string, walletID string, principalCents int64, asset string) (string, error) {
	script := BuildDisburseLoanScript(f.chart.SystemLoanCapital())
	amountStr := fmt.Sprintf("%s %d", asset, principalCents)

	txID, err := f.createTransaction(ctx, ik, shared.V2PostTransaction{
		Script: &shared.V2PostTransactionScript{
			Plain: script,
			Vars: map[string]string{
				"destination": f.chart.MainAccount(walletID),
				"amount":      amountStr,
			},
		},
		Metadata: map[string]string{
			"wallets/transaction": "true",
			"neo/type":            "loan_disbursement",
			"neo/wallet_id":       walletID,
		},
	})
	if err != nil {
		return "", fmt.Errorf("disbursing loan: %w", err)
	}

	return txID, nil
}

func (f *FormanceClient) CollectLoanRepayment(ctx context.Context, ik string, walletID string, principalCents, feeCents int64, asset string) error {
	script := BuildCollectRepaymentScript(f.chart.SystemLoanCapital(), f.chart.SystemInterest())

	_, err := f.createTransaction(ctx, ik, shared.V2PostTransaction{
		Script: &shared.V2PostTransactionScript{
			Plain: script,
			Vars: map[string]string{
				"source":    f.chart.MainAccount(walletID),
				"principal": fmt.Sprintf("%s %d", asset, principalCents),
				"fee":       fmt.Sprintf("%s %d", asset, feeCents),
			},
		},
		Metadata: map[string]string{
			"wallets/transaction": "true",
			"neo/type":            "loan_repayment",
			"neo/wallet_id":       walletID,
		},
	})
	if err != nil {
		return fmt.Errorf("collecting loan repayment: %w", err)
	}

	return nil
}

// --- Overdraft Operations ---

func (f *FormanceClient) CreditFromOverdraft(ctx context.Context, ik string, walletID string, amountCents int64, asset string) (string, error) {
	script := BuildDisburseLoanScript(f.chart.SystemOverdraftCapital())
	amountStr := fmt.Sprintf("%s %d", asset, amountCents)

	txID, err := f.createTransaction(ctx, ik, shared.V2PostTransaction{
		Script: &shared.V2PostTransactionScript{
			Plain: script,
			Vars: map[string]string{
				"destination": f.chart.MainAccount(walletID),
				"amount":      amountStr,
			},
		},
		Metadata: map[string]string{
			"wallets/transaction": "true",
			"neo/type":            "overdraft_credit",
			"neo/wallet_id":       walletID,
		},
	})
	if err != nil {
		return "", fmt.Errorf("crediting from overdraft: %w", err)
	}
	return txID, nil
}

func (f *FormanceClient) DebitToOverdraft(ctx context.Context, ik string, walletID string, amountCents int64, asset string) error {
	src := f.chart.MainAccount(walletID)
	dst := f.chart.SystemOverdraftCapital()
	script := BuildDebitWalletScript(nil, src)
	amountStr := fmt.Sprintf("%s %d", asset, amountCents)

	_, err := f.createTransaction(ctx, ik, shared.V2PostTransaction{
		Script: &shared.V2PostTransactionScript{
			Plain: script,
			Vars: map[string]string{
				"destination": dst,
				"amount":      amountStr,
			},
		},
		Metadata: map[string]string{
			"wallets/transaction": "true",
			"neo/type":            "overdraft_debit",
			"neo/wallet_id":       walletID,
		},
	})
	if err != nil {
		return fmt.Errorf("debiting to overdraft: %w", err)
	}
	return nil
}

// --- Pot Operations ---

func (f *FormanceClient) TransferToPot(ctx context.Context, ik string, walletID, potID string, amountCents int64, asset string) error {
	src := f.chart.MainAccount(walletID)
	dst := f.chart.PotAccount(walletID, potID)

	script := BuildCreditWalletScript(src)
	amountStr := fmt.Sprintf("%s %d", asset, amountCents)

	_, err := f.createTransaction(ctx, ik, shared.V2PostTransaction{
		Script: &shared.V2PostTransactionScript{
			Plain: script,
			Vars: map[string]string{
				"destination": dst,
				"amount":      amountStr,
			},
		},
		Metadata: map[string]string{
			"wallets/transaction": "true",
			"neo/type":            "pot_deposit",
			"neo/wallet_id":       walletID,
			"neo/pot_id":          potID,
		},
	})
	if err != nil {
		return fmt.Errorf("transferring to pot: %w", err)
	}
	return nil
}

func (f *FormanceClient) TransferFromPot(ctx context.Context, ik string, walletID, potID string, amountCents int64, asset string) error {
	src := f.chart.PotAccount(walletID, potID)
	dst := f.chart.MainAccount(walletID)

	script := BuildCreditWalletScript(src)
	amountStr := fmt.Sprintf("%s %d", asset, amountCents)

	_, err := f.createTransaction(ctx, ik, shared.V2PostTransaction{
		Script: &shared.V2PostTransactionScript{
			Plain: script,
			Vars: map[string]string{
				"destination": dst,
				"amount":      amountStr,
			},
		},
		Metadata: map[string]string{
			"wallets/transaction": "true",
			"neo/type":            "pot_withdrawal",
			"neo/wallet_id":       walletID,
			"neo/pot_id":          potID,
		},
	})
	if err != nil {
		return fmt.Errorf("transferring from pot: %w", err)
	}
	return nil
}

func (f *FormanceClient) GetPotBalance(ctx context.Context, walletID, potID, asset string) (*big.Int, error) {
	address := f.chart.PotAccount(walletID, potID)

	resp, err := f.sdk.Ledger.V2.GetAccount(ctx, operations.V2GetAccountRequest{
		Address: address,
		Ledger:  f.ledgerName,
		Expand:  pointer.For("volumes"),
	})
	if err != nil {
		if isFormanceNotFound(err) {
			return big.NewInt(0), nil
		}
		return nil, fmt.Errorf("getting pot balance: %w", err)
	}

	volumes := resp.V2AccountResponse.Data.Volumes
	if volumes == nil {
		return big.NewInt(0), nil
	}
	vol, ok := volumes[asset]
	if !ok {
		return big.NewInt(0), nil
	}
	return new(big.Int).Sub(vol.Input, vol.Output), nil
}

// --- Currency Conversion ---

func (f *FormanceClient) ConvertCurrency(ctx context.Context, ik string, walletID string, fromCents int64, fromAsset string, toCents int64, toAsset string, fxAccount string) (string, error) {
	script := BuildConvertCurrencyScript()
	userAccount := f.chart.MainAccount(walletID)

	txID, err := f.createTransaction(ctx, ik, shared.V2PostTransaction{
		Script: &shared.V2PostTransactionScript{
			Plain: script,
			Vars: map[string]string{
				"from_amount":  fmt.Sprintf("%s %d", fromAsset, fromCents),
				"to_amount":    fmt.Sprintf("%s %d", toAsset, toCents),
				"user_account": userAccount,
				"fx_account":   fxAccount,
			},
		},
		Metadata: map[string]string{
			"wallets/transaction": "true",
			"neo/type":            "conversion",
			"neo/wallet_id":       walletID,
			"neo/from_asset":      fromAsset,
			"neo/to_asset":        toAsset,
		},
	})
	if err != nil {
		return "", fmt.Errorf("converting currency: %w", err)
	}

	return txID, nil
}

// --- Query Operations ---

func (f *FormanceClient) GetAccountHistory(ctx context.Context, account string, limit int) ([]Transaction, error) {
	txs, err := f.listTransactions(ctx, ListTxQuery{
		Account: account,
		Limit:   limit,
	})
	if err != nil {
		if isFormanceNotFound(err) {
			return []Transaction{}, nil
		}
		return nil, fmt.Errorf("getting account history: %w", err)
	}

	result := make([]Transaction, 0, len(txs))
	for _, tx := range txs {
		meta := filterUserMetadata(tx.Metadata)
		ts := tx.Timestamp.UTC().Format(time.RFC3339)

		for _, p := range tx.Postings {
			result = append(result, Transaction{
				ID:          fmt.Sprintf("%d", tx.ID),
				Source:      p.Source,
				Destination: p.Destination,
				AmountCents: p.Amount.Int64(),
				Asset:       p.Asset,
				IsCredit:    p.Destination == account,
				Metadata:    meta,
				Timestamp:   ts,
			})
		}
	}

	return result, nil
}

// --- Internal helpers ---

// createTransaction executes a transaction against the Formance ledger.
// Returns the transaction ID as a string.
func (f *FormanceClient) createTransaction(ctx context.Context, ik string, tx shared.V2PostTransaction) (string, error) {
	resp, err := f.sdk.Ledger.V2.CreateTransaction(ctx, operations.V2CreateTransactionRequest{
		V2PostTransaction: tx,
		Ledger:            f.ledgerName,
		IdempotencyKey:    pointer.For(ik),
	})
	if err != nil {
		// Check for insufficient funds error from Formance.
		if sdkErr, ok := err.(*sdkerrors.WalletsErrorResponse); ok {
			if sdkErr.ErrorCode == sdkerrors.SchemasWalletsErrorResponseErrorCodeInsufficientFund {
				return "", fmt.Errorf("insufficient funds: %w", err)
			}
		}
		return "", err
	}

	return fmt.Sprintf("%d", resp.V2CreateTransactionResponse.Data.ID), nil
}

// ListTxQuery holds the query parameters for listing transactions.
type ListTxQuery struct {
	Account     string
	Source      string
	Destination string
	Limit       int
	Cursor      string
}

func (f *FormanceClient) listTransactions(ctx context.Context, q ListTxQuery) ([]shared.V2Transaction, error) {
	req := operations.V2ListTransactionsRequest{
		Ledger: f.ledgerName,
	}

	if q.Cursor == "" {
		if q.Limit > 0 {
			req.PageSize = pointer.For(int64(q.Limit))
		}

		conditions := make([]query.Builder, 0)
		if q.Destination != "" {
			conditions = append(conditions, query.Match("destination", q.Destination))
		}
		if q.Source != "" {
			conditions = append(conditions, query.Match("source", q.Source))
		}
		if q.Account != "" {
			conditions = append(conditions, query.Match("account", q.Account))
		}

		if len(conditions) > 0 {
			data, err := json.Marshal(query.And(conditions...))
			if err != nil {
				return nil, fmt.Errorf("marshaling query: %w", err)
			}
			body := make(map[string]any)
			if err := json.Unmarshal(data, &body); err != nil {
				return nil, fmt.Errorf("unmarshaling query body: %w", err)
			}
			req.RequestBody = body
		}
	} else {
		req.Cursor = pointer.For(q.Cursor)
	}

	resp, err := f.sdk.Ledger.V2.ListTransactions(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.V2TransactionsCursorResponse.Cursor.Data, nil
}

func (f *FormanceClient) GetAccountBalance(ctx context.Context, accountAddress, asset string) (*big.Int, error) {
	acc, err := f.getAccount(ctx, accountAddress)
	if err != nil {
		if isFormanceNotFound(err) {
			return big.NewInt(0), nil
		}
		return nil, err
	}
	return f.accountBalance(acc, asset), nil
}

func (f *FormanceClient) getAccount(ctx context.Context, address string) (*shared.V2Account, error) {
	resp, err := f.sdk.Ledger.V2.GetAccount(ctx, operations.V2GetAccountRequest{
		Address: address,
		Ledger:  f.ledgerName,
		Expand:  pointer.For("volumes"),
	})
	if err != nil {
		return nil, err
	}

	return &resp.V2AccountResponse.Data, nil
}

// accountBalance computes balance = input - output for a given asset.
func (f *FormanceClient) accountBalance(account *shared.V2Account, asset string) *big.Int {
	if account.Volumes == nil {
		return big.NewInt(0)
	}
	vol, ok := account.Volumes[asset]
	if !ok {
		return big.NewInt(0)
	}
	return new(big.Int).Sub(vol.Input, vol.Output)
}

// filterUserMetadata strips internal Formance/system keys, keeping
// only user-facing metadata for API responses.
func filterUserMetadata(m map[string]string) map[string]string {
	if len(m) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if strings.HasPrefix(k, "wallets/") || strings.HasPrefix(k, "formance/") {
			continue
		}
		out[k] = v
	}
	return out
}

// isFormanceNotFound returns true if the error indicates a NOT_FOUND response
// from the Formance API (account or resource doesn't exist yet).
func isFormanceNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "NOT_FOUND") || strings.Contains(msg, "not found")
}

func (c *FormanceClient) DebitWalletWithFee(ctx context.Context, ik string, walletID string,
	principalCents int64, feeCents int64, asset string,
	destination string, feeAccount string) (holdID string, err error) {
	if feeCents == 0 {
		return c.DebitWallet(ctx, ik, walletID, principalCents, asset, destination, false)
	}
	totalCents := principalCents + feeCents
	holdID, err = c.DebitWallet(ctx, ik+"-total", walletID, totalCents, asset, destination, false)
	return holdID, err
}

// Verify interface compliance at compile time.
var _ Client = (*FormanceClient)(nil)
