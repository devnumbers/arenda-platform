package http

import (
	"net/http"
	"time"

	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
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
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var from, to *time.Time
	if params.From != nil {
		from = &params.From.Time
	}
	if params.To != nil {
		to = &params.To.Time
	}

	report, err := h.svc.GetFinanceReport(r.Context(), actor, from, to)
	if err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
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
			IncomeKopecks:  report.Totals.IncomeKopecks,
			ExpenseKopecks: report.Totals.ExpenseKopecks,
			ProfitKopecks:  report.Totals.IncomeKopecks - report.Totals.ExpenseKopecks,
		},
	}
	for _, row := range report.ByProperty {
		resp.ByProperty = append(resp.ByProperty, openapi.FinanceReportPropertyRow{
			PropertyId:     leasesdomain.PropertyIDPtr(row.PropertyID),
			IncomeKopecks:  row.IncomeKopecks,
			ExpenseKopecks: row.ExpenseKopecks,
			ProfitKopecks:  row.IncomeKopecks - row.ExpenseKopecks,
		})
	}
	for _, row := range report.ByCategory {
		resp.ByCategory = append(resp.ByCategory, openapi.FinanceReportCategoryRow{
			Type:         openapi.OperationType(row.Type),
			CategoryId:   row.CategoryID,
			CategoryName: row.CategoryName,
			TotalKopecks: row.TotalKopecks,
		})
	}
	for _, row := range report.ByMonth {
		resp.ByMonth = append(resp.ByMonth, openapi.FinanceReportMonthRow{
			Month:          openapi_types.Date{Time: row.Month},
			IncomeKopecks:  row.IncomeKopecks,
			ExpenseKopecks: row.ExpenseKopecks,
			ProfitKopecks:  row.IncomeKopecks - row.ExpenseKopecks,
		})
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}
