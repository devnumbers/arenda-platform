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
	_, err := NewSubscriptionPayment(userID, subscriptionID, tariffID, nil, PeriodMonth, -1, ProviderFake, now)
	if !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("NewSubscriptionPayment() error = %v, want ErrInvalidAmount", err)
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
