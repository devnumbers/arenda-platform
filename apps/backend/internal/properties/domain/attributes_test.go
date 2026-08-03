package domain

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// findField returns the first validation error whose Field matches name.
func findField(t *testing.T, res ValidationResult, name string) (AttributeValidationError, bool) {
	t.Helper()
	for _, e := range res.Errors {
		if e.Field == name {
			return e, true
		}
	}
	return AttributeValidationError{}, false
}

func assertHasField(t *testing.T, res ValidationResult, name, reasonContains string) {
	t.Helper()
	e, ok := findField(t, res, name)
	if !ok {
		t.Fatalf("expected error on field %q, got none. errors: %+v", name, res.Errors)
	}
	if reasonContains != "" && !strings.Contains(e.Reason, reasonContains) {
		t.Fatalf("field %q reason = %q, want substring %q", name, e.Reason, reasonContains)
	}
}

func assertNoField(t *testing.T, res ValidationResult, name string) {
	t.Helper()
	if e, ok := findField(t, res, name); ok {
		t.Fatalf("did not expect error on field %q, got %+v", name, e)
	}
}

func TestAttributesCatalogKeys(t *testing.T) {
	apartmentWant := []string{
		"area_kitchen", "area_living", "area_total", "balcony", "bathroom",
		"ceiling_height", "floor", "floors_total", "parking_type", "renovation",
		"rooms", "year_built",
	}
	if got := CatalogKeys(PropertyTypeApartment); !slices.Equal(got, apartmentWant) {
		t.Fatalf("apartment keys = %v, want %v", got, apartmentWant)
	}
	if got := CatalogKeys(PropertyTypeApartments); !slices.Equal(got, apartmentWant) {
		t.Fatalf("apartments keys = %v, want %v", got, apartmentWant)
	}

	roomWant := []string{
		"area_kitchen", "area_total", "balcony", "bathroom", "floor",
		"floors_total", "renovation", "rooms", "year_built",
	}
	if got := CatalogKeys(PropertyTypeRoom); !slices.Equal(got, roomWant) {
		t.Fatalf("room keys = %v, want %v", got, roomWant)
	}

	houseWant := []string{
		"area_total", "bathroom", "floors_total", "house_type", "land_area",
		"land_type", "material", "rooms", "shower", "year_built",
	}
	if got := CatalogKeys(PropertyTypeHouse); !slices.Equal(got, houseWant) {
		t.Fatalf("house keys = %v, want %v", got, houseWant)
	}

	officeWant := []string{
		"area_total", "building_type", "entrance", "floor", "floors_total",
		"renovation", "rooms",
	}
	if got := CatalogKeys(PropertyTypeOffice); !slices.Equal(got, officeWant) {
		t.Fatalf("office keys = %v, want %v", got, officeWant)
	}
	if got := CatalogKeys(PropertyTypeCommercial); !slices.Equal(got, officeWant) {
		t.Fatalf("commercial keys = %v, want %v", got, officeWant)
	}

	warehouseWant := []string{
		"area_total", "building_type", "entrance",
	}
	if got := CatalogKeys(PropertyTypeWarehouse); !slices.Equal(got, warehouseWant) {
		t.Fatalf("warehouse keys = %v, want %v", got, warehouseWant)
	}

	garageWant := []string{"area_total", "material"}
	if got := CatalogKeys(PropertyTypeGarage); !slices.Equal(got, garageWant) {
		t.Fatalf("garage keys = %v, want %v", got, garageWant)
	}

	parkingWant := []string{
		"area_total", "parking_level", "parking_location", "spot_number",
	}
	if got := CatalogKeys(PropertyTypeParking); !slices.Equal(got, parkingWant) {
		t.Fatalf("parking keys = %v, want %v", got, parkingWant)
	}

	landWant := []string{"land_area", "land_type"}
	if got := CatalogKeys(PropertyTypeLand); !slices.Equal(got, landWant) {
		t.Fatalf("land keys = %v, want %v", got, landWant)
	}
}

