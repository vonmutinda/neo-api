package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoan_RemainingCents(t *testing.T) {
	tests := []struct {
		name           string
		totalDueCents  int64
		totalPaidCents int64
		want           int64
	}{
		{"full remaining", 100000, 0, 100000},
		{"partial paid", 100000, 35000, 65000},
		{"fully paid", 100000, 100000, 0},
		{"overpaid", 100000, 120000, -20000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			loan := &Loan{TotalDueCents: tt.totalDueCents, TotalPaidCents: tt.totalPaidCents}
			got := loan.RemainingCents()
			assert.Equal(t, tt.want, got)
		})
	}
}
