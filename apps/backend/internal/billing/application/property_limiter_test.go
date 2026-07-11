package application

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

func TestSubscriptionPropertyLimiterActivePropertyLimit(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	tariffID := uuid.New()
	finiteTariff := domain.Tariff{ID: tariffID, Name: domain.TariffPro, ActivePropertyLimit: 5}

	unlimitedTariffID := uuid.New()
	unlimitedTariff := domain.Tariff{ID: unlimitedTariffID, Name: domain.TariffBusiness, ActivePropertyLimit: domain.UnlimitedPropertyLimit}

	errBoom := errors.New("tariff repo boom")

	validUntilFuture := time.Now().Add(time.Hour)
	validUntilPast := time.Now().Add(-time.Hour)

	activeSub := func(tID uuid.UUID) domain.Subscription {
		return domain.Subscription{UserID: userID, TariffID: tID, Status: domain.SubscriptionStatusActive}
	}

	tests := []struct {
		name          string
		subscriptions map[uuid.UUID]domain.Subscription
		tariffs       *fakeTariffRepo
		viaTx         bool
		wantLimit     int
		wantErr       bool
		wantErrIs     error
	}{
		{
			name:          "no subscription returns zero",
			subscriptions: map[uuid.UUID]domain.Subscription{},
			tariffs:       &fakeTariffRepo{byName: map[domain.TariffName]domain.Tariff{}},
			wantLimit:     0,
		},
		{
			name: "inactive status returns zero",
			subscriptions: map[uuid.UUID]domain.Subscription{
				userID: {UserID: userID, TariffID: tariffID, Status: domain.SubscriptionStatusCancelled},
			},
			tariffs:   &fakeTariffRepo{byName: map[domain.TariffName]domain.Tariff{finiteTariff.Name: finiteTariff}},
			wantLimit: 0,
		},
		{
			name: "cancelled with future valid_until returns limit",
			subscriptions: map[uuid.UUID]domain.Subscription{
				userID: {UserID: userID, TariffID: tariffID, Status: domain.SubscriptionStatusCancelled, ValidUntil: &validUntilFuture},
			},
			tariffs:   &fakeTariffRepo{byName: map[domain.TariffName]domain.Tariff{finiteTariff.Name: finiteTariff}},
			wantLimit: 5,
		},
		{
			name: "cancelled with expired valid_until returns zero",
			subscriptions: map[uuid.UUID]domain.Subscription{
				userID: {UserID: userID, TariffID: tariffID, Status: domain.SubscriptionStatusCancelled, ValidUntil: &validUntilPast},
			},
			tariffs:   &fakeTariffRepo{byName: map[domain.TariffName]domain.Tariff{finiteTariff.Name: finiteTariff}},
			wantLimit: 0,
		},
		{
			name: "cancelled unlimited with future valid_until returns max int32",
			subscriptions: map[uuid.UUID]domain.Subscription{
				userID: {UserID: userID, TariffID: unlimitedTariffID, Status: domain.SubscriptionStatusCancelled, ValidUntil: &validUntilFuture},
			},
			tariffs:   &fakeTariffRepo{byName: map[domain.TariffName]domain.Tariff{unlimitedTariff.Name: unlimitedTariff}},
			wantLimit: math.MaxInt32,
		},
		{
			name: "active with finite limit returns limit",
			subscriptions: map[uuid.UUID]domain.Subscription{
				userID: activeSub(tariffID),
			},
			tariffs:   &fakeTariffRepo{byName: map[domain.TariffName]domain.Tariff{finiteTariff.Name: finiteTariff}},
			wantLimit: 5,
		},
		{
			name: "active with finite limit via locked transaction returns limit",
			subscriptions: map[uuid.UUID]domain.Subscription{
				userID: activeSub(tariffID),
			},
			tariffs:   &fakeTariffRepo{byName: map[domain.TariffName]domain.Tariff{finiteTariff.Name: finiteTariff}},
			viaTx:     true,
			wantLimit: 5,
		},
		{
			name: "active with unlimited limit returns max int32",
			subscriptions: map[uuid.UUID]domain.Subscription{
				userID: activeSub(unlimitedTariffID),
			},
			tariffs:   &fakeTariffRepo{byName: map[domain.TariffName]domain.Tariff{unlimitedTariff.Name: unlimitedTariff}},
			wantLimit: math.MaxInt32,
		},
		{
			name: "tariff repository error is wrapped",
			subscriptions: map[uuid.UUID]domain.Subscription{
				userID: activeSub(tariffID),
			},
			tariffs: &fakeTariffRepo{
				byName:         map[domain.TariffName]domain.Tariff{finiteTariff.Name: finiteTariff},
				failGetByIDFor: &tariffID,
				failGetByIDErr: errBoom,
			},
			wantErr:   true,
			wantErrIs: errBoom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			subs := &fakeSubscriptionRepo{subs: tt.subscriptions}
			limiter := NewSubscriptionPropertyLimiter(subs, tt.tariffs)

			var active PropertyLimiter = limiter
			if tt.viaTx {
				locked, err := limiter.WithTx(nil)
				if err != nil {
					t.Fatalf("WithTx returned error: %v", err)
				}
				active = locked
			}

			got, err := active.ActivePropertyLimit(context.Background(), userID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("expected error wrapping %v, got %v", tt.wantErrIs, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantLimit {
				t.Fatalf("expected limit %d, got %d", tt.wantLimit, got)
			}
		})
	}
}
