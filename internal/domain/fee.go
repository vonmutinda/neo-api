package domain

import (
	"encoding/json"
	"time"
)

type FeeType string

const (
	FeeTypeFXSpread        FeeType = "fx_spread"
	FeeTypeTransferFlat    FeeType = "transfer_flat"
	FeeTypeTransferPercent FeeType = "transfer_percent"
	FeeTypeCorridorMarkup  FeeType = "corridor_markup"
)

type TransactionType string

const (
	TxTypeP2P                   TransactionType = "p2p"
	TxTypeEthSwitchOut          TransactionType = "ethswitch_out"
	TxTypeFXConversion          TransactionType = "fx_conversion"
	TxTypeCardInternational     TransactionType = "card_international"
	TxTypeInternationalTransfer TransactionType = "international_transfer"
)

type FeeSchedule struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	FeeType         FeeType         `json:"feeType"`
	TransactionType TransactionType `json:"transactionType"`
	Currency        *string         `json:"currency,omitempty"`
	Channel         *string         `json:"channel,omitempty"`
	FlatAmountCents int64           `json:"flatAmountCents"`
	PercentBps      int             `json:"percentBps"`
	MinFeeCents     int64           `json:"minFeeCents"`
	MaxFeeCents     int64           `json:"maxFeeCents"`
	IsActive        bool            `json:"isActive"`
	EffectiveFrom   time.Time       `json:"effectiveFrom"`
	EffectiveTo     *time.Time      `json:"effectiveTo,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

type FeeBreakdown struct {
	OurFeeCents     int64       `json:"ourFeeCents"`
	PartnerFeeCents int64       `json:"partnerFeeCents"`
	TotalFeeCents   int64       `json:"totalFeeCents"`
	Details         []FeeDetail `json:"details"`
}

type FeeDetail struct {
	Name        string `json:"name"`
	AmountCents int64  `json:"amountCents"`
	Type        string `json:"type"`
}

type RemittanceQuote struct {
	ProviderID       string        `json:"providerId"`
	ProviderName     string        `json:"providerName"`
	QuoteID          string        `json:"quoteId"`
	SourceCurrency   string        `json:"sourceCurrency"`
	TargetCurrency   string        `json:"targetCurrency"`
	SourceAmount     int64         `json:"sourceAmount"`
	TargetAmount     int64         `json:"targetAmount"`
	ExchangeRate     float64       `json:"exchangeRate"`
	ProviderFeeCents int64         `json:"providerFeeCents"`
	DeliveryEstimate string        `json:"deliveryEstimate"`
	ExpiresAt        time.Time     `json:"expiresAt"`
	Fee              *FeeBreakdown `json:"fee,omitempty"`
}

type RemittanceTransferStatus string

const (
	RemittanceStatusPending        RemittanceTransferStatus = "pending"
	RemittanceStatusProcessing     RemittanceTransferStatus = "processing"
	RemittanceStatusFundsConverted RemittanceTransferStatus = "funds_converted"
	RemittanceStatusSent           RemittanceTransferStatus = "sent"
	RemittanceStatusDelivered      RemittanceTransferStatus = "delivered"
	RemittanceStatusFailed         RemittanceTransferStatus = "failed"
	RemittanceStatusCancelled      RemittanceTransferStatus = "cancelled"
	RemittanceStatusRefunded       RemittanceTransferStatus = "refunded"
)

type RemittanceTransfer struct {
	ID                 string                   `json:"id"`
	UserID             string                   `json:"userId"`
	ProviderID         string                   `json:"providerId"`
	ProviderTransferID string                   `json:"providerTransferId"`
	QuoteID            string                   `json:"quoteId"`
	SourceCurrency     string                   `json:"sourceCurrency"`
	TargetCurrency     string                   `json:"targetCurrency"`
	SourceAmountCents  int64                    `json:"sourceAmountCents"`
	TargetAmountCents  int64                    `json:"targetAmountCents"`
	ExchangeRate       float64                  `json:"exchangeRate"`
	OurFeeCents        int64                    `json:"ourFeeCents"`
	ProviderFeeCents   int64                    `json:"providerFeeCents"`
	TotalFeeCents      int64                    `json:"totalFeeCents"`
	Status             RemittanceTransferStatus `json:"status"`
	RecipientName      string                   `json:"recipientName"`
	RecipientCountry   string                   `json:"recipientCountry"`
	HoldID             *string                  `json:"holdId,omitempty"`
	FailureReason      *string                  `json:"failureReason,omitempty"`
	CreatedAt          time.Time                `json:"createdAt"`
	UpdatedAt          time.Time                `json:"updatedAt"`
}

// JSON serializes a FeeBreakdown for storage in transaction_receipts.
func (fb *FeeBreakdown) JSON() json.RawMessage {
	b, _ := json.Marshal(fb)
	return b
}
