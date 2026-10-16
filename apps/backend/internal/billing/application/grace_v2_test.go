package application

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The grace v2 scenarios (ADR 0055, issue #615): the grace window keeps one
// active property — the entry archives the excess through the properties
// bridge and snapshots the ids on the subscription, every applied payment
// restores exactly that snapshot under the tariff limit, and the fall to
// basic drops the debt with the grace archive left in the archive.

// archiveIDsOf returns fresh property ids the fake archiver reports as
// archived.
func archiveIDsOf(n int) []uuid.UUID {
	ids := make([]uuid.UUID, n)
	for i := range ids {
		ids[i] = mustNewUUID()
	}
	return ids
}

// TestWorkers_GraceEntryArchivesExcessToTheSurvivor proves the failed renewal
// charge keeps one active property: the bridge archives beyond the grace limit
// of one, the recipient slots are enforced with the grace_entry trigger, and
// the archived ids become the subscription's restoration snapshot — in the
// same transaction as the grace transition.
func TestWorkers_GraceEntryArchivesExcessToTheSurvivor(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	archiver := &fakeArchiverSource{archiveIDs: archiveIDsOf(2)}
	slots := &fakeSlotSource{}
	h.setLifecycleBridges(archiver, slots)

	sub := h.seedSubscription(t, nil)
	h.seedActiveMethod(t, sub, testProviderFake, "token_bad")
	h.provider.chargeFn = func(req ChargeRequest) (ChargeResult, error) {
		return ChargeResult{ProviderPaymentID: req.ProviderPaymentID, Status: domain.PaymentStatusFailed}, nil
	}

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Fatalf("status = %q, want grace", stored.Status)
	}
	if got := stored.GraceArchivedPropertyIDs; len(got) != 2 {
		t.Fatalf("grace snapshot = %v, want the two archived ids", got)
	}
	if got := archiver.recorded(); len(got) != 1 || got[0].limit != gracePropertyLimit || got[0].ownerID != sub.UserID {
		t.Errorf("archive calls = %+v, want one at the grace limit for the owner", got)
	}
	if got := slots.recorded(); len(got) != 1 || got[0] != triggerGraceEntry {
		t.Errorf("slot calls = %v, want one %q", got, triggerGraceEntry)
	}
}

// TestWorkers_GraceEntryWithoutChargeableMethodArchives proves the
// no-chargeable-method entry carries the same grace v2 archiving: nothing was
// charged, yet the excess properties are archived and snapshotted.
func TestWorkers_GraceEntryWithoutChargeableMethodArchives(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	archiver := &fakeArchiverSource{archiveIDs: archiveIDsOf(1)}
	slots := &fakeSlotSource{}
	h.setLifecycleBridges(archiver, slots)

	sub := h.seedSubscription(t, nil)

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Fatalf("status = %q, want grace", stored.Status)
	}
	if got := stored.GraceArchivedPropertyIDs; len(got) != 1 {
		t.Fatalf("grace snapshot = %v, want the archived id", got)
	}
	if got := archiver.recorded(); len(got) != 1 || got[0].limit != gracePropertyLimit {
		t.Errorf("archive calls = %+v, want one at the grace limit", got)
	}
	if got := slots.recorded(); len(got) != 1 || got[0] != triggerGraceEntry {
		t.Errorf("slot calls = %v, want one %q", got, triggerGraceEntry)
	}
}

// TestWorkers_GraceEntryWithoutBridgesStillEntersGrace pins the wiring shape
// of issue #252: without the lifecycle bridges the subscription-side change
// still applies, and the empty snapshot owes nothing.
func TestWorkers_GraceEntryWithoutBridgesStillEntersGrace(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)
	h.seedActiveMethod(t, sub, testProviderFake, "token_bad")
	h.provider.chargeFn = func(req ChargeRequest) (ChargeResult, error) {
		return ChargeResult{ProviderPaymentID: req.ProviderPaymentID, Status: domain.PaymentStatusFailed}, nil
	}

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Fatalf("status = %q, want grace", stored.Status)
	}
	if len(stored.GraceArchivedPropertyIDs) != 0 {
		t.Errorf("grace snapshot = %v, want empty without a wired bridge", stored.GraceArchivedPropertyIDs)
	}
}

