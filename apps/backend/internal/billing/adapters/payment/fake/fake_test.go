package fake

import (
	"context"
	"encoding/json"
	"errors"
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
	_, err := p.ConfirmPayment(context.Background(), uuid.Must(uuid.NewV7()).String())
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
		PaymentID:         uuid.Must(uuid.NewV7()),
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
		PaymentID:         uuid.Must(uuid.NewV7()),
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
		PaymentID:         uuid.Must(uuid.NewV7()),
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
	paymentID := uuid.Must(uuid.NewV7())
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
	ctx := context.Background()

	if _, err := p.BindPaymentMethod(ctx, application.BindMethodRequest{}); err == nil {
		t.Error("expected BindPaymentMethod to require a customer ref")
	}

	// The raw-token capability mints the provider-side card fields around the
	// caller's token (issue #251): local runs drive the whole method flow
	// without the binding form.
	fromToken, err := p.AddPaymentMethodFromToken(ctx, "c1", "raw_token")
	if err != nil {
		t.Fatalf("AddPaymentMethodFromToken error: %v", err)
	}
	if fromToken.ChargeToken != "raw_token" || fromToken.ProviderMethodID == "" || fromToken.MaskedPan == "" {
		t.Fatalf("method from token: got %+v, want the token with card fields", fromToken)
	}
	if _, err := p.AddPaymentMethodFromToken(ctx, "c1", ""); err == nil {
		t.Error("expected AddPaymentMethodFromToken to require a token")
	}

	// A started binding is pending until confirmed and its form URL points at
	// the local confirmation endpoint (issue #251).
	bind, err := p.BindPaymentMethod(ctx, application.BindMethodRequest{CustomerRef: "c1"})
	if err != nil {
		t.Fatalf("BindPaymentMethod error: %v", err)
	}
	if bind.BindingID == "" {
		t.Fatal("BindPaymentMethod must return a binding id")
	}
	if want := "http://localhost:8080/internal/fake-card-binding/" + bind.BindingID + "/confirm"; bind.FormURL != want {
		t.Fatalf("FormURL: got %q, want %q", bind.FormURL, want)
	}
	state, err := p.PaymentMethodBinding(ctx, bind.BindingID)
	if err != nil {
		t.Fatalf("PaymentMethodBinding before confirm: %v", err)
	}
	if state.Status != application.MethodBindingPending || state.Method != nil {
		t.Fatalf("state before confirm: got %+v, want pending without a method", state)
	}

	// Confirming completes the binding and yields the add-card event; the
	// poll then reports the completed state with the bound method.
	event, err := p.ConfirmCardBinding(ctx, bind.BindingID)
	if err != nil {
		t.Fatalf("ConfirmCardBinding error: %v", err)
	}
	if event.MethodBound == nil || event.MethodBound.BindingID != bind.BindingID {
		t.Fatalf("confirm event: got %+v, want a method-bound notification", event.MethodBound)
	}
	if event.MethodBound.Method.ChargeToken == "" || event.MethodBound.Method.ProviderMethodID == "" {
		t.Fatalf("bound method: got %+v, want card id and charge token", event.MethodBound.Method)
	}
	state, err = p.PaymentMethodBinding(ctx, bind.BindingID)
	if err != nil {
		t.Fatalf("PaymentMethodBinding after confirm: %v", err)
	}
	if state.Status != application.MethodBindingCompleted || state.Method == nil ||
		state.Method.ChargeToken != event.MethodBound.Method.ChargeToken {
		t.Fatalf("state after confirm: got %+v, want the completed binding", state)
	}

	// Repeated confirmation returns the same notification (idempotent at the
	// provider; the application session state makes reprocessing a no-op).
	again, err := p.ConfirmCardBinding(ctx, bind.BindingID)
	if err != nil {
		t.Fatalf("ConfirmCardBinding(repeat) error: %v", err)
	}
	if again.MethodBound.Method.ChargeToken != event.MethodBound.Method.ChargeToken {
		t.Fatalf("repeat confirm changed the charge token: %q vs %q",
			again.MethodBound.Method.ChargeToken, event.MethodBound.Method.ChargeToken)
	}

	// An unknown request key is a forgotten binding, not a hard failure.
	if _, err := p.PaymentMethodBinding(ctx, "unknown"); !errors.Is(err, application.ErrProviderBindingNotFound) {
		t.Errorf("PaymentMethodBinding(unknown) error = %v, want ErrProviderBindingNotFound", err)
	}
	if _, err := p.ConfirmCardBinding(ctx, "unknown"); !errors.Is(err, application.ErrProviderBindingNotFound) {
		t.Errorf("ConfirmCardBinding(unknown) error = %v, want ErrProviderBindingNotFound", err)
	}

	// Programmed states still take precedence over real entries, so tests can
	// force outcomes the local flow cannot produce.
	method := application.SavedMethod{ProviderMethodID: "card-1", ChargeToken: "t"}
	p.SetBindingState("binding-1", application.MethodBindingState{
		Status: application.MethodBindingCompleted,
		Method: &method,
	})
	state, err = p.PaymentMethodBinding(ctx, "binding-1")
	if err != nil {
		t.Fatalf("PaymentMethodBinding(programmed) error: %v", err)
	}
	if state.Status != application.MethodBindingCompleted || state.Method.ChargeToken != "t" {
		t.Fatalf("programmed state: got %+v", state)
	}

	if err := p.RemovePaymentMethod(ctx, "c1", "card-1"); err != nil {
		t.Errorf("RemovePaymentMethod error: %v", err)
	}

	methods, err := p.ListPaymentMethods(ctx, "c1")
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
