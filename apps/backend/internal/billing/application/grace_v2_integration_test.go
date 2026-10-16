//go:build integration

package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The grace v2 integration scenarios (ADR 0055, issue #615): the snapshot ids
// a grace entry archives round-trip through the real uuid[] column, every
// applied payment restores them through the bridge in the success
// transaction, and a bridge failure rolls the whole application back.

// TestWorkers_Integration_GraceEntrySnapshotsArchivedProperties proves the
// entry keeps one active property on the real schema: the bridge archives at
// the grace limit inside the failed-charge transaction, and the archived ids
// land on the subscription row in restoration-priority order.
func TestWorkers_Integration_GraceEntrySnapshotsArchivedProperties(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	archiver, slots := wireBridges(h)
	archived := []uuid.UUID{uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())}
	archiver.archiveIDs = archived

	userID, sub := seedExpiredProSubscription(t, h, time.Hour)
	seedActiveMethod(t, h, userID, "fake_fail_card")

	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	requireGraceEnteredAfterFailedCharge(t, h, userID, sub.ID)

	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	got := stored.GraceArchivedPropertyIDs
	if len(got) != 2 || got[0] != archived[0] || got[1] != archived[1] {
		t.Errorf("grace snapshot = %v, want %v (order preserved)", got, archived)
	}
	if got := archiver.recorded(); len(got) != 1 || got[0].limit != 1 || got[0].ownerID != userID {
		t.Errorf("archive calls = %+v, want one at the grace limit", got)
	}
	if got := slots.recorded(); len(got) != 1 || got[0] != "grace_entry" {
		t.Errorf("slot calls = %v, want one grace_entry", got)
	}
}

// TestWorkers_Integration_GraceRetryRestoresSnapshot proves the restoration on
// the real schema: the +24 h retry succeeds, the applied payment restores the
// snapshotted ids through the bridge, and the snapshot column clears with the
// same commit.
func TestWorkers_Integration_GraceRetryRestoresSnapshot(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	archiver, slots := wireBridges(h)
	h.paymentsSvc.SetLifecycleBridges(archiver, slots)

	userID, sub := seedExpiredProSubscription(t, h, time.Hour)
	seedActiveMethod(t, h, userID, "fake_fail_card")
	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	requireGraceEnteredAfterFailedCharge(t, h, userID, sub.ID)

	snapshot := []uuid.UUID{uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	stored.SetGraceArchive(snapshot)
	if err := h.subscriptions.Update(h.ctx(), stored); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	pinGraceEpisode(t, h, sub.ID, h.clock.Now().Add(-25*time.Hour))
	seedActiveMethod(t, h, userID, "tok_retry_ok")
	if count, err := h.services.Workers.ProcessGraceRetries(h.ctx(), h.clock.Now()); err != nil || count != 1 {
		t.Fatalf("ProcessGraceRetries() = %d (err %v), want 1", count, err)
	}

	stored, err = h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("re-read subscription: %v", err)
	}
	if stored.Status != domain.SubscriptionStatusActive {
		t.Fatalf("status = %q, want active after the retry", stored.Status)
	}
	if len(stored.GraceArchivedPropertyIDs) != 0 {
		t.Errorf("grace snapshot = %v, want cleared after the restoration", stored.GraceArchivedPropertyIDs)
	}
	calls := archiver.restoreCalls()
	if len(calls) != 1 || calls[0].ownerID != userID || calls[0].limit != 5 || len(calls[0].ids) != 2 {
		t.Fatalf("restore calls = %+v, want the snapshot at the pro limit", calls)
	}
	for i, id := range snapshot {
		if calls[0].ids[i] != id {
			t.Errorf("restored ids = %v, want %v in order", calls[0].ids, snapshot)
		}
	}
}

// TestWorkers_Integration_RestoreBridgeFailureRollsBackApplication proves the
// rollback on the real schema: a restore bridge failure fails the success
// application transaction, so the payment stays pending and the subscription
// keeps its grace window and snapshot for the next tick.
func TestWorkers_Integration_RestoreBridgeFailureRollsBackApplication(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	archiver, slots := wireBridges(h)
	h.paymentsSvc.SetLifecycleBridges(archiver, slots)

	userID, sub := seedExpiredProSubscription(t, h, time.Hour)
	seedActiveMethod(t, h, userID, "fake_fail_card")
	if _, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	graceEnd := requireGraceEnteredAfterFailedCharge(t, h, userID, sub.ID)

	snapshot := []uuid.UUID{uuid.Must(uuid.NewV7())}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	stored.SetGraceArchive(snapshot)
	if err := h.subscriptions.Update(h.ctx(), stored); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	// The retry charge succeeds at the provider, but the restore bridge fails:
	// nothing may stick. The bridge breaks from here on — the entry above
	// already ran with a healthy bridge. The phase logs the failure and
	// reports no progress; the rollback assertions below are the point.
	archiver.mu.Lock()
	archiver.err = errors.New("bridge down")
	archiver.mu.Unlock()
	pinGraceEpisode(t, h, sub.ID, h.clock.Now().Add(-25*time.Hour))
	seedActiveMethod(t, h, userID, "tok_retry_ok") // The charge succeeds; the restore bridge below fails.
	if count, err := h.services.Workers.ProcessGraceRetries(h.ctx(), h.clock.Now()); err != nil {
		t.Fatalf("ProcessGraceRetries() error = %v", err)
	} else if count != 0 {
		t.Fatalf("ProcessGraceRetries() = %d, want 0 (the failed application is not progress)", count)
	}

	stored, err = h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("re-read subscription: %v", err)
	}
	if stored.Status != domain.SubscriptionStatusGrace || stored.ValidUntil == nil || !stored.ValidUntil.Equal(graceEnd) {
		t.Errorf("subscription = %s until %v, want the untouched grace window", stored.Status, stored.ValidUntil)
	}
	if len(stored.GraceArchivedPropertyIDs) != 1 || stored.GraceArchivedPropertyIDs[0] != snapshot[0] {
		t.Errorf("grace snapshot = %v, want intact after the rollback", stored.GraceArchivedPropertyIDs)
	}
	// The failed renewal and the pending retry: the retry payment stays
	// pending for the next tick to finalize.
	pending, err := h.payments.ListPendingByUserID(h.ctx(), userID)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending payments = %d (err %v), want the retry payment pending", len(pending), err)
	}
}
