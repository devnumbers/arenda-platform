package tkassa

import (
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
	expected := sign(data, password)
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
			"OperationInitiatorType": firstPaymentInitiatorType,
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
		if got, want := data["Recurrent"], recurrentYes; got != want {
			t.Fatalf("Recurrent: got %v, want %v", got, want)
		}
		if got, want := data["PayType"], payTypeOneStage; got != want {
			t.Fatalf("PayType: got %v, want %v", got, want)
		}
		dataObj, ok := data["DATA"].(map[string]any)
		if !ok {
			t.Fatalf("DATA missing or not object")
		}
		if got, want := dataObj["OperationInitiatorType"], firstPaymentInitiatorType; got != want {
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
	if got, want := dataObj["OperationInitiatorType"], firstPaymentInitiatorType; got != want {
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
		OperationInitiatorType: renewalInitiatorType,
	})

	if _, ok := captured["Recurrent"]; ok {
		t.Fatal("expected Recurrent field omitted for renewal")
	}
	dataObj := captured["DATA"].(map[string]any)
	if got, want := dataObj["OperationInitiatorType"], renewalInitiatorType; got != want {
		t.Fatalf("renewal OperationInitiatorType: got %v, want %v", got, want)
	}
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
	if status != domain.PaymentStatusPending {
		t.Fatalf("Status: got %v, want %v", status, domain.PaymentStatusPending)
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
			data := verifyRequestToken(t, r, testPassword)
			if got, want := data["CheckType"], "3DSHOLD"; got != want {
				t.Fatalf("CheckType: got %v, want %v", got, want)
			}
			if _, ok := data["SuccessURL"]; ok {
				t.Fatalf("AddCard must not contain SuccessURL")
			}
			if _, ok := data["FailURL"]; ok {
				t.Fatalf("AddCard must not contain FailURL")
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
		"Status":           "SUCCESS",
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
		"Status":           "FAILED",
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
		"Status":           "SUCCESS",
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
