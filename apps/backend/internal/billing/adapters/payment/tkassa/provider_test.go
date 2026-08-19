package tkassa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

const (
	testTerminalKey = "testTerminalKey"
	testPassword    = "testPassword"
	testAppBaseURL  = "https://app.example"
	testAPIBaseURL  = "https://api.example/v2/"
	// Shared wire fixtures: every provider test talks to the same customer,
	// card, and payment-form shapes.
	testCustomerRef = "customer-1"
	testCardID      = "card-1"
	testRebillID    = "rebill-1"
	testChargeToken = "rebill-token"
	testRequestKey  = "request-key-1"
	testMaskedPan   = "4300********1234"
	testPaymentURL  = "https://securepayments.tinkoff.ru/rest/show/123456"
	testAddCardURL  = "https://securepayments.tinkoff.ru/rest/addcard/abc"
	testPaymentUUID = "11111111-1111-1111-1111-111111111111"
	// Shared expected description for a Pro monthly subscription payment.
	testProMonthDescription = "Оплата подписки Pro (месяц)"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// writeJSON encodes a test-fixture response into w, failing the test when the
// encoder errors: a fixture that fails to encode would silently produce an
// empty body and mislead the test about the provider's behavior.
func writeJSON(t *testing.T, w io.Writer, v any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("encode fixture response: %v", err)
	}
}

// closeResp closes a test response body, surfacing a close failure as a test
// error; a nil resp (error-path RoundTrip results) is skipped.
func closeResp(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp == nil {
		return
	}
	if err := resp.Body.Close(); err != nil {
		t.Errorf("close response body: %v", err)
	}
}

func newTestProvider(serverURL string) *Provider {
	p, err := NewProvider(Config{
		BaseURL:     serverURL + "/v2/",
		TerminalKey: testTerminalKey,
		Password:    testPassword,
		AppBaseURL:  testAppBaseURL,
		Timeout:     5 * time.Second,
	}, discardLogger(), nil)
	if err != nil {
		panic(err)
	}
	return p
}

func testPurpose() application.PaymentPurpose {
	return application.PaymentPurpose{
		Kind:       application.PaymentPurposeSubscription,
		TariffName: domain.TariffPro,
		Period:     domain.PeriodMonth,
	}
}

// captureRequest reads the request body, verifies the T-Kassa token, and
// returns both the raw JSON and the decoded map. Fields in exclude are removed
// before token verification to match the server-side schema (e.g. AddCard
// ignores the URL extras the adapter appends outside the schema).
func captureRequest(t *testing.T, r *http.Request, exclude ...string) (body []byte, captured map[string]any) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}

	token, ok := data["Token"].(string)
	if !ok || token == "" {
		t.Fatalf("request missing or empty Token field")
	}

	signData := maps.Clone(data)
	for _, k := range exclude {
		delete(signData, k)
	}
	if expected := sign(signData, testPassword); token != expected {
		t.Fatalf("token mismatch: got %q, want %q", token, expected)
	}

	return body, data
}

func verifyRequestToken(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	_, data := captureRequest(t, r)
	return data
}

// strictDecodeSpecRequest decodes a captured request body into a generated spec
// request struct with DisallowUnknownFields, so any field the adapter sends
// that is not part of the spec schema fails the test. Nested DATA is the
// spec.InitRequest_DATA union with custom unmarshaling, so strictness applies
// to the top-level object only — DATA contents are asserted separately at map
// level.
func strictDecodeSpecRequest(t *testing.T, body []byte, dst any) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		t.Fatalf("strict-decode request into %T: %v", dst, err)
	}
}

// addCardExtraFieldsAllowlist lists the AddCard request fields the adapter
// appends outside the spec schema: the T-Kassa AddCard schema (see
// Provider.BindPaymentMethod) does not contain them, and the server ignores
// them for token verification. They are asserted at map level and stripped
// before the strict spec-schema decode instead of weakening strictness for
// everything.
var addCardExtraFieldsAllowlist = []string{
	fieldRedirectURL, fieldFailRedirectURL,
	fieldSuccessAddCardURL, fieldFailAddCardURL, fieldNotificationURL,
}

