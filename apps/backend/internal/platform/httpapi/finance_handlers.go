package httpapi

import (
	"net/http"
	"time"

	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type FinanceHandlers struct {
	svc *leasesapp.OperationService
}

func NewFinanceHandlers(svc *leasesapp.OperationService) *FinanceHandlers {
	return &FinanceHandlers{svc: svc}
}

func (h *FinanceHandlers) GetFinanceReport(w http.ResponseWriter, r *http.Request, params openapi.GetFinanceReportParams) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var from, to *time.Time
	if params.From != nil {
		from = &params.From.Time
	}
	if params.To != nil {
		to = &params.To.Time
	}

	report, err := h.svc.GetFinanceReport(r.Context(), ownerID, from, to)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}

	// Period echoes the requested date bounds. When a bound is omitted the report
	// covers all time, so the required FinanceReportPeriod fields are filled with
	// the zero date as an explicit "no boundary" sentinel.
	periodFrom := openapi_types.Date{}
	if params.From != nil {
		periodFrom = *params.From
	}
	periodTo := openapi_types.Date{}
	if params.To != nil {
		periodTo = *params.To
	}

	resp := openapi.FinanceReportResponse{
		Period: openapi.FinanceReportPeriod{
			From: periodFrom,
			To:   periodTo,
		},
		Totals: openapi.FinanceReportTotals{
			IncomeKopecks:  int(report.Totals.IncomeKopecks),
			ExpenseKopecks: int(report.Totals.ExpenseKopecks),
			ProfitKopecks:  int(report.Totals.IncomeKopecks - report.Totals.ExpenseKopecks),
		},
	}
	for _, row := range report.ByProperty {
		profit := row.IncomeKopecks - row.ExpenseKopecks
		resp.ByProperty = append(resp.ByProperty, openapi.FinanceReportPropertyRow{
			PropertyId:     row.PropertyID,
			IncomeKopecks:  int(row.IncomeKopecks),
			ExpenseKopecks: int(row.ExpenseKopecks),
			ProfitKopecks:  int(profit),
		})
	}
	for _, row := range report.ByCategory {
		resp.ByCategory = append(resp.ByCategory, openapi.FinanceReportCategoryRow{
			Type:         openapi.OperationType(row.Type),
			CategoryId:   row.CategoryID,
			CategoryName: row.CategoryName,
			TotalKopecks: int(row.TotalKopecks),
		})
	}
	for _, row := range report.ByMonth {
		profit := row.IncomeKopecks - row.ExpenseKopecks
		resp.ByMonth = append(resp.ByMonth, openapi.FinanceReportMonthRow{
			Month:          openapi_types.Date{Time: row.Month},
			IncomeKopecks:  int(row.IncomeKopecks),
			ExpenseKopecks: int(row.ExpenseKopecks),
			ProfitKopecks:  int(profit),
		})
	}

	writeJSON(r.Context(), w, http.StatusOK, resp)
}
