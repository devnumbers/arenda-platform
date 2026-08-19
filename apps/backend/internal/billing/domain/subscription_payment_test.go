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
			_, err := NewSubscriptionPayment(tc.user, subID, tariffID, tc.period, tc.amount, "fake", paymentNow)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestSubscriptionPayment_FinalizationTransitions(t *testing.T) {
	t.Run("mark succeeded", func(t *testing.T) {
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
		payment := newTestPayment(t)
		if err := payment.ReconcileToSucceeded(paymentNow); !errors.Is(err, ErrInvalidPaymentStatus) {
			t.Errorf("err = %v, want ErrInvalidPaymentStatus", err)
		}
	})
}

func TestSubscriptionPayment_RefundTransitions(t *testing.T) {
	t.Run("mark refunded records the full amount", func(t *testing.T) {
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
	})

	t.Run("begin refund reserves succeeded and pending only", func(t *testing.T) {
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
	})

	t.Run("begin refund rejects terminal and reserved payments", func(t *testing.T) {
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
	})

	t.Run("revert refund reservation restores the previous status", func(t *testing.T) {
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
	})

	t.Run("revert requires the reservation and a refundable previous status", func(t *testing.T) {
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
	})
}

func TestSubscriptionPayment_SaveProviderReference(t *testing.T) {
	t.Run("persists reference and url", func(t *testing.T) {
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
		payment := newTestPayment(t)
		if err := payment.SaveProviderReference("prov_1", "https://pay/1", paymentNow); err != nil {
			t.Fatalf("first SaveProviderReference() error = %v", err)
		}
		if err := payment.SaveProviderReference("prov_2", "https://pay/2", paymentNow); err == nil {
			t.Error("second SaveProviderReference() = nil error, want ErrInvalidPaymentStatus")
		}
	})

	t.Run("finalized payment rejects the reference", func(t *testing.T) {
		payment := newTestPayment(t)
		if err := payment.MarkFailed(nil, paymentNow); err != nil {
			t.Fatalf("MarkFailed() error = %v", err)
		}
		if err := payment.SaveProviderReference("prov_1", "", paymentNow); err == nil {
			t.Error("SaveProviderReference on failed = nil error, want ErrInvalidPaymentStatus")
		}
	})

	t.Run("empty reference is rejected", func(t *testing.T) {
		payment := newTestPayment(t)
		if err := payment.SaveProviderReference("", "", paymentNow); !errors.Is(err, ErrInvalidPayment) {
			t.Errorf("err = %v, want ErrInvalidPayment", err)
		}
	})
}

func TestReconstituteSubscriptionPayment(t *testing.T) {
	payment := newTestPayment(t)

	t.Run("valid payment passes", func(t *testing.T) {
		if _, err := ReconstituteSubscriptionPayment(payment); err != nil {
			t.Fatalf("ReconstituteSubscriptionPayment() error = %v", err)
		}
	})

	t.Run("unknown status is rejected", func(t *testing.T) {
		bad := payment
		bad.Status = PaymentStatus("maybe")
		if _, err := ReconstituteSubscriptionPayment(bad); err == nil {
			t.Fatal("expected an error for an unknown status")
		}
	})

	t.Run("missing identity is rejected", func(t *testing.T) {
		bad := payment
		bad.UserID = uuid.Nil
		if _, err := ReconstituteSubscriptionPayment(bad); err == nil {
			t.Fatal("expected an error for a missing identity field")
		}
	})
}

func TestNewAppliedPaymentTransition(t *testing.T) {
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
