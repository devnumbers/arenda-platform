package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

var paymentNow = time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)

func newTestPayment(t *testing.T) SubscriptionPayment {
	t.Helper()
	payment, err := NewSubscriptionPayment(
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		PeriodMonth, 49000, "fake", paymentNow,
	)
	if err != nil {
		t.Fatalf("NewSubscriptionPayment() error = %v", err)
	}
	return payment
}

func TestNewSubscriptionPayment(t *testing.T) {
	t.Parallel()
	payment := newTestPayment(t)
	if payment.Status != PaymentStatusPending {
		t.Errorf("Status = %q, want pending", payment.Status)
	}
	if payment.Provider != "fake" {
		t.Errorf("Provider = %q, want fake", payment.Provider)
	}
	if payment.HasProviderReference() || payment.HasPaymentURL() {
		t.Error("a fresh payment must not carry provider references")
	}
	if !payment.CreatedAt.Equal(paymentNow) || !payment.UpdatedAt.Equal(paymentNow) {
		t.Errorf("timestamps = %v/%v, want %v", payment.CreatedAt, payment.UpdatedAt, paymentNow)
	}
}

func TestNewSubscriptionPayment_RejectsInvalidInput(t *testing.T) {
	t.Parallel()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	subID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tariffID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	cases := []struct {
		name    string
		period  SubscriptionPeriod
		amount  int64
		user    uuid.UUID
		wantErr error
	}{
		{name: "zero amount", period: PeriodMonth, amount: 0, user: userID, wantErr: ErrInvalidAmount},
		{name: "negative amount", period: PeriodMonth, amount: -1, user: userID, wantErr: ErrInvalidAmount},
		{name: "bad period", period: SubscriptionPeriod("week"), amount: 49000, user: userID, wantErr: ErrInvalidPeriod},
		{name: "missing user", period: PeriodMonth, amount: 49000, user: uuid.Nil, wantErr: ErrInvalidPayment},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewSubscriptionPayment(tc.user, subID, tariffID, tc.period, tc.amount, "fake", paymentNow)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestSubscriptionPayment_FinalizationTransitions(t *testing.T) {
	t.Parallel()
	t.Run("mark succeeded", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.MarkSucceeded(paymentNow); err != nil {
			t.Fatalf("MarkSucceeded() error = %v", err)
		}
		if payment.Status != PaymentStatusSucceeded || payment.SucceededAt == nil {
			t.Errorf("payment = %q/%v, want succeeded with a timestamp", payment.Status, payment.SucceededAt)
		}
		if err := payment.MarkFailed(nil, paymentNow); err == nil {
			t.Error("MarkFailed on succeeded = nil error, want ErrInvalidPaymentStatus")
		}
	})

	t.Run("mark failed records the error code", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		code := "card_declined"
		if err := payment.MarkFailed(&code, paymentNow); err != nil {
			t.Fatalf("MarkFailed() error = %v", err)
		}
		if payment.Status != PaymentStatusFailed || payment.ErrorCode == nil || *payment.ErrorCode != code {
			t.Errorf("payment = %q/%v, want failed with the error code", payment.Status, payment.ErrorCode)
		}
	})

	t.Run("reconcile to succeeded clears the failure", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		code := "timeout"
		if err := payment.MarkFailed(&code, paymentNow); err != nil {
			t.Fatalf("MarkFailed() error = %v", err)
		}
		if err := payment.ReconcileToSucceeded(paymentNow); err != nil {
			t.Fatalf("ReconcileToSucceeded() error = %v", err)
		}
		if payment.Status != PaymentStatusSucceeded || payment.ErrorCode != nil || payment.SucceededAt == nil {
			t.Errorf("payment = %q/%v/%v, want succeeded, code cleared, timestamp set", payment.Status, payment.ErrorCode, payment.SucceededAt)
		}
	})

	t.Run("reconcile requires the failed status", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.ReconcileToSucceeded(paymentNow); !errors.Is(err, ErrInvalidPaymentStatus) {
			t.Errorf("err = %v, want ErrInvalidPaymentStatus", err)
		}
	})
}

