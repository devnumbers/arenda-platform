package application

// The book listing's continuation cursors (ticket #600): opaque base64url
// blobs echoing the page's last row's keyset key. The encode/decode
// round-trips, the sort-name helper replicates the SQL's concat_ws exactly
// (empty fields store as NULL, skipped by the concatenation), and every
// malformed cursor folds into ErrInvalidInput.

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/contacts/domain"
)

// studioPropertyName is the shared bound-property literal of the cursor
// fixtures (goconst: one home).
const studioPropertyName = "Студия"

func TestContactCursorRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		key  ContactCursorKey
	}{
		{
			name: "name sort key",
			key:  ContactCursorKey{Name: "Пётр Иванов", ID: uuid.Must(uuid.NewV7())},
		},
		{
			name: "property sort key with bound card",
			key: ContactCursorKey{
				PropertyName: "Моя квартира",
				Name:         "Анна Сергеевна",
				ID:           uuid.Must(uuid.NewV7()),
			},
		},
		{
			name: "property sort key with unbound card",
			key: ContactCursorKey{
				Unbound: true,
				Name:    "Борис",
				ID:      uuid.Must(uuid.NewV7()),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DecodeContactCursor(EncodeContactCursor(tc.key))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got != tc.key {
				t.Fatalf("round trip: want %+v, got %+v", tc.key, got)
			}
		})
	}
}

func TestDecodeContactCursorMalformed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		cursor string
	}{
		{"not base64url", "не курсор!"},
		{"base64 of junk", "bm90IGEgY3Vyc29y"},
		{"empty payload", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := DecodeContactCursor(tc.cursor); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("want ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestListedContactCursorKeySortName(t *testing.T) {
	t.Parallel()

	id := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	cases := []struct {
		name      string
		contact   domain.Contact
		wantSort  string
		wantUnbnd bool
	}{
		{
			name: "full name joins non-empty parts",
			contact: domain.Contact{
				ID: id, FirstName: "Пётр", LastName: "Иванов", Patronymic: "Сергеевич",
				PropertyID: &propertyID,
			},
			wantSort: "Пётр Иванов Сергеевич",
		},
		{
			name: "empty patronymic adds no gap",
			contact: domain.Contact{
				ID: id, FirstName: "Анна", LastName: "Иванова", Patronymic: "",
				PropertyID: &propertyID,
			},
			wantSort: "Анна Иванова",
		},
		{
			name:      "given name alone",
			contact:   domain.Contact{ID: id, FirstName: updatedName},
			wantSort:  "Борис",
			wantUnbnd: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			key := ListedContact{Contact: tc.contact, PropertyName: ""}.CursorKey()
			if key.Name != tc.wantSort {
				t.Fatalf("sort name = %q, want %q", key.Name, tc.wantSort)
			}
			if key.Unbound != tc.wantUnbnd {
				t.Fatalf("unbound = %v, want %v", key.Unbound, tc.wantUnbnd)
			}
			if key.ID != id {
				t.Fatalf("id = %s, want %s", key.ID, id)
			}
		})
	}
}

func TestListedContactCursorKeyPropertyName(t *testing.T) {
	t.Parallel()

	id := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	bound := ListedContact{
		Contact:      domain.Contact{ID: id, FirstName: "Пётр", PropertyID: &propertyID},
		PropertyName: studioPropertyName,
	}
	key := bound.CursorKey()
	if key.Unbound {
		t.Fatal("bound card must not be unbound")
	}
	if key.PropertyName != studioPropertyName {
		t.Fatalf("property name = %q, want %q", key.PropertyName, studioPropertyName)
	}
}
