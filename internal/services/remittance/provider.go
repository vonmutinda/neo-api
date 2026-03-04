package remittance

import (
	"context"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/pkg/phone"
)

type Corridor struct {
	SourceCurrency string `json:"source"`
	TargetCurrency string `json:"target"`
}

type QuoteRequest struct {
	SourceCurrency string
	TargetCurrency string
	SourceAmount   int64
}

type RecipientDetails struct {
	FullName    string
	Country     string
	AccountNo   *string
	RoutingNo   *string
	IBAN        *string
	SwiftBIC    *string
	Email       *string
	PhoneNumber *phone.PhoneNumber
}

type TransferResult struct {
	ProviderTransferID string
	Status             domain.RemittanceTransferStatus
	EstimatedDelivery  *string
}

type TransferStatus struct {
	ProviderTransferID string
	Status             domain.RemittanceTransferStatus
	FailureReason      *string
}

type RemittanceProvider interface {
	ID() string
	Name() string
	SupportedCorridors() []Corridor
	GetQuote(ctx context.Context, req QuoteRequest) (*domain.RemittanceQuote, error)
	CreateTransfer(ctx context.Context, quoteID string, recipient RecipientDetails) (*TransferResult, error)
	GetTransferStatus(ctx context.Context, providerTransferID string) (*TransferStatus, error)
}
