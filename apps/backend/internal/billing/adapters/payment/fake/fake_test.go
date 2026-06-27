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

func TestProviderInit(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()))
	paymentID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	res, err := p.Init(context.Background(), application.InitRequest{
		PaymentID:     paymentID,
		AmountKopecks: 10000,
		Period:        domain.PeriodMonth,
		UserID:        userID,
		Description:   "Test subscription",
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	if res.Status != domain.PaymentStatusPending {
		t.Errorf("expected status %q, got %q", domain.PaymentStatusPending, res.Status)
	}
	if !strings.HasPrefix(res.ProviderPaymentID, "fake_") {
		t.Errorf("expected provider payment id to start with fake_, got %q", res.ProviderPaymentID)
	}
	if !strings.HasPrefix(res.SavedToken, "fake_token_") {
		t.Errorf("expected saved token to start with fake_token_, got %q", res.SavedToken)
	}
	wantURL := "http://localhost:8080/internal/fake-subscription-payment/" + paymentID.String() + "/confirm"
	if res.PaymentURL != wantURL {
		t.Errorf("expected payment URL %q, got %q", wantURL, res.PaymentURL)
	}
}

func TestProviderInit_IdempotentByInternalPaymentID(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()))
	paymentID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	req := application.InitRequest{
		PaymentID:     paymentID,
		AmountKopecks: 10000,
		Period:        domain.PeriodMonth,
		UserID:        userID,
		Description:   "Test subscription",
	}

	first, err := p.Init(context.Background(), req)
	if err != nil {
		t.Fatalf("first Init error: %v", err)
	}

	second, err := p.Init(context.Background(), req)
	if err != nil {
		t.Fatalf("second Init error: %v", err)
	}

	if first.ProviderPaymentID != second.ProviderPaymentID {
		t.Errorf("expected same provider payment id, got %q and %q", first.ProviderPaymentID, second.ProviderPaymentID)
	}
	if first.PaymentURL != second.PaymentURL {
		t.Errorf("expected same confirm URL, got %q and %q", first.PaymentURL, second.PaymentURL)
	}
	if first.SavedToken != second.SavedToken {
		t.Errorf("expected same saved token, got %q and %q", first.SavedToken, second.SavedToken)
	}
}

func TestProviderInitValidation(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()))
	validPaymentID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	validUserID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	cases := []struct {
		name    string
		req     application.InitRequest
		wantErr string
	}{
		{
			name: "nil payment id",
			req: application.InitRequest{
				PaymentID:     uuid.Nil,
				AmountKopecks: 10000,
				Period:        domain.PeriodMonth,
				UserID:        validUserID,
			},
			wantErr: "payment id is required",
		},
		{
			name: "zero amount",
			req: application.InitRequest{
				PaymentID:     validPaymentID,
				AmountKopecks: 0,
				Period:        domain.PeriodMonth,
				UserID:        validUserID,
			},
			wantErr: "amount must be positive",
		},
		{
			name: "negative amount",
			req: application.InitRequest{
				PaymentID:     validPaymentID,
				AmountKopecks: -1,
				Period:        domain.PeriodMonth,
				UserID:        validUserID,
			},
			wantErr: "amount must be positive",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.Init(context.Background(), tc.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error to contain %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestProviderCharge(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()))
	paymentID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	success, err := p.Charge(context.Background(), application.ChargeRequest{
		PaymentID:     paymentID,
		AmountKopecks: 10000,
		Token:         "fake_token_normal",
	})
	if err != nil {
		t.Fatalf("Charge success error: %v", err)
	}
	if success.Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected succeeded status, got %q", success.Status)
	}

	failed, err := p.Charge(context.Background(), application.ChargeRequest{
		PaymentID:     paymentID,
		AmountKopecks: 10000,
		Token:         "fake_fail_token",
	})
	if err != nil {
		t.Fatalf("Charge failed error: %v", err)
	}
	if failed.Status != domain.PaymentStatusFailed {
		t.Errorf("expected failed status, got %q", failed.Status)
	}
}

