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
		// Серийник 1 — без цифры (карта #1077, тикет #1078, владелец
		// 02.10): первый объект типа называется просто «Моя квартира»,
		// со второго — с цифрой.
		{"квартира", PropertyTypeApartment, 1, "Моя квартира"},
		{"комната", PropertyTypeRoom, 2, "Моя комната 2"},
		{"апартаменты", PropertyTypeApartments, 1, "Мои апартаменты"},
		{"студия", PropertyTypeStudio, 1, "Моя студия"},
		{"дом", PropertyTypeHouse, 1, "Мой дом"},
		{"коммерческое — дословно как тип (владелец 30.09, #1001)", PropertyTypeCommercial, 1, "Моё коммерческое помещение"},
		{"офис", PropertyTypeOffice, 1, "Мой офис"},
		{"склад", PropertyTypeWarehouse, 1, "Мой склад"},
		{"гараж", PropertyTypeGarage, 1, "Мой гараж"},
		{"гараж — второй", PropertyTypeGarage, 2, "Мой гараж 2"},
		{"гараж — третий", PropertyTypeGarage, 3, "Мой гараж 3"},
		{"машиноместо", PropertyTypeParking, 1, "Моё машиноместо"},
		{"участок", PropertyTypeLand, 1, "Мой земельный участок"},
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
			if got == "Мой объект" {
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
