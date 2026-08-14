//go:build integration

package application_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// TestSubscriptionRepository_Integration_UpdatePersistsMutations proves Update
// writes every mutable field of the ADR 0008 lifecycle state.
func TestSubscriptionRepository_Integration_UpdatePersistsMutations(t *testing.T) {
	h := newIntegrationHarness(t)
	userID := h.seedUser()
	if err := h.onboarding.OnUserRegistered(h.ctx(), userID); err != nil {
		t.Fatalf("OnUserRegistered() error = %v", err)
	}
	sub, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}

	pro, err := h.tariffs.GetByName(h.ctx(), domain.TariffPro)
	if err != nil {
		t.Fatalf("GetByName(pro) error = %v", err)
	}
	validUntil := h.clock.Now().AddDate(0, 1, 0)
	period := domain.PeriodMonth
	sub.TariffID = pro.ID
	sub.Status = domain.SubscriptionStatusGrace
	sub.ValidUntil = &validUntil
	sub.AutoRenewEnabled = true
	sub.CurrentPeriod = &period
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	reread, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() after update: %v", err)
	}
	if reread.TariffID != pro.ID {
		t.Errorf("TariffID = %v, want pro", reread.TariffID)
	}
	if reread.Status != domain.SubscriptionStatusGrace {
		t.Errorf("Status = %q, want grace", reread.Status)
	}
	if reread.ValidUntil == nil || !reread.ValidUntil.Equal(validUntil) {
		t.Errorf("ValidUntil = %v, want %v", reread.ValidUntil, validUntil)
	}
	if !reread.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = false, want true")
	}
	if reread.CurrentPeriod == nil || *reread.CurrentPeriod != domain.PeriodMonth {
		t.Errorf("CurrentPeriod = %v, want month", reread.CurrentPeriod)
	}
}

// TestSubscriptionRepository_Integration_ForUpdateInsideTx proves the row lock
// variant reads the same aggregate inside a real transaction.
func TestSubscriptionRepository_Integration_ForUpdateInsideTx(t *testing.T) {
	h := newIntegrationHarness(t)
	userID := h.seedUser()
	if err := h.onboarding.OnUserRegistered(h.ctx(), userID); err != nil {
		t.Fatalf("OnUserRegistered() error = %v", err)
	}

	uow := pgdb.NewUoW(h.pool, slog.New(slog.DiscardHandler))
	err := uow.Do(h.ctx(), func(tx transaction.Tx) error {
		locked, err := h.subscriptions.WithTx(tx)
		if err != nil {
			return err
		}
		sub, err := locked.GetByUserIDForUpdate(h.ctx(), userID)
		if err != nil {
			return err
		}
		if sub.Status != domain.SubscriptionStatusActive {
			t.Errorf("Status = %q, want active", sub.Status)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("uow.Do() error = %v", err)
	}
}

// TestSubscriptionRepository_Integration_CreateConflictReturnsExisting proves
// the ON CONFLICT path: creating a subscription for a user who already has one
// returns the existing row instead of failing.
func TestSubscriptionRepository_Integration_CreateConflictReturnsExisting(t *testing.T) {
	h := newIntegrationHarness(t)
	userID := h.seedUser()
	if err := h.onboarding.OnUserRegistered(h.ctx(), userID); err != nil {
		t.Fatalf("OnUserRegistered() error = %v", err)
	}
	first, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}

	basic, err := h.tariffs.GetByName(h.ctx(), domain.TariffBasic)
	if err != nil {
		t.Fatalf("GetByName(basic) error = %v", err)
	}
	duplicate, err := domain.NewBasicSubscription(userID, basic.ID)
	if err != nil {
		t.Fatalf("NewBasicSubscription() error = %v", err)
	}
	returned, err := h.subscriptions.Create(h.ctx(), duplicate)
	if err != nil {
		t.Fatalf("Create() duplicate error = %v", err)
	}
	if returned.ID != first.ID {
		t.Errorf("Create() returned id %v, want the existing %v", returned.ID, first.ID)
	}
}

// TestSubscriptionRepository_Integration_ExpiryWindowRespectedBySchema is a
// schema-level guard: pending_change_at before valid_until violates the CHECK
// constraint, proving the deferred-change invariant lives in the database.
func TestSubscriptionRepository_Integration_ExpiryWindowRespectedBySchema(t *testing.T) {
	h := newIntegrationHarness(t)
	userID := h.seedUser()
	if err := h.onboarding.OnUserRegistered(h.ctx(), userID); err != nil {
		t.Fatalf("OnUserRegistered() error = %v", err)
	}
	sub, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}

	pro, err := h.tariffs.GetByName(h.ctx(), domain.TariffPro)
	if err != nil {
		t.Fatalf("GetByName(pro) error = %v", err)
	}
	basic, err := h.tariffs.GetByName(h.ctx(), domain.TariffBasic)
	if err != nil {
		t.Fatalf("GetByName(basic) error = %v", err)
	}
	validUntil := h.clock.Now().AddDate(0, 1, 0)
	early := h.clock.Now().Add(24 * time.Hour)
	sub.TariffID = pro.ID
	sub.ValidUntil = &validUntil
	sub.PendingTariffID = &basic.ID
	sub.PendingChangeAt = &early
	if err := h.subscriptions.Update(h.ctx(), sub); err == nil {
		t.Error("Update with pending_change_at < valid_until succeeded, want CHECK violation")
	}
}

