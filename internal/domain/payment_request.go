package domain

import (
	"time"

	"github.com/vonmutinda/neo/pkg/phone"
)

type PaymentRequestStatus string

const (
	PaymentRequestPending   PaymentRequestStatus = "pending"
	PaymentRequestPaid      PaymentRequestStatus = "paid"
	PaymentRequestDeclined  PaymentRequestStatus = "declined"
	PaymentRequestCancelled PaymentRequestStatus = "cancelled"
	PaymentRequestExpired   PaymentRequestStatus = "expired"
)

// PaymentRequest represents a P2P fund request from one user to another.
type PaymentRequest struct {
	ID             string               `json:"id"`
	RequesterID    string               `json:"requesterId"`
	PayerID        *string              `json:"payerId,omitempty"`
	PayerPhone     phone.PhoneNumber    `json:"payerPhone"`
	AmountCents    int64                `json:"amountCents"`
	CurrencyCode   string               `json:"currencyCode"`
	Narration      string               `json:"narration"`
	Status         PaymentRequestStatus `json:"status"`
	TransactionID  *string              `json:"transactionId,omitempty"`
	DeclineReason  *string              `json:"declineReason,omitempty"`
	ReminderCount  int                  `json:"reminderCount"`
	LastRemindedAt *time.Time           `json:"lastRemindedAt,omitempty"`
	PaidAt         *time.Time           `json:"paidAt,omitempty"`
	DeclinedAt     *time.Time           `json:"declinedAt,omitempty"`
	CancelledAt    *time.Time           `json:"cancelledAt,omitempty"`
	ExpiresAt      time.Time            `json:"expiresAt"`
	CreatedAt      time.Time            `json:"createdAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`

	RequesterName string `json:"requesterName,omitempty"`
	PayerName     string `json:"payerName,omitempty"`
}
