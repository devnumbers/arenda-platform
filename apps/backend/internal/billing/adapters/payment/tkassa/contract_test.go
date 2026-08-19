package tkassa

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/tkassa/spec"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// The contract tests pin the exact shape of every request the adapter sends:
// fields, token, and deliberate absences, strict-decoded into the vendored
// spec types (ADR 0016). A drift in either direction — a field the spec does
// not know, or a spec field the adapter stopped sending where it must not —
// fails here before it reaches the provider.

// initContractCase is one row of the Init contract table.
type initContractCase struct {
	name                   string
	req                    application.InitPaymentRequest
	wantOperationInitiator spec.CommonOperationInitiatorType
	wantRecurrent          bool
}

func TestProviderInitPaymentContract(t *testing.T) {
	tests := []initContractCase{
		{
			name: "first_payment_save_method",
			req: application.InitPaymentRequest{
				PaymentID:     uuid.MustParse(testPaymentUUID),
				AmountKopecks: 10000,
				Period:        domain.PeriodMonth,
				CustomerRef:   testCustomerRef,
				Purpose: application.PaymentPurpose{
					Kind:       application.PaymentPurposeSubscription,
					TariffName: domain.TariffPro,
					Period:     domain.PeriodMonth,
				},
				SaveMethod:   true,
				Initiator:    application.InitiatorCustomer,
				FormDeadline: time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC),
			},
			wantOperationInitiator: spec.CommonOperationInitiatorTypeN1, // CIT CC per the spec.
			wantRecurrent:          true,
		},
		{
			name: "renewal",
			req: application.InitPaymentRequest{
				PaymentID:     uuid.Must(uuid.NewV7()),
				AmountKopecks: 500,
				Period:        domain.PeriodMonth,
				CustomerRef:   "customer-2",
				Purpose: application.PaymentPurpose{
					Kind:       application.PaymentPurposeRenewal,
					TariffName: domain.TariffPro,
					Period:     domain.PeriodMonth,
				},
				SaveMethod: false,
				Initiator:  application.InitiatorMerchant,
			},
			wantOperationInitiator: spec.CommonOperationInitiatorTypeR, // MIT COF Recurring per the spec.
			wantRecurrent:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paymentURL := testPaymentURL
			var captured []byte
			var capturedMap map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v2/Init" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				captured, capturedMap = captureRequest(t, r)
				// Response fixture from the spec: the Init 200 response is the
				// generated spec.InitResponse type, shaped after the example in
				// openapi.yaml (/v2/Init).
				writeJSON(t, w, spec.InitResponse{
					TerminalKey: testTerminalKey,
					Amount:      tt.req.AmountKopecks,
					OrderId:     tt.req.PaymentID.String(),
					Success:     true,
					Status:      statusNew,
					PaymentId:   "123456",
					ErrorCode:   "0",
					PaymentURL:  &paymentURL,
				})
			}))
			defer server.Close()

			p := newTestProvider(server.URL)
			result, err := p.InitPayment(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("InitPayment failed: %v", err)
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

			assertInitMapBody(t, capturedMap, tt)

			var reqBody spec.InitRequest
			strictDecodeSpecRequest(t, captured, &reqBody)
			assertInitSpecFields(t, reqBody, tt)
			assertInitSpecCallbacksAndPurpose(t, reqBody, tt.req)
			assertInitSpecData(t, reqBody, tt.wantOperationInitiator)
		})
	}
}

