package domain

import "time"

type OverdraftStatus string

const (
	OverdraftInactive  OverdraftStatus = "inactive"
	OverdraftActive    OverdraftStatus = "active"
	OverdraftUsed      OverdraftStatus = "used"
	OverdraftSuspended OverdraftStatus = "suspended"
)

// Overdraft represents a user's ETB overdraft facility.
type Overdraft struct {
	ID                  string          `json:"id"`
	UserID              string          `json:"userId"`
	LimitCents          int64           `json:"limitCents"`
	UsedCents           int64           `json:"usedCents"`
	AvailableCents      int64           `json:"availableCents"`
	DailyFeeBasisPoints int             `json:"dailyFeeBasisPoints"`
	InterestFreeDays    int             `json:"interestFreeDays"`
	AccruedFeeCents     int64           `json:"accruedFeeCents"`
	Status              OverdraftStatus `json:"status"`
	OverdrawnSince      *time.Time      `json:"overdrawnSince,omitempty"`
	LastFeeAccrualAt    *time.Time      `json:"lastFeeAccrualAt,omitempty"`
	OptedInAt           *time.Time      `json:"optedInAt,omitempty"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
}
