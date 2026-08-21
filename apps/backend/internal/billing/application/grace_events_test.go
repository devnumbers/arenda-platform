package application

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The grace lifecycle events of issue #253, tested on the application seam
// with a capture publisher (issue #244 testing decisions): every grace entry
// publishes GraceEntered exactly once after its transaction commits, the
// expiry reminder fires inside its window exactly once, and a failing
// publisher never fails or rolls back the payment transition.

// The grace-events module of issue #284: the single interface both payment
// paths (the webhook finalization of a customer-initiated charge and the
// renewal worker's merchant-initiated charge) publish the Grace events
// through. The module tests below run the module over the in-memory fakes
// with a recording timeline — the transaction commit and every dispatch are
// logged in order, so "strictly after the commit" is asserted, not assumed.

// timeline records one run's ordering: the transaction commit instant and
// each event publication.
type timeline struct {
	events []string
}

func (tl *timeline) add(mark string) { tl.events = append(tl.events, mark) }

// timelinePublisher is a capturePublisher that marks each dispatch on the
// shared timeline.
type timelinePublisher struct {
	capturePublisher
	timeline *timeline
}

func (p *timelinePublisher) PublishGraceEntered(ctx context.Context, event GraceEntered) error {
	p.timeline.add("publish grace-entered")
	return p.capturePublisher.PublishGraceEntered(ctx, event)
}

func (p *timelinePublisher) PublishGraceExpiring(ctx context.Context, event GraceExpiring) error {
	p.timeline.add("publish grace-expiring")
	return p.capturePublisher.PublishGraceExpiring(ctx, event)
}

// graceModuleHarness wires the module over the transition harness's fakes.
// The commit runner wraps the real factory transaction and marks its success
// on the timeline — the observable commit instant the publication ordering is
// asserted against.
type graceModuleHarness struct {
	*transitionHarness
	timeline *timeline
	pub      *timelinePublisher
	factory  txStoreFactory
	graceDur time.Duration
}

func newGraceModuleHarness(t *testing.T) *graceModuleHarness {
	t.Helper()
	th := newTransitionHarness(t)
	tl := &timeline{}
	return &graceModuleHarness{
		transitionHarness: th,
		timeline:          tl,
		pub:               &timelinePublisher{timeline: tl},
		factory:           th.stores.factory(nil),
		graceDur:          7 * 24 * time.Hour,
	}
}

// commitRunner returns the transaction runner of the module's run: the real
// factory transaction, with its successful commit marked on the timeline.
func (h *graceModuleHarness) commitRunner() func(context.Context, func(*txStores) error) error {
	return func(ctx context.Context, work func(*txStores) error) error {
		if err := h.factory.runInTx(ctx, work); err != nil {
			return err
		}
		h.timeline.add("commit")
		return nil
	}
}

// runGrace runs one module transaction with the harness publisher.
func (h *graceModuleHarness) runGrace(ctx context.Context, work func(*graceEvents, *txStores) error) error {
	grace := newGraceEvents(h.pub, slog.New(slog.DiscardHandler))
	return grace.run(ctx, h.commitRunner(), func(stores *txStores) error {
		return work(grace, stores)
	})
}

// TestGraceEvents_EnteredPublishedStrictlyAfterCommit proves the module's
// core contract: the GraceEntered event of a grace transition is published
// strictly after the causing transaction commits — never inside it — and
// identifies the subscription with its fresh window.
func TestGraceEvents_EnteredPublishedStrictlyAfterCommit(t *testing.T) {
	t.Parallel()
	h := newGraceModuleHarness(t)
	// The real grace paths arrive from an expired paid period (ADR 0008);
	// EnterGrace then extends the validity to the fresh grace window.
	expired := h.now.AddDate(0, -1, 0)
	sub := h.seedSubscription(t, domain.TariffPro, func(s *domain.Subscription) {
		s.ValidUntil = &expired
	})

	if err := h.runGrace(t.Context(), func(grace *graceEvents, stores *txStores) error {
		return grace.enterGrace(t.Context(), stores, sub, h.now, h.graceDur)
	}); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !slices.Equal(h.timeline.events, []string{"commit", "publish grace-entered"}) {
		t.Fatalf("timeline = %v, want [commit publish grace-entered] (publication strictly after the commit)", h.timeline.events)
	}
	if len(h.pub.entered) != 1 {
		t.Fatalf("GraceEntered published %d times, want 1", len(h.pub.entered))
	}
	event := h.pub.entered[0]
	if event.UserID != sub.UserID || event.SubscriptionID != sub.ID {
		t.Errorf("event identifies user %s subscription %s, want user %s subscription %s", event.UserID, event.SubscriptionID, sub.UserID, sub.ID)
	}
	if wantUntil := h.now.Add(h.graceDur); !event.GraceUntil.Equal(wantUntil) {
		t.Errorf("event GraceUntil = %v, want %v", event.GraceUntil, wantUntil)
	}
	if !event.At.Equal(h.now) {
		t.Errorf("event At = %v, want the transition's %v", event.At, h.now)
	}
	if stored := h.storedSubscription(t, sub.UserID); stored.Status != domain.SubscriptionStatusGrace {
		t.Errorf("stored status = %q, want grace", stored.Status)
	}
}

