package ethswitch

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/vonmutinda/neo/internal/config"
)

// Client defines the contract for communicating with EthSwitch EthioPay-IPS.
type Client interface {
	InitiateTransfer(ctx context.Context, req TransferRequest) (*TransferResponse, error)
	CheckTransactionStatus(ctx context.Context, idempotencyKey string) (*StatusCheckResponse, error)
}

type httpClient struct {
	client  *http.Client
	baseURL string
}

// NewMTLSClient constructs an HTTP client loaded with NBE-issued certificates.
// Returns a no-op stub in dev mode when certs are not configured.
func NewMTLSClient(cfg *config.EthSwitch) (Client, error) {
	if cfg.BaseURL == "" || cfg.CertPath == "" || cfg.KeyPath == "" || cfg.CAPath == "" {
		return &stubClient{}, nil
	}

	clientCert, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("loading client key pair: %w", err)
	}

	caCert, err := os.ReadFile(cfg.CAPath)
	if err != nil {
		return nil, fmt.Errorf("reading CA certificate: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate to pool")
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS13,
	}

	transport := &http.Transport{
		TLSClientConfig:     tlsCfg,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &httpClient{
		baseURL: cfg.BaseURL,
		client: &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
		},
	}, nil
}

func (c *httpClient) InitiateTransfer(ctx context.Context, req TransferRequest) (*TransferResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling transfer request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/transfers/ips", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating transfer request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Idempotency-Key", req.IdempotencyKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("network failure communicating with EthSwitch: %w", err)
	}
	defer resp.Body.Close()

	var result TransferResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding EthSwitch response: %w", err)
	}
	return &result, nil
}

func (c *httpClient) CheckTransactionStatus(ctx context.Context, idempotencyKey string) (*StatusCheckResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/transfers/status/"+idempotencyKey, nil)
	if err != nil {
		return nil, fmt.Errorf("creating status check request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("network failure checking EthSwitch status: %w", err)
	}
	defer resp.Body.Close()

	var result StatusCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding status response: %w", err)
	}
	return &result, nil
}

var _ Client = (*httpClient)(nil)

type stubClient struct{}

func (s *stubClient) InitiateTransfer(_ context.Context, _ TransferRequest) (*TransferResponse, error) {
	return nil, fmt.Errorf("ethswitch not configured (dev mode)")
}

func (s *stubClient) CheckTransactionStatus(_ context.Context, _ string) (*StatusCheckResponse, error) {
	return nil, fmt.Errorf("ethswitch not configured (dev mode)")
}

var _ Client = (*stubClient)(nil)
