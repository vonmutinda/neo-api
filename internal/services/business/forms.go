package business

import (
	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/pkg/phone"
	"github.com/vonmutinda/neo/pkg/validate"
)

type RegisterRequest struct {
	Name                string `json:"name" validate:"required,min=2,max=200"`
	TradeName           string `json:"tradeName,omitempty"`
	TINNumber           string `json:"tinNumber" validate:"required,min=8,max=20"`
	TradeLicenseNumber  string `json:"tradeLicenseNumber" validate:"required,min=5,max=30"`
	IndustryCategory    string `json:"industryCategory" validate:"required"`
	IndustrySubCategory string `json:"industrySubCategory,omitempty"`
	PhoneNumber         phone.PhoneNumber `json:"phoneNumber" validate:"required"`
	Email               string            `json:"email,omitempty"`
	City                string `json:"city,omitempty"`
	SubCity             string `json:"subCity,omitempty"`
}

func (r *RegisterRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	return validate.EnumValue(r.IndustryCategory, "industryCategory", domain.ValidIndustryCategories)
}

type UpdateBusinessRequest struct {
	Name                string `json:"name,omitempty"`
	TradeName           string `json:"tradeName,omitempty"`
	IndustryCategory    string `json:"industryCategory,omitempty"`
	IndustrySubCategory string `json:"industrySubCategory,omitempty"`
	Address             string `json:"address,omitempty"`
	City                string `json:"city,omitempty"`
	SubCity             string `json:"subCity,omitempty"`
	Woreda              string `json:"woreda,omitempty"`
	PhoneNumber         phone.PhoneNumber `json:"phoneNumber,omitempty"`
	Email               string `json:"email,omitempty"`
	Website             string `json:"website,omitempty"`
}

type InviteMemberRequest struct {
	PhoneNumber phone.PhoneNumber `json:"phoneNumber" validate:"required"`
	RoleID      string  `json:"roleId" validate:"required"`
	Title       *string `json:"title,omitempty"`
}

func (r *InviteMemberRequest) Validate() error {
	return validate.Struct(r)
}

type UpdateMemberRoleRequest struct {
	RoleID string `json:"roleId" validate:"required"`
}

func (r *UpdateMemberRoleRequest) Validate() error {
	return validate.Struct(r)
}

type CreateRoleRequest struct {
	Name                       string   `json:"name" validate:"required,min=2,max=50"`
	Description                string   `json:"description,omitempty"`
	MaxTransferCents           *int64   `json:"maxTransferCents,omitempty"`
	DailyTransferLimitCents    *int64   `json:"dailyTransferLimitCents,omitempty"`
	RequiresApprovalAboveCents *int64   `json:"requiresApprovalAboveCents,omitempty"`
	Permissions                []string `json:"permissions" validate:"required,min=1"`
}

func (r *CreateRoleRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	for _, p := range r.Permissions {
		if !domain.ValidBusinessPermission(p) {
			return domain.ErrInvalidInput
		}
	}
	return nil
}

type UpdateRoleRequest struct {
	Name                       string   `json:"name,omitempty"`
	Description                string   `json:"description,omitempty"`
	MaxTransferCents           *int64   `json:"maxTransferCents,omitempty"`
	DailyTransferLimitCents    *int64   `json:"dailyTransferLimitCents,omitempty"`
	RequiresApprovalAboveCents *int64   `json:"requiresApprovalAboveCents,omitempty"`
	Permissions                []string `json:"permissions,omitempty"`
}

func (r *UpdateRoleRequest) Validate() error {
	for _, p := range r.Permissions {
		if !domain.ValidBusinessPermission(p) {
			return domain.ErrInvalidInput
		}
	}
	return nil
}