// TestWorkers_DunningFailureInsideGraceDoesNotReArchive proves an open grace
// window is never re-entered (one window, one archiving): a failed dunning
// retry changes no archive state.
func TestWorkers_DunningFailureInsideGraceDoesNotReArchive(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	archiver := &fakeArchiverSource{archiveIDs: archiveIDsOf(1)}
	slots := &fakeSlotSource{}
	h.setLifecycleBridges(archiver, slots)

	sub, _ := h.seedGraceEpisode(t)
	h.provider.chargeFn = func(req ChargeRequest) (ChargeResult, error) {
		return ChargeResult{ProviderPaymentID: req.ProviderPaymentID, Status: domain.PaymentStatusFailed}, nil
	}

	if _, err := h.workers.ProcessGraceRetries(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessGraceRetries() error = %v", err)
	}

	if got := archiver.recorded(); len(got) != 0 {
		t.Errorf("archive calls = %+v, want none inside an open grace window", got)
	}
	if got := slots.recorded(); len(got) != 0 {
		t.Errorf("slot calls = %v, want none inside an open grace window", got)
	}
	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Errorf("status = %q, want grace (unchanged)", stored.Status)
	}
}

// TestWorkers_GraceRetrySuccessRestoresGraceArchive proves the happy
// restoration path (ADR 0055): the dunning retry succeeds, the applied payment
// restores exactly the snapshotted ids under the tariff limit, and the debt is
// cleared — all in the success-application transaction.
func TestWorkers_GraceRetrySuccessRestoresGraceArchive(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	archiver := &fakeArchiverSource{}
	slots := &fakeSlotSource{}
	h.setLifecycleBridges(archiver, slots)

	sub, _ := h.seedGraceEpisode(t)
	snapshot := archiveIDsOf(3)
	seeded, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("re-read subscription: %v", err)
	}
	seeded.SetGraceArchive(snapshot)
	if err := h.stores.subscriptions.Update(t.Context(), seeded); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	if _, err := h.workers.ProcessGraceRetries(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessGraceRetries() error = %v", err)
	}

	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusActive {
		t.Fatalf("status = %q, want active after the retry charge", stored.Status)
	}
	if got := stored.GraceArchivedPropertyIDs; len(got) != 0 {
		t.Errorf("grace snapshot = %v, want cleared after the restoration", got)
	}
	calls := archiver.restoreCalls()
	if len(calls) != 1 {
		t.Fatalf("restore calls = %d, want 1", len(calls))
	}
	if calls[0].ownerID != sub.UserID || calls[0].limit != h.pro.ActivePropertyLimit || len(calls[0].ids) != len(snapshot) {
		t.Errorf("restore call = %+v, want the snapshot at the pro limit", calls[0])
	}
}

// TestWorkers_ExpiredGraceDropsSnapshotWithoutRestore proves the fall to basic
// drops the restoration debt: the grace archive stays archived, nothing is
// restored, and the snapshot is gone from the stored row.
func TestWorkers_ExpiredGraceDropsSnapshotWithoutRestore(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	archiver := &fakeArchiverSource{archiveIDs: archiveIDsOf(1)}
	slots := &fakeSlotSource{}
	h.setLifecycleBridges(archiver, slots)

	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		until := h.now.Add(-time.Hour)
		s.ValidUntil = &until
		s.SetGraceArchive(archiveIDsOf(2))
	})

	if _, err := h.workers.ProcessExpiredGrace(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessExpiredGrace() error = %v", err)
	}

	stored := h.storedSubscription(t, sub)
	if stored.TariffID != h.basic.ID || stored.Status != domain.SubscriptionStatusActive {
		t.Fatalf("subscription = %s/%s, want basic/active after expiry", stored.TariffID, stored.Status)
	}
	if len(stored.GraceArchivedPropertyIDs) != 0 {
		t.Errorf("grace snapshot = %v, want dropped with the fall to basic", stored.GraceArchivedPropertyIDs)
	}
	if got := archiver.restoreCalls(); len(got) != 0 {
		t.Errorf("restore calls = %+v, want none on expiry", got)
	}
}

