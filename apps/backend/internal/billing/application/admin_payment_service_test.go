package application

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The refund-saga scenarios of issue #254, ported from the pre-rewrite tests
// TestBilling_RefundPayment_* (the behavioural specification: double refund,
// provider failures, in-flight refunds, the partial-refund anomaly, stuck
// states) and extended with the rewritten module's own seams — the transition
// log, the audit actor and the lifecycle bridges.

// refundHarness wires the payment service over the in-memory fakes with the
// lifecycle bridges attached, so the refund tests exercise the full
// subscription effects of the saga.
type refundHarness struct {
	*paymentHarness
	adminID  uuid.UUID
	archiver *fakeArchiverSource
	slots    *fakeSlotSource
}

func newRefundHarness(t *testing.T) *refundHarness {
	t.Helper()
	h := newPaymentHarness(t)
	archiver := &fakeArchiverSource{}
	slots := &fakeSlotSource{}
	h.payments.SetLifecycleBridges(archiver, slots)
	return &refundHarness{
		paymentHarness: h,
		adminID:        uuid.MustParse("e0eebc99-9c0b-4ef8-bb6d-6bb9bd380a99"),
		archiver:       archiver,
		slots:          slots,
	}
}

// succeededUpgradePayment drives a full upgrade to the business plan and
// returns the succeeded payment — the captured charge every refund starts
// from.
func (h *refundHarness) succeededUpgradePayment(t *testing.T) domain.SubscriptionPayment {
	t.Helper()
	sub := h.seedSubscription(t, nil)
	result := h.initiateUpgrade(t, sub)
	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if err := h.webhookSucceeded(t, payment); err != nil {
		t.Fatalf("webhookSucceeded() error = %v", err)
	}
	stored, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID(final) error = %v", err)
	}
	return stored
}

