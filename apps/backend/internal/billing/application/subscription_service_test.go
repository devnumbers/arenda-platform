package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// captureRecorder wraps auditapp.Noop and records every entry so tests can
// assert on the audit trail.
type captureRecorder struct {
	auditapp.Noop
	mu      sync.Mutex
	entries []auditdomain.Entry
}

func (r *captureRecorder) Record(_ context.Context, entry auditdomain.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entry)
	return nil
}

// WithTx returns the recorder itself: embedded Noop.WithTx would hand out a
// plain Noop inside transactions and discard the entries under test.
func (r *captureRecorder) WithTx(transaction.Tx) auditapp.Recorder { return r }

func (r *captureRecorder) recorded() []auditdomain.Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries := make([]auditdomain.Entry, len(r.entries))
	copy(entries, r.entries)
	return entries
}

// subscriptionHarness bundles the subscription service over in-memory fakes
// with the canonical tariffs, a capture audit recorder and a fixed clock.
type subscriptionHarness struct {
	svc     *SubscriptionService
	stores  *fakeStores
	audit   *captureRecorder
	now     time.Time
	tariffs []domain.Tariff
}

func newSubscriptionHarness(t *testing.T) *subscriptionHarness {
	t.Helper()
	tariffs := testTariffs()
	stores := newFakeStores(tariffs...)
	audit := &captureRecorder{}
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	svc := NewSubscriptionService(stores.factory(audit), SubscriptionServiceConfig{Clock: fakeClock{now: now}})
	return &subscriptionHarness{svc: svc, stores: stores, audit: audit, now: now, tariffs: tariffs}
}

func (h *subscriptionHarness) tariffID(t *testing.T, name domain.TariffName) uuid.UUID {
	t.Helper()
	for _, tariff := range h.tariffs {
		if tariff.Name == name {
			return tariff.ID
		}
	}
	t.Fatalf("harness tariffs contain no %q plan", name)
	return uuid.Nil
}

// seedPaidSubscription creates a paid subscription on the given tariff with a
// month of paid validity and returns it.
func (h *subscriptionHarness) seedPaidSubscription(t *testing.T, name domain.TariffName) domain.Subscription {
	t.Helper()
	userID := uuid.Must(uuid.NewV7())
	sub, err := domain.NewBasicSubscription(userID, h.tariffID(t, name))
	if err != nil {
		t.Fatalf("NewBasicSubscription() error = %v", err)
	}
	validUntil := h.now.AddDate(0, 1, 0)
	period := domain.PeriodMonth
	sub.TariffID = h.tariffID(t, name)
	sub.ValidUntil = &validUntil
	sub.AutoRenewEnabled = true
	sub.CurrentPeriod = &period
	if _, err := h.stores.subscriptions.Create(t.Context(), sub); err != nil {
		t.Fatalf("seed Create() error = %v", err)
	}
	return sub
}

// requireCancelledSubscription asserts the cancelled subscription keeps the
// paid period with auto-renew off and stays mutable until the period ends.
func (h *subscriptionHarness) requireCancelledSubscription(t *testing.T, sub domain.Subscription) {
	t.Helper()
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusCancelled {
		t.Errorf("Status = %q, want %q", stored.Status, domain.SubscriptionStatusCancelled)
	}
	if stored.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = true, want false")
	}
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(*sub.ValidUntil) {
		t.Errorf("ValidUntil = %v, want the retained paid period %v", stored.ValidUntil, sub.ValidUntil)
	}
	if !stored.CanMutateData(h.now) {
		t.Error("cancelled subscription must stay mutable until the paid period ends")
	}
}

