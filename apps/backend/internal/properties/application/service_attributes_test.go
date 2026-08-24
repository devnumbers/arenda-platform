package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
)

// ptr is a small generic helper for taking the address of a value, used to
// populate pointer fields on command structs in attribute tests. It is local
// to this file to avoid clashing with any helper that may be added elsewhere
// in the package.
func ptr[T any](v T) *T { return new(v) }

// newAttrService builds a PropertyService wired against the in-memory fakes,
// suitable for update-path attribute behavior tests (which do not touch the
// subscription limiter). The provided repo is pre-seeded by the caller.
func newAttrService(repo *lockingFakePropertyRepo) *PropertyService {
	return NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo,
			fakePropertyPhotoRepo{},
			nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)
}

// newAttrCreateService wires a PropertyService for create-path attribute
// tests. CreateProperty enforces the active-property limit, so a limiter must
// be supplied (unlike the update path). A generous limit avoids tripping it.
func newAttrCreateService(repo *lockingFakePropertyRepo) *PropertyService {
	return NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo,
			fakePropertyPhotoRepo{},
			fakeSubscriptionLimiter{limit: 100}),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)
}

const (
	attrTestOwnerIDStr    = "11111111-1111-1111-1111-111111111111"
	attrTestPropertyIDStr = "22222222-2222-2222-2222-222222222222"

	// Фикстуры create-команд и коды атрибутов, повторённые в этом файле.
	attrTestCreateName    = "Apt"
	attrTestCreateAddress = "Street 1"
	attrRooms             = "rooms"
	attrAreaTotal         = "area_total"
	attrFloor             = "floor"
)

// TestCreateProperty_WithValidAttributes verifies that a fully valid attribute
// set for an apartment is accepted and stored verbatim.
func TestCreateProperty_WithValidAttributes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse(attrTestOwnerIDStr)

	repo := newLockingFakePropertyRepo()
	svc := newAttrCreateService(repo)

	attrs := map[string]any{
		attrRooms:      "2",
		attrAreaTotal:  float64(50.0),
		attrFloor:      float64(3), // Validator expects JSON-style float64 integers.
		"floors_total": float64(5),
	}

	created, err := svc.CreateProperty(ctx, ownerID, CreatePropertyCommand{
		Name:        attrTestCreateName,
		Type:        string(domain.PropertyTypeApartment),
		Address:     attrTestCreateAddress,
		Description: "",
		Attributes:  attrs,
	})
	if err != nil {
		t.Fatalf("CreateProperty with valid attributes failed: %v", err)
	}

	want := []string{attrAreaTotal, attrFloor, "floors_total", attrRooms}
	if len(created.Attributes) != len(want) {
		t.Fatalf("expected %d attribute keys, got %d (%v)", len(want), len(created.Attributes), created.Attributes)
	}
	for _, k := range want {
		if _, ok := created.Attributes[k]; !ok {
			t.Errorf("expected attribute key %q to be present, got %v", k, created.Attributes)
		}
	}
}

// TestCreateProperty_WithInvalidAttributes verifies that an out-of-range floor
// yields an *AttributeValidationErrors that unwraps to ErrInvalidInput and
// reports exactly one error bound to the "floor" field.
func TestCreateProperty_WithInvalidAttributes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse(attrTestOwnerIDStr)

	repo := newLockingFakePropertyRepo()
	svc := newAttrCreateService(repo)

	_, err := svc.CreateProperty(ctx, ownerID, CreatePropertyCommand{
		Name:    attrTestCreateName,
		Type:    string(domain.PropertyTypeApartment),
		Address: attrTestCreateAddress,
		Attributes: map[string]any{
			attrFloor: float64(999), // Out of range (max 200).
		},
	})
	if err == nil {
		t.Fatalf("expected an error for invalid attributes, got nil")
	}

	var valErrs *AttributesValidationError
	if !errors.As(err, &valErrs) {
		t.Fatalf("expected *AttributesValidationError, got %T: %v", err, err)
	}

	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected error to unwrap to ErrInvalidInput, it does not")
	}

	if len(valErrs.Errors) != 1 {
		t.Fatalf("expected exactly 1 field error, got %d: %+v", len(valErrs.Errors), valErrs.Errors)
	}
	if valErrs.Errors[0].Field != attrFloor {
		t.Errorf("expected field error on \"floor\", got %q", valErrs.Errors[0].Field)
	}
}

