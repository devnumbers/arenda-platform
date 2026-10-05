package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAccessEventDisplayName pins the access display-name canon of the access
// events' texts (карта #1105): "Name Surname" when present, otherwise the
// full phone — the email is never a display name.
func TestAccessEventDisplayName(t *testing.T) {
	t.Parallel()
	name, surname := "Иван", "Иванов"
	empty := ""

	tests := []struct {
		label   string
		name    *string
		surname *string
		phone   string
		want    string
	}{
		{label: "named", name: &name, surname: &surname, phone: "+79991234567", want: "Иван Иванов"},
		{label: "name only", name: &name, surname: nil, phone: "+79991234567", want: "Иван"},
		{label: "empty strings", name: &empty, surname: &empty, phone: "+79991234567", want: "+79991234567"},
		{label: "nils", name: nil, surname: nil, phone: "+79991234567", want: "+79991234567"},
		{label: "no name, no phone", name: nil, surname: nil, phone: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, accessEventDisplayName(tt.name, tt.surname, tt.phone))
		})
	}
}
