package domain

import (
	"encoding/json"
	"time"

	"github.com/vonmutinda/neo/pkg/phone"
)

type ReceiptType string

const (
	ReceiptP2PSend          ReceiptType = "p2p_send"
	ReceiptP2PReceive       ReceiptType = "p2p_receive"
	ReceiptEthSwitchOut     ReceiptType = "ethswitch_out"
	ReceiptEthSwitchIn      ReceiptType = "ethswitch_in"
	ReceiptCardPurchase     ReceiptType = "card_purchase"
	ReceiptCardATM          ReceiptType = "card_atm"
	ReceiptLoanDisbursement ReceiptType = "loan_disbursement"
	ReceiptLoanRepayment    ReceiptType = "loan_repayment"
	ReceiptFee              ReceiptType = "fee"
	ReceiptConvertOut       ReceiptType = "convert_out"
	ReceiptConvertIn        ReceiptType = "convert_in"
	ReceiptBatchSend        ReceiptType = "batch_send"
	ReceiptPotDeposit       ReceiptType = "pot_deposit"
	ReceiptPotWithdraw      ReceiptType = "pot_withdraw"
)

type ReceiptStatus string

const (
	ReceiptPending   ReceiptStatus = "pending"
	ReceiptCompleted ReceiptStatus = "completed"
	ReceiptFailed    ReceiptStatus = "failed"
	ReceiptReversed  ReceiptStatus = "reversed"
)

// TransactionReceipt is a UI-optimized read model for the Next.js frontend.
// This is NOT the source of truth -- Formance is.
type TransactionReceipt struct {
	ID                     string        `json:"id"`
	UserID                 string        `json:"userId"`
	LedgerTransactionID    string        `json:"ledgerTransactionId"`
	EthSwitchReference     *string       `json:"ethswitchReference,omitempty"`
	IdempotencyKey         *string       `json:"idempotencyKey,omitempty"`
	Type                   ReceiptType   `json:"type"`
	Status                 ReceiptStatus `json:"status"`
	AmountCents            int64         `json:"amountCents"`
	Currency               string        `json:"currency"`
	CounterpartyName       *string       `json:"counterpartyName,omitempty"`
	CounterpartyPhone      *phone.PhoneNumber `json:"counterpartyPhone,omitempty"`
	CounterpartyInstitution *string      `json:"counterpartyInstitution,omitempty"`
	Narration              *string          `json:"narration,omitempty"`
	Purpose                *TransferPurpose `json:"purpose,omitempty"`
	BeneficiaryID          *string          `json:"beneficiaryId,omitempty"`
	FeeCents               int64            `json:"feeCents"`
	FeeBreakdown           *json.RawMessage `json:"feeBreakdown,omitempty"`
	Metadata               *json.RawMessage `json:"metadata,omitempty"`
	CreatedAt              time.Time        `json:"createdAt"`
	UpdatedAt              time.Time        `json:"updatedAt"`
}

type BatchSendMetadata struct {
	Recipients []BatchSendRecipient `json:"recipients"`
}

type BatchSendRecipient struct {
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	UserID      string `json:"userId"`
	AmountCents int64  `json:"amountCents"`
	Narration   string `json:"narration"`
}

// InflowOverdraftMetadata is stored in TransactionReceipt.Metadata when an incoming
// ETB flow (P2P receive, batch receive, convert-in) triggered overdraft repayment.
type InflowOverdraftMetadata struct {
	TotalInflowCents         int64 `json:"totalInflowCents"`
	OverdraftRepaymentCents int64 `json:"overdraftRepaymentCents"`
	NetInflowCents          int64 `json:"netInflowCents"`
}

// PotTransferMetadata is stored in TransactionReceipt.Metadata for pot_deposit and pot_withdraw.
type PotTransferMetadata struct {
	PotID   string `json:"potId"`
	PotName string `json:"potName"`
}

// ConvertMetadata is stored on convert_out and convert_in receipts for FX display (from -> to).
// Convert_in receipts may also include overdraft fields when ETB inflow triggered repayment.
type ConvertMetadata struct {
	FromCurrency             string `json:"fromCurrency"`
	ToCurrency               string `json:"toCurrency"`
	TotalInflowCents         int64  `json:"totalInflowCents,omitempty"`
	OverdraftRepaymentCents int64  `json:"overdraftRepaymentCents,omitempty"`
	NetInflowCents          int64  `json:"netInflowCents,omitempty"`
}
