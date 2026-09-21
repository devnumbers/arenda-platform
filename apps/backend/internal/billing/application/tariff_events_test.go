package application

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The tariff events of the notifications catalog (карта #734, решение #737
// №13–№14, #752), tested on the application seam with a capture publisher:
// the shared application seam of an applied success emits PaymentSucceeded
// (and PlanUpgraded for the upgrade leg), the downgrade assignment emits
// PlanDowngradeScheduled, the idempotent no-ops emit nothing, and a failing
// publisher never fails the applied change.

// seedManualPayment seeds an initiated same-or-arbitrary-tariff payment
// pending with its provider reference persisted — the shape a webhook
// success applies.
func (h *paymentHarness) seedManualPayment(
	t *testing.T, sub domain.Subscription, tariffID uuid.UUID, amountKopecks int64,
) domain.SubscriptionPayment {
	t.Helper()
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, tariffID, domain.PeriodMonth, amountKopecks, testProviderFake, h.now)
	if err != nil {
		t.Fatalf("NewSubscriptionPayment() error = %v", err)
	}
	if err := payment.SaveProviderReference("prov_1", "https://pay.example/1", h.now); err != nil {
		t.Fatalf("SaveProviderReference() error = %v", err)
	}
	payment, err = h.stores.payments.Create(t.Context(), payment)
	if err != nil {
		t.Fatalf("seed payment Create() error = %v", err)
	}
	return payment
}

// deliverSuccessNotification feeds the payment a succeeded notification
// through the webhook path.
func (h *paymentHarness) deliverSuccessNotification(t *testing.T, payment domain.SubscriptionPayment) {
	t.Helper()
	h.setNotification(&PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: *payment.ProviderPaymentID,
		Status:            domain.PaymentStatusSucceeded,
		AmountKopecks:     payment.AmountKopecks,
	})
	if err := h.payments.HandleWebhook(t.Context(), testProviderFake, []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}
}

// storedUserSubscription loads the subscription from the fake repository.
func (h *paymentHarness) storedUserSubscription(t *testing.T, userID uuid.UUID) domain.Subscription {
	t.Helper()
	sub, err := h.stores.subscriptions.GetByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	return sub
}

