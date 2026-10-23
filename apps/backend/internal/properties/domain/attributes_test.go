package domain

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// Коды атрибутов каталога (tools/property-attributes/catalog.json) и их
// enum-значения, повторённые в тестах пакета.
const (
	attrRooms           = "rooms"
	attrAreaKitchen     = "area_kitchen"
	attrAreaLiving      = "area_living"
	attrAreaTotal       = "area_total"
	attrBalcony         = "balcony"
	attrBathroom        = "bathroom"
	attrCeilingHeight   = "ceiling_height"
	attrFloor           = "floor"
	attrFloorsTotal     = "floors_total"
	attrHouseType       = "house_type"
	attrLandArea        = "land_area"
	attrLandType        = "land_type"
	attrMaterial        = "material"
	attrBuildingType    = "building_type"
	attrEntrance        = "entrance"
	attrParkingLevel    = "parking_level"
	attrParkingType     = "parking_type"
	attrParkingLocation = "parking_location"
	attrRenovation      = "renovation"
	attrShower          = "shower"
	attrSpotNumber      = "spot_number"
	attrYearBuilt       = "year_built"

	enumStudio            = "studio"
	enumCombined          = "combined"
	enumLoggia            = "loggia"
	enumEuro              = "euro"
	enumUnderground       = "underground"
	enumGarden            = "garden"
	enumResidential       = "residential"
	enumTownhouse         = "townhouse"
	enumBrickMonolithic   = "brick_monolithic"
	enumOutdoor           = "outdoor"
	enumIndoor            = "indoor"
	enumBusinessCenter    = "business_center"
	enumSeparate          = "separate"
	enumDesign            = "design"
	enumCommon            = "common"
	enumWarehouseBuilding = "warehouse_building"
	enumMetal             = "metal"
	enumIzhs              = "izhs"
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
	t.Parallel()

	apartmentWant := []string{
		attrAreaKitchen, attrAreaLiving, attrAreaTotal, attrBalcony, attrBathroom,
		attrCeilingHeight, attrFloor, attrFloorsTotal, attrParkingType, attrRenovation,
		attrYearBuilt,
	}
	if got := CatalogKeys(PropertyTypeApartment); !slices.Equal(got, apartmentWant) {
		t.Fatalf("apartment keys = %v, want %v", got, apartmentWant)
	}
	if got := CatalogKeys(PropertyTypeApartments); !slices.Equal(got, apartmentWant) {
		t.Fatalf("apartments keys = %v, want %v", got, apartmentWant)
	}

	roomWant := []string{
		attrAreaKitchen, attrAreaTotal, attrBalcony, attrBathroom, attrFloor,
		attrFloorsTotal, attrRenovation, attrYearBuilt,
	}
	if got := CatalogKeys(PropertyTypeRoom); !slices.Equal(got, roomWant) {
		t.Fatalf("room keys = %v, want %v", got, roomWant)
	}

	houseWant := []string{
		attrAreaTotal, attrBathroom, attrFloorsTotal, attrHouseType, attrLandArea,
		attrLandType, attrMaterial, attrRooms, attrShower, attrYearBuilt,
	}
	if got := CatalogKeys(PropertyTypeHouse); !slices.Equal(got, houseWant) {
		t.Fatalf("house keys = %v, want %v", got, houseWant)
	}

	officeWant := []string{
		attrAreaTotal, attrBuildingType, attrEntrance, attrFloor, attrFloorsTotal,
		attrRenovation, attrRooms,
	}
	if got := CatalogKeys(PropertyTypeOffice); !slices.Equal(got, officeWant) {
		t.Fatalf("office keys = %v, want %v", got, officeWant)
	}
	if got := CatalogKeys(PropertyTypeCommercial); !slices.Equal(got, officeWant) {
		t.Fatalf("commercial keys = %v, want %v", got, officeWant)
	}

	warehouseWant := []string{
		attrAreaTotal, attrBuildingType, attrEntrance,
	}
	if got := CatalogKeys(PropertyTypeWarehouse); !slices.Equal(got, warehouseWant) {
		t.Fatalf("warehouse keys = %v, want %v", got, warehouseWant)
	}

	garageWant := []string{attrAreaTotal, attrMaterial}
	if got := CatalogKeys(PropertyTypeGarage); !slices.Equal(got, garageWant) {
		t.Fatalf("garage keys = %v, want %v", got, garageWant)
	}

	parkingWant := []string{
		attrAreaTotal, attrParkingLevel, attrParkingLocation, attrSpotNumber,
	}
	if got := CatalogKeys(PropertyTypeParking); !slices.Equal(got, parkingWant) {
		t.Fatalf("parking keys = %v, want %v", got, parkingWant)
	}

	landWant := []string{attrLandArea, attrLandType}
	if got := CatalogKeys(PropertyTypeLand); !slices.Equal(got, landWant) {
		t.Fatalf("land keys = %v, want %v", got, landWant)
	}
}

