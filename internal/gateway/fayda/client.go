package fayda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/vonmutinda/neo/internal/config"
)

// Client defines the contract for communicating with the Fayda eKYC system.
type Client interface {
	RequestOTP(ctx context.Context, fin, transactionID string) error
	VerifyAndFetchKYC(ctx context.Context, fin, otp, transactionID string) (*KYCResponse, error)
}

type httpClient struct {
	baseURL    string
	apiKey     string
	partnerID  string
	httpClient *http.Client
}

// NewClient creates a Fayda API client.
func NewClient(cfg *config.Fayda) Client {
	return &httpClient{
		baseURL:   cfg.BaseURL,
		apiKey:    cfg.APIKey,
		partnerID: cfg.PartnerID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *httpClient) RequestOTP(ctx context.Context, fin, transactionID string) error {
	reqBody := OTPRequest{
		ID:            "fayda.identity.otp",
		Version:       "2.0",
		TransactionID: transactionID,
		RequestTime:   time.Now().UTC(),
		IndividualID:  fin,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling OTP request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/otp", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating OTP request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fayda OTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fayda OTP request returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *httpClient) VerifyAndFetchKYC(ctx context.Context, fin, otp, transactionID string) (*KYCResponse, error) {
	reqBody := AuthRequest{
		ID:            "fayda.identity.auth",
		Version:       "2.0",
		TransactionID: transactionID,
		RequestTime:   time.Now().UTC(),
		IndividualID:  fin,
		PinValue:      otp,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/kyc", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating KYC request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fayda KYC request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fayda KYC request returned status %d", resp.StatusCode)
	}

	var kycResp KYCResponse
	if err := json.NewDecoder(resp.Body).Decode(&kycResp); err != nil {
		return nil, fmt.Errorf("decoding KYC response: %w", err)
	}

	if kycResp.Status != "y" {
		return nil, fmt.Errorf("fayda authentication failed (status=%s)", kycResp.Status)
	}
	return &kycResp, nil
}

func (c *httpClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("X-Partner-ID", c.partnerID)
}

var _ Client = (*httpClient)(nil)