// refundMarkedRecordsFullAmount proves a completed refund stores the refunded
// status with the full original amount.
func refundMarkedRecordsFullAmount(t *testing.T) {
	t.Parallel()
	payment := newTestPayment(t)
	if err := payment.MarkSucceeded(paymentNow); err != nil {
		t.Fatalf("MarkSucceeded() error = %v", err)
	}
	if err := payment.MarkRefunded(paymentNow); err != nil {
		t.Fatalf("MarkRefunded() error = %v", err)
	}
	if payment.Status != PaymentStatusRefunded || payment.RefundedAmountKopecks == nil || *payment.RefundedAmountKopecks != 49000 {
		t.Errorf("payment = %q/%v, want refunded with the full amount", payment.Status, payment.RefundedAmountKopecks)
	}
}

// beginRefundReservesSucceededAndPendingOnly proves both refundable source
// statuses move into the refunding reservation.
func beginRefundReservesSucceededAndPendingOnly(t *testing.T) {
	t.Parallel()
	succeeded := newTestPayment(t)
	if err := succeeded.MarkSucceeded(paymentNow); err != nil {
		t.Fatalf("MarkSucceeded() error = %v", err)
	}
	if err := succeeded.BeginRefund(paymentNow); err != nil {
		t.Fatalf("BeginRefund(succeeded) error = %v", err)
	}
	if succeeded.Status != PaymentStatusRefunding || succeeded.RefundedAmountKopecks != nil {
		t.Errorf("payment = %q/%v, want the refunding reservation without a refunded amount", succeeded.Status, succeeded.RefundedAmountKopecks)
	}

	pending := newTestPayment(t)
	if err := pending.BeginRefund(paymentNow); err != nil {
		t.Fatalf("BeginRefund(pending) error = %v", err)
	}
	if pending.Status != PaymentStatusRefunding {
		t.Errorf("pending refund status = %q, want refunding", pending.Status)
	}
}

// beginRefundRejectsTerminalAndReservedPayments proves every non-refundable
// status rejects the reservation.
func beginRefundRejectsTerminalAndReservedPayments(t *testing.T) {
	t.Parallel()
	for status, prepare := range map[PaymentStatus]func(*SubscriptionPayment){
		PaymentStatusFailed: func(p *SubscriptionPayment) {
			if err := p.MarkFailed(nil, paymentNow); err != nil {
				t.Fatalf("MarkFailed() error = %v", err)
			}
		},
		PaymentStatusRefunded: func(p *SubscriptionPayment) {
			if err := p.MarkSucceeded(paymentNow); err != nil {
				t.Fatalf("MarkSucceeded() error = %v", err)
			}
			if err := p.MarkRefunded(paymentNow); err != nil {
				t.Fatalf("MarkRefunded() error = %v", err)
			}
		},
		PaymentStatusRefunding: func(p *SubscriptionPayment) {
			if err := p.MarkSucceeded(paymentNow); err != nil {
				t.Fatalf("MarkSucceeded() error = %v", err)
			}
			if err := p.BeginRefund(paymentNow); err != nil {
				t.Fatalf("BeginRefund() error = %v", err)
			}
		},
	} {
		payment := newTestPayment(t)
		prepare(&payment)
		if err := payment.BeginRefund(paymentNow); !errors.Is(err, ErrInvalidPaymentStatus) {
			t.Errorf("BeginRefund(%s) err = %v, want ErrInvalidPaymentStatus", status, err)
		}
	}
}

// revertRefundReservationRestoresPreviousStatus proves reverting a reservation
// returns the payment to the status it was reserved from.
func revertRefundReservationRestoresPreviousStatus(t *testing.T) {
	t.Parallel()
	succeeded := newTestPayment(t)
	if err := succeeded.MarkSucceeded(paymentNow); err != nil {
		t.Fatalf("MarkSucceeded() error = %v", err)
	}
	if err := succeeded.BeginRefund(paymentNow); err != nil {
		t.Fatalf("BeginRefund() error = %v", err)
	}
	if err := succeeded.RevertRefundReservation(PaymentStatusSucceeded, paymentNow); err != nil {
		t.Fatalf("RevertRefundReservation() error = %v", err)
	}
	if succeeded.Status != PaymentStatusSucceeded {
		t.Errorf("status = %q, want succeeded restored", succeeded.Status)
	}

	pending := newTestPayment(t)
	if err := pending.BeginRefund(paymentNow); err != nil {
		t.Fatalf("BeginRefund() error = %v", err)
	}
	if err := pending.RevertRefundReservation(PaymentStatusPending, paymentNow); err != nil {
		t.Fatalf("RevertRefundReservation(pending) error = %v", err)
	}
	if pending.Status != PaymentStatusPending {
		t.Errorf("status = %q, want pending restored", pending.Status)
	}
}

