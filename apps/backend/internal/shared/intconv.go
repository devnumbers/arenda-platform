// Package shared holds cross-context guard helpers: small seams every bounded
// context needs without importing another context. The int-conversion clamp
// below is the single sanctioned int→int32 crossing for sqlc parameters.
package shared

import "math"

// ToInt32Clamped converts v to the int32 parameter type of the sqlc queries,
// saturating at the int32 bounds instead of wrapping around. Saturation — not
// an error — is the contract for operational values (pagination, batch sizes,
// counters): a caller past 2^31 gets the extreme page or batch. The bounds
// must stay explicit if-comparisons: gosec G115 recognizes only those, not
// min(v, math.MaxInt32) (research #321, probe).
func ToInt32Clamped(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}
