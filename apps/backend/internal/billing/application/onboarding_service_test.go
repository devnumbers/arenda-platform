package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// newOnboardingHarness builds an onboarding service over in-memory fakes with
// the three seeded tariffs.
func newOnboardingHarness() (*OnboardingService, *fakeStores) {
	stores := newFakeStores(testTariffs()...)
	svc := NewOnboardingService(stores.factory(nil), OnboardingServiceConfig{Logger: slog.New(slog.DiscardHandler)})
	return svc, stores
}

// testTariffs returns the three canonical tariffs with generated ids.
func testTariffs() []domain.Tariff {
	return []domain.Tariff{
		{ID: uuid.Must(uuid.NewV7()), Name: domain.TariffBasic, ActivePropertyLimit: 1, IsActive: true},
		{
			ID: uuid.Must(uuid.NewV7()), Name: domain.TariffPro, ActivePropertyLimit: 5,
			MonthlyPriceKopecks: 49000, YearlyPriceKopecks: 440000, IsActive: true,
		},
		{
			ID: uuid.Must(uuid.NewV7()), Name: domain.TariffBusiness, ActivePropertyLimit: -1,
			MonthlyPriceKopecks: 99000, YearlyPriceKopecks: 890000, IsActive: true,
		},
	}
}

func basicTariffID(t *testing.T, tariffs []domain.Tariff) uuid.UUID {
	t.Helper()
	for _, tariff := range tariffs {
		if tariff.Name == domain.TariffBasic {
			return tariff.ID
		}
	}
	t.Fatal("test tariffs contain no basic plan")
	return uuid.Nil
}

// TestOnboardingService_OnUserRegistered_CreatesBasicSubscription proves
// registration creates the basic subscription — active, paid source, no expiry,
// auto-renew off — and appends the registration transition in the same
// transaction.
func TestOnboardingService_OnUserRegistered_CreatesBasicSubscription(t *testing.T) {
	svc, stores := newOnboardingHarness()
	userID := uuid.Must(uuid.NewV7())

	if err := svc.OnUserRegistered(t.Context(), userID); err != nil {
		t.Fatalf("OnUserRegistered() error = %v", err)
	}

	sub, err := stores.subscriptions.GetByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("Status = %q, want %q", sub.Status, domain.SubscriptionStatusActive)
	}
	if sub.Source != domain.SubscriptionSourcePaid {
		t.Errorf("Source = %q, want %q", sub.Source, domain.SubscriptionSourcePaid)
	}
	if sub.ValidUntil != nil {
		t.Errorf("ValidUntil = %v, want nil", sub.ValidUntil)
	}
	if sub.AutoRenewEnabled {
		t.Error("AutoRenewEnabled = true, want false")
	}
	if sub.TariffID != basicTariffID(t, stores.tariffs.tariffs) {
		t.Errorf("TariffID = %v, want the basic tariff", sub.TariffID)
	}

	transitions, err := stores.transitions.ListBySubscriptionID(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	if len(transitions) != 1 {
		t.Fatalf("transitions = %d, want 1", len(transitions))
	}
	tr := transitions[0]
	if tr.Reason != domain.TransitionReasonRegistered {
		t.Errorf("Reason = %q, want %q", tr.Reason, domain.TransitionReasonRegistered)
	}
	if tr.Initiator != domain.InitiatorSystem {
		t.Errorf("Initiator = %q, want %q", tr.Initiator, domain.InitiatorSystem)
	}
	if tr.FromStatus != nil {
		t.Errorf("FromStatus = %v, want nil", *tr.FromStatus)
	}
	if tr.ToTariffID != sub.TariffID {
		t.Errorf("ToTariffID = %v, want %v", tr.ToTariffID, sub.TariffID)
	}
}

