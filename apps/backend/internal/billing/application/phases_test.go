package application

import (
	"slices"
	"testing"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The phase scheduler of issue #421: the workers' seam for time, tested here on
// a fixed clock so every boundary is deterministic — the renewal onset, the
// grace phases, the notification lead window and the reconciliation staleness
// windows. The tests drive the scheduler's selections through the
// selection-aware fakes (the Go mirror of the SQL predicates, issue #286) and
// pin the boundary instants of the selections themselves.

// phasesNow anchors the scheduler clock; phasesLead and phasesStaleness are
// chosen so every boundary instant below is distinct.
var (
	phasesNow       = time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	phasesLead      = 48 * time.Hour
	phasesStaleness = 5 * time.Minute
)

// phasesHarness wires the scheduler over the fixed clock and the fakes.
type phasesHarness struct {
	scheduler *phaseScheduler
	stores    *fakeStores
	basic     domain.Tariff
	pro       domain.Tariff
}

func newPhasesHarness(t *testing.T) *phasesHarness {
	t.Helper()
	cfg := DefaultConfig()
	cfg.GraceExpiryReminderBefore = phasesLead
	cfg.PendingPaymentStaleness = phasesStaleness
	basic := domain.Tariff{ID: mustNewUUID(), Name: domain.TariffBasic, ActivePropertyLimit: 1, IsActive: true}
	pro := domain.Tariff{
		ID: mustNewUUID(), Name: domain.TariffPro, ActivePropertyLimit: 5,
		MonthlyPriceKopecks: 49000, IsActive: true,
	}
	return &phasesHarness{
		scheduler: newPhaseScheduler(fakeClock{now: phasesNow}, cfg),
		stores:    newFakeStores(basic, pro),
		basic:     basic,
		pro:       pro,
	}
}

// seedSub stores one subscription described by the mutator.
func (h *phasesHarness) seedSub(t *testing.T, mutate func(*domain.Subscription)) domain.Subscription {
	t.Helper()
	sub, err := domain.NewBasicSubscription(mustNewUUID(), h.basic.ID)
	if err != nil {
		t.Fatalf("new subscription: %v", err)
	}
	if mutate != nil {
		mutate(&sub)
	}
	stored, err := h.stores.subscriptions.Create(t.Context(), sub)
	if err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return stored
}

// listedUserIDs runs a subscription selection through the fake listing and
// returns the matched user ids, sorted.
func (h *phasesHarness) listedUserIDs(t *testing.T, sel SubscriptionSelection) []string {
	t.Helper()
	subs, err := h.stores.subscriptions.List(t.Context(), sel)
	if err != nil {
		t.Fatalf("list selection: %v", err)
	}
	ids := make([]string, 0, len(subs))
	for _, sub := range subs {
		ids = append(ids, sub.UserID.String())
	}
	slices.Sort(ids)
	return ids
}

// seedPayment stores one payment built by the caller's mutator, aged past the
// staleness by default so the staleness tests control freshness themselves.
func (h *phasesHarness) seedPayment(
	t *testing.T, sub domain.Subscription, mutate func(*domain.SubscriptionPayment),
) domain.SubscriptionPayment {
	t.Helper()
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth,
		h.pro.MonthlyPriceKopecks, testProviderFake, phasesNow.Add(-2*phasesStaleness))
	if err != nil {
		t.Fatalf("new payment: %v", err)
	}
	if err := payment.SaveProviderReference("prov_"+sub.UserID.String(), "http://pay", phasesNow.Add(-2*phasesStaleness)); err != nil {
		t.Fatalf("save reference: %v", err)
	}
	if mutate != nil {
		mutate(&payment)
	}
	stored, err := h.stores.payments.Create(t.Context(), payment)
	if err != nil {
		t.Fatalf("seed payment: %v", err)
	}
	return stored
}

// listedPaymentIDs runs a payment selection through the fake listing and
// returns the matched payment ids, sorted.
func (h *phasesHarness) listedPaymentIDs(t *testing.T, sel PaymentSelection) []string {
	t.Helper()
	payments, err := h.stores.payments.List(t.Context(), sel)
	if err != nil {
		t.Fatalf("list payment selection: %v", err)
	}
	ids := make([]string, 0, len(payments))
	for _, payment := range payments {
		ids = append(ids, payment.ID.String())
	}
	slices.Sort(ids)
	return ids
}

