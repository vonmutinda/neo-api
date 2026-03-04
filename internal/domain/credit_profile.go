package domain

import "time"

// CreditProfile holds the trust-score algorithm's output for a user.
// Recalculated weekly by the lending-worker cron job.
type CreditProfile struct {
	UserID                 string    `json:"userId"`
	TrustScore             int       `json:"trustScore"`
	ApprovedLimitCents     int64     `json:"approvedLimitCents"`
	AvgMonthlyInflowCents  int64     `json:"avgMonthlyInflowCents"`
	AvgMonthlyBalanceCents int64     `json:"avgMonthlyBalanceCents"`
	ActiveDaysPerMonth     int       `json:"activeDaysPerMonth"`
	TotalLoansRepaid       int       `json:"totalLoansRepaid"`
	LatePaymentsCount      int       `json:"latePaymentsCount"`
	CurrentOutstandingCents int64    `json:"currentOutstandingCents"`
	IsNBEBlacklisted       bool      `json:"isNbeBlacklisted"`
	BlacklistCheckedAt     *time.Time `json:"blacklistCheckedAt,omitempty"`
	LastCalculatedAt       time.Time `json:"lastCalculatedAt"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

// IsEligibleForLoan returns true if the user meets the minimum threshold
// and has no outstanding loans.
func (cp *CreditProfile) IsEligibleForLoan() bool {
	return cp.TrustScore > 600 &&
		cp.ApprovedLimitCents > 0 &&
		cp.CurrentOutstandingCents == 0 &&
		!cp.IsNBEBlacklisted
}

// AvailableBorrowingCents returns how much the user can still borrow,
// accounting for any outstanding loans against the approved limit.
func (cp *CreditProfile) AvailableBorrowingCents() int64 {
	available := cp.ApprovedLimitCents - cp.CurrentOutstandingCents
	if available < 0 {
		return 0
	}
	return available
}