// TestGraceEvents_EnteredPublishedOncePerGraceWindow proves the exactly-once
// semantics: a grace entry attempted inside an already-open window (the late
// webhook of a crashed run, the next worker tick) captures nothing and
// publishes no second event for the same window.
func TestGraceEvents_EnteredPublishedOncePerGraceWindow(t *testing.T) {
	t.Parallel()
	h := newGraceModuleHarness(t)
	sub := h.seedSubscription(t, domain.TariffPro, nil)

	for range 2 {
		if err := h.runGrace(t.Context(), func(grace *graceEvents, stores *txStores) error {
			return grace.enterGrace(t.Context(), stores, sub, h.now, h.graceDur)
		}); err != nil {
			t.Fatalf("run() error = %v", err)
		}
		// The second attempt sees the already-entered subscription, like the
		// re-read under the subscription lock does.
		sub = h.storedSubscription(t, sub.UserID)
	}

	if len(h.pub.entered) != 1 {
		t.Fatalf("GraceEntered published %d times, want 1 (no re-entry inside an open window)", len(h.pub.entered))
	}
}

// TestGraceEvents_RollbackPublishesNothing proves the strict-after-commit
// contract from the failure side: a transaction that rolls back after the
// grace transition publishes nothing — the event belongs to the commit.
func TestGraceEvents_RollbackPublishesNothing(t *testing.T) {
	t.Parallel()
	h := newGraceModuleHarness(t)
	sub := h.seedSubscription(t, domain.TariffPro, nil)
	cause := errors.New("payment finalization failed")

	err := h.runGrace(t.Context(), func(grace *graceEvents, stores *txStores) error {
		if err := grace.enterGrace(t.Context(), stores, sub, h.now, h.graceDur); err != nil {
			return err
		}
		return cause
	})
	if !errors.Is(err, cause) {
		t.Fatalf("run() error = %v, want the work's %v", err, cause)
	}
	if len(h.timeline.events) != 0 {
		t.Errorf("timeline = %v, want empty (neither commit nor publication)", h.timeline.events)
	}
	if len(h.pub.entered) != 0 {
		t.Errorf("GraceEntered published %d times after a rollback, want 0", len(h.pub.entered))
	}
}

// TestGraceEvents_PublisherFailureDoesNotFailTheTransition proves the
// best-effort contract: a failing publisher is logged and swallowed — the
// committed grace transition stands and no error surfaces to the payment flow.
func TestGraceEvents_PublisherFailureDoesNotFailTheTransition(t *testing.T) {
	t.Parallel()
	h := newGraceModuleHarness(t)
	h.pub.err = errors.New("publisher down")
	sub := h.seedSubscription(t, domain.TariffPro, nil)

	if err := h.runGrace(t.Context(), func(grace *graceEvents, stores *txStores) error {
		return grace.enterGrace(t.Context(), stores, sub, h.now, h.graceDur)
	}); err != nil {
		t.Fatalf("run() error = %v (a publisher failure must never surface)", err)
	}
	if stored := h.storedSubscription(t, sub.UserID); stored.Status != domain.SubscriptionStatusGrace {
		t.Errorf("stored status = %q, want grace despite the publisher failure", stored.Status)
	}
	if len(h.pub.entered) != 0 {
		t.Errorf("GraceEntered recorded %d events despite the error, want 0", len(h.pub.entered))
	}
}

