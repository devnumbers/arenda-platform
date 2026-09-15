package application

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The pending-payment form TTL of issue #616: a customer-initiated payment
// carries a 15-minute deadline — persisted on the payment, passed to the
// provider as the same instant — while it is alive it holds the user's tariff
// decision (one live pending payment per user), and once it runs out the TTL
// worker marks the payment failed, which unlocks the tariff choice. A provider
// success arriving after the expiry is reconciled against the provider, the
// shared out-of-order seam.

func TestChangeTariff_LivePendingBlocksOtherSelections(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	first := h.initiateUpgrade(t, sub)

	cases := []struct {
		name   string
		tariff domain.TariffName
		period domain.SubscriptionPeriod
	}{
		{name: "free downgrade", tariff: domain.TariffBasic, period: domain.PeriodMonth},
		{name: "other paid tariff", tariff: domain.TariffPro, period: domain.PeriodYear},
		{name: "same tariff, other period", tariff: domain.TariffBusiness, period: domain.PeriodYear},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
				TariffName: tc.tariff,
				Period:     tc.period,
			})
			if !errors.Is(err, ErrPendingPaymentExists) {
				t.Fatalf("err = %v, want ErrPendingPaymentExists", err)
			}
		})
	}

	repeat, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("repeat ChangeTariff: %v", err)
	}
	if repeat.PaymentID != first.PaymentID {
		t.Errorf("repeat payment = %v, want the existing pending %v", repeat.PaymentID, first.PaymentID)
	}
}

func TestChangeTariff_MerchantInitiatedPendingDoesNotBlock(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub, _ := h.seedStaleMitCharge(t)

	if _, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodYear,
	}); err != nil {
		t.Fatalf("ChangeTariff(upgrade) with a pending merchant-initiated charge: %v", err)
	}
}

func TestCancelSubscription_LivePendingBlocked(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	h.initiateUpgrade(t, sub)

	if err := h.subs.CancelSubscription(t.Context(), sub.UserID, nil); !errors.Is(err, ErrPendingPaymentExists) {
		t.Fatalf("err = %v, want ErrPendingPaymentExists", err)
	}

	// Once the form deadline has run out — what the TTL worker marks — the
	// cancellation unlocks.
	payment := h.singlePending(t, sub.UserID)
	if err := payment.MarkExpired(h.now.Add(16 * time.Minute)); err != nil {
		t.Fatalf("MarkExpired() error = %v", err)
	}
	if err := h.stores.payments.Update(t.Context(), payment); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := h.subs.CancelSubscription(t.Context(), sub.UserID, nil); err != nil {
		t.Fatalf("CancelSubscription() after expiry error = %v", err)
	}
}

func TestChangeTariff_ExpiredPendingCyclesToFreshPayment(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	first := h.initiateUpgrade(t, sub)

	// The worker has not reached the payment yet, but its deadline is spent:
	// the server-side expiry is the truth, so a repeat request must not hand
	// back a dead payment URL.
	stored := h.singlePending(t, sub.UserID)
	if err := stored.MarkExpired(h.now.Add(16 * time.Minute)); err != nil {
		t.Fatalf("MarkExpired() error = %v", err)
	}
	if err := h.stores.payments.Update(t.Context(), stored); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	second, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffBusiness,
		Period:     domain.PeriodMonth,
	})
	if err != nil {
		t.Fatalf("ChangeTariff() after expiry: %v", err)
	}
	if second.PaymentID == first.PaymentID {
		t.Fatal("ChangeTariff() returned the expired pending payment, want a fresh one")
	}
}

func TestSubscriptionView_CarriesPendingPayment(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	h.initiateUpgrade(t, sub)

	view, err := h.subs.GetSubscription(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetSubscription() error = %v", err)
	}
	if view.PendingPayment == nil {
		t.Fatal("GetSubscription() has no pendingPayment, want the live upgrade payment")
	}
	pending := view.PendingPayment
	if pending.Tariff.ID != h.tariffID(t, domain.TariffBusiness) {
		t.Errorf("pending tariff = %v, want business", pending.Tariff.ID)
	}
	if pending.Payment.Period != domain.PeriodMonth {
		t.Errorf("pending period = %q, want month", pending.Payment.Period)
	}
	if pending.Payment.AmountKopecks != 99000 {
		t.Errorf("pending amount = %d, want 99000", pending.Payment.AmountKopecks)
	}
	if !pending.Payment.HasPaymentURL() {
		t.Error("pending payment carries no confirm url")
	}
	if pending.Payment.ExpiresAt == nil || !pending.Payment.ExpiresAt.Equal(h.now.Add(15*time.Minute)) {
		t.Errorf("pending expiresAt = %v, want %v", pending.Payment.ExpiresAt, h.now.Add(15*time.Minute))
	}
}