// requireCancelledTransitionLog asserts the transition log holds exactly one
// user-initiated cancelled entry with the active from-side.
func (h *subscriptionHarness) requireCancelledTransitionLog(t *testing.T, sub domain.Subscription) {
	t.Helper()
	transitions, err := h.stores.transitions.ListBySubscriptionID(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	if len(transitions) != 1 {
		t.Fatalf("transitions = %d, want 1", len(transitions))
	}
	tr := transitions[0]
	if tr.Reason != domain.TransitionReasonCancelled {
		t.Errorf("Reason = %q, want %q", tr.Reason, domain.TransitionReasonCancelled)
	}
	if tr.Initiator != domain.InitiatorUser || tr.InitiatorID == nil || *tr.InitiatorID != sub.UserID {
		t.Errorf("initiator = %q/%v, want user/%v", tr.Initiator, tr.InitiatorID, sub.UserID)
	}
	if tr.FromStatus == nil || *tr.FromStatus != domain.SubscriptionStatusActive {
		t.Errorf("FromStatus = %v, want active", tr.FromStatus)
	}
	if tr.ToStatus != domain.SubscriptionStatusCancelled {
		t.Errorf("ToStatus = %q, want cancelled", tr.ToStatus)
	}
}

// requireCancelledAudit asserts the audit trail holds exactly the
// user-attributed cancellation entry of the subscription.
func (h *subscriptionHarness) requireCancelledAudit(t *testing.T, sub domain.Subscription) {
	t.Helper()
	entries := h.audit.recorded()
	if len(entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(entries))
	}
	if entries[0].Action != auditdomain.ActionSubscriptionCancelled {
		t.Errorf("audit action = %q, want %q", entries[0].Action, auditdomain.ActionSubscriptionCancelled)
	}
	if entries[0].ActorID == nil || *entries[0].ActorID != sub.UserID {
		t.Errorf("audit actor = %v, want %v", entries[0].ActorID, sub.UserID)
	}
	if entries[0].EntityID == nil || *entries[0].EntityID != sub.ID {
		t.Errorf("audit entity = %v, want %v", entries[0].EntityID, sub.ID)
	}
}

func TestSubscriptionService_CancelSubscription_CancelsWithTransitionAndAudit(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)

	if err := h.svc.CancelSubscription(t.Context(), sub.UserID, nil); err != nil {
		t.Fatalf("CancelSubscription() error = %v", err)
	}

	h.requireCancelledSubscription(t, sub)
	h.requireCancelledTransitionLog(t, sub)
	h.requireCancelledAudit(t, sub)
}

func TestSubscriptionService_CancelSubscription_NotFound(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)

	if err := h.svc.CancelSubscription(t.Context(), uuid.Must(uuid.NewV7()), nil); !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("err = %v, want ErrSubscriptionNotFound", err)
	}
}

func TestSubscriptionService_CancelSubscription_AlreadyCancelledRejected(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)

	if err := h.svc.CancelSubscription(t.Context(), sub.UserID, nil); err != nil {
		t.Fatalf("first CancelSubscription() error = %v", err)
	}
	if err := h.svc.CancelSubscription(t.Context(), sub.UserID, nil); !errors.Is(err, domain.ErrInvalidSubscriptionState) {
		t.Fatalf("second err = %v, want domain.ErrInvalidSubscriptionState", err)
	}
	transitions, err := h.stores.transitions.ListBySubscriptionID(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	if len(transitions) != 1 {
		t.Fatalf("transitions = %d, want 1 (rejected retry appends nothing)", len(transitions))
	}
}

func TestSubscriptionService_CancelSubscription_ServiceSourceRejected(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)
	sub.Source = domain.SubscriptionSourceService
	if err := h.stores.subscriptions.Update(t.Context(), sub); err != nil {
		t.Fatalf("seed Update() error = %v", err)
	}

	if err := h.svc.CancelSubscription(t.Context(), sub.UserID, nil); !errors.Is(err, domain.ErrInvalidSubscriptionState) {
		t.Fatalf("err = %v, want domain.ErrInvalidSubscriptionState", err)
	}
}

