package auth

import (
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/vonmutinda/neo/pkg/validate"
)

type RegisterRequest struct {
	PhoneNumber phone.PhoneNumber `json:"phoneNumber" validate:"required"`
	Username    string            `json:"username" validate:"required,min=3,max=30"`
	Password    string            `json:"password" validate:"required,min=8"`
}

func (r *RegisterRequest) Validate() error {
	return validate.Struct(r)
}

type LoginRequest struct {
	Identifier string `json:"identifier" validate:"required"`
	Password   string `json:"password" validate:"required"`
}

func (r *LoginRequest) Validate() error {
	return validate.Struct(r)
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" validate:"required"`
}

func (r *RefreshRequest) Validate() error {
	return validate.Struct(r)
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
	NewPassword     string `json:"newPassword" validate:"required,min=8"`
}

func (r *ChangePasswordRequest) Validate() error {
	return validate.Struct(r)
}

type TokenResponse struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    string    `json:"expiresAt"`
	User         *UserInfo `json:"user"`
}

type UserInfo struct {
	ID          string            `json:"id"`
	PhoneNumber phone.PhoneNumber `json:"phoneNumber"`
	Username    *string           `json:"username,omitempty"`
	FirstName   *string           `json:"firstName,omitempty"`
	LastName    *string           `json:"lastName,omitempty"`
	KYCLevel    int               `json:"kycLevel"`
}