// revertRequiresReservationAndRefundableStatus proves the revert guards: no
// reservation to revert, and no reverting into a terminal status.
func revertRequiresReservationAndRefundableStatus(t *testing.T) {
	t.Parallel()
	payment := newTestPayment(t)
	if err := payment.RevertRefundReservation(PaymentStatusSucceeded, paymentNow); !errors.Is(err, ErrInvalidPaymentStatus) {
		t.Errorf("revert without reservation err = %v, want ErrInvalidPaymentStatus", err)
	}
	reserved := newTestPayment(t)
	if err := reserved.MarkSucceeded(paymentNow); err != nil {
		t.Fatalf("MarkSucceeded() error = %v", err)
	}
	if err := reserved.BeginRefund(paymentNow); err != nil {
		t.Fatalf("BeginRefund() error = %v", err)
	}
	if err := reserved.RevertRefundReservation(PaymentStatusRefunded, paymentNow); !errors.Is(err, ErrInvalidPaymentStatus) {
		t.Errorf("revert to refunded err = %v, want ErrInvalidPaymentStatus", err)
	}
}

func TestSubscriptionPayment_RefundTransitions(t *testing.T) {
	t.Parallel()
	t.Run("mark refunded records the full amount", refundMarkedRecordsFullAmount)
	t.Run("begin refund reserves succeeded and pending only", beginRefundReservesSucceededAndPendingOnly)
	t.Run("begin refund rejects terminal and reserved payments", beginRefundRejectsTerminalAndReservedPayments)
	t.Run("revert refund reservation restores the previous status", revertRefundReservationRestoresPreviousStatus)
	t.Run("revert requires the reservation and a refundable previous status", revertRequiresReservationAndRefundableStatus)
}

func TestSubscriptionPayment_SaveProviderReference(t *testing.T) {
	t.Parallel()
	t.Run("persists reference and url", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.SaveProviderReference("prov_1", "https://pay/1", paymentNow); err != nil {
			t.Fatalf("SaveProviderReference() error = %v", err)
		}
		if !payment.HasProviderReference() || *payment.ProviderPaymentID != "prov_1" {
			t.Errorf("provider payment id = %v, want prov_1", payment.ProviderPaymentID)
		}
		if !payment.HasPaymentURL() || *payment.PaymentURL != "https://pay/1" {
			t.Errorf("payment url = %v, want the initiation url", payment.PaymentURL)
		}
	})

	t.Run("second save is rejected (the reference never changes)", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.SaveProviderReference("prov_1", "https://pay/1", paymentNow); err != nil {
			t.Fatalf("first SaveProviderReference() error = %v", err)
		}
		if err := payment.SaveProviderReference("prov_2", "https://pay/2", paymentNow); err == nil {
			t.Error("second SaveProviderReference() = nil error, want ErrInvalidPaymentStatus")
		}
	})

	t.Run("finalized payment rejects the reference", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.MarkFailed(nil, paymentNow); err != nil {
			t.Fatalf("MarkFailed() error = %v", err)
		}
		if err := payment.SaveProviderReference("prov_1", "", paymentNow); err == nil {
			t.Error("SaveProviderReference on failed = nil error, want ErrInvalidPaymentStatus")
		}
	})

	t.Run("empty reference is rejected", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.SaveProviderReference("", "", paymentNow); !errors.Is(err, ErrInvalidPayment) {
			t.Errorf("err = %v, want ErrInvalidPayment", err)
		}
	})
}