// TestGraceEvents_NilPublisherKeepsPreEventBehaviour proves the nil-publisher
// wiring keeps the pre-#253 behaviour: no dispatch, no crash, the grace
// transition itself unaffected.
func TestGraceEvents_NilPublisherKeepsPreEventBehaviour(t *testing.T) {
	t.Parallel()
	h := newGraceModuleHarness(t)
	sub := h.seedSubscription(t, domain.TariffPro, nil)

	grace := newGraceEvents(nil, slog.New(slog.DiscardHandler))
	err := grace.run(t.Context(), h.commitRunner(), func(stores *txStores) error {
		return grace.enterGrace(t.Context(), stores, sub, h.now, h.graceDur)
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if stored := h.storedSubscription(t, sub.UserID); stored.Status != domain.SubscriptionStatusGrace {
		t.Errorf("stored status = %q, want grace", stored.Status)
	}
}

// TestGraceEvents_ExpiringReminderPublishedStrictlyAfterCommit proves the
// reminder half of the module: a window marked reminded inside the transaction
// publishes its GraceExpiring event strictly after the commit, once.
func TestGraceEvents_ExpiringReminderPublishedStrictlyAfterCommit(t *testing.T) {
	t.Parallel()
	h := newGraceModuleHarness(t)
	graceUntil := h.now.Add(24 * time.Hour)
	sub := h.seedSubscription(t, domain.TariffPro, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		s.ValidUntil = &graceUntil
	})

	if err := h.runGrace(t.Context(), func(grace *graceEvents, stores *txStores) error {
		return grace.remindWindow(t.Context(), stores, sub, h.now)
	}); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !slices.Equal(h.timeline.events, []string{"commit", "publish grace-expiring"}) {
		t.Fatalf("timeline = %v, want [commit publish grace-expiring]", h.timeline.events)
	}
	if len(h.pub.expiring) != 1 {
		t.Fatalf("GraceExpiring published %d times, want 1", len(h.pub.expiring))
	}
	event := h.pub.expiring[0]
	if event.UserID != sub.UserID || event.SubscriptionID != sub.ID {
		t.Errorf("event identifies user %s subscription %s, want user %s subscription %s", event.UserID, event.SubscriptionID, sub.UserID, sub.ID)
	}
	if !event.GraceUntil.Equal(graceUntil) {
		t.Errorf("event GraceUntil = %v, want %v", event.GraceUntil, graceUntil)
	}
	if !event.At.Equal(h.now) {
		t.Errorf("event At = %v, want the marking's %v", event.At, h.now)
	}
	stored := h.storedSubscription(t, sub.UserID)
	if stored.GraceRemindedAt == nil || !stored.GraceRemindedAt.Equal(h.now) {
		t.Errorf("GraceRemindedAt = %v, want %v", stored.GraceRemindedAt, h.now)
	}
}

// The window edges of the reminder — nothing before the window opens, one
// reminder inside it, nothing once reminded — live in the worker selection
// since issue #286 and are proven by TestWorkers_GraceExpiryReminderWindow
// over the selection-aware fakes plus the per-Selection repository
// integration test; the module itself only owns the marking and the
// strictly-after-commit publication above.

// capturePublisher is an EventPublisher that records the published events and
// can be scripted to fail.
type capturePublisher struct {
	entered  []GraceEntered
	expiring []GraceExpiring
	err      error
}

func (p *capturePublisher) PublishGraceEntered(_ context.Context, event GraceEntered) error {
	if p.err != nil {
		return p.err
	}
	p.entered = append(p.entered, event)
	return nil
}

func (p *capturePublisher) PublishGraceExpiring(_ context.Context, event GraceExpiring) error {
	if p.err != nil {
		return p.err
	}
	p.expiring = append(p.expiring, event)
	return nil
}

// graceTestConfig returns the workers config with the grace-expiry reminder
// lead time set, so the reminder-window tests are independent of the default.
func graceTestConfig(lead time.Duration) Config {
	cfg := DefaultConfig()
	cfg.GraceExpiryReminderBefore = lead
	return cfg
}

// seedGraceSubscription stores a subscription inside an open grace window
// ending at the given instant.
func (h *workersHarness) seedGraceSubscription(t *testing.T, graceUntil time.Time) domain.Subscription {
	t.Helper()
	return h.seedSubscription(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		s.ValidUntil = &graceUntil
	})
}