// transitionByID finds the subscription's transition with the given id.
func (h *paymentHarness) transitionByID(t *testing.T, subscriptionID, transitionID uuid.UUID) domain.Transition {
	t.Helper()
	transitions, err := h.stores.transitions.ListBySubscriptionID(t.Context(), subscriptionID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	for _, tr := range transitions {
		if tr.ID == transitionID {
			return tr
		}
	}
	t.Fatalf("transitions contain no %s entry: %+v", transitionID, transitions)
	return domain.Transition{}
}

// TestWebhook_SucceededUpgradePublishesPaymentSucceededAndPlanUpgraded proves
// the acceptance criteria of #752: an upgrade paid through the webhook flow —
// the owner paid for a better plan — publishes both catalog events once the
// finalizing transaction commits: «Оплата прошла» for the payment itself and
// «Тариф изменён» for the activated plan, the applied transition identifying
// the change.
func TestWebhook_SucceededUpgradePublishesPaymentSucceededAndPlanUpgraded(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	pub := &capturePublisher{}
	h.payments.publisher = pub

	sub := h.seedSubscription(t, nil)
	payment := h.seedPendingUpgradeWithReference(t, sub)
	h.deliverSuccessNotification(t, payment)

	if len(pub.succeeded) != 1 {
		t.Fatalf("PaymentSucceeded published %d times, want 1", len(pub.succeeded))
	}
	succeeded := pub.succeeded[0]
	if succeeded.UserID != sub.UserID || succeeded.PaymentID != payment.ID {
		t.Errorf("event identifies user %s payment %s, want user %s payment %s",
			succeeded.UserID, succeeded.PaymentID, sub.UserID, payment.ID)
	}
	if succeeded.TariffID != payment.TariffID || succeeded.AmountKopecks != payment.AmountKopecks {
		t.Errorf("event tariff/amount = %s/%d, want %s/%d",
			succeeded.TariffID, succeeded.AmountKopecks, payment.TariffID, payment.AmountKopecks)
	}
	if succeeded.Period != domain.PeriodMonth {
		t.Errorf("event period = %q, want month", succeeded.Period)
	}
	stored := h.storedUserSubscription(t, sub.UserID)
	if stored.ValidUntil == nil || !succeeded.ActiveUntil.Equal(*stored.ValidUntil) {
		t.Errorf("event ActiveUntil = %v, want the applied validity %v",
			succeeded.ActiveUntil, stored.ValidUntil)
	}

	if len(pub.upgraded) != 1 {
		t.Fatalf("PlanUpgraded published %d times, want 1", len(pub.upgraded))
	}
	upgraded := pub.upgraded[0]
	if upgraded.UserID != sub.UserID || upgraded.TariffID != payment.TariffID {
		t.Errorf("event identifies user %s tariff %s, want user %s tariff %s",
			upgraded.UserID, upgraded.TariffID, sub.UserID, payment.TariffID)
	}
	if upgraded.AmountKopecks != payment.AmountKopecks {
		t.Errorf("event amount = %d, want %d (the charge that activated the plan)",
			upgraded.AmountKopecks, payment.AmountKopecks)
	}
	if upgraded.TransitionID == uuid.Nil {
		t.Error("event transition id is nil — the transition is the dedup entity")
	}
	applied := h.transitionByID(t, sub.ID, upgraded.TransitionID)
	if applied.Reason != domain.TransitionReasonPaymentApplied {
		t.Errorf("event transition reason = %q, want %q", applied.Reason, domain.TransitionReasonPaymentApplied)
	}
}

// TestWebhook_SucceededRenewalPublishesPaymentSucceededOnly proves the
// renewal leg: a same-tariff payment applied to the subscription publishes
// «Оплата прошла» alone — «Тариф изменён» names a plan change, and a renewal
// is none.
func TestWebhook_SucceededRenewalPublishesPaymentSucceededOnly(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	pub := &capturePublisher{}
	h.payments.publisher = pub

	sub := h.seedSubscription(t, nil)
	payment := h.seedManualPayment(t, sub, h.tariffID(t, domain.TariffPro), 49000)
	h.deliverSuccessNotification(t, payment)

	if len(pub.succeeded) != 1 {
		t.Fatalf("PaymentSucceeded published %d times, want 1", len(pub.succeeded))
	}
	if len(pub.upgraded) != 0 {
		t.Errorf("PlanUpgraded published %d times, want 0 — a renewal changes no plan", len(pub.upgraded))
	}
}

// TestWebhook_SucceededDowngradePublishesPaymentSucceededOnly pins the
// catalog's leg split on the rare paid downgrade an applied success can
// carry: the payment notifies (№13), the plan change does not — №14's
// downgrade leg belongs to the assignment, not to a payment.
func TestWebhook_SucceededDowngradePublishesPaymentSucceededOnly(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	pub := &capturePublisher{}
	h.payments.publisher = pub

	sub := h.seedSubscription(t, nil)
	payment := h.seedManualPayment(t, sub, h.tariffID(t, domain.TariffBasic), 100)
	h.deliverSuccessNotification(t, payment)

	if len(pub.succeeded) != 1 {
		t.Fatalf("PaymentSucceeded published %d times, want 1", len(pub.succeeded))
	}
	if len(pub.upgraded) != 0 {
		t.Errorf("PlanUpgraded published %d times, want 0 — a paid downgrade is not the upgrade leg", len(pub.upgraded))
	}
}

// TestWebhook_DuplicateDeliveryEmitsNothing proves the idempotence of the
// emission: a repeated success notification of the same payment applies as a
// no-op and publishes nothing again — the payment is the event's identity.
func TestWebhook_DuplicateDeliveryEmitsNothing(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	pub := &capturePublisher{}
	h.payments.publisher = pub

	sub := h.seedSubscription(t, nil)
	payment := h.seedPendingUpgradeWithReference(t, sub)
	h.deliverSuccessNotification(t, payment)
	h.deliverSuccessNotification(t, payment)

	if len(pub.succeeded) != 1 || len(pub.upgraded) != 1 {
		t.Errorf("events published = %d succeeded / %d upgraded, want 1 / 1",
			len(pub.succeeded), len(pub.upgraded))
	}
}

// TestChangeTariff_DowngradeScheduledPublishesPlanDowngradeScheduled proves
// the downgrade leg of #752: scheduling a period-end downgrade publishes
// «Тариф изменён» once the planning transaction commits, the assignment
// transition identifying the event and the paid period's end dating the
// copy («С {дата} тариф сменится на „{тариф}“»).
func TestChangeTariff_DowngradeScheduledPublishesPlanDowngradeScheduled(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	pub := &capturePublisher{}
	h.subs.publisher = pub

	sub := h.seedSubscription(t, nil)
	validUntil := *sub.ValidUntil

	if _, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffBasic,
		Period:     domain.PeriodMonth,
	}); err != nil {
		t.Fatalf("ChangeTariff(downgrade) error = %v", err)
	}

	if len(pub.downgradeScheduled) != 1 {
		t.Fatalf("PlanDowngradeScheduled published %d times, want 1", len(pub.downgradeScheduled))
	}
	event := pub.downgradeScheduled[0]
	if event.UserID != sub.UserID || event.TariffID != h.tariffID(t, domain.TariffBasic) {
		t.Errorf("event identifies user %s tariff %s, want user %s tariff %s",
			event.UserID, event.TariffID, sub.UserID, h.tariffID(t, domain.TariffBasic))
	}
	if !event.EffectiveAt.Equal(validUntil) {
		t.Errorf("event EffectiveAt = %v, want the paid period's end %v", event.EffectiveAt, validUntil)
	}
	if event.Period != domain.PeriodMonth {
		t.Errorf("event period = %q, want month", event.Period)
	}
	if event.TransitionID == uuid.Nil {
		t.Error("event transition id is nil — the transition is the dedup entity")
	}
	scheduled := h.transitionByID(t, sub.ID, event.TransitionID)
	if scheduled.Reason != domain.TransitionReasonDowngradeScheduled {
		t.Errorf("event transition reason = %q, want %q",
			scheduled.Reason, domain.TransitionReasonDowngradeScheduled)
	}
}

