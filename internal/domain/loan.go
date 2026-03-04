package domain

import "time"

type LoanStatus string

const (
	LoanActive     LoanStatus = "active"
	LoanInArrears  LoanStatus = "in_arrears"
	LoanDefaulted  LoanStatus = "defaulted"
	LoanRepaid     LoanStatus = "repaid"
	LoanWrittenOff LoanStatus = "written_off"
)

// Loan represents an active micro-credit contract.
type Loan struct {
	ID                   string     `json:"id"`
	UserID               string     `json:"userId"`
	PrincipalAmountCents int64      `json:"principalAmountCents"`
	InterestFeeCents     int64      `json:"interestFeeCents"`
	TotalDueCents        int64      `json:"totalDueCents"`
	TotalPaidCents       int64      `json:"totalPaidCents"`
	DurationDays         int        `json:"durationDays"`
	DisbursedAt          time.Time  `json:"disbursedAt"`
	DueDate              time.Time  `json:"dueDate"`
	Status               LoanStatus `json:"status"`
	DaysPastDue          int        `json:"daysPastDue"`
	LedgerLoanAccount    string     `json:"ledgerLoanAccount"`
	LedgerDisbursementTx *string    `json:"ledgerDisbursementTx,omitempty"`
	IdempotencyKey       *string    `json:"idempotencyKey,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

// RemainingCents returns how much the borrower still owes.
func (l *Loan) RemainingCents() int64 {
	return l.TotalDueCents - l.TotalPaidCents
}

// LoanInstallment represents a single scheduled repayment.
type LoanInstallment struct {
	ID                string     `json:"id"`
	LoanID            string     `json:"loanId"`
	InstallmentNumber int        `json:"installmentNumber"`
	AmountDueCents    int64      `json:"amountDueCents"`
	AmountPaidCents   int64      `json:"amountPaidCents"`
	DueDate           time.Time  `json:"dueDate"`
	IsPaid            bool       `json:"isPaid"`
	PaidAt            *time.Time `json:"paidAt,omitempty"`
	LedgerRepaymentTx *string   `json:"ledgerRepaymentTx,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}
