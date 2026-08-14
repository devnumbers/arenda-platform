package fake

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

type testClock struct {
	t time.Time
}

func newTestClock(t time.Time) *testClock {
	return &testClock{t: t}
}

func (c *testClock) Now() time.Time { return c.t }

func (c *testClock) Add(d time.Duration) { c.t = c.t.Add(d) }

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func validInitRequest(paymentID uuid.UUID) application.InitPaymentRequest {
	return application.InitPaymentRequest{
		PaymentID:     paymentID,
		AmountKopecks: 10000,
		Period:        domain.PeriodMonth,
		CustomerRef:   "22222222-2222-2222-2222-222222222222",
		Purpose: application.PaymentPurpose{
			Kind:       application.PaymentPurposeSubscription,
			TariffName: domain.TariffPro,
			Period:     domain.PeriodMonth,
		},
		SaveMethod: true,
		Initiator:  application.InitiatorCustomer,
	}
}

func TestProviderInitPayment(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	paymentID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	res, err := p.InitPayment(context.Background(), validInitRequest(paymentID))
	if err != nil {
		t.Fatalf("InitPayment error: %v", err)
	}

	if res.Status != domain.PaymentStatusPending {
		t.Errorf("expected status %q, got %q", domain.PaymentStatusPending, res.Status)
	}
	if !strings.HasPrefix(res.ProviderPaymentID, "fake_") {
		t.Errorf("expected provider payment id to start with fake_, got %q", res.ProviderPaymentID)
	}
	if res.SavedMethod == nil || !strings.HasPrefix(res.SavedMethod.ChargeToken, "fake_token_") {
		t.Errorf("expected saved method with fake_token_ charge token, got %+v", res.SavedMethod)
	}
	wantURL := "http://localhost:8080/internal/fake-subscription-payment/" + paymentID.String() + "/confirm"
	if res.PaymentURL != wantURL {
		t.Errorf("expected payment URL %q, got %q", wantURL, res.PaymentURL)
	}
}

func TestProviderInitPaymentIdempotentByPaymentID(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	paymentID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	req := validInitRequest(paymentID)

	first, err := p.InitPayment(context.Background(), req)
	if err != nil {
		t.Fatalf("first InitPayment error: %v", err)
	}
	second, err := p.InitPayment(context.Background(), req)
	if err != nil {
		t.Fatalf("second InitPayment error: %v", err)
	}

	if first.ProviderPaymentID != second.ProviderPaymentID {
		t.Errorf("expected same provider payment id, got %q and %q", first.ProviderPaymentID, second.ProviderPaymentID)
	}
	if first.PaymentURL != second.PaymentURL {
		t.Errorf("expected same confirm URL, got %q and %q", first.PaymentURL, second.PaymentURL)
	}
	if first.SavedMethod.ChargeToken != second.SavedMethod.ChargeToken {
		t.Errorf("expected same charge token, got %q and %q", first.SavedMethod.ChargeToken, second.SavedMethod.ChargeToken)
	}
}

