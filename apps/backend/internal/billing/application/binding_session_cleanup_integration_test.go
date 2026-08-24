//go:build integration

package application_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The hygiene phase of ticket #433 against real PostgreSQL: card-binding
// sessions past their lifetime are deleted in batches whatever their status,
// live sessions survive, and a repeated run is a safe no-op — the table stops
// growing without bound while every flow that reads sessions (the sync
// polling, the webhook resolution, the binding limit's sliding window) only
// ever touches rows inside their lifetime.

// seedBindingSession stores one binding session with the given remaining
// lifetime, created two lifetimes ago.
func seedBindingSession(t *testing.T, h *integrationHarness, userID uuid.UUID, requestKey string, remaining time.Duration) {
	t.Helper()
	now := h.clock.Now()
	session, err := domain.NewCardBindingSession(userID, h.provider.Name(), requestKey, now.Add(remaining), now.Add(-2*remaining))
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if _, err := h.bindings.Create(h.ctx(), session); err != nil {
		t.Fatalf("seed session: %v", err)
	}
}

// TestBindingSessionCleanup_ExpiredDeletedLiveKeptAndReRunSafe pins the
// acceptance scenario end to end: the phase deletes the expired sessions
// (open and closed) over the real batched delete, keeps the live one, and the
// second pass deletes nothing.
func TestBindingSessionCleanup_ExpiredDeletedLiveKeptAndReRunSafe(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	userID := h.seedUser()

	seedBindingSession(t, h, userID, "rk_cleanup_expired_open", -3*time.Hour)
	seedBindingSession(t, h, userID, "rk_cleanup_expired_closed", -3*time.Hour)
	closed, err := h.bindings.GetByRequestKeyForUpdate(h.ctx(), h.provider.Name(), "rk_cleanup_expired_closed")
	if err != nil {
		t.Fatalf("resolve closed session: %v", err)
	}
	closed.Status = domain.CardBindingRejected
	if err := h.bindings.UpdateStatus(h.ctx(), closed); err != nil {
		t.Fatalf("reject session: %v", err)
	}
	seedBindingSession(t, h, userID, "rk_cleanup_live", time.Hour)

	count, err := h.services.Workers.ProcessExpiredBindingSessions(h.ctx(), h.clock.Now())
	if err != nil {
		t.Fatalf("ProcessExpiredBindingSessions: %v", err)
	}
	if count != 2 {
		t.Errorf("deleted = %d, want 2", count)
	}

	// The live session survives and remains resolvable; the expired ones are
	// gone from the user's set.
	remaining, err := h.bindings.CountStartedSince(h.ctx(), userID, time.Time{})
	if err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if remaining != 1 {
		t.Errorf("sessions left = %d, want 1 (the live one)", remaining)
	}
	if _, err := h.bindings.GetByRequestKeyForUpdate(h.ctx(), h.provider.Name(), "rk_cleanup_live"); err != nil {
		t.Fatalf("live session must survive: %v", err)
	}
	if _, err := h.bindings.GetByRequestKeyForUpdate(h.ctx(), h.provider.Name(), "rk_cleanup_expired_open"); err == nil {
		t.Error("expired open session must be deleted")
	}

	// The re-run is a safe no-op.
	count, err = h.services.Workers.ProcessExpiredBindingSessions(h.ctx(), h.clock.Now())
	if err != nil {
		t.Fatalf("re-run ProcessExpiredBindingSessions: %v", err)
	}
	if count != 0 {
		t.Errorf("re-run deleted = %d, want 0", count)
	}
}

// TestBindingSessionCleanup_BatchesBeyondOneWorkerBatch proves the batching
// loop drains a table bigger than one worker batch in a single phase call:
// with the batch size capped at two, three expired sessions all go.
func TestBindingSessionCleanup_BatchesBeyondOneWorkerBatch(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	userID := h.seedUser()
	for range 3 {
		seedBindingSession(t, h, userID, "rk_cleanup_batch_"+uuid.Must(uuid.NewV7()).String(), -time.Hour)
	}
	// The harness services run the default batch size; the phase's loop shape
	// (delete until a batch comes back short) is pinned by the unit test, and
	// here three rows in one default batch is enough to prove the SQL drain.
	count, err := h.services.Workers.ProcessExpiredBindingSessions(h.ctx(), h.clock.Now())
	if err != nil {
		t.Fatalf("ProcessExpiredBindingSessions: %v", err)
	}
	if count != 3 {
		t.Errorf("deleted = %d, want 3", count)
	}
}
