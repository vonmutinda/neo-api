package onboarding

import (
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/vonmutinda/neo/pkg/validate"
)

type RegisterRequest struct {
	PhoneNumber phone.PhoneNumber `json:"phoneNumber" validate:"required"`
}

func (r *RegisterRequest) Validate() error {
	return validate.Struct(r)
}

type KYCOTPRequest struct {
	FaydaFIN string `json:"faydaFin" validate:"required,min=5"`
}

func (r *KYCOTPRequest) Validate() error {
	return validate.Struct(r)
}

type KYCVerifyRequest struct {
	FaydaFIN      string `json:"faydaFin" validate:"required,min=5"`
	OTP           string `json:"otp" validate:"required,len=6"`
	TransactionID string `json:"transactionId" validate:"required"`
}

func (r *KYCVerifyRequest) Validate() error {
	return validate.Struct(r)
}
