package ledger

import "strings"

// Chart defines the Formance account naming convention for the neobank.
// All account addresses are colon-separated strings with a configurable prefix.
//
// Account layout:
//
//	@{prefix}:wallets:{walletID}:main           -- User's primary balance
//	@{prefix}:wallets:{walletID}:{balanceName}  -- Named sub-balance
//	@{prefix}:wallets:holds:{holdID}            -- Transit hold account
//	@{prefix}:transit:ethswitch_out             -- EthSwitch outbound transit
//	@{prefix}:transit:card_auth                 -- Card authorization transit
//	@{prefix}:system:loan_capital               -- Loan disbursement pool
//	@{prefix}:system:fees                       -- Platform fee collection
//	@{prefix}:system:interest                   -- Loan interest/facilitation fees
type Chart struct {
	prefix string
}

// NewChart creates a Chart with the given account prefix.
// The prefix is prepended to all account addresses.
func NewChart(prefix string) *Chart {
	return &Chart{prefix: prefix}
}

// --- Address helpers ---

func (c *Chart) base() string {
	if c.prefix != "" {
		return c.prefix + ":"
	}
	return ""
}

// sanitize removes hyphens from UUIDs to comply with Formance's
// account naming rules (alphanumeric + colons only).
func sanitize(s string) string {
	return strings.ReplaceAll(s, "-", "")
}

// --- Wallet Accounts ---

// MainAccount returns the address for a user's primary wallet balance.
// Example: "neo:wallets:abc123:main"
func (c *Chart) MainAccount(walletID string) string {
	return c.base() + "wallets:" + sanitize(walletID) + ":main"
}

// BalanceAccount returns the address for a named sub-balance within a wallet.
// Example: "neo:wallets:abc123:savings"
func (c *Chart) BalanceAccount(walletID, balanceName string) string {
	return c.base() + "wallets:" + sanitize(walletID) + ":" + balanceName
}

// HoldAccount returns the address for a transit hold account.
// Example: "neo:wallets:holds:def456"
func (c *Chart) HoldAccount(holdID string) string {
	return c.base() + "wallets:holds:" + sanitize(holdID)
}

// --- Transit Accounts ---

// TransitEthSwitch returns the holding account for pending EthSwitch outbound transfers.
func (c *Chart) TransitEthSwitch() string {
	return c.base() + "transit:ethswitch_out"
}

// TransitCardAuth returns the holding account for pending card authorizations.
func (c *Chart) TransitCardAuth() string {
	return c.base() + "transit:card_auth"
}

// TransitP2P returns the holding account for pending internal P2P transfers.
func (c *Chart) TransitP2P() string {
	return c.base() + "transit:p2p"
}

// SystemFX returns the FX conversion pool account.
// Used when converting between currencies (e.g., USD → ETB).
func (c *Chart) SystemFX() string {
	return c.base() + "system:fx"
}

// --- System Accounts ---

// SystemLoanCapital returns the loan disbursement pool account.
func (c *Chart) SystemLoanCapital() string {
	return c.base() + "system:loan_capital"
}

// SystemFees returns the platform fee collection account.
func (c *Chart) SystemFees() string {
	return c.base() + "system:fees"
}

// SystemInterest returns the loan interest/facilitation fee account.
func (c *Chart) SystemInterest() string {
	return c.base() + "system:interest"
}

// SystemOverdraftCapital returns the overdraft capital pool account.
func (c *Chart) SystemOverdraftCapital() string {
	return c.base() + "system:overdraft_capital"
}

// --- Pot Accounts ---

// PotAccount returns the address for a user's pot sub-wallet.
// Example: "neo:wallets:abc123:pot:def456"
func (c *Chart) PotAccount(walletID, potID string) string {
	return c.base() + "wallets:" + sanitize(walletID) + ":pot:" + sanitize(potID)
}

// --- Business Accounts ---

// BusinessMainAccount returns the primary balance for a business wallet.
// Example: "neo:wallets:biz:abc123:main"
func (c *Chart) BusinessMainAccount(bizWalletID string) string {
	return c.base() + "wallets:biz:" + sanitize(bizWalletID) + ":main"
}

// BusinessPotAccount returns the address for a business pot sub-wallet.
// Example: "neo:wallets:biz:abc123:pot:def456"
func (c *Chart) BusinessPotAccount(bizWalletID, potID string) string {
	return c.base() + "wallets:biz:" + sanitize(bizWalletID) + ":pot:" + sanitize(potID)
}

// BusinessHoldAccount returns the transit hold account for business operations.
// Example: "neo:wallets:biz:holds:def456"
func (c *Chart) BusinessHoldAccount(holdID string) string {
	return c.base() + "wallets:biz:holds:" + sanitize(holdID)
}

// --- Special Accounts ---

// World returns the Formance infinite source account.
// Used for external inflows (deposits, funding).
func (c *Chart) World() string {
	return "world"
}
