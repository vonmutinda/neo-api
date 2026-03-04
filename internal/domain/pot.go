package domain

import "time"

// Pot is a sub-wallet for organizing money by purpose (e.g., "Emergency Fund",
// "Vacation"). Each pot holds a single currency and maps to a Formance account:
// {prefix}:wallets:{walletID}:pot:{potID}.
type Pot struct {
	ID           string     `json:"id"`
	UserID       string     `json:"userId"`
	Name         string     `json:"name"`
	CurrencyCode string     `json:"currencyCode"`
	TargetCents  *int64     `json:"targetCents,omitempty"`
	Emoji        *string    `json:"emoji,omitempty"`
	IsArchived   bool       `json:"isArchived"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	ArchivedAt   *time.Time `json:"-"`

	// Populated from Formance at query time, not stored in Postgres.
	BalanceCents    int64   `json:"balanceCents"`
	Display         string  `json:"display"`
	ProgressPercent float64 `json:"progressPercent,omitempty"`
}
