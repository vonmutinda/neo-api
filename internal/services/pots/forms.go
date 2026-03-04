package pots

import (
	"github.com/vonmutinda/neo/pkg/validate"
)

type CreatePotRequest struct {
	Name         string  `json:"name" validate:"required,min=1,max=50"`
	CurrencyCode string  `json:"currencyCode" validate:"required,currency"`
	TargetCents  *int64  `json:"targetCents"`
	Emoji        *string `json:"emoji"`
}

func (r *CreatePotRequest) Validate() error {
	return validate.Struct(r)
}

type UpdatePotRequest struct {
	Name        *string `json:"name" validate:"omitempty,min=1,max=50"`
	TargetCents *int64  `json:"targetCents"`
	Emoji       *string `json:"emoji"`
}

func (r *UpdatePotRequest) Validate() error {
	return validate.Struct(r)
}

type PotTransferRequest struct {
	AmountCents int64 `json:"amountCents" validate:"required,gt=0"`
}

func (r *PotTransferRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	return validate.AmountCents(r.AmountCents)
}
