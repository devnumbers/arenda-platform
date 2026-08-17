//go:build integration

package application_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// TestOnboarding_Integration_CreatesBasicSubscriptionAndTransition proves the
// registration flow lands both the basic subscription and its transition entry
// in one transaction against the real schema (seeded tariffs included).
func TestOnboarding_Integration_CreatesBasicSubscriptionAndTransition(t *testing.T) {
	h := newIntegrationHarness(t)
	userID := h.seedUser()

	if err := h.onboarding.OnUserRegistered(h.ctx(), userID); err != nil {
		t.Fatalf("OnUserRegistered() error = %v", err)
	}

	sub, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("Status = %q, want active", sub.Status)
	}
	if sub.ValidUntil != nil {
		t.Errorf("ValidUntil = %v, want nil", sub.ValidUntil)
	}
	if sub.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = true, want false")
	}

	basic, err := h.tariffs.GetByName(h.ctx(), domain.TariffBasic)
	if err != nil {
		t.Fatalf("GetByName(basic) error = %v — seed missing?", err)
	}
	if sub.TariffID != basic.ID {
		t.Errorf("TariffID = %v, want the seeded basic tariff %v", sub.TariffID, basic.ID)
	}

	transitions, err := h.transitions.ListBySubscriptionID(h.ctx(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	if len(transitions) != 1 {
		t.Fatalf("transitions = %d, want 1", len(transitions))
	}
	if transitions[0].Reason != domain.TransitionReasonRegistered {
		t.Errorf("Reason = %q, want %q", transitions[0].Reason, domain.TransitionReasonRegistered)
	}
}

// TestOnboarding_Integration_RedeliveryIsIdempotent proves a repeated
// user_registered event neither duplicates the subscription nor the transition.
func TestOnboarding_Integration_RedeliveryIsIdempotent(t *testing.T) {
	h := newIntegrationHarness(t)
	userID := h.seedUser()

	for range 2 {
		if err := h.onboarding.OnUserRegistered(h.ctx(), userID); err != nil {
			t.Fatalf("OnUserRegistered() error = %v", err)
		}
	}

	if got := h.countRows("SELECT count(*) FROM user_subscriptions WHERE user_id = $1", userID); got != 1 {
		t.Errorf("subscriptions = %d, want 1", got)
	}
	if got := h.countRows("SELECT count(*) FROM subscription_transitions"); got != 1 {
		t.Errorf("transitions = %d, want 1", got)
	}
}

// TestSubscriptionService_Integration_GetSubscription proves the read model
// assembles over the real repositories, including ErrSubscriptionNotFound for
// a user without a subscription.
func TestSubscriptionService_Integration_GetSubscription(t *testing.T) {
	h := newIntegrationHarness(t)

	if _, err := h.subscriptionsSvc.GetSubscription(h.ctx(), uuid.Must(uuid.NewV7())); err == nil {
		t.Fatal("GetSubscription() error = nil, want ErrSubscriptionNotFound")
	}

	userID := h.seedUser()
	if err := h.onboarding.OnUserRegistered(h.ctx(), userID); err != nil {
		t.Fatalf("OnUserRegistered() error = %v", err)
	}

	view, err := h.subscriptionsSvc.GetSubscription(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetSubscription() error = %v", err)
	}
	if view.Tariff.Name != domain.TariffBasic {
		t.Errorf("Tariff.Name = %q, want %q", view.Tariff.Name, domain.TariffBasic)
	}
	if view.PendingTariff != nil {
		t.Errorf("PendingTariff = %+v, want nil", *view.PendingTariff)
	}
}

// TestTariffService_Integration_ListsSeededActiveTariffs proves the destructive
// migration seeded the three canonical tariffs and the listing serves them.
func TestTariffService_Integration_ListsSeededActiveTariffs(t *testing.T) {
	h := newIntegrationHarness(t)

	tariffs, err := h.tariffsSvc.ListTariffs(h.ctx())
	if err != nil {
		t.Fatalf("ListTariffs() error = %v", err)
	}
	if len(tariffs) != 3 {
		t.Fatalf("tariffs = %d, want 3 (migration seed)", len(tariffs))
	}
}

// TestTariffRepository_Integration_HiddenTariffNotListed proves is_active=false
// hides a tariff from the user listing while GetByName still resolves it.
func TestTariffRepository_Integration_HiddenTariffNotListed(t *testing.T) {
	h := newIntegrationHarness(t)

	if _, err := h.pool.Exec(h.ctx(), "UPDATE tariffs SET is_active = false WHERE name = 'business'"); err != nil {
		t.Fatalf("hide business tariff: %v", err)
	}

	tariffs, err := h.tariffs.List(h.ctx())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(tariffs) != 2 {
		t.Fatalf("tariffs = %d, want 2 (business hidden)", len(tariffs))
	}

	// The hidden tariff stays resolvable: FKs keep pointing at it.
	hidden, err := h.tariffs.GetByName(h.ctx(), domain.TariffBusiness)
	if err != nil {
		t.Fatalf("GetByName(business) error = %v, want the hidden row", err)
	}
	if hidden.IsActive {
		t.Error("IsActive = true, want false")
	}
}

// TestLimiter_Integration_ActivePropertyLimitFollowsSubscription proves the
// cross-context bridge reads the limit through the real schema: basic is 1,
// no subscription is 0.
func TestLimiter_Integration_ActivePropertyLimitFollowsSubscription(t *testing.T) {
	h := newIntegrationHarness(t)

	userIdle := h.seedUser()
	got, err := h.limiter.ActivePropertyLimit(h.ctx(), userIdle)
	if err != nil {
		t.Fatalf("ActivePropertyLimit() error = %v", err)
	}
	if got != 0 {
		t.Errorf("limit without subscription = %d, want 0", got)
	}

	userID := h.seedUser()
	if err := h.onboarding.OnUserRegistered(h.ctx(), userID); err != nil {
		t.Fatalf("OnUserRegistered() error = %v", err)
	}
	got, err = h.limiter.ActivePropertyLimit(h.ctx(), userID)
	if err != nil {
		t.Fatalf("ActivePropertyLimit() error = %v", err)
	}
	if got != 1 {
		t.Errorf("basic limit = %d, want 1", got)
	}
}

// TestTransitionRepository_Integration_AppendOnly proves the append-only guard
// rejects UPDATE at the database level. DELETE stays unguarded by design:
// cascading user/subscription deletions fire row-level triggers and must be
// able to remove the log together with its subscription (ADR 0037).
func TestTransitionRepository_Integration_AppendOnly(t *testing.T) {
	h := newIntegrationHarness(t)
	userID := h.seedUser()
	if err := h.onboarding.OnUserRegistered(h.ctx(), userID); err != nil {
		t.Fatalf("OnUserRegistered() error = %v", err)
	}

	if _, err := h.pool.Exec(h.ctx(), "UPDATE subscription_transitions SET reason = 'forged'"); err == nil {
		t.Error("UPDATE on transition log succeeded, want rejected by trigger")
	}

	// Cascading delete (user erasure) removes the log together with the
	// subscription instead of failing on the immutability guard.
	if _, err := h.pool.Exec(h.ctx(), "DELETE FROM users WHERE id = $1", userID); err != nil {
		t.Fatalf("cascade delete with transitions failed: %v", err)
	}
	if got := h.countRows("SELECT count(*) FROM subscription_transitions"); got != 0 {
		t.Errorf("transitions after user delete = %d, want 0 (cascade)", got)
	}
}

// TestWorkers_Integration_EmptyDatabaseIsNoOp proves the worker phases tick
// over an empty database without errors or side effects (the lifecycle
// scenarios themselves live in workers_integration_test.go, issue #252).
func TestWorkers_Integration_EmptyDatabaseIsNoOp(t *testing.T) {
	h := newIntegrationHarness(t)
	for name, phase := range map[string]func() (int, error){
		"ProcessScheduledChanges": func() (int, error) { return h.services.Workers.ProcessScheduledChanges(h.ctx(), h.clock.Now()) },
		"ProcessRenewals":         func() (int, error) { return h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()) },
		"ProcessPendingUpgradePayments": func() (int, error) {
			return h.services.Workers.ProcessPendingUpgradePayments(h.ctx(), h.clock.Now())
		},
		"ProcessExpiredGrace": func() (int, error) { return h.services.Workers.ProcessExpiredGrace(h.ctx(), h.clock.Now()) },
		"ReconcilePendingPayments": func() (int, error) {
			return h.services.Workers.ReconcilePendingPayments(h.ctx(), h.clock.Now())
		},
	} {
		count, err := phase()
		if err != nil {
			t.Fatalf("%s() error = %v", name, err)
		}
		if count != 0 {
			t.Errorf("%s() = %d, want 0", name, count)
		}
	}
}
