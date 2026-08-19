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

// failingTariffRepo delegates to a base repo but fails GetByID on demand.
type failingTariffRepo struct {
	*fakeTariffRepo
	failErr error
}

func (r *failingTariffRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Tariff, error) {
	if r.failErr != nil {
		return domain.Tariff{}, r.failErr
	}
	return r.fakeTariffRepo.GetByID(ctx, id)
}

// TestSubscriptionPropertyLimiter_ActivePropertyLimit proves the limit follows
// the subscription status: active (or cancelled within the paid period) keeps
// the tariff limit, anything else is zero; unlimited tariffs are MaxInt32.
func TestSubscriptionPropertyLimiter_ActivePropertyLimit(t *testing.T) {
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	clk := fakeClock{now: now}

	finiteTariff := domain.Tariff{ID: uuid.Must(uuid.NewV7()), Name: domain.TariffPro, ActivePropertyLimit: 5, IsActive: true}
	unlimitedTariff := domain.Tariff{
		ID: uuid.Must(uuid.NewV7()), Name: domain.TariffBusiness,
		ActivePropertyLimit: domain.UnlimitedPropertyLimit, IsActive: true,
	}

	validUntilFuture := now.Add(time.Hour)
	validUntilPast := now.Add(-time.Hour)

	tests := []struct {
		name      string
		sub       *domain.Subscription
		tariffs   []domain.Tariff
		viaTx     bool
		wantLimit int
	}{
		{
			name:      "no subscription returns zero",
			sub:       nil,
			tariffs:   []domain.Tariff{finiteTariff},
			wantLimit: 0,
		},
		{
			name: "cancelled without valid_until returns zero",
			sub: &domain.Subscription{
				UserID: uuid.Must(uuid.NewV7()), TariffID: finiteTariff.ID, Status: domain.SubscriptionStatusCancelled,
			},
			tariffs:   []domain.Tariff{finiteTariff},
			wantLimit: 0,
		},
		{
			name: "cancelled with future valid_until returns limit",
			sub: &domain.Subscription{
				UserID: uuid.Must(uuid.NewV7()), TariffID: finiteTariff.ID, Status: domain.SubscriptionStatusCancelled, ValidUntil: &validUntilFuture,
			},
			tariffs:   []domain.Tariff{finiteTariff},
			wantLimit: 5,
		},
		{
			name: "cancelled with expired valid_until returns zero",
			sub: &domain.Subscription{
				UserID: uuid.Must(uuid.NewV7()), TariffID: finiteTariff.ID, Status: domain.SubscriptionStatusCancelled, ValidUntil: &validUntilPast,
			},
			tariffs:   []domain.Tariff{finiteTariff},
			wantLimit: 0,
		},
		{
			name: "cancelled unlimited with future valid_until returns max int32",
			sub: &domain.Subscription{
				UserID: uuid.Must(uuid.NewV7()), TariffID: unlimitedTariff.ID,
				Status: domain.SubscriptionStatusCancelled, ValidUntil: &validUntilFuture,
			},
			tariffs:   []domain.Tariff{unlimitedTariff},
			wantLimit: math.MaxInt32,
		},
		{
			name: "active with finite limit returns limit",
			sub: &domain.Subscription{
				UserID: uuid.Must(uuid.NewV7()), TariffID: finiteTariff.ID, Status: domain.SubscriptionStatusActive,
			},
			tariffs:   []domain.Tariff{finiteTariff},
			wantLimit: 5,
		},
		{
			name: "active with finite limit via locked transaction returns limit",
			sub: &domain.Subscription{
				UserID: uuid.Must(uuid.NewV7()), TariffID: finiteTariff.ID, Status: domain.SubscriptionStatusActive,
			},
			tariffs:   []domain.Tariff{finiteTariff},
			viaTx:     true,
			wantLimit: 5,
		},
		{
			name: "active with unlimited limit returns max int32",
			sub: &domain.Subscription{
				UserID: uuid.Must(uuid.NewV7()), TariffID: unlimitedTariff.ID, Status: domain.SubscriptionStatusActive,
			},
			tariffs:   []domain.Tariff{unlimitedTariff},
			wantLimit: math.MaxInt32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stores := newFakeStores(tt.tariffs...)
			limiter := NewSubscriptionPropertyLimiter(stores.subscriptions, stores.tariffs, clk)

			userID := uuid.Must(uuid.NewV7())
			if tt.sub != nil {
				userID = tt.sub.UserID
				if _, err := stores.subscriptions.Create(context.Background(), *tt.sub); err != nil {
					t.Fatalf("seed Create() error: %v", err)
				}
			}

			active := limiter
			if tt.viaTx {
				locked, err := limiter.WithTx(nil)
				if err != nil {
					t.Fatalf("WithTx returned error: %v", err)
				}
				active = locked
			}

			got, err := active.ActivePropertyLimit(context.Background(), userID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantLimit {
				t.Fatalf("expected limit %d, got %d", tt.wantLimit, got)
			}
			if tt.viaTx && stores.subscriptions.forUpdateCalls == 0 {
				t.Error("WithTx-bound limiter did not read with FOR UPDATE")
			}
		})
	}
}

// TestSubscriptionPropertyLimiter_TariffErrorIsWrapped proves a tariff lookup
// failure surfaces as an error rather than silently allowing zero.
func TestSubscriptionPropertyLimiter_TariffErrorIsWrapped(t *testing.T) {
	errBoom := errors.New("tariff repo boom")
	stores := newFakeStores()
	sub := domain.Subscription{UserID: uuid.Must(uuid.NewV7()), TariffID: uuid.Must(uuid.NewV7()), Status: domain.SubscriptionStatusActive}
	if _, err := stores.subscriptions.Create(context.Background(), sub); err != nil {
		t.Fatalf("seed Create() error: %v", err)
	}
	limiter := NewSubscriptionPropertyLimiter(
		stores.subscriptions,
		&failingTariffRepo{fakeTariffRepo: stores.tariffs, failErr: errBoom},
		fakeClock{now: time.Now()},
	)

	_, err := limiter.ActivePropertyLimit(context.Background(), sub.UserID)
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected error wrapping %v, got %v", errBoom, err)
	}
}

// TestDefaultConfig pins the operational defaults carried over from the
// pre-rewrite module (ADR 0008, issue #244).
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.GraceDuration != 7*24*time.Hour {
		t.Errorf("GraceDuration = %v, want 7d", cfg.GraceDuration)
	}
	if cfg.PaymentFormTTL != 15*time.Minute {
		t.Errorf("PaymentFormTTL = %v, want 15m", cfg.PaymentFormTTL)
	}
	if cfg.ChargeAttemptLimit != 3 {
		t.Errorf("ChargeAttemptLimit = %d, want 3", cfg.ChargeAttemptLimit)
	}
	if cfg.WorkerBatchSize != 100 {
		t.Errorf("WorkerBatchSize = %d, want 100", cfg.WorkerBatchSize)
	}
	if cfg.CardBindingTTL != 24*time.Hour {
		t.Errorf("CardBindingTTL = %v, want 24h", cfg.CardBindingTTL)
	}
	if cfg.PendingPaymentStaleness != 5*time.Minute {
		t.Errorf("PendingPaymentStaleness = %v, want 5m", cfg.PendingPaymentStaleness)
	}
}