func TestProviderInitPaymentValidation(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	validPaymentID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	cases := []struct {
		name    string
		mutate  func(req *application.InitPaymentRequest)
		wantErr string
	}{
		{
			name:    "nil payment id",
			mutate:  func(req *application.InitPaymentRequest) { req.PaymentID = uuid.Nil },
			wantErr: "payment id is required",
		},
		{
			name:    "zero amount",
			mutate:  func(req *application.InitPaymentRequest) { req.AmountKopecks = 0 },
			wantErr: "amount must be positive",
		},
		{
			name:    "empty initiator",
			mutate:  func(req *application.InitPaymentRequest) { req.Initiator = "" },
			wantErr: "unknown operation initiator",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			req := validInitRequest(validPaymentID)
			tt.mutate(&req)
			_, err := p.InitPayment(context.Background(), req)
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestProviderConfirmPaymentFlow(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	paymentID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	init, err := p.InitPayment(context.Background(), validInitRequest(paymentID))
	if err != nil {
		t.Fatalf("InitPayment error: %v", err)
	}

	event, err := p.ConfirmPayment(context.Background(), paymentID.String())
	if err != nil {
		t.Fatalf("ConfirmPayment error: %v", err)
	}
	if event.Payment == nil || event.Payment.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("expected succeeded payment event, got %+v", event)
	}
	if event.Payment.AmountKopecks != 10000 {
		t.Errorf("amount: got %d, want 10000", event.Payment.AmountKopecks)
	}
	if event.Payment.SavedMethod == nil || event.Payment.SavedMethod.ChargeToken != init.SavedMethod.ChargeToken {
		t.Errorf("expected saved method with the init token, got %+v", event.Payment.SavedMethod)
	}

	// Status resolves a confirmed payment as succeeded and reports the
	// provider payment id of the init.
	status, err := p.PaymentStatus(context.Background(), paymentID, init.ProviderPaymentID)
	if err != nil {
		t.Fatalf("PaymentStatus error: %v", err)
	}
	if status.Status != domain.PaymentStatusSucceeded {
		t.Errorf("status: got %q, want %q", status.Status, domain.PaymentStatusSucceeded)
	}
}

func TestProviderConfirmPaymentFailedFlow(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	paymentID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	if _, err := p.InitPayment(context.Background(), validInitRequest(paymentID)); err != nil {
		t.Fatalf("InitPayment error: %v", err)
	}

	event, err := p.ConfirmPaymentFailed(context.Background(), paymentID.String(), nil)
	if err != nil {
		t.Fatalf("ConfirmPaymentFailed error: %v", err)
	}
	if event.Payment == nil || event.Payment.Status != domain.PaymentStatusFailed {
		t.Fatalf("expected failed payment event, got %+v", event)
	}
	if event.Payment.ErrorCode == nil || *event.Payment.ErrorCode == "" {
		t.Errorf("expected a default error code on failed confirm")
	}
	if event.Payment.SavedMethod != nil {
		t.Errorf("failed confirm must not produce a saved method")
	}
}

func TestProviderConfirmPaymentUnknownID(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	_, err := p.ConfirmPayment(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error for unknown payment")
	}
}

func TestProviderPendingPurgedAfterTTL(t *testing.T) {
	clk := newTestClock(time.Now())
	p := NewProvider("http://localhost:8080", discardLogger(), clk, nil)
	paymentID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	if _, err := p.InitPayment(context.Background(), validInitRequest(paymentID)); err != nil {
		t.Fatalf("InitPayment error: %v", err)
	}
	clk.Add(pendingTTL + time.Minute)
	if _, err := p.ConfirmPayment(context.Background(), paymentID.String()); err == nil {
		t.Fatal("expected error for purged pending payment")
	}
}

func TestProviderChargePayment(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)

	res, err := p.ChargePayment(context.Background(), application.ChargeRequest{
		PaymentID:         uuid.New(),
		ProviderPaymentID: "fake_1",
		AmountKopecks:     5000,
		ChargeToken:       "fake_token_1",
	})
	if err != nil {
		t.Fatalf("ChargePayment error: %v", err)
	}
	if res.Status != domain.PaymentStatusSucceeded {
		t.Errorf("status: got %q, want %q", res.Status, domain.PaymentStatusSucceeded)
	}

	// A charge token with the fail prefix declines.
	failRes, err := p.ChargePayment(context.Background(), application.ChargeRequest{
		PaymentID:         uuid.New(),
		ProviderPaymentID: "fake_2",
		AmountKopecks:     5000,
		ChargeToken:       fakeFailTokenPrefix + "1",
	})
	if err != nil {
		t.Fatalf("ChargePayment error: %v", err)
	}
	if failRes.Status != domain.PaymentStatusFailed {
		t.Errorf("status: got %q, want %q", failRes.Status, domain.PaymentStatusFailed)
	}
	if failRes.ErrorCode == "" {
		t.Errorf("expected error code on failed charge")
	}
}

func TestProviderRefundPayment(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)

	res, err := p.RefundPayment(context.Background(), application.RefundRequest{
		PaymentID:         uuid.New(),
		ProviderPaymentID: "fake_1",
		AmountKopecks:     10000,
	})
	if err != nil {
		t.Fatalf("RefundPayment error: %v", err)
	}
	if res.Status != domain.PaymentStatusRefunded {
		t.Errorf("status: got %q, want %q", res.Status, domain.PaymentStatusRefunded)
	}
	if res.RefundedAmountKopecks != 10000 {
		t.Errorf("refunded amount: got %d, want 10000", res.RefundedAmountKopecks)
	}
}

