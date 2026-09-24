package application

// The pure composition of the edit's journal row (карта #704, ADR 0061 §4):
// propertyUpdateHistoryEntry picks the precise action id for a single changed
// field group, falls back to the generic property.updated with the ordered RU
// label list for several at once, and stays silent (ok=false) when no
// journalable group moved — the status is not in the journal dictionary, and
// equal attribute maps diff to nothing.

import (
	"reflect"
	"strings"
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
// before/after pair, the expected verdict, and the pinned context payloads.
type propertyUpdateCase struct {
	name       string
	before     domain.Property
	after      domain.Property
	wantOK     bool
	wantAction historydomain.Action
	// The wantLabels field pins Context["fields"] of the generic
	// property.updated row, including the RU label order; nil for the
	// precise-action rows.
	wantLabels []string
	// The wantAttrs field pins Context["attributes"] of the
	// attributes_changed row.
	wantAttrs []historydomain.AttributeChange
}

func TestPropertyUpdateHistoryEntry(t *testing.T) {
	t.Parallel()

	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	tests := []propertyUpdateCase{
		{
			name:   "single_group_rename",
			before: historyEntryProperty(nil),
			after:  historyEntryProperty(func(p *domain.Property) { p.Name = renamedName }),
			wantOK: true, wantAction: historydomain.ActionPropertyRenamed,
		},
		{
			name:   "single_group_address",
			before: historyEntryProperty(nil),
			after:  historyEntryProperty(func(p *domain.Property) { p.Address = renamedAddress }),
			wantOK: true, wantAction: historydomain.ActionPropertyAddressChanged,
		},
		{
			name:   "single_group_description",
			before: historyEntryProperty(nil),
			after:  historyEntryProperty(func(p *domain.Property) { p.Description = "Тихий двор, рядом метро" }),
			wantOK: true, wantAction: historydomain.ActionPropertyDescriptionChanged,
		},
		{
			name:   "single_group_attributes_changed",
			before: historyEntryProperty(func(p *domain.Property) { p.Attributes = domain.Attributes{attrBalcony: attrBalconyLoggia} }),
			after:  historyEntryProperty(func(p *domain.Property) { p.Attributes = domain.Attributes{attrBalcony: attrBalconyNone} }),
			wantOK: true, wantAction: historydomain.ActionPropertyAttributesChanged,
			wantAttrs: []historydomain.AttributeChange{{Key: attrBalcony, Old: attrBalconyLoggia, New: attrBalconyNone}},
		},
		{
			// The type is the only group without a dedicated row id: it still
			// reads as a change through the generic property.updated row.
			name:       "single_group_type_is_generic_updated",
			before:     historyEntryProperty(nil),
			after:      historyEntryProperty(func(p *domain.Property) { p.Type = domain.PropertyTypeHouse }),
			wantOK:     true,
			wantAction: historydomain.ActionPropertyUpdated,
			wantLabels: []string{"тип"},
		},
		{
			name:   "two_groups_generic_with_ordered_labels",
			before: historyEntryProperty(nil),
			after: historyEntryProperty(func(p *domain.Property) {
				p.Name = renamedName
				p.Address = renamedAddress
			}),
			wantOK:     true,
			wantAction: historydomain.ActionPropertyUpdated,
			wantLabels: []string{"название", "адрес"},
		},
		{
			name:   "five_groups_generic_with_full_label_list",
			before: historyEntryProperty(func(p *domain.Property) { p.Attributes = domain.Attributes{attrBalcony: attrBalconyLoggia} }),
			after: historyEntryProperty(func(p *domain.Property) {
				p.Name = renamedName
				p.Address = renamedAddress
				p.Description = "Тихий двор, рядом метро"
				p.Type = domain.PropertyTypeHouse
				p.Attributes = domain.Attributes{attrBalcony: attrBalconyNone}
			}),
			wantOK:     true,
			wantAction: historydomain.ActionPropertyUpdated,
			wantLabels: []string{"название", "адрес", "описание", "тип", "характеристики"},
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

			assertPropertyUpdateCase(t, propertyID, tt)
		})
	}
}