// assertInitMapBody checks the raw Init body at map level: Recurrent must be
// present only for the parent (save-method) payment, RedirectDueDate rides
// along only when set (the user-facing parent payment carries the payment-form
// deadline, renewals must not), and DATA carries the initiator type.
func assertInitMapBody(t *testing.T, capturedMap map[string]any, tt initContractCase) {
	t.Helper()
	if tt.wantRecurrent {
		if got, want := capturedMap["Recurrent"], any(string(spec.Y)); got != want {
			t.Errorf("Recurrent: got %v, want %q", got, want)
		}
	} else if _, ok := capturedMap["Recurrent"]; ok {
		t.Errorf("Recurrent must be omitted for merchant-initiated Init, got %v", capturedMap["Recurrent"])
	}
	if !tt.req.FormDeadline.IsZero() {
		raw, ok := capturedMap["RedirectDueDate"].(string)
		if !ok {
			t.Fatalf("RedirectDueDate missing or not a string")
		}
		if _, err := time.Parse(time.RFC3339, raw); err != nil {
			t.Errorf("RedirectDueDate is not RFC3339: %q: %v", raw, err)
		}
		if want := tt.req.FormDeadline.UTC().Format(time.RFC3339); raw != want {
			t.Errorf("RedirectDueDate: got %q, want %q", raw, want)
		}
	} else if _, ok := capturedMap["RedirectDueDate"]; ok {
		t.Errorf("RedirectDueDate must be omitted for renewal Init, got %v", capturedMap["RedirectDueDate"])
	}
	dataObj, ok := capturedMap["DATA"].(map[string]any)
	if !ok {
		t.Fatalf("DATA missing or not an object")
	}
	if got, want := dataObj["OperationInitiatorType"], any(string(tt.wantOperationInitiator)); got != want {
		t.Errorf("DATA.OperationInitiatorType: got %v, want %q", got, want)
	}
}

