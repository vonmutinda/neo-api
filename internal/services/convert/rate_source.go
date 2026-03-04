package convert

import (
	"context"
	"log/slog"
	"time"
)

// RateSource fetches raw mid-market rates from an external upstream.
// Implementations are pluggable: NBE API, third-party feed, or manual.
type RateSource interface {
	FetchRates(ctx context.Context) ([]RawRate, error)
	Name() string
}

// RawRate is an unprocessed rate from an upstream source.
// Spread is applied downstream by the rate refresh service.
type RawRate struct {
	From      string
	To        string
	MidRate   float64
	FetchedAt time.Time
}

// ChainedRateSource tries the primary source first; if it fails, falls back
// to the secondary. This provides resilience without complexity.
type ChainedRateSource struct {
	primary  RateSource
	fallback RateSource
}

func NewChainedRateSource(primary, fallback RateSource) *ChainedRateSource {
	return &ChainedRateSource{primary: primary, fallback: fallback}
}

func (c *ChainedRateSource) FetchRates(ctx context.Context) ([]RawRate, error) {
	rates, err := c.primary.FetchRates(ctx)
	if err == nil {
		return rates, nil
	}
	slog.Warn("primary rate source failed, trying fallback",
		slog.String("primary", c.primary.Name()),
		slog.String("error", err.Error()))
	return c.fallback.FetchRates(ctx)
}

func (c *ChainedRateSource) Name() string { return c.primary.Name() }