// idsOf collects the string forms of the given subscriptions' user ids, sorted.
func idsOf(subs ...domain.Subscription) []string {
	ids := make([]string, 0, len(subs))
	for _, sub := range subs {
		ids = append(ids, sub.UserID.String())
	}
	slices.Sort(ids)
	return ids
}

// requireSelection pins a selection's matched rows exactly (want) and proves
// the excluded ids stay out.
func requireSelection(t *testing.T, matched, want []string, excluded ...string) {
	t.Helper()
	if !slices.Equal(matched, want) {
		t.Errorf("selection matched %v, want %v", matched, want)
	}
	for _, id := range excluded {
		if slices.Contains(matched, id) {
			t.Errorf("selection matched %s, want it excluded", id)
		}
	}
}

// requireBoundary asserts the selection's optional time bound equals want.
func requireBoundary(t *testing.T, name string, got *time.Time, want time.Time) {
	t.Helper()
	if got == nil || !got.Equal(want) {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

// TestPhaseScheduler_NowReadsTheInjectedClock proves the scheduler's snapshot
// comes from the injected clock alone — advance the clock, the snapshot moves.
func TestPhaseScheduler_NowReadsTheInjectedClock(t *testing.T) {
	t.Parallel()
	clk := &fakeClock{now: phasesNow}
	sched := newPhaseScheduler(clk, DefaultConfig())
	if !sched.now().Equal(phasesNow) {
		t.Fatalf("now() = %v, want %v", sched.now(), phasesNow)
	}
	clk.now = phasesNow.Add(time.Hour)
	if want := phasesNow.Add(time.Hour); !sched.now().Equal(want) {
		t.Fatalf("now() after advance = %v, want %v", sched.now(), want)
	}
}

// TestPhaseScheduler_RenewalOnset pins the renewal phase's onset boundary: an
// active auto-renewing subscription is due exactly when its paid period has
// ended by the tick — valid_until <= now, the boundary instant included — and
// nothing else is (not the still-valid, not the grace, not the auto-renew-off
// rows). The same closed boundary governs the non-renewing and cancelled
// expiries with their own status and toggle.
func TestPhaseScheduler_RenewalOnset(t *testing.T) {
	t.Parallel()
	h := newPhasesHarness(t)
	now := h.scheduler.now()

	due := h.seedSub(t, func(s *domain.Subscription) {
		s.TariffID = h.pro.ID
		s.AutoRenewEnabled = true
		until := now.Add(-time.Hour)
		s.ValidUntil = &until
	})
	dueExactlyNow := h.seedSub(t, func(s *domain.Subscription) {
		s.AutoRenewEnabled = true
		s.ValidUntil = &now
	})
	stillValid := h.seedSub(t, func(s *domain.Subscription) {
		s.AutoRenewEnabled = true
		until := now.Add(time.Hour)
		s.ValidUntil = &until
	})
	inGrace := h.seedSub(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		s.AutoRenewEnabled = true
		until := now.Add(-time.Hour)
		s.ValidUntil = &until
	})
	renewalOff := h.seedSub(t, func(s *domain.Subscription) {
		s.AutoRenewEnabled = false
		until := now.Add(-time.Hour)
		s.ValidUntil = &until
	})

	sel := h.scheduler.renewalDue(now, 100)
	requireBoundary(t, "renewalDue ValidUntilBefore", sel.ValidUntilBefore, now)
	requireSelection(t, h.listedUserIDs(t, sel), idsOf(due, dueExactlyNow), idsOf(stillValid, inGrace)...)

	nonRenewing := h.scheduler.nonRenewingExpired(now, 100)
	requireBoundary(t, "nonRenewingExpired ValidUntilBefore", nonRenewing.ValidUntilBefore, now)
	requireSelection(t, h.listedUserIDs(t, nonRenewing), idsOf(renewalOff), idsOf(due, stillValid)...)

	if err := renewalOff.Cancel(); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if err := h.stores.subscriptions.Update(t.Context(), renewalOff); err != nil {
		t.Fatalf("store cancelled: %v", err)
	}
	cancelledSel := h.scheduler.cancelledExpired(now, 100)
	requireBoundary(t, "cancelledExpired ValidUntilBefore", cancelledSel.ValidUntilBefore, now)
	requireSelection(t, h.listedUserIDs(t, cancelledSel), idsOf(renewalOff), idsOf(due)...)
}

// TestPhaseScheduler_PendingChangesDue pins the deferred-change boundary: a
// change is due when its clock has passed the tick (pending_change_at <= now,
// the boundary instant included) and only active subscriptions carry one.
func TestPhaseScheduler_PendingChangesDue(t *testing.T) {
	t.Parallel()
	h := newPhasesHarness(t)
	now := h.scheduler.now()

	overdue := h.seedSub(t, func(s *domain.Subscription) {
		target := h.pro.ID
		changeAt := now.Add(-time.Minute)
		s.PendingTariffID = &target
		s.PendingChangeAt = &changeAt
	})
	dueExactlyNow := h.seedSub(t, func(s *domain.Subscription) {
		target := h.pro.ID
		s.PendingTariffID = &target
		s.PendingChangeAt = &now
	})
	scheduledAhead := h.seedSub(t, func(s *domain.Subscription) {
		target := h.pro.ID
		changeAt := now.Add(time.Hour)
		s.PendingTariffID = &target
		s.PendingChangeAt = &changeAt
	})

	sel := h.scheduler.pendingChangesDue(now, 100)
	requireBoundary(t, "pendingChangesDue PendingChangeDue", sel.PendingChangeDue, now)
	requireSelection(t, h.listedUserIDs(t, sel), idsOf(overdue, dueExactlyNow), idsOf(scheduledAhead)...)
}

// TestPhaseScheduler_GraceExpiry pins the expired-grace boundary: a grace
// subscription is downgraded when its grace window has ended by the tick —
// valid_until <= now, the boundary instant included — and a still-open window
// is not expired.
func TestPhaseScheduler_GraceExpiry(t *testing.T) {
	t.Parallel()
	h := newPhasesHarness(t)
	now := h.scheduler.now()

	expired := h.seedSub(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		until := now.Add(-time.Minute)
		s.ValidUntil = &until
	})
	expiredExactlyNow := h.seedSub(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		s.ValidUntil = &now
	})
	windowOpen := h.seedSub(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		until := now.Add(time.Hour)
		s.ValidUntil = &until
	})

	sel := h.scheduler.graceExpired(now, 100)
	requireBoundary(t, "graceExpired ValidUntilBefore", sel.ValidUntilBefore, now)
	requireSelection(t, h.listedUserIDs(t, sel), idsOf(expired, expiredExactlyNow), idsOf(windowOpen)...)
}