func TestProviderCharge_FullRefund(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()))
	paymentID := uuid.MustParse("33333333-3333-3333-3333-333333333334")
	amount := int64(15000)

	chargeRes, err := p.Charge(context.Background(), application.ChargeRequest{
		PaymentID:     paymentID,
		AmountKopecks: amount,
		Token:         "fake_token_normal",
	})
	if err != nil {
		t.Fatalf("Charge error: %v", err)
	}
	if chargeRes.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("expected succeeded status, got %q", chargeRes.Status)
	}

	cancelRes, err := p.Cancel(context.Background(), application.CancelRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: chargeRes.ProviderPaymentID,
		AmountKopecks:     0,
	})
	if err != nil {
		t.Fatalf("Cancel error: %v", err)
	}
	if cancelRes.Status != domain.PaymentStatusRefunded {
		t.Errorf("expected status %q, got %q", domain.PaymentStatusRefunded, cancelRes.Status)
	}
	if cancelRes.RefundedAmountKopecks != amount {
		t.Errorf("expected refunded amount %d, got %d", amount, cancelRes.RefundedAmountKopecks)
	}
}

func TestProviderChargeValidation(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()))
	validPaymentID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	cases := []struct {
		name    string
		req     application.ChargeRequest
		wantErr string
	}{
		{
			name: "nil payment id",
			req: application.ChargeRequest{
				PaymentID:     uuid.Nil,
				AmountKopecks: 10000,
				Token:         "fake_token_normal",
			},
			wantErr: "payment id is required",
		},
		{
			name: "zero amount",
			req: application.ChargeRequest{
				PaymentID:     validPaymentID,
				AmountKopecks: 0,
				Token:         "fake_token_normal",
			},
			wantErr: "amount must be positive",
		},
		{
			name: "negative amount",
			req: application.ChargeRequest{
				PaymentID:     validPaymentID,
				AmountKopecks: -1,
				Token:         "fake_token_normal",
			},
			wantErr: "amount must be positive",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.Charge(context.Background(), tc.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error to contain %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestProviderParseWebhook(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()))
	internalID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	cases := []struct {
		name        string
		status      string
		errorCode   string
		wantErr     bool
		wantStatus  domain.PaymentStatus
		wantErrCode bool
	}{
		{"succeeded", "succeeded", "", false, domain.PaymentStatusSucceeded, false},
		{"failed with code", "failed", "card_declined", false, domain.PaymentStatusFailed, true},
		{"failed default code", "failed", "", false, domain.PaymentStatusFailed, true},
		{"invalid status", "unknown", "", true, "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]string{
				"provider_payment_id": "fake_abc",
				"internal_payment_id": internalID.String(),
				"status":              tc.status,
			}
			if tc.errorCode != "" {
				payload["error_code"] = tc.errorCode
			}
			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}

			wh, err := p.ParseWebhook(context.Background(), body)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseWebhook error: %v", err)
			}
			if wh.ProviderPaymentID != "fake_abc" {
				t.Errorf("expected provider payment id %q, got %q", "fake_abc", wh.ProviderPaymentID)
			}
			if wh.InternalPaymentID != internalID {
				t.Errorf("expected internal payment id %q, got %q", internalID, wh.InternalPaymentID)
			}
			if wh.Status != tc.wantStatus {
				t.Errorf("expected status %q, got %q", tc.wantStatus, wh.Status)
			}
			if tc.wantErrCode && wh.ErrorCode == nil {
				t.Errorf("expected error code, got nil")
			}
			if !tc.wantErrCode && wh.ErrorCode != nil {
				t.Errorf("expected nil error code, got %q", *wh.ErrorCode)
			}
			if tc.errorCode != "" && wh.ErrorCode != nil && *wh.ErrorCode != tc.errorCode {
				t.Errorf("expected error code %q, got %q", tc.errorCode, *wh.ErrorCode)
			}
		})
	}
}