// TestCreateProperty_WithNilAttributes verifies that nil attributes are accepted
// and stored as a non-nil empty map.
func TestCreateProperty_WithNilAttributes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse(attrTestOwnerIDStr)

	repo := newLockingFakePropertyRepo()
	svc := newAttrCreateService(repo)

	created, err := svc.CreateProperty(ctx, ownerID, CreatePropertyCommand{
		Name:       attrTestCreateName,
		Type:       string(domain.PropertyTypeApartment),
		Address:    attrTestCreateAddress,
		Attributes: nil,
	})
	if err != nil {
		t.Fatalf("CreateProperty with nil attributes failed: %v", err)
	}

	if created.Attributes == nil {
		t.Errorf("expected Attributes to be a non-nil empty map, got nil")
	}
	if len(created.Attributes) != 0 {
		t.Errorf("expected empty Attributes map, got %v", created.Attributes)
	}
}

// TestCreateProperty_WithEmptyAttributes verifies that an empty attribute map
// is accepted and stored empty.
func TestCreateProperty_WithEmptyAttributes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse(attrTestOwnerIDStr)

	repo := newLockingFakePropertyRepo()
	svc := newAttrCreateService(repo)

	created, err := svc.CreateProperty(ctx, ownerID, CreatePropertyCommand{
		Name:       attrTestCreateName,
		Type:       string(domain.PropertyTypeApartment),
		Address:    attrTestCreateAddress,
		Attributes: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateProperty with empty attributes failed: %v", err)
	}

	if len(created.Attributes) != 0 {
		t.Errorf("expected empty Attributes map, got %v", created.Attributes)
	}
}

// TestUpdateProperty_AttributesNotPresent_LeavesAttributesUntouched verifies
// that omitting Attributes from an update (nil pointer) does not touch the
// existing stored attributes.
func TestUpdateProperty_AttributesNotPresent_LeavesAttributesUntouched(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse(attrTestOwnerIDStr)
	propertyID := uuid.MustParse(attrTestPropertyIDStr)

	property := domain.Property{
		ID:         propertyID,
		OwnerID:    ownerID,
		Name:       testPropertyName,
		Type:       domain.PropertyTypeApartment,
		Address:    testPropertyAddress,
		Status:     domain.PropertyStatusActive,
		Attributes: domain.Attributes{attrRooms: "2"},
	}
	repo := newLockingFakePropertyRepo(property)
	svc := newAttrService(repo)

	updated, err := svc.UpdateProperty(ctx, ownerID, propertyID, UpdatePropertyCommand{
		Name: ptr("New"),
		// Attributes is nil -> must be left untouched.
	})
	if err != nil {
		t.Fatalf("UpdateProperty failed: %v", err)
	}

	if got := updated.Attributes[attrRooms]; got != "2" {
		t.Errorf("expected rooms to remain \"2\", got %v (attrs=%v)", got, updated.Attributes)
	}
}

// TestUpdateProperty_AttributesEmptyObject_ClearsAll verifies that passing a
// pointer to an empty attributes map clears all stored attributes.
func TestUpdateProperty_AttributesEmptyObject_ClearsAll(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse(attrTestOwnerIDStr)
	propertyID := uuid.MustParse(attrTestPropertyIDStr)

	property := domain.Property{
		ID:         propertyID,
		OwnerID:    ownerID,
		Name:       testPropertyName,
		Type:       domain.PropertyTypeApartment,
		Address:    testPropertyAddress,
		Status:     domain.PropertyStatusActive,
		Attributes: domain.Attributes{attrRooms: "2"},
	}
	repo := newLockingFakePropertyRepo(property)
	svc := newAttrService(repo)

	emptyAttrs := map[string]any{}
	updated, err := svc.UpdateProperty(ctx, ownerID, propertyID, UpdatePropertyCommand{
		Attributes: &emptyAttrs,
	})
	if err != nil {
		t.Fatalf("UpdateProperty failed: %v", err)
	}

	if len(updated.Attributes) != 0 {
		t.Errorf("expected cleared attributes, got %v", updated.Attributes)
	}
}

