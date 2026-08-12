package email

import (
	"testing"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

func TestFormatLoginCodeTTL(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "actual domain TTL", d: domain.LoginCodeTTL, want: "5 минут"},
		{name: "1 minute", d: 1 * time.Minute, want: "1 минута"},
		{name: "2 minutes", d: 2 * time.Minute, want: "2 минуты"},
		{name: "5 minutes", d: 5 * time.Minute, want: "5 минут"},
		{name: "11 minutes", d: 11 * time.Minute, want: "11 минут"},
		{name: "21 minute", d: 21 * time.Minute, want: "21 минута"},
		{name: "22 minutes", d: 22 * time.Minute, want: "22 минуты"},
		{name: "90 minutes", d: 90 * time.Minute, want: "90 минут"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatLoginCodeTTL(tc.d); got != tc.want {
				t.Fatalf("formatLoginCodeTTL(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}
