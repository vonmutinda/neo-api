package domain

import "time"

// CurrencyBalance represents an activated currency within a user's wallet.
// The actual monetary balance lives in Formance; this is the Postgres state.
type CurrencyBalance struct {
	ID           string     `json:"id"`
	UserID       string     `json:"userId"`
	CurrencyCode string     `json:"currencyCode"`
	IsPrimary    bool       `json:"isPrimary"`
	FXSource     *FXSource  `json:"fxSource,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	DeletedAt    *time.Time `json:"-"`
}

// AccountDetails holds per-currency banking details (IBAN, account number, etc.)
// generated when a currency balance is activated for a currency that supports them.
type AccountDetails struct {
	ID                string    `json:"id"`
	CurrencyBalanceID string    `json:"currencyBalanceId"`
	IBAN              string    `json:"iban"`
	AccountNumber     string    `json:"accountNumber"`
	BankName          string    `json:"bankName"`
	SwiftCode         string    `json:"swiftCode"`
	RoutingNumber     *string   `json:"routingNumber,omitempty"`
	SortCode          *string   `json:"sortCode,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
}

// CurrencyBalanceWithDetails combines a currency balance with its account
// details and a live Formance balance, used by API responses.
type CurrencyBalanceWithDetails struct {
	CurrencyBalance
	AccountDetails *AccountDetails `json:"accountDetails,omitempty"`
	BalanceCents   int64           `json:"balanceCents"`
	Display        string          `json:"display"`
}