// TestUpdateProperty_AttributesFullReplacement verifies that passing attributes
// fully replaces the stored set (old keys are dropped, new keys are stored).
func TestUpdateProperty_AttributesFullReplacement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse(attrTestOwnerIDStr)
	propertyID := uuid.MustParse(attrTestPropertyIDStr)

	property := domain.Property{
		ID:         propertyID,
		OwnerID:    ownerID,
		Name:       testPropertyName,
		Type:       domain.PropertyTypeApartment,
		Address:    testPropertyAddress,
		Status:     domain.PropertyStatusActive,
		Attributes: domain.Attributes{attrRooms: "2"},
	}
	repo := newLockingFakePropertyRepo(property)
	svc := newAttrService(repo)

	newAttrs := map[string]any{
		attrRooms:     "3",
		attrAreaTotal: float64(45.0),
	}
	updated, err := svc.UpdateProperty(ctx, ownerID, propertyID, UpdatePropertyCommand{
		Attributes: &newAttrs,
	})
	if err != nil {
		t.Fatalf("UpdateProperty failed: %v", err)
	}

	if len(updated.Attributes) != 2 {
		t.Fatalf("expected exactly 2 attribute keys, got %d: %v", len(updated.Attributes), updated.Attributes)
	}
	if got := updated.Attributes[attrRooms]; got != "3" {
		t.Errorf("expected rooms to be \"3\", got %v", got)
	}
	if got := updated.Attributes[attrAreaTotal]; got != float64(45.0) {
		t.Errorf("expected area_total to be 45.0, got %v", got)
	}
}

// TestUpdateProperty_AttributesIdempotentReplacement verifies that applying the
// same attributes twice produces an identical, stable result.
func TestUpdateProperty_AttributesIdempotentReplacement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse(attrTestOwnerIDStr)
	propertyID := uuid.MustParse(attrTestPropertyIDStr)

	property := domain.Property{
		ID:         propertyID,
		OwnerID:    ownerID,
		Name:       testPropertyName,
		Type:       domain.PropertyTypeApartment,
		Address:    testPropertyAddress,
		Status:     domain.PropertyStatusActive,
		Attributes: domain.Attributes{attrRooms: "2"},
	}
	repo := newLockingFakePropertyRepo(property)
	svc := newAttrService(repo)

	attrs := map[string]any{
		attrRooms:     "3",
		attrAreaTotal: float64(45.0),
	}

	first, err := svc.UpdateProperty(ctx, ownerID, propertyID, UpdatePropertyCommand{
		Attributes: &attrs,
	})
	if err != nil {
		t.Fatalf("first UpdateProperty failed: %v", err)
	}

	second, err := svc.UpdateProperty(ctx, ownerID, propertyID, UpdatePropertyCommand{
		Attributes: &attrs,
	})
	if err != nil {
		t.Fatalf("second UpdateProperty failed: %v", err)
	}

	if len(second.Attributes) != len(first.Attributes) {
		t.Fatalf("idempotent mismatch: first has %d keys, second has %d", len(first.Attributes), len(second.Attributes))
	}
	for k, v := range first.Attributes {
		got, ok := second.Attributes[k]
		if !ok {
			t.Errorf("second update lost key %q", k)
			continue
		}
		if got != v {
			t.Errorf("value for key %q changed: first=%v second=%v", k, v, got)
		}
	}
	if len(second.Attributes) != 2 {
		t.Errorf("expected 2 keys after idempotent replacement, got %d: %v", len(second.Attributes), second.Attributes)
	}
}

// TestUpdateProperty_InvalidAttributes_ReturnsValidationError verifies that
// updating with an out-of-range attribute yields an *AttributeValidationErrors
// bound to the offending field.
func TestUpdateProperty_InvalidAttributes_ReturnsValidationError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse(attrTestOwnerIDStr)
	propertyID := uuid.MustParse(attrTestPropertyIDStr)

	property := domain.Property{
		ID:         propertyID,
		OwnerID:    ownerID,
		Name:       testPropertyName,
		Type:       domain.PropertyTypeApartment,
		Address:    testPropertyAddress,
		Status:     domain.PropertyStatusActive,
		Attributes: domain.Attributes{attrRooms: "2"},
	}
	repo := newLockingFakePropertyRepo(property)
	svc := newAttrService(repo)

	badAttrs := map[string]any{attrFloor: 999} // Out of range.
	_, err := svc.UpdateProperty(ctx, ownerID, propertyID, UpdatePropertyCommand{
		Attributes: &badAttrs,
	})
	if err == nil {
		t.Fatalf("expected an error for invalid attributes, got nil")
	}

	var valErrs *AttributesValidationError
	if !errors.As(err, &valErrs) {
		t.Fatalf("expected *AttributesValidationError, got %T: %v", err, err)
	}

	if len(valErrs.Errors) != 1 {
		t.Fatalf("expected exactly 1 field error, got %d: %+v", len(valErrs.Errors), valErrs.Errors)
	}
	if valErrs.Errors[0].Field != attrFloor {
		t.Errorf("expected field error on \"floor\", got %q", valErrs.Errors[0].Field)
	}
}