// TestPhaseScheduler_GraceReminderLeadWindow pins the notification lead window:
// a grace subscription is reminded while its window ends inside (now,
// now + lead] — open after the tick, closed by the lead — never once the
// window has ended at the tick, never past the lead, and never twice.
func TestPhaseScheduler_GraceReminderLeadWindow(t *testing.T) {
	t.Parallel()
	h := newPhasesHarness(t)
	now := h.scheduler.now()

	inside := h.seedSub(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		until := now.Add(phasesLead / 2)
		s.ValidUntil = &until
	})
	edgeOfLead := h.seedSub(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		until := now.Add(phasesLead)
		s.ValidUntil = &until
	})
	endedAtTick := h.seedSub(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		s.ValidUntil = &now
	})
	beyondLead := h.seedSub(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		until := now.Add(phasesLead + time.Minute)
		s.ValidUntil = &until
	})
	alreadyReminded := h.seedSub(t, func(s *domain.Subscription) {
		s.Status = domain.SubscriptionStatusGrace
		until := now.Add(phasesLead / 2)
		s.ValidUntil = &until
		reminded := now.Add(-time.Minute)
		s.GraceRemindedAt = &reminded
	})

	sel := h.scheduler.graceReminderWindow(now, 100)
	requireBoundary(t, "graceReminderWindow ValidUntilAfter", sel.ValidUntilAfter, now)
	requireBoundary(t, "graceReminderWindow ValidUntilBefore", sel.ValidUntilBefore, now.Add(phasesLead))
	requireSelection(t, h.listedUserIDs(t, sel), idsOf(inside, edgeOfLead),
		idsOf(endedAtTick, beyondLead, alreadyReminded)...)
}

