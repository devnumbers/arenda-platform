//go:build integration

package application_test

import (
	"sync"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// TestAttemptWindowRace_ConcurrentIncrementsAreNotLost proves the atomic
// database-side increment survives concurrent failed-login attempts: N goroutines
// call VerifyCode with an invalid code for the same phone at once, and the
// persisted failure count must equal N — no increment lost to a lost update.
//
// This is the regression guard for issue #215. The old UpsertLoginAttempt form
// (SET failures = EXCLUDED.failures) would lose increments whenever two
// transactions read-modify-wrote the row without serializing; the current
// IncrementLoginAttempt form (SET failures = login_attempts.failures +
// EXCLUDED.failures) makes the increment atomic at the data layer.
//
// N stays below MaxLoginFailures so checkNotBlocked does not short-circuit any
// caller before it reaches the increment path.
func TestAttemptWindowRace_ConcurrentIncrementsAreNotLost(t *testing.T) {
	const concurrentAttempts = 10 // < domain.MaxLoginFailures (15)

	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000200")
	email := mustEmail(t, "race@example.com")
	h.seedVerifiedUser(t, phone, email)
	ctx := h.ctx()

	// Issue one live code so Verify reaches the invalid-code branch and the
	// orchestrator records a failure for each attempt.
	if err := h.auth.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode: %v", err)
	}

	// A barrier so every goroutine races VerifyCode at the same instant.
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range concurrentAttempts {
		wg.Go(func() {
			<-start
			// An invalid code always yields ErrLoginCodeInvalid (or
			// ErrTooManyAttempts near the threshold), both of which record a
			// failure via recordFailedLogin. We ignore the per-goroutine error;
			// the assertion is on the final persisted count.
			_, _, _ = h.auth.VerifyCode(ctx, phone, &email, "000000")
		})
	}
	close(start)
	wg.Wait()

	// Every concurrent increment must have been persisted: no lost update.
	got := h.countFailedAttempts(t, phone)
	if got != concurrentAttempts {
		t.Fatalf("failed attempts after %d concurrent VerifyCode = %d, want %d (lost update detected)",
			concurrentAttempts, got, concurrentAttempts)
	}

	// Sanity: each attempt produced a failed-login audit entry through the real
	// UoW, proving the increment path completed its transaction.
	if n := h.auditCount(t, "auth.login_failed"); n != concurrentAttempts {
		t.Fatalf("auth.login_failed audit entries = %d, want %d", n, concurrentAttempts)
	}
}

// auditCount returns the number of audit_log rows for the given action. It is a
// race-test companion to auditActionExists that needs the exact count.
func (h *integrationHarness) auditCount(t *testing.T, action string) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(h.ctx(),
		"SELECT count(*) FROM audit_log WHERE action = $1", action,
	).Scan(&n); err != nil {
		t.Fatalf("count audit %q: %v", action, err)
	}
	return n
}
