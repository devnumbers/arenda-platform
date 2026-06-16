package domain

import (
	"github.com/google/uuid"
)

type TariffName string

const TariffBasic TariffName = "basic"

type Tariff struct {
	ID                  uuid.UUID
	Name                TariffName
	ActivePropertyLimit int
	MonthlyPriceKopecks int64
	YearlyPriceKopecks  int64
}
