package tkassa

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
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
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func newTestProvider(serverURL string) *Provider {
	return NewProvider(serverURL, testTerminalKey, testPassword, 5*time.Second, discardLogger())
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

func TestSignSkipsNullAndNested(t *testing.T) {
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
	// NullValue (nil), DATA and Receipt are skipped; Blank (empty string) is included.
	wantConcat := "1" + "" + "o" + testPassword + "Term"
	hash := sha256.Sum256([]byte(wantConcat))
	want := hex.EncodeToString(hash[:])
	if got != want {
		t.Fatalf("sign mismatch: got %q, want %q", got, want)
	}
}

func TestVerifyWebhookToken(t *testing.T) {
	payload := map[string]any{
		"TerminalKey": testTerminalKey,
		"OrderId":     "order-123",
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
			PaymentID:    123456,
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

func TestProviderInitOperationInitiatorTypeDefault(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = verifyRequestToken(t, r, testPassword)
		_ = json.NewEncoder(w).Encode(initResponse{
			baseResponse: baseResponse{Success: true, Status: "NEW"},
			PaymentID:    1,
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
		if got, want := data["PaymentId"], float64(123); got != want {
			t.Fatalf("PaymentId: got %v, want %v", got, want)
		}
		if got, want := data["RebillId"], "rebill-token"; got != want {
			t.Fatalf("RebillId: got %v, want %v", got, want)
		}

		_ = json.NewEncoder(w).Encode(chargeResponse{
			baseResponse: baseResponse{Success: true, Status: "CONFIRMED"},
			PaymentID:    123,
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
			PaymentID:    999,
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
			if got, want := data["SuccessURL"], "https://example.com/success"; got != want {
				t.Fatalf("SuccessURL: got %v, want %v", got, want)
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
		UserID:          uuid.New(),
		CustomerKey:     "customer-1",
		SuccessURL:      "https://example.com/success",
		NotificationURL: "https://example.com/notify",
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

func TestProviderInitAddCardCustomerAPIErrorContinues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/AddCustomer":
			_ = verifyRequestToken(t, r, testPassword)
			_ = json.NewEncoder(w).Encode(addCustomerResponse{
				baseResponse: baseResponse{
					Success:   false,
					ErrorCode: "8",
					Message:   "Customer already exists",
				},
				CustomerKey: "customer-1",
			})
		case "/v2/AddCard":
			_ = verifyRequestToken(t, r, testPassword)
			_ = json.NewEncoder(w).Encode(addCardResponse{
				baseResponse: baseResponse{Success: true},
				PaymentURL:   "https://pay",
				RequestKey:   "rk",
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
	if result.RequestKey != "rk" {
		t.Fatalf("RequestKey: got %q, want %q", result.RequestKey, "rk")
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

func TestProviderTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(getStateResponse{baseResponse: baseResponse{Success: true}})
	}))
	defer server.Close()

	p := NewProvider(server.URL+"/v2/", testTerminalKey, testPassword, 1*time.Nanosecond, discardLogger())
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
		{"UNKNOWN", domain.PaymentStatusPending},
	}
	for _, tc := range cases {
		if got := mapStatus(tc.in); got != tc.want {
			t.Errorf("mapStatus(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestNewProviderDefaults(t *testing.T) {
	p := NewProvider("", testTerminalKey, testPassword, 0, discardLogger())
	if !strings.HasSuffix(p.baseURL, "/") {
		t.Fatalf("baseURL should have trailing slash: %q", p.baseURL)
	}
	if p.client.Timeout != defaultTimeout {
		t.Fatalf("timeout: got %v, want %v", p.client.Timeout, defaultTimeout)
	}
}

func TestNewProviderTrimsTrailingSlash(t *testing.T) {
	p := NewProvider("https://example.com/rest", testTerminalKey, testPassword, 0, discardLogger())
	if p.baseURL != "https://example.com/rest/" {
		t.Fatalf("baseURL: got %q", p.baseURL)
	}
}

func TestSignDoesNotMutateInput(t *testing.T) {
	data := map[string]any{"TerminalKey": "T", "Amount": int64(1)}
	original := make(map[string]any, len(data))
	for k, v := range data {
		original[k] = v
	}
	_ = sign(data, testPassword)
	if len(data) != len(original) {
		t.Fatalf("sign mutated input map")
	}
}
