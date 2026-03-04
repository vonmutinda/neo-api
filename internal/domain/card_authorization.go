package domain

import "time"

type AuthStatus string

const (
	AuthApproved         AuthStatus = "approved"
	AuthDeclined         AuthStatus = "declined"
	AuthCleared          AuthStatus = "cleared"
	AuthReversed         AuthStatus = "reversed"
	AuthPartiallyCleared AuthStatus = "partially_cleared"
	AuthExpired          AuthStatus = "expired"
)

// CardAuthorization tracks the full lifecycle of a card authorization
// from the ISO 8583 dual-message system (Authorization → Clearing).
type CardAuthorization struct {
	ID                        string     `json:"id"`
	CardID                    string     `json:"cardId"`
	RetrievalReferenceNumber  string     `json:"rrn"`
	STAN                      string     `json:"stan"`
	AuthCode                  *string    `json:"authCode,omitempty"`
	MerchantName              *string    `json:"merchantName,omitempty"`
	MerchantID                *string    `json:"merchantId,omitempty"`
	MerchantCategoryCode      *string    `json:"mcc,omitempty"`
	TerminalID                *string    `json:"terminalId,omitempty"`
	AcquiringInstitution      *string    `json:"acquiringInstitution,omitempty"`
	AuthAmountCents           int64      `json:"authAmountCents"`
	SettlementAmountCents     *int64     `json:"settlementAmountCents,omitempty"`
	Currency                  string     `json:"currency"`
	MerchantCurrency          *string    `json:"merchantCurrency,omitempty"`
	FXRateApplied             *float64   `json:"fxRateApplied,omitempty"`
	FXFromCurrency            *string    `json:"fxFromCurrency,omitempty"`
	FXFromAmountCents         *int64     `json:"fxFromAmountCents,omitempty"`
	Status                    AuthStatus `json:"status"`
	DeclineReason             *string    `json:"declineReason,omitempty"`
	ResponseCode              *string    `json:"responseCode,omitempty"`
	LedgerHoldID              *string    `json:"ledgerHoldId,omitempty"`
	AuthorizedAt              time.Time  `json:"authorizedAt"`
	SettledAt                 *time.Time `json:"settledAt,omitempty"`
	ReversedAt                *time.Time `json:"reversedAt,omitempty"`
	ExpiresAt                 time.Time  `json:"expiresAt"`
	CreatedAt                 time.Time  `json:"createdAt"`
	UpdatedAt                 time.Time  `json:"updatedAt"`
}
