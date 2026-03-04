package ethswitch

import "time"

// TransferRequest represents the payload sent to EthioPay-IPS.
type TransferRequest struct {
	IdempotencyKey     string    `json:"idempotency_key"`
	SourceInstitution  string    `json:"source_institution"`
	DestInstitution    string    `json:"dest_institution"`
	SourceAccount      string    `json:"source_account"`
	DestinationAccount string    `json:"destination_account"`
	AmountCents        int64     `json:"amount_cents"`
	Currency           string    `json:"currency"`
	Narration          string    `json:"narration"`
	Timestamp          time.Time `json:"timestamp"`
}

// TransferResponse represents EthSwitch's reply.
type TransferResponse struct {
	EthSwitchReference string    `json:"ethswitch_reference"`
	Status             string    `json:"status"` // SUCCESS, FAILED, PENDING_ASYNC
	ErrorCode          string    `json:"error_code,omitempty"`
	ErrorMessage       string    `json:"error_message,omitempty"`
	SettlementDate     time.Time `json:"settlement_date"`
}

// StatusCheckResponse is returned when polling a pending transaction.
type StatusCheckResponse struct {
	EthSwitchReference string `json:"ethswitch_reference"`
	Status             string `json:"status"`
	ErrorCode          string `json:"error_code,omitempty"`
	ErrorMessage       string `json:"error_message,omitempty"`
}