// TestOnboardingService_OnUserRegistered_IdempotentOnRedelivery proves a
// redelivered registration event leaves the existing subscription untouched and
// does not append a second transition.
func TestOnboardingService_OnUserRegistered_IdempotentOnRedelivery(t *testing.T) {
	svc, stores := newOnboardingHarness()
	userID := uuid.Must(uuid.NewV7())

	if err := svc.OnUserRegistered(t.Context(), userID); err != nil {
		t.Fatalf("first OnUserRegistered() error = %v", err)
	}
	first, err := stores.subscriptions.GetByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}

	if err := svc.OnUserRegistered(t.Context(), userID); err != nil {
		t.Fatalf("second OnUserRegistered() error = %v", err)
	}
	second, err := stores.subscriptions.GetByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("subscription id changed on redelivery: %v -> %v", first.ID, second.ID)
	}

	transitions, err := stores.transitions.ListBySubscriptionID(t.Context(), second.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	if len(transitions) != 1 {
		t.Fatalf("transitions = %d, want 1 (no duplicates on redelivery)", len(transitions))
	}
}

// racingSubscriptionRepo simulates a concurrent delivery winning the create
// race: reads miss (as before the winner commits), but Create returns the
// winner's row.
type racingSubscriptionRepo struct {
	*fakeSubscriptionRepo
	existing domain.Subscription
}

func (r *racingSubscriptionRepo) GetByUserID(context.Context, uuid.UUID) (domain.Subscription, error) {
	return domain.Subscription{}, ErrNotFound
}

func (r *racingSubscriptionRepo) Create(context.Context, domain.Subscription) (domain.Subscription, error) {
	return r.existing, nil
}

// TestOnboardingService_OnUserRegistered_ConcurrentCreateSkipsTransition proves
// that when the repository's create returns an existing row (a concurrent
// delivery won the race), no duplicate transition is appended.
func TestOnboardingService_OnUserRegistered_ConcurrentCreateSkipsTransition(t *testing.T) {
	stores := newFakeStores(testTariffs()...)
	userID := uuid.Must(uuid.NewV7())
	winner, err := domain.NewBasicSubscription(userID, basicTariffID(t, testTariffs()))
	if err != nil {
		t.Fatalf("NewBasicSubscription() error = %v", err)
	}
	racing := &racingSubscriptionRepo{fakeSubscriptionRepo: stores.subscriptions, existing: winner}
	svc := NewOnboardingService(
		NewTxStoreFactory(stores.tariffs, racing, stores.transitions, stores.payments,
			stores.methods, stores.bindings, nil, &fakeUoW{beginner: stores.beginner}),
		OnboardingServiceConfig{Logger: slog.New(slog.DiscardHandler)},
	)

	if err := svc.OnUserRegistered(t.Context(), userID); err != nil {
		t.Fatalf("OnUserRegistered() error = %v", err)
	}
	transitions, err := stores.transitions.ListBySubscriptionID(t.Context(), winner.ID)
	if err != nil {
		t.Fatalf("ListBySubscriptionID() error = %v", err)
	}
	if len(transitions) != 0 {
		t.Fatalf("transitions = %d, want 0 (concurrent winner owns the transition)", len(transitions))
	}
}

// TestOnboardingService_OnUserRegistered_MissingBasicSeedFails proves
// onboarding fails loudly when the basic tariff seed is absent instead of
// creating a subscription with a nil tariff.
func TestOnboardingService_OnUserRegistered_MissingBasicSeedFails(t *testing.T) {
	stores := newFakeStores() // No tariffs seeded.
	svc := NewOnboardingService(stores.factory(nil), OnboardingServiceConfig{Logger: slog.New(slog.DiscardHandler)})

	err := svc.OnUserRegistered(t.Context(), uuid.Must(uuid.NewV7()))
	if err == nil {
		t.Fatal("OnUserRegistered() error = nil, want error for missing seed")
	}
	if !errors.Is(err, ErrTariffNotFound) {
		t.Errorf("err = %v, want wrap of ErrTariffNotFound", err)
	}
}
