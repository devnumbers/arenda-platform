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
	if b.Factor <= 1 {
		return b.min(b.Base)
	}

	d := float64(b.Base) * math.Pow(b.Factor, float64(attempt-1))
	return b.min(time.Duration(d))
}

func (b *ExponentialBackoff) min(d time.Duration) time.Duration {
	if b.Max > 0 && d > b.Max {
		return b.Max
	}
	return d
}
