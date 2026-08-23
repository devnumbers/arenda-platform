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

// testProviderPaymentID is the shared fake provider payment id fixture.
const testProviderPaymentID = "fake_1"

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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestProviderStatusUnknownPaymentAnswersNotFound pins the GetState contract
// (issue #420): a payment this provider instance never saw answers
// ErrProviderPaymentNotFound, never a fabricated success — the same semantics
// the real adapter maps from T-Kassa error code 255.
func TestProviderStatusUnknownPaymentAnswersNotFound(t *testing.T) {
	t.Parallel()
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)

	_, err := p.PaymentStatus(context.Background(), uuid.Must(uuid.NewV7()), "fake_never_seen")
	if !errors.Is(err, application.ErrProviderPaymentNotFound) {
		t.Fatalf("PaymentStatus(unknown) error = %v, want ErrProviderPaymentNotFound", err)
	}
}

// TestProviderStatusPurgedPendingAnswersNotFound proves the TTL purge makes a
// pending payment forgotten: once the session is purged, its status read
// answers not-found instead of assuming the payment completed.
func TestProviderStatusPurgedPendingAnswersNotFound(t *testing.T) {
	t.Parallel()
	clk := newTestClock(time.Now())
	p := NewProvider("http://localhost:8080", discardLogger(), clk, nil)
	paymentID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	init, err := p.InitPayment(context.Background(), validInitRequest(paymentID))
	if err != nil {
		t.Fatalf("InitPayment error: %v", err)
	}

	clk.Add(pendingTTL + time.Minute)
	_, err = p.PaymentStatus(context.Background(), paymentID, init.ProviderPaymentID)
	if !errors.Is(err, application.ErrProviderPaymentNotFound) {
		t.Fatalf("PaymentStatus(purged pending) error = %v, want ErrProviderPaymentNotFound", err)
	}
}

// TestProviderStatusRemembersFinalOutcomes proves a payment the provider
// finalized stays known: after a failed confirm and after charges, the status
// read reports the actual outcome — not the assumed success of the pre-contract
// fake.
func TestProviderStatusRemembersFinalOutcomes(t *testing.T) {
	t.Parallel()
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	ctx := context.Background()

	failedID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	if _, err := p.InitPayment(ctx, validInitRequest(failedID)); err != nil {
		t.Fatalf("InitPayment error: %v", err)
	}
	if _, err := p.ConfirmPaymentFailed(ctx, failedID.String(), nil); err != nil {
		t.Fatalf("ConfirmPaymentFailed error: %v", err)
	}
	failed, err := p.PaymentStatus(ctx, failedID, "")
	if err != nil {
		t.Fatalf("PaymentStatus(failed confirm) error: %v", err)
	}
	if failed.Status != domain.PaymentStatusFailed || failed.ErrorCode != defaultErrorCode {
		t.Errorf("status after failed confirm = %+v, want failed with the default error code", failed)
	}

	// A charge finalizes the provider-side payment even without a prior init —
	// GetState keeps resolving the outcome afterwards.
	chargedOK := uuid.Must(uuid.NewV7())
	if _, err := p.ChargePayment(ctx, application.ChargeRequest{
		PaymentID: chargedOK, AmountKopecks: 5000, ChargeToken: "fake_token_1",
	}); err != nil {
		t.Fatalf("ChargePayment error: %v", err)
	}
	okStatus, err := p.PaymentStatus(ctx, chargedOK, "")
	if err != nil {
		t.Fatalf("PaymentStatus(charged) error: %v", err)
	}
	if okStatus.Status != domain.PaymentStatusSucceeded {
		t.Errorf("status after charge = %q, want succeeded", okStatus.Status)
	}

	chargedFail := uuid.Must(uuid.NewV7())
	if _, err := p.ChargePayment(ctx, application.ChargeRequest{
		PaymentID: chargedFail, AmountKopecks: 5000, ChargeToken: fakeFailTokenPrefix + "1",
	}); err != nil {
		t.Fatalf("ChargePayment(declined) error: %v", err)
	}
	failStatus, err := p.PaymentStatus(ctx, chargedFail, "")
	if err != nil {
		t.Fatalf("PaymentStatus(declined charge) error: %v", err)
	}
	if failStatus.Status != domain.PaymentStatusFailed || failStatus.ErrorCode != defaultErrorCode {
		t.Errorf("status after declined charge = %+v, want failed with the default error code", failStatus)
	}
}

