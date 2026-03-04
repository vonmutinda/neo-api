package domain

import "time"

type ExceptionType string

const (
	ExceptionMissingInLedger    ExceptionType = "missing_in_ledger"
	ExceptionMissingInEthSwitch ExceptionType = "missing_in_ethswitch"
	ExceptionAmountMismatch     ExceptionType = "amount_mismatch"
	ExceptionStatusMismatch     ExceptionType = "status_mismatch"
	ExceptionDuplicateReference ExceptionType = "duplicate_reference"
	ExceptionOrphanedHold       ExceptionType = "orphaned_hold"
)

type ExceptionStatus string

const (
	ExceptionOpen          ExceptionStatus = "open"
	ExceptionInvestigating ExceptionStatus = "investigating"
	ExceptionResolved      ExceptionStatus = "resolved"
	ExceptionEscalated     ExceptionStatus = "escalated"
)

// ReconException represents a single discrepancy found during EOD reconciliation.
type ReconException struct {
	ID                          string          `json:"id"`
	EthSwitchReference          string          `json:"ethswitchReference"`
	IdempotencyKey              *string         `json:"idempotencyKey,omitempty"`
	ErrorType                   ExceptionType   `json:"errorType"`
	EthSwitchReportedAmountCents int64          `json:"ethswitchReportedAmountCents"`
	LedgerReportedAmountCents   *int64          `json:"ledgerReportedAmountCents,omitempty"`
	PostgresReportedAmountCents *int64          `json:"postgresReportedAmountCents,omitempty"`
	AmountDifferenceCents       *int64          `json:"amountDifferenceCents,omitempty"`
	Status                      ExceptionStatus `json:"status"`
	AssignedTo                  *string         `json:"assignedTo,omitempty"`
	ResolutionNotes             *string         `json:"resolutionNotes,omitempty"`
	ResolutionAction            *string         `json:"resolutionAction,omitempty"`
	ReconRunDate                time.Time       `json:"reconRunDate"`
	ClearingFileName            *string         `json:"clearingFileName,omitempty"`
	CreatedAt                   time.Time       `json:"createdAt"`
	ResolvedAt                  *time.Time      `json:"resolvedAt,omitempty"`
	UpdatedAt                   time.Time       `json:"updatedAt"`
}

// ReconRun tracks a single EOD reconciliation execution.
type ReconRun struct {
	ID               string     `json:"id"`
	RunDate          time.Time  `json:"runDate"`
	ClearingFileName string     `json:"clearingFileName"`
	TotalRecords     int        `json:"totalRecords"`
	MatchedCount     int        `json:"matchedCount"`
	ExceptionCount   int        `json:"exceptionCount"`
	StartedAt        time.Time  `json:"startedAt"`
	FinishedAt       *time.Time `json:"finishedAt,omitempty"`
	Status           string     `json:"status"`
	ErrorMessage     *string    `json:"errorMessage,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}