func TestProviderParseWebhook(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	paymentID := uuid.New()
	payload, err := json.Marshal(map[string]any{
		"provider_payment_id": "fake_1",
		"internal_payment_id": paymentID.String(),
		"status":              "succeeded",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	event, err := p.ParseWebhook(context.Background(), payload)
	if err != nil {
		t.Fatalf("ParseWebhook error: %v", err)
	}
	if event.Payment == nil {
		t.Fatal("expected payment event")
	}
	if event.Payment.InternalPaymentID != paymentID {
		t.Errorf("internal payment id: got %v, want %v", event.Payment.InternalPaymentID, paymentID)
	}
	if event.Payment.Status != domain.PaymentStatusSucceeded {
		t.Errorf("status: got %q, want succeeded", event.Payment.Status)
	}

	// Unsupported statuses and broken payloads are rejected.
	if _, err := p.ParseWebhook(context.Background(), []byte(`{"status":"refunding"}`)); err == nil {
		t.Error("expected error for unsupported webhook status")
	}
	if _, err := p.ParseWebhook(context.Background(), []byte(`not json`)); err == nil {
		t.Error("expected error for invalid JSON")
	}
	if _, err := p.ParseWebhook(context.Background(), []byte(`{"internal_payment_id":"nope","status":"succeeded"}`)); err == nil {
		t.Error("expected error for invalid internal payment id")
	}
}

func TestProviderBindingAPIs(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)

	if _, err := p.BindPaymentMethod(context.Background(), application.BindMethodRequest{CustomerRef: "c1"}); err == nil {
		t.Error("expected BindPaymentMethod to be unsupported")
	}

	method := application.SavedMethod{ProviderMethodID: "card-1", ChargeToken: "t"}
	p.SetBindingState("binding-1", application.MethodBindingState{
		Status: application.MethodBindingCompleted,
		Method: &method,
	})
	state, err := p.PaymentMethodBinding(context.Background(), "binding-1")
	if err != nil {
		t.Fatalf("PaymentMethodBinding error: %v", err)
	}
	if state.Status != application.MethodBindingCompleted || state.Method.ChargeToken != "t" {
		t.Fatalf("state: got %+v", state)
	}

	if _, err := p.PaymentMethodBinding(context.Background(), "unknown"); err == nil {
		t.Error("expected error for unprogrammed binding id")
	}

	if err := p.RemovePaymentMethod(context.Background(), "c1", "card-1"); err != nil {
		t.Errorf("RemovePaymentMethod error: %v", err)
	}

	methods, err := p.ListPaymentMethods(context.Background(), "c1")
	if err != nil {
		t.Fatalf("ListPaymentMethods error: %v", err)
	}
	if len(methods) != 0 {
		t.Errorf("methods: got %d, want 0", len(methods))
	}
}

func TestProviderWebhookAck(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	if got, want := string(p.WebhookAck()), `{"status":"ok"}`; got != want {
		t.Fatalf("WebhookAck: got %q, want %q", got, want)
	}
}

func TestProviderName(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	if got, want := p.Name(), domain.PaymentProvider("fake"); got != want {
		t.Fatalf("Name: got %q, want %q", got, want)
	}
}

// TestProviderSatisfiesPort asserts the aggregate port at runtime in addition
// to the package-level compile-time assertions.
func TestProviderSatisfiesPort(t *testing.T) {
	var provider application.PaymentProvider = NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	if got, want := provider.Name(), domain.PaymentProvider("fake"); got != want {
		t.Fatalf("Name: got %q, want %q", got, want)
	}
}
