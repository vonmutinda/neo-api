package payment_requests

import (
	"fmt"

	"github.com/vonmutinda/neo/pkg/money"
	"github.com/vonmutinda/neo/pkg/validate"
)

type CreatePaymentRequestForm struct {
	Recipient    string `json:"recipient" validate:"required"`
	AmountCents  int64  `json:"amountCents" validate:"required,gt=0"`
	CurrencyCode string `json:"currencyCode" validate:"required,currency"`
	Narration    string `json:"narration" validate:"required,min=1,max=140"`
}

func (f *CreatePaymentRequestForm) Validate() error {
	if err := validate.Struct(f); err != nil {
		return err
	}
	return money.ValidateAmountCents(f.AmountCents)
}

type DeclineForm struct {
	Reason string `json:"reason" validate:"max=500"`
}

type BatchPaymentRequestForm struct {
	TotalAmountCents int64              `json:"totalAmountCents" validate:"required,gt=0"`
	CurrencyCode     string             `json:"currencyCode" validate:"required"`
	Narration        string             `json:"narration" validate:"required"`
	Recipients       []string           `json:"recipients" validate:"required,min=2,max=10"`
	CustomAmounts    map[string]int64   `json:"customAmounts,omitempty"`
}

func (f *BatchPaymentRequestForm) IsCustomSplit() bool {
	return len(f.CustomAmounts) > 0
}

func (f *BatchPaymentRequestForm) Validate() error {
	if err := validate.Struct(f); err != nil {
		return err
	}
	if err := money.ValidateAmountCents(f.TotalAmountCents); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(f.Recipients))
	for i, r := range f.Recipients {
		if r == "" {
			return fmt.Errorf("%w: recipients[%d] must not be empty", validate.ErrValidation, i)
		}
		if _, dup := seen[r]; dup {
			return fmt.Errorf("%w: duplicate recipient %q", validate.ErrValidation, r)
		}
		seen[r] = struct{}{}
	}
	if f.IsCustomSplit() {
		var customTotal int64
		for _, r := range f.Recipients {
			amt, ok := f.CustomAmounts[r]
			if !ok || amt <= 0 {
				return fmt.Errorf("%w: custom amount for %q must be > 0", validate.ErrValidation, r)
			}
			customTotal += amt
		}
		if customTotal != f.TotalAmountCents {
			return fmt.Errorf("%w: custom amounts (%d) must equal total (%d)", validate.ErrValidation, customTotal, f.TotalAmountCents)
		}
	}
	return nil
}