// TestSubscriptionRepository_Integration_GraceReminderWindowListing proves
// the reminder-window listing of issue #253 against the real schema: only
// unreminded grace subscriptions whose window ends within the lead time and
// has not ended yet are selected, and the reminded flag round-trips through
// Update.
func TestSubscriptionRepository_Integration_GraceReminderWindowListing(t *testing.T) {
	h := newIntegrationHarness(t)
	now := h.clock.Now()

	seedGrace := func(validUntil time.Time) domain.Subscription {
		t.Helper()
		userID := h.seedUser()
		if err := h.onboarding.OnUserRegistered(h.ctx(), userID); err != nil {
			t.Fatalf("OnUserRegistered() error = %v", err)
		}
		sub, err := h.subscriptions.GetByUserID(h.ctx(), userID)
		if err != nil {
			t.Fatalf("GetByUserID() error = %v", err)
		}
		sub.Status = domain.SubscriptionStatusGrace
		sub.ValidUntil = &validUntil
		if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		return sub
	}

	const lead = 48 * time.Hour
	inside := seedGrace(now.Add(24 * time.Hour))
	seedGrace(now.Add(7 * 24 * time.Hour)) // window not open yet
	seedGrace(now.Add(-time.Hour))         // window already ended

	found, err := h.subscriptions.ListInGraceReminderWindow(h.ctx(), now, lead, 100)
	if err != nil {
		t.Fatalf("ListInGraceReminderWindow() error = %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("listed %d subscriptions, want 1 (only the window-inside one)", len(found))
	}
	if found[0].ID != inside.ID {
		t.Errorf("listed subscription = %s, want %s", found[0].ID, inside.ID)
	}

	// Marking the window reminded removes it from the selection, and the
	// flag persists through a re-read.
	reminded := found[0]
	reminded.MarkGraceReminded(now)
	if err := h.subscriptions.Update(h.ctx(), reminded); err != nil {
		t.Fatalf("Update() after MarkGraceReminded: %v", err)
	}
	reread, err := h.subscriptions.GetByUserID(h.ctx(), reminded.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if reread.GraceRemindedAt == nil || !reread.GraceRemindedAt.Equal(now) {
		t.Fatalf("GraceRemindedAt = %v, want %v", reread.GraceRemindedAt, now)
	}
	after, err := h.subscriptions.ListInGraceReminderWindow(h.ctx(), now, lead, 100)
	if err != nil {
		t.Fatalf("ListInGraceReminderWindow() after reminder: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("listed %d subscriptions after reminder, want 0", len(after))
	}
}