func TestProviderConfirmPayment(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()))
	paymentID := uuid.MustParse("55555555-5555-5555-5555-555555555555")

	initRes, err := p.Init(context.Background(), application.InitRequest{
		PaymentID:     paymentID,
		AmountKopecks: 10000,
		Period:        domain.PeriodYear,
		UserID:        uuid.MustParse("66666666-6666-6666-6666-666666666666"),
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	wh, err := p.ConfirmPayment(context.Background(), paymentID.String())
	if err != nil {
		t.Fatalf("ConfirmPayment error: %v", err)
	}
	if wh.ProviderPaymentID != initRes.ProviderPaymentID {
		t.Errorf("expected provider payment id %q, got %q", initRes.ProviderPaymentID, wh.ProviderPaymentID)
	}
	if wh.InternalPaymentID != paymentID {
		t.Errorf("expected internal payment id %q, got %q", paymentID, wh.InternalPaymentID)
	}
	if wh.Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected status %q, got %q", domain.PaymentStatusSucceeded, wh.Status)
	}
	if wh.ErrorCode != nil {
		t.Errorf("expected nil error code, got %q", *wh.ErrorCode)
	}

	_, err = p.ConfirmPayment(context.Background(), paymentID.String())
	if err == nil {
		t.Fatal("expected error for already confirmed internal payment id")
	}

	_, err = p.ConfirmPayment(context.Background(), "fake_unknown")
	if err == nil {
		t.Fatal("expected error for unknown internal payment id")
	}
}