func TestSubscriptionService_ToggleAutoRenew_PersistsAndAudits(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)

	if err := h.svc.ToggleAutoRenew(t.Context(), sub.UserID, false); err != nil {
		t.Fatalf("ToggleAutoRenew(false) error = %v", err)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = true after disable, want false")
	}

	if err := h.svc.ToggleAutoRenew(t.Context(), sub.UserID, true); err != nil {
		t.Fatalf("ToggleAutoRenew(true) error = %v", err)
	}
	stored, err = h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() after enable: %v", err)
	}
	if !stored.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = false after enable, want true")
	}

	entries := h.audit.recorded()
	if len(entries) != 2 {
		t.Fatalf("audit entries = %d, want 2", len(entries))
	}
	for i, enabled := range []bool{false, true} {
		if entries[i].Action != auditdomain.ActionSubscriptionAutoRenewToggled {
			t.Errorf("entry %d action = %q, want %q", i, entries[i].Action, auditdomain.ActionSubscriptionAutoRenewToggled)
		}
		if got, ok := entries[i].Context["enabled"].(bool); !ok || got != enabled {
			t.Errorf("entry %d context enabled = %v, want %v", i, entries[i].Context["enabled"], enabled)
		}
	}
	// Auto-renew is a setting, not a state transition: nothing lands in the
	// transition log.
	transitions, err := h.stores.transitions.ListBySubscriptionID(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	if len(transitions) != 0 {
		t.Fatalf("transitions = %d, want 0", len(transitions))
	}
}

func TestSubscriptionService_CancelSubscription_DropsScheduledDowngrade(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffBusiness)

	if _, err := h.svc.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodYear,
	}); err != nil {
		t.Fatalf("ChangeTariff() error = %v", err)
	}
	if err := h.svc.CancelSubscription(t.Context(), sub.UserID, nil); err != nil {
		t.Fatalf("CancelSubscription() error = %v", err)
	}

	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.HasPendingChange() {
		t.Error("scheduled downgrade survived cancellation, want it dropped (the subscription runs out and falls to basic)")
	}
}

func TestSubscriptionService_ToggleAutoRenew_CancelledRejected(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)
	if err := h.svc.CancelSubscription(t.Context(), sub.UserID, nil); err != nil {
		t.Fatalf("CancelSubscription() error = %v", err)
	}

	if err := h.svc.ToggleAutoRenew(t.Context(), sub.UserID, true); !errors.Is(err, domain.ErrInvalidSubscriptionState) {
		t.Fatalf("err = %v, want domain.ErrInvalidSubscriptionState (a cancelled subscription never renews)", err)
	}
}

func TestSubscriptionService_ToggleAutoRenew_EnableWithoutValidityRejected(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub, err := domain.NewBasicSubscription(uuid.Must(uuid.NewV7()), h.tariffID(t, domain.TariffBasic))
	if err != nil {
		t.Fatalf("NewBasicSubscription() error = %v", err)
	}
	if _, err := h.stores.subscriptions.Create(t.Context(), sub); err != nil {
		t.Fatalf("seed Create() error = %v", err)
	}

	if err := h.svc.ToggleAutoRenew(t.Context(), sub.UserID, true); !errors.Is(err, domain.ErrCannotEnableAutoRenew) {
		t.Fatalf("err = %v, want domain.ErrCannotEnableAutoRenew", err)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = true, want unchanged false")
	}
}

func TestSubscriptionService_ToggleAutoRenew_NotFound(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)

	if err := h.svc.ToggleAutoRenew(t.Context(), uuid.Must(uuid.NewV7()), true); !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("err = %v, want ErrSubscriptionNotFound", err)
	}
}