func TestAttributesEnumValues(t *testing.T) {
	enumCases := []struct {
		propType PropertyType
		field    string
		valid    string
		invalid  string
	}{
		{PropertyTypeApartment, "rooms", "studio", "studioo"},
		{PropertyTypeApartment, "bathroom", "combined", "ensuite"},
		{PropertyTypeApartment, "balcony", "loggia", "terrace"},
		{PropertyTypeApartment, "renovation", "euro", "lux"},
		{PropertyTypeApartment, "parking_type", "underground", "valet"},
		{PropertyTypeRoom, "rooms", "2", "studio"},
		{PropertyTypeHouse, "land_type", "garden", "residential"},
		{PropertyTypeHouse, "house_type", "townhouse", "cottage"},
		{PropertyTypeHouse, "material", "brick_monolithic", "straw"},
		{PropertyTypeHouse, "bathroom", "outdoor", "bidet"},
		{PropertyTypeHouse, "shower", "indoor", "bathtub"},
		{PropertyTypeOffice, "building_type", "business_center", "skyscraper"},
		{PropertyTypeOffice, "entrance", "separate", "private"},
		{PropertyTypeOffice, "renovation", "design", "luxury"},
		{PropertyTypeWarehouse, "entrance", "common", "shared"},
		{PropertyTypeWarehouse, "building_type", "warehouse_building", "industrial"},
		{PropertyTypeGarage, "material", "metal", "plastic"},
		{PropertyTypeParking, "parking_location", "underground", "rooftop"},
		{PropertyTypeLand, "land_type", "izhs", "recreational"},
	}
	for _, c := range enumCases {
		res := ValidateAttributes(c.propType, Attributes{c.field: c.valid})
		assertNoField(t, res, c.field)

		res = ValidateAttributes(c.propType, Attributes{c.field: c.invalid})
		assertHasField(t, res, c.field, "must be one of")
	}
}

func TestAttributesNumberRanges(t *testing.T) {
	type tc struct {
		propType PropertyType
		field    string
		value    float64
		wantErr  bool
	}
	cases := []tc{
		{PropertyTypeApartment, "area_total", 1.0, false},
		{PropertyTypeApartment, "area_total", 0.5, true},
		{PropertyTypeApartment, "area_total", 100000.0, false},
		{PropertyTypeApartment, "area_total", 100001.0, true},
		{PropertyTypeApartment, "ceiling_height", 2.0, false},
		{PropertyTypeApartment, "ceiling_height", 1.9, true},
		{PropertyTypeApartment, "ceiling_height", 10.0, false},
		{PropertyTypeApartment, "ceiling_height", 10.1, true},
		{PropertyTypeHouse, "land_area", 0.01, false},
		{PropertyTypeHouse, "land_area", 0.001, true},
		{PropertyTypeWarehouse, "area_total", 1.0, false},
		{PropertyTypeWarehouse, "area_total", 0.5, true},
		{PropertyTypeGarage, "area_total", 100000.0, false},
		{PropertyTypeGarage, "area_total", 100001.0, true},
		{PropertyTypeParking, "area_total", 25.0, false},
		{PropertyTypeLand, "land_area", 0.01, false},
		{PropertyTypeLand, "land_area", 0.001, true},
		{PropertyTypeLand, "land_area", 1000000.0, false},
		{PropertyTypeLand, "land_area", 1000001.0, true},
	}
	for _, c := range cases {
		res := ValidateAttributes(c.propType, Attributes{c.field: c.value})
		if c.wantErr {
			assertHasField(t, res, c.field, "")
		} else {
			assertNoField(t, res, c.field)
		}
	}
}

