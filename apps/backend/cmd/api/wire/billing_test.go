package wire

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	billingdomain "github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/config"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// stubSubscriber is a minimal subscriptionViewer (the billing port consumed
// by the /me enricher glue, ADR 0035) returning canned results.
type stubSubscriber struct {
	view billingapp.SubscriptionView
	err  error
}

func (s stubSubscriber) GetSubscription(_ context.Context, _ uuid.UUID) (billingapp.SubscriptionView, error) {
	return s.view, s.err
}

func TestBillingMeEnricher(t *testing.T) {
	t.Parallel()

	userID := uuid.Must(uuid.NewV7())

	t.Run("nil billing is a noop", func(t *testing.T) {
		t.Parallel()
		enricher := BillingMeEnricher(nil)
		resp := &openapi.MeResponse{}

		err := enricher(t.Context(), userID, resp)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if resp.Subscription != nil {
			t.Fatalf("subscription = %v, want nil (enricher must not populate)", *resp.Subscription)
		}
	})

	t.Run("ErrSubscriptionNotFound returns nil (subscription is optional)", func(t *testing.T) {
		t.Parallel()
		sub := stubSubscriber{err: billingapp.ErrSubscriptionNotFound}
		resp := &openapi.MeResponse{}

		err := BillingMeEnricher(sub)(t.Context(), userID, resp)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if resp.Subscription != nil {
			t.Fatalf("subscription = %v, want nil", *resp.Subscription)
		}
	})

	t.Run("wrapped ErrSubscriptionNotFound still returns nil", func(t *testing.T) {
		t.Parallel()
		sub := stubSubscriber{err: errors.Join(billingapp.ErrSubscriptionNotFound, errors.New("ctx"))}
		resp := &openapi.MeResponse{}

		err := BillingMeEnricher(sub)(t.Context(), userID, resp)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if resp.Subscription != nil {
			t.Fatalf("subscription = %v, want nil", *resp.Subscription)
		}
	})

	t.Run("success populates resp.Subscription", func(t *testing.T) {
		t.Parallel()
		tariff := billingdomain.Tariff{Name: billingdomain.TariffPro, ActivePropertyLimit: 10}
		view := billingapp.SubscriptionView{
			Subscription: billingdomain.Subscription{
				ID:     uuid.Must(uuid.NewV7()),
				UserID: userID,
				Status: billingdomain.SubscriptionStatusActive,
			},
			Tariff: tariff,
		}
		sub := stubSubscriber{view: view}
		resp := &openapi.MeResponse{}

		err := BillingMeEnricher(sub)(t.Context(), userID, resp)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if resp.Subscription == nil {
			t.Fatal("subscription = nil, want populated")
		}
		if resp.Subscription.Tariff.Name != openapi.TariffName(tariff.Name) {
			t.Fatalf("tariff name = %q, want %q", resp.Subscription.Tariff.Name, tariff.Name)
		}
		if resp.Subscription.Status != openapi.SubscriptionStatusActive {
			t.Fatalf("status = %q, want %q", resp.Subscription.Status, openapi.SubscriptionStatusActive)
		}
	})

	t.Run("infrastructure error is propagated", func(t *testing.T) {
		t.Parallel()
		infraErr := errors.New("connection reset")
		sub := stubSubscriber{err: infraErr}
		resp := &openapi.MeResponse{}

		err := BillingMeEnricher(sub)(t.Context(), userID, resp)
		if !errors.Is(err, infraErr) {
			t.Fatalf("err = %v, want %v", err, infraErr)
		}
		if resp.Subscription != nil {
			t.Fatalf("subscription = %v, want nil on error", *resp.Subscription)
		}
	})
}

// TestWirePaymentProvider pins the single-active-provider selection (issue
// #248, ADR 0038): the configuration picks exactly one adapter behind the
// neutral port, and a provider endpoint that was not configured explicitly is
// a startup error — the test URL is never a default.
func TestWirePaymentProvider(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)
	clk := clock.Real{}
	var metrics *payment.Metrics // nil-safe: RecordRequest guards nil

	t.Run("fake", func(t *testing.T) {
		t.Parallel()
		provider, err := wirePaymentProvider(&config.Config{
			PaymentProvider: "fake",
			AppBaseURL:      "http://localhost:8080",
		}, log, clk, metrics)
		if err != nil {
			t.Fatalf("wirePaymentProvider: %v", err)
		}
		if got, want := provider.Name(), billingdomain.PaymentProvider("fake"); got != want {
			t.Fatalf("Name: got %q, want %q", got, want)
		}
	})

	t.Run("tkassa", func(t *testing.T) {
		t.Parallel()
		provider, err := wirePaymentProvider(&config.Config{
			PaymentProvider:   "tkassa",
			AppBaseURL:        "https://app.example",
			TKassaBaseURL:     "https://rest-api-test.tinkoff.ru/v2/",
			TKassaTerminalKey: "term",
			TKassaPassword:    "pass",
			TKassaTimeout:     30 * time.Second,
		}, log, clk, metrics)
		if err != nil {
			t.Fatalf("wirePaymentProvider: %v", err)
		}
		if got, want := provider.Name(), billingdomain.PaymentProvider("tkassa"); got != want {
			t.Fatalf("Name: got %q, want %q", got, want)
		}
	})

	t.Run("tkassa without explicit base URL fails with actionable error", func(t *testing.T) {
		t.Parallel()
		_, err := wirePaymentProvider(&config.Config{
			PaymentProvider:   "tkassa",
			AppBaseURL:        "https://app.example",
			TKassaTerminalKey: "term",
			TKassaPassword:    "pass",
		}, log, clk, metrics)
		if err == nil {
			t.Fatal("expected error for missing provider base URL")
		}
		if !strings.Contains(err.Error(), "T_KASSA_BASE_URL") {
			t.Fatalf("error must name T_KASSA_BASE_URL, got: %v", err)
		}
	})

	t.Run("unknown provider fails", func(t *testing.T) {
		t.Parallel()
		_, err := wirePaymentProvider(&config.Config{PaymentProvider: "mir"}, log, clk, metrics)
		if err == nil {
			t.Fatal("expected error for unknown provider")
		}
	})
}
