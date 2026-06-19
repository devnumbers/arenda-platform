package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewPaymentMethod(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	pm, err := NewPaymentMethod(userID, ProviderFake, "fake_token_123", "*1234", now)
	if err != nil {
		t.Fatalf("NewPaymentMethod() error = %v", err)
	}

	if pm.UserID != userID {
		t.Errorf("UserID = %v, want %v", pm.UserID, userID)
	}
	if pm.Provider != ProviderFake {
		t.Errorf("Provider = %v, want %v", pm.Provider, ProviderFake)
	}
	if pm.ProviderToken != "fake_token_123" {
		t.Errorf("ProviderToken = %v, want fake_token_123", pm.ProviderToken)
	}
	if pm.DisplayMask != "*1234" {
		t.Errorf("DisplayMask = %v, want *1234", pm.DisplayMask)
	}
	if pm.IsActive {
		t.Error("new payment method should not be active")
	}
	if pm.ID == uuid.Nil {
		t.Error("ID must be set")
	}
}

func TestPaymentMethodActivate(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	pm, _ := NewPaymentMethod(userID, ProviderFake, "fake_token_123", "*1234", now)

	activateAt := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	pm.Activate(activateAt)
	if !pm.IsActive {
		t.Error("Activate() should set IsActive to true")
	}
	if !pm.UpdatedAt.Equal(activateAt) {
		t.Errorf("Activate() updated_at = %v, want %v", pm.UpdatedAt, activateAt)
	}

	deactivateAt := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	pm.Deactivate(deactivateAt)
	if pm.IsActive {
		t.Error("Deactivate() should set IsActive to false")
	}
	if !pm.UpdatedAt.Equal(deactivateAt) {
		t.Errorf("Deactivate() updated_at = %v, want %v", pm.UpdatedAt, deactivateAt)
	}
}
