package domain

import (
	"errors"
	"testing"
)

// testTimezone is the canonical IANA id shared by the domain timezone tests.
const testTimezone = "Europe/Moscow"

func TestNewTimezone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"valid IANA id Europe/Moscow", testTimezone, testTimezone, false},
		{"valid IANA id Asia/Yekaterinburg", "Asia/Yekaterinburg", "Asia/Yekaterinburg", false},
		{"valid IANA id UTC", "UTC", "UTC", false},
		{"surrounding whitespace is trimmed", "  Europe/Moscow  ", testTimezone, false},
		{"empty after trim", "   ", "", true},
		{"unknown identifier", "Mars/Olympus", "", true},
		{"garbage", "not-a-timezone", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewTimezone(tc.input)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidTimezone) {
					t.Fatalf("NewTimezone(%q) error = %v, want ErrInvalidTimezone", tc.input, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewTimezone(%q) error = %v, want nil", tc.input, err)
			}
			if got.String() != tc.want {
				t.Fatalf("NewTimezone(%q).String() = %q, want %q", tc.input, got.String(), tc.want)
			}
		})
	}
}

func TestTimezoneFrom(t *testing.T) {
	t.Parallel()

	t.Run("accepts a valid trusted identifier", func(t *testing.T) {
		t.Parallel()
		got, err := TimezoneFrom(testTimezone)
		if err != nil {
			t.Fatalf("TimezoneFrom error = %v", err)
		}
		if got.String() != testTimezone {
			t.Fatalf("TimezoneFrom = %q, want %s", got.String(), testTimezone)
		}
	})

	t.Run("rejects an empty value", func(t *testing.T) {
		t.Parallel()
		_, err := TimezoneFrom("")
		if !errors.Is(err, ErrInvalidTimezone) {
			t.Fatalf("TimezoneFrom(\"\") error = %v, want ErrInvalidTimezone", err)
		}
	})

	t.Run("rejects an invalid identifier", func(t *testing.T) {
		t.Parallel()
		_, err := TimezoneFrom("Not/AZone")
		if !errors.Is(err, ErrInvalidTimezone) {
			t.Fatalf("TimezoneFrom(invalid) error = %v, want ErrInvalidTimezone", err)
		}
	})
}
