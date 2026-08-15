package domain

import (
	"fmt"

	"github.com/google/uuid"
)

type TariffName string

const (
	TariffBasic    TariffName = "basic"
	TariffPro      TariffName = "pro"
	TariffBusiness TariffName = "business"
)

// ParseTariffName validates and converts a string to TariffName.
func ParseTariffName(s string) (TariffName, error) {
	switch TariffName(s) {
	case TariffBasic, TariffPro, TariffBusiness:
		return TariffName(s), nil
	}
	return "", ErrInvalidTariff
}

// TariffChangeType describes the direction of a tariff change.
type TariffChangeType string

const (
	TariffChangeUpgrade   TariffChangeType = "upgrade"
	TariffChangeDowngrade TariffChangeType = "downgrade"
	TariffChangeSame      TariffChangeType = "same"
)

// UnlimitedPropertyLimit is the value used for ActivePropertyLimit to denote
// an unlimited number of properties.
const UnlimitedPropertyLimit = -1

// Tariff is a subscription plan. IsActive=false hides the plan from users
// (listing) without breaking foreign keys that still reference it (issue #245).
type Tariff struct {
	ID                  uuid.UUID
	Name                TariffName
	ActivePropertyLimit int
	MonthlyPriceKopecks int64
	YearlyPriceKopecks  int64
	IsActive            bool
}

// NewTariff builds a new plan with a minted id, validating the name and the
// pricing fields (issue #256). It is the only way the admin create flow
// produces a Tariff, so an unvalidated plan never reaches persistence.
func NewTariff(name TariffName, activePropertyLimit int, monthlyPriceKopecks, yearlyPriceKopecks int64, isActive bool) (Tariff, error) {
	if _, err := ParseTariffName(string(name)); err != nil {
		return Tariff{}, fmt.Errorf("%w: %q", ErrInvalidTariff, name)
	}
	tariff := Tariff{
		ID:                  uuid.Must(uuid.NewV7()),
		Name:                name,
		ActivePropertyLimit: activePropertyLimit,
		MonthlyPriceKopecks: monthlyPriceKopecks,
		YearlyPriceKopecks:  yearlyPriceKopecks,
		IsActive:            isActive,
	}
	if err := tariff.Validate(); err != nil {
		return Tariff{}, err
	}
	return tariff, nil
}

// Validate checks the admin-editable tariff invariants (issue #256): prices
// are non-negative kopecks and the property limit is UnlimitedPropertyLimit
// (-1) or a non-negative count. The schema's CHECK constraints mirror these
// rules durably; this is the application-side gate.
func (t Tariff) Validate() error {
	if t.MonthlyPriceKopecks < 0 || t.YearlyPriceKopecks < 0 {
		return fmt.Errorf("%w: prices must be non-negative kopecks", ErrInvalidTariffPricing)
	}
	if t.ActivePropertyLimit < UnlimitedPropertyLimit {
		return fmt.Errorf("%w: active property limit must be %d (unlimited) or greater", ErrInvalidTariffPricing, UnlimitedPropertyLimit)
	}
	return nil
}

// ClassifyTariffChange compares current and next tariffs and returns the
// direction of the change.
//
// Rules (in order):
//  1. Higher active property limit is better. Unlimited (-1) beats any finite limit.
//  2. If limits are equal, higher monthly price is better.
//  3. If monthly prices are also equal, higher yearly price is the tie-breaker.
//  4. If all compared fields are equal, the tariffs are considered the same.
func ClassifyTariffChange(current, next Tariff) TariffChangeType {
	nextBetter := isTariffBetter(next, current)
	currentBetter := isTariffBetter(current, next)

	if nextBetter && !currentBetter {
		return TariffChangeUpgrade
	}
	if currentBetter && !nextBetter {
		return TariffChangeDowngrade
	}
	return TariffChangeSame
}

// isTariffBetter reports whether a is strictly better than b according to the
// tariff comparison rules.
func isTariffBetter(a, b Tariff) bool {
	aUnlimited := a.ActivePropertyLimit == UnlimitedPropertyLimit
	bUnlimited := b.ActivePropertyLimit == UnlimitedPropertyLimit

	if aUnlimited != bUnlimited {
		return aUnlimited
	}
	if a.ActivePropertyLimit != b.ActivePropertyLimit {
		return a.ActivePropertyLimit > b.ActivePropertyLimit
	}
	if a.MonthlyPriceKopecks != b.MonthlyPriceKopecks {
		return a.MonthlyPriceKopecks > b.MonthlyPriceKopecks
	}
	if a.YearlyPriceKopecks != b.YearlyPriceKopecks {
		return a.YearlyPriceKopecks > b.YearlyPriceKopecks
	}
	return false
}
