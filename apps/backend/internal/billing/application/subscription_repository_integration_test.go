//go:build integration

package application_test

import (
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// TestSubscriptionRepository_Integration_UpdatePersistsMutations proves Update
// writes every mutable field of the ADR 0008 lifecycle state.
func TestSubscriptionRepository_Integration_UpdatePersistsMutations(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// seedLifecycleSelectionRow onboards a fresh user, applies the mutation to
// their subscription and persists it — one labelled row of the ListSelection
// state matrix.
func seedLifecycleSelectionRow(
	t *testing.T, h *integrationHarness, name string, mutate func(*domain.Subscription),
) domain.Subscription {
	t.Helper()
	userID := h.seedUser()
	if err := h.onboarding.OnUserRegistered(h.ctx(), userID); err != nil {
		t.Fatalf("OnUserRegistered(%s) error = %v", name, err)
	}
	sub, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(%s) error = %v", name, err)
	}
	if mutate != nil {
		mutate(&sub)
	}
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("Update(%s) error = %v", name, err)
	}
	return sub
}

// requireSelectionUserIDs asserts the selection lists exactly the wanted
// users in order.
func requireSelectionUserIDs(t *testing.T, h *integrationHarness, sel billingapp.SubscriptionSelection, want []uuid.UUID) {
	t.Helper()
	found, err := h.subscriptions.List(h.ctx(), sel)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	got := make([]uuid.UUID, 0, len(found))
	for _, sub := range found {
		got = append(got, sub.UserID)
	}
	if !slices.Equal(got, want) {
		t.Errorf("List() = %v, want %v", got, want)
	}
}

// requireNarrowedSelectionIsMembershipRecheck proves the user-narrowed
// selection is the under-lock membership re-check: the listed row is still in
// the batch, any other state is not.
func requireNarrowedSelectionIsMembershipRecheck(
	t *testing.T, h *integrationHarness, renewSel billingapp.SubscriptionSelection,
	listed, foreign domain.Subscription,
) {
	t.Helper()
	narrowed := renewSel
	narrowed.UserID = &listed.UserID
	found, err := h.subscriptions.List(h.ctx(), narrowed)
	if err != nil {
		t.Fatalf("List(narrowed) error = %v", err)
	}
	if len(found) != 1 || found[0].UserID != listed.UserID {
		t.Errorf("List(narrowed to the listed row) = %+v, want the row itself", found)
	}
	narrowed.UserID = &foreign.UserID
	found, err = h.subscriptions.List(h.ctx(), narrowed)
	if err != nil {
		t.Fatalf("List(narrowed to foreign state) error = %v", err)
	}
	if len(found) != 0 {
		t.Errorf("List(narrowed to a subscription outside the selection) = %d rows, want 0", len(found))
	}
}

