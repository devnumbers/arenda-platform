package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewLeaseEndDateBeforeStart(t *testing.T) {
	ownerID := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	startDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	lease, err := NewLease(ownerID, propertyID, startDate, 10000, 1)
	if err != nil {
		t.Fatalf("NewLease failed: %v", err)
	}
	lease.EndDate = &endDate

	if err := lease.Validate(); err == nil {
		t.Fatal("expected validation error for end date before start date")
	}
}

func TestLeaseCalculateStatus(t *testing.T) {
	ownerID := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	startDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)

	lease, err := NewLease(ownerID, propertyID, startDate, 10000, 1)
	if err != nil {
		t.Fatalf("NewLease failed: %v", err)
	}

	if got := lease.CalculateStatus(time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)); got != LeaseStatusAwaitingStart {
		t.Fatalf("expected awaiting_start, got %s", got)
	}
	if got := lease.CalculateStatus(time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)); got != LeaseStatusActive {
		t.Fatalf("expected active, got %s", got)
	}

	lease.EndDate = &endDate
	if got := lease.CalculateStatus(time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)); got != LeaseStatusRequiresAction {
		t.Fatalf("expected requires_action, got %s", got)
	}
}
