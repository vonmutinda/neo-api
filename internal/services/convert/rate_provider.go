package convert

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
)

// staticDefaultRates is a local copy of the hardcoded rates, used only by
// StaticRateProvider in tests. Keeps this file free of any money.DefaultRates
// reference so the deprecation warning stays clean.
var staticDefaultRates = map[string]float64{
	"USD_ETB": 57.50,
	"EUR_ETB": 62.25,
	"ETB_USD": 1.0 / 57.50,
	"ETB_EUR": 1.0 / 62.25,
	"USD_EUR": 62.25 / 57.50,
	"EUR_USD": 57.50 / 62.25,
}

// Rate holds the full rate quote for a currency pair, including bid/ask spread.
type Rate struct {
	From      string
	To        string
	Mid       float64
	Bid       float64   // Bank buys (user sells) — slightly below mid
	Ask       float64   // Bank sells (user buys) — slightly above mid
	Spread    float64   // Percentage spread applied
	Timestamp time.Time
	Source    string // "nbe_indicative", "manual", "static"
}

// RateProvider abstracts exchange rate lookups.
// Production: DatabaseRateProvider (db_rate_provider.go).
// Tests only: StaticRateProvider below.
type RateProvider interface {
	GetRate(ctx context.Context, from, to string) (Rate, error)
}

// Deprecated: StaticRateProvider exists only for unit tests that don't need a
// database. Production code must use DatabaseRateProvider.
type StaticRateProvider struct {
	rates         map[string]float64
	spreadPercent float64
}

// Deprecated: Use NewDatabaseRateProvider for production code.
// NewStaticRateProvider creates a test-only provider. Pass a custom rates map
// or omit it to use built-in defaults.
func NewStaticRateProvider(spreadPercent float64, rates ...map[string]float64) *StaticRateProvider {
	r := staticDefaultRates
	if len(rates) > 0 && rates[0] != nil {
		r = rates[0]
	}
	return &StaticRateProvider{rates: r, spreadPercent: spreadPercent}
}

func (p *StaticRateProvider) GetRate(_ context.Context, from, to string) (Rate, error) {
	if from == to {
		return Rate{
			From: from, To: to,
			Mid: 1.0, Bid: 1.0, Ask: 1.0,
			Timestamp: time.Now(), Source: "static",
		}, nil
	}

	key := from + "_" + to
	mid, ok := p.rates[key]
	if !ok || mid <= 0 {
		return Rate{}, fmt.Errorf("no rate for %s -> %s: %w", from, to, domain.ErrInvalidCurrency)
	}

	spreadFactor := p.spreadPercent / 100.0
	bid := mid * (1 - spreadFactor/2)
	ask := mid * (1 + spreadFactor/2)

	return Rate{
		From:      from,
		To:        to,
		Mid:       mid,
		Bid:       bid,
		Ask:       ask,
		Spread:    p.spreadPercent,
		Timestamp: time.Now(),
		Source:    "static",
	}, nil
}

// ConvertWithSpread applies the ask rate (user buys `to` currency) and returns
// the converted amount in cents along with the effective rate used.
func ConvertWithSpread(rate Rate, amountCents int64) (toCents int64, effectiveRate float64) {
	effectiveRate = rate.Ask
	toCents = int64(math.Round(float64(amountCents) * effectiveRate))
	return toCents, effectiveRate
}
