package domain

import "testing"

func TestTimeOfDay_Forms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		hhmm string
		want TimeOfDay
	}{
		{"00:00", 0},
		{"09:05", 9*60 + 5},
		{"15:13", 15*60 + 13},
		{"23:59", 23*60 + 59},
	}
	for _, tt := range tests {
		t.Run(tt.hhmm, func(t *testing.T) {
			t.Parallel()
			got, err := ParseTimeOfDay(tt.hhmm)
			if err != nil {
				t.Fatalf("ParseTimeOfDay(%q) unexpected error: %v", tt.hhmm, err)
			}
			if got != tt.want {
				t.Fatalf("ParseTimeOfDay(%q) = %d, want %d", tt.hhmm, got, tt.want)
			}
			if got.String() != tt.hhmm {
				t.Fatalf("round trip %q -> %q", tt.hhmm, got.String())
			}
		})
	}
}

func TestParseTimeOfDay_Rejects(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"", "15", "15:1", "15:130", "24:00", "12:60", "9:05", "15:13:40", " 15:13", "15:13 ", "15-13"} {
		if got, err := ParseTimeOfDay(in); err == nil {
			t.Fatalf("ParseTimeOfDay(%q) = %d, want error", in, got)
		}
	}
}

func TestNewTimeOfDay_RejectsOutOfRange(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ h, m int }{{24, 0}, {-1, 0}, {0, -1}, {0, 60}, {23, 60}} {
		if _, err := NewTimeOfDay(tc.h, tc.m); err == nil {
			t.Fatalf("NewTimeOfDay(%d, %d) = _, want error", tc.h, tc.m)
		}
	}
	if got, err := NewTimeOfDay(23, 59); err != nil || got != 23*60+59 {
		t.Fatalf("NewTimeOfDay(23, 59) = %d, %v; want %d, nil", got, err, 23*60+59)
	}
}