// requireDowngradeScheduled asserts the stored subscription carries the
// deferred change — target tariff, period, due date at the paid period end —
// while the current tariff stays until the apply.
func (h *subscriptionHarness) requireDowngradeScheduled(t *testing.T, sub domain.Subscription) {
	t.Helper()
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.TariffID != sub.TariffID {
		t.Errorf("TariffID = %v, want unchanged %v (downgrade is deferred)", stored.TariffID, sub.TariffID)
	}
	if stored.PendingTariffID == nil || *stored.PendingTariffID != h.tariffID(t, domain.TariffPro) {
		t.Errorf("PendingTariffID = %v, want pro", stored.PendingTariffID)
	}
	if stored.PendingPeriod == nil || *stored.PendingPeriod != domain.PeriodYear {
		t.Errorf("PendingPeriod = %v, want year", stored.PendingPeriod)
	}
	if stored.PendingChangeAt == nil || !stored.PendingChangeAt.Equal(*sub.ValidUntil) {
		t.Errorf("PendingChangeAt = %v, want the end of the paid period %v", stored.PendingChangeAt, sub.ValidUntil)
	}
	if !stored.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = false, want true (ADR 0008: the new tariff renews)")
	}
}

// requireDowngradeScheduledTransition asserts the transition log holds exactly
// the user-initiated downgrade_scheduled entry naming the target tariff.
func (h *subscriptionHarness) requireDowngradeScheduledTransition(t *testing.T, sub domain.Subscription) {
	t.Helper()
	transitions, err := h.stores.transitions.ListBySubscriptionID(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	if len(transitions) != 1 {
		t.Fatalf("transitions = %d, want 1", len(transitions))
	}
	tr := transitions[0]
	if tr.Reason != domain.TransitionReasonDowngradeScheduled {
		t.Errorf("Reason = %q, want %q", tr.Reason, domain.TransitionReasonDowngradeScheduled)
	}
	if tr.ToTariffID != h.tariffID(t, domain.TariffPro) {
		t.Errorf("ToTariffID = %v, want the scheduled target", tr.ToTariffID)
	}
	if tr.Initiator != domain.InitiatorUser || tr.InitiatorID == nil || *tr.InitiatorID != sub.UserID {
		t.Errorf("initiator = %q/%v, want user/%v", tr.Initiator, tr.InitiatorID, sub.UserID)
	}
}

// requireTariffChangeAudit asserts the audit trail holds exactly the
// tariff-change entry with the from/to tariff context.
func (h *subscriptionHarness) requireTariffChangeAudit(t *testing.T, sub domain.Subscription) {
	t.Helper()
	entries := h.audit.recorded()
	if len(entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(entries))
	}
	if entries[0].Action != auditdomain.ActionSubscriptionTariffChanged {
		t.Errorf("audit action = %q, want %q", entries[0].Action, auditdomain.ActionSubscriptionTariffChanged)
	}
	if entries[0].Context[auditKeyFromTariffID] != sub.TariffID || entries[0].Context[auditKeyToTariffID] != h.tariffID(t, domain.TariffPro) {
		t.Errorf("audit context = %v, want from business to pro", entries[0].Context)
	}
}

func TestSubscriptionService_ChangeTariff_DowngradeIsScheduled(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffBusiness)

	result, err := h.svc.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodYear,
	})
	if err != nil {
		t.Fatalf("ChangeTariff() error = %v", err)
	}
	if result.PaymentID != uuid.Nil || result.ConfirmURL != "" {
		t.Errorf("result = %+v, want zero values (no payment on the free path)", result)
	}

	h.requireDowngradeScheduled(t, sub)
	h.requireDowngradeScheduledTransition(t, sub)
	h.requireTariffChangeAudit(t, sub)
}

func TestSubscriptionService_ChangeTariff_SameTariffRejected(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)

	_, err := h.svc.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, domain.ErrAlreadyOnTariff) {
		t.Fatalf("err = %v, want domain.ErrAlreadyOnTariff", err)
	}
	if pending, transitionCount, auditCount := h.sideEffects(t, sub.ID); pending || transitionCount != 0 || auditCount != 0 {
		t.Errorf("rejected change left side effects: pending=%v transitions=%d audit=%d", pending, transitionCount, auditCount)
	}
}

