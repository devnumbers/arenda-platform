package httpapi

import (
	"net/http"

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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	report, err := h.svc.GetFinanceReport(r.Context(), ownerID, params.From.Time, params.To.Time)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}

	resp := openapi.FinanceReportResponse{
		Period: openapi.FinanceReportPeriod{
			From: openapi_types.Date{Time: params.From.Time},
			To:   openapi_types.Date{Time: params.To.Time},
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
			Category:     openapi.OperationCategory(row.Category),
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