func TestProviderConfirmPaymentUnknownID(t *testing.T) {
	t.Parallel()
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	_, err := p.ConfirmPayment(context.Background(), uuid.Must(uuid.NewV7()).String())
	if err == nil {
		t.Fatal("expected error for unknown payment")
	}
}

func TestProviderPendingPurgedAfterTTL(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)

	res, err := p.ChargePayment(context.Background(), application.ChargeRequest{
		PaymentID:         uuid.Must(uuid.NewV7()),
		ProviderPaymentID: testProviderPaymentID,
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
	t.Parallel()
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)

	res, err := p.RefundPayment(context.Background(), application.RefundRequest{
		PaymentID:         uuid.Must(uuid.NewV7()),
		ProviderPaymentID: testProviderPaymentID,
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
	t.Parallel()
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	paymentID := uuid.Must(uuid.NewV7())
	payload, err := json.Marshal(map[string]any{
		"provider_payment_id": testProviderPaymentID,
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

// TestProviderParseWebhookCardBindingRefusal covers the fake counterpart of
// the refused add-card notification: a request-key payload with a failed
// status parses into the binding-failed event the refused-webhook flow
// consumes (issue #422).
func TestProviderParseWebhookCardBindingRefusal(t *testing.T) {
	t.Parallel()
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	payload, err := json.Marshal(map[string]any{
		"request_key": "req_refused",
		"status":      "failed",
		"error_code":  "6",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	event, err := p.ParseWebhook(context.Background(), payload)
	if err != nil {
		t.Fatalf("ParseWebhook error: %v", err)
	}
	if event.Payment != nil || event.MethodBound != nil {
		t.Fatalf("event = %+v, want only the binding-failed payload", event)
	}
	if event.MethodBindingFailed == nil {
		t.Fatal("expected binding-failed event")
	}
	if event.MethodBindingFailed.BindingID != "req_refused" || event.MethodBindingFailed.ErrorCode != "6" {
		t.Errorf("binding-failed notification: got %+v", event.MethodBindingFailed)
	}

	// Binding payloads with a non-failure status stay unsupported.
	if _, err := p.ParseWebhook(context.Background(), []byte(`{"request_key":"req_x","status":"pending"}`)); err == nil {
		t.Error("expected error for unsupported binding webhook status")
	}
}

func TestProviderAddPaymentMethodFromToken(t *testing.T) {
	t.Parallel()
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
}

// startFakeBinding starts a binding and proves the form URL points at the
// local confirmation endpoint (issue #251).
func startFakeBinding(t *testing.T, p *Provider, ctx context.Context) application.BindMethodResult {
	t.Helper()
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
	return bind
}

// confirmFakeBinding confirms the binding and proves the add-card event shape.
func confirmFakeBinding(t *testing.T, p *Provider, ctx context.Context, bindingID string) application.MethodBoundNotification {
	t.Helper()
	event, err := p.ConfirmCardBinding(ctx, bindingID)
	if err != nil {
		t.Fatalf("ConfirmCardBinding error: %v", err)
	}
	if event.MethodBound == nil || event.MethodBound.BindingID != bindingID {
		t.Fatalf("confirm event: got %+v, want a method-bound notification", event.MethodBound)
	}
	if event.MethodBound.Method.ChargeToken == "" || event.MethodBound.Method.ProviderMethodID == "" {
		t.Fatalf("bound method: got %+v, want card id and charge token", event.MethodBound.Method)
	}
	return *event.MethodBound
}

// requireFakeBindingCompleted polls the binding and proves the completed state
// carries the bound method.
func requireFakeBindingCompleted(t *testing.T, p *Provider, ctx context.Context, bindingID, chargeToken string) {
	t.Helper()
	state, err := p.PaymentMethodBinding(ctx, bindingID)
	if err != nil {
		t.Fatalf("PaymentMethodBinding after confirm: %v", err)
	}
	if state.Status != application.MethodBindingCompleted || state.Method == nil ||
		state.Method.ChargeToken != chargeToken {
		t.Fatalf("state after confirm: got %+v, want the completed binding", state)
	}
}

// TestProviderBindingAPIs drives the binding lifecycle: a started binding is
// pending until confirmed, its form URL points at the local confirmation
// endpoint (issue #251), and confirming completes it idempotently.
func TestProviderBindingAPIs(t *testing.T) {
	t.Parallel()
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	ctx := context.Background()

	bind := startFakeBinding(t, p, ctx)
	state, err := p.PaymentMethodBinding(ctx, bind.BindingID)
	if err != nil {
		t.Fatalf("PaymentMethodBinding before confirm: %v", err)
	}
	if state.Status != application.MethodBindingPending || state.Method != nil {
		t.Fatalf("state before confirm: got %+v, want pending without a method", state)
	}

	// Confirming completes the binding and yields the add-card event; the
	// poll then reports the completed state with the bound method.
	bound := confirmFakeBinding(t, p, ctx, bind.BindingID)
	requireFakeBindingCompleted(t, p, ctx, bind.BindingID, bound.Method.ChargeToken)

	// Repeated confirmation returns the same notification (idempotent at the
	// provider; the application session state makes reprocessing a no-op).
	again, err := p.ConfirmCardBinding(ctx, bind.BindingID)
	if err != nil {
		t.Fatalf("ConfirmCardBinding(repeat) error: %v", err)
	}
	if again.MethodBound.Method.ChargeToken != bound.Method.ChargeToken {
		t.Fatalf("repeat confirm changed the charge token: %q vs %q",
			again.MethodBound.Method.ChargeToken, bound.Method.ChargeToken)
	}
}

// TestProviderProgrammedBindingStateAndRemoval proves programmed states take
// precedence over real entries (tests can force outcomes the local flow cannot
// produce), and that removing a method empties the listing.
func TestProviderProgrammedBindingStateAndRemoval(t *testing.T) {
	t.Parallel()
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	ctx := context.Background()

	// An unknown request key is a forgotten binding, not a hard failure.
	if _, err := p.PaymentMethodBinding(ctx, "unknown"); !errors.Is(err, application.ErrProviderBindingNotFound) {
		t.Errorf("PaymentMethodBinding(unknown) error = %v, want ErrProviderBindingNotFound", err)
	}
	if _, err := p.ConfirmCardBinding(ctx, "unknown"); !errors.Is(err, application.ErrProviderBindingNotFound) {
		t.Errorf("ConfirmCardBinding(unknown) error = %v, want ErrProviderBindingNotFound", err)
	}

	method := application.SavedMethod{ProviderMethodID: "card-1", ChargeToken: "t"}
	p.SetBindingState("binding-1", application.MethodBindingState{
		Status: application.MethodBindingCompleted,
		Method: &method,
	})
	state, err := p.PaymentMethodBinding(ctx, "binding-1")
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
	t.Parallel()
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	if got, want := string(p.WebhookAck()), `{"status":"ok"}`; got != want {
		t.Fatalf("WebhookAck: got %q, want %q", got, want)
	}
}

func TestProviderName(t *testing.T) {
	t.Parallel()
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	if got, want := p.Name(), domain.PaymentProvider("fake"); got != want {
		t.Fatalf("Name: got %q, want %q", got, want)
	}
}

// TestProviderSatisfiesPort asserts the aggregate port at runtime in addition
// to the package-level compile-time assertions.
func TestProviderSatisfiesPort(t *testing.T) {
	t.Parallel()
	var provider application.PaymentProvider = NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()), nil)
	if got, want := provider.Name(), domain.PaymentProvider("fake"); got != want {
		t.Fatalf("Name: got %q, want %q", got, want)
	}
}