// TestPhaseScheduler_ReconciliationStaleness pins the staleness windows of the
// reconciliation phases: a payment is stale strictly past the configured age —
// created (or updated, for the refunding reservation) before now − staleness,
// the boundary instant itself still fresh — and the upgrade listing narrows to
// the tariff-change payments.
func TestPhaseScheduler_ReconciliationStaleness(t *testing.T) {
	t.Parallel()
	h := newPhasesHarness(t)
	now := h.scheduler.now()
	cutoff := now.Add(-phasesStaleness)

	// On the pro tariff, so the pro payment below is a renewal, not a tariff
	// change — the upgrade listing must not pick it up.
	sub := h.seedSub(t, func(s *domain.Subscription) { s.TariffID = h.pro.ID })
	stale := h.seedPayment(t, sub, nil)
	// The fake mirrors the pending-payments unique index, so the boundary
	// payment needs its own subscription.
	freshSub := h.seedSub(t, func(s *domain.Subscription) { s.TariffID = h.pro.ID })
	freshAtBoundary := h.seedPayment(t, freshSub, func(p *domain.SubscriptionPayment) {
		p.CreatedAt = cutoff
	})

	sel := h.scheduler.stalePending(now, 100)
	requireBoundary(t, "stalePending CreatedBefore", sel.CreatedBefore, cutoff)
	requireSelection(t, h.listedPaymentIDs(t, sel), []string{stale.ID.String()}, freshAtBoundary.ID.String())

	upgradeSub := h.seedSub(t, func(s *domain.Subscription) { s.TariffID = h.basic.ID })
	upgradePayment := h.seedPayment(t, upgradeSub, nil) // Targets pro, subscription on basic.
	upgradeSel := h.scheduler.stalePendingUpgrades(now, 100)
	requireSelection(t, h.listedPaymentIDs(t, upgradeSel), []string{upgradePayment.ID.String()}, stale.ID.String())

	// The fake's pending-payments unique index again: the stuck refund gets
	// its own subscription.
	stuckSub := h.seedSub(t, func(s *domain.Subscription) { s.TariffID = h.pro.ID })
	stuck := h.seedPayment(t, stuckSub, func(p *domain.SubscriptionPayment) {
		if err := p.MarkSucceeded(cutoff.Add(-time.Minute)); err != nil {
			t.Errorf("mark succeeded: %v", err)
		}
		if err := p.BeginRefund(cutoff); err != nil {
			t.Errorf("begin refund: %v", err)
		}
		p.UpdatedAt = cutoff.Add(-time.Minute)
	})
	refundSel := h.scheduler.staleRefunding(now, 100)
	requireBoundary(t, "staleRefunding UpdatedBefore", refundSel.UpdatedBefore, cutoff)
	requireSelection(t, h.listedPaymentIDs(t, refundSel), []string{stuck.ID.String()}, stale.ID.String())
}

// seedGraceEntered logs the subscription's grace entry at the instant — the
// dunning anchor the grace-retry schedule counts from (ticket #431).
func (h *phasesHarness) seedGraceEntered(t *testing.T, sub domain.Subscription, enteredAt time.Time) {
	t.Helper()
	transition := domain.Transition{
		ID:             mustNewUUID(),
		SubscriptionID: sub.ID,
		ToStatus:       domain.SubscriptionStatusGrace,
		ToTariffID:     sub.TariffID,
		Reason:         domain.TransitionReasonGraceEntered,
		Initiator:      domain.InitiatorSystem,
		CreatedAt:      enteredAt,
	}
	if err := h.stores.transitions.Append(t.Context(), transition); err != nil {
		t.Fatalf("seed grace entry: %v", err)
	}
}

// linkActiveMethod stores an active payment method and links it as the
// subscription's charge target — the grace-retry bound requires one.
func (h *phasesHarness) linkActiveMethod(t *testing.T, sub domain.Subscription) {
	t.Helper()
	method, err := domain.NewPaymentMethod(sub.UserID, testProviderFake, "rebill_"+sub.UserID.String(), phasesNow)
	if err != nil {
		t.Fatalf("new method: %v", err)
	}
	method.IsActive = true
	stored, err := h.stores.methods.UpsertByTokenHash(t.Context(), method)
	if err != nil {
		t.Fatalf("seed method: %v", err)
	}
	sub.ActivePaymentMethodID = &stored.ID
	if err := h.stores.subscriptions.Update(t.Context(), sub); err != nil {
		t.Fatalf("link method: %v", err)
	}
}

