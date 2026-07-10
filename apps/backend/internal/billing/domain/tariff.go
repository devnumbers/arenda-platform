package domain

import (
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

type Tariff struct {
	ID                  uuid.UUID
	Name                TariffName
	ActivePropertyLimit int
	MonthlyPriceKopecks int64
	YearlyPriceKopecks  int64
}

// ClassifyTariffChange compares current and next tariffs and returns the
// direction of the change.
//
// Rules (in order):
//   1. Higher active property limit is better. Unlimited (-1) beats any finite limit.
//   2. If limits are equal, higher monthly price is better.
//   3. If monthly prices are also equal, higher yearly price is the tie-breaker.
//   4. If all compared fields are equal, the tariffs are considered the same.
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
