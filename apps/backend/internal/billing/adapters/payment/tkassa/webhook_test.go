package tkassa

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

func signedWebhookPayload(t *testing.T, payload map[string]any) []byte {
	t.Helper()
	payload["Token"] = sign(payload, testPassword)
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal webhook: %v", err)
	}
	return body
}

func TestParseWebhookPayment(t *testing.T) {
	t.Parallel()
	paymentID := uuid.MustParse(testPaymentUUID)
	payload := map[string]any{
		fieldTerminalKey: testTerminalKey,
		fieldOrderID:     paymentID.String(),
		fieldStatus:      statusConfirmed,
		fieldSuccess:     true,
		fieldPaymentID:   json.Number("12345"),
		fieldAmount:      json.Number("10000"),
	}

	p := newTestProvider("")
	event, err := p.ParseWebhook(context.Background(), signedWebhookPayload(t, payload))
	if err != nil {
		t.Fatalf("ParseWebhook error: %v", err)
	}
	if event.MethodBound != nil {
		t.Fatalf("MethodBound must be nil for payment notifications")
	}
	if event.Payment == nil {
		t.Fatal("Payment notification payload missing")
	}
	got := event.Payment
	if got.InternalPaymentID != paymentID {
		t.Errorf("InternalPaymentID: got %v, want %v", got.InternalPaymentID, paymentID)
	}
	if got.ProviderPaymentID != "12345" {
		t.Errorf("ProviderPaymentID: got %q, want %q", got.ProviderPaymentID, "12345")
	}
	if got.Status != domain.PaymentStatusSucceeded {
		t.Errorf("Status: got %v, want %v", got.Status, domain.PaymentStatusSucceeded)
	}
	if got.ErrorCode != nil {
		t.Errorf("ErrorCode: got %v, want nil on success", *got.ErrorCode)
	}
	if got.AmountKopecks != 10000 {
		t.Errorf("AmountKopecks: got %d, want %d", got.AmountKopecks, 10000)
	}
	if got.SavedMethod != nil {
		t.Errorf("SavedMethod must be nil without RebillId, got %+v", *got.SavedMethod)
	}
}

// TestParseWebhookPaymentWithRebillIdSurfacesSavedMethod covers the token
// delivery moment of a save-method chain: the RebillId arriving in a payment
// notification (typically AUTHORIZED) becomes the saved method.
func TestParseWebhookPaymentWithRebillIdSurfacesSavedMethod(t *testing.T) {
	t.Parallel()
	paymentID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	payload := map[string]any{
		fieldTerminalKey: testTerminalKey,
		fieldOrderID:     paymentID.String(),
		fieldStatus:      statusAuthorized,
		fieldSuccess:     true,
		fieldPaymentID:   json.Number("777"),
		fieldAmount:      json.Number("500"),
		fieldRebillID:    "rebill-777",
		fieldCardID:      "card-777",
		fieldPan:         testMaskedPan,
		fieldExpDate:     "1230",
		"CustomerKey":    testCustomerRef,
	}

	p := newTestProvider("")
	event, err := p.ParseWebhook(context.Background(), signedWebhookPayload(t, payload))
	if err != nil {
		t.Fatalf("ParseWebhook error: %v", err)
	}
	if event.Payment == nil || event.Payment.SavedMethod == nil {
		t.Fatal("SavedMethod missing on payment with RebillId")
	}
	m := event.Payment.SavedMethod
	if m.ProviderMethodID != "card-777" || m.ChargeToken != "rebill-777" ||
		m.MaskedPan != testMaskedPan || m.ExpDate != "1230" || m.CustomerRef != testCustomerRef {
		t.Fatalf("SavedMethod: got %+v", *m)
	}
}

