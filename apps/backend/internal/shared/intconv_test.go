package shared

import (
	"math"
	"testing"
)

// TestToInt32Clamped verifies the int→int32 seam for sqlc parameters
// (issue #341): in-range values pass through unchanged, out-of-range values
// saturate at the nearest bound instead of wrapping around.
func TestToInt32Clamped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   int
		want int32
	}{
		{"zero", 0, 0},
		{"positive", 42, 42},
		{"negative", -42, -42},
		{"max int32", math.MaxInt32, math.MaxInt32},
		{"min int32", math.MinInt32, math.MinInt32},
		{"above max clamps", math.MaxInt32 + 1, math.MaxInt32},
		{"below min clamps", math.MinInt32 - 1, math.MinInt32},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ToInt32Clamped(tt.in); got != tt.want {
				t.Errorf("ToInt32Clamped(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