// TestWorkers_NoChargeableMethodPublishesGraceEntered proves the acceptance
// criterion of issue #253: a renewal with no chargeable payment method moves
// the subscription into grace and publishes GraceEntered exactly once, after
// the planning transaction commits, with the subscription and window
// identified.
func TestWorkers_NoChargeableMethodPublishesGraceEntered(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	pub := &capturePublisher{}
	h.workers.publisher = pub

	sub := h.seedSubscription(t, nil) // No method linked.

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Fatalf("subscription status = %q, want grace", stored.Status)
	}
	if len(pub.entered) != 1 {
		t.Fatalf("GraceEntered published %d times, want 1", len(pub.entered))
	}
	event := pub.entered[0]
	if event.UserID != sub.UserID || event.SubscriptionID != sub.ID {
		t.Errorf("event identifies user %s subscription %s, want user %s subscription %s", event.UserID, event.SubscriptionID, sub.UserID, sub.ID)
	}
	wantUntil := h.now.Add(h.cfg.GraceDuration)
	if !event.GraceUntil.Equal(wantUntil) {
		t.Errorf("event GraceUntil = %v, want %v", event.GraceUntil, wantUntil)
	}
	if !event.At.Equal(h.now) {
		t.Errorf("event At = %v, want %v", event.At, h.now)
	}
}

// TestWorkers_FailedChargePublishesGraceEntered proves a definitively failed
// renewal charge publishes GraceEntered once (issue #253).
func TestWorkers_FailedChargePublishesGraceEntered(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	pub := &capturePublisher{}
	h.workers.publisher = pub
	h.provider.chargeFn = func(ChargeRequest) (ChargeResult, error) {
		return ChargeResult{Status: domain.PaymentStatusFailed, ErrorCode: "declined"}, nil
	}
	sub := h.seedSubscription(t, nil)
	h.seedActiveMethod(t, sub, testProviderFake, "token_bad")

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}

	if len(pub.entered) != 1 {
		t.Fatalf("GraceEntered published %d times, want 1", len(pub.entered))
	}
	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Errorf("subscription status = %q, want grace", stored.Status)
	}
}

// TestWorkers_GraceEnteredNotRepublishedInsideWindow proves the event fires
// once per grace window: a second failed charge while the subscription is
// already in grace publishes nothing new (issue #253).
func TestWorkers_GraceEnteredNotRepublishedInsideWindow(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	pub := &capturePublisher{}
	h.workers.publisher = pub
	h.provider.chargeFn = func(ChargeRequest) (ChargeResult, error) {
		return ChargeResult{Status: domain.PaymentStatusFailed, ErrorCode: "declined"}, nil
	}
	sub := h.seedSubscription(t, nil)
	h.seedActiveMethod(t, sub, testProviderFake, "token_bad")

	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("first ProcessRenewals() error = %v", err)
	}
	// A late failed webhook for the same renewal: the subscription is already
	// in grace, the window is not re-entered and no second event fires.
	payment := h.singlePaymentOf(t, sub.UserID)
	if err := h.workers.failRenewalPayment(t.Context(), payment.ID, nil, h.now); err != nil {
		t.Fatalf("failRenewalPayment() error = %v", err)
	}

	if len(pub.entered) != 1 {
		t.Fatalf("GraceEntered published %d times, want 1 (no re-entry inside an open window)", len(pub.entered))
	}
}

// TestWorkers_PublisherFailureDoesNotAffectTransition proves the best-effort
// contract of issue #253: a failing publisher is swallowed — the subscription
// still enters grace, the payment still fails, and no error surfaces.
func TestWorkers_PublisherFailureDoesNotAffectTransition(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	pub := &capturePublisher{err: errors.New("publisher down")}
	h.workers.publisher = pub
	sub := h.seedSubscription(t, nil)

	count, err := h.workers.ProcessRenewals(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ProcessRenewals() error = %v (a publisher failure must not surface)", err)
	}
	if count != 1 {
		t.Fatalf("ProcessRenewals() = %d, want 1", count)
	}
	stored := h.storedSubscription(t, sub)
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Errorf("subscription status = %q, want grace despite the publisher failure", stored.Status)
	}
	if len(pub.entered) != 0 {
		t.Errorf("GraceEntered recorded %d events despite the error, want 0", len(pub.entered))
	}
}

