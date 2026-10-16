package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAccessEventDisplayName pins the access display-name canon of the access
// events' texts (карта #1105, аменд #1123): "Name Surname" when present,
// otherwise the anonymous label «Пользователь» — the phone and the email are
// never a display name.
func TestAccessEventDisplayName(t *testing.T) {
	t.Parallel()
	name, surname := "Иван", "Иванов"
	empty := ""

	tests := []struct {
		label   string
		name    *string
		surname *string
		want    string
	}{
		{label: "named", name: &name, surname: &surname, want: "Иван Иванов"},
		{label: "name only", name: &name, surname: nil, want: "Иван"},
		{label: "empty strings", name: &empty, surname: &empty, want: "Пользователь"},
		{label: "nils", name: nil, surname: nil, want: "Пользователь"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, accessEventDisplayName(tt.name, tt.surname))
		})
	}
}
