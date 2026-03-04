package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreditProfile_IsEligibleForLoan(t *testing.T) {
	tests := []struct {
		name     string
		profile  CreditProfile
		expected bool
	}{
		{
			name:     "eligible",
			profile:  CreditProfile{TrustScore: 700, ApprovedLimitCents: 500000, IsNBEBlacklisted: false},
			expected: true,
		},
		{
			name:     "score too low",
			profile:  CreditProfile{TrustScore: 500, ApprovedLimitCents: 500000, IsNBEBlacklisted: false},
			expected: false,
		},
		{
			name:     "zero limit",
			profile:  CreditProfile{TrustScore: 800, ApprovedLimitCents: 0, IsNBEBlacklisted: false},
			expected: false,
		},
		{
			name:     "blacklisted",
			profile:  CreditProfile{TrustScore: 900, ApprovedLimitCents: 1000000, IsNBEBlacklisted: true},
			expected: false,
		},
		{
			name:     "boundary score 600",
			profile:  CreditProfile{TrustScore: 600, ApprovedLimitCents: 100000, IsNBEBlacklisted: false},
			expected: false,
		},
		{
			name:     "boundary score 601",
			profile:  CreditProfile{TrustScore: 601, ApprovedLimitCents: 100000, IsNBEBlacklisted: false},
			expected: true,
		},
		{
			name:     "IsNBEBlacklisted overrides high score and limit",
			profile:  CreditProfile{TrustScore: 999, ApprovedLimitCents: 1000000, IsNBEBlacklisted: true},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.profile.IsEligibleForLoan()
			assert.Equal(t, tt.expected, got, "IsEligibleForLoan()")
		})
	}
}

func TestCreditProfile_AvailableBorrowingCents(t *testing.T) {
	tests := []struct {
		name     string
		profile  CreditProfile
		expected int64
	}{
		{
			name:     "nothing outstanding",
			profile:  CreditProfile{ApprovedLimitCents: 5000000, CurrentOutstandingCents: 0},
			expected: 5000000,
		},
		{
			name:     "partially utilized",
			profile:  CreditProfile{ApprovedLimitCents: 5000000, CurrentOutstandingCents: 2000000},
			expected: 3000000,
		},
		{
			name:     "fully utilized",
			profile:  CreditProfile{ApprovedLimitCents: 5000000, CurrentOutstandingCents: 5000000},
			expected: 0,
		},
		{
			name:     "over-utilized (shouldn't happen but safe)",
			profile:  CreditProfile{ApprovedLimitCents: 5000000, CurrentOutstandingCents: 6000000},
			expected: 0,
		},
		{
			name:     "outstanding exceeds limit returns 0",
			profile:  CreditProfile{ApprovedLimitCents: 100000, CurrentOutstandingCents: 150000},
			expected: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.profile.AvailableBorrowingCents()
			assert.Equal(t, tt.expected, got, "AvailableBorrowingCents()")
		})
	}
}