func TestSubscriptionService_ChangeTariff_UpgradeTemporarilyUnavailable(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)

	_, err := h.svc.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, ErrPaymentUnavailable) {
		t.Fatalf("err = %v, want ErrPaymentUnavailable", err)
	}
	if pending, transitionCount, auditCount := h.sideEffects(t, sub.ID); pending || transitionCount != 0 || auditCount != 0 {
		t.Errorf("unavailable upgrade left side effects: pending=%v transitions=%d audit=%d", pending, transitionCount, auditCount)
	}
}

func TestSubscriptionService_ChangeTariff_SameTariffInGraceNeedsPayment(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)
	graceUntil := h.now.Add(48 * time.Hour)
	sub.Status = domain.SubscriptionStatusGrace
	sub.ValidUntil = &graceUntil
	if err := h.stores.subscriptions.Update(t.Context(), sub); err != nil {
		t.Fatalf("seed Update() error = %v", err)
	}

	_, err := h.svc.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, ErrPaymentUnavailable) {
		t.Fatalf("err = %v, want ErrPaymentUnavailable (grace renewal is the payment path)", err)
	}
}

func TestSubscriptionService_ChangeTariff_UnknownTariffRejected(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)

	_, err := h.svc.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffName("gold"),
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, ErrTariffNotFound) {
		t.Fatalf("err = %v, want ErrTariffNotFound", err)
	}
}

func TestSubscriptionService_ChangeTariff_CancelledSubscriptionRejected(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffBusiness)
	if err := sub.Cancel(nil); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if err := h.stores.subscriptions.Update(t.Context(), sub); err != nil {
		t.Fatalf("seed Update() error = %v", err)
	}

	_, err := h.svc.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, domain.ErrInvalidSubscriptionState) {
		t.Fatalf("err = %v, want domain.ErrInvalidSubscriptionState", err)
	}
}

func TestSubscriptionService_ChangeTariff_NoSubscription(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)

	_, err := h.svc.ChangeTariff(t.Context(), uuid.Must(uuid.NewV7()), ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	})
	if !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("err = %v, want ErrSubscriptionNotFound", err)
	}
}

// sideEffects reports whether the subscription carries a pending change and
// how many transitions and audit entries exist for it.
func (h *subscriptionHarness) sideEffects(t *testing.T, subscriptionID uuid.UUID) (pending bool, transitions, auditEntries int) {
	t.Helper()
	rows, err := h.stores.transitions.ListBySubscriptionID(t.Context(), subscriptionID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	return h.hasPending(subscriptionID), len(rows), len(h.audit.recorded())
}

func (h *subscriptionHarness) hasPending(subscriptionID uuid.UUID) bool {
	for _, sub := range h.stores.subscriptions.subs {
		if sub.ID == subscriptionID {
			return sub.HasPendingChange()
		}
	}
	return false
}

// withKeepBridge wires the lifecycle bridges with an archiver whose
// ActivePropertyExists answers from the given map — the properties-side half
// of the cancel keep-choice validation (issue #617).
func (h *subscriptionHarness) withKeepBridge(keepID uuid.UUID, exists bool) *fakeArchiverSource {
	src := &fakeArchiverSource{activeExists: map[uuid.UUID]bool{keepID: exists}}
	h.svc.SetLifecycleBridges(src, &fakeSlotSource{})
	return src
}

func TestSubscriptionService_CancelSubscription_KeepPropertyValidatedAndStored(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)
	keepID := uuid.Must(uuid.NewV7())
	src := h.withKeepBridge(keepID, true)

	if err := h.svc.CancelSubscription(t.Context(), sub.UserID, &keepID); err != nil {
		t.Fatalf("CancelSubscription(keep) error = %v", err)
	}

	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.KeepPropertyID == nil || *stored.KeepPropertyID != keepID {
		t.Errorf("KeepPropertyID = %v, want %v", stored.KeepPropertyID, keepID)
	}
	if len(src.recorded()) != 0 {
		t.Errorf("cancel must not archive: archive calls = %d, want 0", len(src.recorded()))
	}
	entries := h.audit.recorded()
	if len(entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(entries))
	}
	if got, ok := entries[0].Context["keep_property_id"]; !ok || got != keepID {
		t.Errorf("audit keep_property_id = %v, want %v", got, keepID)
	}
}

