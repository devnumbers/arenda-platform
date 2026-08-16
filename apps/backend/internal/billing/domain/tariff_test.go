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

// TestTariffPrice pins the price-for-period rule (issue #283): the period
// selects the price field, and an unknown period is an error instead of a
// silent fallback to the monthly price.
func TestTariffPrice(t *testing.T) {
	t.Parallel()
	pro := Tariff{Name: TariffPro, MonthlyPriceKopecks: 49000, YearlyPriceKopecks: 440000}
	free := Tariff{Name: TariffBasic, MonthlyPriceKopecks: 0, YearlyPriceKopecks: 0}

	tests := []struct {
		name    string
		tariff  Tariff
		period  SubscriptionPeriod
		want    int64
		wantErr error
	}{
		{"pro monthly", pro, PeriodMonth, 49000, nil},
		{"pro yearly", pro, PeriodYear, 440000, nil},
		{"free monthly", free, PeriodMonth, 0, nil},
		{"free yearly", free, PeriodYear, 0, nil},
		{"empty period", pro, "", 0, ErrInvalidPeriod},
		{"unknown period", pro, "weekly", 0, ErrInvalidPeriod},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tt.tariff.Price(tt.period)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Price() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Price() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestTariffValidate pins the admin-editable tariff invariants (issue #256):
// prices are non-negative kopecks and the property limit is -1 (unlimited) or
// a non-negative count.
func TestTariffValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		tariff  Tariff
		wantErr error
	}{
		{"free plan", Tariff{Name: TariffBasic, ActivePropertyLimit: 1, MonthlyPriceKopecks: 0, YearlyPriceKopecks: 0}, nil},
		{"unlimited limit", Tariff{Name: TariffBusiness, ActivePropertyLimit: UnlimitedPropertyLimit, MonthlyPriceKopecks: 99000, YearlyPriceKopecks: 890000}, nil},
		{"zero limit", Tariff{Name: TariffBasic, ActivePropertyLimit: 0}, nil},
		{"negative monthly price", Tariff{Name: TariffPro, ActivePropertyLimit: 5, MonthlyPriceKopecks: -1, YearlyPriceKopecks: 440000}, ErrInvalidTariffPricing},
		{"negative yearly price", Tariff{Name: TariffPro, ActivePropertyLimit: 5, MonthlyPriceKopecks: 49000, YearlyPriceKopecks: -1}, ErrInvalidTariffPricing},
		{"limit below unlimited", Tariff{Name: TariffPro, ActivePropertyLimit: -2, MonthlyPriceKopecks: 49000, YearlyPriceKopecks: 440000}, ErrInvalidTariffPricing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.tariff.Validate(); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestNewTariff proves the constructor mints an id, keeps the requested
// fields and rejects invalid pricing before any persistence sees it
// (issue #256).
func TestNewTariff(t *testing.T) {
	t.Parallel()

	tariff, err := NewTariff(TariffPro, 5, 49000, 440000, true)
	if err != nil {
		t.Fatalf("NewTariff() error = %v", err)
	}
	if tariff.ID == uuid.Nil {
		t.Error("ID = nil, want a minted uuid")
	}
	if tariff.Name != TariffPro || tariff.ActivePropertyLimit != 5 ||
		tariff.MonthlyPriceKopecks != 49000 || tariff.YearlyPriceKopecks != 440000 || !tariff.IsActive {
		t.Errorf("NewTariff() = %+v, want the requested fields kept", tariff)
	}

	if _, err := NewTariff(TariffPro, 5, -49000, 440000, true); !errors.Is(err, ErrInvalidTariffPricing) {
		t.Errorf("NewTariff(invalid) error = %v, want %v", err, ErrInvalidTariffPricing)
	}
	if _, err := NewTariff("premium", 5, 49000, 440000, true); !errors.Is(err, ErrInvalidTariff) {
		t.Errorf("NewTariff(unknown name) error = %v, want %v", err, ErrInvalidTariff)
	}
}