func TestAttributesIntegerRanges(t *testing.T) {
	currentYear := time.Now().UTC().Year()
	cases := []struct {
		propType PropertyType
		field    string
		value    float64
		wantErr  bool
	}{
		{PropertyTypeApartment, "floor", -3, false},
		{PropertyTypeApartment, "floor", -4, true},
		{PropertyTypeApartment, "floor", 200, false},
		{PropertyTypeApartment, "floor", 201, true},
		{PropertyTypeApartment, "floors_total", 1, false},
		{PropertyTypeApartment, "floors_total", 0, true},
		{PropertyTypeApartment, "year_built", 1800, false},
		{PropertyTypeApartment, "year_built", 1799, true},
		{PropertyTypeApartment, "year_built", float64(currentYear + 5), false},
		{PropertyTypeApartment, "year_built", float64(currentYear + 6), true},
		{PropertyTypeHouse, "year_built", 1800, false},
		{PropertyTypeHouse, "year_built", float64(currentYear + 5), false},
		{PropertyTypeOffice, "floor", -3, false},
		{PropertyTypeOffice, "floor", -4, true},
		{PropertyTypeOffice, "floor", 200, false},
		{PropertyTypeOffice, "floor", 201, true},
		{PropertyTypeOffice, "floors_total", 1, false},
		{PropertyTypeOffice, "floors_total", 0, true},
		{PropertyTypeParking, "parking_level", -5, false},
		{PropertyTypeParking, "parking_level", -6, true},
		{PropertyTypeParking, "parking_level", 100, false},
		{PropertyTypeParking, "parking_level", 101, true},
	}
	for _, c := range cases {
		res := ValidateAttributes(c.propType, Attributes{c.field: c.value})
		if c.wantErr {
			assertHasField(t, res, c.field, "")
		} else {
			assertNoField(t, res, c.field)
		}
	}
}

func TestAttributesRoomHasNoAreaLiving(t *testing.T) {
	res := ValidateAttributes(PropertyTypeRoom, Attributes{"area_living": 50.0})
	assertHasField(t, res, "area_living", "unknown attribute for property type room")
}

func TestAttributesRoomRoomsStartsAtTwo(t *testing.T) {
	for _, v := range []string{"studio", "1"} {
		res := ValidateAttributes(PropertyTypeRoom, Attributes{"rooms": v})
		assertHasField(t, res, "rooms", "must be one of")
	}
	res := ValidateAttributes(PropertyTypeRoom, Attributes{"rooms": "2"})
	if !res.Valid() {
		t.Fatalf("rooms=2 on room should be valid, got errors: %+v", res.Errors)
	}
}

func TestAttributesCrossValidations(t *testing.T) {
	t.Run("apartment floor exceeds floors_total", func(t *testing.T) {
		res := ValidateAttributes(PropertyTypeApartment, Attributes{
			"floor":        float64(5),
			"floors_total": float64(3),
		})
		assertHasField(t, res, "floor", "")
	})

	t.Run("apartment area_living exceeds area_total", func(t *testing.T) {
		res := ValidateAttributes(PropertyTypeApartment, Attributes{
			"area_living": 80.0,
			"area_total":  50.0,
		})
		assertHasField(t, res, "area_living", "")
	})

	t.Run("apartment area_kitchen exceeds area_total", func(t *testing.T) {
		res := ValidateAttributes(PropertyTypeApartment, Attributes{
			"area_kitchen": 40.0,
			"area_total":   30.0,
		})
		assertHasField(t, res, "area_kitchen", "")
	})

	t.Run("floor alone without floors_total passes", func(t *testing.T) {
		res := ValidateAttributes(PropertyTypeApartment, Attributes{"floor": float64(3)})
		if !res.Valid() {
			t.Fatalf("floor=3 alone should be valid, got errors: %+v", res.Errors)
		}
	})

	t.Run("room floor exceeds floors_total", func(t *testing.T) {
		res := ValidateAttributes(PropertyTypeRoom, Attributes{
			"floor":        float64(5),
			"floors_total": float64(3),
		})
		assertHasField(t, res, "floor", "")
	})

	t.Run("room area_kitchen exceeds area_total", func(t *testing.T) {
		res := ValidateAttributes(PropertyTypeRoom, Attributes{
			"area_kitchen": 40.0,
			"area_total":   30.0,
		})
		assertHasField(t, res, "area_kitchen", "")
	})

	t.Run("office floor exceeds floors_total", func(t *testing.T) {
		res := ValidateAttributes(PropertyTypeOffice, Attributes{
			"floor":        float64(10),
			"floors_total": float64(5),
		})
		assertHasField(t, res, "floor", "")
	})

	t.Run("commercial floor exceeds floors_total", func(t *testing.T) {
		res := ValidateAttributes(PropertyTypeCommercial, Attributes{
			"floor":        float64(10),
			"floors_total": float64(5),
		})
		assertHasField(t, res, "floor", "")
	})
}

