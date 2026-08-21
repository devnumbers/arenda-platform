//go:build integration

package application_test

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// The admin tariff management of issue #256 against real PostgreSQL: the
// repository writes (creation, edit, the duplicate-name backstop, the
// read-cache invalidation) and the acceptance scenario — a price change does
// not reprice the paid period, but the next renewal charges the updated
// price.

// TestTariffRepository_Integration_CreateInsertsAndDuplicateNarrows proves
// Create persists the plan and narrows the name unique violation to
// ErrTariffAlreadyExists' underlying ErrAlreadyExists sentinel.
func TestTariffRepository_Integration_CreateInsertsAndDuplicateNarrows(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	// Free the 'business' name: no subscription references it in a fresh
	// harness, so the row can go and the create below recreates it.
	if _, err := h.pool.Exec(h.ctx(), `DELETE FROM tariffs WHERE name = 'business'`); err != nil {
		t.Fatalf("free business name: %v", err)
	}

	tariff, err := domain.NewTariff(domain.TariffBusiness, 10, 119000, 1090000, true)
	if err != nil {
		t.Fatalf("NewTariff() error = %v", err)
	}
	created, err := h.tariffs.Create(h.ctx(), tariff)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID != tariff.ID || created.Name != domain.TariffBusiness || created.ActivePropertyLimit != 10 {
		t.Errorf("created = %+v, want the persisted plan", created)
	}

	duplicate, err := domain.NewTariff(domain.TariffBusiness, 10, 119000, 1090000, true)
	if err != nil {
		t.Fatalf("NewTariff() error = %v", err)
	}
	if _, err := h.tariffs.Create(h.ctx(), duplicate); !errors.Is(err, billingapp.ErrAlreadyExists) {
		t.Fatalf("Create(duplicate) error = %v, want %v", err, billingapp.ErrAlreadyExists)
	}
}

