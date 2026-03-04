package ledger

import (
	"context"
	"math/big"
)

// Client defines the interface for all financial ledger operations.
// The only implementation talks to Formance CE, but the interface enables
// testing with mocks and future migration to other ledger systems.
//
// Every method accepts context.Context for timeout/cancellation propagation.
// All monetary amounts are in minor units / cents (int64).
type Client interface {
	// --- Lifecycle ---

	// EnsureLedgerExists creates the Formance ledger if it doesn't already exist.
	EnsureLedgerExists(ctx context.Context) error

	// --- Wallet Operations ---

	// CreateWallet creates a new wallet for a user by setting metadata on the
	// user's main balance account in Formance.
	CreateWallet(ctx context.Context, walletID, name string) error

	// GetWalletBalance returns the current balance for a given asset in the
	// user's main wallet account. Returns the balance in minor units.
	GetWalletBalance(ctx context.Context, walletID, asset string) (*big.Int, error)

	// GetMultiCurrencyBalances returns the balance for each of the given assets
	// in a single wallet. Returns a map of asset string -> balance in minor units.
	// Used by the Wise-style account summary endpoint.
	GetMultiCurrencyBalances(ctx context.Context, walletID string, assets []string) (map[string]*big.Int, error)

	// --- Credit & Debit ---

	// CreditWallet moves funds into a wallet from the given source account.
	// If source is empty, funds come from @world (external inflow).
	CreditWallet(ctx context.Context, ik string, walletID string, amountCents int64, asset string, source string) error

	// DebitWallet moves funds out of a wallet to the given destination account.
	// If pending is true, creates a hold instead of an immediate transfer.
	// Returns the hold ID if pending, empty string otherwise.
	DebitWallet(ctx context.Context, ik string, walletID string, amountCents int64, asset string, destination string, pending bool) (holdID string, err error)

	// --- Hold Operations (Two-Phase Commit) ---

	// HoldFunds creates a pending hold: moves funds from the user's wallet
	// to a transit account. Returns the hold ID.
	HoldFunds(ctx context.Context, ik string, walletID, transitAccount string, amountCents int64, asset string) (holdID string, err error)

	// SettleHold confirms a pending hold, moving funds from transit to the
	// final destination. This is called after EthSwitch confirms SUCCESS.
	SettleHold(ctx context.Context, ik string, holdID string) error

	// VoidHold cancels a pending hold, returning funds from transit back to
	// the user's wallet. Called after EthSwitch returns FAILED.
	VoidHold(ctx context.Context, ik string, holdID string) error

	// DebitWalletWithFee atomically debits the principal to a destination and
	// the fee to the fee account in a single Formance transaction.
	DebitWalletWithFee(ctx context.Context, ik string, walletID string,
		principalCents int64, feeCents int64, asset string,
		destination string, feeAccount string) (holdID string, err error)

	// --- Loan Operations ---

	// DisburseLoan moves funds from @system:loan_capital to the user's wallet.
	DisburseLoan(ctx context.Context, ik string, walletID string, principalCents int64, asset string) (txID string, err error)

	// CollectLoanRepayment moves funds from the user's wallet to @system:loan_capital
	// and fees to @system:interest.
	CollectLoanRepayment(ctx context.Context, ik string, walletID string, principalCents, feeCents int64, asset string) error

	// --- Overdraft Operations ---

	// CreditFromOverdraft moves funds from @system:overdraft_capital to the wallet.
	CreditFromOverdraft(ctx context.Context, ik string, walletID string, amountCents int64, asset string) (txID string, err error)

	// DebitToOverdraft moves funds from the wallet to @system:overdraft_capital.
	DebitToOverdraft(ctx context.Context, ik string, walletID string, amountCents int64, asset string) error

	// --- Pot Operations ---

	// TransferToPot moves funds from a user's main wallet to a pot.
	TransferToPot(ctx context.Context, ik string, walletID, potID string, amountCents int64, asset string) error

	// TransferFromPot moves funds from a pot back to the user's main wallet.
	TransferFromPot(ctx context.Context, ik string, walletID, potID string, amountCents int64, asset string) error

	// GetPotBalance returns the current balance for a pot account.
	GetPotBalance(ctx context.Context, walletID, potID, asset string) (*big.Int, error)

	// --- Currency Conversion ---

	// ConvertCurrency atomically debits fromAsset from the user's wallet into
	// the FX system account and credits toAsset from @world into the user's
	// wallet. This is a single Formance transaction with two postings, so
	// either both succeed or neither does.
	ConvertCurrency(ctx context.Context, ik string, walletID string, fromCents int64, fromAsset string, toCents int64, toAsset string, fxAccount string) (txID string, err error)

	// --- Query Operations ---

	// GetAccountBalance returns the balance for an arbitrary account and asset (e.g. system capital pools).
	GetAccountBalance(ctx context.Context, accountAddress, asset string) (*big.Int, error)

	// GetAccountHistory returns transactions for a specific account within
	// a time window. Used by the trust score algorithm.
	GetAccountHistory(ctx context.Context, account string, limit int) ([]Transaction, error)
}

// Transaction is a simplified view of a Formance transaction,
// used for analytics and trust score calculation.
type Transaction struct {
	ID          string            `json:"id"`
	Source      string            `json:"source"`
	Destination string            `json:"destination"`
	AmountCents int64             `json:"amountCents"`
	Asset       string            `json:"asset"`
	IsCredit    bool              `json:"isCredit"`
	Metadata    map[string]string `json:"metadata"`
	Timestamp   string            `json:"timestamp"`
}