func TestSubscriptionView_MerchantInitiatedPendingNotSurfaced(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub, _ := h.seedStaleMitCharge(t)

	view, err := h.subs.GetSubscription(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetSubscription() error = %v", err)
	}
	if view.PendingPayment != nil {
		t.Errorf("pendingPayment = %+v, want none — a merchant-initiated charge has no payer form", view.PendingPayment)
	}
}

func TestChangeTariff_PersistsFormDeadlineAndReusesItAtInit(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	result := h.initiateUpgrade(t, sub)

	stored, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.ExpiresAt == nil || !stored.ExpiresAt.Equal(h.now.Add(15*time.Minute)) {
		t.Fatalf("expiresAt = %v, want %v", stored.ExpiresAt, h.now.Add(15*time.Minute))
	}
	if h.provider.initCalls != 1 {
		t.Fatalf("init calls = %d, want 1", h.provider.initCalls)
	}
	if !h.provider.initReqs[0].FormDeadline.Equal(*stored.ExpiresAt) {
		t.Errorf("provider form deadline = %v, want the persisted %v",
			h.provider.initReqs[0].FormDeadline, *stored.ExpiresAt)
	}
}

func TestWebhook_SuccessAfterTTLExpiry_Applies(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	result := h.initiateUpgrade(t, sub)

	// The TTL worker expires the payment before any provider outcome arrives.
	payment := h.singlePending(t, sub.UserID)
	if err := payment.MarkExpired(h.now.Add(16 * time.Minute)); err != nil {
		t.Fatalf("MarkExpired() error = %v", err)
	}
	if err := h.stores.payments.Update(t.Context(), payment); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// The user actually paid at the last second: the late success webhook is
	// reconciled against the provider — the source of truth — and applied.
	h.provider.statusRes = PaymentStatusResult{Status: domain.PaymentStatusSucceeded}
	h.setNotification(&PaymentNotification{
		InternalPaymentID: result.PaymentID,
		ProviderPaymentID: "stub_" + result.PaymentID.String(),
		Status:            domain.PaymentStatusSucceeded,
	})
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err != nil {
		t.Fatalf("late success HandleWebhook() error = %v", err)
	}
	if h.provider.statusCalls == 0 {
		t.Fatal("provider status was never consulted for the late success")
	}

	final, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if final.Status != domain.PaymentStatusSucceeded || final.ErrorCode != nil {
		t.Errorf("payment = %q/%v, want reconciled to succeeded", final.Status, final.ErrorCode)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.TariffID != h.tariffID(t, domain.TariffBusiness) {
		t.Errorf("subscription tariff = %v, want the late payment applied", stored.TariffID)
	}
}

func TestWorkers_ExpirePendingPayments(t *testing.T) {
	t.Parallel()
	h := newWorkersHarness(t, Config{})
	sub := h.seedSubscription(t, nil)

	expired := h.seedExpiredPending(t, sub)
	// A merchant-initiated charge carries no deadline: never TTL-expired.
	mit := h.seedPendingPayment(t, sub, domain.PeriodYear, nil)

	n, err := h.workers.ProcessExpiredPendingPayments(t.Context(), h.now)
	if err != nil {
		t.Fatalf("ProcessExpiredPendingPayments() error = %v", err)
	}
	if n != 1 {
		t.Fatalf("expired = %d, want 1", n)
	}

	stored := h.storedPayment(t, expired.ID)
	if stored.Status != domain.PaymentStatusFailed || stored.ErrorCode == nil || *stored.ErrorCode != domain.PaymentErrorCodeFormExpired {
		t.Errorf("payment = %q/%v, want failed with %q", stored.Status, stored.ErrorCode, domain.PaymentErrorCodeFormExpired)
	}
	if untouched := h.storedPayment(t, mit.ID); untouched.Status != domain.PaymentStatusPending {
		t.Errorf("merchant-initiated payment = %q, want still pending", untouched.Status)
	}
	if subStored := h.storedSubscription(t, sub); subStored.Status != domain.SubscriptionStatusActive {
		t.Errorf("subscription status = %q, want untouched active", subStored.Status)
	}
}

// seedExpiredPending persists a pending form payment whose deadline has
// already run out.
func (h *workersHarness) seedExpiredPending(t *testing.T, sub domain.Subscription) domain.SubscriptionPayment {
	t.Helper()
	created := h.now.Add(-16 * time.Minute)
	deadline := h.now.Add(-time.Minute)
	payment := h.seedPendingPayment(t, sub, domain.PeriodMonth, &created)
	if err := payment.AttachFormDeadline(deadline, created); err != nil {
		t.Fatalf("attach deadline: %v", err)
	}
	if _, err := h.stores.payments.Create(t.Context(), payment); err != nil {
		t.Fatalf("create payment: %v", err)
	}
	return payment
}

// seedPendingPayment persists a pending payment for the subscription; a nil
// createdAt fallback uses the harness now.
func (h *workersHarness) seedPendingPayment(
	t *testing.T, sub domain.Subscription, period domain.SubscriptionPeriod, createdAt *time.Time,
) domain.SubscriptionPayment {
	t.Helper()
	created := h.now
	if createdAt != nil {
		created = *createdAt
	}
	tariffID := h.pro.ID
	amount := int64(49000)
	if period == domain.PeriodYear {
		amount = 440000
	}
	payment, err := domain.NewSubscriptionPayment(sub.UserID, sub.ID, tariffID, period, amount, testProviderFake, created)
	if err != nil {
		t.Fatalf("new payment: %v", err)
	}
	payment, err = h.stores.payments.Create(t.Context(), payment)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	return payment
}

func (h *workersHarness) storedPayment(t *testing.T, id uuid.UUID) domain.SubscriptionPayment {
	t.Helper()
	payment, err := h.stores.payments.GetByID(t.Context(), id)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	return payment
}

// singlePending returns the user's only pending payment, failing the test
// otherwise.
func (h *paymentHarness) singlePending(t *testing.T, userID uuid.UUID) domain.SubscriptionPayment {
	t.Helper()
	pending, err := h.stores.payments.ListPendingByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("ListPendingByUserID() error = %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending payments = %d, want 1", len(pending))
	}
	return pending[0]
}

