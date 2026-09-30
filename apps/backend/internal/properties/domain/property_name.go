package domain

import "fmt"

// propertyNamePhrases is the type → possessive-phrase map of the
// auto-generated name: «Мой/Моя/Моё» + the type's noun, gendered per the
// noun (ticket #1001). The commercial phrase repeats the type label
// verbatim — the name derives from the type, the owner's wording.
var propertyNamePhrases = map[PropertyType]string{
	PropertyTypeApartment:  "Моя квартира",
	PropertyTypeRoom:       "Моя комната",
	PropertyTypeApartments: "Мои апартаменты",
	PropertyTypeHouse:      "Мой дом",
	PropertyTypeCommercial: "Моё коммерческое помещение",
	PropertyTypeOffice:     "Мой офис",
	PropertyTypeWarehouse:  "Мой склад",
	PropertyTypeGarage:     "Мой гараж",
	PropertyTypeParking:    "Моё машиноместо",
	PropertyTypeLand:       "Мой земельный участок",
}

// AutoPropertyName builds the generated property name (properties/CONTEXT.md
// «Автогенерация названия», ticket #1001): the possessive type phrase plus
// the serial number. The serial comes from the application layer — the
// owner's property count of this type (active and archived alike, deleted
// rows are gone) plus one, the owner's count-based rule: gaps after renames
// are expected, duplicate names are acceptable (names carry no uniqueness).
func AutoPropertyName(t PropertyType, serial int) string {
	phrase, ok := propertyNamePhrases[t]
	if !ok {
		phrase = "Мой объект"
	}
	return fmt.Sprintf("%s %d", phrase, serial)
}
