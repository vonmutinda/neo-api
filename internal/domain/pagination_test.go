package domain

import "testing"

func TestNormalizePagination(t *testing.T) {
	tests := []struct {
		name       string
		limit      int
		offset     int
		wantLimit  int
		wantOffset int
	}{
		{"defaults", 0, 0, 20, 0},
		{"negative limit", -5, 0, 20, 0},
		{"over max", 200, 0, 100, 0},
		{"valid", 50, 10, 50, 10},
		{"negative offset", 20, -5, 20, 0},
		{"limit 101 clamped to 100", 101, 0, 100, 0},
		{"offset -100 clamped to 0", 20, -100, 20, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotL, gotO := NormalizePagination(tt.limit, tt.offset)
			if gotL != tt.wantLimit || gotO != tt.wantOffset {
				t.Errorf("NormalizePagination(%d, %d) = (%d, %d), want (%d, %d)",
					tt.limit, tt.offset, gotL, gotO, tt.wantLimit, tt.wantOffset)
			}
		})
	}
}

func TestNewPaginatedResult(t *testing.T) {
	data := []string{"a", "b", "c"}
	result := NewPaginatedResult(data, 10, 3, 0)

	if len(result.Data) != 3 {
		t.Errorf("data length = %d, want 3", len(result.Data))
	}
	if result.Pagination.Total != 10 {
		t.Errorf("total = %d, want 10", result.Pagination.Total)
	}
	if !result.Pagination.HasMore {
		t.Error("hasMore should be true when offset+limit < total")
	}

	result2 := NewPaginatedResult(data, 3, 3, 0)
	if result2.Pagination.HasMore {
		t.Error("hasMore should be false when offset+limit >= total")
	}
}
