package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewSubscriptionPayment(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	subscriptionID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	paymentMethodID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	payment, err := NewSubscriptionPayment(userID, subscriptionID, tariffID, &paymentMethodID, PeriodMonth, 49000, ProviderFake, now)
	if err != nil {
		t.Fatalf("NewSubscriptionPayment() error = %v", err)
	}

	if payment.UserID != userID {
		t.Errorf("UserID = %v, want %v", payment.UserID, userID)
	}
	if payment.SubscriptionID != subscriptionID {
		t.Errorf("SubscriptionID = %v, want %v", payment.SubscriptionID, subscriptionID)
	}
	if payment.TariffID != tariffID {
		t.Errorf("TariffID = %v, want %v", payment.TariffID, tariffID)
	}
	if *payment.PaymentMethodID != paymentMethodID {
		t.Errorf("PaymentMethodID = %v, want %v", payment.PaymentMethodID, paymentMethodID)
	}
	if payment.Period != PeriodMonth {
		t.Errorf("Period = %v, want %v", payment.Period, PeriodMonth)
	}
	if payment.AmountKopecks != 49000 {
		t.Errorf("AmountKopecks = %v, want 49000", payment.AmountKopecks)
	}
	if payment.Provider != ProviderFake {
		t.Errorf("Provider = %v, want %v", payment.Provider, ProviderFake)
	}
	if payment.Status != PaymentStatusPending {
		t.Errorf("Status = %v, want %v", payment.Status, PaymentStatusPending)
	}
	if payment.ID == uuid.Nil {
		t.Error("ID must be set")
	}
}

func TestSubscriptionPaymentUpdateStatus(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	subscriptionID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	payment, _ := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodYear, 440000, ProviderFake, now)
	if payment.Status != PaymentStatusPending {
		t.Fatalf("initial status = %v, want pending", payment.Status)
	}

	if err := payment.MarkSucceeded(now); err != nil {
		t.Fatalf("MarkSucceeded() error = %v", err)
	}
	if payment.Status != PaymentStatusSucceeded {
		t.Errorf("after MarkSucceeded() status = %v, want %v", payment.Status, PaymentStatusSucceeded)
	}
}

func TestNewSubscriptionPaymentInvalidAmount(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	subscriptionID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name          string
		amountKopecks int64
		wantErr       error
	}{
		{"negative amount", -1, ErrInvalidAmount},
		{"zero amount", 0, ErrInvalidAmount},
		{"positive amount", 1, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodMonth, tt.amountKopecks, ProviderFake, now)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewSubscriptionPayment() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewSubscriptionPaymentInvalidPeriod(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	subscriptionID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	_, err := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, SubscriptionPeriod("weekly"), 1000, ProviderFake, now)
	if !errors.Is(err, ErrInvalidPeriod) {
		t.Errorf("NewSubscriptionPayment() error = %v, want ErrInvalidPeriod", err)
	}
}

func TestSubscriptionPaymentRefund(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	subscriptionID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	t.Run("full refund", func(t *testing.T) {
		payment, _ := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodYear, 440000, ProviderFake, now)
		if err := payment.MarkSucceeded(now); err != nil {
			t.Fatalf("MarkSucceeded() error = %v", err)
		}
		if err := payment.MarkRefunded(now); err != nil {
			t.Fatalf("MarkRefunded() error = %v", err)
		}
		if payment.Status != PaymentStatusRefunded {
			t.Errorf("status = %v, want %v", payment.Status, PaymentStatusRefunded)
		}
		if payment.RefundedAmountKopecks == nil || *payment.RefundedAmountKopecks != 440000 {
			t.Errorf("refunded amount = %v, want 440000", payment.RefundedAmountKopecks)
		}
		if !payment.IsFinalized() {
			t.Error("expected payment to be finalized")
		}
	})

	t.Run("refund from pending succeeds", func(t *testing.T) {
		payment, _ := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodYear, 440000, ProviderFake, now)
		if err := payment.MarkRefunded(now); err != nil {
			t.Fatalf("MarkRefunded() error = %v", err)
		}
		if payment.Status != PaymentStatusRefunded {
			t.Errorf("status = %v, want %v", payment.Status, PaymentStatusRefunded)
		}
		if payment.RefundedAmountKopecks == nil || *payment.RefundedAmountKopecks != 440000 {
			t.Errorf("refunded amount = %v, want 440000", payment.RefundedAmountKopecks)
		}
	})

	t.Run("refund from failed fails", func(t *testing.T) {
		payment, _ := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodYear, 440000, ProviderFake, now)
		code := "error"
		_ = payment.MarkFailed(&code, now)
		if err := payment.MarkRefunded(now); !errors.Is(err, ErrInvalidPaymentStatus) {
			t.Errorf("MarkRefunded() error = %v, want ErrInvalidPaymentStatus", err)
		}
	})
}

