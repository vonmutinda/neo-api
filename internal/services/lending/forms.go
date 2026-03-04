package lending

import (
	"github.com/vonmutinda/neo/pkg/validate"
)

type LoanApplyRequest struct {
	PrincipalCents int64 `json:"principalCents" validate:"required,gt=0"`
	DurationDays   int   `json:"durationDays" validate:"required,gte=7,lte=365"`
}

func (r *LoanApplyRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	return validate.AmountCents(r.PrincipalCents)
}

type LoanRepayRequest struct {
	AmountCents int64 `json:"amountCents" validate:"required,gt=0"`
}

func (r *LoanRepayRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	return validate.AmountCents(r.AmountCents)
}
