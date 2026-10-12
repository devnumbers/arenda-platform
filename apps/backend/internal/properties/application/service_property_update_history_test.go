package application

// The pure composition of the edit's journal row (карта #704, ADR 0061 §4;
// единые change-строки — решение владельца 12.10, карта #1324):
// propertyUpdateHistoryEntry строит одну строку «Объект изменён: [новое
// имя]» с пояснением на каждое изменившееся поле и молчит (ok=false), когда
// ни одно журналируемое поле не сдвинулось — статус не в словаре истории,
// равные карты характеристик не дают диффа.

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
)

// historyEntryProperty builds the before/after state for one table case: the
// base object plus the caller's field overrides, so each case names only what
// it changes. Attributes are always fresh maps — the struct copy must not
// alias the base's set.
func historyEntryProperty(mutate func(p *domain.Property)) domain.Property {
	p := domain.Property{
		Name:    fixtureOldName,
		Address: fixtureOldAddress,
		Type:    domain.PropertyTypeApartment,
	}
	if mutate != nil {
		mutate(&p)
	}
	return p
}

// propertyUpdateCase is one row of the propertyUpdateHistoryEntry table: the
// before/after pair, the expected verdict, and the pinned details.
type propertyUpdateCase struct {
	name   string
	before domain.Property
	after  domain.Property
	wantOK bool
	// The wantDetails field pins the gray diff lines in order; the header
	// text is pinned separately in the assert helper.
	wantDetails historydomain.DetailLines
}

func TestPropertyUpdateHistoryEntry(t *testing.T) {
	t.Parallel()

	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	tests := []propertyUpdateCase{
		{
			name:        "single_group_rename",
			before:      historyEntryProperty(nil),
			after:       historyEntryProperty(func(p *domain.Property) { p.Name = renamedName }),
			wantOK:      true,
			wantDetails: historydomain.DetailLines{`Название: «` + fixtureOldName + `» → «` + renamedName + `»`},
		},
		{
			name:        "single_group_address",
			before:      historyEntryProperty(nil),
			after:       historyEntryProperty(func(p *domain.Property) { p.Address = renamedAddress }),
			wantOK:      true,
			wantDetails: historydomain.DetailLines{`Адрес: «` + fixtureOldAddress + `» → «` + renamedAddress + `»`},
		},
		{
			name:        "single_group_description",
			before:      historyEntryProperty(nil),
			after:       historyEntryProperty(func(p *domain.Property) { p.Description = "Тихий двор, рядом метро" }),
			wantOK:      true,
			wantDetails: historydomain.DetailLines{`Описание: «Тихий двор, рядом метро»`},
		},
		{
			name:   "single_group_attributes_changed",
			before: historyEntryProperty(func(p *domain.Property) { p.Attributes = domain.Attributes{attrBalcony: attrBalconyLoggia} }),
			after:  historyEntryProperty(func(p *domain.Property) { p.Attributes = domain.Attributes{attrBalcony: attrBalconyNone} }),
			wantOK: true,
			wantDetails: historydomain.DetailLines{
				`Балкон: «` + attrBalconyLoggia + `» → «` + attrBalconyNone + `»`,
			},
		},
		{
			// The type reads its RU label from PropertyTypeLabel in the diff.
			name:        "single_group_type",
			before:      historyEntryProperty(nil),
			after:       historyEntryProperty(func(p *domain.Property) { p.Type = domain.PropertyTypeHouse }),
			wantOK:      true,
			wantDetails: historydomain.DetailLines{`Тип: «Квартира» → «Дом»`},
		},
		{
			name:   "two_groups_ordered_lines",
			before: historyEntryProperty(nil),
			after: historyEntryProperty(func(p *domain.Property) {
				p.Name = renamedName
				p.Address = renamedAddress
			}),
			wantOK: true,
			wantDetails: historydomain.DetailLines{
				`Название: «` + fixtureOldName + `» → «` + renamedName + `»`,
				`Адрес: «` + fixtureOldAddress + `» → «` + renamedAddress + `»`,
			},
		},
		{
			name:   "five_groups_full_line_list",
			before: historyEntryProperty(func(p *domain.Property) { p.Attributes = domain.Attributes{attrBalcony: attrBalconyLoggia} }),
			after: historyEntryProperty(func(p *domain.Property) {
				p.Name = renamedName
				p.Address = renamedAddress
				p.Description = "Тихий двор, рядом метро"
				p.Type = domain.PropertyTypeHouse
				p.Attributes = domain.Attributes{attrBalcony: attrBalconyNone}
			}),
			wantOK: true,
			wantDetails: historydomain.DetailLines{
				`Название: «` + fixtureOldName + `» → «` + renamedName + `»`,
				`Адрес: «` + fixtureOldAddress + `» → «` + renamedAddress + `»`,
				`Описание: «Тихий двор, рядом метро»`,
				`Тип: «Квартира» → «Дом»`,
				// Метки характеристик читаются по НОВОМУ типу: у дома нет
				// балкона — деталь читает сырой ключ (attributeLabel).
				`balcony: «` + attrBalconyLoggia + `» → «` + attrBalconyNone + `»`,
			},
		},
		{
			// The status is not part of the journal dictionary: a status-only
			// edit (archive lives elsewhere) writes no row at all.
			name:   "status_only_edit_is_a_silent_no_op",
			before: historyEntryProperty(nil),
			after:  historyEntryProperty(func(p *domain.Property) { p.Status = domain.PropertyStatusArchived }),
			wantOK: false,
		},
		{
			name: "equal_attributes_are_a_silent_no_op",
			before: historyEntryProperty(func(p *domain.Property) {
				p.Attributes = domain.Attributes{attrBalcony: attrBalconyLoggia, attrAreaTotal: float64(50)}
			}),
			after: historyEntryProperty(func(p *domain.Property) {
				p.Attributes = domain.Attributes{attrAreaTotal: float64(50), attrBalcony: attrBalconyLoggia}
			}),
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entry, ok := propertyUpdateHistoryEntry(propertyID, tt.before, tt.after)
			assertPropertyUpdateCase(t, tt, entry, ok)
		})
	}
}