func TestSubscriptionPaymentInvalidStatusTransitions(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	subscriptionID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		initialStatus PaymentStatus
	}{
		{"succeeded->succeeded", PaymentStatusSucceeded},
		{"failed->failed", PaymentStatusFailed},
		{"succeeded->failed", PaymentStatusSucceeded},
		{"failed->succeeded", PaymentStatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payment, err := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodMonth, 1000, ProviderFake, now)
			if err != nil {
				t.Fatalf("NewSubscriptionPayment() error = %v", err)
			}
			payment.Status = tt.initialStatus

			if err := payment.MarkSucceeded(now); !errors.Is(err, ErrInvalidPaymentStatus) {
				t.Errorf("MarkSucceeded() error = %v, want ErrInvalidPaymentStatus", err)
			}
			if err := payment.MarkFailed(nil, now); !errors.Is(err, ErrInvalidPaymentStatus) {
				t.Errorf("MarkFailed() error = %v, want ErrInvalidPaymentStatus", err)
			}
		})
	}
}

func TestSubscriptionPaymentBeginRefund(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	subscriptionID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	t.Run("from succeeded", func(t *testing.T) {
		payment, _ := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodMonth, 1000, ProviderFake, now)
		if err := payment.MarkSucceeded(now); err != nil {
			t.Fatalf("MarkSucceeded() error = %v", err)
		}
		if err := payment.BeginRefund(now); err != nil {
			t.Fatalf("BeginRefund() error = %v", err)
		}
		if payment.Status != PaymentStatusRefunding {
			t.Errorf("status = %v, want %v", payment.Status, PaymentStatusRefunding)
		}
		if !payment.UpdatedAt.Equal(now) {
			t.Errorf("UpdatedAt = %v, want %v", payment.UpdatedAt, now)
		}
		if payment.IsFinalized() {
			t.Error("refunding must not be a finalized state")
		}
	})

	t.Run("from pending", func(t *testing.T) {
		payment, _ := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodMonth, 1000, ProviderFake, now)
		if err := payment.BeginRefund(now); err != nil {
			t.Fatalf("BeginRefund() error = %v", err)
		}
		if payment.Status != PaymentStatusRefunding {
			t.Errorf("status = %v, want %v", payment.Status, PaymentStatusRefunding)
		}
	})

	t.Run("rejected from non-refundable", func(t *testing.T) {
		for _, status := range []PaymentStatus{PaymentStatusFailed, PaymentStatusRefunded, PaymentStatusRefunding} {
			payment, _ := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodMonth, 1000, ProviderFake, now)
			payment.Status = status
			if err := payment.BeginRefund(now); !errors.Is(err, ErrInvalidPaymentStatus) {
				t.Errorf("BeginRefund() from %s error = %v, want ErrInvalidPaymentStatus", status, err)
			}
		}
	})
}