func TestSubscriptionService_CancelSubscription_KeepPropertyInvalidRejected(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)
	foreignID := uuid.Must(uuid.NewV7())
	h.withKeepBridge(foreignID, false)

	if err := h.svc.CancelSubscription(t.Context(), sub.UserID, &foreignID); !errors.Is(err, ErrInvalidKeepProperty) {
		t.Fatalf("err = %v, want ErrInvalidKeepProperty", err)
	}

	// The rejection leaves the subscription untouched: still active, no
	// transition appended.
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("Status = %q, want still active", stored.Status)
	}
	transitions, err := h.stores.transitions.ListBySubscriptionID(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	if len(transitions) != 0 {
		t.Fatalf("transitions = %d, want 0", len(transitions))
	}
}

func TestSubscriptionService_ResumeSubscription_RestoresCancelledInsidePaidPeriod(t *testing.T) {
	t.Parallel()
	h := newSubscriptionHarness(t)
	sub := h.seedPaidSubscription(t, domain.TariffPro)
	keepID := uuid.Must(uuid.NewV7())
	h.withKeepBridge(keepID, true)
	stored := seedCancelledAndResumed(t, h, sub, keepID)

	if stored.Status != domain.SubscriptionStatusActive {
		t.Errorf("Status = %q, want %q", stored.Status, domain.SubscriptionStatusActive)
	}
	if !stored.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = false, want enabled by the resume")
	}
	if stored.ValidUntil == nil || !stored.ValidUntil.Equal(*sub.ValidUntil) {
		t.Errorf("ValidUntil = %v, want the retained paid period %v", stored.ValidUntil, sub.ValidUntil)
	}
	if stored.KeepPropertyID != nil {
		t.Errorf("KeepPropertyID = %v, want dropped with the undone cancellation", stored.KeepPropertyID)
	}
	requireResumedTransitionLog(t, h, sub)
	requireResumedAudit(t, h, sub)
}

// requireResumedTransitionLog asserts the log holds cancelled then resumed,
// the resume the newest user-initiated entry with the cancelled from-side.
// The fake transition repo lists in insertion order — the real repository
// returns newest first.
func requireResumedTransitionLog(t *testing.T, h *subscriptionHarness, sub domain.Subscription) {
	t.Helper()
	transitions, err := h.stores.transitions.ListBySubscriptionID(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	if len(transitions) != 2 {
		t.Fatalf("transitions = %d, want 2 (cancelled, resumed)", len(transitions))
	}
	resumed := transitions[len(transitions)-1]
	if resumed.Reason != domain.TransitionReasonResumed {
		t.Errorf("Reason = %q, want %q", resumed.Reason, domain.TransitionReasonResumed)
	}
	if resumed.Initiator != domain.InitiatorUser || resumed.InitiatorID == nil || *resumed.InitiatorID != sub.UserID {
		t.Errorf("initiator = %q/%v, want user/%v", resumed.Initiator, resumed.InitiatorID, sub.UserID)
	}
	if resumed.FromStatus == nil || *resumed.FromStatus != domain.SubscriptionStatusCancelled {
		t.Errorf("FromStatus = %v, want cancelled", resumed.FromStatus)
	}
	if resumed.ToStatus != domain.SubscriptionStatusActive {
		t.Errorf("ToStatus = %q, want active", resumed.ToStatus)
	}
}

// requireResumedAudit asserts the audit trail holds exactly the
// user-attributed resume entry of the subscription.
func requireResumedAudit(t *testing.T, h *subscriptionHarness, sub domain.Subscription) {
	t.Helper()
	entries := h.audit.recorded()
	if len(entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(entries))
	}
	if entries[0].Action != auditdomain.ActionSubscriptionResumed {
		t.Errorf("audit action = %q, want %q", entries[0].Action, auditdomain.ActionSubscriptionResumed)
	}
	if entries[0].ActorID == nil || *entries[0].ActorID != sub.UserID {
		t.Errorf("audit actor = %v, want %v", entries[0].ActorID, sub.UserID)
	}
	if entries[0].EntityID == nil || *entries[0].EntityID != sub.ID {
		t.Errorf("audit entity = %v, want %v", entries[0].EntityID, sub.ID)
	}
}

