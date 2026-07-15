package application

import "time"

type OperationsSummary struct {
	MonthlyProfitKopecks  int64
	AllTimeProfitKopecks  int64
	AllTimeIncomeKopecks  int64
	AllTimeExpenseKopecks int64
	OverdueRentCount      int
	OverdueTotalCount     int
	NextPaymentDate       *time.Time
}