func TestSubscriptionPaymentRevertRefund(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	subscriptionID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	t.Run("restores previous status", func(t *testing.T) {
		payment, _ := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodMonth, 1000, ProviderFake, now)
		if err := payment.MarkSucceeded(now); err != nil {
			t.Fatalf("MarkSucceeded() error = %v", err)
		}
		if err := payment.BeginRefund(now); err != nil {
			t.Fatalf("BeginRefund() error = %v", err)
		}
		if err := payment.RevertRefund(now, PaymentStatusSucceeded); err != nil {
			t.Fatalf("RevertRefund() error = %v", err)
		}
		if payment.Status != PaymentStatusSucceeded {
			t.Errorf("status = %v, want %v", payment.Status, PaymentStatusSucceeded)
		}
	})

	t.Run("rejected when not refunding", func(t *testing.T) {
		payment, _ := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodMonth, 1000, ProviderFake, now)
		if err := payment.MarkSucceeded(now); err != nil {
			t.Fatalf("MarkSucceeded() error = %v", err)
		}
		if err := payment.RevertRefund(now, PaymentStatusSucceeded); !errors.Is(err, ErrInvalidPaymentStatus) {
			t.Errorf("RevertRefund() error = %v, want ErrInvalidPaymentStatus", err)
		}
	})
}

func TestSubscriptionPaymentReconcileFromFailed(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	subscriptionID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	t.Run("to succeeded", func(t *testing.T) {
		payment, _ := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodMonth, 1000, ProviderFake, now)
		code := "error"
		_ = payment.MarkFailed(&code, now)
		if err := payment.ReconcileToSucceeded(now); err != nil {
			t.Fatalf("ReconcileToSucceeded() error = %v", err)
		}
		if payment.Status != PaymentStatusSucceeded {
			t.Errorf("status = %v, want %v", payment.Status, PaymentStatusSucceeded)
		}
		if payment.ErrorCode != nil {
			t.Errorf("error code = %v, want nil", payment.ErrorCode)
		}
		if payment.SucceededAt == nil || !payment.SucceededAt.Equal(now) {
			t.Errorf("succeeded at = %v, want %v", payment.SucceededAt, now)
		}
	})

	t.Run("to refunded", func(t *testing.T) {
		payment, _ := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodMonth, 1000, ProviderFake, now)
		code := "error"
		_ = payment.MarkFailed(&code, now)
		if err := payment.ReconcileToRefunded(now); err != nil {
			t.Fatalf("ReconcileToRefunded() error = %v", err)
		}
		if payment.Status != PaymentStatusRefunded {
			t.Errorf("status = %v, want %v", payment.Status, PaymentStatusRefunded)
		}
		if payment.RefundedAmountKopecks == nil || *payment.RefundedAmountKopecks != 1000 {
			t.Errorf("refunded amount = %v, want 1000", payment.RefundedAmountKopecks)
		}
	})

	t.Run("rejected from non-failed", func(t *testing.T) {
		payment, _ := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodMonth, 1000, ProviderFake, now)
		if err := payment.ReconcileToSucceeded(now); !errors.Is(err, ErrInvalidPaymentStatus) {
			t.Errorf("ReconcileToSucceeded() from pending error = %v, want ErrInvalidPaymentStatus", err)
		}
		if err := payment.ReconcileToRefunded(now); !errors.Is(err, ErrInvalidPaymentStatus) {
			t.Errorf("ReconcileToRefunded() from pending error = %v, want ErrInvalidPaymentStatus", err)
		}
	})
}

func TestSubscriptionPaymentMarkRefundedFromRefunding(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	subscriptionID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	payment, _ := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodMonth, 1000, ProviderFake, now)
	if err := payment.MarkSucceeded(now); err != nil {
		t.Fatalf("MarkSucceeded() error = %v", err)
	}
	if err := payment.BeginRefund(now); err != nil {
		t.Fatalf("BeginRefund() error = %v", err)
	}
	if err := payment.MarkRefunded(now); err != nil {
		t.Fatalf("MarkRefunded() from refunding error = %v", err)
	}
	if payment.Status != PaymentStatusRefunded {
		t.Errorf("status = %v, want %v", payment.Status, PaymentStatusRefunded)
	}
}