// TestWebhook_FailedRenewalChargePublishesGraceEntered proves the webhook
// finalization of a failed merchant-initiated renewal charge (the charge
// carries the payment method) publishes GraceEntered after the finalizing
// transaction commits (issue #253).
func TestWebhook_FailedRenewalChargePublishesGraceEntered(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	pub := &capturePublisher{}
	h.payments.publisher = pub
	method, err := domain.NewPaymentMethod(uuid.Must(uuid.NewV7()), testProviderFake, "token_bad", h.now)
	if err != nil {
		t.Fatalf("new method: %v", err)
	}
	storedMethod, err := h.stores.methods.UpsertByTokenHash(t.Context(), method)
	if err != nil {
		t.Fatalf("seed method: %v", err)
	}

	sub := h.seedSubscription(t, nil)
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.tariffID(t, domain.TariffPro),
		domain.PeriodMonth, 49000, testProviderFake, h.now)
	if err != nil {
		t.Fatalf("new payment: %v", err)
	}
	payment.PaymentMethodID = &storedMethod.ID
	if err := payment.SaveProviderReference("prov_1", "https://pay.example/1", h.now); err != nil {
		t.Fatalf("save provider reference: %v", err)
	}
	payment, err = h.stores.payments.Create(t.Context(), payment)
	if err != nil {
		t.Fatalf("seed payment: %v", err)
	}

	h.setNotification(&PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: "prov_1",
		Status:            domain.PaymentStatusFailed,
		AmountKopecks:     payment.AmountKopecks,
	})
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}

	if len(pub.entered) != 1 {
		t.Fatalf("GraceEntered published %d times, want 1", len(pub.entered))
	}
	if pub.entered[0].SubscriptionID != sub.ID {
		t.Errorf("event subscription = %s, want %s", pub.entered[0].SubscriptionID, sub.ID)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.Status != domain.SubscriptionStatusGrace {
		t.Errorf("subscription status = %q, want grace", stored.Status)
	}
}

// The grace-expiry reminder phase (issue #253): the reminder fires inside the
// window [valid_until - lead, valid_until), exactly once per window.

// requireGraceReminderPublished asserts the publisher emitted exactly one
// GraceExpiring event identifying the subscription with its window, and the
// window is marked reminded at the given tick.
func requireGraceReminderPublished(
	t *testing.T, h *workersHarness, pub *capturePublisher, sub domain.Subscription, graceUntil, remindedAt time.Time,
) {
	t.Helper()
	if len(pub.expiring) != 1 {
		t.Fatalf("GraceExpiring published %d times, want 1", len(pub.expiring))
	}
	event := pub.expiring[0]
	if event.SubscriptionID != sub.ID || event.UserID != sub.UserID {
		t.Errorf("event identifies user %s subscription %s, want user %s subscription %s", event.UserID, event.SubscriptionID, sub.UserID, sub.ID)
	}
	if !event.GraceUntil.Equal(graceUntil) {
		t.Errorf("event GraceUntil = %v, want %v", event.GraceUntil, graceUntil)
	}
	reminded := h.storedSubscription(t, sub)
	if reminded.GraceRemindedAt == nil || !reminded.GraceRemindedAt.Equal(remindedAt) {
		t.Errorf("GraceRemindedAt = %v, want %v", reminded.GraceRemindedAt, remindedAt)
	}
}

// TestWorkers_GraceExpiryReminderWindow proves the window edges: no reminder
// before the lead time arrives, one reminder inside the window, no second
// reminder on the next tick, and no reminder after the window has ended.
func TestWorkers_GraceExpiryReminderWindow(t *testing.T) {
	t.Parallel()
	const lead = 48 * time.Hour
	h := newWorkersHarness(t, graceTestConfig(lead))
	pub := &capturePublisher{}
	h.workers.publisher = pub
	graceUntil := h.now.Add(7 * 24 * time.Hour)
	sub := h.seedGraceSubscription(t, graceUntil)

	// Before the window: five days left, the 48h lead has not arrived.
	if n, err := h.workers.ProcessGraceExpiryReminders(t.Context(), h.now); err != nil || n != 0 {
		t.Fatalf("ProcessGraceExpiryReminders() = %d, %v; want 0, nil before the window", n, err)
	}

	// Inside the window: 24h left.
	inside := graceUntil.Add(-24 * time.Hour)
	if n, err := h.workers.ProcessGraceExpiryReminders(t.Context(), inside); err != nil {
		t.Fatalf("ProcessGraceExpiryReminders() error = %v", err)
	} else if n != 1 {
		t.Fatalf("ProcessGraceExpiryReminders() = %d, want 1 inside the window", n)
	}
	requireGraceReminderPublished(t, h, pub, sub, graceUntil, inside)

	// Next tick inside the same window: already reminded, no second event.
	if n, err := h.workers.ProcessGraceExpiryReminders(t.Context(), inside.Add(time.Hour)); err != nil || n != 0 {
		t.Fatalf("ProcessGraceExpiryReminders() = %d, %v; want 0, nil when already reminded", n, err)
	}
	if len(pub.expiring) != 1 {
		t.Fatalf("GraceExpiring published %d times after re-run, want 1", len(pub.expiring))
	}

	// After the window: the grace has ended, the reminder is moot.
	if n, err := h.workers.ProcessGraceExpiryReminders(t.Context(), graceUntil.Add(time.Minute)); err != nil || n != 0 {
		t.Fatalf("ProcessGraceExpiryReminders() = %d, %v; want 0, nil after the window", n, err)
	}
	if len(pub.expiring) != 1 {
		t.Fatalf("GraceExpiring published %d times after grace end, want 1", len(pub.expiring))
	}
}