// requireReminderFlagDropsRow proves marking the grace window reminded removes
// it from the reminder selection and the flag persists through a re-read.
func requireReminderFlagDropsRow(
	t *testing.T, h *integrationHarness, sub domain.Subscription, reminderSel billingapp.SubscriptionSelection, now time.Time,
) {
	t.Helper()
	reminded := sub
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
	after, err := h.subscriptions.List(h.ctx(), reminderSel)
	if err != nil {
		t.Fatalf("List(reminder) after reminder: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("List(reminder) after reminder = %d rows, want 0", len(after))
	}
}

// TestSubscriptionRepository_Integration_ListSelection proves the
// parameterized worker selection (issue #286) against the real schema: every
// phase's selection picks exactly its batch from a state matrix, the batch
// order follows the phase clock, and a user-narrowed selection is the
// under-lock membership re-check. The matrix is immutable, so the subtests
// run in parallel; the mutating reminder-flag check lives in its own test
// because parallel subtests observe the seeded state only after the parent
// body has finished.
func TestSubscriptionRepository_Integration_ListSelection(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	now := h.clock.Now()
	seed := func(name string, mutate func(*domain.Subscription)) domain.Subscription {
		t.Helper()
		return seedLifecycleSelectionRow(t, h, name, mutate)
	}

	basic, err := h.tariffs.GetByName(h.ctx(), domain.TariffBasic)
	if err != nil {
		t.Fatalf("GetByName(basic) error = %v", err)
	}

	// The state matrix of the ADR 0008 lifecycle.
	renewOlder := seed("renew older expired", func(s *domain.Subscription) {
		until := now.Add(-48 * time.Hour)
		s.ValidUntil = &until
		s.AutoRenewEnabled = true
	})
	renewExpired := seed("renew expired", func(s *domain.Subscription) {
		until := now.Add(-time.Hour)
		s.ValidUntil = &until
		s.AutoRenewEnabled = true
	})
	seed("renew paid ahead", func(s *domain.Subscription) {
		until := now.Add(7 * 24 * time.Hour)
		s.ValidUntil = &until
		s.AutoRenewEnabled = true
	})
	nonRenewing := seed("non-renewing expired", func(s *domain.Subscription) {
		until := now.Add(-time.Hour)
		s.ValidUntil = &until
		s.AutoRenewEnabled = false
	})
	graceExpired := seed("grace expired", func(s *domain.Subscription) {
		until := now.Add(-time.Hour)
		s.Status = domain.SubscriptionStatusGrace
		s.ValidUntil = &until
	})
	graceInWindow := seed("grace inside reminder window", func(s *domain.Subscription) {
		until := now.Add(24 * time.Hour)
		s.Status = domain.SubscriptionStatusGrace
		s.ValidUntil = &until
	})
	seed("grace window already reminded", func(s *domain.Subscription) {
		until := now.Add(24 * time.Hour)
		s.Status = domain.SubscriptionStatusGrace
		s.ValidUntil = &until
		s.MarkGraceReminded(now.Add(-time.Hour))
	})
	seed("grace window not open yet", func(s *domain.Subscription) {
		until := now.Add(7 * 24 * time.Hour)
		s.Status = domain.SubscriptionStatusGrace
		s.ValidUntil = &until
	})
	cancelledExpired := seed("cancelled expired", func(s *domain.Subscription) {
		until := now.Add(-time.Hour)
		s.Status = domain.SubscriptionStatusCancelled
		s.ValidUntil = &until
	})
	seed("cancelled retained", func(s *domain.Subscription) {
		until := now.Add(7 * 24 * time.Hour)
		s.Status = domain.SubscriptionStatusCancelled
		s.ValidUntil = &until
	})
	// A deferred change rides an auto-renewing subscription (ScheduleDowngrade
	// enables auto-renew), so the due one also lands in the renewal batch —
	// the renewal phase charges a paid target at apply time.
	pendingDue := seed("pending change due", func(s *domain.Subscription) {
		// The schema CHECK keeps pending_change_at at or after valid_until.
		until := now.Add(-2 * time.Hour)
		changeAt := now.Add(-time.Hour)
		s.ValidUntil = &until
		s.AutoRenewEnabled = true
		s.PendingTariffID = &basic.ID
		s.PendingChangeAt = &changeAt
	})
	seed("pending change not due", func(s *domain.Subscription) {
		until := now.Add(24 * time.Hour)
		changeAt := now.Add(48 * time.Hour)
		s.ValidUntil = &until
		s.AutoRenewEnabled = true
		s.PendingTariffID = &basic.ID
		s.PendingChangeAt = &changeAt
	})

	renewSel := billingapp.SubscriptionSelection{
		Status:           domain.SubscriptionStatusActive,
		AutoRenewEnabled: new(true),
		ValidUntilBefore: new(now),
		Limit:            100,
	}
	graceSel := billingapp.SubscriptionSelection{
		Status:           domain.SubscriptionStatusGrace,
		ValidUntilBefore: new(now),
		Limit:            100,
	}
	const lead = 48 * time.Hour
	reminderSel := billingapp.SubscriptionSelection{
		Status:           domain.SubscriptionStatusGrace,
		ValidUntilAfter:  new(now),
		ValidUntilBefore: new(now.Add(lead)),
		Unreminded:       true,
		Limit:            100,
	}
	nonRenewingSel := billingapp.SubscriptionSelection{
		Status:           domain.SubscriptionStatusActive,
		AutoRenewEnabled: new(false),
		ValidUntilBefore: new(now),
		Limit:            100,
	}
	cancelledSel := billingapp.SubscriptionSelection{
		Status:           domain.SubscriptionStatusCancelled,
		ValidUntilBefore: new(now),
		Limit:            100,
	}
	pendingSel := billingapp.SubscriptionSelection{
		Status:           domain.SubscriptionStatusActive,
		PendingChangeDue: new(now),
		Limit:            100,
	}

	for _, tc := range []struct {
		name string
		sel  billingapp.SubscriptionSelection
		want []uuid.UUID
	}{
		{
			name: "up for renewal, oldest window first", sel: renewSel,
			want: []uuid.UUID{renewOlder.UserID, pendingDue.UserID, renewExpired.UserID},
		},
		{name: "expired grace", sel: graceSel, want: []uuid.UUID{graceExpired.UserID}},
		{name: "grace reminder window", sel: reminderSel, want: []uuid.UUID{graceInWindow.UserID}},
		{name: "expired non-renewing", sel: nonRenewingSel, want: []uuid.UUID{nonRenewing.UserID}},
		{name: "expired cancelled", sel: cancelledSel, want: []uuid.UUID{cancelledExpired.UserID}},
		{name: "pending changes due", sel: pendingSel, want: []uuid.UUID{pendingDue.UserID}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			requireSelectionUserIDs(t, h, tc.sel, tc.want)
		})
	}

	requireNarrowedSelectionIsMembershipRecheck(t, h, renewSel, renewExpired, graceInWindow)
}

// TestSubscriptionRepository_Integration_ReminderFlagDropsRow proves marking
// the grace window reminded persists the flag through a re-read and removes
// the row from the reminder selection. It runs on its own harness because the
// mutation must not race the read-only matrix subtests of ListSelection.
func TestSubscriptionRepository_Integration_ReminderFlagDropsRow(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	now := h.clock.Now()
	const lead = 48 * time.Hour

	unreminded := seedLifecycleSelectionRow(t, h, "grace inside reminder window", func(s *domain.Subscription) {
		until := now.Add(24 * time.Hour)
		s.Status = domain.SubscriptionStatusGrace
		s.ValidUntil = &until
	})
	reminderSel := billingapp.SubscriptionSelection{
		Status:           domain.SubscriptionStatusGrace,
		ValidUntilAfter:  new(now),
		ValidUntilBefore: new(now.Add(lead)),
		Unreminded:       true,
		Limit:            100,
	}
	requireSelectionUserIDs(t, h, reminderSel, []uuid.UUID{unreminded.UserID})
	requireReminderFlagDropsRow(t, h, unreminded, reminderSel, now)
}
