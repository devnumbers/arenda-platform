package wire

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	billingdomain "github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// stubSubscriber is a minimal billingapp.Subscriber whose only exercised
// method in these tests is GetSubscription; the rest return their zero values.
type stubSubscriber struct {
	view billingapp.SubscriptionView
	err  error
}

func (s stubSubscriber) GetSubscription(_ context.Context, _ uuid.UUID) (billingapp.SubscriptionView, error) {
	return s.view, s.err
}

func (s stubSubscriber) ChangeTariff(_ context.Context, _ uuid.UUID, _ billingapp.ChangeTariffRequest) (billingapp.ChangeTariffResponse, error) {
	return billingapp.ChangeTariffResponse{}, nil
}

func (s stubSubscriber) CancelSubscription(_ context.Context, _ uuid.UUID) error { return nil }

func (s stubSubscriber) ToggleAutoRenew(_ context.Context, _ uuid.UUID, _ bool) error { return nil }

var _ billingapp.Subscriber = stubSubscriber{}

func TestBillingMeEnricher(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

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
				ID:     uuid.New(),
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
