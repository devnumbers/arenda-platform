package http

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// newPaidGate builds the paid-sections gate adapter over an in-memory
// subscription service whose tariff repo answers the given plan name (the
// readonly-gate builder pins basic; this one parameterises it, карта #997).
func newPaidGate(sub domain.Subscription, subErr error, tariffName domain.TariffName) *PaidSectionsGate {
	tariff := domain.Tariff{ID: sub.TariffID, Name: tariffName, ActivePropertyLimit: 1, IsActive: true}
	factory := billingapp.NewTxStoreFactory(
		&gateTariffRepo{tariff: tariff},
		&gateSubscriptionRepo{sub: sub, err: subErr},
		&gateTransitionRepo{},
		&gatePaymentRepo{},
		nil,
		nil,
		nil,
		nil,
	)
	return NewPaidSectionsGate(billingapp.NewSubscriptionService(factory, billingapp.SubscriptionServiceConfig{}))
}

func paidGateSubscription(status domain.SubscriptionStatus) domain.Subscription {
	return domain.Subscription{
		ID:         uuid.Must(uuid.NewV7()),
		UserID:     uuid.Must(uuid.NewV7()),
		TariffID:   uuid.Must(uuid.NewV7()),
		Source:     domain.SubscriptionSourcePaid,
		Status:     status,
		ValidUntil: nil,
	}
}

// TestPaidSectionsGate_TariffDecides proves the acceptance rule of карта #997:
// the current tariff name alone decides — grace and cancelled subscriptions
// keep their paid tariff name until the downgrade is actually applied, so
// they pass; basic blocks; a missing subscription counts as basic.
func TestPaidSectionsGate_TariffDecides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     domain.SubscriptionStatus
		tariffName domain.TariffName
		want       bool
	}{
		{"basic active blocks", domain.SubscriptionStatusActive, domain.TariffBasic, false},
		{"pro passes", domain.SubscriptionStatusActive, domain.TariffPro, true},
		{"business passes", domain.SubscriptionStatusActive, domain.TariffBusiness, true},
		{"pro grace passes", domain.SubscriptionStatusGrace, domain.TariffPro, true},
		{"pro cancelled passes", domain.SubscriptionStatusCancelled, domain.TariffPro, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gate := newPaidGate(paidGateSubscription(tt.status), nil, tt.tariffName)
			got, err := gate.CanUsePaidFeatures(t.Context(), paidGateSubscription(tt.status).UserID)
			if err != nil {
				t.Fatalf("CanUsePaidFeatures() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("CanUsePaidFeatures() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPaidSectionsGate_NoSubscriptionIsBasic proves the null-subscription
// rule: without a subscription the user counts as basic — the gate blocks
// (the readonly gate treats the same absence as mutable; here the paid
// sections stay closed).
func TestPaidSectionsGate_NoSubscriptionIsBasic(t *testing.T) {
	t.Parallel()

	userID := uuid.Must(uuid.NewV7())
	gate := newPaidGate(domain.Subscription{}, billingapp.ErrSubscriptionNotFound, domain.TariffBasic)

	got, err := gate.CanUsePaidFeatures(t.Context(), userID)
	if err != nil {
		t.Fatalf("CanUsePaidFeatures() error = %v", err)
	}
	if got {
		t.Error("CanUsePaidFeatures() = true, want false without a subscription")
	}
}

// TestPaidSectionsGate_StoreErrorPropagates proves the adapter is not a
// fail-open: a store failure surfaces as an error (the middleware answers
// 500, never 402-by-accident or pass-through).
func TestPaidSectionsGate_StoreErrorPropagates(t *testing.T) {
	t.Parallel()

	gate := newPaidGate(domain.Subscription{}, errors.New("db down"), domain.TariffPro)

	if _, err := gate.CanUsePaidFeatures(t.Context(), uuid.Must(uuid.NewV7())); err == nil {
		t.Fatal("CanUsePaidFeatures() error = nil, want the store error")
	}
}

// Compile-time guard that time is genuinely irrelevant to the gate: the
// adapter must not consult the clock even for a grace subscription whose
// window has expired — the tariff name is the only input (карта #997).
func TestPaidSectionsGate_GraceBeyondWindowStillPassesOnPaidTariff(t *testing.T) {
	t.Parallel()

	sub := paidGateSubscription(domain.SubscriptionStatusGrace)
	expired := time.Now().Add(-time.Hour)
	sub.ValidUntil = &expired

	gate := newPaidGate(sub, nil, domain.TariffPro)

	got, err := gate.CanUsePaidFeatures(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("CanUsePaidFeatures() error = %v", err)
	}
	if !got {
		t.Error("CanUsePaidFeatures() = false, want true: the paid tariff name stands until the downgrade is applied")
	}
}