func TestProviderConfirmPaymentFailed(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()))
	paymentID := uuid.MustParse("77777777-7777-7777-7777-777777777777")

	_, err := p.Init(context.Background(), application.InitRequest{
		PaymentID:     paymentID,
		AmountKopecks: 20000,
		Period:        domain.PeriodMonth,
		UserID:        uuid.MustParse("88888888-8888-8888-8888-888888888888"),
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	customCode := "insufficient_funds"
	wh, err := p.ConfirmPaymentFailed(context.Background(), paymentID.String(), &customCode)
	if err != nil {
		t.Fatalf("ConfirmPaymentFailed error: %v", err)
	}
	if wh.Status != domain.PaymentStatusFailed {
		t.Errorf("expected status %q, got %q", domain.PaymentStatusFailed, wh.Status)
	}
	if wh.ErrorCode == nil {
		t.Fatal("expected error code, got nil")
	}
	if *wh.ErrorCode != customCode {
		t.Errorf("expected error code %q, got %q", customCode, *wh.ErrorCode)
	}

	// Default error code when nil.
	paymentID2 := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	_, err = p.Init(context.Background(), application.InitRequest{
		PaymentID:     paymentID2,
		AmountKopecks: 30000,
		Period:        domain.PeriodYear,
		UserID:        uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}
	wh2, err := p.ConfirmPaymentFailed(context.Background(), paymentID2.String(), nil)
	if err != nil {
		t.Fatalf("ConfirmPaymentFailed error: %v", err)
	}
	if wh2.ErrorCode == nil || *wh2.ErrorCode != defaultErrorCode {
		t.Errorf("expected default error code %q, got %v", defaultErrorCode, wh2.ErrorCode)
	}
}

func TestProviderConfirmPaymentIgnoresErrorCodeOnSuccess(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()))
	paymentID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	_, err := p.Init(context.Background(), application.InitRequest{
		PaymentID:     paymentID,
		AmountKopecks: 10000,
		Period:        domain.PeriodMonth,
		UserID:        uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	wh, err := p.ConfirmPayment(context.Background(), paymentID.String())
	if err != nil {
		t.Fatalf("ConfirmPayment error: %v", err)
	}
	if wh.Status != domain.PaymentStatusSucceeded {
		t.Errorf("expected status %q, got %q", domain.PaymentStatusSucceeded, wh.Status)
	}
	if wh.ErrorCode != nil {
		t.Errorf("expected nil error code on success, got %q", *wh.ErrorCode)
	}
}

func TestProviderPurgePendingTTL(t *testing.T) {
	clk := newTestClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	p := NewProvider("http://localhost:8080", discardLogger(), clk)
	paymentID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")

	_, err := p.Init(context.Background(), application.InitRequest{
		PaymentID:     paymentID,
		AmountKopecks: 10000,
		Period:        domain.PeriodMonth,
		UserID:        uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"),
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	// Move time past the TTL and trigger a purge via Init.
	clk.Add(pendingTTL + time.Second)
	_, err = p.Init(context.Background(), application.InitRequest{
		PaymentID:     uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff"),
		AmountKopecks: 20000,
		Period:        domain.PeriodYear,
		UserID:        uuid.MustParse("11111111-1111-1111-1111-111111111112"),
	})
	if err != nil {
		t.Fatalf("second Init error: %v", err)
	}

	_, err = p.ConfirmPayment(context.Background(), paymentID.String())
	if err == nil {
		t.Fatal("expected error for purged payment")
	}
}

func TestProviderCancel(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()))
	paymentID := uuid.MustParse("11111111-1111-1111-1111-111111111113")
	providerPaymentID := "fake_cancel_1"
	amount := int64(10000)

	if _, err := p.Init(context.Background(), application.InitRequest{
		PaymentID:     paymentID,
		AmountKopecks: amount,
		Period:        domain.PeriodMonth,
		UserID:        uuid.MustParse("22222222-2222-2222-2222-222222222222"),
	}); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	if _, err := p.ConfirmPayment(context.Background(), paymentID.String()); err != nil {
		t.Fatalf("ConfirmPayment error: %v", err)
	}

	full, err := p.Cancel(context.Background(), application.CancelRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: providerPaymentID,
		AmountKopecks:     0,
	})
	if err != nil {
		t.Fatalf("Cancel full refund error: %v", err)
	}
	if full.Status != domain.PaymentStatusRefunded {
		t.Errorf("expected status %q for full refund, got %q", domain.PaymentStatusRefunded, full.Status)
	}
	if full.RefundedAmountKopecks != amount {
		t.Errorf("expected refunded amount %d for full refund, got %d", amount, full.RefundedAmountKopecks)
	}
	if full.ProviderPaymentID != providerPaymentID {
		t.Errorf("expected provider payment id %q, got %q", providerPaymentID, full.ProviderPaymentID)
	}

	partialAmount := int64(3000)
	partial, err := p.Cancel(context.Background(), application.CancelRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: providerPaymentID,
		AmountKopecks:     partialAmount,
	})
	if err != nil {
		t.Fatalf("Cancel partial refund error: %v", err)
	}
	if partial.Status != domain.PaymentStatusPartialRefunded {
		t.Errorf("expected status %q for partial refund, got %q", domain.PaymentStatusPartialRefunded, partial.Status)
	}
	if partial.RefundedAmountKopecks != partialAmount {
		t.Errorf("expected refunded amount %d for partial refund, got %d", partialAmount, partial.RefundedAmountKopecks)
	}
}

func TestProviderCancelValidation(t *testing.T) {
	p := NewProvider("http://localhost:8080", discardLogger(), newTestClock(time.Now()))

	cases := []struct {
		name    string
		req     application.CancelRequest
		wantErr string
	}{
		{
			name: "nil payment id",
			req: application.CancelRequest{
				PaymentID:         uuid.Nil,
				ProviderPaymentID: "fake_cancel_1",
				AmountKopecks:     0,
			},
			wantErr: "payment id is required",
		},
		{
			name: "empty provider payment id",
			req: application.CancelRequest{
				PaymentID:         uuid.MustParse("11111111-1111-1111-1111-111111111113"),
				ProviderPaymentID: "",
				AmountKopecks:     0,
			},
			wantErr: "provider payment id is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.Cancel(context.Background(), tc.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error to contain %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}
