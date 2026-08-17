package tkassa

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// TestPaymentDescription pins the Russian payment descriptions rendered from
// the structured purpose (issue #244/#248): the wording lives in the adapter,
// not in the application layer.
func TestPaymentDescription(t *testing.T) {
	tests := []struct {
		name    string
		purpose application.PaymentPurpose
		want    string
	}{
		{
			name: "subscription month",
			purpose: application.PaymentPurpose{
				Kind:       application.PaymentPurposeSubscription,
				TariffName: domain.TariffPro,
				Period:     domain.PeriodMonth,
			},
			want: "Оплата подписки Pro (месяц)",
		},
		{
			name: "subscription year",
			purpose: application.PaymentPurpose{
				Kind:       application.PaymentPurposeSubscription,
				TariffName: domain.TariffBusiness,
				Period:     domain.PeriodYear,
			},
			want: "Оплата подписки Business (год)",
		},
		{
			name: "renewal",
			purpose: application.PaymentPurpose{
				Kind:       application.PaymentPurposeRenewal,
				TariffName: domain.TariffBasic,
				Period:     domain.PeriodMonth,
			},
			want: "Продление подписки Basic (месяц)",
		},
		{
			name: "unknown kind defaults to subscription wording",
			purpose: application.PaymentPurpose{
				Kind:       application.PaymentPurposeKind("something_else"),
				TariffName: domain.TariffPro,
				Period:     domain.PeriodMonth,
			},
			want: "Оплата подписки Pro (месяц)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := paymentDescription(tt.purpose); got != tt.want {
				t.Fatalf("paymentDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestTruncateDescriptionRunes pins the rune-based truncation: a byte-based
// cut would split a UTF-8 rune in half and send an invalid string (issue #246
// §2.1). The limit is 140 characters for card and COF payments.
func TestTruncateDescriptionRunes(t *testing.T) {
	// Short strings pass through untouched.
	short := "Оплата подписки Pro (месяц)"
	if got := truncateDescription(short); got != short {
		t.Fatalf("short string modified: got %q, want %q", got, short)
	}

	// A long Russian string is cut at exactly maxDescriptionLength runes and
	// every rune stays intact.
	long := strings.Repeat("я", maxDescriptionLength+10)
	got := truncateDescription(long)
	if n := utf8.RuneCountInString(got); n != maxDescriptionLength {
		t.Fatalf("rune count: got %d, want %d", n, maxDescriptionLength)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("truncated string is not valid UTF-8: %q", got)
	}
	if got != strings.Repeat("я", maxDescriptionLength) {
		t.Fatalf("truncated content mismatch")
	}

	// Mixed ASCII and Cyrillic truncation preserves the boundary runes.
	mixed := strings.Repeat("ab", 100) + strings.Repeat("вг", 100)
	got = truncateDescription(mixed)
	if n := utf8.RuneCountInString(got); n != maxDescriptionLength {
		t.Fatalf("mixed rune count: got %d, want %d", n, maxDescriptionLength)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("mixed truncated string is not valid UTF-8: %q", got)
	}
}