// assertPropertyUpdateCase pins one case's shape: the silent no-op stays an
// empty entry, the row reads «Объект изменён: [новое имя]» with one detail
// line per changed field.
func assertPropertyUpdateCase(t *testing.T, tt propertyUpdateCase, entry historydomain.Entry, ok bool) {
	t.Helper()
	if ok != tt.wantOK {
		t.Fatalf("ok = %v, want %v (action=%s)", ok, tt.wantOK, entry.Action)
	}
	if !tt.wantOK {
		if !reflect.DeepEqual(entry, historydomain.Entry{}) {
			t.Fatalf("no-op edit returned a non-empty entry: %+v", entry)
		}
		return
	}
	// Единая строка: один action, заголовок «Объект изменён: [новое имя]»,
	// пояснение на каждое изменившееся поле.
	if entry.Action != historydomain.ActionPropertyUpdated {
		t.Errorf("action = %s, want property.updated", entry.Action)
	}
	if want := "Объект изменён: " + tt.after.Name; entry.Segments.PlainText() != want {
		t.Errorf("row text = %q, want %q", entry.Segments.PlainText(), want)
	}
	if len(entry.Details) != len(tt.wantDetails) {
		t.Fatalf("details = %q, want %q", entry.Details, tt.wantDetails)
	}
	for i := range tt.wantDetails {
		if entry.Details[i] != tt.wantDetails[i] {
			t.Errorf("details[%d] = %q, want %q", i, entry.Details[i], tt.wantDetails[i])
		}
	}
	if _, ok := entry.Context["attributes"]; !ok {
		t.Error("context must keep the raw changes")
	}
}

// Метки каталога, повторяющиеся в ожиданиях таблиц (goconst).
const (
	labelBalcony   = "Балкон"
	labelAreaTotal = "Общая площадь"
)

func TestAttributeChanges(t *testing.T) {
	t.Parallel()

	// The label comes from the generated catalog by the property's type;
	// an apartment has no land_area, so that key's label falls back empty
	// (the detail line then reads the raw key).
	tests := []struct {
		name     string
		propType domain.PropertyType
		before   domain.Attributes
		after    domain.Attributes
		want     []historydomain.AttributeChange
	}{
		{
			// Changed and added keys in one edit; the result is sorted by the
			// catalog key regardless of the map iteration order, and the
			// unchanged key is dropped.
			name:     "changed_and_added_keys_sorted_by_key",
			propType: domain.PropertyTypeHouse,
			before:   domain.Attributes{attrFloor: float64(3), attrBalcony: attrBalconyLoggia, attrAreaTotal: float64(50)},
			after:    domain.Attributes{attrFloor: float64(5), attrBalcony: attrBalconyLoggia, attrAreaTotal: float64(45), attrLandArea: float64(6)},
			want: []historydomain.AttributeChange{
				{Key: attrAreaTotal, Label: labelAreaTotal, Old: float64(50), New: float64(45)},
				// У дома нет поля floor — метка пуста, деталь прочитает сырой
				// ключ (фолбэк attributeLabel).
				{Key: attrFloor, Label: "", Old: float64(3), New: float64(5)},
				{Key: attrLandArea, Label: "Площадь участка", Old: nil, New: float64(6)},
			},
		},
		{
			name:     "removed_key_carries_only_the_old_value",
			propType: domain.PropertyTypeApartment,
			before:   domain.Attributes{attrBalcony: attrBalconyLoggia},
			after:    domain.Attributes{},
			want:     []historydomain.AttributeChange{{Key: attrBalcony, Label: labelBalcony, Old: attrBalconyLoggia, New: nil}},
		},
		{
			// A nil before-side folds to an empty map: the whole new set reads
			// as additions.
			name:     "nil_before_reads_every_key_as_added",
			propType: domain.PropertyTypeApartment,
			before:   nil,
			after:    domain.Attributes{attrAreaTotal: float64(50)},
			want:     []historydomain.AttributeChange{{Key: attrAreaTotal, Label: labelAreaTotal, Old: nil, New: float64(50)}},
		},
		{
			// A nil after-side folds the same way: the whole old set reads as
			// removals (a cleared attribute set).
			name:     "nil_after_reads_every_key_as_removed",
			propType: domain.PropertyTypeApartment,
			before:   domain.Attributes{attrBalcony: attrBalconyLoggia, attrAreaTotal: float64(50)},
			after:    nil,
			want: []historydomain.AttributeChange{
				{Key: attrAreaTotal, Label: labelAreaTotal, Old: float64(50), New: nil},
				{Key: attrBalcony, Label: labelBalcony, Old: attrBalconyLoggia, New: nil},
			},
		},
		{
			name:     "equal_maps_diff_to_an_empty_change_list",
			propType: domain.PropertyTypeApartment,
			before:   domain.Attributes{attrBalcony: attrBalconyLoggia, attrAreaTotal: float64(50)},
			after:    domain.Attributes{attrAreaTotal: float64(50), attrBalcony: attrBalconyLoggia},
			want:     []historydomain.AttributeChange{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := attributeChanges(tt.before, tt.after, tt.propType)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attributeChanges = %v, want %v", got, tt.want)
			}
		})
	}
}