// TestWorkers_ReenteredGraceOverwritesSnapshot proves the fresh grace window
// owes a fresh restoration: the entry replaces whatever snapshot survived from
// the previous window.
func TestWorkers_ReenteredGraceOverwritesSnapshot(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	fresh := archiveIDsOf(2)
	archiver := &fakeArchiverSource{archiveIDs: fresh}
	slots := &fakeSlotSource{}
	h.setLifecycleBridges(archiver, slots)

	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		s.SetGraceArchive(archiveIDsOf(3)) // Stale ids from a previous window.
	})
	h.seedActiveMethod(t, sub, testProviderFake, "token_bad")
	h.provider.chargeFn = func(req ChargeRequest) (ChargeResult, error) {
		return ChargeResult{ProviderPaymentID: req.ProviderPaymentID, Status: domain.PaymentStatusFailed}, nil
	}

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Fatalf("status = %q, want grace", stored.Status)
	}
	got := stored.GraceArchivedPropertyIDs
	if len(got) != len(fresh) {
		t.Fatalf("grace snapshot length = %d, want the fresh %d", len(got), len(fresh))
	}
	for i := range fresh {
		if got[i] != fresh[i] {
			t.Fatalf("grace snapshot = %v, want the fresh %v", got, fresh)
		}
	}
}

// TestWorkers_GraceEntryWithNothingExcessStoresEmptySnapshot pins the 0/1
// active-properties edge (issue #615): with nothing beyond the grace limit the
// bridge is still consulted, but the snapshot stays empty — nothing is owed.
func TestWorkers_GraceEntryWithNothingExcessStoresEmptySnapshot(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	archiver := &fakeArchiverSource{}
	slots := &fakeSlotSource{}
	h.setLifecycleBridges(archiver, slots)

	sub := h.seedSubscription(t, nil)
	h.seedActiveMethod(t, sub, testProviderFake, "token_bad")
	h.provider.chargeFn = func(req ChargeRequest) (ChargeResult, error) {
		return ChargeResult{ProviderPaymentID: req.ProviderPaymentID, Status: domain.PaymentStatusFailed}, nil
	}

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Fatalf("status = %q, want grace", stored.Status)
	}
	if len(stored.GraceArchivedPropertyIDs) != 0 {
		t.Errorf("grace snapshot = %v, want empty (nothing was archived)", stored.GraceArchivedPropertyIDs)
	}
	if got := archiver.recorded(); len(got) != 1 || got[0].limit != gracePropertyLimit {
		t.Errorf("archive calls = %+v, want the consulted bridge at the grace limit", got)
	}
}

// TestWorkers_AsyncFailedRenewalArchivesExcess proves the webhook-finalized
// failure carries the grace v2 archiving too: the async path lands in the same
// enterGrace seam as the worker's failed charge.
func TestWorkers_AsyncFailedRenewalArchivesExcess(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	archiver := &fakeArchiverSource{archiveIDs: archiveIDsOf(2)}
	slots := &fakeSlotSource{}
	h.setLifecycleBridges(archiver, slots)

	sub, payment := h.seedStaleMitCharge(t)

	if err := h.workers.payments.ApplyPaymentNotification(t.Context(), &PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: *payment.ProviderPaymentID,
		Status:            domain.PaymentStatusFailed,
	}); err != nil {
		t.Fatalf("ApplyPaymentNotification() error = %v", err)
	}

	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Fatalf("status = %q, want grace after the async failed renewal", stored.Status)
	}
	if got := stored.GraceArchivedPropertyIDs; len(got) != 2 {
		t.Fatalf("grace snapshot = %v, want the archived ids", got)
	}
	if got := archiver.recorded(); len(got) != 1 || got[0].limit != gracePropertyLimit {
		t.Errorf("archive calls = %+v, want one at the grace limit", got)
	}
	if got := slots.recorded(); len(got) != 1 || got[0] != triggerGraceEntry {
		t.Errorf("slot calls = %v, want one %q", got, triggerGraceEntry)
	}
}

// TestWorkers_ServiceSubscriptionNeverEntersGrace pins the structural
// guarantee: a service subscription (auto-renew off, no payments) expires
// through the shared basic path and never passes through grace.
func TestWorkers_ServiceSubscriptionNeverEntersGrace(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	slots := &fakeSlotSource{}
	h.workers.SetLifecycleBridges(nil, slots)

	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		s.Source = domain.SubscriptionSourceService
		s.AutoRenewEnabled = false
	})

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusActive || stored.TariffID != h.basic.ID {
		t.Fatalf("subscription = %s/%s, want basic/active (expired service term)", stored.Status, stored.TariffID)
	}
	if got := slots.recorded(); len(got) != 1 || got[0] != triggerNonRenewingExpired {
		t.Errorf("slot calls = %v, want one %q — the expiry path, not grace", got, triggerNonRenewingExpired)
	}
}