func TestWebhook_SuccessAfterTTLExpiry_NoPersistedReference_Applies(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)

	// The initiation crashed before the provider reference was saved: the
	// pending payment carries no reference when the TTL worker expires it.
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, h.tariffID(t, domain.TariffBusiness),
		domain.PeriodMonth, 99000, testProviderFake, h.now)
	if err != nil {
		t.Fatalf("new payment: %v", err)
	}
	if err := payment.AttachFormDeadline(h.now.Add(15*time.Minute), h.now); err != nil {
		t.Fatalf("attach deadline: %v", err)
	}
	if _, err := h.stores.payments.Create(t.Context(), payment); err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if err := payment.MarkExpired(h.now.Add(16 * time.Minute)); err != nil {
		t.Fatalf("MarkExpired() error = %v", err)
	}
	if err := h.stores.payments.Update(t.Context(), payment); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// The user paid before the deadline: the late success webhook carries the
	// provider reference itself, the provider confirms the capture, and the
	// success applies.
	h.provider.statusRes = PaymentStatusResult{Status: domain.PaymentStatusSucceeded}
	h.setNotification(&PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: "prov_from_notification",
		Status:            domain.PaymentStatusSucceeded,
	})
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err != nil {
		t.Fatalf("late success HandleWebhook() error = %v", err)
	}

	final, err := h.stores.payments.GetByID(t.Context(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if final.Status != domain.PaymentStatusSucceeded {
		t.Errorf("payment status = %q, want reconciled to succeeded", final.Status)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.TariffID != h.tariffID(t, domain.TariffBusiness) {
		t.Errorf("subscription tariff = %v, want the late payment applied", stored.TariffID)
	}
}

func TestResumeSubscription_LivePendingBlocked(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	sub := h.seedSubscription(t, nil)
	if err := h.subs.CancelSubscription(t.Context(), sub.UserID, nil); err != nil {
		t.Fatalf("seed CancelSubscription() error = %v", err)
	}
	// The reactivation payment (issue #429) is a same-tariff request on the
	// cancelled subscription: the user already holds the form for it, so the
	// free resume would supersede the decision being paid for.
	if _, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	}); err != nil {
		t.Fatalf("seed reactivation payment error = %v", err)
	}
	if err := h.subs.ResumeSubscription(t.Context(), sub.UserID); !errors.Is(err, ErrPendingPaymentExists) {
		t.Fatalf("err = %v, want ErrPendingPaymentExists", err)
	}

	// Once the form deadline has run out, the resume unlocks.
	payment := h.singlePending(t, sub.UserID)
	if err := payment.MarkExpired(h.now.Add(16 * time.Minute)); err != nil {
		t.Fatalf("MarkExpired() error = %v", err)
	}
	if err := h.stores.payments.Update(t.Context(), payment); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := h.subs.ResumeSubscription(t.Context(), sub.UserID); err != nil {
		t.Fatalf("ResumeSubscription() after expiry error = %v", err)
	}
}