func TestReconstituteSubscriptionPayment(t *testing.T) {
	t.Parallel()
	payment := newTestPayment(t)

	t.Run("valid payment passes", func(t *testing.T) {
		t.Parallel()
		if _, err := ReconstituteSubscriptionPayment(payment); err != nil {
			t.Fatalf("ReconstituteSubscriptionPayment() error = %v", err)
		}
	})

	t.Run("unknown status is rejected", func(t *testing.T) {
		t.Parallel()
		bad := payment
		bad.Status = PaymentStatus("maybe")
		if _, err := ReconstituteSubscriptionPayment(bad); err == nil {
			t.Fatal("expected an error for an unknown status")
		}
	})

	t.Run("missing identity is rejected", func(t *testing.T) {
		t.Parallel()
		bad := payment
		bad.UserID = uuid.Nil
		if _, err := ReconstituteSubscriptionPayment(bad); err == nil {
			t.Fatal("expected an error for a missing identity field")
		}
	})
}

func TestNewAppliedPaymentTransition(t *testing.T) {
	t.Parallel()
	sub, err := NewBasicSubscription(
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		uuid.MustParse("22222222-2222-2222-2222-222222222222"),
	)
	if err != nil {
		t.Fatalf("NewBasicSubscription() error = %v", err)
	}
	fromStatus := SubscriptionStatusGrace
	fromTariff := sub.TariffID
	paymentID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	transition, err := NewAppliedPaymentTransition(sub, &fromStatus, &fromTariff, paymentID)
	if err != nil {
		t.Fatalf("NewAppliedPaymentTransition() error = %v", err)
	}
	if transition.Reason != TransitionReasonPaymentApplied {
		t.Errorf("Reason = %q, want payment_applied", transition.Reason)
	}
	if transition.PaymentID == nil || *transition.PaymentID != paymentID {
		t.Errorf("PaymentID = %v, want %v", transition.PaymentID, paymentID)
	}
	if transition.Initiator != InitiatorSystem || transition.InitiatorID != nil {
		t.Errorf("initiator = %q/%v, want system without an actor", transition.Initiator, transition.InitiatorID)
	}
	if transition.FromStatus == nil || *transition.FromStatus != SubscriptionStatusGrace {
		t.Errorf("FromStatus = %v, want grace", transition.FromStatus)
	}

	if _, err := NewAppliedPaymentTransition(sub, &fromStatus, &fromTariff, uuid.Nil); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("nil payment err = %v, want ErrInvalidTransition", err)
	}
}

// snapshotMethodAttachesIDAndMask proves the charged-method snapshot of a
// merchant-initiated payment records the method and the card mask together
// (issue #619).
func snapshotMethodAttachesIDAndMask(t *testing.T) {
	t.Parallel()
	payment := newTestPayment(t)
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	if err := payment.AttachMethodSnapshot(methodID, testMaskVisa, paymentNow); err != nil {
		t.Fatalf("AttachMethodSnapshot() error = %v", err)
	}
	if payment.PaymentMethodID == nil || *payment.PaymentMethodID != methodID {
		t.Errorf("PaymentMethodID = %v, want %v", payment.PaymentMethodID, methodID)
	}
	if payment.CardMask == nil || *payment.CardMask != testMaskVisa {
		t.Errorf("CardMask = %v, want the charged card mask", payment.CardMask)
	}
}

// rePointedPaymentFollowsTheChargedCard proves a recovered pending payment
// pointed at a switched card carries the new card's mask: the snapshot must
// describe the card actually charged, not the one planned first.
func rePointedPaymentFollowsTheChargedCard(t *testing.T) {
	t.Parallel()
	payment := newTestPayment(t)
	first := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	if err := payment.AttachMethodSnapshot(first, testMaskVisa, paymentNow); err != nil {
		t.Fatalf("first AttachMethodSnapshot() error = %v", err)
	}
	second := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	if err := payment.AttachMethodSnapshot(second, "2202********9876", paymentNow); err != nil {
		t.Fatalf("re-point AttachMethodSnapshot() error = %v", err)
	}
	if payment.PaymentMethodID == nil || *payment.PaymentMethodID != second {
		t.Errorf("PaymentMethodID = %v, want the switched method", payment.PaymentMethodID)
	}
	if payment.CardMask == nil || *payment.CardMask != "2202********9876" {
		t.Errorf("CardMask = %v, want the switched card mask", payment.CardMask)
	}
}

