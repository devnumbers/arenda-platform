package application

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// TestTariffService_ListTariffs_ReturnsActiveOnly proves the user-facing tariff
// listing hides inactive tariffs.
func TestTariffService_ListTariffs_ReturnsActiveOnly(t *testing.T) {
	t.Parallel()
	hidden := domain.Tariff{ID: uuid.Must(uuid.NewV7()), Name: domain.TariffBusiness, ActivePropertyLimit: -1, IsActive: false}
	stores := newFakeStores(append(testTariffs(), hidden)...)
	svc := NewTariffService(stores.factory(nil), TariffServiceConfig{})

	tariffs, err := svc.ListTariffs(t.Context())
	if err != nil {
		t.Fatalf("ListTariffs() error = %v", err)
	}
	if len(tariffs) != 3 {
		t.Fatalf("tariffs = %d, want 3 (hidden tariff excluded)", len(tariffs))
	}
	for _, tariff := range tariffs {
		if !tariff.IsActive {
			t.Errorf("tariff %q returned while inactive", tariff.Name)
		}
	}
}

// TestTariffService_ListAllTariffs_ReturnsHiddenToo proves the admin tariff
// listing includes hidden tariffs (issue #247).
func TestTariffService_ListAllTariffs_ReturnsHiddenToo(t *testing.T) {
	t.Parallel()
	hidden := domain.Tariff{ID: uuid.Must(uuid.NewV7()), Name: domain.TariffBusiness, ActivePropertyLimit: -1, IsActive: false}
	seeded := append(testTariffs(), hidden)
	stores := newFakeStores(seeded...)
	svc := NewTariffService(stores.factory(nil), TariffServiceConfig{})

	tariffs, err := svc.ListAllTariffs(t.Context())
	if err != nil {
		t.Fatalf("ListAllTariffs() error = %v", err)
	}
	if len(tariffs) != len(seeded) {
		t.Fatalf("tariffs = %d, want %d (hidden tariff included)", len(tariffs), len(seeded))
	}
	foundHidden := false
	for _, tariff := range tariffs {
		if tariff.ID == hidden.ID {
			foundHidden = true
			if tariff.IsActive {
				t.Error("hidden tariff returned with IsActive=true, want false")
			}
		}
	}
	if !foundHidden {
		t.Error("hidden tariff missing from ListAllTariffs result")
	}
}

// TestSubscriptionService_GetSubscription_AssemblesView proves GetSubscription
// resolves the current and pending tariffs into the view.
func TestSubscriptionService_GetSubscription_AssemblesView(t *testing.T) {
	t.Parallel()
	tariffs := testTariffs()
	stores := newFakeStores(tariffs...)
	svc := NewSubscriptionService(stores.factory(nil), SubscriptionServiceConfig{})
	userID := uuid.Must(uuid.NewV7())

	sub, err := domain.NewBasicSubscription(userID, tariffs[0].ID)
	if err != nil {
		t.Fatalf("NewBasicSubscription() error = %v", err)
	}
	if _, err := stores.subscriptions.Create(t.Context(), sub); err != nil {
		t.Fatalf("seed Create() error = %v", err)
	}

	view, err := svc.GetSubscription(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetSubscription() error = %v", err)
	}
	if view.Subscription.ID != sub.ID {
		t.Errorf("Subscription.ID = %v, want %v", view.Subscription.ID, sub.ID)
	}
	if view.Tariff.ID != tariffs[0].ID || view.Tariff.Name != domain.TariffBasic {
		t.Errorf("Tariff = %+v, want the basic tariff", view.Tariff)
	}
	if view.PendingTariff != nil {
		t.Errorf("PendingTariff = %+v, want nil", *view.PendingTariff)
	}
}

// TestSubscriptionService_GetSubscription_NotFound proves a user without a
// subscription maps to ErrSubscriptionNotFound.
func TestSubscriptionService_GetSubscription_NotFound(t *testing.T) {
	t.Parallel()
	svc := NewSubscriptionService(newFakeStores().factory(nil), SubscriptionServiceConfig{})

	_, err := svc.GetSubscription(t.Context(), uuid.Must(uuid.NewV7()))
	if !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("err = %v, want ErrSubscriptionNotFound", err)
	}
}
