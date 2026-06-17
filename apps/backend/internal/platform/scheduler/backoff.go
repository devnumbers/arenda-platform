package scheduler

import (
	"math"
	"time"
)

// ExponentialBackoff computes a backoff duration that grows by Factor on each
// attempt, capped at Max.
type ExponentialBackoff struct {
	Base   time.Duration
	Max    time.Duration
	Factor float64
}

// Next returns the backoff duration for the given 1-based attempt number.
func (b *ExponentialBackoff) Next(attempt int) time.Duration {
	if attempt <= 0 {
		return b.Base
	}
	if b.Max <= 0 {
		return b.Base
	}
	if b.Factor <= 1 {
		return minDuration(b.Base, b.Max)
	}

	next := float64(b.Base) * math.Pow(b.Factor, float64(attempt-1))
	if next <= 0 || next > float64(b.Max) || next > float64(math.MaxInt64) {
		return b.Max
	}
	return minDuration(time.Duration(next), b.Max)
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