// assertPropertyUpdateCase runs one table case against
// propertyUpdateHistoryEntry and pins the entry's shape: the silent no-op
// stays an empty entry, the row's action id, the generic row's ordered RU
// labels with the rendered row text, and the attributes_changed diff.
func assertPropertyUpdateCase(t *testing.T, propertyID uuid.UUID, tt propertyUpdateCase) {
	t.Helper()

	entry, ok := propertyUpdateHistoryEntry(propertyID, tt.before, tt.after)
	if ok != tt.wantOK {
		t.Fatalf("ok = %v, want %v (action=%s)", ok, tt.wantOK, entry.Action)
	}
	if !tt.wantOK {
		if !reflect.DeepEqual(entry, historydomain.Entry{}) {
			t.Fatalf("no-op edit returned a non-empty entry: %+v", entry)
		}
		return
	}
	if entry.Action != tt.wantAction {
		t.Errorf("action = %s, want %s", entry.Action, tt.wantAction)
	}
	if tt.wantLabels != nil {
		assertGenericRowLabels(t, entry, tt.wantLabels)
	}
	if tt.wantAttrs != nil {
		assertAttributeChangeList(t, entry, tt.wantAttrs)
	}
}

// assertGenericRowLabels pins the generic row's Context["fields"] list and the
// rendered row text built from it.
func assertGenericRowLabels(t *testing.T, entry historydomain.Entry, wantLabels []string) {
	t.Helper()

	fields, ok := entry.Context["fields"].([]string)
	if !ok {
		t.Fatalf("context fields = %T, want []string", entry.Context["fields"])
	}
	if !reflect.DeepEqual(fields, wantLabels) {
		t.Errorf("context fields = %v, want %v", fields, wantLabels)
	}
	if got, want := entry.Segments.PlainText(), "Данные объекта изменены: "+strings.Join(wantLabels, ", "); got != want {
		t.Errorf("row text = %q, want %q", got, want)
	}
}

// assertAttributeChangeList pins Context["attributes"] of the
// attributes_changed row.
func assertAttributeChangeList(t *testing.T, entry historydomain.Entry, wantAttrs []historydomain.AttributeChange) {
	t.Helper()

	changes, ok := entry.Context["attributes"].([]historydomain.AttributeChange)
	if !ok {
		t.Fatalf("context attributes = %T, want []historydomain.AttributeChange", entry.Context["attributes"])
	}
	if !reflect.DeepEqual(changes, wantAttrs) {
		t.Errorf("context attributes = %v, want %v", changes, wantAttrs)
	}
}

func TestAttributeChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		before domain.Attributes
		after  domain.Attributes
		want   []historydomain.AttributeChange
	}{
		{
			// Changed and added keys in one edit; the result is sorted by the
			// catalog key regardless of the map iteration order, and the
			// unchanged key is dropped.
			name:   "changed_and_added_keys_sorted_by_key",
			before: domain.Attributes{attrFloor: float64(3), attrBalcony: attrBalconyLoggia, attrAreaTotal: float64(50)},
			after:  domain.Attributes{attrFloor: float64(5), attrBalcony: attrBalconyLoggia, attrAreaTotal: float64(45), attrLandArea: float64(6)},
			want: []historydomain.AttributeChange{
				{Key: attrAreaTotal, Old: float64(50), New: float64(45)},
				{Key: attrFloor, Old: float64(3), New: float64(5)},
				{Key: attrLandArea, Old: nil, New: float64(6)},
			},
		},
		{
			name:   "removed_key_carries_only_the_old_value",
			before: domain.Attributes{attrBalcony: attrBalconyLoggia},
			after:  domain.Attributes{},
			want:   []historydomain.AttributeChange{{Key: attrBalcony, Old: attrBalconyLoggia, New: nil}},
		},
		{
			// A nil before-side folds to an empty map: the whole new set reads
			// as additions.
			name:   "nil_before_reads_every_key_as_added",
			before: nil,
			after:  domain.Attributes{attrAreaTotal: float64(50)},
			want:   []historydomain.AttributeChange{{Key: attrAreaTotal, Old: nil, New: float64(50)}},
		},
		{
			// A nil after-side folds the same way: the whole old set reads as
			// removals (a cleared attribute set).
			name:   "nil_after_reads_every_key_as_removed",
			before: domain.Attributes{attrBalcony: attrBalconyLoggia, attrAreaTotal: float64(50)},
			after:  nil,
			want: []historydomain.AttributeChange{
				{Key: attrAreaTotal, Old: float64(50), New: nil},
				{Key: attrBalcony, Old: attrBalconyLoggia, New: nil},
			},
		},
		{
			name:   "equal_maps_diff_to_an_empty_change_list",
			before: domain.Attributes{attrBalcony: attrBalconyLoggia, attrAreaTotal: float64(50)},
			after:  domain.Attributes{attrAreaTotal: float64(50), attrBalcony: attrBalconyLoggia},
			want:   []historydomain.AttributeChange{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := attributeChanges(tt.before, tt.after)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attributeChanges = %v, want %v", got, tt.want)
			}
		})
	}
}
