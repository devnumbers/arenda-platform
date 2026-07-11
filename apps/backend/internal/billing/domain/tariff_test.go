package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestClassifyTariffChange(t *testing.T) {
	basic := Tariff{
		ID:                  uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Name:                TariffBasic,
		ActivePropertyLimit: 5,
		MonthlyPriceKopecks: 1000,
		YearlyPriceKopecks:  10000,
	}
	pro := Tariff{
		ID:                  uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Name:                TariffPro,
		ActivePropertyLimit: 50,
		MonthlyPriceKopecks: 5000,
		YearlyPriceKopecks:  50000,
	}
	business := Tariff{
		ID:                  uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		Name:                TariffBusiness,
		ActivePropertyLimit: UnlimitedPropertyLimit,
		MonthlyPriceKopecks: 10000,
		YearlyPriceKopecks:  100000,
	}
	cheaperPro := Tariff{
		ID:                  uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		Name:                TariffPro,
		ActivePropertyLimit: 50,
		MonthlyPriceKopecks: 4000,
		YearlyPriceKopecks:  40000,
	}
	sameAsPro := Tariff{
		ID:                  uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		Name:                TariffPro,
		ActivePropertyLimit: 50,
		MonthlyPriceKopecks: 5000,
		YearlyPriceKopecks:  50000,
	}
	yearlyTieBreak := Tariff{
		ID:                  uuid.MustParse("66666666-6666-6666-6666-666666666666"),
		Name:                TariffPro,
		ActivePropertyLimit: 50,
		MonthlyPriceKopecks: 5000,
		YearlyPriceKopecks:  60000,
	}

	tests := []struct {
		name    string
		current Tariff
		next    Tariff
		want    TariffChangeType
	}{
		{"basic to pro", basic, pro, TariffChangeUpgrade},
		{"pro to basic", pro, basic, TariffChangeDowngrade},
		{"same tariff", pro, sameAsPro, TariffChangeSame},
		{"pro to unlimited", pro, business, TariffChangeUpgrade},
		{"unlimited to pro", business, pro, TariffChangeDowngrade},
		{"pro to cheaper same limit", pro, cheaperPro, TariffChangeDowngrade},
		{"cheaper to pro same limit", cheaperPro, pro, TariffChangeUpgrade},
		{"yearly price tie-break", pro, yearlyTieBreak, TariffChangeUpgrade},
		{"yearly price tie-break reverse", yearlyTieBreak, pro, TariffChangeDowngrade},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyTariffChange(tt.current, tt.next)
			if got != tt.want {
				t.Errorf("ClassifyTariffChange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseTariffName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    TariffName
		wantErr error
	}{
		{"basic", "basic", TariffBasic, nil},
		{"pro", "pro", TariffPro, nil},
		{"business", "business", TariffBusiness, nil},
		{"empty", "", "", ErrInvalidTariff},
		{"invalid", "premium", "", ErrInvalidTariff},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseTariffName(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ParseTariffName() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseTariffName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSubscriptionPeriod(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    SubscriptionPeriod
		wantErr error
	}{
		{"month", "month", PeriodMonth, nil},
		{"year", "year", PeriodYear, nil},
		{"empty", "", "", ErrInvalidPeriod},
		{"invalid", "weekly", "", ErrInvalidPeriod},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseSubscriptionPeriod(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ParseSubscriptionPeriod() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseSubscriptionPeriod() = %v, want %v", got, tt.want)
			}
		})
	}
}