// finalizedPaymentRejectsMethodSnapshot proves the snapshot never rewrites
// history: only a pending payment accepts it.
func finalizedPaymentRejectsMethodSnapshot(t *testing.T) {
	t.Parallel()
	payment := newTestPayment(t)
	if err := payment.MarkSucceeded(paymentNow); err != nil {
		t.Fatalf("MarkSucceeded() error = %v", err)
	}
	methodID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	if err := payment.AttachMethodSnapshot(methodID, testMaskVisa, paymentNow); !errors.Is(err, ErrInvalidPaymentStatus) {
		t.Errorf("err = %v, want ErrInvalidPaymentStatus", err)
	}
}

// cardMaskRecordsProviderReportedCard proves a customer-initiated payment
// snapshot lands from the provider notification (issue #619).
func cardMaskRecordsProviderReportedCard(t *testing.T) {
	t.Parallel()
	payment := newTestPayment(t)
	payment.AttachCardMask(testMaskVisa, paymentNow)
	if payment.CardMask == nil || *payment.CardMask != testMaskVisa {
		t.Errorf("CardMask = %v, want the provider-reported mask", payment.CardMask)
	}
}

// cardMaskNeverOverwrites proves the snapshot is fill-once: a repeated or
// later delivery cannot replace the recorded card.
func cardMaskNeverOverwrites(t *testing.T) {
	t.Parallel()
	payment := newTestPayment(t)
	payment.AttachCardMask(testMaskVisa, paymentNow)
	payment.AttachCardMask("2202********9876", paymentNow)
	if payment.CardMask == nil || *payment.CardMask != testMaskVisa {
		t.Errorf("CardMask = %v, want the first recorded mask", payment.CardMask)
	}
}

// emptyCardMaskIsNoOp proves a notification without card data leaves an
// existing snapshot (and the empty one) untouched.
func emptyCardMaskIsNoOp(t *testing.T) {
	t.Parallel()
	payment := newTestPayment(t)
	payment.AttachCardMask("", paymentNow)
	if payment.CardMask != nil {
		t.Errorf("CardMask = %v, want nil", payment.CardMask)
	}
}

func TestSubscriptionPayment_CardSnapshot(t *testing.T) {
	t.Parallel()
	t.Run("method snapshot attaches id and mask", snapshotMethodAttachesIDAndMask)
	t.Run("re-pointed payment follows the charged card", rePointedPaymentFollowsTheChargedCard)
	t.Run("finalized payment rejects the method snapshot", finalizedPaymentRejectsMethodSnapshot)
	t.Run("card mask records the provider-reported card", cardMaskRecordsProviderReportedCard)
	t.Run("card mask never overwrites", cardMaskNeverOverwrites)
	t.Run("empty card mask is a no-op", emptyCardMaskIsNoOp)
}

func TestSubscriptionPayment_AttachFormDeadline(t *testing.T) {
	t.Parallel()
	t.Run("persists the deadline on a fresh pending payment", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		deadline := paymentNow.Add(15 * time.Minute)
		if err := payment.AttachFormDeadline(deadline, paymentNow); err != nil {
			t.Fatalf("AttachFormDeadline() error = %v", err)
		}
		if payment.ExpiresAt == nil || !payment.ExpiresAt.Equal(deadline) {
			t.Errorf("ExpiresAt = %v, want %v", payment.ExpiresAt, deadline)
		}
	})

	t.Run("second attach is rejected (the deadline never changes)", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.AttachFormDeadline(paymentNow.Add(15*time.Minute), paymentNow); err != nil {
			t.Fatalf("first AttachFormDeadline() error = %v", err)
		}
		if err := payment.AttachFormDeadline(paymentNow.Add(30*time.Minute), paymentNow); err == nil {
			t.Error("second AttachFormDeadline() = nil error, want an error")
		}
	})

	t.Run("past deadline is rejected", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.AttachFormDeadline(paymentNow.Add(-time.Minute), paymentNow); !errors.Is(err, ErrInvalidPayment) {
			t.Errorf("err = %v, want ErrInvalidPayment", err)
		}
	})

	t.Run("finalized payment rejects the deadline", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.MarkSucceeded(paymentNow); err != nil {
			t.Fatalf("MarkSucceeded() error = %v", err)
		}
		if err := payment.AttachFormDeadline(paymentNow.Add(15*time.Minute), paymentNow); !errors.Is(err, ErrInvalidPaymentStatus) {
			t.Errorf("err = %v, want ErrInvalidPaymentStatus", err)
		}
	})
}

