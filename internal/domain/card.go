package domain

import "time"

type CardType string

const (
	CardTypePhysical  CardType = "physical"
	CardTypeVirtual   CardType = "virtual"
	CardTypeEphemeral CardType = "ephemeral"
)

type CardStatus string

const (
	CardStatusActive            CardStatus = "active"
	CardStatusFrozen            CardStatus = "frozen"
	CardStatusCancelled         CardStatus = "cancelled"
	CardStatusExpired           CardStatus = "expired"
	CardStatusPendingActivation CardStatus = "pending_activation"
)

// Card represents a debit card linked to a user's wallet.
// PCI DSS: we NEVER store the raw PAN -- only a token and masked last four.
type Card struct {
	ID                string     `json:"id"`
	UserID            string     `json:"userId"`
	TokenizedPAN      string     `json:"-"`
	LastFour          string     `json:"lastFour"`
	ExpiryMonth       int        `json:"expiryMonth"`
	ExpiryYear        int        `json:"expiryYear"`
	Type              CardType   `json:"type"`
	Status            CardStatus `json:"status"`
	AllowOnline       bool       `json:"allowOnline"`
	AllowContactless  bool       `json:"allowContactless"`
	AllowATM          bool       `json:"allowAtm"`
	AllowInternational bool      `json:"allowInternational"`
	DailyLimitCents   int64      `json:"dailyLimitCents"`
	MonthlyLimitCents int64      `json:"monthlyLimitCents"`
	PerTxnLimitCents  int64      `json:"perTxnLimitCents"`
	LedgerCardAccount *string    `json:"ledgerCardAccount,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// IsActive returns true if the card can be used for transactions.
func (c *Card) IsActive() bool {
	return c.Status == CardStatusActive
}