// TestTariffRepository_Integration_UpdateEditsAndMissNarrows proves Update
// rewrites the editable fields (the name stays) and narrows a miss to
// ErrNotFound.
func TestTariffRepository_Integration_UpdateEditsAndMissNarrows(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	pro, err := h.tariffs.GetByName(h.ctx(), domain.TariffPro)
	if err != nil {
		t.Fatalf("GetByName(pro): %v", err)
	}

	edited := pro
	edited.ActivePropertyLimit = 7
	edited.MonthlyPriceKopecks = 59000
	edited.YearlyPriceKopecks = 540000
	edited.IsActive = false
	updated, err := h.tariffs.Update(h.ctx(), edited)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != domain.TariffPro || updated.ActivePropertyLimit != 7 ||
		updated.MonthlyPriceKopecks != 59000 || updated.IsActive {
		t.Errorf("updated = %+v, want the edited plan with the name kept", updated)
	}

	var storedName string
	var storedLimit int
	var storedActive bool
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT name, active_property_limit, is_active FROM tariffs WHERE id = $1`, pro.ID,
	).Scan(&storedName, &storedLimit, &storedActive); err != nil {
		t.Fatalf("read stored row: %v", err)
	}
	if storedName != "pro" || storedLimit != 7 || storedActive {
		t.Errorf("stored = %s/%d/%v, want pro/7/hidden", storedName, storedLimit, storedActive)
	}

	if _, err := h.tariffs.Update(h.ctx(), domain.Tariff{ID: uuid.Must(uuid.NewV7())}); !errors.Is(err, billingapp.ErrNotFound) {
		t.Fatalf("Update(missing) error = %v, want %v", err, billingapp.ErrNotFound)
	}
}

// primeTariffCaches primes the shared repository's caches with the seeded
// plans — both List entries and the byName map must hold the pre-write row for
// a staleness check to be meaningful — and returns the pro plan.
func (h *integrationHarness) primeTariffCaches(t *testing.T) domain.Tariff {
	t.Helper()
	if _, err := h.tariffs.List(h.ctx()); err != nil {
		t.Fatalf("List() prime: %v", err)
	}
	if _, err := h.tariffs.ListAll(h.ctx()); err != nil {
		t.Fatalf("ListAll() prime: %v", err)
	}
	pro, err := h.tariffs.GetByName(h.ctx(), domain.TariffPro)
	if err != nil {
		t.Fatalf("GetByName(pro): %v", err)
	}
	return pro
}

// commitRepricingThroughTx writes the repriced pro plan through a
// transaction-bound repository instance and commits, the way a committed
// service write does.
func (h *integrationHarness) commitRepricingThroughTx(t *testing.T, pro domain.Tariff) {
	t.Helper()
	uow := pgdb.NewUoW(h.pool, slog.New(slog.DiscardHandler))
	err := uow.Do(h.ctx(), func(tx transaction.Tx) error {
		bound, err := h.tariffs.WithTx(tx)
		if err != nil {
			return err
		}
		edited := pro
		edited.MonthlyPriceKopecks = 59000
		_, err = bound.Update(h.ctx(), edited)
		return err
	})
	if err != nil {
		t.Fatalf("committed update: %v", err)
	}
}

// requireStaleCachedProPrice asserts the shared instance still serves its
// pre-write cached pro row through both reads — the transaction-bound instance
// updated only its own cache, and the fixed clock keeps the shared cache
// inside its TTL.
func (h *integrationHarness) requireStaleCachedProPrice(t *testing.T, proID uuid.UUID) {
	t.Helper()
	cached, err := h.tariffs.GetByName(h.ctx(), domain.TariffPro)
	if err != nil {
		t.Fatalf("GetByName(pro) before invalidate: %v", err)
	}
	if cached.MonthlyPriceKopecks != 49000 {
		t.Fatalf("cached price before Invalidate = %d, want the stale 49000", cached.MonthlyPriceKopecks)
	}
	listed, err := h.tariffs.ListAll(h.ctx())
	if err != nil {
		t.Fatalf("ListAll() before invalidate: %v", err)
	}
	for _, tariff := range listed {
		if tariff.ID == proID && tariff.MonthlyPriceKopecks != 49000 {
			t.Fatalf("list price before Invalidate = %d, want the stale 49000", tariff.MonthlyPriceKopecks)
		}
	}
}

// requireFreshProPrice asserts both reads observe the committed write — only
// the invalidation can produce this under the fixed clock.
func (h *integrationHarness) requireFreshProPrice(t *testing.T, proID uuid.UUID) {
	t.Helper()
	fresh, err := h.tariffs.GetByName(h.ctx(), domain.TariffPro)
	if err != nil {
		t.Fatalf("GetByName(pro) after invalidate: %v", err)
	}
	if fresh.MonthlyPriceKopecks != 59000 {
		t.Errorf("price after Invalidate = %d, want 59000", fresh.MonthlyPriceKopecks)
	}
	freshList, err := h.tariffs.ListAll(h.ctx())
	if err != nil {
		t.Fatalf("ListAll() after invalidate: %v", err)
	}
	for _, tariff := range freshList {
		if tariff.ID == proID && tariff.MonthlyPriceKopecks != 59000 {
			t.Errorf("list price after Invalidate = %d, want 59000", tariff.MonthlyPriceKopecks)
		}
	}
}

// TestTariffRepository_Integration_InvalidateMakesCommittedWriteVisible
// proves why the service invalidates after a committed write: writes go
// through a transaction-bound repository instance, and the shared instance's
// TTL cache would keep serving the pre-write values. Invalidate drops them
// and the next read observes the write. The clock stays fixed, so the fresh
// read can only come from the invalidation, not the TTL.
func TestTariffRepository_Integration_InvalidateMakesCommittedWriteVisible(t *testing.T) {
	t.Parallel()
	h := newIntegrationHarness(t)
	pro := h.primeTariffCaches(t)
	h.commitRepricingThroughTx(t, pro)
	h.requireStaleCachedProPrice(t, pro.ID)

	if err := h.tariffs.Invalidate(h.ctx()); err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}
	h.requireFreshProPrice(t, pro.ID)
}

// TestAdminTariff_Integration_UpdatePersistsWithAudit proves the service edit
// lands the row change and its audit entry in one transaction, and the
// invalidation makes the shared repository observe the change immediately.
func TestAdminTariff_Integration_UpdatePersistsWithAudit(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	adminID := h.seedUser()
	proID := h.tariffIDByName(t, domain.TariffPro)

	updated, err := h.tariffsSvc.UpdateTariff(h.ctx(), adminID, proID, billingapp.UpdateTariffRequest{
		ActivePropertyLimit: 7,
		MonthlyPriceKopecks: 59000,
		YearlyPriceKopecks:  540000,
		IsActive:            false,
	})
	if err != nil {
		t.Fatalf("UpdateTariff() error = %v", err)
	}
	if updated.Name != domain.TariffPro || updated.IsActive {
		t.Errorf("updated = %+v, want hidden pro", updated)
	}

	// The shared repository observes the write without waiting out the TTL
	// (the fixed clock keeps the cache otherwise valid).
	stored, err := h.tariffs.GetByName(h.ctx(), domain.TariffPro)
	if err != nil {
		t.Fatalf("GetByName(pro): %v", err)
	}
	if stored.MonthlyPriceKopecks != 59000 || stored.ActivePropertyLimit != 7 || stored.IsActive {
		t.Errorf("stored = %+v, want the edited plan", stored)
	}
	if n := h.countRows(
		"SELECT COUNT(*) FROM audit_log WHERE action = 'tariff.updated' AND actor_id = $1 AND entity_id = $2",
		adminID, proID); n != 1 {
		t.Errorf("audit rows = %d, want 1", n)
	}

	// The user-facing listing no longer offers the hidden plan.
	active, err := h.tariffs.List(h.ctx())
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	for _, tariff := range active {
		if tariff.ID == proID {
			t.Error("hidden tariff is still offered by the user-facing List")
		}
	}
}

// seedAutoRenewingBusinessPayer onboards a fresh user, lifts them onto an
// auto-renewing month of the business plan and binds an active payment method
// — the state a scheduled paid downgrade starts from. It returns the payer.
func (h *paymentIntegrationHarness) seedAutoRenewingBusinessPayer(t *testing.T) uuid.UUID {
	t.Helper()
	userID := h.seedUser()
	if err := h.onboarding.OnUserRegistered(h.ctx(), userID); err != nil {
		t.Fatalf("OnUserRegistered() error = %v", err)
	}
	sub, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	business, err := h.tariffs.GetByName(h.ctx(), domain.TariffBusiness)
	if err != nil {
		t.Fatalf("GetByName(business): %v", err)
	}
	validUntil := h.clock.Now().AddDate(0, 1, 0)
	period := domain.PeriodMonth
	sub.TariffID = business.ID
	sub.ValidUntil = &validUntil
	sub.AutoRenewEnabled = true
	sub.CurrentPeriod = &period
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("seed Update() error = %v", err)
	}
	seedActiveMethod(t, h.integrationHarness, userID, "tok_deferred_price")
	return userID
}

// expirePaidSubscription re-reads the subscription and moves its validity an
// hour into the past so the renewal phase picks it up (the clock itself stays
// fixed; the re-read keeps the payment-method binding).
func (h *paymentIntegrationHarness) expirePaidSubscription(t *testing.T, userID uuid.UUID) {
	t.Helper()
	expired := h.clock.Now().Add(-time.Hour)
	fresh, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(): %v", err)
	}
	fresh.ValidUntil = &expired
	if err := h.subscriptions.Update(h.ctx(), fresh); err != nil {
		t.Fatalf("expire subscription: %v", err)
	}
}

// requireApplyTimeCharge asserts the deferred change was charged exactly once
// at apply time for the updated price and the subscription now runs on the
// target tariff.
func (h *paymentIntegrationHarness) requireApplyTimeCharge(t *testing.T, userID, wantTariffID uuid.UUID) {
	t.Helper()
	payments, err := h.payments.ListByUserID(h.ctx(), userID)
	if err != nil || len(payments) != 1 {
		t.Fatalf("payments = %d (err %v), want the single apply-time charge", len(payments), err)
	}
	if payments[0].AmountKopecks != 59000 {
		t.Errorf("apply-time amount = %d, want the updated 59000", payments[0].AmountKopecks)
	}
	if payments[0].TariffID != wantTariffID {
		t.Errorf("payment tariff = %v, want the deferred target pro", payments[0].TariffID)
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() after renewal: %v", err)
	}
	if stored.TariffID != wantTariffID {
		t.Errorf("subscription tariff = %v, want the applied pro", stored.TariffID)
	}
}

// TestAdminTariff_Integration_DeferredChangeChargesUpdatedPrice proves the
// second half of the pricing criterion of issue #256: a deferred change to a
// paid target charges the price current at apply time, not at schedule time
// (the paid target is applied by the renewal phase, ADR 0008).
func TestAdminTariff_Integration_DeferredChangeChargesUpdatedPrice(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	adminID := h.seedUser()
	userID := h.seedAutoRenewingBusinessPayer(t)

	// Schedule the business → pro downgrade; it defers to the paid period end.
	if _, err := h.subscriptionsSvc.ChangeTariff(h.ctx(), userID, billingapp.ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	}); err != nil {
		t.Fatalf("ChangeTariff() error = %v", err)
	}

	// Reprice pro after the change was scheduled.
	pro, err := h.tariffs.GetByName(h.ctx(), domain.TariffPro)
	if err != nil {
		t.Fatalf("GetByName(pro): %v", err)
	}
	if _, err := h.tariffsSvc.UpdateTariff(h.ctx(), adminID, pro.ID, billingapp.UpdateTariffRequest{
		ActivePropertyLimit: 5,
		MonthlyPriceKopecks: 59000,
		YearlyPriceKopecks:  540000,
		IsActive:            true,
	}); err != nil {
		t.Fatalf("UpdateTariff() error = %v", err)
	}

	// The paid period ended an hour ago; the deferred change is due.
	h.expirePaidSubscription(t, userID)

	if count, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil || count != 1 {
		t.Fatalf("ProcessRenewals() = %d (err %v), want 1", count, err)
	}
	h.requireApplyTimeCharge(t, userID, pro.ID)
}

// TestAdminTariff_Integration_HiddenTariffSubscriptionStillRenews proves the
// second half of the hiding criterion of issue #256: an existing subscription
// on a hidden tariff keeps renewing — the renewal worker charges it at the
// plan's current price even though the plan is no longer offered or
// selectable, and the subscription stays on it.
func TestAdminTariff_Integration_HiddenTariffSubscriptionStillRenews(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	adminID := h.seedUser()
	userID, sub := seedPaidProSubscription(t, h.integrationHarness)
	seedActiveMethod(t, h.integrationHarness, userID, "tok_hidden_renew")

	// Hide the plan the subscription runs on.
	if _, err := h.tariffsSvc.UpdateTariff(h.ctx(), adminID, sub.TariffID, billingapp.UpdateTariffRequest{
		ActivePropertyLimit: 5,
		MonthlyPriceKopecks: 59000,
		YearlyPriceKopecks:  540000,
		IsActive:            false,
	}); err != nil {
		t.Fatalf("UpdateTariff() error = %v", err)
	}

	// The paid period ended an hour ago. The row is re-read first so the
	// expire-update does not clobber the payment-method binding added above.
	expired := h.clock.Now().Add(-time.Hour)
	fresh, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(): %v", err)
	}
	fresh.ValidUntil = &expired
	if err := h.subscriptions.Update(h.ctx(), fresh); err != nil {
		t.Fatalf("expire subscription: %v", err)
	}

	if count, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil || count != 1 {
		t.Fatalf("ProcessRenewals() = %d (err %v), want 1", count, err)
	}

	payments, err := h.payments.ListByUserID(h.ctx(), userID)
	if err != nil || len(payments) != 1 {
		t.Fatalf("payments = %d (err %v), want the renewal on the hidden plan", len(payments), err)
	}
	if payments[0].Status != domain.PaymentStatusSucceeded || payments[0].AmountKopecks != 59000 {
		t.Errorf("renewal = %s/%d, want succeeded at the current 59000", payments[0].Status, payments[0].AmountKopecks)
	}
	stored, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() after renewal: %v", err)
	}
	if stored.Status != domain.SubscriptionStatusActive || stored.TariffID != sub.TariffID {
		t.Errorf("subscription = %s on %v, want active on the hidden plan", stored.Status, stored.TariffID)
	}
}

// TestAdminTariff_Integration_RenewalChargesUpdatedPrice proves the pricing
// acceptance criterion of issue #256 on the real schema: an admin price edit
// does not reprice the running paid period, and the next renewal charges the
// updated price — including through the read cache, which the service
// invalidated at the edit.
func TestAdminTariff_Integration_RenewalChargesUpdatedPrice(t *testing.T) {
	t.Parallel()
	h := newPaymentIntegrationHarness(t)
	adminID := h.seedUser()
	userID, sub := seedPaidProSubscription(t, h.integrationHarness)
	seedActiveMethod(t, h.integrationHarness, userID, "tok_new_price")

	if _, err := h.tariffsSvc.UpdateTariff(h.ctx(), adminID, sub.TariffID, billingapp.UpdateTariffRequest{
		ActivePropertyLimit: 5,
		MonthlyPriceKopecks: 59000,
		YearlyPriceKopecks:  540000,
		IsActive:            true,
	}); err != nil {
		t.Fatalf("UpdateTariff() error = %v", err)
	}

	// The paid period ended an hour ago (the clock itself stays fixed). The
	// row is re-read first so the expire-update does not clobber the payment
	// method binding added above.
	expired := h.clock.Now().Add(-time.Hour)
	fresh, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(): %v", err)
	}
	fresh.ValidUntil = &expired
	if err := h.subscriptions.Update(h.ctx(), fresh); err != nil {
		t.Fatalf("expire subscription: %v", err)
	}

	if count, err := h.services.Workers.ProcessRenewals(h.ctx(), h.clock.Now()); err != nil || count != 1 {
		t.Fatalf("ProcessRenewals() = %d (err %v), want 1", count, err)
	}

	payments, err := h.payments.ListByUserID(h.ctx(), userID)
	if err != nil || len(payments) != 1 {
		t.Fatalf("payments = %d (err %v), want the single renewal", len(payments), err)
	}
	if payments[0].AmountKopecks != 59000 {
		t.Errorf("renewal amount = %d, want the updated 59000", payments[0].AmountKopecks)
	}
}