func TestNewProviderRequiresBaseURL(t *testing.T) {
	_, err := NewProvider(Config{
		TerminalKey: testTerminalKey,
		Password:    testPassword,
		AppBaseURL:  testAppBaseURL,
	}, discardLogger(), nil)
	if err == nil {
		t.Fatal("expected error for empty base URL")
	}
	if !strings.Contains(err.Error(), "base URL is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewProviderRequiresCredentialsAndAppBaseURL(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{"terminal key", Config{BaseURL: testAPIBaseURL, Password: "p", AppBaseURL: testAppBaseURL}, "terminal key is required"},
		{"password", Config{BaseURL: testAPIBaseURL, TerminalKey: "t", AppBaseURL: testAppBaseURL}, "password is required"},
		{"app base url", Config{BaseURL: testAPIBaseURL, TerminalKey: "t", Password: "p"}, "app base URL is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProvider(tt.cfg, discardLogger(), nil)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNewProviderTrimsTrailingSlash(t *testing.T) {
	cfg := Config{
		BaseURL:     "https://api.example/v2",
		TerminalKey: testTerminalKey,
		Password:    testPassword,
		AppBaseURL:  testAppBaseURL,
	}
	p, err := NewProvider(cfg, discardLogger(), nil)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	// A base URL without the trailing slash must be normalized to exactly one,
	// so method paths concatenate cleanly.
	if got, want := p.baseURL, testAPIBaseURL; got != want {
		t.Fatalf("baseURL: got %q, want %q", got, want)
	}
}

func TestProviderInitPayment(t *testing.T) {
	paymentID := uuid.MustParse(testPaymentUUID)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/"+methodInit {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		data := verifyRequestToken(t, r)

		if got, want := data[fieldTerminalKey], any(testTerminalKey); got != want {
			t.Fatalf("TerminalKey: got %v, want %v", got, want)
		}
		if got, want := data[fieldOrderID], any(paymentID.String()); got != want {
			t.Fatalf("OrderId: got %v, want %v", got, want)
		}
		if got, want := data[fieldAmount], any(float64(10000)); got != want {
			t.Fatalf("Amount: got %v, want %v", got, want)
		}
		if got, want := data["Recurrent"], any("Y"); got != want {
			t.Fatalf("Recurrent: got %v, want %v", got, want)
		}
		if got, want := data["PayType"], any("O"); got != want {
			t.Fatalf("PayType: got %v, want %v", got, want)
		}
		if got, want := data["CustomerKey"], any(testCustomerRef); got != want {
			t.Fatalf("CustomerKey: got %v, want %v", got, want)
		}
		// The adapter builds every callback URL from the configured app base
		// URL (issue #248): the application layer no longer passes them.
		if got, want := data["NotificationURL"], any(testAppBaseURL+"/webhooks/payment/tkassa"); got != want {
			t.Fatalf("NotificationURL: got %v, want %v", got, want)
		}
		if got, want := data["SuccessURL"], any(testAppBaseURL+"/subscription/payments/"+paymentID.String()+"/success"); got != want {
			t.Fatalf("SuccessURL: got %v, want %v", got, want)
		}
		if got, want := data["FailURL"], any(testAppBaseURL+"/subscription/payments/"+paymentID.String()+"/fail"); got != want {
			t.Fatalf("FailURL: got %v, want %v", got, want)
		}
		if got, want := data["Description"], any(testProMonthDescription); got != want {
			t.Fatalf("Description: got %v, want %v", got, want)
		}
		dataObj, ok := data["DATA"].(map[string]any)
		if !ok {
			t.Fatalf("DATA missing or not object")
		}
		if got, want := dataObj["OperationInitiatorType"], any("1"); got != want {
			t.Fatalf("OperationInitiatorType: got %v, want %v", got, want)
		}
		if _, ok := data["Token"]; !ok {
			t.Fatalf("Token missing from DATA exclusion test")
		}

		writeJSON(t, w, initResponse{
			baseResponse: baseResponse{Success: true, Status: statusNew},
			PaymentID:    "123456",
			PaymentURL:   testPaymentURL,
			OrderID:      paymentID.String(),
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	result, err := p.InitPayment(context.Background(), application.InitPaymentRequest{
		PaymentID:     paymentID,
		AmountKopecks: 10000,
		Period:        domain.PeriodMonth,
		CustomerRef:   testCustomerRef,
		Purpose:       testPurpose(),
		SaveMethod:    true,
		Initiator:     application.InitiatorCustomer,
	})
	if err != nil {
		t.Fatalf("InitPayment failed: %v", err)
	}
	if result.ProviderPaymentID != "123456" {
		t.Fatalf("ProviderPaymentID: got %q, want %q", result.ProviderPaymentID, "123456")
	}
	if result.PaymentURL != testPaymentURL {
		t.Fatalf("PaymentURL: got %q", result.PaymentURL)
	}
	if result.Status != domain.PaymentStatusPending {
		t.Fatalf("Status: got %v, want %v", result.Status, domain.PaymentStatusPending)
	}
	if result.SavedMethod != nil {
		t.Fatalf("SavedMethod should be nil — the token arrives via webhook, got %v", *result.SavedMethod)
	}
}

// TestProviderInitPaymentInitiatorRequired pins the port contract: the
// initiator is a mandatory explicit parameter and must not default to CIT
// silently (issue #246 §6).
func TestProviderInitPaymentInitiatorRequired(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		writeJSON(t, w, initResponse{baseResponse: baseResponse{Success: true, Status: statusNew}, PaymentID: "1"})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	_, err := p.InitPayment(context.Background(), application.InitPaymentRequest{
		PaymentID:     uuid.Must(uuid.NewV7()),
		AmountKopecks: 100,
		CustomerRef:   "ck",
	})
	if err == nil {
		t.Fatal("expected error for empty initiator")
	}
	if !strings.Contains(err.Error(), "unknown operation initiator") {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != 0 {
		t.Fatalf("no request may be sent for an invalid initiator, sent %d", requests)
	}
}

func TestProviderInitPaymentMerchantInitiated(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = verifyRequestToken(t, r)
		writeJSON(t, w, initResponse{
			baseResponse: baseResponse{Success: true, Status: statusNew},
			PaymentID:    "123",
			PaymentURL:   "https://pay.example.com/123",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	paymentID := uuid.Must(uuid.NewV7())
	_, err := p.InitPayment(context.Background(), application.InitPaymentRequest{
		PaymentID:     paymentID,
		AmountKopecks: 100,
		Period:        domain.PeriodMonth,
		CustomerRef:   "ck",
		Purpose: application.PaymentPurpose{
			Kind:       application.PaymentPurposeRenewal,
			TariffName: domain.TariffBusiness,
			Period:     domain.PeriodYear,
		},
		SaveMethod: false,
		Initiator:  application.InitiatorMerchant,
	})
	if err != nil {
		t.Fatalf("InitPayment failed: %v", err)
	}

	if _, ok := captured["Recurrent"]; ok {
		t.Fatal("expected Recurrent field omitted for merchant-initiated payment")
	}
	dataObj, ok := captured["DATA"].(map[string]any)
	if !ok {
		t.Fatalf("DATA missing or not an object")
	}
	if got, want := dataObj["OperationInitiatorType"], any("R"); got != want {
		t.Fatalf("OperationInitiatorType: got %v, want %v", got, want)
	}
	if got, want := captured["Description"], any("Продление подписки Business (год)"); got != want {
		t.Fatalf("Description: got %v, want %v", got, want)
	}
}

func TestProviderInitPaymentFormDeadline(t *testing.T) {
	newServer := func(captured *map[string]any) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*captured = verifyRequestToken(t, r)
			writeJSON(t, w, initResponse{
				baseResponse: baseResponse{Success: true, Status: statusNew},
				PaymentID:    "1",
				PaymentURL:   "https://pay",
			})
		}))
	}

	t.Run("set normalizes to utc", func(t *testing.T) {
		var captured map[string]any
		server := newServer(&captured)
		defer server.Close()

		p := newTestProvider(server.URL)
		due := time.Date(2026, 7, 13, 15, 0, 0, 0, time.FixedZone("MSK", 3*60*60))
		_, err := p.InitPayment(context.Background(), application.InitPaymentRequest{
			PaymentID:     uuid.Must(uuid.NewV7()),
			AmountKopecks: 100,
			CustomerRef:   "ck",
			Purpose:       testPurpose(),
			Initiator:     application.InitiatorCustomer,
			FormDeadline:  due,
		})
		if err != nil {
			t.Fatalf("InitPayment failed: %v", err)
		}

		got, ok := captured["RedirectDueDate"].(string)
		if !ok {
			t.Fatalf("RedirectDueDate missing or not a string")
		}
		if want := "2026-07-13T12:00:00Z"; got != want {
			t.Fatalf("RedirectDueDate: got %q, want %q", got, want)
		}
	})

	t.Run("zero omits field", func(t *testing.T) {
		var captured map[string]any
		server := newServer(&captured)
		defer server.Close()

		p := newTestProvider(server.URL)
		_, err := p.InitPayment(context.Background(), application.InitPaymentRequest{
			PaymentID:     uuid.Must(uuid.NewV7()),
			AmountKopecks: 100,
			CustomerRef:   "ck",
			Purpose:       testPurpose(),
			Initiator:     application.InitiatorCustomer,
		})
		if err != nil {
			t.Fatalf("InitPayment failed: %v", err)
		}

		if v, ok := captured["RedirectDueDate"]; ok {
			t.Fatalf("RedirectDueDate must be omitted when zero, got %v", v)
		}
	})
}

func TestProviderInitPaymentError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = verifyRequestToken(t, r)
		writeJSON(t, w, initResponse{
			baseResponse: baseResponse{
				Success:   false,
				ErrorCode: "7",
				Message:   "Invalid amount",
			},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	_, err := p.InitPayment(context.Background(), application.InitPaymentRequest{
		PaymentID:     uuid.Must(uuid.NewV7()),
		AmountKopecks: 100,
		CustomerRef:   "ck",
		Purpose:       testPurpose(),
		Initiator:     application.InitiatorCustomer,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerErr.ErrorCode != "7" {
		t.Fatalf("ErrorCode: got %q, want %q", providerErr.ErrorCode, "7")
	}
}

func TestProviderChargePayment(t *testing.T) {
	paymentID := uuid.Must(uuid.NewV7())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/"+methodCharge {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		data := verifyRequestToken(t, r)
		if got, want := data[fieldPaymentID], any("123"); got != want {
			t.Fatalf("PaymentId: got %v, want %v", got, want)
		}
		if got, want := data[fieldRebillID], any(testChargeToken); got != want {
			t.Fatalf("RebillId: got %v, want %v", got, want)
		}
		// The charge amount comes from the original Init call; the Charge body
		// must not carry Amount.
		if _, ok := data[fieldAmount]; ok {
			t.Fatalf("Charge body must not contain Amount, got %v", data[fieldAmount])
		}

		writeJSON(t, w, chargeResponse{
			baseResponse: baseResponse{Success: true, Status: statusConfirmed, ErrorCode: "0"},
			PaymentID:    "123",
			OrderID:      paymentID.String(),
			Amount:       10000,
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	result, err := p.ChargePayment(context.Background(), application.ChargeRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: "123",
		AmountKopecks:     10000,
		ChargeToken:       testChargeToken,
	})
	if err != nil {
		t.Fatalf("ChargePayment failed: %v", err)
	}
	if result.ProviderPaymentID != "123" {
		t.Fatalf("ProviderPaymentID: got %q, want %q", result.ProviderPaymentID, "123")
	}
	if result.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("Status: got %v, want %v", result.Status, domain.PaymentStatusSucceeded)
	}
	if result.ErrorCode != "" {
		t.Fatalf("ErrorCode: got %q, want empty on success", result.ErrorCode)
	}
}

func TestProviderChargePaymentRejectedCarriesErrorCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, chargeResponse{
			baseResponse: baseResponse{Success: true, Status: statusRejected, ErrorCode: "103"},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	result, err := p.ChargePayment(context.Background(), application.ChargeRequest{
		PaymentID:         uuid.Must(uuid.NewV7()),
		ProviderPaymentID: "123",
		AmountKopecks:     10000,
		ChargeToken:       testChargeToken,
	})
	if err != nil {
		t.Fatalf("ChargePayment failed: %v", err)
	}
	if result.Status != domain.PaymentStatusFailed {
		t.Fatalf("Status: got %v, want %v", result.Status, domain.PaymentStatusFailed)
	}
	if result.ErrorCode != "103" {
		t.Fatalf("ErrorCode: got %q, want %q", result.ErrorCode, "103")
	}
}

func TestProviderPaymentStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/"+methodGetState {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		data := verifyRequestToken(t, r)
		if got, want := data[fieldPaymentID], any("999"); got != want {
			t.Fatalf("PaymentId: got %v, want %v", got, want)
		}

		writeJSON(t, w, getStateResponse{
			baseResponse: baseResponse{Success: true, Status: statusAuthorized, ErrorCode: "0"},
			PaymentID:    "999",
			Amount:       1230,
			RebillID:     "rebill-42",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	paymentID := uuid.Must(uuid.NewV7())
	status, err := p.PaymentStatus(context.Background(), paymentID, "999")
	if err != nil {
		t.Fatalf("PaymentStatus failed: %v", err)
	}
	if status.Status != domain.PaymentStatusPending {
		t.Fatalf("Status: got %v, want %v", status.Status, domain.PaymentStatusPending)
	}
	// RebillId recovery: the token of a save-method payment is re-read from
	// GetState when its delivery notification was lost.
	if status.ChargeToken != "rebill-42" {
		t.Fatalf("ChargeToken: got %q, want %q", status.ChargeToken, "rebill-42")
	}
	if status.ErrorCode != "" {
		t.Fatalf("ErrorCode: got %q, want empty on success", status.ErrorCode)
	}
}

func TestProviderRefundPayment(t *testing.T) {
	paymentID := uuid.Must(uuid.NewV7())
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/"+methodCancel {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		captured = verifyRequestToken(t, r)
		writeJSON(t, w, cancelResponse{
			baseResponse:   baseResponse{Success: true, Status: statusRefunded, ErrorCode: "0"},
			OrderID:        paymentID.String(),
			PaymentID:      "777",
			OriginalAmount: 10000,
			NewAmount:      0,
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	result, err := p.RefundPayment(context.Background(), application.RefundRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: "777",
		AmountKopecks:     10000,
	})
	if err != nil {
		t.Fatalf("RefundPayment failed: %v", err)
	}
	if result.Status != domain.PaymentStatusRefunded {
		t.Fatalf("Status: got %v, want %v", result.Status, domain.PaymentStatusRefunded)
	}
	if result.RefundedAmountKopecks != 10000 {
		t.Fatalf("RefundedAmountKopecks: got %d, want %d", result.RefundedAmountKopecks, 10000)
	}
	// ExternalRequestId carries the internal payment id so network duplicates
	// are deduplicated by the provider (issue #246 §2.3).
	if got, want := captured["ExternalRequestId"], any(paymentID.String()); got != want {
		t.Fatalf("ExternalRequestId: got %v, want %v", got, want)
	}
}

func TestProviderRefundPaymentReversedPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, cancelResponse{
			baseResponse:   baseResponse{Success: true, Status: statusReversed, ErrorCode: "0"},
			OriginalAmount: 5000,
			NewAmount:      0,
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	result, err := p.RefundPayment(context.Background(), application.RefundRequest{
		PaymentID:         uuid.Must(uuid.NewV7()),
		ProviderPaymentID: "778",
		AmountKopecks:     5000,
	})
	if err != nil {
		t.Fatalf("RefundPayment failed: %v", err)
	}
	if result.Status != domain.PaymentStatusRefunded {
		t.Fatalf("Status: got %v, want %v", result.Status, domain.PaymentStatusRefunded)
	}
	if result.RefundedAmountKopecks != 5000 {
		t.Fatalf("RefundedAmountKopecks: got %d, want %d", result.RefundedAmountKopecks, 5000)
	}
}

func TestProviderRefundPaymentError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, cancelResponse{
			baseResponse: baseResponse{
				Success:   false,
				ErrorCode: "255",
				Message:   "Платеж не найден",
			},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	_, err := p.RefundPayment(context.Background(), application.RefundRequest{
		PaymentID:         uuid.Must(uuid.NewV7()),
		ProviderPaymentID: "404",
		AmountKopecks:     100,
	})
	if !errors.Is(err, application.ErrProviderPaymentNotFound) {
		t.Fatalf("expected ErrProviderPaymentNotFound, got %v", err)
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.ErrorCode != "255" {
		t.Fatalf("expected ProviderError 255 in chain, got %v", err)
	}
}

func TestProviderBindPaymentMethod(t *testing.T) {
	customerRef := testCustomerRef
	var addCardCaptured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/" + methodAddCustomer:
			data := verifyRequestToken(t, r)
			if got, want := data["CustomerKey"], any(customerRef); got != want {
				t.Fatalf("AddCustomer CustomerKey: got %v, want %v", got, want)
			}
			writeJSON(t, w, addCustomerResponse{
				baseResponse: baseResponse{Success: true, ErrorCode: "0"},
				CustomerKey:  customerRef,
			})
		case "/v2/" + methodAddCard:
			// The extras sit outside the schema and outside the token: verify
			// the token over the schema fields only, then assert the extras.
			addCardCaptured = verifyRequestTokenExcluding(t, r, addCardExtraFieldsAllowlist...)
			writeJSON(t, w, addCardResponse{
				baseResponse: baseResponse{Success: true, ErrorCode: "0"},
				PaymentURL:   testAddCardURL,
				RequestKey:   testRequestKey,
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	result, err := p.BindPaymentMethod(context.Background(), application.BindMethodRequest{
		CustomerRef: customerRef,
	})
	if err != nil {
		t.Fatalf("BindPaymentMethod failed: %v", err)
	}
	if result.FormURL != testAddCardURL {
		t.Fatalf("FormURL: got %q", result.FormURL)
	}
	if result.BindingID != testRequestKey {
		t.Fatalf("BindingID: got %q, want %q", result.BindingID, testRequestKey)
	}

	if got, want := addCardCaptured["CheckType"], any("3DSHOLD"); got != want {
		t.Fatalf("CheckType: got %v, want %v", got, want)
	}
	wantExtras := map[string]string{
		fieldRedirectURL:       testAppBaseURL + bindingReturnSuccessPath,
		fieldFailRedirectURL:   testAppBaseURL + bindingReturnFailPath,
		fieldSuccessAddCardURL: testAppBaseURL + bindingReturnSuccessPath,
		fieldFailAddCardURL:    testAppBaseURL + bindingReturnFailPath,
		fieldNotificationURL:   testAppBaseURL + notificationPath,
	}
	for field, want := range wantExtras {
		if got := addCardCaptured[field]; got != want {
			t.Fatalf("AddCard extra %s: got %v, want %v", field, got, want)
		}
	}
}

// verifyRequestTokenExcluding models the T-Kassa server token check: the token
// is recomputed over the request body without the excluded keys. The server
// ignores fields outside the method schema — e.g. the AddCard URL extras
// (prod incident, error 204) — so a server-faithful check must exclude them
// instead of mirroring the client's sign over the whole body.
func verifyRequestTokenExcluding(t *testing.T, r *http.Request, exclude ...string) map[string]any {
	t.Helper()
	_, data := captureRequest(t, r, exclude...)
	return data
}

func TestProviderBindPaymentMethodCustomerAlreadyExistsProceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/" + methodAddCustomer:
			_ = verifyRequestToken(t, r)
			writeJSON(t, w, addCustomerResponse{
				baseResponse: baseResponse{Success: false, ErrorCode: "7", Message: "Покупатель уже существует"},
			})
		case "/v2/" + methodAddCard:
			_ = verifyRequestTokenExcluding(t, r, addCardExtraFieldsAllowlist...)
			writeJSON(t, w, addCardResponse{
				baseResponse: baseResponse{Success: true, ErrorCode: "0"},
				PaymentURL:   "https://pay",
				RequestKey:   "rk",
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	result, err := p.BindPaymentMethod(context.Background(), application.BindMethodRequest{CustomerRef: testCustomerRef})
	if err != nil {
		t.Fatalf("BindPaymentMethod should proceed after customer-exists, got %v", err)
	}
	if result.BindingID != "rk" {
		t.Fatalf("BindingID: got %q, want %q", result.BindingID, "rk")
	}
}

func TestProviderBindPaymentMethodOtherAPIErrorPropagated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/"+methodAddCustomer {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = verifyRequestToken(t, r)
		writeJSON(t, w, addCustomerResponse{
			baseResponse: baseResponse{Success: false, ErrorCode: "255", Message: "other"},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	_, err := p.BindPaymentMethod(context.Background(), application.BindMethodRequest{CustomerRef: testCustomerRef})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "add customer failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderPaymentMethodBinding(t *testing.T) {
	t.Run("completed carries method", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v2/GetAddCardState" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			data := verifyRequestToken(t, r)
			if got, want := data[fieldRequestKey], any("rk-1"); got != want {
				t.Fatalf("RequestKey: got %v, want %v", got, want)
			}
			writeJSON(t, w, getAddCardStateResponse{
				baseResponse: baseResponse{Success: true, ErrorCode: "0", Status: statusCompleted},
				CardID:       testCardID,
				RebillID:     testRebillID,
				CustomerKey:  testCustomerRef,
				RequestKey:   "rk-1",
			})
		}))
		defer server.Close()

		p := newTestProvider(server.URL)
		state, err := p.PaymentMethodBinding(context.Background(), "rk-1")
		if err != nil {
			t.Fatalf("PaymentMethodBinding failed: %v", err)
		}
		if state.Status != application.MethodBindingCompleted {
			t.Fatalf("Status: got %q, want %q", state.Status, application.MethodBindingCompleted)
		}
		if state.Method == nil {
			t.Fatal("Method must be set on completed binding")
		}
		if state.Method.ProviderMethodID != testCardID || state.Method.ChargeToken != testRebillID {
			t.Fatalf("Method: got %+v", *state.Method)
		}
	})

	t.Run("intermediate states are pending", func(t *testing.T) {
		for _, status := range []string{
			statusNew, statusFormShowed, status3DSChecking, status3DSChecked, statusAuthorizing, statusAuthorized,
		} {
			t.Run(status, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					writeJSON(t, w, getAddCardStateResponse{
						baseResponse: baseResponse{Success: true, Status: status},
						RequestKey:   "rk-2",
					})
				}))
				defer server.Close()

				p := newTestProvider(server.URL)
				state, err := p.PaymentMethodBinding(context.Background(), "rk-2")
				if err != nil {
					t.Fatalf("PaymentMethodBinding failed: %v", err)
				}
				if state.Status != application.MethodBindingPending {
					t.Fatalf("Status: got %q, want pending", state.Status)
				}
				if state.Method != nil {
					t.Fatalf("Method must be nil while pending, got %+v", *state.Method)
				}
			})
		}
	})

	t.Run("rejected is failed with error code", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, getAddCardStateResponse{
				baseResponse: baseResponse{Success: true, ErrorCode: "7", Status: statusRejected},
				RequestKey:   "rk-3",
			})
		}))
		defer server.Close()

		p := newTestProvider(server.URL)
		state, err := p.PaymentMethodBinding(context.Background(), "rk-3")
		if err != nil {
			t.Fatalf("PaymentMethodBinding failed: %v", err)
		}
		if state.Status != application.MethodBindingFailed {
			t.Fatalf("Status: got %q, want %q", state.Status, application.MethodBindingFailed)
		}
		if state.ErrorCode != "7" {
			t.Fatalf("ErrorCode: got %q, want %q", state.ErrorCode, "7")
		}
	})

	t.Run("unknown status errors", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, getAddCardStateResponse{
				baseResponse: baseResponse{Success: true, Status: "TOTALLY_NEW_STATUS"},
				RequestKey:   "rk-4",
			})
		}))
		defer server.Close()

		p := newTestProvider(server.URL)
		_, err := p.PaymentMethodBinding(context.Background(), "rk-4")
		if err == nil {
			t.Fatal("expected an error for an unknown add card status")
		}
		if !strings.Contains(err.Error(), "unknown add card status") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestProviderRemovePaymentMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/RemoveCard" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		data := verifyRequestToken(t, r)
		if got, want := data[fieldCardID], any(testCardID); got != want {
			t.Fatalf("CardId: got %v, want %v", got, want)
		}
		writeJSON(t, w, removeCardResponse{
			baseResponse: baseResponse{Success: true, ErrorCode: "0"},
			CardID:       testCardID,
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	if err := p.RemovePaymentMethod(context.Background(), testCustomerRef, testCardID); err != nil {
		t.Fatalf("RemovePaymentMethod failed: %v", err)
	}
}

func TestProviderRemovePaymentMethodNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, removeCardResponse{
			baseResponse: baseResponse{Success: false, ErrorCode: "107", Message: "Неверно введен CardId"},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	err := p.RemovePaymentMethod(context.Background(), testCustomerRef, testCardID)
	if !errors.Is(err, application.ErrProviderMethodNotFound) {
		t.Fatalf("expected ErrProviderMethodNotFound, got %v", err)
	}
}

func TestProviderListPaymentMethods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/GetCardList" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		data := verifyRequestToken(t, r)
		if got, want := data["CustomerKey"], any(testCustomerRef); got != want {
			t.Fatalf("CustomerKey: got %v, want %v", got, want)
		}
		writeJSON(t, w, []cardListItem{
			{CardID: "card-a", Pan: testMaskedPan, ExpDate: "1230", Status: "A", RebillID: "rebill-a"},
			{CardID: "card-d", Pan: "5500********5678", ExpDate: "1130", Status: "D", RebillID: "rebill-d"},
			{CardID: "card-i", Pan: "2200********9012", ExpDate: "1029", Status: "I", RebillID: "rebill-i"},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	methods, err := p.ListPaymentMethods(context.Background(), testCustomerRef)
	if err != nil {
		t.Fatalf("ListPaymentMethods failed: %v", err)
	}
	// Only chargeable (active) cards survive the neutral port.
	if len(methods) != 1 {
		t.Fatalf("methods: got %d, want 1 (only active cards)", len(methods))
	}
	got := methods[0]
	if got.ProviderMethodID != "card-a" || got.ChargeToken != "rebill-a" || got.MaskedPan != testMaskedPan ||
		got.ExpDate != "1230" || got.CustomerRef != testCustomerRef {
		t.Fatalf("method: got %+v", got)
	}
}

func TestProviderListPaymentMethodsCustomerNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, baseResponse{
			Success:   false,
			ErrorCode: "7",
			Message:   "Покупатель не найден",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	_, err := p.ListPaymentMethods(context.Background(), testCustomerRef)
	if !errors.Is(err, application.ErrProviderCustomerNotFound) {
		t.Fatalf("expected ErrProviderCustomerNotFound, got %v", err)
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.ErrorCode != "7" {
		t.Fatalf("expected ProviderError 7 in chain, got %v", err)
	}
}

func TestProviderListPaymentMethodsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, []cardListItem{})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	methods, err := p.ListPaymentMethods(context.Background(), testCustomerRef)
	if err != nil {
		t.Fatalf("ListPaymentMethods failed: %v", err)
	}
	if len(methods) != 0 {
		t.Fatalf("methods: got %d, want 0", len(methods))
	}
}

func TestProviderWebhookAck(t *testing.T) {
	p := newTestProvider("")
	if got, want := string(p.WebhookAck()), "OK"; got != want {
		t.Fatalf("WebhookAck: got %q, want %q", got, want)
	}
}

func TestProviderHTTPErrorIncludesBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("upstream exploded")); err != nil {
			t.Errorf("write fixture body: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	_, err := p.ChargePayment(context.Background(), application.ChargeRequest{
		PaymentID:         uuid.Must(uuid.NewV7()),
		ProviderPaymentID: "1",
		AmountKopecks:     100,
		ChargeToken:       "t",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTP 500") || !strings.Contains(err.Error(), "upstream exploded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderName(t *testing.T) {
	p := newTestProvider("")
	if got, want := p.Name(), domain.PaymentProvider("tkassa"); got != want {
		t.Fatalf("Name: got %q, want %q", got, want)
	}
}
