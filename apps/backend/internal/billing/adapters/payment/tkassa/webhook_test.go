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
	paymentID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	payload := map[string]any{
		"TerminalKey": testTerminalKey,
		"OrderId":     paymentID.String(),
		"Status":      "CONFIRMED",
		"Success":     true,
		"PaymentId":   json.Number("12345"),
		"Amount":      json.Number("10000"),
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
	paymentID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	payload := map[string]any{
		"TerminalKey": testTerminalKey,
		"OrderId":     paymentID.String(),
		"Status":      "AUTHORIZED",
		"Success":     true,
		"PaymentId":   json.Number("777"),
		"Amount":      json.Number("500"),
		"RebillId":    "rebill-777",
		"CardId":      "card-777",
		"Pan":         "4300********1234",
		"ExpDate":     "1230",
		"CustomerKey": "customer-1",
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
		m.MaskedPan != "4300********1234" || m.ExpDate != "1230" || m.CustomerRef != "customer-1" {
		t.Fatalf("SavedMethod: got %+v", *m)
	}
}

func TestParseWebhookFailedPaymentCarriesErrorCode(t *testing.T) {
	paymentID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	payload := map[string]any{
		"TerminalKey": testTerminalKey,
		"OrderId":     paymentID.String(),
		"Status":      "REJECTED",
		"Success":     false,
		"PaymentId":   json.Number("888"),
		"ErrorCode":   "103",
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
			payload := map[string]any{
				"TerminalKey": testTerminalKey,
				"OrderId":     "22222222-2222-2222-2222-222222222222",
				"PaymentId":   json.Number("12345"),
				"Status":      tt.status,
				"Amount":      json.Number(strconv.FormatInt(tt.amount, 10)),
				"Success":     true,
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
	payload := map[string]any{
		"TerminalKey":      testTerminalKey,
		"RequestKey":       "request-key-1",
		"CustomerKey":      "customer-1",
		"CardId":           "card-1",
		"RebillId":         "rebill-1",
		"Pan":              "4300********1234",
		"ExpDate":          "1230",
		"Status":           "COMPLETED",
		"Success":          true,
		"NotificationType": "NotificationAddCard",
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
	if event.MethodBound.BindingID != "request-key-1" {
		t.Errorf("BindingID: got %q, want %q", event.MethodBound.BindingID, "request-key-1")
	}
	m := event.MethodBound.Method
	if m.ProviderMethodID != "card-1" || m.ChargeToken != "rebill-1" ||
		m.MaskedPan != "4300********1234" || m.ExpDate != "1230" || m.CustomerRef != "customer-1" {
		t.Errorf("Method: got %+v", m)
	}
}

func TestParseWebhookAddCardLegacyType(t *testing.T) {
	payload := map[string]any{
		"TerminalKey": testTerminalKey,
		"RequestKey":  "request-key-2",
		"CardId":      "card-2",
		"RebillId":    "rebill-2",
		"Status":      "COMPLETED",
		"Success":     true,
		// Legacy notifications carry the bare "AddCard" type (ADR 0017).
		"NotificationType": "AddCard",
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
	payload := map[string]any{
		"TerminalKey": testTerminalKey,
		"RequestKey":  "request-key-3",
		"CardId":      "card-3",
		"RebillId":    "rebill-3",
		"Status":      "COMPLETED",
		"Success":     true,
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

func TestParseWebhookAddCardRejectsNonSuccess(t *testing.T) {
	payload := map[string]any{
		"TerminalKey":      testTerminalKey,
		"RequestKey":       "request-key-4",
		"CardId":           "card-4",
		"Status":           "REJECTED",
		"Success":          false,
		"NotificationType": "NotificationAddCard",
	}

	p := newTestProvider("")
	_, err := p.ParseWebhook(context.Background(), signedWebhookPayload(t, payload))
	if err == nil {
		t.Fatal("expected error for unsuccessful binding")
	}
	if !strings.Contains(err.Error(), "binding not successful") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseWebhookUnknownNotificationType(t *testing.T) {
	payload := map[string]any{
		"TerminalKey":      testTerminalKey,
		"NotificationType": "NotificationSomethingElse",
		"Status":           "CONFIRMED",
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
	payload := map[string]any{
		"TerminalKey": testTerminalKey,
		"OrderId":     "not-a-uuid",
		"Status":      "CONFIRMED",
		"Success":     true,
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
	payload := map[string]any{
		"TerminalKey": testTerminalKey,
		"OrderId":     "11111111-1111-1111-1111-111111111111",
		"Status":      "CONFIRMED",
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
	payload := map[string]any{
		"TerminalKey": testTerminalKey,
		"OrderId":     "11111111-1111-1111-1111-111111111111",
		"Status":      "CONFIRMED",
		"Success":     true,
		"PaymentId":   json.Number("12345"),
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
	payload := map[string]any{
		"TerminalKey": "anotherTerminal",
		"OrderId":     "11111111-1111-1111-1111-111111111111",
		"Status":      "CONFIRMED",
		"Success":     true,
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
	tests := []struct {
		name    string
		data    map[string]any
		want    int64
		wantErr bool
	}{
		{"number", map[string]any{"Amount": json.Number("10000")}, 10000, false},
		{"float-ish number", map[string]any{"Amount": json.Number("10000")}, 10000, false},
		{"string digits", map[string]any{"Amount": "10000"}, 10000, false},
		{"missing", map[string]any{}, 0, false},
		{"empty string", map[string]any{"Amount": ""}, 0, false},
		{"garbage", map[string]any{"Amount": "abc"}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
