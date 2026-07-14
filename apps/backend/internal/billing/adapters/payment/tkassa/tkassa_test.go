package tkassa

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/tkassa/spec"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

const (
	testTerminalKey = "testTerminalKey"
	testPassword    = "testPassword"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func newTestProvider(serverURL string) *Provider {
	return NewProvider(serverURL, testTerminalKey, testPassword, 5*time.Second, 0, 0, 0, discardLogger(), nil)
}

func verifyRequestToken(t *testing.T, r *http.Request, password string) map[string]any {
	t.Helper()
	return verifyRequestTokenExcluding(t, r, password)
}

// verifyRequestTokenExcluding models the T-Kassa server token check: the token
// is recomputed over the request body without the excluded keys. The server
// ignores fields outside the method schema — e.g. RedirectUrl/FailRedirectUrl
// for AddCard (prod incident, error 204) — so a server-faithful check must
// exclude them instead of mirroring the client's sign over the whole body.
func verifyRequestTokenExcluding(t *testing.T, r *http.Request, password string, exclude ...string) map[string]any {
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
	if !ok {
		t.Fatalf("request missing Token field")
	}
	signData := maps.Clone(data)
	for _, k := range exclude {
		delete(signData, k)
	}
	expected := sign(signData, password)
	if token != expected {
		t.Fatalf("token mismatch: got %q, want %q", token, expected)
	}
	return data
}

func TestSign(t *testing.T) {
	data := map[string]any{
		"TerminalKey": "TinkoffBankTest",
		"Amount":      int64(1000),
		"OrderId":     "order-123",
		"DATA": map[string]string{
			"OperationInitiatorType": string(spec.CommonOperationInitiatorTypeN1),
		},
		"Receipt": map[string]any{
			"Items": []any{"item1"},
		},
		"Description": nil,
	}

	got := sign(data, testPassword)

	// Build expected concatenation manually in sorted key order:
	// Amount, OrderId, Password, TerminalKey.
	wantConcat := "1000" + "order-123" + testPassword + "TinkoffBankTest"
	hash := sha256.Sum256([]byte(wantConcat))
	want := hex.EncodeToString(hash[:])
	if got != want {
		t.Fatalf("sign mismatch: got %q, want %q", got, want)
	}
}

func TestSignSkipsNullBlankAndNested(t *testing.T) {
	data := map[string]any{
		"TerminalKey": "Term",
		"Amount":      int64(1),
		"OrderId":     "o",
		"NullValue":   nil,
		"Blank":       "",
		"DATA":        map[string]string{"k": "v"},
		"Receipt":     []any{1, 2},
	}

	got := sign(data, testPassword)
	// Sorted keys: Amount, Blank, NullValue, OrderId, Password, TerminalKey.
	// NullValue (nil), Blank (empty string), DATA and Receipt are all skipped.
	wantConcat := "1" + "o" + testPassword + "Term"
	hash := sha256.Sum256([]byte(wantConcat))
	want := hex.EncodeToString(hash[:])
	if got != want {
		t.Fatalf("sign mismatch: got %q, want %q", got, want)
	}
}

func TestVerifyWebhookToken(t *testing.T) {
	payload := map[string]any{
		"TerminalKey": testTerminalKey,
		"OrderId":     "11111111-1111-1111-1111-111111111111",
		"Status":      "CONFIRMED",
		"Success":     true,
		"PaymentId":   json.Number("12345"),
	}
	payload["Token"] = sign(payload, testPassword)

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal webhook: %v", err)
	}

	p := newTestProvider("")
	_, err = p.ParseWebhook(context.Background(), body)
	if err != nil {
		t.Fatalf("valid webhook token rejected: %v", err)
	}

	payload["Token"] = "invalid"
	body, _ = json.Marshal(payload)
	_, err = p.ParseWebhook(context.Background(), body)
	if err == nil {
		t.Fatalf("invalid webhook token accepted")
	}
	if !strings.Contains(err.Error(), "invalid webhook token") {
		t.Fatalf("unexpected error: %v", err)
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
		{"partial_refunded", "PARTIAL_REFUNDED", 49000, domain.PaymentStatusPartialRefunded, 49000},
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
			payload["Token"] = sign(payload, testPassword)

			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("marshal webhook: %v", err)
			}

			p := newTestProvider("")
			got, err := p.ParseWebhook(context.Background(), body)
			if err != nil {
				t.Fatalf("ParseWebhook error: %v", err)
			}
			if got.Status != tt.wantStatus {
				t.Errorf("status = %v, want %v", got.Status, tt.wantStatus)
			}
			if got.AmountKopecks != tt.wantAmount {
				t.Errorf("amount = %v, want %v", got.AmountKopecks, tt.wantAmount)
			}
		})
	}
}

