package payments

import (
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/pkg/money"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/vonmutinda/neo/pkg/validate"
)

type OutboundTransferRequest struct {
	AmountCents     int64             `json:"amountCents" validate:"required,gt=0"`
	Currency        string            `json:"currency" validate:"required,currency"`
	DestPhone       phone.PhoneNumber `json:"destPhone" validate:"required"`
	DestInstitution string            `json:"destInstitution" validate:"required"`
	Narration       string `json:"narration" validate:"required"`
	Purpose         string `json:"purpose"`
	Destination     string `json:"destination"`
	BeneficiaryID   string `json:"beneficiaryId"`
}

func (r *OutboundTransferRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	return money.ValidateAmountCents(r.AmountCents)
}

type InboundTransferRequest struct {
	Recipient      string `json:"recipient"`
	RecipientPhone phone.PhoneNumber `json:"recipientPhone"`
	AmountCents    int64  `json:"amountCents" validate:"required,gt=0"`
	Currency       string `json:"currency" validate:"required,currency"`
	Narration      string `json:"narration" validate:"required"`
	Purpose        string `json:"purpose"`
}

func (r *InboundTransferRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	if err := money.ValidateAmountCents(r.AmountCents); err != nil {
		return err
	}
	return r.validateRecipient()
}

func (r *InboundTransferRequest) validateRecipient() error {
	if r.Recipient == "" && r.RecipientPhone.IsZero() {
		return fmt.Errorf("recipient is required: %w", domain.ErrInvalidInput)
	}

	// If the new Recipient field is set, use it
	if r.Recipient != "" {
		if _, err := phone.Parse(r.Recipient); err == nil {
			return nil
		}
		if len(r.Recipient) < 3 || len(r.Recipient) > 30 {
			return fmt.Errorf("username must be 3-30 characters: %w", domain.ErrInvalidInput)
		}
		return nil
	}

	// Legacy: RecipientPhone was provided directly
	if !r.RecipientPhone.IsValid() {
		return fmt.Errorf("invalid phone number: %w", domain.ErrInvalidInput)
	}
	return nil
}

// --- Batch Transfer ---

type BatchTransferItem struct {
	Recipient   string `json:"recipient" validate:"required"`
	AmountCents int64  `json:"amountCents" validate:"required,gt=0"`
	Narration   string `json:"narration" validate:"required"`
}

type BatchTransferRequest struct {
	Currency string              `json:"currency" validate:"required,currency"`
	Items    []BatchTransferItem `json:"items" validate:"required,min=2,max=10"`
}

func (r *BatchTransferRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(r.Items))
	for i, item := range r.Items {
		if err := money.ValidateAmountCents(item.AmountCents); err != nil {
			return fmt.Errorf("item %d: %w", i, err)
		}
		norm := item.Recipient
		if _, err := phone.Parse(norm); err == nil {
			p, _ := phone.Parse(norm)
			norm = p.E164()
		}
		if _, dup := seen[norm]; dup {
			return fmt.Errorf("duplicate recipient %q: %w", item.Recipient, domain.ErrInvalidInput)
		}
		seen[norm] = struct{}{}
	}
	return nil
}

func (r *BatchTransferRequest) TotalCents() int64 {
	var total int64
	for _, item := range r.Items {
		total += item.AmountCents
	}
	return total
}

type BatchTransferResponse struct {
	Status         string `json:"status"`
	ReceiptID      string `json:"receiptId"`
	RecipientCount int    `json:"recipientCount"`
	TotalCents     int64  `json:"totalCents"`
	Currency       string `json:"currency"`
}
