package wise

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/services/remittance"
)

type Provider struct {
	apiToken   string
	profileID  string
	baseURL    string
	httpClient *http.Client
	corridors  []remittance.Corridor
}

func NewProvider(apiToken, profileID, baseURL string) *Provider {
	return &Provider{
		apiToken:   apiToken,
		profileID:  profileID,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		corridors: []remittance.Corridor{
			{SourceCurrency: "ETB", TargetCurrency: "USD"},
			{SourceCurrency: "ETB", TargetCurrency: "EUR"},
			{SourceCurrency: "ETB", TargetCurrency: "GBP"},
			{SourceCurrency: "USD", TargetCurrency: "ETB"},
			{SourceCurrency: "EUR", TargetCurrency: "ETB"},
			{SourceCurrency: "GBP", TargetCurrency: "ETB"},
		},
	}
}

func (p *Provider) ID() string                                { return "wise" }
func (p *Provider) Name() string                              { return "Wise Platform" }
func (p *Provider) SupportedCorridors() []remittance.Corridor { return p.corridors }

func (p *Provider) GetQuote(ctx context.Context, req remittance.QuoteRequest) (*domain.RemittanceQuote, error) {
	// TODO: implement Wise Platform API v3 quote creation
	return nil, fmt.Errorf("wise provider not yet implemented: %w", domain.ErrProviderUnavailable)
}

func (p *Provider) CreateTransfer(ctx context.Context, quoteID string, recipient remittance.RecipientDetails) (*remittance.TransferResult, error) {
	return nil, fmt.Errorf("wise provider not yet implemented: %w", domain.ErrProviderUnavailable)
}

func (p *Provider) GetTransferStatus(ctx context.Context, providerTransferID string) (*remittance.TransferStatus, error) {
	return nil, fmt.Errorf("wise provider not yet implemented: %w", domain.ErrProviderUnavailable)
}