func TestAttributesUnknownKeys(t *testing.T) {
	res := ValidateAttributes(PropertyTypeApartment, Attributes{"foo": float64(1)})
	assertHasField(t, res, "foo", "unknown attribute for property type apartment")
}

func TestAttributesNullValues(t *testing.T) {
	res := ValidateAttributes(PropertyTypeApartment, Attributes{"floor": nil})
	assertHasField(t, res, "floor", "null")
}

func TestAttributesMultipleErrorsCollected(t *testing.T) {
	// floor=-4 (out of range), rooms=invalid, area_total=0.5 (below min).
	res := ValidateAttributes(PropertyTypeApartment, Attributes{
		"floor":      float64(-4),
		"rooms":      "studioo",
		"area_total": 0.5,
	})
	fields := make(map[string]bool)
	for _, e := range res.Errors {
		fields[e.Field] = true
	}
	for _, want := range []string{"floor", "rooms", "area_total"} {
		if !fields[want] {
			t.Fatalf("expected an error on field %q, got errors: %+v", want, res.Errors)
		}
	}
	if len(res.Errors) != 3 {
		t.Fatalf("expected exactly 3 errors, got %d: %+v", len(res.Errors), res.Errors)
	}
}

func TestAttributesEmptyAttrsValid(t *testing.T) {
	for _, pt := range []PropertyType{
		PropertyTypeApartment,
		PropertyTypeApartments,
		PropertyTypeRoom,
		PropertyTypeHouse,
		PropertyTypeOffice,
		PropertyTypeCommercial,
		PropertyTypeWarehouse,
		PropertyTypeGarage,
		PropertyTypeParking,
		PropertyTypeLand,
	} {
		res := ValidateAttributes(pt, Attributes{})
		if !res.Valid() {
			t.Fatalf("empty attrs for type %s should be valid, got errors: %+v", pt, res.Errors)
		}
	}
}

func TestAttributesUnknownPropertyType(t *testing.T) {
	// An unrecognised property type string is treated as unsupported: empty
	// attrs => valid (vacuous); non-empty => one error per key.
	const unknownType PropertyType = "unknown"

	res := ValidateAttributes(unknownType, Attributes{})
	if !res.Valid() {
		t.Fatalf("empty attrs for unknown type should be valid, got errors: %+v", res.Errors)
	}

	res = ValidateAttributes(unknownType, Attributes{
		"floor":      float64(3),
		"area_total": 50.0,
	})
	if len(res.Errors) != 2 {
		t.Fatalf("expected 2 errors for unknown type non-empty, got %d: %+v", len(res.Errors), res.Errors)
	}
	for _, e := range res.Errors {
		if !strings.Contains(e.Reason, "unknown attribute for property type unknown") {
			t.Fatalf("error reason = %q, want substring %q", e.Reason, "unknown attribute for property type unknown")
		}
	}
}

