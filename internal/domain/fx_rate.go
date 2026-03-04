package domain

import "time"

type FXRateSource string

const (
	FXRateSourceNBE    FXRateSource = "nbe_indicative"
	FXRateSourceManual FXRateSource = "manual"
	FXRateSourceAPI    FXRateSource = "api"
)

// FXRate represents a point-in-time exchange rate between two currencies.
type FXRate struct {
	ID            string       `json:"id"`
	FromCurrency  string       `json:"fromCurrency"`
	ToCurrency    string       `json:"toCurrency"`
	MidRate       float64      `json:"midRate"`
	BidRate       float64      `json:"bidRate"`
	AskRate       float64      `json:"askRate"`
	SpreadPercent float64      `json:"spreadPercent"`
	Source        FXRateSource `json:"source"`
	FetchedAt     time.Time    `json:"fetchedAt"`
	CreatedAt     time.Time    `json:"createdAt"`
}
