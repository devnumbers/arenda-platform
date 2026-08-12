package domain

import (
	"errors"
	"testing"
	"time"
)

func TestAttemptWindow_RecordFailure(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("increments counter and returns nil below the threshold", func(t *testing.T) {
		w := AttemptWindow{}
		now := start

		for i := 1; i < MaxLoginFailures; i++ {
			if err := w.RecordFailure(now); err != nil {
				t.Fatalf("RecordFailure #%d error = %v, want nil", i, err)
			}
			if w.Failures != i {
				t.Fatalf("after #%d: Failures = %d, want %d", i, w.Failures, i)
			}
			if !w.FirstFailureAt.Equal(start) {
				t.Fatalf("FirstFailureAt = %v, want %v", w.FirstFailureAt, start)
			}
			if !w.LastFailureAt.Equal(now) {
				t.Fatalf("LastFailureAt = %v, want %v", w.LastFailureAt, now)
			}
			now = now.Add(time.Minute)
		}
	})

	t.Run("returns ErrTooManyAttempts on the MaxLoginFailures-th failure", func(t *testing.T) {
		w := AttemptWindow{}
		now := start

		for i := 1; i < MaxLoginFailures; i++ {
			_ = w.RecordFailure(now)
		}
		// The 15th failure trips the threshold.
		err := w.RecordFailure(now)
		if !errors.Is(err, ErrTooManyAttempts) {
			t.Fatalf("RecordFailure #%d error = %v, want ErrTooManyAttempts", MaxLoginFailures, err)
		}
		if w.Failures != MaxLoginFailures {
			t.Fatalf("Failures = %d, want %d", w.Failures, MaxLoginFailures)
		}
	})

	t.Run("keeps returning ErrTooManyAttempts within the same window", func(t *testing.T) {
		w := AttemptWindow{}
		now := start

		for range MaxLoginFailures {
			_ = w.RecordFailure(now)
		}
		// A subsequent failure inside the window still trips the threshold.
		err := w.RecordFailure(now.Add(time.Second))
		if !errors.Is(err, ErrTooManyAttempts) {
			t.Fatalf("RecordFailure error = %v, want ErrTooManyAttempts", err)
		}
		if !w.FirstFailureAt.Equal(start) {
			t.Fatalf("FirstFailureAt = %v, want unchanged %v (window must not reset before TTL)", w.FirstFailureAt, start)
		}
	})

	t.Run("resets the window once LoginAttemptWindowTTL has elapsed", func(t *testing.T) {
		w := AttemptWindow{}
		// Fill the window past the threshold.
		for range MaxLoginFailures {
			_ = w.RecordFailure(start)
		}
		// After the TTL, the first failure starts a fresh window.
		afterTTL := start.Add(LoginAttemptWindowTTL)

		err := w.RecordFailure(afterTTL)
		if err != nil {
			t.Fatalf("RecordFailure after TTL error = %v, want nil", err)
		}
		if w.Failures != 1 {
			t.Fatalf("Failures = %d, want 1 after window reset", w.Failures)
		}
		if !w.FirstFailureAt.Equal(afterTTL) {
			t.Fatalf("FirstFailureAt = %v, want %v", w.FirstFailureAt, afterTTL)
		}
		if !w.LastFailureAt.Equal(afterTTL) {
			t.Fatalf("LastFailureAt = %v, want %v", w.LastFailureAt, afterTTL)
		}
	})
}

func TestAttemptWindow_Blocked(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("not blocked below the threshold", func(t *testing.T) {
		w := AttemptWindow{Failures: MaxLoginFailures - 1, FirstFailureAt: start}
		if w.Blocked(start.Add(time.Minute)) {
			t.Fatal("Blocked = true, want false below MaxLoginFailures")
		}
	})

	t.Run("blocked until the window expires", func(t *testing.T) {
		w := AttemptWindow{Failures: MaxLoginFailures, FirstFailureAt: start}
		if !w.Blocked(start.Add(LoginAttemptWindowTTL - time.Second)) {
			t.Fatal("Blocked = false, want true inside the window")
		}
	})

	t.Run("unblocked after the window expires", func(t *testing.T) {
		w := AttemptWindow{Failures: MaxLoginFailures, FirstFailureAt: start}
		if w.Blocked(start.Add(LoginAttemptWindowTTL)) {
			t.Fatal("Blocked = true at window expiry, want false")
		}
		if w.Blocked(start.Add(LoginAttemptWindowTTL + time.Second)) {
			t.Fatal("Blocked = true after the window, want false")
		}
	})

	t.Run("zero failures never blocks", func(t *testing.T) {
		w := AttemptWindow{}
		if w.Blocked(start) {
			t.Fatal("Blocked = true for an empty window, want false")
		}
	})
}
