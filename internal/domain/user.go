package domain

import (
	"time"

	"github.com/vonmutinda/neo/pkg/phone"
)

// KYCLevel represents the user's verification tier, which determines
// transaction limits per NBE regulation.
type KYCLevel int

const (
	KYCBasic    KYCLevel = 1 // 75,000 ETB daily limit
	KYCVerified KYCLevel = 2 // 150,000 ETB daily limit
	KYCEnhanced KYCLevel = 3 // Higher limits (institutional)
)

// User represents a customer in the neobank.
// Financial balances are NOT stored here -- they live in Formance.
type User struct {
	ID              string      `json:"id"`
	PhoneNumber     phone.PhoneNumber `json:"phoneNumber"`
	Username        *string           `json:"username,omitempty"`
	PasswordHash    string            `json:"-"`
	AccountType     AccountType       `json:"accountType"`
	FaydaIDNumber   *string    `json:"faydaIdNumber,omitempty"`
	FirstName       *string    `json:"firstName,omitempty"`
	MiddleName      *string    `json:"middleName,omitempty"`
	LastName        *string    `json:"lastName,omitempty"`
	DateOfBirth     *time.Time `json:"dateOfBirth,omitempty"`
	Gender          *string    `json:"gender,omitempty"`
	FaydaPhotoURL   *string    `json:"faydaPhotoUrl,omitempty"`
	KYCLevel        KYCLevel   `json:"kycLevel"`
	IsFrozen        bool       `json:"isFrozen"`
	FrozenReason    *string    `json:"frozenReason,omitempty"`
	FrozenAt        *time.Time `json:"frozenAt,omitempty"`
	LedgerWalletID  string     `json:"ledgerWalletId"`
	TelegramID          *int64         `json:"telegramId,omitempty"`
	TelegramUsername    *string        `json:"telegramUsername,omitempty"`
	SpendWaterfallOrder SpendWaterfall `json:"spendWaterfallOrder,omitempty"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
}

// FullName returns the user's concatenated display name.
func (u *User) FullName() string {
	var name string
	if u.FirstName != nil {
		name = *u.FirstName
	}
	if u.MiddleName != nil {
		name += " " + *u.MiddleName
	}
	if u.LastName != nil {
		name += " " + *u.LastName
	}
	return name
}