func TestParseWebhookFailedPaymentCarriesErrorCode(t *testing.T) {
	t.Parallel()
	paymentID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	payload := map[string]any{
		fieldTerminalKey: testTerminalKey,
		fieldOrderID:     paymentID.String(),
		fieldStatus:      statusRejected,
		fieldSuccess:     false,
		fieldPaymentID:   json.Number("888"),
		fieldErrorCode:   "103",
	}

	p := newTestProvider("")
	event, err := p.ParseWebhook(context.Background(), signedWebhookPayload(t, payload))
	if err != nil {
		t.Fatalf("ParseWebhook error: %v", err)
	}
	if event.Payment.Status != domain.PaymentStatusFailed {
		t.Errorf("Status: got %v, want %v", event.Payment.Status, domain.PaymentStatusFailed)
	}
	if event.Payment.ErrorCode == nil || *event.Payment.ErrorCode != "103" {
		t.Errorf("ErrorCode: got %v, want 103", event.Payment.ErrorCode)
	}
}

func TestParseWebhookRefundStatuses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		status     string
		amount     int64
		wantStatus domain.PaymentStatus
		wantAmount int64
	}{
		{"refunded", "REFUNDED", 99000, domain.PaymentStatusRefunded, 99000},
		// The new domain has no partial-refund status: PARTIAL_REFUNDED lands
		// in refunded (ADR 0037, issue #246 §3.1).
		{"partial_refunded", "PARTIAL_REFUNDED", 49000, domain.PaymentStatusRefunded, 49000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			payload := map[string]any{
				fieldTerminalKey: testTerminalKey,
				fieldOrderID:     "22222222-2222-2222-2222-222222222222",
				fieldPaymentID:   json.Number("12345"),
				fieldStatus:      tt.status,
				fieldAmount:      json.Number(strconv.FormatInt(tt.amount, 10)),
				fieldSuccess:     true,
			}

			p := newTestProvider("")
			event, err := p.ParseWebhook(context.Background(), signedWebhookPayload(t, payload))
			if err != nil {
				t.Fatalf("ParseWebhook error: %v", err)
			}
			if event.Payment.Status != tt.wantStatus {
				t.Errorf("status = %v, want %v", event.Payment.Status, tt.wantStatus)
			}
			if event.Payment.AmountKopecks != tt.wantAmount {
				t.Errorf("amount = %v, want %v", event.Payment.AmountKopecks, tt.wantAmount)
			}
		})
	}
}

func TestParseWebhookAddCard(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		fieldTerminalKey:      testTerminalKey,
		fieldRequestKey:       testRequestKey,
		"CustomerKey":         testCustomerRef,
		fieldCardID:           testCardID,
		fieldRebillID:         testRebillID,
		fieldPan:              testMaskedPan,
		fieldExpDate:          "1230",
		fieldStatus:           statusCompleted,
		fieldSuccess:          true,
		fieldNotificationType: notificationTypeAddCard,
	}

	p := newTestProvider("")
	event, err := p.ParseWebhook(context.Background(), signedWebhookPayload(t, payload))
	if err != nil {
		t.Fatalf("ParseWebhook error: %v", err)
	}
	if event.Payment != nil {
		t.Fatalf("Payment must be nil for add-card notifications")
	}
	if event.MethodBound == nil {
		t.Fatal("MethodBound missing")
	}
	if event.MethodBound.BindingID != testRequestKey {
		t.Errorf("BindingID: got %q, want %q", event.MethodBound.BindingID, testRequestKey)
	}
	m := event.MethodBound.Method
	if m.ProviderMethodID != testCardID || m.ChargeToken != testRebillID ||
		m.MaskedPan != testMaskedPan || m.ExpDate != "1230" || m.CustomerRef != testCustomerRef {
		t.Errorf("Method: got %+v", m)
	}
}

func TestParseWebhookAddCardLegacyType(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		fieldTerminalKey: testTerminalKey,
		fieldRequestKey:  "request-key-2",
		fieldCardID:      "card-2",
		fieldRebillID:    "rebill-2",
		fieldStatus:      statusCompleted,
		fieldSuccess:     true,
		// Legacy notifications carry the bare "AddCard" type (ADR 0017).
		fieldNotificationType: notificationTypeAddCardLegacy,
	}

	p := newTestProvider("")
	event, err := p.ParseWebhook(context.Background(), signedWebhookPayload(t, payload))
	if err != nil {
		t.Fatalf("ParseWebhook error: %v", err)
	}
	if event.MethodBound == nil || event.MethodBound.BindingID != "request-key-2" {
		t.Fatalf("MethodBound: got %+v", event.MethodBound)
	}
}

