package httpsupport

import (
	"strings"
	"testing"
)

func TestAddCardReturnURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
		result  string
		want    string
	}{
		{
			name:    "success flag on production-like origin",
			baseURL: "https://app.rentli.ru",
			result:  "success",
			want:    "https://app.rentli.ru/profile/tariff/payment-methods?addCard=success",
		},
		{
			name:    "fail flag keeps the query contract",
			baseURL: "https://app.rentli.ru",
			result:  "fail",
			want:    "https://app.rentli.ru/profile/tariff/payment-methods?addCard=fail",
		},
		{
			name:    "localhost origin with trailing slash is normalized",
			baseURL: "http://localhost:3000/",
			result:  "success",
			want:    "http://localhost:3000/profile/tariff/payment-methods?addCard=success",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := AddCardReturnURL(tt.baseURL, tt.result)
			if got != tt.want {
				t.Fatalf("AddCardReturnURL(%q, %q) = %q, want %q", tt.baseURL, tt.result, got, tt.want)
			}
			if !strings.HasPrefix(got, tt.baseURL) {
				t.Fatalf("result %q must carry the base origin", got)
			}
		})
	}
}