// TestChangeTariff_PublishFailureDoesNotFailPlanning proves the best-effort
// canon: a broken publisher never fails or rolls back the scheduled
// downgrade.
func TestChangeTariff_PublishFailureDoesNotFailPlanning(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	pub := &capturePublisher{err: errors.New("publisher down")}
	h.subs.publisher = pub

	sub := h.seedSubscription(t, nil)
	if _, err := h.subs.ChangeTariff(t.Context(), sub.UserID, ChangeTariffRequest{
		TariffName: domain.TariffBasic,
		Period:     domain.PeriodMonth,
	}); err != nil {
		t.Fatalf("ChangeTariff(downgrade) error = %v, want success (publication is best-effort)", err)
	}
	stored := h.storedUserSubscription(t, sub.UserID)
	if stored.PendingTariffID == nil || *stored.PendingTariffID != h.tariffID(t, domain.TariffBasic) {
		t.Errorf("pending tariff = %v, want basic scheduled", stored.PendingTariffID)
	}
}

// TestWebhook_SucceededUpgradePublishFailureDoesNotFailApplication proves the
// best-effort canon on the payment path: a broken publisher never fails the
// applied success.
func TestWebhook_SucceededUpgradePublishFailureDoesNotFailApplication(t *testing.T) {
	t.Parallel()
	h := newPaymentHarness(t)
	pub := &capturePublisher{err: errors.New("publisher down")}
	h.payments.publisher = pub

	sub := h.seedSubscription(t, nil)
	payment := h.seedPendingUpgradeWithReference(t, sub)
	h.deliverSuccessNotification(t, payment)

	stored := h.storedUserSubscription(t, sub.UserID)
	if stored.TariffID != h.tariffID(t, domain.TariffBusiness) {
		t.Errorf("subscription tariff = %v, want business applied", stored.TariffID)
	}
}

// TestTariffEvents_NilPublisherKeepsSilent proves the pre-#752 constructor
// behaviour: a nil publisher captures but never dispatches.
func TestTariffEvents_NilPublisherKeepsSilent(t *testing.T) {
	t.Parallel()

	carrier := newTariffEvents(nil, slog.New(slog.DiscardHandler))
	carrier.succeeded = &PaymentSucceeded{UserID: uuid.Must(uuid.NewV7())}
	carrier.upgraded = &PlanUpgraded{UserID: uuid.Must(uuid.NewV7())}
	carrier.downgradeScheduled = &PlanDowngradeScheduled{UserID: uuid.Must(uuid.NewV7())}
	carrier.publishAfterCommit(t.Context()) // Must not panic; nothing to assert on.
}