// TestParseWebhookAddCardWithoutNotificationType covers the prod fact that
// add-card notifications may omit NotificationType entirely: discrimination
// falls back to RequestKey presence (ADR 0017).
func TestParseWebhookAddCardWithoutNotificationType(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		fieldTerminalKey: testTerminalKey,
		fieldRequestKey:  "request-key-3",
		fieldCardID:      "card-3",
		fieldRebillID:    "rebill-3",
		fieldStatus:      statusCompleted,
		fieldSuccess:     true,
	}

	p := newTestProvider("")
	event, err := p.ParseWebhook(context.Background(), signedWebhookPayload(t, payload))
	if err != nil {
		t.Fatalf("ParseWebhook error: %v", err)
	}
	if event.MethodBound == nil || event.MethodBound.BindingID != "request-key-3" {
		t.Fatalf("MethodBound: got %+v", event.MethodBound)
	}
}

// TestParseWebhookAddCardRejectedMapsToBindingFailed proves a validly signed
// refused binding is a legitimate outcome, not a delivery defect: the parser
// surfaces the binding-failed event so the application closes the session and
// the handler answers 200 (issue #422).
func TestParseWebhookAddCardRejectedMapsToBindingFailed(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		fieldTerminalKey:      testTerminalKey,
		fieldRequestKey:       "request-key-4",
		fieldCardID:           "card-4",
		fieldStatus:           statusRejected,
		fieldSuccess:          false,
		fieldErrorCode:        "6",
		fieldNotificationType: notificationTypeAddCard,
	}

	p := newTestProvider("")
	event, err := p.ParseWebhook(context.Background(), signedWebhookPayload(t, payload))
	if err != nil {
		t.Fatalf("ParseWebhook error: %v", err)
	}
	if event.Payment != nil || event.MethodBound != nil {
		t.Fatalf("event = %+v, want only the binding-failed payload", event)
	}
	if event.MethodBindingFailed == nil {
		t.Fatal("MethodBindingFailed missing")
	}
	if event.MethodBindingFailed.BindingID != "request-key-4" {
		t.Errorf("BindingID: got %q, want %q", event.MethodBindingFailed.BindingID, "request-key-4")
	}
	if event.MethodBindingFailed.ErrorCode != "6" {
		t.Errorf("ErrorCode: got %q, want %q", event.MethodBindingFailed.ErrorCode, "6")
	}
}

// TestParseWebhookAddCardRejectedWithoutNotificationType covers the prod fact
// that add-card notifications may omit NotificationType (ADR 0017): a refused
// binding is still discriminated by RequestKey and maps to the failed event.
func TestParseWebhookAddCardRejectedWithoutNotificationType(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		fieldTerminalKey: testTerminalKey,
		fieldRequestKey:  "request-key-5",
		fieldStatus:      statusRejected,
		fieldSuccess:     false,
	}

	p := newTestProvider("")
	event, err := p.ParseWebhook(context.Background(), signedWebhookPayload(t, payload))
	if err != nil {
		t.Fatalf("ParseWebhook error: %v", err)
	}
	if event.MethodBindingFailed == nil || event.MethodBindingFailed.BindingID != "request-key-5" {
		t.Fatalf("MethodBindingFailed: got %+v", event.MethodBindingFailed)
	}
}

