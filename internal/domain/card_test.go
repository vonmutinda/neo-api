package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCard_IsActive(t *testing.T) {
	tests := []struct {
		name   string
		status CardStatus
		want   bool
	}{
		{"active", CardStatusActive, true},
		{"frozen", CardStatusFrozen, false},
		{"cancelled", CardStatusCancelled, false},
		{"expired", CardStatusExpired, false},
		{"pending_activation", CardStatusPendingActivation, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			card := &Card{Status: tt.status}
			got := card.IsActive()
			assert.Equal(t, tt.want, got)
		})
	}
}