func TestSubscriptionPayment_MarkExpired(t *testing.T) {
	t.Parallel()
	deadline := paymentNow.Add(15 * time.Minute)

	t.Run("past the deadline marks failed with the form-expired code", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.AttachFormDeadline(deadline, paymentNow); err != nil {
			t.Fatalf("AttachFormDeadline() error = %v", err)
		}
		if err := payment.MarkExpired(deadline.Add(time.Minute)); err != nil {
			t.Fatalf("MarkExpired() error = %v", err)
		}
		if payment.Status != PaymentStatusFailed {
			t.Errorf("Status = %q, want failed", payment.Status)
		}
		if payment.ErrorCode == nil || *payment.ErrorCode != PaymentErrorCodeFormExpired {
			t.Errorf("ErrorCode = %v, want %q", payment.ErrorCode, PaymentErrorCodeFormExpired)
		}
	})

	t.Run("before the deadline is rejected", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.AttachFormDeadline(deadline, paymentNow); err != nil {
			t.Fatalf("AttachFormDeadline() error = %v", err)
		}
		if err := payment.MarkExpired(deadline.Add(-time.Second)); !errors.Is(err, ErrInvalidPaymentStatus) {
			t.Errorf("err = %v, want ErrInvalidPaymentStatus", err)
		}
	})

	t.Run("payment without a deadline is rejected", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.MarkExpired(deadline.Add(time.Minute)); !errors.Is(err, ErrInvalidPaymentStatus) {
			t.Errorf("err = %v, want ErrInvalidPaymentStatus", err)
		}
	})

	t.Run("finalized payment is rejected", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.AttachFormDeadline(deadline, paymentNow); err != nil {
			t.Fatalf("AttachFormDeadline() error = %v", err)
		}
		if err := payment.MarkSucceeded(paymentNow); err != nil {
			t.Fatalf("MarkSucceeded() error = %v", err)
		}
		if err := payment.MarkExpired(deadline.Add(time.Minute)); !errors.Is(err, ErrInvalidPaymentStatus) {
			t.Errorf("err = %v, want ErrInvalidPaymentStatus", err)
		}
	})
}

func TestSubscriptionPayment_IsLivePending(t *testing.T) {
	t.Parallel()
	deadline := paymentNow.Add(15 * time.Minute)

	t.Run("pending without a deadline is live (merchant-initiated charges)", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if !payment.IsLivePending(paymentNow.Add(time.Hour)) {
			t.Error("a pending payment without a deadline must stay live")
		}
	})

	t.Run("pending within the deadline is live", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.AttachFormDeadline(deadline, paymentNow); err != nil {
			t.Fatalf("AttachFormDeadline() error = %v", err)
		}
		if !payment.IsLivePending(deadline.Add(-time.Second)) {
			t.Error("a pending payment before its deadline must be live")
		}
	})

	t.Run("pending past the deadline is dead", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.AttachFormDeadline(deadline, paymentNow); err != nil {
			t.Fatalf("AttachFormDeadline() error = %v", err)
		}
		if payment.IsLivePending(deadline) || payment.IsLivePending(deadline.Add(time.Minute)) {
			t.Error("a pending payment past its deadline must not be live")
		}
	})

	t.Run("non-pending payment is never live", func(t *testing.T) {
		t.Parallel()
		payment := newTestPayment(t)
		if err := payment.AttachFormDeadline(deadline, paymentNow); err != nil {
			t.Fatalf("AttachFormDeadline() error = %v", err)
		}
		if err := payment.MarkSucceeded(paymentNow); err != nil {
			t.Fatalf("MarkSucceeded() error = %v", err)
		}
		if payment.IsLivePending(paymentNow) {
			t.Error("a finalized payment must not be live-pending")
		}
	})
}