// assertInitSpecFields checks the strict-decoded spec.InitRequest core fields:
// terminal, token, order id, amount, pay type, customer key, and the spec-level
// Recurrent/RedirectDueDate presence.
func assertInitSpecFields(t *testing.T, reqBody spec.InitRequest, tt initContractCase) {
	t.Helper()
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
	if reqBody.CustomerKey == nil || *reqBody.CustomerKey != tt.req.CustomerRef {
		t.Errorf("CustomerKey: got %v, want %q", reqBody.CustomerKey, tt.req.CustomerRef)
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
	if !tt.req.FormDeadline.IsZero() {
		if reqBody.RedirectDueDate == nil {
			t.Fatalf("spec.InitRequest.RedirectDueDate missing")
		}
		if want := tt.req.FormDeadline.UTC(); !reqBody.RedirectDueDate.Equal(want) {
			t.Errorf("spec.InitRequest.RedirectDueDate: got %v, want %v", reqBody.RedirectDueDate, want)
		}
	}
}

// assertInitSpecCallbacksAndPurpose checks the callback URLs (built from the
// configured app base URL, issue #248) and the rendered description truncated
// to the provider's rune limit.
func assertInitSpecCallbacksAndPurpose(t *testing.T, reqBody spec.InitRequest, req application.InitPaymentRequest) {
	t.Helper()
	if reqBody.NotificationURL == nil || *reqBody.NotificationURL != testAppBaseURL+"/webhooks/payment/tkassa" {
		t.Errorf("NotificationURL: got %v", reqBody.NotificationURL)
	}
	wantSuccess := testAppBaseURL + "/subscription/payments/" + req.PaymentID.String() + "/success"
	if reqBody.SuccessURL == nil || *reqBody.SuccessURL != wantSuccess {
		t.Errorf("SuccessURL: got %v, want %q", reqBody.SuccessURL, wantSuccess)
	}
	wantFail := testAppBaseURL + "/subscription/payments/" + req.PaymentID.String() + "/fail"
	if reqBody.FailURL == nil || *reqBody.FailURL != wantFail {
		t.Errorf("FailURL: got %v, want %q", reqBody.FailURL, wantFail)
	}

	wantDesc := truncateDescription(paymentDescription(req.Purpose))
	if reqBody.Description == nil || *reqBody.Description != wantDesc {
		t.Errorf("Description: got %v, want %q", reqBody.Description, wantDesc)
	}
}

// assertInitSpecData checks the strict-decoded DATA object carries the
// expected operation initiator type.
func assertInitSpecData(t *testing.T, reqBody spec.InitRequest, wantInitiator spec.CommonOperationInitiatorType) {
	t.Helper()
	if reqBody.DATA == nil {
		t.Fatalf("DATA is required for T-Kassa Init")
	}
	common, err := reqBody.DATA.AsCommon()
	if err != nil {
		t.Fatalf("DATA as Common: %v", err)
	}
	if common.OperationInitiatorType == nil || *common.OperationInitiatorType != wantInitiator {
		t.Errorf("DATA.OperationInitiatorType: got %v, want %v", common.OperationInitiatorType, wantInitiator)
	}
}

func TestProviderChargePaymentContract(t *testing.T) {
	paymentID := uuid.Must(uuid.NewV7())
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
		writeJSON(t, w, map[string]any{
			fieldTerminalKey: testTerminalKey,
			"Amount":         10000,
			fieldOrderID:     paymentID.String(),
			fieldSuccess:     true,
			fieldStatus:      statusConfirmed,
			fieldPaymentID:   "123",
			fieldErrorCode:   "0",
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
		t.Errorf("ProviderPaymentID: got %q, want %q", result.ProviderPaymentID, "123")
	}
	if result.Status != domain.PaymentStatusSucceeded {
		t.Errorf("Status: got %v, want %v", result.Status, domain.PaymentStatusSucceeded)
	}
	if result.ErrorCode != "" {
		t.Errorf("ErrorCode: got %q, want empty on success", result.ErrorCode)
	}
	if captured == nil {
		t.Fatalf("request was not captured")
	}

	// The charge amount comes from the original Init call; the Charge body
	// must not carry Amount (see Provider.ChargePayment).
	if _, ok := capturedMap["Amount"]; ok {
		t.Errorf("Charge body must not contain Amount, got %v", capturedMap["Amount"])
	}

	var reqBody spec.ChargeRequest
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
	if reqBody.RebillId != testChargeToken {
		t.Errorf("RebillId: got %q, want %q", reqBody.RebillId, testChargeToken)
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
		writeJSON(t, w, map[string]any{
			fieldSuccess:     true,
			fieldErrorCode:   "0",
			"Message":        "OK",
			fieldTerminalKey: testTerminalKey,
			fieldStatus:      statusAuthorized,
			fieldPaymentID:   "999",
			fieldOrderID:     "21050",
			"Params": []map[string]any{
				{fieldKey: "Route", fieldValue: "ACQ"},
				{fieldKey: "Source", fieldValue: "cards"},
				{fieldKey: "CreditAmount", fieldValue: "100000"},
			},
			"Amount": 1230,
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	status, err := p.PaymentStatus(context.Background(), uuid.Must(uuid.NewV7()), "999")
	if err != nil {
		t.Fatalf("PaymentStatus failed: %v", err)
	}
	if status.Status != domain.PaymentStatusPending {
		t.Errorf("Status: got %v, want %v", status.Status, domain.PaymentStatusPending)
	}
	if status.ErrorCode != "" {
		t.Errorf("ErrorCode: got %q, want empty on success", status.ErrorCode)
	}
	if captured == nil {
		t.Fatalf("request was not captured")
	}

	var reqBody spec.GetStateRequest
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
	paymentID := uuid.Must(uuid.NewV7())
	providerPaymentID := "cancel-contract-123"
	var captured []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/Cancel" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		captured, _ = captureRequest(t, r)
		// Response fixture from the spec: the Cancel 200 schema carries
		// float32 amounts in the generated type, so the JSON is built as a
		// literal matching the Cancel 200 schema and its example in
		// openapi.yaml (/v2/Cancel).
		writeJSON(t, w, map[string]any{
			fieldTerminalKey:    testTerminalKey,
			fieldOrderID:        paymentID.String(),
			fieldSuccess:        true,
			fieldStatus:         statusRefunded,
			"OriginalAmount":    10000,
			"NewAmount":         0,
			fieldPaymentID:      providerPaymentID,
			fieldErrorCode:      "0",
			"Message":           "OK",
			"Details":           "None",
			"ExternalRequestId": "756478567845678436",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	result, err := p.RefundPayment(context.Background(), application.RefundRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: providerPaymentID,
		AmountKopecks:     10000,
	})
	if err != nil {
		t.Fatalf("RefundPayment failed: %v", err)
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

	var reqBody spec.CancelRequest
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
	// ExternalRequestId is the documented refund idempotency key and carries
	// the internal payment id (issue #246 §2.3).
	if reqBody.ExternalRequestId == nil || *reqBody.ExternalRequestId != paymentID.String() {
		t.Errorf("ExternalRequestId: got %v, want %q", reqBody.ExternalRequestId, paymentID.String())
	}
}

func TestProviderAddCustomerAddCardContract(t *testing.T) {
	var addCustomerCaptured []byte
	var addCardCapturedMap map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/" + methodAddCustomer:
			addCustomerCaptured, _ = captureRequest(t, r)
			// Response fixture from the spec: generated
			// spec.AddCustomerResponse, shaped after the /v2/AddCustomer
			// example in openapi.yaml.
			writeJSON(t, w, spec.AddCustomerResponse{
				TerminalKey: testTerminalKey,
				CustomerKey: testCustomerRef,
				Success:     true,
				ErrorCode:   "0",
			})
		case "/v2/" + methodAddCard:
			_, addCardCapturedMap = captureRequest(t, r, addCardExtraFieldsAllowlist...)
			writeJSON(t, w, spec.AddCardResponse{
				TerminalKey: testTerminalKey,
				CustomerKey: testCustomerRef,
				RequestKey:  testRequestKey,
				Success:     true,
				ErrorCode:   "0",
				PaymentURL:  testAddCardURL,
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	result, err := p.BindPaymentMethod(context.Background(), application.BindMethodRequest{
		CustomerRef: testCustomerRef,
	})
	if err != nil {
		t.Fatalf("BindPaymentMethod failed: %v", err)
	}
	if result.FormURL != testAddCardURL {
		t.Errorf("FormURL: got %q", result.FormURL)
	}
	if result.BindingID != testRequestKey {
		t.Errorf("BindingID: got %q, want %q", result.BindingID, testRequestKey)
	}

	var addCustomerBody spec.AddCustomerRequest
	strictDecodeSpecRequest(t, addCustomerCaptured, &addCustomerBody)
	if addCustomerBody.TerminalKey != testTerminalKey {
		t.Errorf("AddCustomer TerminalKey: got %q, want %q", addCustomerBody.TerminalKey, testTerminalKey)
	}
	if addCustomerBody.Token == "" {
		t.Errorf("AddCustomer Token is empty")
	}
	if addCustomerBody.CustomerKey != testCustomerRef {
		t.Errorf("AddCustomer CustomerKey: got %q, want %q", addCustomerBody.CustomerKey, testCustomerRef)
	}

	// The AddCard extras are outside the spec schema (undocumented per-request
	// redirect/notification URLs, ADR 0017): assert them at map level, strip
	// them, then strict-decode the remaining body into spec.AddCardRequest.
	wantExtras := map[string]string{
		fieldRedirectURL:       testAppBaseURL + bindingReturnSuccessPath,
		fieldFailRedirectURL:   testAppBaseURL + bindingReturnFailPath,
		fieldSuccessAddCardURL: testAppBaseURL + bindingReturnSuccessPath,
		fieldFailAddCardURL:    testAppBaseURL + bindingReturnFailPath,
		fieldNotificationURL:   testAppBaseURL + notificationPath,
	}
	for field, want := range wantExtras {
		if got := addCardCapturedMap[field]; got != want {
			t.Fatalf("AddCard extra %s: got %v, want %v", field, got, want)
		}
	}
	stripped := maps.Clone(addCardCapturedMap)
	for _, field := range addCardExtraFieldsAllowlist {
		delete(stripped, field)
	}
	strippedBody, err := json.Marshal(stripped)
	if err != nil {
		t.Fatalf("marshal stripped AddCard body: %v", err)
	}
	var addCardBody spec.AddCardRequest
	strictDecodeSpecRequest(t, strippedBody, &addCardBody)
	if addCardBody.TerminalKey != testTerminalKey {
		t.Errorf("AddCard TerminalKey: got %q, want %q", addCardBody.TerminalKey, testTerminalKey)
	}
	if addCardBody.Token == "" {
		t.Errorf("AddCard Token is empty")
	}
	if addCardBody.CustomerKey != testCustomerRef {
		t.Errorf("AddCard CustomerKey: got %q, want %q", addCardBody.CustomerKey, testCustomerRef)
	}
	if addCardBody.CheckType == nil || *addCardBody.CheckType != spec.N3DSHOLD {
		t.Errorf("AddCard CheckType: got %v, want %v", addCardBody.CheckType, spec.N3DSHOLD)
	}
}

func TestProviderRemoveCardContract(t *testing.T) {
	var captured []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/RemoveCard" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		captured, _ = captureRequest(t, r)
		writeJSON(t, w, spec.RemoveCardResponse{
			TerminalKey: testTerminalKey,
			CustomerKey: testCustomerRef,
			CardId:      testCardID,
			Success:     true,
			ErrorCode:   "0",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	if err := p.RemovePaymentMethod(context.Background(), testCustomerRef, testCardID); err != nil {
		t.Fatalf("RemovePaymentMethod failed: %v", err)
	}

	var reqBody spec.RemoveCardRequest
	strictDecodeSpecRequest(t, captured, &reqBody)
	if reqBody.TerminalKey != testTerminalKey {
		t.Errorf("TerminalKey: got %q, want %q", reqBody.TerminalKey, testTerminalKey)
	}
	if reqBody.Token == "" {
		t.Errorf("Token is empty")
	}
	if reqBody.CustomerKey != testCustomerRef {
		t.Errorf("CustomerKey: got %q, want %q", reqBody.CustomerKey, testCustomerRef)
	}
	if reqBody.CardId != testCardID {
		t.Errorf("CardId: got %q, want %q", reqBody.CardId, testCardID)
	}
}

func TestProviderGetCardListContract(t *testing.T) {
	var captured []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/GetCardList" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		captured, _ = captureRequest(t, r)
		// Response fixture from the spec: the GetCardList 200 body is a bare
		// array of cards (oneOf[array, ErrorResponse] in 1.27).
		writeJSON(t, w, []map[string]any{
			{
				fieldPan:     testMaskedPan,
				fieldExpDate: "1230",
				"CardId":     testCardID,
				"RebillId":   testRebillID,
				fieldStatus:  "A",
				"IsDefault":  false,
			},
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	methods, err := p.ListPaymentMethods(context.Background(), testCustomerRef)
	if err != nil {
		t.Fatalf("ListPaymentMethods failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("methods: got %d, want 1", len(methods))
	}
	if captured == nil {
		t.Fatalf("request was not captured")
	}

	var reqBody spec.GetCardListRequest
	strictDecodeSpecRequest(t, captured, &reqBody)
	if reqBody.TerminalKey != testTerminalKey {
		t.Errorf("TerminalKey: got %q, want %q", reqBody.TerminalKey, testTerminalKey)
	}
	if reqBody.Token == "" {
		t.Errorf("Token is empty")
	}
	if reqBody.CustomerKey != testCustomerRef {
		t.Errorf("CustomerKey: got %q, want %q", reqBody.CustomerKey, testCustomerRef)
	}
}

func TestProviderGetAddCardStateContract(t *testing.T) {
	var captured []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/GetAddCardState" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		captured, _ = captureRequest(t, r)
		// Response fixture from the spec: generated
		// spec.GetAddCardStateResponse, shaped after the /v2/GetAddCardState
		// example in openapi.yaml.
		errorCode := "0"
		cardID := testCardID
		rebillID := testRebillID
		customerKey := testCustomerRef
		writeJSON(t, w, spec.GetAddCardStateResponse{
			TerminalKey: testTerminalKey,
			CustomerKey: &customerKey,
			Success:     true,
			ErrorCode:   &errorCode,
			Status:      spec.GetAddCardStateResponseStatusCOMPLETED,
			RequestKey:  "request-key-contract-1",
			CardId:      &cardID,
			RebillId:    &rebillID,
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	state, err := p.PaymentMethodBinding(context.Background(), "request-key-contract-1")
	if err != nil {
		t.Fatalf("PaymentMethodBinding failed: %v", err)
	}
	if state.Status != application.MethodBindingCompleted {
		t.Errorf("Status: got %q, want %q", state.Status, application.MethodBindingCompleted)
	}
	if state.Method == nil || state.Method.ProviderMethodID != testCardID || state.Method.ChargeToken != testRebillID {
		t.Errorf("Method: got %+v", state.Method)
	}

	var reqBody spec.GetAddCardStateRequest
	strictDecodeSpecRequest(t, captured, &reqBody)
	if reqBody.TerminalKey != testTerminalKey {
		t.Errorf("TerminalKey: got %q, want %q", reqBody.TerminalKey, testTerminalKey)
	}
	if reqBody.Token == "" {
		t.Errorf("Token is empty")
	}
	if reqBody.RequestKey != "request-key-contract-1" {
		t.Errorf("RequestKey: got %q, want %q", reqBody.RequestKey, "request-key-contract-1")
	}
}

// TestClassifyProviderError pins the error-code catalogue classification
// against the officially documented meanings (issue #246 §5).
func TestClassifyProviderError(t *testing.T) {
	tests := []struct {
		name string
		code string
		want error
	}{
		{"charge blocked", "10", application.ErrProviderChargeBlocked},
		{"auth rejected 204", "204", application.ErrProviderAuthRejected},
		{"auth rejected 205", "205", application.ErrProviderAuthRejected},
		{"payment not found", "255", application.ErrProviderPaymentNotFound},
		{"redirect url empty", "9", application.ErrProviderInvalidOperation},
		{"invalid redirect due date", "12", application.ErrProviderInvalidOperation},
		{"invalid operation 1125", "1125", application.ErrProviderInvalidOperation},
		{"invalid operation 1126", "1126", application.ErrProviderInvalidOperation},
		{"insufficient funds 103", "103", application.ErrProviderInsufficientFunds},
		{"insufficient funds 116", "116", application.ErrProviderInsufficientFunds},
		{"insufficient funds 1051", "1051", application.ErrProviderInsufficientFunds},
		{"recurring failed", "104", application.ErrProviderRecurringFailed},
		{"saved method expired", "262", application.ErrProviderSavedMethodExpired},
		{"duplicate operation", "100", application.ErrProviderDuplicateOperation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := classifyProviderError(&ProviderError{Method: methodCharge, ErrorCode: tt.code})
			if !errors.Is(err, tt.want) {
				t.Fatalf("classify code %s: got %v, want %v", tt.code, err, tt.want)
			}
		})
	}

	t.Run("unknown code returns unchanged", func(t *testing.T) {
		providerErr := &ProviderError{Method: methodCharge, ErrorCode: "9999"}
		if got := classifyProviderError(providerErr); !errors.Is(got, providerErr) || got.Error() != providerErr.Error() {
			t.Fatalf("expected unchanged error, got %v", got)
		}
	})
}
