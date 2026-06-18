package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewOwnerSubscription(t *testing.T) {
	userID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")

	sub, err := NewOwnerSubscription(userID, tariffID)
	if err != nil {
		t.Fatalf("NewOwnerSubscription() error = %v", err)
	}

	if sub.UserID != userID {
		t.Errorf("UserID = %v, want %v", sub.UserID, userID)
	}
	if sub.TariffID != tariffID {
		t.Errorf("TariffID = %v, want %v", sub.TariffID, tariffID)
	}
	if sub.Source != SubscriptionSourcePaid {
		t.Errorf("Source = %v, want %v", sub.Source, SubscriptionSourcePaid)
	}
	if sub.Status != SubscriptionStatusActive {
		t.Errorf("Status = %v, want %v", sub.Status, SubscriptionStatusActive)
	}
	if sub.AutoRenewEnabled {
		t.Errorf("AutoRenewEnabled = true, want false")
	}
	if sub.ValidUntil != nil {
		t.Errorf("ValidUntil = %v, want nil", sub.ValidUntil)
	}
}

func TestSubscriptionCanMutateData(t *testing.T) {
	tests := []struct {
		name   string
		status SubscriptionStatus
		want   bool
	}{
		{"active", SubscriptionStatusActive, true},
		{"grace", SubscriptionStatusGrace, true},
		{"blocked", SubscriptionStatusBlocked, false},
		{"cancelled", SubscriptionStatusCancelled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := Subscription{Status: tt.status}
			if got := sub.CanMutateData(); got != tt.want {
				t.Errorf("CanMutateData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionIsPaidSource(t *testing.T) {
	tests := []struct {
		name   string
		source SubscriptionSource
		want   bool
	}{
		{"paid", SubscriptionSourcePaid, true},
		{"service", SubscriptionSourceService, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := Subscription{Source: tt.source}
			if got := sub.IsPaidSource(); got != tt.want {
				t.Errorf("IsPaidSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionIsInGrace(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name      string
		status    SubscriptionStatus
		validUntil *time.Time
		want      bool
	}{
		{"grace with future valid_until", SubscriptionStatusGrace, &future, true},
		{"grace with nil valid_until", SubscriptionStatusGrace, nil, false},
		{"grace with past valid_until", SubscriptionStatusGrace, &past, false},
		{"active with future valid_until", SubscriptionStatusActive, &future, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := Subscription{Status: tt.status, ValidUntil: tt.validUntil}
			if got := sub.IsInGrace(now); got != tt.want {
				t.Errorf("IsInGrace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionHasPendingChange(t *testing.T) {
	tariffID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	changeAt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		pendingTariffID *uuid.UUID
		pendingChangeAt *time.Time
		want            bool
	}{
		{"pending change", &tariffID, &changeAt, true},
		{"missing tariff", nil, &changeAt, false},
		{"missing date", &tariffID, nil, false},
		{"no pending", nil, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := Subscription{PendingTariffID: tt.pendingTariffID, PendingChangeAt: tt.pendingChangeAt}
			if got := sub.HasPendingChange(); got != tt.want {
				t.Errorf("HasPendingChange() = %v, want %v", got, tt.want)
			}
		})
	}
}