func TestAttributesFilterByType(t *testing.T) {
	// Use keys that are apartment-only (not in the house catalog) plus one
	// house-only key (land_area). rooms/area_total are shared by both
	// apartment and house, so they are excluded to keep the assertions exact.
	attrs := Attributes{
		"area_living":    40.0,  // apartment-only
		"area_kitchen":   10.0,  // apartment/room-only (not house)
		"ceiling_height": 2.7,   // apartment-only
		"land_area":      100.0, // house-only
	}

	t.Run("apartment filter drops land_area", func(t *testing.T) {
		out := attrs.FilterByType(PropertyTypeApartment)
		if _, ok := out["land_area"]; ok {
			t.Fatal("land_area should be dropped for apartment")
		}
		for _, k := range []string{"area_living", "area_kitchen", "ceiling_height"} {
			if _, ok := out[k]; !ok {
				t.Fatalf("%s should be retained for apartment", k)
			}
		}
	})

	t.Run("house filter keeps only land_area", func(t *testing.T) {
		out := attrs.FilterByType(PropertyTypeHouse)
		if _, ok := out["land_area"]; !ok {
			t.Fatal("land_area should be retained for house")
		}
		for _, k := range []string{"area_living", "area_kitchen", "ceiling_height"} {
			if _, ok := out[k]; ok {
				t.Fatalf("%s should be dropped for house", k)
			}
		}
		if len(out) != 1 {
			t.Fatalf("house filter should keep exactly 1 key, got %d: %+v", len(out), out)
		}
	})

	t.Run("unknown type drops everything", func(t *testing.T) {
		const unknownType PropertyType = "unknown"
		out := attrs.FilterByType(unknownType)
		if len(out) != 0 {
			t.Fatalf("unknown type filter should drop everything, got %+v", out)
		}
	})
}

func TestAttributesNumberTypeStrictness(t *testing.T) {
	res := ValidateAttributes(PropertyTypeApartment, Attributes{"floor": 5.5})
	assertHasField(t, res, "floor", "expected an integer")

	// float64 in a string field position is a type mismatch too.
	res = ValidateAttributes(PropertyTypeApartment, Attributes{"rooms": 3.0})
	assertHasField(t, res, "rooms", "expected a string")
}

func TestAttributesApartmentsMirrorsApartment(t *testing.T) {
	// A valid apartment set must also be valid for the apartments type, and a
	// cross-field violation surfaces identically.
	valid := Attributes{
		"rooms":        "3",
		"area_total":   60.0,
		"area_living":  40.0,
		"area_kitchen": 10.0,
		"floor":        float64(2),
		"floors_total": float64(5),
	}
	if res := ValidateAttributes(PropertyTypeApartments, valid); !res.Valid() {
		t.Fatalf("apartments valid set should be valid, got errors: %+v", res.Errors)
	}

	bad := Attributes{
		"floor":        float64(9),
		"floors_total": float64(5),
	}
	res := ValidateAttributes(PropertyTypeApartments, bad)
	assertHasField(t, res, "floor", "")
}

func TestAttributesCommercialMirrorsOffice(t *testing.T) {
	valid := Attributes{
		"building_type": "business_center",
		"floor":         float64(3),
		"floors_total":  float64(10),
		"area_total":    120.0,
		"rooms":         "4",
		"entrance":      "separate",
		"renovation":    "euro",
	}
	if res := ValidateAttributes(PropertyTypeCommercial, valid); !res.Valid() {
		t.Fatalf("commercial valid set should be valid, got errors: %+v", res.Errors)
	}

	bad := Attributes{
		"floor":        float64(15),
		"floors_total": float64(5),
	}
	res := ValidateAttributes(PropertyTypeCommercial, bad)
	assertHasField(t, res, "floor", "")
}

func TestAttributesParkingSpotNumber(t *testing.T) {
	// Valid string.
	res := ValidateAttributes(PropertyTypeParking, Attributes{"spot_number": "A-42"})
	if !res.Valid() {
		t.Fatalf("spot_number=A-42 should be valid, got errors: %+v", res.Errors)
	}

	// Over max length (51 chars).
	longSpot := strings.Repeat("A", 51)
	res = ValidateAttributes(PropertyTypeParking, Attributes{"spot_number": longSpot})
	assertHasField(t, res, "spot_number", "must be at most 50 characters")

	// Non-string value.
	res = ValidateAttributes(PropertyTypeParking, Attributes{"spot_number": 42.0})
	assertHasField(t, res, "spot_number", "expected a string")
}