// TestRefundPayment_SucceedsAndDowngradesToBasic proves the happy path of the
// saga (migrated from TestBilling_RefundPayment_SucceedsAndDowngradesToBasic):
// the reservation is committed before the provider call, the provider refunds
// the full amount, and the finalizing transaction marks the payment refunded,
// downgrades the subscription to basic with the excess properties archived,
// and records the transition-log entry and the audit entry attributed to the
// acting admin.
func TestRefundPayment_SucceedsAndDowngradesToBasic(t *testing.T) {
	h := newRefundHarness(t)
	sub := h.seedSubscription(t, nil)
	result := h.initiateUpgrade(t, sub)
	payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if err := h.webhookSucceeded(t, payment); err != nil {
		t.Fatalf("webhookSucceeded() error = %v", err)
	}

	if err := h.payments.RefundPayment(t.Context(), h.adminID, payment.ID); err != nil {
		t.Fatalf("RefundPayment() error = %v", err)
	}

	if h.provider.refundCalls != 1 {
		t.Fatalf("provider refund calls = %d, want exactly one", h.provider.refundCalls)
	}
	req := h.provider.refundReqs[0]
	if req.PaymentID != payment.ID || req.ProviderPaymentID != *payment.ProviderPaymentID {
		t.Errorf("refund request = %v, want the internal and provider payment ids", req)
	}
	if req.AmountKopecks != payment.AmountKopecks {
		t.Errorf("refund amount = %d, want the full %d", req.AmountKopecks, payment.AmountKopecks)
	}

	stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID(refunded) error = %v", err)
	}
	if stored.Status != domain.PaymentStatusRefunded {
		t.Errorf("payment status = %q, want refunded", stored.Status)
	}
	if stored.RefundedAmountKopecks == nil || *stored.RefundedAmountKopecks != payment.AmountKopecks {
		t.Errorf("refunded amount = %v, want the full %d", stored.RefundedAmountKopecks, payment.AmountKopecks)
	}

	subStored, err := h.stores.subscriptions.GetByUserID(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if subStored.TariffID != h.tariffID(t, domain.TariffBasic) {
		t.Errorf("tariff = %v, want basic after the refund", subStored.TariffID)
	}
	if subStored.ValidUntil != nil || subStored.AutoRenewEnabled {
		t.Errorf("subscription = valid_until %v, auto-renew %t; want both cleared", subStored.ValidUntil, subStored.AutoRenewEnabled)
	}

	// The transition log attributes the downgrade to the acting admin and the
	// refunded payment.
	transitions, err := h.stores.transitions.ListBySubscriptionID(t.Context(), subStored.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	var refundTransition *domain.Transition
	for i := range transitions {
		if transitions[i].Reason == domain.TransitionReasonRefunded {
			refundTransition = &transitions[i]
		}
	}
	if refundTransition == nil {
		t.Fatalf("transitions contain no refunded entry: %+v", transitions)
	}
	if refundTransition.Initiator != domain.InitiatorAdmin || refundTransition.InitiatorID == nil || *refundTransition.InitiatorID != h.adminID {
		t.Errorf("refund transition initiator = %q/%v, want the acting admin", refundTransition.Initiator, refundTransition.InitiatorID)
	}
	if refundTransition.PaymentID == nil || *refundTransition.PaymentID != payment.ID {
		t.Errorf("refund transition payment = %v, want %v", refundTransition.PaymentID, payment.ID)
	}

	// The audit entry attributes the refund to the admin.
	var refundAudit *auditdomain.Entry
	for _, entry := range h.audit.recorded() {
		if entry.Action == auditdomain.ActionSubscriptionPaymentRefunded {
			refundAudit = &entry
		}
	}
	if refundAudit == nil {
		t.Fatalf("audit contains no refunded entry: %+v", h.audit.recorded())
	}
	if refundAudit.ActorRole != auditdomain.ActorRoleAdmin || refundAudit.ActorID == nil || *refundAudit.ActorID != h.adminID {
		t.Errorf("refund audit actor = %q/%v, want the acting admin", refundAudit.ActorRole, refundAudit.ActorID)
	}

	// The lifecycle bridges ran inside the finalizing transaction: the excess
	// properties beyond the basic limit were archived and the recipient slots
	// enforced with the refund trigger.
	calls := h.archiver.recorded()
	if len(calls) != 1 || calls[0].limit != 1 {
		t.Errorf("archiver calls = %v, want one call with the basic limit 1", calls)
	}
	triggers := h.slots.recorded()
	if len(triggers) != 1 || triggers[0] != triggerRefund {
		t.Errorf("slot triggers = %v, want [%s]", triggers, triggerRefund)
	}
}

// TestRefundPayment_DoubleRefundRejected proves the double-refund guard
// (migrated from TestBilling_RefundPayment_DoubleRefundRejected): a repeated
// refund of the same payment is rejected by the reservation, and the provider
// is never asked twice.
func TestRefundPayment_DoubleRefundRejected(t *testing.T) {
	h := newRefundHarness(t)
	payment := h.succeededUpgradePayment(t)

	if err := h.payments.RefundPayment(t.Context(), h.adminID, payment.ID); err != nil {
		t.Fatalf("first RefundPayment() error = %v", err)
	}
	err := h.payments.RefundPayment(t.Context(), h.adminID, payment.ID)
	if !errors.Is(err, domain.ErrInvalidPaymentStatus) {
		t.Fatalf("second RefundPayment() err = %v, want ErrInvalidPaymentStatus", err)
	}
	if h.provider.refundCalls != 1 {
		t.Errorf("provider refund calls = %d, want one", h.provider.refundCalls)
	}
}

// TestRefundPayment_RejectedBeforeProviderCall proves the reservation rejects
// everything a full refund cannot start from — terminal states and a payment
// without a provider reference — before the provider is called
// (migrated from TestBilling_RefundPayment_RejectedForNonSucceededOrPendingPayment
// and TestBilling_RefundPayment_RejectedWhenProviderPaymentIDMissing).
func TestRefundPayment_RejectedBeforeProviderCall(t *testing.T) {
	h := newRefundHarness(t)

	t.Run("failed payment", func(t *testing.T) {
		sub := h.seedSubscription(t, nil)
		result := h.initiateUpgrade(t, sub)
		payment, err := h.stores.payments.GetByID(t.Context(), result.PaymentID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if err := payment.MarkFailed(nil, h.now); err != nil {
			t.Fatalf("MarkFailed() error = %v", err)
		}
		if err := h.stores.payments.Update(t.Context(), payment); err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if err := h.payments.RefundPayment(t.Context(), h.adminID, payment.ID); !errors.Is(err, domain.ErrInvalidPaymentStatus) {
			t.Errorf("err = %v, want ErrInvalidPaymentStatus", err)
		}
	})

	t.Run("payment without a provider reference", func(t *testing.T) {
		sub := h.seedSubscription(t, nil)
		payment, err := domain.NewSubscriptionPayment(sub.UserID, sub.ID, h.tariffID(t, domain.TariffBusiness), domain.PeriodMonth, 99000, "fake", h.now)
		if err != nil {
			t.Fatalf("NewSubscriptionPayment() error = %v", err)
		}
		created, err := h.stores.payments.Create(t.Context(), payment)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if err := h.payments.RefundPayment(t.Context(), h.adminID, created.ID); !errors.Is(err, domain.ErrInvalidPaymentStatus) {
			t.Errorf("err = %v, want ErrInvalidPaymentStatus", err)
		}
	})

	t.Run("unknown payment", func(t *testing.T) {
		if err := h.payments.RefundPayment(t.Context(), h.adminID, uuid.New()); !errors.Is(err, ErrPaymentNotFound) {
			t.Errorf("err = %v, want ErrPaymentNotFound", err)
		}
	})

	if h.provider.refundCalls != 0 {
		t.Errorf("provider refund calls = %d, want none", h.provider.refundCalls)
	}
}

// TestRefundPayment_ProviderErrorRevertsReservation proves the compensation
// step (migrated from TestBilling_RefundPayment_ProviderCancelError): a
// provider failure rolls the reservation back to the previous status, so the
// payment stays refundable, and the error propagates to the admin.
func TestRefundPayment_ProviderErrorRevertsReservation(t *testing.T) {
	h := newRefundHarness(t)
	payment := h.succeededUpgradePayment(t)
	h.provider.refundErr = errors.New("provider is down")

	err := h.payments.RefundPayment(t.Context(), h.adminID, payment.ID)
	if err == nil {
		t.Fatal("RefundPayment() error = nil, want the provider failure")
	}

	stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.Status != domain.PaymentStatusSucceeded {
		t.Errorf("payment status = %q, want the succeeded status restored", stored.Status)
	}

	// The reservation was released, so a later refund can be retried.
	h.provider.refundErr = nil
	if err := h.payments.RefundPayment(t.Context(), h.adminID, payment.ID); err != nil {
		t.Fatalf("retry RefundPayment() error = %v", err)
	}
	stored, err = h.stores.payments.GetByID(t.Context(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID(retried) error = %v", err)
	}
	if stored.Status != domain.PaymentStatusRefunded {
		t.Errorf("payment status = %q, want refunded after the retry", stored.Status)
	}
}

// TestRefundPayment_NonRefundAnswerRevertsReservation proves a provider
// answer that is not a refund outcome rolls the reservation back and fails
// the request (migrated from TestBilling_RefundPayment_CancelFailureRevertsStatus).
func TestRefundPayment_NonRefundAnswerRevertsReservation(t *testing.T) {
	h := newRefundHarness(t)
	payment := h.succeededUpgradePayment(t)
	h.provider.refundRes = RefundResult{
		ProviderPaymentID: *payment.ProviderPaymentID,
		Status:            domain.PaymentStatusFailed,
	}

	if err := h.payments.RefundPayment(t.Context(), h.adminID, payment.ID); err == nil {
		t.Fatal("RefundPayment() error = nil, want the non-refund answer to fail")
	}
	stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.Status != domain.PaymentStatusSucceeded {
		t.Errorf("payment status = %q, want succeeded restored", stored.Status)
	}
}

// TestRefundPayment_InFlightRefundKeepsReservation proves the сторож
// (migrated from TestBilling_RefundPayment_CancelRefundingKeepsReservation):
// a provider that accepted the refund but has not settled it yet leaves the
// reservation in place — reverting would cancel a refund in flight — and the
// request answers success: the reconciliation worker resolves the outcome.
func TestRefundPayment_InFlightRefundKeepsReservation(t *testing.T) {
	h := newRefundHarness(t)
	payment := h.succeededUpgradePayment(t)
	h.provider.refundRes = RefundResult{
		ProviderPaymentID:     *payment.ProviderPaymentID,
		Status:                domain.PaymentStatusRefunding,
		RefundedAmountKopecks: 0,
	}

	if err := h.payments.RefundPayment(t.Context(), h.adminID, payment.ID); err != nil {
		t.Fatalf("RefundPayment() error = %v", err)
	}
	stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.Status != domain.PaymentStatusRefunding {
		t.Errorf("payment status = %q, want the refunding reservation kept", stored.Status)
	}
	// The subscription effects wait for the refund settlement.
	subStored, err := h.stores.subscriptions.GetByUserID(t.Context(), payment.UserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if subStored.TariffID != h.tariffID(t, domain.TariffBusiness) {
		t.Errorf("tariff = %v, want business until the refund settles", subStored.TariffID)
	}
}

// TestRefundPayment_PartialRefundAnomalyKeepsReservation proves the
// partial-refund anomaly handling (migrated from
// TestBilling_RefundPayment_CancelPartialRefundedStaysRefunding): the system
// always refunds the full amount, so a provider answering with a smaller
// refunded amount is an anomaly — the reservation is kept for manual review
// instead of recording a full refund that did not happen.
func TestRefundPayment_PartialRefundAnomalyKeepsReservation(t *testing.T) {
	h := newRefundHarness(t)
	payment := h.succeededUpgradePayment(t)
	h.provider.refundRes = RefundResult{
		ProviderPaymentID:     *payment.ProviderPaymentID,
		Status:                domain.PaymentStatusRefunded,
		RefundedAmountKopecks: payment.AmountKopecks - 1000,
	}

	if err := h.payments.RefundPayment(t.Context(), h.adminID, payment.ID); err != nil {
		t.Fatalf("RefundPayment() error = %v", err)
	}
	stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.Status != domain.PaymentStatusRefunding {
		t.Errorf("payment status = %q, want the refunding reservation kept for review", stored.Status)
	}
	if stored.RefundedAmountKopecks != nil {
		t.Errorf("refunded amount = %v, want none recorded", stored.RefundedAmountKopecks)
	}
}

// TestRefundPayment_DuplicateOperationResolvesFromProvider proves the
// duplicate-answer resolution: a network duplicate of an already performed
// refund is resolved from the provider's status — a refunded payment
// finalizes, a still-captured charge reverts — instead of failing blindly.
func TestRefundPayment_DuplicateOperationResolvesFromProvider(t *testing.T) {
	t.Run("provider confirms the refund", func(t *testing.T) {
		h := newRefundHarness(t)
		payment := h.succeededUpgradePayment(t)
		h.provider.refundErr = ErrProviderDuplicateOperation
		h.provider.statusRes = PaymentStatusResult{Status: domain.PaymentStatusRefunded}

		if err := h.payments.RefundPayment(t.Context(), h.adminID, payment.ID); err != nil {
			t.Fatalf("RefundPayment() error = %v", err)
		}
		stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if stored.Status != domain.PaymentStatusRefunded {
			t.Errorf("payment status = %q, want refunded", stored.Status)
		}
	})

	t.Run("provider still holds the charge", func(t *testing.T) {
		h := newRefundHarness(t)
		payment := h.succeededUpgradePayment(t)
		h.provider.refundErr = ErrProviderDuplicateOperation
		h.provider.statusRes = PaymentStatusResult{Status: domain.PaymentStatusSucceeded}

		if err := h.payments.RefundPayment(t.Context(), h.adminID, payment.ID); err != nil {
			t.Fatalf("RefundPayment() error = %v", err)
		}
		stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if stored.Status != domain.PaymentStatusSucceeded {
			t.Errorf("payment status = %q, want the succeeded status restored", stored.Status)
		}
	})
}

// TestSyncPayment applies the provider's status through the synchronous paths
// and audits the admin action (issue #254).
func TestSyncPayment(t *testing.T) {
	t.Run("provider refunded applies the refund effects", func(t *testing.T) {
		h := newRefundHarness(t)
		payment := h.succeededUpgradePayment(t)
		h.provider.statusRes = PaymentStatusResult{Status: domain.PaymentStatusRefunded}

		if err := h.payments.SyncPayment(t.Context(), h.adminID, payment.ID); err != nil {
			t.Fatalf("SyncPayment() error = %v", err)
		}
		stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if stored.Status != domain.PaymentStatusRefunded {
			t.Errorf("payment status = %q, want refunded", stored.Status)
		}
		subStored, err := h.stores.subscriptions.GetByUserID(t.Context(), payment.UserID)
		if err != nil {
			t.Fatalf("GetByUserID() error = %v", err)
		}
		if subStored.TariffID != h.tariffID(t, domain.TariffBasic) {
			t.Errorf("tariff = %v, want basic after the synced refund", subStored.TariffID)
		}

		var synced *auditdomain.Entry
		for _, entry := range h.audit.recorded() {
			if entry.Action == auditdomain.ActionSubscriptionPaymentSynced {
				synced = &entry
			}
		}
		if synced == nil {
			t.Fatalf("audit contains no synced entry: %+v", h.audit.recorded())
		}
		if synced.ActorRole != auditdomain.ActorRoleAdmin || synced.ActorID == nil || *synced.ActorID != h.adminID {
			t.Errorf("sync audit actor = %q/%v, want the acting admin", synced.ActorRole, synced.ActorID)
		}
	})

	t.Run("provider pending changes nothing", func(t *testing.T) {
		h := newRefundHarness(t)
		payment := h.succeededUpgradePayment(t)
		h.provider.statusRes = PaymentStatusResult{Status: domain.PaymentStatusPending}

		if err := h.payments.SyncPayment(t.Context(), h.adminID, payment.ID); err != nil {
			t.Fatalf("SyncPayment() error = %v", err)
		}
		stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if stored.Status != domain.PaymentStatusSucceeded {
			t.Errorf("payment status = %q, want succeeded untouched", stored.Status)
		}
	})

	t.Run("a stuck refund reservation resolves like the worker", func(t *testing.T) {
		h := newRefundHarness(t)
		payment := h.succeededUpgradePayment(t)
		// Reserve the payment the way the saga does, then have the provider
		// settle the refund.
		stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if err := stored.BeginRefund(h.now); err != nil {
			t.Fatalf("BeginRefund() error = %v", err)
		}
		if err := h.stores.payments.Update(t.Context(), stored); err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		h.provider.statusRes = PaymentStatusResult{Status: domain.PaymentStatusRefunded}

		if err := h.payments.SyncPayment(t.Context(), h.adminID, payment.ID); err != nil {
			t.Fatalf("SyncPayment() error = %v", err)
		}
		resolved, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil {
			t.Fatalf("GetByID(resolved) error = %v", err)
		}
		if resolved.Status != domain.PaymentStatusRefunded {
			t.Errorf("payment status = %q, want refunded", resolved.Status)
		}
		subStored, err := h.stores.subscriptions.GetByUserID(t.Context(), payment.UserID)
		if err != nil {
			t.Fatalf("GetByUserID() error = %v", err)
		}
		if subStored.TariffID != h.tariffID(t, domain.TariffBasic) {
			t.Errorf("tariff = %v, want basic after the synced refund", subStored.TariffID)
		}
	})

	t.Run("without a provider reference", func(t *testing.T) {
		h := newRefundHarness(t)
		sub := h.seedSubscription(t, nil)
		payment, err := domain.NewSubscriptionPayment(sub.UserID, sub.ID, h.tariffID(t, domain.TariffBusiness), domain.PeriodMonth, 99000, "fake", h.now)
		if err != nil {
			t.Fatalf("NewSubscriptionPayment() error = %v", err)
		}
		created, err := h.stores.payments.Create(t.Context(), payment)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if err := h.payments.SyncPayment(t.Context(), h.adminID, created.ID); !errors.Is(err, domain.ErrInvalidPaymentStatus) {
			t.Errorf("err = %v, want ErrInvalidPaymentStatus", err)
		}
	})

	t.Run("unknown payment", func(t *testing.T) {
		h := newRefundHarness(t)
		if err := h.payments.SyncPayment(t.Context(), h.adminID, uuid.New()); !errors.Is(err, ErrPaymentNotFound) {
			t.Errorf("err = %v, want ErrPaymentNotFound", err)
		}
	})
}

// TestReconcileStaleRefunds proves the сторож of the refund saga (migrated
// from TestBilling_ReconcileStaleRefunds): stuck refunding payments are
// resolved against the provider — refunded finalizes with the subscription
// effects, a still-captured charge reverts the reservation, and ambiguous or
// unsettled outcomes stay for the next tick or manual review.
func TestReconcileStaleRefunds(t *testing.T) {
	seedStuckRefund := func(h *workersHarness, status domain.PaymentStatus) domain.SubscriptionPayment {
		t.Helper()
		sub := h.seedSubscription(t, nil)
		payment, err := domain.NewSubscriptionPayment(sub.UserID, sub.ID, h.pro.ID, domain.PeriodMonth, 49000, "fake", h.now)
		if err != nil {
			t.Fatalf("NewSubscriptionPayment() error = %v", err)
		}
		providerPaymentID := "prov_" + payment.ID.String()
		payment.ProviderPaymentID = &providerPaymentID
		if status == domain.PaymentStatusSucceeded {
			if err := payment.MarkSucceeded(h.now); err != nil {
				t.Fatalf("MarkSucceeded() error = %v", err)
			}
		}
		if err := payment.BeginRefund(h.now); err != nil {
			t.Fatalf("BeginRefund() error = %v", err)
		}
		created, err := h.stores.payments.Create(t.Context(), payment)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		return created
	}

	t.Run("provider refunded finalizes", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		payment := seedStuckRefund(h, domain.PaymentStatusSucceeded)
		h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
			return PaymentStatusResult{Status: domain.PaymentStatusRefunded}, nil
		}

		processed, err := h.workers.ReconcileStaleRefunds(t.Context(), h.now.Add(2*h.cfg.PendingPaymentStaleness))
		if err != nil {
			t.Fatalf("ReconcileStaleRefunds() error = %v", err)
		}
		if processed != 1 {
			t.Errorf("processed = %d, want 1", processed)
		}
		stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if stored.Status != domain.PaymentStatusRefunded {
			t.Errorf("payment status = %q, want refunded", stored.Status)
		}
		subStored, err := h.stores.subscriptions.GetByUserID(t.Context(), payment.UserID)
		if err != nil {
			t.Fatalf("GetByUserID() error = %v", err)
		}
		if subStored.TariffID != h.basic.ID {
			t.Errorf("tariff = %v, want basic after the finalized refund", subStored.TariffID)
		}
	})

	t.Run("provider still captured reverts a succeeded origin", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		payment := seedStuckRefund(h, domain.PaymentStatusSucceeded)
		h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
			return PaymentStatusResult{Status: domain.PaymentStatusSucceeded}, nil
		}

		if _, err := h.workers.ReconcileStaleRefunds(t.Context(), h.now.Add(2*h.cfg.PendingPaymentStaleness)); err != nil {
			t.Fatalf("ReconcileStaleRefunds() error = %v", err)
		}
		stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if stored.Status != domain.PaymentStatusSucceeded {
			t.Errorf("payment status = %q, want the succeeded status restored", stored.Status)
		}
		subStored, err := h.stores.subscriptions.GetByUserID(t.Context(), payment.UserID)
		if err != nil {
			t.Fatalf("GetByUserID() error = %v", err)
		}
		if subStored.TariffID != h.pro.ID {
			t.Errorf("tariff = %v, want pro untouched by the reverted refund", subStored.TariffID)
		}
	})

	t.Run("provider still captured reverts a pending origin to pending", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		payment := seedStuckRefund(h, domain.PaymentStatusPending)
		h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
			return PaymentStatusResult{Status: domain.PaymentStatusSucceeded}, nil
		}

		if _, err := h.workers.ReconcileStaleRefunds(t.Context(), h.now.Add(2*h.cfg.PendingPaymentStaleness)); err != nil {
			t.Fatalf("ReconcileStaleRefunds() error = %v", err)
		}
		stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if stored.Status != domain.PaymentStatusPending {
			t.Errorf("payment status = %q, want pending restored for the pending reconciliation", stored.Status)
		}
	})

	t.Run("failed provider status stays for manual review", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		payment := seedStuckRefund(h, domain.PaymentStatusSucceeded)
		h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
			return PaymentStatusResult{Status: domain.PaymentStatusFailed}, nil
		}

		processed, err := h.workers.ReconcileStaleRefunds(t.Context(), h.now.Add(2*h.cfg.PendingPaymentStaleness))
		if err != nil {
			t.Fatalf("ReconcileStaleRefunds() error = %v", err)
		}
		if processed != 1 {
			t.Errorf("processed = %d, want the payment checked", processed)
		}
		stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if stored.Status != domain.PaymentStatusRefunding {
			t.Errorf("payment status = %q, want the reservation kept", stored.Status)
		}
	})

	t.Run("unsettled refund waits for the next tick", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		payment := seedStuckRefund(h, domain.PaymentStatusSucceeded)
		h.provider.statusFn = func(uuid.UUID, string) (PaymentStatusResult, error) {
			return PaymentStatusResult{Status: domain.PaymentStatusPending}, nil
		}

		if _, err := h.workers.ReconcileStaleRefunds(t.Context(), h.now.Add(2*h.cfg.PendingPaymentStaleness)); err != nil {
			t.Fatalf("ReconcileStaleRefunds() error = %v", err)
		}
		stored, err := h.stores.payments.GetByID(t.Context(), payment.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if stored.Status != domain.PaymentStatusRefunding {
			t.Errorf("payment status = %q, want the reservation kept", stored.Status)
		}
	})

	t.Run("fresh reservations are not listed", func(t *testing.T) {
		h := newWorkersHarness(t, Config{})
		seedStuckRefund(h, domain.PaymentStatusSucceeded)

		processed, err := h.workers.ReconcileStaleRefunds(t.Context(), h.now)
		if err != nil {
			t.Fatalf("ReconcileStaleRefunds() error = %v", err)
		}
		if processed != 0 {
			t.Errorf("processed = %d, want 0 before the staleness threshold", processed)
		}
	})
}
