package balances

import (
	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/pkg/validate"
)

type CreateBalanceRequest struct {
	CurrencyCode string          `json:"currencyCode" validate:"required,currency"`
	FXSource     *domain.FXSource `json:"fxSource,omitempty"`
}

func (r *CreateBalanceRequest) Validate() error {
	return validate.Struct(r)
}
