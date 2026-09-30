package domain

import "testing"

func TestAutoPropertyName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		phrase string
		typ    PropertyType
		serial int
		want   string
	}{
		{"квартира", PropertyTypeApartment, 1, "Моя квартира 1"},
		{"комната", PropertyTypeRoom, 2, "Моя комната 2"},
		{"апартаменты", PropertyTypeApartments, 1, "Мои апартаменты 1"},
		{"студия", PropertyTypeStudio, 1, "Моя студия 1"},
		{"дом", PropertyTypeHouse, 1, "Мой дом 1"},
		{"коммерческое — дословно как тип (владелец 30.09, #1001)", PropertyTypeCommercial, 1, "Моё коммерческое помещение 1"},
		{"офис", PropertyTypeOffice, 1, "Мой офис 1"},
		{"склад", PropertyTypeWarehouse, 1, "Мой склад 1"},
		{"гараж", PropertyTypeGarage, 3, "Мой гараж 3"},
		{"машиноместо", PropertyTypeParking, 1, "Моё машиноместо 1"},
		{"участок", PropertyTypeLand, 1, "Мой земельный участок 1"},
	}
	for _, tt := range tests {
		t.Run(tt.phrase, func(t *testing.T) {
			t.Parallel()
			if got := AutoPropertyName(tt.typ, tt.serial); got != tt.want {
				t.Fatalf("AutoPropertyName(%q, %d) = %q, want %q", tt.typ, tt.serial, got, tt.want)
			}
		})
	}
}

func TestAutoPropertyName_CoversAllValidTypes(t *testing.T) {
	t.Parallel()
	// Каждому валидному типу — своя фраза с притяжательным местоимением:
	// тип без фразы в карте роняет тест, а не молча даёт голый серийник
	// или catch-all. Список — все значения Valid() (domain/property.go).
	all := []PropertyType{
		PropertyTypeApartment,
		PropertyTypeRoom,
		PropertyTypeApartments,
		PropertyTypeStudio,
		PropertyTypeHouse,
		PropertyTypeCommercial,
		PropertyTypeOffice,
		PropertyTypeWarehouse,
		PropertyTypeGarage,
		PropertyTypeParking,
		PropertyTypeLand,
	}
	for _, typ := range all {
		t.Run(string(typ), func(t *testing.T) {
			t.Parallel()
			got := AutoPropertyName(typ, 1)
			if got == "Мой объект 1" {
				t.Fatalf("AutoPropertyName(%q, 1) fell back to the catch-all phrase — add the map entry", typ)
			}
		})
	}
}

func TestPropertyTypeStudio(t *testing.T) {
	t.Parallel()

	// Студия — полноценное значение типа (карта #984, тикет #1003): валиден
	// в ParsePropertyType/Valid, живёт в CHECK-констрейнте.
	studio, err := ParsePropertyType("studio")
	if err != nil {
		t.Fatalf("ParsePropertyType(studio): %v", err)
	}
	if !studio.Valid() {
		t.Fatal("studio must be a valid property type")
	}
	if _, err := ParsePropertyType("Studio"); err == nil {
		t.Fatal("type codes are lowercase: 'Studio' must be rejected")
	}
}