// seedGraceSnapshot puts a subscription inside its grace window with the given
// restoration snapshot — the state a manual payment or an upgrade finds.
func (h *paymentHarness) seedGraceSnapshot(t *testing.T, ids []uuid.UUID) domain.Subscription {
	t.Helper()
	sub := h.seedSubscription(t, func(s *domain.Subscription) {
		until := h.now.Add(3 * 24 * time.Hour)
		s.Status = domain.SubscriptionStatusGrace
		s.ValidUntil = &until
		s.SetGraceArchive(ids)
	})
	return sub
}

// TestPayment_ManualGraceRenewalRestoresArchive proves the manual «Оплатить
// тариф» path of grace v2: the succeeded same-tariff payment applies as a
// renewal and restores the grace snapshot under the tariff limit, clearing the
// debt with the same commit.
func TestPayment_ManualGraceRenewalRestoresArchive(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	archiver := &fakeArchiverSource{}
	slots := &fakeSlotSource{}
	h.payments.SetLifecycleBridges(archiver, slots)

	snapshot := archiveIDsOf(2)
	sub := h.seedGraceSnapshot(t, snapshot)

	result, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffPro, Period: domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(grace renewal) error = %v", err)
	}
	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if err := h.webhookSucceeded(t, payment); err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}

	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusActive {
		t.Fatalf("status = %q, want active after the renewal", stored.Status)
	}
	if len(stored.GraceArchivedPropertyIDs) != 0 {
		t.Errorf("grace snapshot = %v, want cleared after the restoration", stored.GraceArchivedPropertyIDs)
	}
	calls := archiver.restoreCalls()
	if len(calls) != 1 {
		t.Fatalf("restore calls = %d, want 1", len(calls))
	}
	if calls[0].ownerID != sub.UserID || calls[0].limit != 5 || len(calls[0].ids) != len(snapshot) {
		t.Errorf("restore call = %+v, want the snapshot at the pro limit", calls[0])
	}
}

// TestPayment_UpgradeRestoresArchiveUnderNewLimit proves the upgrade path of
// grace v2: the applied payment restores the grace snapshot respecting the new
// plan's limit — here the unlimited business limit passed through.
func TestPayment_UpgradeRestoresArchiveUnderNewLimit(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	archiver := &fakeArchiverSource{}
	slots := &fakeSlotSource{}
	h.payments.SetLifecycleBridges(archiver, slots)

	sub := h.seedGraceSnapshot(t, archiveIDsOf(2))

	payment := h.seedSucceededUpgrade(t, sub)

	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.TariffID != payment.TariffID || stored.Status != domain.SubscriptionStatusActive {
		t.Fatalf("subscription = %s/%s, want the upgraded business/active", stored.TariffID, stored.Status)
	}
	if len(stored.GraceArchivedPropertyIDs) != 0 {
		t.Errorf("grace snapshot = %v, want cleared after the restoration", stored.GraceArchivedPropertyIDs)
	}
	calls := archiver.restoreCalls()
	if len(calls) != 1 || calls[0].limit != -1 {
		t.Fatalf("restore calls = %+v, want one at the unlimited business limit", calls)
	}
}

// TestPayment_PlainRenewalWithoutSnapshotSkipsRestore proves the restoration
// is bound to the snapshot: a success on a subscription that owes nothing
// calls the bridge zero times.
func TestPayment_PlainRenewalWithoutSnapshotSkipsRestore(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	archiver := &fakeArchiverSource{}
	slots := &fakeSlotSource{}
	h.payments.SetLifecycleBridges(archiver, slots)

	sub := h.seedGraceSnapshot(t, nil)

	result, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffPro, Period: domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(grace renewal) error = %v", err)
	}
	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if err := h.webhookSucceeded(t, payment); err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}

	if got := archiver.restoreCalls(); len(got) != 0 {
		t.Errorf("restore calls = %+v, want none without a snapshot", got)
	}
}