func TestAttributesEnumValues(t *testing.T) {
	t.Parallel()

	enumCases := []struct {
		propType PropertyType
		field    string
		valid    string
		invalid  string
	}{
		{PropertyTypeApartment, attrBathroom, enumCombined, "ensuite"},
		{PropertyTypeApartment, attrBalcony, enumLoggia, "terrace"},
		{PropertyTypeApartment, attrRenovation, enumEuro, "lux"},
		{PropertyTypeApartment, attrParkingType, enumUnderground, "valet"},
		{PropertyTypeRoom, attrBathroom, enumSeparate, "ensuite"},
		{PropertyTypeHouse, attrLandType, enumGarden, enumResidential},
		{PropertyTypeHouse, attrHouseType, enumTownhouse, "cottage"},
		{PropertyTypeHouse, attrMaterial, enumBrickMonolithic, "straw"},
		{PropertyTypeHouse, attrBathroom, enumOutdoor, "bidet"},
		{PropertyTypeHouse, attrShower, enumIndoor, "bathtub"},
		{PropertyTypeOffice, attrBuildingType, enumBusinessCenter, "skyscraper"},
		{PropertyTypeOffice, attrEntrance, enumSeparate, "private"},
		{PropertyTypeOffice, attrRenovation, enumDesign, "luxury"},
		{PropertyTypeWarehouse, attrEntrance, enumCommon, "shared"},
		{PropertyTypeWarehouse, attrBuildingType, enumWarehouseBuilding, "industrial"},
		{PropertyTypeGarage, attrMaterial, enumMetal, "plastic"},
		{PropertyTypeParking, attrParkingLocation, enumUnderground, "rooftop"},
		{PropertyTypeLand, attrLandType, enumIzhs, "recreational"},
	}
	for _, c := range enumCases {
		res := ValidateAttributes(c.propType, Attributes{c.field: c.valid})
		assertNoField(t, res, c.field)

		res = ValidateAttributes(c.propType, Attributes{c.field: c.invalid})
		assertHasField(t, res, c.field, "must be one of")
	}
}