func TestVerifyWebhookTokenMissing(t *testing.T) {
	payload := map[string]any{
		"TerminalKey": testTerminalKey,
		"OrderId":     "order-123",
		"Status":      "CONFIRMED",
	}
	body, _ := json.Marshal(payload)

	p := newTestProvider("")
	_, err := p.ParseWebhook(context.Background(), body)
	if err == nil {
		t.Fatalf("expected error for missing token")
	}
	if !strings.Contains(err.Error(), "invalid webhook token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderInit(t *testing.T) {
	paymentID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/Init" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		data := verifyRequestToken(t, r, testPassword)

		if got, want := data["TerminalKey"], testTerminalKey; got != want {
			t.Fatalf("TerminalKey: got %v, want %v", got, want)
		}
		if got, want := data["OrderId"], paymentID.String(); got != want {
			t.Fatalf("OrderId: got %v, want %v", got, want)
		}
		if got, want := data["Amount"], float64(10000); got != want {
			t.Fatalf("Amount: got %v, want %v", got, want)
		}
		if got, want := data["Recurrent"], string(spec.Y); got != want {
			t.Fatalf("Recurrent: got %v, want %v", got, want)
		}
		if got, want := data["PayType"], string(spec.O); got != want {
			t.Fatalf("PayType: got %v, want %v", got, want)
		}
		dataObj, ok := data["DATA"].(map[string]any)
		if !ok {
			t.Fatalf("DATA missing or not object")
		}
		if got, want := dataObj["OperationInitiatorType"], string(spec.CommonOperationInitiatorTypeN1); got != want {
			t.Fatalf("OperationInitiatorType: got %v, want %v", got, want)
		}
		if _, ok := data["Token"]; !ok {
			t.Fatalf("Token missing from DATA exclusion test")
		}

		resp := initResponse{
			baseResponse: baseResponse{Success: true, Status: "NEW"},
			PaymentID:    "123456",
			PaymentURL:   "https://securepayments.tinkoff.ru/rest/show/123456",
			OrderID:      paymentID.String(),
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	result, err := p.Init(context.Background(), application.InitRequest{
		PaymentID:       paymentID,
		AmountKopecks:   10000,
		UserID:          userID,
		CustomerKey:     userID.String(),
		Description:     "Test payment",
		NotificationURL: "https://example.com/webhook",
		SuccessURL:      "https://example.com/success",
		FailURL:         "https://example.com/fail",
		Recurrent:       true,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if result.ProviderPaymentID != "123456" {
		t.Fatalf("ProviderPaymentID: got %q, want %q", result.ProviderPaymentID, "123456")
	}
	if result.PaymentURL != "https://securepayments.tinkoff.ru/rest/show/123456" {
		t.Fatalf("PaymentURL: got %q", result.PaymentURL)
	}
	if result.SavedToken != "" {
		t.Fatalf("SavedToken should be empty, got %q", result.SavedToken)
	}
	if result.Status != domain.PaymentStatusPending {
		t.Fatalf("Status: got %v, want %v", result.Status, domain.PaymentStatusPending)
	}
}

func TestProviderInitDescriptionTruncated(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = verifyRequestToken(t, r, testPassword)
		_ = json.NewEncoder(w).Encode(initResponse{
			baseResponse: baseResponse{Success: true, Status: "NEW"},
			PaymentID:    "1",
			PaymentURL:   "https://pay",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	longDesc := strings.Repeat("a", maxDescriptionLength+10)
	_, err := p.Init(context.Background(), application.InitRequest{
		PaymentID:     uuid.New(),
		AmountKopecks: 100,
		UserID:        uuid.New(),
		CustomerKey:   "ck",
		Description:   longDesc,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	got, ok := captured["Description"].(string)
	if !ok {
		t.Fatalf("Description missing or not string")
	}
	if len(got) != maxDescriptionLength {
		t.Fatalf("Description length: got %d, want %d", len(got), maxDescriptionLength)
	}
}

func TestProviderInitOperationInitiatorTypeDefault(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = verifyRequestToken(t, r, testPassword)
		_ = json.NewEncoder(w).Encode(initResponse{
			baseResponse: baseResponse{Success: true, Status: "NEW"},
			PaymentID:    "1",
			PaymentURL:   "https://pay",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	paymentID := uuid.New()
	_, _ = p.Init(context.Background(), application.InitRequest{
		PaymentID:     paymentID,
		AmountKopecks: 100,
		UserID:        uuid.New(),
		CustomerKey:   "ck",
	})

	dataObj := captured["DATA"].(map[string]any)
	if got, want := dataObj["OperationInitiatorType"], string(spec.CommonOperationInitiatorTypeN1); got != want {
		t.Fatalf("default OperationInitiatorType: got %v, want %v", got, want)
	}
}

func TestProviderInitRenewal(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = verifyRequestToken(t, r, testPassword)
		_ = json.NewEncoder(w).Encode(initResponse{
			baseResponse: baseResponse{Success: true, Status: "NEW"},
			PaymentID:    "123",
			PaymentURL:   "https://pay.example.com/123",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	paymentID := uuid.New()
	_, _ = p.Init(context.Background(), application.InitRequest{
		PaymentID:              paymentID,
		AmountKopecks:          100,
		UserID:                 uuid.New(),
		CustomerKey:            "ck",
		Recurrent:              false,
		OperationInitiatorType: application.InitiatorTypeMITRecurring,
	})

	if _, ok := captured["Recurrent"]; ok {
		t.Fatal("expected Recurrent field omitted for renewal")
	}
	dataObj := captured["DATA"].(map[string]any)
	if got, want := dataObj["OperationInitiatorType"], string(spec.CommonOperationInitiatorTypeR); got != want {
		t.Fatalf("renewal OperationInitiatorType: got %v, want %v", got, want)
	}
}

func TestProviderInitRedirectDueDate(t *testing.T) {
	newServer := func(captured *map[string]any) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*captured = verifyRequestToken(t, r, testPassword)
			_ = json.NewEncoder(w).Encode(initResponse{
				baseResponse: baseResponse{Success: true, Status: "NEW"},
				PaymentID:    "1",
				PaymentURL:   "https://pay",
			})
		}))
	}

	t.Run("set", func(t *testing.T) {
		var captured map[string]any
		server := newServer(&captured)
		defer server.Close()

		p := newTestProvider(server.URL + "/v2/")
		due := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
		_, err := p.Init(context.Background(), application.InitRequest{
			PaymentID:       uuid.New(),
			AmountKopecks:   100,
			UserID:          uuid.New(),
			CustomerKey:     "ck",
			RedirectDueDate: due,
		})
		if err != nil {
			t.Fatalf("Init failed: %v", err)
		}

		got, ok := captured["RedirectDueDate"].(string)
		if !ok {
			t.Fatalf("RedirectDueDate missing or not a string")
		}
		if want := "2026-07-13T15:00:00Z"; got != want {
			t.Fatalf("RedirectDueDate: got %q, want %q", got, want)
		}
	})

	t.Run("non_utc_normalizes_to_utc", func(t *testing.T) {
		var captured map[string]any
		server := newServer(&captured)
		defer server.Close()

		p := newTestProvider(server.URL + "/v2/")
		due := time.Date(2026, 7, 13, 15, 0, 0, 0, time.FixedZone("MSK", 3*60*60))
		_, err := p.Init(context.Background(), application.InitRequest{
			PaymentID:       uuid.New(),
			AmountKopecks:   100,
			UserID:          uuid.New(),
			CustomerKey:     "ck",
			RedirectDueDate: due,
		})
		if err != nil {
			t.Fatalf("Init failed: %v", err)
		}

		got, ok := captured["RedirectDueDate"].(string)
		if !ok {
			t.Fatalf("RedirectDueDate missing or not a string")
		}
		if want := "2026-07-13T12:00:00Z"; got != want {
			t.Fatalf("RedirectDueDate: got %q, want %q", got, want)
		}
	})

	t.Run("zero_omits_field", func(t *testing.T) {
		var captured map[string]any
		server := newServer(&captured)
		defer server.Close()

		p := newTestProvider(server.URL + "/v2/")
		_, err := p.Init(context.Background(), application.InitRequest{
			PaymentID:     uuid.New(),
			AmountKopecks: 100,
			UserID:        uuid.New(),
			CustomerKey:   "ck",
		})
		if err != nil {
			t.Fatalf("Init failed: %v", err)
		}

		if v, ok := captured["RedirectDueDate"]; ok {
			t.Fatalf("RedirectDueDate must be omitted when zero, got %v", v)
		}
	})
}

func TestProviderInitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = verifyRequestToken(t, r, testPassword)
		_ = json.NewEncoder(w).Encode(initResponse{
			baseResponse: baseResponse{
				Success:   false,
				ErrorCode: "7",
				Message:   "Invalid amount",
			},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	_, err := p.Init(context.Background(), application.InitRequest{
		PaymentID:     uuid.New(),
		AmountKopecks: 100,
		UserID:        uuid.New(),
		CustomerKey:   "ck",
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

func TestProviderCharge(t *testing.T) {
	paymentID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/Charge" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		data := verifyRequestToken(t, r, testPassword)
		if got, want := data["PaymentId"], "123"; got != want {
			t.Fatalf("PaymentId: got %v, want %v", got, want)
		}
		if got, want := data["RebillId"], "rebill-token"; got != want {
			t.Fatalf("RebillId: got %v, want %v", got, want)
		}

		_ = json.NewEncoder(w).Encode(chargeResponse{
			baseResponse: baseResponse{Success: true, Status: "CONFIRMED"},
			PaymentID:    "123",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	result, err := p.Charge(context.Background(), application.ChargeRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: "123",
		AmountKopecks:     10000,
		Token:             "rebill-token",
	})
	if err != nil {
		t.Fatalf("Charge failed: %v", err)
	}
	if result.ProviderPaymentID != "123" {
		t.Fatalf("ProviderPaymentID: got %q", result.ProviderPaymentID)
	}
	if result.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("Status: got %v, want %v", result.Status, domain.PaymentStatusSucceeded)
	}
}

func TestProviderChargeFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = verifyRequestToken(t, r, testPassword)
		_ = json.NewEncoder(w).Encode(chargeResponse{
			baseResponse: baseResponse{
				Success:   false,
				ErrorCode: "105",
				Message:   "Charge rejected",
				Status:    "REJECTED",
			},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	_, err := p.Charge(context.Background(), application.ChargeRequest{
		PaymentID:         uuid.New(),
		ProviderPaymentID: "123",
		AmountKopecks:     10000,
		Token:             "rebill-token",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.ErrorCode != "105" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderStatus(t *testing.T) {
	paymentID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/GetState" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = verifyRequestToken(t, r, testPassword)
		_ = json.NewEncoder(w).Encode(getStateResponse{
			baseResponse: baseResponse{Success: true, Status: "AUTHORIZED"},
			PaymentID:    "999",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	status, err := p.Status(context.Background(), paymentID, "999")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.Status != domain.PaymentStatusPending {
		t.Fatalf("Status: got %v, want %v", status.Status, domain.PaymentStatusPending)
	}
}

func TestProviderStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/GetState" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = verifyRequestToken(t, r, testPassword)
		_ = json.NewEncoder(w).Encode(getStateResponse{
			baseResponse: baseResponse{
				Success:   false,
				ErrorCode: "10",
				Message:   "Payment not found",
			},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	_, err := p.Status(context.Background(), uuid.New(), "999")
	if err == nil {
		t.Fatalf("expected error")
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.ErrorCode != "10" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderStatusHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	_, err := p.Status(context.Background(), uuid.New(), "999")
	if err == nil {
		t.Fatalf("expected HTTP error")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderInitAddCard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/AddCustomer":
			_ = verifyRequestToken(t, r, testPassword)
			_ = json.NewEncoder(w).Encode(addCustomerResponse{
				baseResponse: baseResponse{Success: true},
				CustomerKey:  "customer-1",
			})
		case "/v2/AddCard":
			// The T-Kassa server verifies the AddCard token over the schema
			// fields only; RedirectUrl/FailRedirectUrl are outside the schema
			// and excluded from the check.
			data := verifyRequestTokenExcluding(t, r, testPassword, "RedirectUrl", "FailRedirectUrl")
			if got, want := data["CheckType"], "3DSHOLD"; got != want {
				t.Fatalf("CheckType: got %v, want %v", got, want)
			}
			if got, want := data["RedirectUrl"], "https://app.example/api/subscription/payment-methods/add-card/success"; got != want {
				t.Fatalf("RedirectUrl: got %v, want %v", got, want)
			}
			if got, want := data["FailRedirectUrl"], "https://app.example/api/subscription/payment-methods/add-card/fail"; got != want {
				t.Fatalf("FailRedirectUrl: got %v, want %v", got, want)
			}
			if _, ok := data["NotificationURL"]; ok {
				t.Fatalf("AddCard must not contain NotificationURL")
			}
			_ = json.NewEncoder(w).Encode(addCardResponse{
				baseResponse: baseResponse{Success: true},
				PaymentURL:   "https://securepayments.tinkoff.ru/rest/addcard/abc",
				RequestKey:   "request-key-1",
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	result, err := p.InitAddCard(context.Background(), application.InitAddCardRequest{
		UserID:      uuid.New(),
		CustomerKey: "customer-1",
		SuccessURL:  "https://app.example/api/subscription/payment-methods/add-card/success",
		FailURL:     "https://app.example/api/subscription/payment-methods/add-card/fail",
	})
	if err != nil {
		t.Fatalf("InitAddCard failed: %v", err)
	}
	if result.PaymentURL != "https://securepayments.tinkoff.ru/rest/addcard/abc" {
		t.Fatalf("PaymentURL: got %q", result.PaymentURL)
	}
	if result.RequestKey != "request-key-1" {
		t.Fatalf("RequestKey: got %q", result.RequestKey)
	}
}

func TestProviderInitAddCardCustomerAlreadyExistsProceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/AddCustomer":
			_ = verifyRequestToken(t, r, testPassword)
			_ = json.NewEncoder(w).Encode(addCustomerResponse{
				baseResponse: baseResponse{
					Success:   false,
					ErrorCode: "7",
					Message:   "Customer already exists",
				},
				CustomerKey: "customer-1",
			})
		case "/v2/AddCard":
			_ = json.NewEncoder(w).Encode(addCardResponse{
				baseResponse: baseResponse{Success: true},
				PaymentURL:   "https://securepayments.tinkoff.ru/rest/addcard/abc",
				RequestKey:   "request-key-1",
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	result, err := p.InitAddCard(context.Background(), application.InitAddCardRequest{
		UserID:      uuid.New(),
		CustomerKey: "customer-1",
	})
	if err != nil {
		t.Fatalf("InitAddCard failed: %v", err)
	}
	if result.PaymentURL != "https://securepayments.tinkoff.ru/rest/addcard/abc" {
		t.Fatalf("PaymentURL: got %q", result.PaymentURL)
	}
	if result.RequestKey != "request-key-1" {
		t.Fatalf("RequestKey: got %q", result.RequestKey)
	}
}

func TestProviderInitAddCardOtherAPIErrorPropagated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/AddCustomer":
			_ = verifyRequestToken(t, r, testPassword)
			_ = json.NewEncoder(w).Encode(addCustomerResponse{
				baseResponse: baseResponse{
					Success:   false,
					ErrorCode: "3",
					Message:   "Internal error",
				},
			})
		case "/v2/AddCard":
			t.Fatalf("AddCard should not be called when AddCustomer fails with a non-duplicate error")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	_, err := p.InitAddCard(context.Background(), application.InitAddCardRequest{
		UserID:      uuid.New(),
		CustomerKey: "customer-1",
	})
	if err == nil {
		t.Fatalf("expected error when AddCustomer fails with a non-duplicate error")
	}
	if !strings.Contains(err.Error(), "add customer failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderInitAddCardCustomerNetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/AddCustomer" {
			_ = verifyRequestToken(t, r, testPassword)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		t.Fatalf("unexpected path: %s", r.URL.Path)
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	_, err := p.InitAddCard(context.Background(), application.InitAddCardRequest{
		UserID:      uuid.New(),
		CustomerKey: "customer-1",
	})
	if err == nil {
		t.Fatalf("expected error when AddCustomer fails with HTTP error")
	}
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		t.Fatalf("expected network error, not ProviderError: %v", err)
	}
}

func TestProviderRemoveCard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/RemoveCard" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		data := verifyRequestToken(t, r, testPassword)
		if got, want := data["CardId"], "card-1"; got != want {
			t.Fatalf("CardId: got %v, want %v", got, want)
		}
		_ = json.NewEncoder(w).Encode(removeCardResponse{
			baseResponse: baseResponse{Success: true},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	err := p.RemoveCard(context.Background(), "customer-1", "card-1")
	if err != nil {
		t.Fatalf("RemoveCard failed: %v", err)
	}
}

func TestProviderRemoveCardError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = verifyRequestToken(t, r, testPassword)
		_ = json.NewEncoder(w).Encode(removeCardResponse{
			baseResponse: baseResponse{
				Success:   false,
				ErrorCode: "9",
				Message:   "Card not found",
			},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	err := p.RemoveCard(context.Background(), "customer-1", "card-1")
	if err == nil {
		t.Fatalf("expected error")
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.ErrorCode != "9" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderRemoveCardNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = verifyRequestToken(t, r, testPassword)
		_ = json.NewEncoder(w).Encode(removeCardResponse{
			baseResponse: baseResponse{
				Success:   false,
				ErrorCode: "107",
				Message:   "Card not found",
			},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	err := p.RemoveCard(context.Background(), "customer-1", "card-1")
	if !errors.Is(err, application.ErrProviderCardNotFound) {
		t.Fatalf("expected ErrProviderCardNotFound, got %v", err)
	}
}

func TestProviderGetCardList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/GetCardList" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		data := verifyRequestToken(t, r, testPassword)
		if got, want := data["TerminalKey"], testTerminalKey; got != want {
			t.Fatalf("TerminalKey: got %v, want %v", got, want)
		}
		if got, want := data["CustomerKey"], "customer-1"; got != want {
			t.Fatalf("CustomerKey: got %v, want %v", got, want)
		}
		// T-Kassa answers GetCardList with a bare JSON array, and RebillId may
		// be absent for cards that were not saved for recurrent charges.
		_ = json.NewEncoder(w).Encode([]cardListItem{
			{CardID: "card-1", Pan: "430000******0777", ExpDate: "1230", Status: "A", RebillID: "rebill-1"},
			{CardID: "card-2", Pan: "430000******0888", ExpDate: "1231", Status: "I"},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	cards, err := p.GetCardList(context.Background(), "customer-1")
	if err != nil {
		t.Fatalf("GetCardList failed: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(cards))
	}
	if cards[0].CardID != "card-1" || cards[0].Pan != "430000******0777" || cards[0].ExpDate != "1230" {
		t.Fatalf("unexpected first card: %+v", cards[0])
	}
	if cards[0].RebillID != "rebill-1" {
		t.Fatalf("RebillID: got %q, want %q", cards[0].RebillID, "rebill-1")
	}
	if cards[0].Status != application.ProviderCardStatusActive {
		t.Fatalf("Status: got %q, want %q", cards[0].Status, application.ProviderCardStatusActive)
	}
	if cards[1].RebillID != "" {
		t.Fatalf("expected missing RebillId to decode as empty, got %q", cards[1].RebillID)
	}
	if cards[1].Status != application.ProviderCardStatusInactive {
		t.Fatalf("Status: got %q, want %q", cards[1].Status, application.ProviderCardStatusInactive)
	}
}

func TestProviderGetCardListEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = verifyRequestToken(t, r, testPassword)
		_ = json.NewEncoder(w).Encode([]cardListItem{})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	cards, err := p.GetCardList(context.Background(), "customer-1")
	if err != nil {
		t.Fatalf("GetCardList failed: %v", err)
	}
	if len(cards) != 0 {
		t.Fatalf("expected no cards, got %d", len(cards))
	}
}

func TestProviderGetCardListError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = verifyRequestToken(t, r, testPassword)
		_ = json.NewEncoder(w).Encode(baseResponse{
			Success:   false,
			ErrorCode: "503",
			Message:   "CustomerKey not found",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	_, err := p.GetCardList(context.Background(), "customer-1")
	if err == nil {
		t.Fatalf("expected error")
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.ErrorCode != "503" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderGetCardListTerminalNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = verifyRequestToken(t, r, testPassword)
		_ = json.NewEncoder(w).Encode(baseResponse{
			Success:   false,
			ErrorCode: "501",
			Message:   "Терминал не найден",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	_, err := p.GetCardList(context.Background(), "customer-1")
	if !errors.Is(err, application.ErrProviderTerminalNotFound) {
		t.Fatalf("expected ErrProviderTerminalNotFound, got %v", err)
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.ErrorCode != "501" {
		t.Fatalf("expected wrapped ProviderError with code 501, got %v", err)
	}
}

func TestProviderWebhookResponse(t *testing.T) {
	p := newTestProvider("")
	if got, want := string(p.WebhookResponse()), "OK"; got != want {
		t.Fatalf("WebhookResponse: got %q, want %q", got, want)
	}
}

func TestParseWebhookPayment(t *testing.T) {
	paymentID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	payload := map[string]any{
		"TerminalKey": testTerminalKey,
		"OrderId":     paymentID.String(),
		"PaymentId":   json.Number("777"),
		"Status":      "CONFIRMED",
		"Success":     true,
		"ErrorCode":   "0",
		"RebillId":    "rebill-1",
		"CardId":      "card-1",
		"Pan":         "430000******0777",
		"ExpDate":     "12/25",
		"CustomerKey": "customer-1",
	}
	payload["Token"] = sign(payload, testPassword)
	body, _ := json.Marshal(payload)

	p := newTestProvider("")
	result, err := p.ParseWebhook(context.Background(), body)
	if err != nil {
		t.Fatalf("ParseWebhook failed: %v", err)
	}
	if result.InternalPaymentID != paymentID {
		t.Fatalf("InternalPaymentID: got %v, want %v", result.InternalPaymentID, paymentID)
	}
	if result.ProviderPaymentID != "777" {
		t.Fatalf("ProviderPaymentID: got %q", result.ProviderPaymentID)
	}
	if result.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("Status: got %v", result.Status)
	}
	if result.ErrorCode != nil {
		t.Fatalf("ErrorCode should be nil for code 0")
	}
	if result.RebillID != "rebill-1" {
		t.Fatalf("RebillID: got %q", result.RebillID)
	}
	if result.Pan != "430000******0777" {
		t.Fatalf("Pan: got %q", result.Pan)
	}
}

func TestParseWebhookAddCard(t *testing.T) {
	payload := map[string]any{
		"TerminalKey":      testTerminalKey,
		"NotificationType": notificationTypeAddCard,
		"CustomerKey":      "customer-1",
		"RequestKey":       "request-key-1",
		"RebillId":         "rebill-1",
		"CardId":           "card-1",
		"Pan":              "430000******0777",
		"ExpDate":          "12/25",
		"Status":           "COMPLETED",
		"Success":          true,
	}
	payload["Token"] = sign(payload, testPassword)
	body, _ := json.Marshal(payload)

	p := newTestProvider("")
	result, err := p.ParseWebhook(context.Background(), body)
	if err != nil {
		t.Fatalf("ParseWebhook failed: %v", err)
	}
	if result.NotificationType != notificationTypeAddCard {
		t.Fatalf("NotificationType: got %q", result.NotificationType)
	}
	if result.InternalPaymentID != uuid.Nil {
		t.Fatalf("InternalPaymentID should be zero for AddCard webhook")
	}
	if result.RequestKey != "request-key-1" {
		t.Fatalf("RequestKey: got %q", result.RequestKey)
	}
}

func TestParseWebhookAddCardRejectsNonSuccess(t *testing.T) {
	payload := map[string]any{
		"TerminalKey":      testTerminalKey,
		"NotificationType": notificationTypeAddCard,
		"CustomerKey":      "customer-1",
		"RequestKey":       "request-key-1",
		"RebillId":         "rebill-1",
		"CardId":           "card-1",
		"Pan":              "430000******0777",
		"ExpDate":          "12/25",
		"Status":           "REJECTED",
		"Success":          false,
	}
	payload["Token"] = sign(payload, testPassword)
	body, _ := json.Marshal(payload)

	p := newTestProvider("")
	_, err := p.ParseWebhook(context.Background(), body)
	if err == nil {
		t.Fatalf("expected error for non-success AddCard webhook")
	}
	if !strings.Contains(err.Error(), "binding not successful") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseWebhookAddCardLegacyType(t *testing.T) {
	payload := map[string]any{
		"TerminalKey":      testTerminalKey,
		"NotificationType": notificationTypeAddCardLegacy,
		"CustomerKey":      "customer-1",
		"RequestKey":       "request-key-1",
		"RebillId":         "rebill-1",
		"CardId":           "card-1",
		"Pan":              "430000******0777",
		"ExpDate":          "12/25",
		"Status":           "COMPLETED",
		"Success":          true,
	}
	payload["Token"] = sign(payload, testPassword)
	body, _ := json.Marshal(payload)

	p := newTestProvider("")
	result, err := p.ParseWebhook(context.Background(), body)
	if err != nil {
		t.Fatalf("ParseWebhook failed: %v", err)
	}
	if result.NotificationType != notificationTypeAddCardLegacy {
		t.Fatalf("NotificationType: got %q, want %q", result.NotificationType, notificationTypeAddCardLegacy)
	}
	if result.RequestKey != "request-key-1" {
		t.Fatalf("RequestKey: got %q", result.RequestKey)
	}
}

func TestParseWebhookAddCardWithoutNotificationType(t *testing.T) {
	// The official NotificationAddCard payload omits NotificationType and
	// OrderId; it must be routed to the add-card branch via RequestKey.
	payload := map[string]any{
		"TerminalKey": testTerminalKey,
		"CustomerKey": "customer-1",
		"RequestKey":  "request-key-1",
		"RebillId":    "rebill-1",
		"CardId":      "card-1",
		"Pan":         "430000******0777",
		"ExpDate":     "12/25",
		"Status":      "COMPLETED",
		"Success":     true,
	}
	payload["Token"] = sign(payload, testPassword)
	body, _ := json.Marshal(payload)

	p := newTestProvider("")
	result, err := p.ParseWebhook(context.Background(), body)
	if err != nil {
		t.Fatalf("ParseWebhook failed: %v", err)
	}
	if result.NotificationType != notificationTypeAddCard {
		t.Fatalf("NotificationType: got %q, want %q", result.NotificationType, notificationTypeAddCard)
	}
	if result.InternalPaymentID != uuid.Nil {
		t.Fatalf("InternalPaymentID should be zero for AddCard webhook")
	}
	if result.RequestKey != "request-key-1" {
		t.Fatalf("RequestKey: got %q", result.RequestKey)
	}
	if result.CustomerKey != "customer-1" {
		t.Fatalf("CustomerKey: got %q", result.CustomerKey)
	}
	if result.RebillID != "rebill-1" {
		t.Fatalf("RebillID: got %q", result.RebillID)
	}
}

func TestParseWebhookUnknownNotificationType(t *testing.T) {
	payload := map[string]any{
		"TerminalKey":      testTerminalKey,
		"NotificationType": "NotificationFiscalization",
		"Success":          true,
	}
	payload["Token"] = sign(payload, testPassword)
	body, _ := json.Marshal(payload)

	p := newTestProvider("")
	_, err := p.ParseWebhook(context.Background(), body)
	if err == nil {
		t.Fatalf("expected error for unknown notification type")
	}
	if !strings.Contains(err.Error(), "unknown notification type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetInt64(t *testing.T) {
	t.Run("missing key returns zero without error", func(t *testing.T) {
		got, err := getInt64(map[string]any{}, "Amount")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 0 {
			t.Fatalf("got %d, want 0", got)
		}
	})

	t.Run("empty string returns zero without error", func(t *testing.T) {
		got, err := getInt64(map[string]any{"Amount": ""}, "Amount")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 0 {
			t.Fatalf("got %d, want 0", got)
		}
	})

	t.Run("valid number returns value", func(t *testing.T) {
		got, err := getInt64(map[string]any{"Amount": json.Number("99000")}, "Amount")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 99000 {
			t.Fatalf("got %d, want 99000", got)
		}
	})

	t.Run("non-numeric string returns error", func(t *testing.T) {
		_, err := getInt64(map[string]any{"Amount": "not-a-number"}, "Amount")
		if err == nil {
			t.Fatalf("expected error for non-numeric string")
		}
		if !strings.Contains(err.Error(), "tkassa: parse Amount") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestParseWebhookInvalidOrderID(t *testing.T) {
	payload := map[string]any{
		"TerminalKey": testTerminalKey,
		"OrderId":     "not-a-uuid",
		"PaymentId":   json.Number("12345"),
		"Status":      "CONFIRMED",
		"Success":     true,
	}
	payload["Token"] = sign(payload, testPassword)
	body, _ := json.Marshal(payload)

	p := newTestProvider("")
	_, err := p.ParseWebhook(context.Background(), body)
	if err == nil {
		t.Fatalf("expected error for invalid OrderId")
	}
	if !strings.Contains(err.Error(), "parse OrderId") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	_, err := p.Status(context.Background(), uuid.New(), "1")
	if err == nil {
		t.Fatalf("expected HTTP error")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderHTTPErrorIncludesBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid request body"))
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	_, err := p.Status(context.Background(), uuid.New(), "1")
	if err == nil {
		t.Fatalf("expected HTTP error")
	}
	if !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("expected HTTP 400 in error: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid request body") {
		t.Fatalf("expected response body in error: %v", err)
	}
}

func TestProviderTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(getStateResponse{baseResponse: baseResponse{Success: true}})
	}))
	defer server.Close()

	p := NewProvider(server.URL+"/v2/", testTerminalKey, testPassword, 1*time.Nanosecond, 0, 0, 0, discardLogger(), nil)
	_, err := p.Status(context.Background(), uuid.New(), "1")
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderInvalidProviderPaymentID(t *testing.T) {
	p := newTestProvider("")
	_, err := p.Charge(context.Background(), application.ChargeRequest{
		PaymentID:         uuid.New(),
		ProviderPaymentID: "not-a-number",
	})
	if err == nil {
		t.Fatalf("expected error for invalid provider payment id")
	}
}

func TestMapStatus(t *testing.T) {
	cases := []struct {
		in   string
		want domain.PaymentStatus
	}{
		{"AUTHORIZED", domain.PaymentStatusPending},
		{"NEW", domain.PaymentStatusPending},
		{"CONFIRMED", domain.PaymentStatusSucceeded},
		{"REJECTED", domain.PaymentStatusFailed},
		{"AUTH_FAIL", domain.PaymentStatusFailed},
		{"REVERSED", domain.PaymentStatusFailed},
		{"PARTIAL_REVERSED", domain.PaymentStatusFailed},
		{"REVERSING", domain.PaymentStatusPending},
		{"REFUNDING", domain.PaymentStatusPending},
		{"ASYNC_REFUNDING", domain.PaymentStatusPending},
		{"REFUNDED", domain.PaymentStatusRefunded},
		{"PARTIAL_REFUNDED", domain.PaymentStatusPartialRefunded},
		{"UNKNOWN", domain.PaymentStatusPending},
	}
	for _, tc := range cases {
		if got := mapStatus(tc.in); got != tc.want {
			t.Errorf("mapStatus(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestMapCancelStatus(t *testing.T) {
	cases := []struct {
		in   string
		want domain.PaymentStatus
	}{
		{"REFUNDED", domain.PaymentStatusRefunded},
		{"REVERSED", domain.PaymentStatusRefunded},
		{"PARTIAL_REFUNDED", domain.PaymentStatusPartialRefunded},
		{"PARTIAL_REVERSED", domain.PaymentStatusPartialRefunded},
		{"NEW", domain.PaymentStatusPending},
		{"AUTHORIZED", domain.PaymentStatusPending},
		{"REVERSING", domain.PaymentStatusRefunding},
		{"REFUNDING", domain.PaymentStatusRefunding},
		{"ASYNC_REFUNDING", domain.PaymentStatusRefunding},
		{"REJECTED", domain.PaymentStatusFailed},
		{"UNKNOWN", domain.PaymentStatusFailed},
	}
	for _, tc := range cases {
		if got := mapCancelStatus(tc.in); got != tc.want {
			t.Errorf("mapCancelStatus(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestNewProviderDefaults(t *testing.T) {
	p := NewProvider("", testTerminalKey, testPassword, 0, 0, 0, 0, discardLogger(), nil)
	if !strings.HasSuffix(p.baseURL, "/") {
		t.Fatalf("baseURL should have trailing slash: %q", p.baseURL)
	}
	if p.client.Timeout != defaultTimeout {
		t.Fatalf("timeout: got %v, want %v", p.client.Timeout, defaultTimeout)
	}
}

func TestNewProviderTrimsTrailingSlash(t *testing.T) {
	p := NewProvider("https://example.com/rest", testTerminalKey, testPassword, 0, 0, 0, 0, discardLogger(), nil)
	if p.baseURL != "https://example.com/rest/" {
		t.Fatalf("baseURL: got %q", p.baseURL)
	}
}

func TestSignDoesNotMutateInput(t *testing.T) {
	data := map[string]any{"TerminalKey": "T", "Amount": int64(1)}
	original := maps.Clone(data)
	_ = sign(data, testPassword)
	if len(data) != len(original) {
		t.Fatalf("sign mutated input map")
	}
}

func TestProviderCancel_FullRefund(t *testing.T) {
	paymentID := uuid.New()
	providerPaymentID := "cancel-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/Cancel" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		data := verifyRequestToken(t, r, testPassword)
		if got, want := data["PaymentId"], providerPaymentID; got != want {
			t.Fatalf("PaymentId: got %v, want %v", got, want)
		}
		if got, want := data["Amount"], float64(10000); got != want {
			t.Fatalf("Amount: got %v, want %v", got, want)
		}

		_ = json.NewEncoder(w).Encode(cancelResponse{
			baseResponse:   baseResponse{Success: true, Status: "REFUNDED"},
			PaymentID:      providerPaymentID,
			OrderID:        paymentID.String(),
			OriginalAmount: 10000,
			NewAmount:      0,
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	result, err := p.Cancel(t.Context(), application.CancelRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: providerPaymentID,
		AmountKopecks:     10000,
	})
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	if result.Status != domain.PaymentStatusRefunded {
		t.Fatalf("Status: got %v, want %v", result.Status, domain.PaymentStatusRefunded)
	}
	if result.RefundedAmountKopecks != 10000 {
		t.Fatalf("RefundedAmountKopecks: got %d, want 10000", result.RefundedAmountKopecks)
	}
}

func TestProviderCancel_ReversedPending(t *testing.T) {
	paymentID := uuid.New()
	providerPaymentID := "cancel-reversed"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/Cancel" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = verifyRequestToken(t, r, testPassword)
		_ = json.NewEncoder(w).Encode(cancelResponse{
			baseResponse:   baseResponse{Success: true, Status: "REVERSED"},
			PaymentID:      providerPaymentID,
			OrderID:        paymentID.String(),
			OriginalAmount: 10000,
			NewAmount:      0,
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	result, err := p.Cancel(context.Background(), application.CancelRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: providerPaymentID,
		AmountKopecks:     0,
	})
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	if result.Status != domain.PaymentStatusRefunded {
		t.Fatalf("Status: got %v, want %v", result.Status, domain.PaymentStatusRefunded)
	}
	if result.RefundedAmountKopecks != 10000 {
		t.Fatalf("RefundedAmountKopecks: got %d, want 10000", result.RefundedAmountKopecks)
	}
}

func TestProviderCancel_Error(t *testing.T) {
	paymentID := uuid.New()
	providerPaymentID := "cancel-789"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/Cancel" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = verifyRequestToken(t, r, testPassword)
		_ = json.NewEncoder(w).Encode(cancelResponse{
			baseResponse: baseResponse{
				Success:   false,
				ErrorCode: "204",
				Message:   "Cannot cancel payment",
			},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	_, err := p.Cancel(context.Background(), application.CancelRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: providerPaymentID,
		AmountKopecks:     0,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.ErrorCode != "204" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// captureRequest reads the request body, verifies the T-Kassa token, and returns
// both the raw JSON and the decoded map. Fields in exclude are removed before
// token verification to match the server-side schema (e.g. AddCard ignores
// RedirectUrl/FailRedirectUrl).
func captureRequest(t *testing.T, r *http.Request, exclude ...string) ([]byte, map[string]any) {
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

// strictDecodeSpecRequest decodes a captured request body into a generated spec
// request struct with DisallowUnknownFields, so any field the adapter sends
// that is not part of the spec schema fails the test. Nested DATA is the
// spec.Init_DATA union with custom unmarshaling, so strictness applies to the
// top-level object only — DATA contents are asserted separately at map level.
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
// Provider.InitAddCard) does not contain them, and the server ignores them for
// token verification. They are asserted at map level and stripped before the
// strict spec-schema decode instead of weakening strictness for everything.
var addCardExtraFieldsAllowlist = []string{"RedirectUrl", "FailRedirectUrl"}

// strictDecodeAddCardRequest verifies the allowlisted AddCard extras are
// present in the captured body with the expected values, strips them, and
// strict-decodes the remaining body into spec.AddCard.
func strictDecodeAddCardRequest(t *testing.T, captured map[string]any, wantRedirectURL, wantFailRedirectURL string) spec.AddCard {
	t.Helper()
	wantExtras := map[string]string{
		"RedirectUrl":     wantRedirectURL,
		"FailRedirectUrl": wantFailRedirectURL,
	}
	stripped := maps.Clone(captured)
	for _, key := range addCardExtraFieldsAllowlist {
		got, ok := stripped[key].(string)
		if !ok {
			t.Fatalf("AddCard extra field %s missing or not a string", key)
		}
		if want := wantExtras[key]; got != want {
			t.Fatalf("AddCard extra field %s: got %q, want %q", key, got, want)
		}
		delete(stripped, key)
	}
	body, err := json.Marshal(stripped)
	if err != nil {
		t.Fatalf("re-marshal stripped AddCard body: %v", err)
	}
	var reqBody spec.AddCard
	strictDecodeSpecRequest(t, body, &reqBody)
	return reqBody
}

func TestProviderInitContract(t *testing.T) {
	tests := []struct {
		name                   string
		req                    application.InitRequest
		wantOperationInitiator spec.CommonOperationInitiatorType
		wantRecurrent          bool
	}{
		{
			name: "first_payment_recurrent",
			req: application.InitRequest{
				PaymentID:       uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				AmountKopecks:   10000,
				UserID:          uuid.MustParse("22222222-2222-2222-2222-222222222222"),
				CustomerKey:     "customer-1",
				Description:     "Test payment",
				NotificationURL: "https://example.com/webhook",
				SuccessURL:      "https://example.com/success",
				FailURL:         "https://example.com/fail",
				Recurrent:       true,
				RedirectDueDate: time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC),
			},
			wantOperationInitiator: spec.CommonOperationInitiatorTypeN1, // CIT CC per the T-Kassa spec
			wantRecurrent:          true,
		},
		{
			name: "renewal",
			req: application.InitRequest{
				PaymentID:              uuid.New(),
				AmountKopecks:          500,
				UserID:                 uuid.New(),
				CustomerKey:            "customer-2",
				Recurrent:              false,
				OperationInitiatorType: application.InitiatorTypeMITRecurring,
			},
			wantOperationInitiator: spec.CommonOperationInitiatorTypeR, // MIT COF Recurring per the T-Kassa spec
			wantRecurrent:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paymentURL := "https://securepayments.tinkoff.ru/rest/show/123456"
			var captured []byte
			var capturedMap map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v2/Init" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				captured, capturedMap = captureRequest(t, r)
				// Response fixture from the spec: the Init 200 response is the
				// generated spec.Response type, shaped after the example in
				// openapi.yaml (/v2/Init).
				_ = json.NewEncoder(w).Encode(spec.Response{
					TerminalKey: testTerminalKey,
					Amount:      float32(tt.req.AmountKopecks),
					OrderId:     tt.req.PaymentID.String(),
					Success:     true,
					Status:      "NEW",
					PaymentId:   "123456",
					ErrorCode:   "0",
					PaymentURL:  &paymentURL,
				})
			}))
			defer server.Close()

			p := newTestProvider(server.URL + "/v2/")
			result, err := p.Init(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("Init failed: %v", err)
			}
			if result.ProviderPaymentID != "123456" {
				t.Errorf("ProviderPaymentID: got %q, want %q", result.ProviderPaymentID, "123456")
			}
			if result.PaymentURL != paymentURL {
				t.Errorf("PaymentURL: got %q, want %q", result.PaymentURL, paymentURL)
			}
			if result.Status != domain.PaymentStatusPending {
				t.Errorf("Status: got %v, want %v", result.Status, domain.PaymentStatusPending)
			}
			if captured == nil {
				t.Fatalf("request was not captured")
			}

			// Map-level assertions on the raw body: Recurrent must be present
			// only for the parent (recurrent) payment.
			if tt.wantRecurrent {
				if got, want := capturedMap["Recurrent"], string(spec.Y); got != want {
					t.Errorf("Recurrent: got %v, want %q", got, want)
				}
			} else if _, ok := capturedMap["Recurrent"]; ok {
				t.Errorf("Recurrent must be omitted for renewal Init, got %v", capturedMap["Recurrent"])
			}
			// RedirectDueDate rides along only when set: the user-facing parent
			// payment carries the payment-form deadline, renewals (MIT) must not.
			if !tt.req.RedirectDueDate.IsZero() {
				raw, ok := capturedMap["RedirectDueDate"].(string)
				if !ok {
					t.Fatalf("RedirectDueDate missing or not a string")
				}
				if _, err := time.Parse(time.RFC3339, raw); err != nil {
					t.Errorf("RedirectDueDate is not RFC3339: %q: %v", raw, err)
				}
				if want := tt.req.RedirectDueDate.UTC().Format(time.RFC3339); raw != want {
					t.Errorf("RedirectDueDate: got %q, want %q", raw, want)
				}
			} else if _, ok := capturedMap["RedirectDueDate"]; ok {
				t.Errorf("RedirectDueDate must be omitted for renewal Init, got %v", capturedMap["RedirectDueDate"])
			}
			dataObj, ok := capturedMap["DATA"].(map[string]any)
			if !ok {
				t.Fatalf("DATA missing or not an object")
			}
			if got, want := dataObj["OperationInitiatorType"], string(tt.wantOperationInitiator); got != want {
				t.Errorf("DATA.OperationInitiatorType: got %v, want %q", got, want)
			}

			var reqBody spec.Init
			strictDecodeSpecRequest(t, captured, &reqBody)

			if reqBody.TerminalKey != testTerminalKey {
				t.Errorf("TerminalKey: got %q, want %q", reqBody.TerminalKey, testTerminalKey)
			}
			if reqBody.Token == "" {
				t.Errorf("Token is empty")
			}
			if reqBody.OrderId != tt.req.PaymentID.String() {
				t.Errorf("OrderId: got %q, want %q", reqBody.OrderId, tt.req.PaymentID.String())
			}
			if reqBody.Amount != tt.req.AmountKopecks {
				t.Errorf("Amount: got %v, want %v", reqBody.Amount, tt.req.AmountKopecks)
			}
			if reqBody.PayType == nil || *reqBody.PayType != spec.O {
				t.Errorf("PayType: got %v, want %v", reqBody.PayType, spec.O)
			}
			if reqBody.CustomerKey == nil || *reqBody.CustomerKey != tt.req.CustomerKey {
				t.Errorf("CustomerKey: got %v, want %q", reqBody.CustomerKey, tt.req.CustomerKey)
			}

			if tt.wantRecurrent {
				if reqBody.Recurrent == nil || *reqBody.Recurrent != spec.Y {
					t.Errorf("Recurrent: got %v, want %v", reqBody.Recurrent, spec.Y)
				}
			} else {
				if reqBody.Recurrent != nil {
					t.Errorf("Recurrent should be omitted, got %v", *reqBody.Recurrent)
				}
			}
			if !tt.req.RedirectDueDate.IsZero() {
				if want := tt.req.RedirectDueDate.UTC().Format(time.RFC3339); reqBody.RedirectDueDate != want {
					t.Errorf("spec.Init.RedirectDueDate: got %v, want %q", reqBody.RedirectDueDate, want)
				}
			}

			if tt.req.NotificationURL != "" {
				if reqBody.NotificationURL == nil || *reqBody.NotificationURL != tt.req.NotificationURL {
					t.Errorf("NotificationURL: got %v, want %q", reqBody.NotificationURL, tt.req.NotificationURL)
				}
			}
			if tt.req.SuccessURL != "" {
				if reqBody.SuccessURL == nil || *reqBody.SuccessURL != tt.req.SuccessURL {
					t.Errorf("SuccessURL: got %v, want %q", reqBody.SuccessURL, tt.req.SuccessURL)
				}
			}
			if tt.req.FailURL != "" {
				if reqBody.FailURL == nil || *reqBody.FailURL != tt.req.FailURL {
					t.Errorf("FailURL: got %v, want %q", reqBody.FailURL, tt.req.FailURL)
				}
			}
			if tt.req.Description != "" {
				wantDesc := truncateDescription(tt.req.Description, maxDescriptionLength)
				if reqBody.Description == nil || *reqBody.Description != wantDesc {
					t.Errorf("Description: got %v, want %q", reqBody.Description, wantDesc)
				}
			}

			if reqBody.DATA == nil {
				t.Fatalf("DATA is required for T-Kassa Init")
			}
			common, err := reqBody.DATA.AsCommon()
			if err != nil {
				t.Fatalf("DATA as Common: %v", err)
			}
			if common.OperationInitiatorType == nil || *common.OperationInitiatorType != tt.wantOperationInitiator {
				t.Errorf("DATA.OperationInitiatorType: got %v, want %v", common.OperationInitiatorType, tt.wantOperationInitiator)
			}
		})
	}
}

func TestProviderChargeContract(t *testing.T) {
	paymentID := uuid.New()
	var captured []byte
	var capturedMap map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/Charge" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		captured, capturedMap = captureRequest(t, r)
		// Response fixture from the spec: the Charge 200 schema is inline in
		// openapi.yaml (no generated type), so the JSON is built as a literal
		// matching that schema and its example (/v2/Charge).
		_ = json.NewEncoder(w).Encode(map[string]any{
			"TerminalKey": testTerminalKey,
			"Amount":      10000,
			"OrderId":     paymentID.String(),
			"Success":     true,
			"Status":      "CONFIRMED",
			"PaymentId":   "123",
			"ErrorCode":   "0",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	result, err := p.Charge(context.Background(), application.ChargeRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: "123",
		AmountKopecks:     10000,
		Token:             "rebill-token",
	})
	if err != nil {
		t.Fatalf("Charge failed: %v", err)
	}
	if result.ProviderPaymentID != "123" {
		t.Errorf("ProviderPaymentID: got %q, want %q", result.ProviderPaymentID, "123")
	}
	if result.Status != domain.PaymentStatusSucceeded {
		t.Errorf("Status: got %v, want %v", result.Status, domain.PaymentStatusSucceeded)
	}
	if captured == nil {
		t.Fatalf("request was not captured")
	}

	// The charge amount comes from the original Init call; the Charge body
	// must not carry Amount (see Provider.Charge).
	if _, ok := capturedMap["Amount"]; ok {
		t.Errorf("Charge body must not contain Amount, got %v", capturedMap["Amount"])
	}

	var reqBody spec.Charge
	strictDecodeSpecRequest(t, captured, &reqBody)
	if reqBody.TerminalKey != testTerminalKey {
		t.Errorf("TerminalKey: got %q, want %q", reqBody.TerminalKey, testTerminalKey)
	}
	if reqBody.Token == "" {
		t.Errorf("Token is empty")
	}
	if reqBody.PaymentId != "123" {
		t.Errorf("PaymentId: got %q, want %q", reqBody.PaymentId, "123")
	}
	if reqBody.RebillId != "rebill-token" {
		t.Errorf("RebillId: got %q, want %q", reqBody.RebillId, "rebill-token")
	}
}

func TestProviderGetStateContract(t *testing.T) {
	var captured []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/GetState" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		captured, _ = captureRequest(t, r)
		// Response fixture from the spec: the GetState 200 schema is inline in
		// openapi.yaml (no generated type), so the JSON is built as a literal
		// matching that schema and its example (/v2/GetState).
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Success":     true,
			"ErrorCode":   "0",
			"Message":     "OK",
			"TerminalKey": testTerminalKey,
			"Status":      "AUTHORIZED",
			"PaymentId":   "999",
			"OrderId":     "21050",
			"Params": []map[string]any{
				{"Key": "Route", "Value": "ACQ"},
				{"Key": "Source", "Value": "cards"},
				{"Key": "CreditAmount", "Value": "100000"},
			},
			"Amount": 1230,
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	status, err := p.Status(context.Background(), uuid.New(), "999")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.Status != domain.PaymentStatusPending {
		t.Errorf("Status: got %v, want %v", status.Status, domain.PaymentStatusPending)
	}
	if captured == nil {
		t.Fatalf("request was not captured")
	}

	var reqBody spec.GetState
	strictDecodeSpecRequest(t, captured, &reqBody)
	if reqBody.TerminalKey != testTerminalKey {
		t.Errorf("TerminalKey: got %q, want %q", reqBody.TerminalKey, testTerminalKey)
	}
	if reqBody.Token == "" {
		t.Errorf("Token is empty")
	}
	if reqBody.PaymentId != "999" {
		t.Errorf("PaymentId: got %q, want %q", reqBody.PaymentId, "999")
	}
}

func TestProviderCancelContract(t *testing.T) {
	paymentID := uuid.New()
	providerPaymentID := "cancel-contract-123"
	var captured []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/Cancel" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		captured, _ = captureRequest(t, r)
		// Response fixture from the spec: the generated spec.Cancel2 type is
		// broken for the wire (PaymentId/OriginalAmount/NewAmount are float32,
		// while the real API returns PaymentId as a string), so the JSON is
		// built as a literal matching the Cancel 200 schema and its example in
		// openapi.yaml (/v2/Cancel).
		_ = json.NewEncoder(w).Encode(map[string]any{
			"TerminalKey":       testTerminalKey,
			"OrderId":           paymentID.String(),
			"Success":           true,
			"Status":            "REFUNDED",
			"OriginalAmount":    10000,
			"NewAmount":         0,
			"PaymentId":         providerPaymentID,
			"ErrorCode":         "0",
			"Message":           "OK",
			"Details":           "None",
			"ExternalRequestId": "756478567845678436",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	result, err := p.Cancel(context.Background(), application.CancelRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: providerPaymentID,
		AmountKopecks:     10000,
	})
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	if result.Status != domain.PaymentStatusRefunded {
		t.Errorf("Status: got %v, want %v", result.Status, domain.PaymentStatusRefunded)
	}
	if result.RefundedAmountKopecks != 10000 {
		t.Errorf("RefundedAmountKopecks: got %d, want %d", result.RefundedAmountKopecks, 10000)
	}
	if captured == nil {
		t.Fatalf("request was not captured")
	}

	var reqBody spec.Cancel
	strictDecodeSpecRequest(t, captured, &reqBody)
	if reqBody.TerminalKey != testTerminalKey {
		t.Errorf("TerminalKey: got %q, want %q", reqBody.TerminalKey, testTerminalKey)
	}
	if reqBody.Token == "" {
		t.Errorf("Token is empty")
	}
	if reqBody.PaymentId != providerPaymentID {
		t.Errorf("PaymentId: got %q, want %q", reqBody.PaymentId, providerPaymentID)
	}
	if reqBody.Amount == nil || *reqBody.Amount != 10000 {
		t.Errorf("Amount: got %v, want %v", reqBody.Amount, 10000)
	}
}

func TestProviderAddCustomerContract(t *testing.T) {
	var captured []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/AddCustomer":
			captured, _ = captureRequest(t, r)
			// Response fixture from the spec: generated spec.AddCustomerResponse,
			// shaped after the /v2/AddCustomer example in openapi.yaml.
			_ = json.NewEncoder(w).Encode(spec.AddCustomerResponse{
				TerminalKey: testTerminalKey,
				CustomerKey: "customer-1",
				Success:     true,
				ErrorCode:   "0",
			})
		case "/v2/AddCard":
			_ = json.NewEncoder(w).Encode(spec.AddCardResponse{
				TerminalKey: testTerminalKey,
				CustomerKey: "customer-1",
				RequestKey:  "request-key-1",
				Success:     true,
				ErrorCode:   "0",
				PaymentURL:  "https://securepayments.tinkoff.ru/rest/addcard/abc",
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	result, err := p.InitAddCard(context.Background(), application.InitAddCardRequest{
		UserID:      uuid.New(),
		CustomerKey: "customer-1",
		SuccessURL:  "https://app.example/api/subscription/payment-methods/add-card/success",
		FailURL:     "https://app.example/api/subscription/payment-methods/add-card/fail",
	})
	if err != nil {
		t.Fatalf("InitAddCard failed: %v", err)
	}
	if result.PaymentURL != "https://securepayments.tinkoff.ru/rest/addcard/abc" {
		t.Errorf("PaymentURL: got %q", result.PaymentURL)
	}
	if result.RequestKey != "request-key-1" {
		t.Errorf("RequestKey: got %q, want %q", result.RequestKey, "request-key-1")
	}
	if captured == nil {
		t.Fatalf("AddCustomer request was not captured")
	}

	var reqBody spec.AddCustomer
	strictDecodeSpecRequest(t, captured, &reqBody)
	if reqBody.TerminalKey != testTerminalKey {
		t.Errorf("TerminalKey: got %q, want %q", reqBody.TerminalKey, testTerminalKey)
	}
	if reqBody.Token == "" {
		t.Errorf("Token is empty")
	}
	if reqBody.CustomerKey != "customer-1" {
		t.Errorf("CustomerKey: got %q, want %q", reqBody.CustomerKey, "customer-1")
	}
}

func TestProviderAddCardContract(t *testing.T) {
	const (
		successURL = "https://app.example/api/subscription/payment-methods/add-card/success"
		failURL    = "https://app.example/api/subscription/payment-methods/add-card/fail"
	)
	var capturedMap map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/AddCustomer":
			_ = verifyRequestToken(t, r, testPassword)
			_ = json.NewEncoder(w).Encode(spec.AddCustomerResponse{
				TerminalKey: testTerminalKey,
				CustomerKey: "customer-1",
				Success:     true,
				ErrorCode:   "0",
			})
		case "/v2/AddCard":
			// The server verifies the AddCard token over the schema fields only;
			// RedirectUrl/FailRedirectUrl are outside the schema and excluded.
			_, capturedMap = captureRequest(t, r, addCardExtraFieldsAllowlist...)
			// Response fixture from the spec: generated spec.AddCardResponse,
			// shaped after the /v2/AddCard example in openapi.yaml.
			_ = json.NewEncoder(w).Encode(spec.AddCardResponse{
				TerminalKey: testTerminalKey,
				CustomerKey: "customer-1",
				RequestKey:  "request-key-1",
				Success:     true,
				ErrorCode:   "0",
				PaymentURL:  "https://securepayments.tinkoff.ru/rest/addcard/abc",
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	result, err := p.InitAddCard(context.Background(), application.InitAddCardRequest{
		UserID:      uuid.New(),
		CustomerKey: "customer-1",
		SuccessURL:  successURL,
		FailURL:     failURL,
	})
	if err != nil {
		t.Fatalf("InitAddCard failed: %v", err)
	}
	if result.PaymentURL != "https://securepayments.tinkoff.ru/rest/addcard/abc" {
		t.Errorf("PaymentURL: got %q", result.PaymentURL)
	}
	if result.RequestKey != "request-key-1" {
		t.Errorf("RequestKey: got %q, want %q", result.RequestKey, "request-key-1")
	}
	if capturedMap == nil {
		t.Fatalf("AddCard request was not captured")
	}

	// Strict schema decode with the documented allowlist: RedirectUrl and
	// FailRedirectUrl are asserted for presence and value, then stripped.
	reqBody := strictDecodeAddCardRequest(t, capturedMap, successURL, failURL)
	if reqBody.TerminalKey != testTerminalKey {
		t.Errorf("TerminalKey: got %q, want %q", reqBody.TerminalKey, testTerminalKey)
	}
	if reqBody.Token == "" {
		t.Errorf("Token is empty")
	}
	if reqBody.CustomerKey != "customer-1" {
		t.Errorf("CustomerKey: got %q, want %q", reqBody.CustomerKey, "customer-1")
	}
	if reqBody.CheckType == nil || *reqBody.CheckType != spec.N3DSHOLD {
		t.Errorf("CheckType: got %v, want %v", reqBody.CheckType, spec.N3DSHOLD)
	}

	if _, ok := capturedMap["NotificationURL"]; ok {
		t.Errorf("AddCard must not contain NotificationURL")
	}
}

func TestProviderRemoveCardContract(t *testing.T) {
	var captured []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/RemoveCard" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		captured, _ = captureRequest(t, r)
		// Response fixture from the spec: generated spec.RemoveCardResponse,
		// shaped after the /v2/RemoveCard example in openapi.yaml.
		_ = json.NewEncoder(w).Encode(spec.RemoveCardResponse{
			TerminalKey: testTerminalKey,
			Status:      "D",
			CustomerKey: "customer-1",
			CardId:      "card-1",
			CardType:    0,
			Success:     true,
			ErrorCode:   "0",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	if err := p.RemoveCard(context.Background(), "customer-1", "card-1"); err != nil {
		t.Fatalf("RemoveCard failed: %v", err)
	}
	if captured == nil {
		t.Fatalf("request was not captured")
	}

	var reqBody spec.RemoveCard
	strictDecodeSpecRequest(t, captured, &reqBody)
	if reqBody.TerminalKey != testTerminalKey {
		t.Errorf("TerminalKey: got %q, want %q", reqBody.TerminalKey, testTerminalKey)
	}
	if reqBody.Token == "" {
		t.Errorf("Token is empty")
	}
	if reqBody.CustomerKey != "customer-1" {
		t.Errorf("CustomerKey: got %q, want %q", reqBody.CustomerKey, "customer-1")
	}
	if reqBody.CardId != "card-1" {
		t.Errorf("CardId: got %q, want %q", reqBody.CardId, "card-1")
	}
}

func TestProviderGetCardListContract(t *testing.T) {
	var captured []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/GetCardList" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		captured, _ = captureRequest(t, r)
		// Response fixture from the spec: the GetCardList 200 schema is an
		// inline array in openapi.yaml (no generated type; note the spec
		// example even has a typo, "RebillId'"), so the JSON is built as a
		// literal matching that schema's properties.
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"CardId":   "card-1",
				"Pan":      "518223******0036",
				"Status":   "A",
				"RebillId": "6155312073",
				"CardType": 0,
				"ExpDate":  "1122",
			},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	cards, err := p.GetCardList(context.Background(), "customer-1")
	if err != nil {
		t.Fatalf("GetCardList failed: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
	if cards[0].CardID != "card-1" || cards[0].Pan != "518223******0036" || cards[0].ExpDate != "1122" {
		t.Errorf("unexpected card: %+v", cards[0])
	}
	if cards[0].RebillID != "6155312073" {
		t.Errorf("RebillID: got %q, want %q", cards[0].RebillID, "6155312073")
	}
	if cards[0].Status != application.ProviderCardStatusActive {
		t.Errorf("Status: got %q, want %q", cards[0].Status, application.ProviderCardStatusActive)
	}
	if captured == nil {
		t.Fatalf("request was not captured")
	}

	var reqBody spec.GetCardList
	strictDecodeSpecRequest(t, captured, &reqBody)
	if reqBody.TerminalKey != testTerminalKey {
		t.Errorf("TerminalKey: got %q, want %q", reqBody.TerminalKey, testTerminalKey)
	}
	if reqBody.Token == "" {
		t.Errorf("Token is empty")
	}
	if reqBody.CustomerKey != "customer-1" {
		t.Errorf("CustomerKey: got %q, want %q", reqBody.CustomerKey, "customer-1")
	}
}

func TestProviderGetAddCardStateContract(t *testing.T) {
	t.Run("completed", func(t *testing.T) {
		requestKey := "request-key-contract-1"
		var captured []byte
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v2/GetAddCardState" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			captured, _ = captureRequest(t, r)
			// Response fixture from the spec: generated
			// spec.GetAddCardStateResponse, shaped after the
			// /v2/GetAddCardState 200 schema in openapi.yaml.
			cardID := "card-1"
			rebillID := "rebill-1"
			customerKey := "customer-1"
			errorCode := "0"
			_ = json.NewEncoder(w).Encode(spec.GetAddCardStateResponse{
				TerminalKey: testTerminalKey,
				Success:     true,
				ErrorCode:   &errorCode,
				Status:      spec.GetAddCardStateResponseStatusCOMPLETED,
				RequestKey:  requestKey,
				CardId:      &cardID,
				RebillId:    &rebillID,
				CustomerKey: &customerKey,
			})
		}))
		defer server.Close()

		p := newTestProvider(server.URL + "/v2/")
		state, err := p.GetAddCardState(context.Background(), requestKey)
		if err != nil {
			t.Fatalf("GetAddCardState failed: %v", err)
		}
		if state.Status != application.CardBindingStatusCompleted {
			t.Errorf("Status: got %q, want %q", state.Status, application.CardBindingStatusCompleted)
		}
		if state.CardID != "card-1" {
			t.Errorf("CardID: got %q, want %q", state.CardID, "card-1")
		}
		if state.RebillID != "rebill-1" {
			t.Errorf("RebillID: got %q, want %q", state.RebillID, "rebill-1")
		}
		if state.CustomerKey != "customer-1" {
			t.Errorf("CustomerKey: got %q, want %q", state.CustomerKey, "customer-1")
		}
		if captured == nil {
			t.Fatalf("request was not captured")
		}

		var reqBody spec.GetAddCardState
		strictDecodeSpecRequest(t, captured, &reqBody)
		if reqBody.TerminalKey != testTerminalKey {
			t.Errorf("TerminalKey: got %q, want %q", reqBody.TerminalKey, testTerminalKey)
		}
		if reqBody.Token == "" {
			t.Errorf("Token is empty")
		}
		if reqBody.RequestKey != requestKey {
			t.Errorf("RequestKey: got %q, want %q", reqBody.RequestKey, requestKey)
		}
	})

	t.Run("rejected", func(t *testing.T) {
		requestKey := "request-key-contract-2"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v2/GetAddCardState" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			_ = verifyRequestToken(t, r, testPassword)
			// Response fixture from the spec: a REJECTED binding reports the
			// provider error code in the envelope, not as a request failure.
			errorCode := "7"
			message := "Card declined"
			_ = json.NewEncoder(w).Encode(spec.GetAddCardStateResponse{
				TerminalKey: testTerminalKey,
				Success:     true,
				ErrorCode:   &errorCode,
				Message:     &message,
				Status:      spec.GetAddCardStateResponseStatusREJECTED,
				RequestKey:  requestKey,
			})
		}))
		defer server.Close()

		p := newTestProvider(server.URL + "/v2/")
		state, err := p.GetAddCardState(context.Background(), requestKey)
		if err != nil {
			t.Fatalf("GetAddCardState failed: %v", err)
		}
		if state.Status != application.CardBindingStatusRejected {
			t.Errorf("Status: got %q, want %q", state.Status, application.CardBindingStatusRejected)
		}
		if state.ErrorCode != "7" {
			t.Errorf("ErrorCode: got %q, want %q", state.ErrorCode, "7")
		}
	})

	t.Run("unknown status errors", func(t *testing.T) {
		requestKey := "request-key-contract-3"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v2/GetAddCardState" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			_ = verifyRequestToken(t, r, testPassword)
			errorCode := "0"
			_ = json.NewEncoder(w).Encode(spec.GetAddCardStateResponse{
				TerminalKey: testTerminalKey,
				Success:     true,
				ErrorCode:   &errorCode,
				Status:      spec.GetAddCardStateResponseStatus("TOTALLY_NEW_STATUS"),
				RequestKey:  requestKey,
			})
		}))
		defer server.Close()

		p := newTestProvider(server.URL + "/v2/")
		_, err := p.GetAddCardState(context.Background(), requestKey)
		if err == nil {
			t.Fatal("expected an error for an unknown add card status")
		}
		if !strings.Contains(err.Error(), "unknown add card status") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("provider error code 502 surfaces", func(t *testing.T) {
		requestKey := "request-key-contract-4"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v2/GetAddCardState" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			_ = verifyRequestToken(t, r, testPassword)
			// T-Kassa error 502: no card found for the RequestKey — the binding
			// session expired. It must surface with the provider error code so
			// the sync can classify it.
			errorCode := "502"
			message := "Card not found"
			_ = json.NewEncoder(w).Encode(spec.GetAddCardStateResponse{
				TerminalKey: testTerminalKey,
				Success:     false,
				ErrorCode:   &errorCode,
				Message:     &message,
				Status:      spec.GetAddCardStateResponseStatusNEW,
				RequestKey:  requestKey,
			})
		}))
		defer server.Close()

		p := newTestProvider(server.URL + "/v2/")
		_, err := p.GetAddCardState(context.Background(), requestKey)
		if err == nil {
			t.Fatal("expected a provider error")
		}
		var providerErr *ProviderError
		if !errors.As(err, &providerErr) || providerErr.ErrorCode != "502" {
			t.Fatalf("expected ProviderError with code 502, got %v", err)
		}
	})
}

func TestProviderStatusDecodesRebillID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/GetState" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = verifyRequestToken(t, r, testPassword)
		// The inline GetState 200 schema in openapi.yaml carries RebillId for
		// payments made with saved credentials; the adapter must surface it.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Success":     true,
			"ErrorCode":   "0",
			"Message":     "OK",
			"TerminalKey": testTerminalKey,
			"Status":      "CONFIRMED",
			"PaymentId":   "13660",
			"OrderId":     "21050",
			"Amount":      1230,
			"RebillId":    "rebill-42",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL + "/v2/")
	status, err := p.Status(context.Background(), uuid.New(), "13660")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.RebillID != "rebill-42" {
		t.Fatalf("RebillID: got %q, want %q", status.RebillID, "rebill-42")
	}
}