// graceSub seeds a grace subscription entered at the instant, on the pro
// tariff with auto-renew on, a seven-day window and a linked active method.
func (h *phasesHarness) graceSub(t *testing.T, enteredAt time.Time) domain.Subscription {
	t.Helper()
	sub := h.seedSub(t, func(s *domain.Subscription) {
		s.TariffID = h.pro.ID
		s.Status = domain.SubscriptionStatusGrace
		s.AutoRenewEnabled = true
		until := enteredAt.Add(7 * 24 * time.Hour)
		s.ValidUntil = &until
	})
	h.seedGraceEntered(t, sub, enteredAt)
	h.linkActiveMethod(t, sub)
	return sub
}

// TestPhaseScheduler_GraceRetrySchedule pins the dunning schedule of ticket
// #431: the first retry is due at +24 h from the grace entry, the second at
// +72 h, each boundary consumed by a payment created since the entry (a prior
// retry or a manual one), and never a third. The batch keeps only rows with
// auto-renew on, a linked active method and a still-open window.
func TestPhaseScheduler_GraceRetrySchedule(t *testing.T) {
	t.Parallel()
	h := newPhasesHarness(t)
	now := h.scheduler.now()
	entered := now.Add(-48 * time.Hour)

	// The +24 h boundary passed with nothing charged since the entry.
	firstDue := h.graceSub(t, entered)
	// A retry payment since the entry consumed the +24 h boundary; the +72 h
	// one is still ahead (entered + 72 h > now).
	firstConsumed := h.graceSub(t, entered)
	h.seedPayment(t, firstConsumed, func(p *domain.SubscriptionPayment) {
		p.CreatedAt = entered.Add(25 * time.Hour)
	})
	// Both boundaries passed with one payment since the entry: the second
	// retry is due.
	secondDue := h.graceSub(t, now.Add(-73*time.Hour))
	h.seedPayment(t, secondDue, func(p *domain.SubscriptionPayment) {
		p.CreatedAt = now.Add(-48 * time.Hour)
	})
	// Two payments since the entry: the schedule is spent.
	spent := h.graceSub(t, now.Add(-73*time.Hour))
	h.seedPayment(t, spent, func(p *domain.SubscriptionPayment) {
		p.CreatedAt = now.Add(-70 * time.Hour)
		if err := p.MarkFailed(nil, p.CreatedAt); err != nil {
			t.Errorf("mark failed: %v", err)
		}
	})
	h.seedPayment(t, spent, func(p *domain.SubscriptionPayment) {
		p.CreatedAt = now.Add(-48 * time.Hour)
	})

	// Entered less than 24 h ago: nothing due yet.
	fresh := h.graceSub(t, now.Add(-23*time.Hour))
	// Auto-renew off: the user opted out of charges.
	optedOut := h.graceSub(t, entered)
	optedOut.AutoRenewEnabled = false
	if err := h.stores.subscriptions.Update(t.Context(), optedOut); err != nil {
		t.Fatalf("store opted out: %v", err)
	}
	// No active method linked: nothing to charge.
	noMethod := h.graceSub(t, entered)
	noMethod.ActivePaymentMethodID = nil
	if err := h.stores.subscriptions.Update(t.Context(), noMethod); err != nil {
		t.Fatalf("store no-method: %v", err)
	}
	// The grace window ended: the expired-grace phase owns the row now.
	windowOver := h.graceSub(t, now.Add(-8*24*time.Hour))
	until := now.Add(-time.Hour)
	windowOver.ValidUntil = &until
	if err := h.stores.subscriptions.Update(t.Context(), windowOver); err != nil {
		t.Fatalf("store window over: %v", err)
	}

	sel := h.scheduler.graceRetryDue(now, 100)
	requireBoundary(t, "graceRetryDue GraceRetryDue", sel.GraceRetryDue, now)
	requireSelection(t, h.listedUserIDs(t, sel), idsOf(firstDue, secondDue),
		idsOf(firstConsumed, spent, fresh, optedOut, noMethod, windowOver)...)
}
