package convert

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OpenExchangeRateSource fetches mid-market rates from the Open Exchange Rates
// API. The free tier returns USD-based rates; cross-rates are computed by
// dividing through USD.
type OpenExchangeRateSource struct {
	appID      string
	httpClient *http.Client
}

func NewOpenExchangeRateSource(appID string) *OpenExchangeRateSource {
	return &OpenExchangeRateSource{
		appID:      appID,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *OpenExchangeRateSource) Name() string { return "api" }

type oxrResponse struct {
	Timestamp int64              `json:"timestamp"`
	Base      string             `json:"base"`
	Rates     map[string]float64 `json:"rates"`
}

func (s *OpenExchangeRateSource) FetchRates(ctx context.Context) ([]RawRate, error) {
	url := fmt.Sprintf(
		"https://openexchangerates.org/api/latest.json?app_id=%s&symbols=ETB,EUR,USD",
		s.appID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building OXR request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling OXR API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OXR API returned status %d", resp.StatusCode)
	}

	var data oxrResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding OXR response: %w", err)
	}

	return buildCrossRates(data.Rates, time.Unix(data.Timestamp, 0)), nil
}

// buildCrossRates takes USD-based rates and produces all pair permutations.
// Given {ETB: 57.5, EUR: 0.92, USD: 1.0}, it produces:
// USD/ETB, ETB/USD, USD/EUR, EUR/USD, ETB/EUR, EUR/ETB
func buildCrossRates(usdRates map[string]float64, fetchedAt time.Time) []RawRate {
	usdRates["USD"] = 1.0
	codes := []string{"USD", "ETB", "EUR"}

	var rates []RawRate
	for _, from := range codes {
		for _, to := range codes {
			if from == to {
				continue
			}
			fromRate, fromOK := usdRates[from]
			toRate, toOK := usdRates[to]
			if !fromOK || !toOK || fromRate <= 0 {
				continue
			}
			rates = append(rates, RawRate{
				From:      from,
				To:        to,
				MidRate:   toRate / fromRate,
				FetchedAt: fetchedAt,
			})
		}
	}
	return rates
}
