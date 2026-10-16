package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
)

// The occupancy projection mapping (ticket #585): the enriched list row
// carries occupancy and the overdue flag; a row that was not enriched
// reports neither — the write and detail responses stay as before.

const (
	occTestName    = "Flat"
	occTestAddress = "ul. Testovaya 1"
)

func occTestProperty() domain.Property {
	return domain.Property{
		ID:      uuid.Must(uuid.NewV7()),
		Name:    occTestName,
		Type:    domain.PropertyTypeApartment,
		Address: occTestAddress,
		Status:  domain.PropertyStatusActive,
	}
}

func TestPropertyResponse_IncludesOccupancy(t *testing.T) {
	t.Parallel()

	h := newPropertyHandlersForMapping(t)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)

	property := occTestProperty()
	property.Occupancy = &domain.Occupancy{
		Status:         domain.OccupancyActive,
		StartDate:      &start,
		PlannedEndDate: &end,
	}
	property.HasOverdueOperations = true

	resp := h.propertyResponse(property)

	if resp.Occupancy == nil {
		t.Fatalf("occupancy = nil, want mapped")
	}
	if resp.Occupancy.Status != "active" {
		t.Errorf("occupancy.status = %q, want active", resp.Occupancy.Status)
	}
	if resp.Occupancy.StartDate == nil || !resp.Occupancy.StartDate.Equal(start) {
		t.Errorf("occupancy.start_date = %v, want %v", resp.Occupancy.StartDate, start)
	}
	if resp.Occupancy.PlannedEndDate == nil || !resp.Occupancy.PlannedEndDate.Equal(end) {
		t.Errorf("occupancy.planned_end_date = %v, want %v", resp.Occupancy.PlannedEndDate, end)
	}
	if resp.HasOverdueOperations == nil || !*resp.HasOverdueOperations {
		t.Errorf("has_overdue_operations = %v, want true", resp.HasOverdueOperations)
	}
}

func TestPropertyResponse_OccupancyNotEnriched(t *testing.T) {
	t.Parallel()

	h := newPropertyHandlersForMapping(t)

	resp := h.propertyResponse(occTestProperty())

	if resp.Occupancy != nil {
		t.Errorf("occupancy = %+v, want nil on the un-enriched row", resp.Occupancy)
	}
	if resp.HasOverdueOperations != nil {
		t.Errorf("has_overdue_operations = %v, want nil on the un-enriched row", resp.HasOverdueOperations)
	}
}
