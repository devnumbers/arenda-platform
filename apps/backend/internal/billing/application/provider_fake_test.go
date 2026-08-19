// Package application_test exercises the provider-neutral payment port with
// the fake adapter (issue #248): the fake implements the port exactly like the
// real adapter, so application-layer tests drive payments through it the same
// way production code drives the real provider. The payment-flow services
// themselves land with #250-#252 and consume the same port.
package application_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	paymentfake "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/fake"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// testProviderFake is the provider identity shared by the external application tests.
const testProviderFake = "fake"

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func newFakeProvider(t *testing.T) application.PaymentProvider {
	t.Helper()
	return paymentfake.NewProvider(
		"http://localhost:8080",
		slog.New(slog.DiscardHandler),
		fixedClock{t: time.Now()},
		nil, // Nil metrics is safe: RecordRequest is nil-safe.
	)
}

// TestProviderPortRoundTrip drives the fake provider through the port types
// the application layer owns: init a customer-initiated save-method payment,
// confirm it via the provider's webhook-shaped event, read the saved method
// back, charge it merchant-initiated, and refund it.
func TestProviderPortRoundTrip(t *testing.T) {
	ctx := context.Background()
	provider := newFakeProvider(t)
	paymentID := mustUUID(t)

	init, err := provider.InitPayment(ctx, application.InitPaymentRequest{
		PaymentID:     paymentID,
		AmountKopecks: 10000,
		Period:        domain.PeriodMonth,
		CustomerRef:   "customer-1",
		Purpose: application.PaymentPurpose{
			Kind:       application.PaymentPurposeSubscription,
			TariffName: domain.TariffPro,
			Period:     domain.PeriodMonth,
		},
		SaveMethod: true,
		Initiator:  application.InitiatorCustomer,
	})
	if err != nil {
		t.Fatalf("InitPayment: %v", err)
	}
	if init.Status != domain.PaymentStatusPending {
		t.Fatalf("init status: got %q, want pending", init.Status)
	}
	if init.PaymentURL == "" {
		t.Fatal("init must return a payment URL for customer-initiated payments")
	}
	if init.SavedMethod == nil || init.SavedMethod.ChargeToken == "" {
		t.Fatal("fake init must surface the saved method synchronously")
	}

	// The confirmation resolves the payment the way a webhook would.
	status, err := provider.PaymentStatus(ctx, paymentID, init.ProviderPaymentID)
	if err != nil {
		t.Fatalf("PaymentStatus: %v", err)
	}
	if status.Status != domain.PaymentStatusPending {
		t.Fatalf("status before confirm: got %q, want pending", status.Status)
	}

	chargeToken := init.SavedMethod.ChargeToken
	charge, err := provider.ChargePayment(ctx, application.ChargeRequest{
		PaymentID:         mustUUID(t),
		ProviderPaymentID: init.ProviderPaymentID,
		AmountKopecks:     10000,
		ChargeToken:       chargeToken,
	})
	if err != nil {
		t.Fatalf("ChargePayment: %v", err)
	}
	if charge.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("charge status: got %q, want succeeded", charge.Status)
	}

	refund, err := provider.RefundPayment(ctx, application.RefundRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: init.ProviderPaymentID,
		AmountKopecks:     10000,
	})
	if err != nil {
		t.Fatalf("RefundPayment: %v", err)
	}
	if refund.Status != domain.PaymentStatusRefunded {
		t.Fatalf("refund status: got %q, want refunded", refund.Status)
	}
	if refund.RefundedAmountKopecks != 10000 {
		t.Fatalf("refunded amount: got %d, want 10000", refund.RefundedAmountKopecks)
	}
}

// TestProviderPortWebhookShape checks the neutral webhook event consumed by
// the (upcoming) webhook flow: the fake's webhook payload parses into the
// port's WebhookEvent with the payment branch populated.
func TestProviderPortWebhookShape(t *testing.T) {
	ctx := context.Background()
	provider := newFakeProvider(t)
	paymentID := mustUUID(t)

	init, err := provider.InitPayment(ctx, application.InitPaymentRequest{
		PaymentID:     paymentID,
		AmountKopecks: 5000,
		Period:        domain.PeriodMonth,
		CustomerRef:   "customer-2",
		Purpose: application.PaymentPurpose{
			Kind:       application.PaymentPurposeRenewal,
			TariffName: domain.TariffBasic,
			Period:     domain.PeriodMonth,
		},
		Initiator: application.InitiatorMerchant,
	})
	if err != nil {
		t.Fatalf("InitPayment: %v", err)
	}

	payload := []byte(`{"provider_payment_id":"` + init.ProviderPaymentID +
		`","internal_payment_id":"` + paymentID.String() + `","status":"succeeded"}`)
	event, err := provider.ParseWebhook(ctx, payload)
	if err != nil {
		t.Fatalf("ParseWebhook: %v", err)
	}
	if event.MethodBound != nil {
		t.Fatal("MethodBound must be nil for payment webhooks")
	}
	if event.Payment == nil {
		t.Fatal("Payment branch missing")
	}
	if event.Payment.InternalPaymentID != paymentID {
		t.Fatalf("internal payment id: got %v, want %v", event.Payment.InternalPaymentID, paymentID)
	}
	if event.Payment.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("status: got %q, want succeeded", event.Payment.Status)
	}
	if string(provider.WebhookAck()) == "" {
		t.Fatal("webhook ack must not be empty")
	}
}

// TestProviderPortInitiatorContract pins the port contract shared by every
// adapter: the initiator is mandatory and the empty value is rejected before
// any provider interaction.
func TestProviderPortInitiatorContract(t *testing.T) {
	ctx := context.Background()
	provider := newFakeProvider(t)

	_, err := provider.InitPayment(ctx, application.InitPaymentRequest{
		PaymentID:     mustUUID(t),
		AmountKopecks: 100,
		Period:        domain.PeriodMonth,
		CustomerRef:   "customer-3",
		Purpose: application.PaymentPurpose{
			Kind:       application.PaymentPurposeSubscription,
			TariffName: domain.TariffPro,
			Period:     domain.PeriodMonth,
		},
		Initiator: "",
	})
	if err == nil {
		t.Fatal("expected the empty initiator to be rejected")
	}
}

func mustUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	return id
}