// TestPayment_RestoreBridgeFailureFailsApplication proves a bridge failure
// fails the success application: the webhook answers non-200 so the provider
// redelivers and the seam retries. The full transactional rollback is pinned
// by the integration suite (the in-memory stores cannot roll back).
func TestPayment_RestoreBridgeFailureFailsApplication(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	archiver := &fakeArchiverSource{err: errors.New("bridge down")}
	slots := &fakeSlotSource{}
	h.payments.SetLifecycleBridges(archiver, slots)

	sub := h.seedGraceSnapshot(t, archiveIDsOf(1))

	result, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffPro, Period: domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff(grace renewal) error = %v", err)
	}
	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if err := h.webhookSucceeded(t, payment); err == nil {
		t.Fatal("HandleWebhook() error = nil, want the bridge failure to surface")
	}
	if got := archiver.restoreCalls(); len(got) != 1 || got[0].limit != 5 {
		t.Errorf("restore calls = %+v, want the attempted restoration at the pro limit", got)
	}
}

// TestPayment_PartialRestoreKeepsRemainderForNextPayment proves the debt
// survives a limit-constrained restoration: a payment that could not fit the
// whole snapshot leaves the remainder on the subscription, and the NEXT
// applied payment restores it (ADR 0055: the debt persists until settled or
// the fall to basic).
func TestPayment_PartialRestoreKeepsRemainderForNextPayment(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	archiver := &fakeArchiverSource{}
	slots := &fakeSlotSource{}
	h.payments.SetLifecycleBridges(archiver, slots)

	snapshot := archiveIDsOf(2)
	sub := h.seedGraceSnapshot(t, snapshot)

	// First payment: the bridge restores only the head of the snapshot.
	archiver.remainingAs = snapshot[1:]
	result, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffPro, Period: domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff #1 error = %v", err)
	}
	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if err := h.webhookSucceeded(t, payment); err != nil {
		t.Fatalf("HandleWebhook() #1 error = %v", err)
	}

	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if len(stored.GraceArchivedPropertyIDs) != 1 || stored.GraceArchivedPropertyIDs[0] != snapshot[1] {
		t.Fatalf("grace snapshot after the partial restore = %v, want the remainder [%s]", stored.GraceArchivedPropertyIDs, snapshot[1])
	}

	// Second payment (a later renewal): the bridge settles the rest.
	archiver.remainingAs = nil
	// Move the clock past the paid period so a renewal is due again? No — a
	// manual same-tariff payment needs the grace window; drive an upgrade
	// instead, which is payable from an active subscription.
	result, err = h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffBusiness, Period: domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff #2 error = %v", err)
	}
	payment, err = h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() #2 error = %v", err)
	}
	if err := h.webhookSucceeded(t, payment); err != nil {
		t.Fatalf("HandleWebhook() #2 error = %v", err)
	}

	stored, err = h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("re-read subscription: %v", err)
	}
	if len(stored.GraceArchivedPropertyIDs) != 0 {
		t.Errorf("grace snapshot after the second payment = %v, want settled", stored.GraceArchivedPropertyIDs)
	}
	calls := archiver.restoreCalls()
	if len(calls) != 2 || len(calls[1].ids) != 1 || calls[1].ids[0] != snapshot[1] {
		t.Errorf("restore calls = %+v, want the remainder restored on the second payment", calls)
	}
}

// TestSubscription_CancelInGraceKeepsSnapshot pins the cancel edge of
// grace v2: cancelling inside the window renounces nothing — the snapshot
// survives so a later reactivation payment restores the archive, and the fall
// to basic drops it.
func TestSubscription_CancelInGraceKeepsSnapshot(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)

	snapshot := archiveIDsOf(2)
	sub := h.seedGraceSnapshot(t, snapshot)

	if err := h.subs.CancelSubscription(t.Context(), sub.UserID, nil); err != nil {
		t.Fatalf("CancelSubscription() error = %v", err)
	}

	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusCancelled {
		t.Fatalf("status = %q, want cancelled", stored.Status)
	}
	if got := stored.GraceArchivedPropertyIDs; len(got) != len(snapshot) {
		t.Errorf("grace snapshot = %v, want kept across the cancel", got)
	}
}