// TestParseWebhookAddCardRejectedForgedTokenStillRejected proves the refused
// outcome never bypasses verification: a REJECTED payload with a forged
// signature stays a delivery defect (issue #422).
func TestParseWebhookAddCardRejectedForgedTokenStillRejected(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		fieldTerminalKey:      testTerminalKey,
		fieldRequestKey:       "request-key-6",
		fieldStatus:           statusRejected,
		fieldSuccess:          false,
		fieldNotificationType: notificationTypeAddCard,
	}
	payload["Token"] = sign(payload, "wrongPassword")
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal webhook: %v", err)
	}

	p := newTestProvider("")
	_, err = p.ParseWebhook(context.Background(), body)
	if err == nil {
		t.Fatal("expected error for forged token")
	}
	if !strings.Contains(err.Error(), "invalid webhook token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestParseWebhookAddCardRejectsNonTerminalStatus keeps the guard for
// notifications without a terminal binding outcome: the application must not
// resolve a session on an intermediate status (issue #422).
func TestParseWebhookAddCardRejectsNonTerminalStatus(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		fieldTerminalKey:      testTerminalKey,
		fieldRequestKey:       "request-key-7",
		fieldStatus:           status3DSChecking,
		fieldSuccess:          false,
		fieldNotificationType: notificationTypeAddCard,
	}

	p := newTestProvider("")
	_, err := p.ParseWebhook(context.Background(), signedWebhookPayload(t, payload))
	if err == nil {
		t.Fatal("expected error for non-terminal binding status")
	}
	if !strings.Contains(err.Error(), "binding outcome not final") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseWebhookUnknownNotificationType(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		fieldTerminalKey:      testTerminalKey,
		fieldNotificationType: "NotificationSomethingElse",
		fieldStatus:           statusConfirmed,
	}

	p := newTestProvider("")
	_, err := p.ParseWebhook(context.Background(), signedWebhookPayload(t, payload))
	if err == nil {
		t.Fatal("expected error for unknown notification type")
	}
	if !strings.Contains(err.Error(), "unknown notification type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseWebhookInvalidOrderID(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		fieldTerminalKey: testTerminalKey,
		fieldOrderID:     "not-a-uuid",
		fieldStatus:      statusConfirmed,
		fieldSuccess:     true,
	}

	p := newTestProvider("")
	_, err := p.ParseWebhook(context.Background(), signedWebhookPayload(t, payload))
	if err == nil {
		t.Fatal("expected error for invalid OrderId")
	}
	if !strings.Contains(err.Error(), "parse OrderId") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyWebhookTokenMissing(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		fieldTerminalKey: testTerminalKey,
		fieldOrderID:     testPaymentUUID,
		fieldStatus:      statusConfirmed,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal webhook: %v", err)
	}

	p := newTestProvider("")
	_, err = p.ParseWebhook(context.Background(), body)
	if err == nil {
		t.Fatalf("expected error for missing token")
	}
	if !strings.Contains(err.Error(), "invalid webhook token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyWebhookTokenInvalid(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		fieldTerminalKey: testTerminalKey,
		fieldOrderID:     testPaymentUUID,
		fieldStatus:      statusConfirmed,
		fieldSuccess:     true,
		fieldPaymentID:   json.Number("12345"),
	}
	payload["Token"] = "invalid"
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal webhook: %v", err)
	}

	p := newTestProvider("")
	_, err = p.ParseWebhook(context.Background(), body)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if !strings.Contains(err.Error(), "invalid webhook token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseWebhookTerminalKeyMismatch(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		fieldTerminalKey: "anotherTerminal",
		fieldOrderID:     testPaymentUUID,
		fieldStatus:      statusConfirmed,
		fieldSuccess:     true,
	}

	p := newTestProvider("")
	_, err := p.ParseWebhook(context.Background(), signedWebhookPayload(t, payload))
	if err == nil {
		t.Fatal("expected error for terminal key mismatch")
	}
	if !strings.Contains(err.Error(), "terminal key mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetAmount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		data    map[string]any
		want    int64
		wantErr bool
	}{
		{"number", map[string]any{fieldAmount: json.Number("10000")}, 10000, false},
		{"float-ish number", map[string]any{fieldAmount: json.Number("10000")}, 10000, false},
		{"string digits", map[string]any{fieldAmount: "10000"}, 10000, false},
		{"missing", map[string]any{}, 0, false},
		{"empty string", map[string]any{fieldAmount: ""}, 0, false},
		{"garbage", map[string]any{fieldAmount: "abc"}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := getAmount(tt.data)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("getAmount error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("getAmount = %d, want %d", got, tt.want)
			}
		})
	}
}