// seedCancelledAndResumed drives the seed-cancel-resume sequence of the
// resume tests: it cancels with the keep choice, wipes the audit capture and
// resumes, returning the stored post-resume subscription.
func seedCancelledAndResumed(t *testing.T, h *subscriptionHarness, sub domain.Subscription, keepID uuid.UUID) domain.Subscription {
	t.Helper()
	if err := h.svc.CancelSubscription(t.Context(), sub.UserID, &keepID); err != nil {
		t.Fatalf("seed CancelSubscription() error = %v", err)
	}
	h.audit.entries = nil
	if err := h.svc.ResumeSubscription(t.Context(), sub.UserID); err != nil {
		t.Fatalf("ResumeSubscription() error = %v", err)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	return stored
}

func TestSubscriptionService_ResumeSubscription_Rejections(t *testing.T) {
	t.Parallel()

	t.Run("active subscription is not resumable", func(t *testing.T) {
		t.Parallel()
		h := newSubscriptionHarness(t)
		sub := h.seedPaidSubscription(t, domain.TariffPro)
		if err := h.svc.ResumeSubscription(t.Context(), sub.UserID); !errors.Is(err, domain.ErrResumeNotAvailable) {
			t.Fatalf("err = %v, want domain.ErrResumeNotAvailable", err)
		}
	})

	t.Run("expired paid period falls to the paid restoration path", func(t *testing.T) {
		t.Parallel()
		h := newSubscriptionHarness(t)
		sub := h.seedPaidSubscription(t, domain.TariffPro)
		h.withKeepBridge(uuid.Must(uuid.NewV7()), true)
		if err := h.svc.CancelSubscription(t.Context(), sub.UserID, nil); err != nil {
			t.Fatalf("seed CancelSubscription() error = %v", err)
		}
		expired := h.now.Add(-time.Hour)
		sub.ValidUntil = &expired
		if err := h.stores.subscriptions.Update(t.Context(), sub); err != nil {
			t.Fatalf("seed Update() error = %v", err)
		}

		if err := h.svc.ResumeSubscription(t.Context(), sub.UserID); !errors.Is(err, domain.ErrResumeNotAvailable) {
			t.Fatalf("err = %v, want domain.ErrResumeNotAvailable", err)
		}
	})

	t.Run("second resume is rejected", func(t *testing.T) {
		t.Parallel()
		h := newSubscriptionHarness(t)
		sub := h.seedPaidSubscription(t, domain.TariffPro)
		if err := h.svc.CancelSubscription(t.Context(), sub.UserID, nil); err != nil {
			t.Fatalf("seed CancelSubscription() error = %v", err)
		}
		if err := h.svc.ResumeSubscription(t.Context(), sub.UserID); err != nil {
			t.Fatalf("first ResumeSubscription() error = %v", err)
		}
		if err := h.svc.ResumeSubscription(t.Context(), sub.UserID); !errors.Is(err, domain.ErrResumeNotAvailable) {
			t.Fatalf("second err = %v, want domain.ErrResumeNotAvailable", err)
		}
	})
}
