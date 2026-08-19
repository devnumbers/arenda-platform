//go:build integration

package application_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// TestAttemptWindowRace_ConcurrentIncrementsAreNotLost exercises the full
// failed-login path: N goroutines call VerifyCode with an invalid code for the
// same phone at once, and the persisted failure count must equal N — no
// increment lost. This guards the end-to-end orchestrator path (VerifyCode →
// recordFailedLogin → RecordFailure → Save).
//
// Note: this path always acquires a FOR UPDATE lock inside RecordFailure, so it
// proves the domain+repository cooperate correctly under concurrency but does
// NOT by itself prove the SQL increment is atomic without the lock. That narrower
// property is covered by TestAttemptWindowRace_SaveWithoutLockIsAtomic.
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
	errs := make(chan error, concurrentAttempts)
	var wg sync.WaitGroup
	for range concurrentAttempts {
		wg.Go(func() {
			<-start
			// An invalid code always yields ErrLoginCodeInvalid (or
			// ErrTooManyAttempts near the threshold), both of which record a
			// failure via recordFailedLogin; anything else would break the
			// count assertion below.
			_, _, err := h.auth.VerifyCode(ctx, phone, &email, "000000")
			errs <- err
		})
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if !errors.Is(err, domain.ErrLoginCodeInvalid) && !errors.Is(err, domain.ErrTooManyAttempts) {
			t.Fatalf("racing VerifyCode error = %v, want ErrLoginCodeInvalid or ErrTooManyAttempts", err)
		}
	}

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

// TestAttemptWindowRace_SaveWithoutLockIsAtomic is the precise regression guard
// for the SQL atomicity (issue #215). It calls AttemptRepository.Save directly
// with delta=1 from N goroutines, each on the shared pool connection (no
// transaction, no FOR UPDATE lock). This is exactly the scenario the spec warns
// about: "любой будущий caller Save без предшествующего GetForUpdate вернёт
// lost-update тихо". The old UpsertLoginAttempt form
// (SET failures = EXCLUDED.failures) would lose increments here whenever two
// auto-commit upserts raced on the same row; the current IncrementLoginAttempt
// form (SET failures = login_attempts.failures + EXCLUDED.failures) survives
// because ON CONFLICT serializes the row write and the increment is computed
// against the locked existing value.
//
// The first call lands on the INSERT branch (row absent) and seeds failures=1;
// the remaining N-1 calls hit ON CONFLICT and each add 1 atomically.
func TestAttemptWindowRace_SaveWithoutLockIsAtomic(t *testing.T) {
	const concurrentAttempts = 10

	h := newIntegrationHarness(t)
	phone := mustPhone(t, "+79160000201")
	ctx := h.ctx()
	now := h.clock.Now()

	// Each goroutine saves a single-failure window with delta=1, racing on the
	// same phone with no preceding GetForUpdate and no shared transaction.
	start := make(chan struct{})
	errs := make(chan error, concurrentAttempts)
	var wg sync.WaitGroup
	for range concurrentAttempts {
		wg.Go(func() {
			<-start
			errs <- h.attempts.Save(ctx, phone, uuid.Nil, domain.AttemptWindow{
				FirstFailureAt: now,
				LastFailureAt:  now,
			}, 1)
		})
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("racing Save error = %v, want nil", err)
		}
	}

	got := h.countFailedAttempts(t, phone)
	if got != concurrentAttempts {
		t.Fatalf("failed attempts after %d unlocked concurrent Save(delta=1) = %d, want %d "+
			"(SQL increment is not atomic — lost update detected)",
			concurrentAttempts, got, concurrentAttempts)
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
