package domain

import "time"

type RecipientType string

const (
	RecipientNeoUser     RecipientType = "neo_user"
	RecipientBankAccount RecipientType = "bank_account"
)

type RecipientStatus string

const (
	RecipientActive   RecipientStatus = "active"
	RecipientArchived RecipientStatus = "archived"
)

// Recipient is a sender-scoped record of a transfer counterparty.
type Recipient struct {
	ID          string        `json:"id"`
	OwnerUserID string        `json:"ownerUserId"`
	Type        RecipientType `json:"type"`
	DisplayName string        `json:"displayName"`

	// Neo user fields (type = neo_user)
	NeoUserID   *string `json:"neoUserId,omitempty"`
	CountryCode *string `json:"countryCode,omitempty"`
	Number      *string `json:"number,omitempty"`
	Username    *string `json:"username,omitempty"`

	// Bank account fields (type = bank_account)
	InstitutionCode     *string `json:"institutionCode,omitempty"`
	BankName            *string `json:"bankName,omitempty"`
	SwiftBIC            *string `json:"swiftBic,omitempty"`
	AccountNumber       *string `json:"accountNumber,omitempty"`
	AccountNumberMasked *string `json:"accountNumberMasked,omitempty"`
	BankCountryCode     *string `json:"bankCountryCode,omitempty"`

	// Regulatory link
	BeneficiaryID *string `json:"beneficiaryId,omitempty"`
	IsBeneficiary bool    `json:"isBeneficiary"`

	// Usage tracking
	IsFavorite       bool       `json:"isFavorite"`
	LastUsedAt       *time.Time `json:"lastUsedAt,omitempty"`
	LastUsedCurrency *string    `json:"lastUsedCurrency,omitempty"`
	TransferCount    int        `json:"transferCount"`

	// State
	Status    RecipientStatus `json:"status"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}
