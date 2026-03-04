package convert

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ExchangeRateAPISource fetches mid-market rates from the free
// ExchangeRate-API endpoint. No API key required.
type ExchangeRateAPISource struct {
	httpClient *http.Client
}

func NewExchangeRateAPISource() *ExchangeRateAPISource {
	return &ExchangeRateAPISource{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *ExchangeRateAPISource) Name() string { return "api" }

type erAPIResponse struct {
	Result         string             `json:"result"`
	TimeLastUpdate int64              `json:"time_last_update_unix"`
	BaseCode       string             `json:"base_code"`
	Rates          map[string]float64 `json:"rates"`
}

func (s *ExchangeRateAPISource) FetchRates(ctx context.Context) ([]RawRate, error) {
	url := "https://open.er-api.com/v6/latest/USD"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building ExchangeRate-API request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling ExchangeRate-API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ExchangeRate-API returned status %d", resp.StatusCode)
	}

	var data erAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding ExchangeRate-API response: %w", err)
	}

	if data.Result != "success" {
		return nil, fmt.Errorf("ExchangeRate-API returned result %q", data.Result)
	}

	return buildCrossRates(data.Rates, time.Unix(data.TimeLastUpdate, 0)), nil
}