func TestAttributesNumberRanges(t *testing.T) {
	t.Parallel()

	type tc struct {
		propType PropertyType
		field    string
		value    float64
		wantErr  bool
	}
	cases := []tc{
		// Границ нет вовсе (ADR 0069, второй раунд: минимумы сняты) —
		// валидны любые числа; ошибки только по типу.
		{PropertyTypeApartment, attrAreaTotal, 1.0, false},
		{PropertyTypeApartment, attrAreaTotal, 0.5, false},
		{PropertyTypeApartment, attrAreaTotal, 0, false},
		{PropertyTypeApartment, attrAreaTotal, -5, false},
		{PropertyTypeApartment, attrAreaTotal, 100000.0, false},
		{PropertyTypeApartment, attrAreaTotal, 5000000.0, false},
		{PropertyTypeApartment, attrCeilingHeight, 2.0, false},
		{PropertyTypeApartment, attrCeilingHeight, 1.9, false},
		{PropertyTypeApartment, attrCeilingHeight, 0, false},
		{PropertyTypeHouse, attrLandArea, 0.01, false},
		{PropertyTypeHouse, attrLandArea, 0.001, false},
		{PropertyTypeHouse, attrLandArea, 0, false},
		{PropertyTypeWarehouse, attrAreaTotal, 1.0, false},
		{PropertyTypeWarehouse, attrAreaTotal, 0.5, false},
		{PropertyTypeGarage, attrAreaTotal, 100000.0, false},
		{PropertyTypeGarage, attrAreaTotal, 100001.0, false},
		{PropertyTypeParking, attrAreaTotal, 25.0, false},
		{PropertyTypeLand, attrLandArea, 0.01, false},
		{PropertyTypeLand, attrLandArea, 0.001, false},
		{PropertyTypeLand, attrLandArea, 1000000.0, false},
		{PropertyTypeLand, attrLandArea, 1000001.0, false},
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
	t.Parallel()

	currentYear := time.Now().UTC().Year()
	cases := []struct {
		propType PropertyType
		field    string
		value    float64
		wantErr  bool
	}{
		{PropertyTypeApartment, attrFloor, -3, false},
		{PropertyTypeApartment, attrFloor, -4, false},
		// Границ нет вовсе (ADR 0069): этаж/этажность/год — любые целые.
		{PropertyTypeApartment, attrFloor, 0, false},
		{PropertyTypeApartment, attrFloor, 200, false},
		{PropertyTypeApartment, attrFloor, 201, false},
		{PropertyTypeApartment, attrFloorsTotal, 1, false},
		{PropertyTypeApartment, attrFloorsTotal, 0, false},
		{PropertyTypeApartment, attrYearBuilt, 1800, false},
		{PropertyTypeApartment, attrYearBuilt, 1799, false},
		{PropertyTypeApartment, attrYearBuilt, float64(currentYear + 5), false},
		{PropertyTypeApartment, attrYearBuilt, float64(currentYear + 6), false},
		{PropertyTypeApartment, attrYearBuilt, 1000, false},
		{PropertyTypeHouse, attrYearBuilt, 1800, false},
		{PropertyTypeHouse, attrYearBuilt, float64(currentYear + 5), false},
		{PropertyTypeOffice, attrFloor, -3, false},
		{PropertyTypeOffice, attrFloor, -4, false},
		{PropertyTypeOffice, attrFloor, 200, false},
		{PropertyTypeOffice, attrFloor, 201, false},
		{PropertyTypeOffice, attrFloorsTotal, 1, false},
		{PropertyTypeOffice, attrFloorsTotal, 0, false},
		{PropertyTypeParking, attrParkingLevel, -5, false},
		{PropertyTypeParking, attrParkingLevel, -6, false},
		{PropertyTypeParking, attrParkingLevel, 100, false},
		{PropertyTypeParking, attrParkingLevel, 101, false},
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
	t.Parallel()

	res := ValidateAttributes(PropertyTypeRoom, Attributes{attrAreaLiving: 50.0})
	assertHasField(t, res, attrAreaLiving, "unknown attribute for property type room")
}

func TestAttributesRoomRoomsUnknown(t *testing.T) {
	t.Parallel()

	// Ключ rooms выведен из наборов квартиры и комнаты: он больше не входит
	// в каталог и отвергается как неизвестный.
	res := ValidateAttributes(PropertyTypeRoom, Attributes{attrRooms: "2"})
	assertHasField(t, res, attrRooms, "unknown attribute")
}

// Кросс-правила согласованности сняты (решение владельца 23.10.2026):
// «этаж ≤ этажность» и «жилая/кухня ≤ общая» больше не валидируются —
// любые сочетания значений валидны.
func TestAttributesNoCrossValidations(t *testing.T) {
	t.Parallel()

	t.Run("apartment floor exceeds floors_total", func(t *testing.T) {
		t.Parallel()
		res := ValidateAttributes(PropertyTypeApartment, Attributes{
			attrFloor:       float64(5),
			attrFloorsTotal: float64(3),
		})
		assertValid(t, res)
	})

	t.Run("apartment area_living exceeds area_total", func(t *testing.T) {
		t.Parallel()
		res := ValidateAttributes(PropertyTypeApartment, Attributes{
			attrAreaLiving: 80.0,
			attrAreaTotal:  50.0,
		})
		assertValid(t, res)
	})

	t.Run("apartment area_kitchen exceeds area_total", func(t *testing.T) {
		t.Parallel()
		res := ValidateAttributes(PropertyTypeApartment, Attributes{
			attrAreaKitchen: 40.0,
			attrAreaTotal:   30.0,
		})
		assertValid(t, res)
	})

	t.Run("room floor exceeds floors_total", func(t *testing.T) {
		t.Parallel()
		res := ValidateAttributes(PropertyTypeRoom, Attributes{
			attrFloor:       float64(5),
			attrFloorsTotal: float64(3),
		})
		assertValid(t, res)
	})

	t.Run("room area_kitchen exceeds area_total", func(t *testing.T) {
		t.Parallel()
		res := ValidateAttributes(PropertyTypeRoom, Attributes{
			attrAreaKitchen: 40.0,
			attrAreaTotal:   30.0,
		})
		assertValid(t, res)
	})

	t.Run("office floor exceeds floors_total", func(t *testing.T) {
		t.Parallel()
		res := ValidateAttributes(PropertyTypeOffice, Attributes{
			attrFloor:       float64(10),
			attrFloorsTotal: float64(5),
		})
		assertValid(t, res)
	})

	t.Run("commercial floor exceeds floors_total", func(t *testing.T) {
		t.Parallel()
		res := ValidateAttributes(PropertyTypeCommercial, Attributes{
			attrFloor:       float64(10),
			attrFloorsTotal: float64(5),
		})
		assertValid(t, res)
	})
}

// assertValid fails when the result carries any validation errors.
func assertValid(t *testing.T, res ValidationResult) {
	t.Helper()
	if !res.Valid() {
		t.Fatalf("expected valid attributes, got errors: %+v", res.Errors)
	}
}

func TestAttributesUnknownKeys(t *testing.T) {
	t.Parallel()

	res := ValidateAttributes(PropertyTypeApartment, Attributes{"foo": float64(1)})
	assertHasField(t, res, "foo", "unknown attribute for property type apartment")
}

func TestAttributesNullValues(t *testing.T) {
	t.Parallel()

	res := ValidateAttributes(PropertyTypeApartment, Attributes{attrFloor: nil})
	assertHasField(t, res, attrFloor, "null")
}

func TestAttributesMultipleErrorsCollected(t *testing.T) {
	t.Parallel()

	// Floor=5.5 (not an integer), balcony=invalid — границы не валидируются,
	// ошибки только по типу/принадлежности.
	res := ValidateAttributes(PropertyTypeApartment, Attributes{
		attrFloor:   5.5,
		attrBalcony: "studioo",
	})
	fields := make(map[string]bool)
	for _, e := range res.Errors {
		fields[e.Field] = true
	}
	for _, want := range []string{attrFloor, attrBalcony} {
		if !fields[want] {
			t.Fatalf("expected an error on field %q, got errors: %+v", want, res.Errors)
		}
	}
	if len(res.Errors) != 2 {
		t.Fatalf("expected exactly 2 errors, got %d: %+v", len(res.Errors), res.Errors)
	}
}

func TestAttributesEmptyAttrsValid(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	// An unrecognised property type string is treated as unsupported: empty
	// attrs => valid (vacuous); non-empty => one error per key.
	const unknownType PropertyType = "unknown"

	res := ValidateAttributes(unknownType, Attributes{})
	if !res.Valid() {
		t.Fatalf("empty attrs for unknown type should be valid, got errors: %+v", res.Errors)
	}

	res = ValidateAttributes(unknownType, Attributes{
		attrFloor:     float64(3),
		attrAreaTotal: 50.0,
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
	t.Parallel()

	// Use keys that are apartment-only (not in the house catalog) plus one
	// house-only key (land_area). The rooms and area_total keys are shared
	// by both apartment and house, so they are excluded to keep the
	// assertions exact.
	attrs := Attributes{
		attrAreaLiving:    40.0,  // Apartment-only.
		attrAreaKitchen:   10.0,  // Apartment/room-only (not house).
		attrCeilingHeight: 2.7,   // Apartment-only.
		attrLandArea:      100.0, // House-only.
	}

	t.Run("apartment filter drops land_area", func(t *testing.T) {
		t.Parallel()
		out := attrs.FilterByType(PropertyTypeApartment)
		if _, ok := out[attrLandArea]; ok {
			t.Fatal("land_area should be dropped for apartment")
		}
		for _, k := range []string{attrAreaLiving, attrAreaKitchen, attrCeilingHeight} {
			if _, ok := out[k]; !ok {
				t.Fatalf("%s should be retained for apartment", k)
			}
		}
	})

	t.Run("house filter keeps only land_area", func(t *testing.T) {
		t.Parallel()
		out := attrs.FilterByType(PropertyTypeHouse)
		if _, ok := out[attrLandArea]; !ok {
			t.Fatal("land_area should be retained for house")
		}
		for _, k := range []string{attrAreaLiving, attrAreaKitchen, attrCeilingHeight} {
			if _, ok := out[k]; ok {
				t.Fatalf("%s should be dropped for house", k)
			}
		}
		if len(out) != 1 {
			t.Fatalf("house filter should keep exactly 1 key, got %d: %+v", len(out), out)
		}
	})

	t.Run("unknown type drops everything", func(t *testing.T) {
		t.Parallel()
		const unknownType PropertyType = "unknown"
		out := attrs.FilterByType(unknownType)
		if len(out) != 0 {
			t.Fatalf("unknown type filter should drop everything, got %+v", out)
		}
	})
}

func TestAttributesNumberTypeStrictness(t *testing.T) {
	t.Parallel()

	res := ValidateAttributes(PropertyTypeApartment, Attributes{attrFloor: 5.5})
	assertHasField(t, res, attrFloor, "expected an integer")

	// A float64 in a string field position is a type mismatch too.
	res = ValidateAttributes(PropertyTypeParking, Attributes{attrSpotNumber: 3.0})
	assertHasField(t, res, attrSpotNumber, "expected a string")
}

func TestAttributesApartmentsMirrorsApartment(t *testing.T) {
	t.Parallel()

	// A valid apartment set must also be valid for the apartments type, and a
	// former cross-rule violation no longer surfaces (правила сняты).
	valid := Attributes{
		attrBalcony:     enumLoggia,
		attrAreaTotal:   60.0,
		attrAreaLiving:  40.0,
		attrAreaKitchen: 10.0,
		attrFloor:       float64(2),
		attrFloorsTotal: float64(5),
	}
	if res := ValidateAttributes(PropertyTypeApartments, valid); !res.Valid() {
		t.Fatalf("apartments valid set should be valid, got errors: %+v", res.Errors)
	}

	bad := Attributes{
		attrFloor:       float64(9),
		attrFloorsTotal: float64(5),
	}
	if res := ValidateAttributes(PropertyTypeApartments, bad); !res.Valid() {
		t.Fatalf("floor above floors_total is valid now, got errors: %+v", res.Errors)
	}
}

func TestAttributesCommercialMirrorsOffice(t *testing.T) {
	t.Parallel()

	valid := Attributes{
		attrBuildingType: enumBusinessCenter,
		attrFloor:        float64(3),
		attrFloorsTotal:  float64(10),
		attrAreaTotal:    120.0,
		attrRooms:        "4",
		attrEntrance:     enumSeparate,
		attrRenovation:   enumEuro,
	}
	if res := ValidateAttributes(PropertyTypeCommercial, valid); !res.Valid() {
		t.Fatalf("commercial valid set should be valid, got errors: %+v", res.Errors)
	}

	bad := Attributes{
		attrFloor:       float64(15),
		attrFloorsTotal: float64(5),
	}
	if res := ValidateAttributes(PropertyTypeCommercial, bad); !res.Valid() {
		t.Fatalf("floor above floors_total is valid now, got errors: %+v", res.Errors)
	}
}

func TestAttributesParkingSpotNumber(t *testing.T) {
	t.Parallel()

	// Valid string.
	res := ValidateAttributes(PropertyTypeParking, Attributes{attrSpotNumber: "A-42"})
	if !res.Valid() {
		t.Fatalf("spot_number=A-42 should be valid, got errors: %+v", res.Errors)
	}

	// Over max length (51 chars).
	longSpot := strings.Repeat("A", 51)
	res = ValidateAttributes(PropertyTypeParking, Attributes{attrSpotNumber: longSpot})
	assertHasField(t, res, attrSpotNumber, "must be at most 50 characters")

	// Non-string value.
	res = ValidateAttributes(PropertyTypeParking, Attributes{attrSpotNumber: 42.0})
	assertHasField(t, res, attrSpotNumber, "expected a string")
}

func TestAttributesStudio(t *testing.T) {
	t.Parallel()

	// Студия делит набор характеристик с квартирой (каталог, тикет #1003):
	// те же поля; кросс-правила сняты (решение владельца 23.10.2026).
	t.Run("apartment field set is valid for studio", func(t *testing.T) {
		t.Parallel()
		res := ValidateAttributes(PropertyTypeStudio, Attributes{
			attrAreaTotal:     24.5,
			attrAreaLiving:    18.0,
			attrAreaKitchen:   6.5,
			attrFloor:         float64(3),
			attrFloorsTotal:   float64(9),
			attrYearBuilt:     float64(2021),
			attrCeilingHeight: 2.7,
		})
		if !res.Valid() {
			t.Fatalf("apartment-style attributes must be valid for studio, got: %+v", res.Errors)
		}
	})

	t.Run("floor exceeds floors_total", func(t *testing.T) {
		t.Parallel()
		res := ValidateAttributes(PropertyTypeStudio, Attributes{
			attrFloor:       float64(5),
			attrFloorsTotal: float64(3),
		})
		assertValid(t, res)
	})

	t.Run("area_living exceeds area_total", func(t *testing.T) {
		t.Parallel()
		res := ValidateAttributes(PropertyTypeStudio, Attributes{
			attrAreaLiving: 80.0,
			attrAreaTotal:  50.0,
		})
		assertValid(t, res)
	})

	t.Run("area_kitchen exceeds area_total", func(t *testing.T) {
		t.Parallel()
		res := ValidateAttributes(PropertyTypeStudio, Attributes{
			attrAreaKitchen: 40.0,
			attrAreaTotal:   30.0,
		})
		assertValid(t, res)
	})

	t.Run("house-only key is unknown for studio", func(t *testing.T) {
		t.Parallel()
		res := ValidateAttributes(PropertyTypeStudio, Attributes{attrLandArea: 6.0})
		assertHasField(t, res, attrLandArea, "")
	})
}