// TestWorkers_GraceExpiryReminderFreshWindow proves a subscription that
// recovers, fails again and enters a new grace window is reminded again: a new
// window starts unreminded (issue #253).
func TestWorkers_GraceExpiryReminderFreshWindow(t *testing.T) {
	t.Parallel()
	const lead = 48 * time.Hour
	h := newWorkersHarness(t, graceTestConfig(lead))
	pub := &capturePublisher{}
	h.workers.publisher = pub
	firstUntil := h.now.Add(30 * time.Hour) // Window [now-18h, now+30h) is already open.
	sub := h.seedGraceSubscription(t, firstUntil)
	remindedAt := h.now.Add(time.Hour)

	if _, err := h.workers.ProcessGraceExpiryReminders(t.Context(), remindedAt); err != nil {
		t.Fatalf("ProcessGraceExpiryReminders() error = %v", err)
	}
	if len(pub.expiring) != 1 {
		t.Fatalf("GraceExpiring published %d times, want 1 for the first window", len(pub.expiring))
	}

	// The subscription recovers (grace exits on a succeeded charge), then a
	// later failed renewal opens a fresh window — EnterGrace must reset the
	// reminded flag so the new window is reminded too.
	stored := h.storedSubscription(t, sub)
	stored.Status = domain.SubscriptionStatusActive
	until := h.now.Add(-time.Hour)
	stored.ValidUntil = &until
	if err := h.stores.subscriptions.Update(t.Context(), stored); err != nil {
		t.Fatalf("recover subscription: %v", err)
	}
	if _, err := h.workers.ProcessRenewals(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessRenewals() error = %v", err)
	}
	inGrace := h.storedSubscription(t, sub)
	if inGrace.Status != domain.SubscriptionStatusGrace {
		t.Fatalf("subscription status = %q, want grace after the second failed renewal", inGrace.Status)
	}
	if inGrace.GraceRemindedAt != nil {
		t.Fatalf("GraceRemindedAt = %v on a fresh window, want nil (reset by EnterGrace)", inGrace.GraceRemindedAt)
	}

	secondUntil := inGrace.ValidUntil
	inside := secondUntil.Add(-lead / 2)
	if n, err := h.workers.ProcessGraceExpiryReminders(t.Context(), inside); err != nil {
		t.Fatalf("ProcessGraceExpiryReminders() error = %v", err)
	} else if n != 1 {
		t.Fatalf("ProcessGraceExpiryReminders() = %d, want 1 for the fresh window", n)
	}
	if len(pub.expiring) != 2 {
		t.Fatalf("GraceExpiring published %d times, want 2 (one per window)", len(pub.expiring))
	}
	if !pub.expiring[1].GraceUntil.Equal(*secondUntil) {
		t.Errorf("second event GraceUntil = %v, want %v", pub.expiring[1].GraceUntil, *secondUntil)
	}
}

// TestWorkers_GraceExpiryReminderPublisherFailure proves the best-effort
// contract for the reminder: the window is marked reminded (at most one
// dispatch attempt per window) and the publisher failure never fails the
// phase (issue #253).
func TestWorkers_GraceExpiryReminderPublisherFailure(t *testing.T) {
	t.Parallel()
	const lead = 48 * time.Hour
	h := newWorkersHarness(t, graceTestConfig(lead))
	pub := &capturePublisher{err: errors.New("publisher down")}
	h.workers.publisher = pub
	graceUntil := h.now.Add(24 * time.Hour)
	sub := h.seedGraceSubscription(t, graceUntil)

	if n, err := h.workers.ProcessGraceExpiryReminders(t.Context(), h.now); err != nil {
		t.Fatalf("ProcessGraceExpiryReminders() error = %v (a publisher failure must not surface)", err)
	} else if n != 1 {
		t.Fatalf("ProcessGraceExpiryReminders() = %d, want 1", n)
	}
	stored := h.storedSubscription(t, sub)
	if stored.GraceRemindedAt == nil {
		t.Fatal("GraceRemindedAt = nil, want the window marked reminded despite the publisher failure")
	}
}