// TestUpdateProperty_LosslessOnTypeChange verifies that changing a property's
// type without supplying Attributes does NOT destroy the stored attribute blob:
// the original keys remain present in storage.
func TestUpdateProperty_LosslessOnTypeChange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse(attrTestOwnerIDStr)
	propertyID := uuid.MustParse(attrTestPropertyIDStr)

	property := domain.Property{
		ID:         propertyID,
		OwnerID:    ownerID,
		Name:       testPropertyName,
		Type:       domain.PropertyTypeApartment,
		Address:    testPropertyAddress,
		Status:     domain.PropertyStatusActive,
		Attributes: domain.Attributes{attrRooms: "2", attrAreaTotal: float64(50.0)},
	}
	repo := newLockingFakePropertyRepo(property)
	svc := newAttrService(repo)

	updated, err := svc.UpdateProperty(ctx, ownerID, propertyID, UpdatePropertyCommand{
		Type: ptr(string(domain.PropertyTypeHouse)),
		// Attributes intentionally nil: the blob must survive the type change.
	})
	if err != nil {
		t.Fatalf("UpdateProperty type change failed: %v", err)
	}

	if updated.Type != domain.PropertyTypeHouse {
		t.Errorf("expected type house, got %q", updated.Type)
	}
	if got := updated.Attributes[attrRooms]; got != "2" {
		t.Errorf("expected rooms to survive type change (\"2\"), got %v", got)
	}
	if got := updated.Attributes[attrAreaTotal]; got != float64(50.0) {
		t.Errorf("expected area_total to survive type change (50.0), got %v", got)
	}
}

// TestUpdateProperty_AttributesValidatedAgainstNewType verifies that when both
// a new type and new attributes are supplied, the attributes are validated
// against the NEW type: a house-valid key is accepted, while an
// apartment-only key (ceiling_height) is rejected as unknown for a house.
func TestUpdateProperty_AttributesValidatedAgainstNewType(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse(attrTestOwnerIDStr)
	propertyID := uuid.MustParse(attrTestPropertyIDStr)

	t.Run("house_valid_key_accepted", func(t *testing.T) {
		t.Parallel()
		property := domain.Property{
			ID:         propertyID,
			OwnerID:    ownerID,
			Name:       testPropertyName,
			Type:       domain.PropertyTypeApartment,
			Address:    testPropertyAddress,
			Status:     domain.PropertyStatusActive,
			Attributes: domain.Attributes{attrRooms: "2"},
		}
		repo := newLockingFakePropertyRepo(property)
		svc := newAttrService(repo)

		houseAttrs := map[string]any{"land_area": float64(6.0)}
		updated, err := svc.UpdateProperty(ctx, ownerID, propertyID, UpdatePropertyCommand{
			Type:       ptr(string(domain.PropertyTypeHouse)),
			Attributes: &houseAttrs,
		})
		if err != nil {
			t.Fatalf("expected house-valid attributes to be accepted, got: %v", err)
		}
		if got := updated.Attributes["land_area"]; got != float64(6.0) {
			t.Errorf("expected land_area to be 6.0, got %v", got)
		}
	})

	t.Run("apartment_only_key_rejected_for_house", func(t *testing.T) {
		t.Parallel()
		property := domain.Property{
			ID:         propertyID,
			OwnerID:    ownerID,
			Name:       testPropertyName,
			Type:       domain.PropertyTypeApartment,
			Address:    testPropertyAddress,
			Status:     domain.PropertyStatusActive,
			Attributes: domain.Attributes{attrRooms: "2"},
		}
		repo := newLockingFakePropertyRepo(property)
		svc := newAttrService(repo)

		apartmentOnlyAttrs := map[string]any{"ceiling_height": float64(2.7)}
		_, err := svc.UpdateProperty(ctx, ownerID, propertyID, UpdatePropertyCommand{
			Type:       ptr(string(domain.PropertyTypeHouse)),
			Attributes: &apartmentOnlyAttrs,
		})
		if err == nil {
			t.Fatalf("expected ceiling_height to be rejected for house, got nil error")
		}

		var valErrs *AttributesValidationError
		if !errors.As(err, &valErrs) {
			t.Fatalf("expected *AttributesValidationError, got %T: %v", err, err)
		}

		found := false
		for _, e := range valErrs.Errors {
			if e.Field == "ceiling_height" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected a field error on \"ceiling_height\", got %+v", valErrs.Errors)
		}
	})
}
