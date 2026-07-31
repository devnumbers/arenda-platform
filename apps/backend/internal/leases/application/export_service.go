package application

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/xuri/excelize/v2"
)

// ExportOperationRow is a read-model row of a completed (paid/received)
// operation for the property xlsx export.
type ExportOperationRow struct {
	OperationDate    time.Time
	Type             domain.OperationType
	CategoryName     string
	Name             string
	AmountKopecks    int64
	LeaseID          *uuid.UUID
	TenantSurname    *string
	TenantName       *string
	TenantPatronymic *string
	Comment          *string
}

// ExportLeaseRow is a read-model row of a lease joined with its tenant
// contact for the property xlsx export.
type ExportLeaseRow struct {
	LeaseID              uuid.UUID
	Status               domain.LeaseStatus
	StartDate            time.Time
	EndDate              *time.Time
	RentAmountKopecks    int64
	DepositAmountKopecks int64
	PaymentDay           int
	Comment              *string
	TenantContactID      *uuid.UUID
	TenantSurname        *string
	TenantName           *string
	TenantPatronymic     *string
	TenantPhone          *string
	TenantEmail          *string
	TenantComment        *string
}

// ExportFile is a generated export workbook with its download filename.
type ExportFile struct {
	Filename string
	Content  []byte
}

const (
	summarySheetName     = "Сводка"
	exportSheetName      = "Операции"
	leasesSheetName      = "Аренды"
	tenantsSheetName     = "Арендаторы"
	exportMaxRows        = 1_000_000
	exportFilenameMaxLen = 50
)

var exportHeaders = []string{"Дата", "Тип", "Категория", "Название", "Сумма", "Аренда/арендатор", "Комментарий"}

var exportLeaseHeaders = []string{"Арендатор", "Статус", "Дата начала", "Дата окончания", "Ставка ₽/мес", "Депозит ₽", "День платежа", "Комментарий"}

var exportTenantHeaders = []string{"Фамилия", "Имя", "Отчество", "Телефон", "Email", "Связанная аренда/период", "Комментарий"}

// ExportService builds the property xlsx export.
type ExportService struct {
	operations OperationRepository
	leases     LeaseRepository
	properties PropertyRepository
	clock      clock.Clock
	logger     *slog.Logger
}

// NewExportService creates the property export use case.
func NewExportService(operations OperationRepository, leases LeaseRepository, properties PropertyRepository, clk clock.Clock, logger *slog.Logger) *ExportService {
	return &ExportService{operations: operations, leases: leases, properties: properties, clock: clk, logger: logger}
}

// ExportProperty returns the xlsx workbook with the property summary,
// operations, leases, and tenants.
func (s *ExportService) ExportProperty(ctx context.Context, ownerID, propertyID uuid.UUID) (ExportFile, error) {
	name, err := s.properties.GetNameByOwner(ctx, propertyID, ownerID)
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: get property name: %w", err)
	}
	if name == "" {
		return ExportFile{}, ErrNotFound
	}

	rows, err := s.operations.ListCompletedForExport(ctx, ownerID, propertyID)
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: list operations: %w", err)
	}
	if len(rows) > exportMaxRows {
		s.logger.WarnContext(ctx, "property export truncated", slog.String("property_id", propertyID.String()), slog.Int("rows", len(rows)))
		rows = rows[:exportMaxRows]
	}

	leases, err := s.leases.ListWithTenantForExport(ctx, ownerID, propertyID)
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: list leases: %w", err)
	}

	summary, err := s.operations.GetPropertyOperationsSummary(ctx, ownerID, propertyID, s.clock.Now())
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: get summary: %w", err)
	}

	months, err := s.operations.GetPropertyFinanceByMonth(ctx, ownerID, propertyID)
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: get finance by month: %w", err)
	}

	categories, err := s.operations.GetPropertyFinanceByCategory(ctx, ownerID, propertyID)
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: get finance by category: %w", err)
	}

	content, err := buildExportWorkbook(name, summary, months, categories, rows, leases)
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: build workbook: %w", err)
	}

	filename := fmt.Sprintf("Объект_%s_экспорт_%s.xlsx", sanitizeExportFilename(name), s.clock.Now().Format("02.01.2006"))
	return ExportFile{Filename: filename, Content: content}, nil
}

// buildExportWorkbook renders the "Сводка", "Операции", "Аренды", and
// "Арендаторы" sheets in memory.
func buildExportWorkbook(name string, summary OperationsSummary, months []FinanceReportMonthRow, categories []FinanceReportCategoryRow, rows []ExportOperationRow, leases []ExportLeaseRow) (_ []byte, err error) {
	f := excelize.NewFile()
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close workbook: %w", closeErr)
		}
	}()

	if err := f.SetSheetName("Sheet1", summarySheetName); err != nil {
		return nil, fmt.Errorf("rename sheet: %w", err)
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true},
		Fill:   excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"D9E1F2"}},
		Border: []excelize.Border{{Type: "bottom", Color: "000000", Style: 1}},
	})
	if err != nil {
		return nil, fmt.Errorf("header style: %w", err)
	}
	boldStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, fmt.Errorf("bold style: %w", err)
	}
	dateStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: new("DD.MM.YYYY")})
	if err != nil {
		return nil, fmt.Errorf("date style: %w", err)
	}
	moneyStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: new(`#,##0.00" ₽"`)})
	if err != nil {
		return nil, fmt.Errorf("money style: %w", err)
	}

	if err := renderSummarySheet(f, name, summary, months, categories, headerStyle, boldStyle, moneyStyle); err != nil {
		return nil, fmt.Errorf("summary sheet: %w", err)
	}

	if _, err := f.NewSheet(exportSheetName); err != nil {
		return nil, fmt.Errorf("new sheet: %w", err)
	}

	for i, header := range exportHeaders {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, fmt.Errorf("header cell: %w", err)
		}
		if err := f.SetCellStr(exportSheetName, cell, header); err != nil {
			return nil, fmt.Errorf("set header: %w", err)
		}
	}
	if err := f.SetCellStyle(exportSheetName, "A1", "G1", headerStyle); err != nil {
		return nil, fmt.Errorf("style header: %w", err)
	}

	for i, row := range rows {
		rowNum := i + 2
		dateCell, err := excelize.CoordinatesToCellName(1, rowNum)
		if err != nil {
			return nil, fmt.Errorf("date cell: %w", err)
		}
		if err := f.SetCellValue(exportSheetName, dateCell, row.OperationDate); err != nil {
			return nil, fmt.Errorf("set date: %w", err)
		}
		if err := f.SetCellStyle(exportSheetName, dateCell, dateCell, dateStyle); err != nil {
			return nil, fmt.Errorf("style date: %w", err)
		}

		opType := "Расход"
		if row.Type == domain.OperationTypeIncome {
			opType = "Доход"
		}
		if err := f.SetCellStr(exportSheetName, "B"+strconv.Itoa(rowNum), opType); err != nil {
			return nil, fmt.Errorf("set type: %w", err)
		}
		if err := f.SetCellStr(exportSheetName, "C"+strconv.Itoa(rowNum), row.CategoryName); err != nil {
			return nil, fmt.Errorf("set category: %w", err)
		}
		if err := f.SetCellStr(exportSheetName, "D"+strconv.Itoa(rowNum), row.Name); err != nil {
			return nil, fmt.Errorf("set name: %w", err)
		}

		amountCell := "E" + strconv.Itoa(rowNum)
		if err := f.SetCellFloat(exportSheetName, amountCell, float64(row.AmountKopecks)/100, 2, 64); err != nil {
			return nil, fmt.Errorf("set amount: %w", err)
		}
		if err := f.SetCellStyle(exportSheetName, amountCell, amountCell, moneyStyle); err != nil {
			return nil, fmt.Errorf("style amount: %w", err)
		}

		if err := f.SetCellStr(exportSheetName, "F"+strconv.Itoa(rowNum), exportTenantName(row)); err != nil {
			return nil, fmt.Errorf("set tenant: %w", err)
		}
		comment := ""
		if row.Comment != nil {
			comment = *row.Comment
		}
		if err := f.SetCellStr(exportSheetName, "G"+strconv.Itoa(rowNum), comment); err != nil {
			return nil, fmt.Errorf("set comment: %w", err)
		}
	}

	lastRow := len(rows) + 1
	if err := f.AutoFilter(exportSheetName, fmt.Sprintf("A1:G%d", lastRow), nil); err != nil {
		return nil, fmt.Errorf("auto filter: %w", err)
	}
	if err := f.SetPanes(exportSheetName, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return nil, fmt.Errorf("freeze panes: %w", err)
	}
	colWidths := []struct {
		col   string
		width float64
	}{
		{"A", 12},
		{"B", 10},
		{"C", 22},
		{"D", 34},
		{"E", 16},
		{"F", 30},
		{"G", 40},
	}
	for _, cw := range colWidths {
		if err := f.SetColWidth(exportSheetName, cw.col, cw.col, cw.width); err != nil {
			return nil, fmt.Errorf("column %s width: %w", cw.col, err)
		}
	}

	if err := renderLeasesSheet(f, leases, headerStyle, dateStyle, moneyStyle); err != nil {
		return nil, fmt.Errorf("leases sheet: %w", err)
	}

	if err := renderTenantsSheet(f, leases, headerStyle); err != nil {
		return nil, fmt.Errorf("tenants sheet: %w", err)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write workbook: %w", err)
	}
	return buf.Bytes(), nil
}

// renderLeasesSheet fills the "Аренды" sheet with one row per lease.
func renderLeasesSheet(f *excelize.File, leases []ExportLeaseRow, headerStyle, dateStyle, moneyStyle int) error {
	if _, err := f.NewSheet(leasesSheetName); err != nil {
		return fmt.Errorf("new sheet: %w", err)
	}

	for i, header := range exportLeaseHeaders {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return fmt.Errorf("header cell: %w", err)
		}
		if err := f.SetCellStr(leasesSheetName, cell, header); err != nil {
			return fmt.Errorf("set header: %w", err)
		}
	}
	if err := f.SetCellStyle(leasesSheetName, "A1", "H1", headerStyle); err != nil {
		return fmt.Errorf("style header: %w", err)
	}

	for i, lease := range leases {
		rowNum := i + 2
		row := strconv.Itoa(rowNum)

		if err := f.SetCellStr(leasesSheetName, "A"+row, exportLeaseTenantName(lease)); err != nil {
			return fmt.Errorf("set tenant: %w", err)
		}
		if err := f.SetCellStr(leasesSheetName, "B"+row, exportLeaseStatusLabel(lease.Status)); err != nil {
			return fmt.Errorf("set status: %w", err)
		}

		startCell := "C" + row
		if err := f.SetCellValue(leasesSheetName, startCell, lease.StartDate); err != nil {
			return fmt.Errorf("set start date: %w", err)
		}
		if err := f.SetCellStyle(leasesSheetName, startCell, startCell, dateStyle); err != nil {
			return fmt.Errorf("style start date: %w", err)
		}

		endCell := "D" + row
		if lease.EndDate == nil {
			if err := f.SetCellStr(leasesSheetName, endCell, "—"); err != nil {
				return fmt.Errorf("set end date: %w", err)
			}
		} else {
			if err := f.SetCellValue(leasesSheetName, endCell, *lease.EndDate); err != nil {
				return fmt.Errorf("set end date: %w", err)
			}
			if err := f.SetCellStyle(leasesSheetName, endCell, endCell, dateStyle); err != nil {
				return fmt.Errorf("style end date: %w", err)
			}
		}

		rentCell := "E" + row
		if err := f.SetCellFloat(leasesSheetName, rentCell, float64(lease.RentAmountKopecks)/100, 2, 64); err != nil {
			return fmt.Errorf("set rent: %w", err)
		}
		if err := f.SetCellStyle(leasesSheetName, rentCell, rentCell, moneyStyle); err != nil {
			return fmt.Errorf("style rent: %w", err)
		}

		depositCell := "F" + row
		if err := f.SetCellFloat(leasesSheetName, depositCell, float64(lease.DepositAmountKopecks)/100, 2, 64); err != nil {
			return fmt.Errorf("set deposit: %w", err)
		}
		if err := f.SetCellStyle(leasesSheetName, depositCell, depositCell, moneyStyle); err != nil {
			return fmt.Errorf("style deposit: %w", err)
		}

		if err := f.SetCellInt(leasesSheetName, "G"+row, int64(lease.PaymentDay)); err != nil {
			return fmt.Errorf("set payment day: %w", err)
		}

		if err := f.SetCellStr(leasesSheetName, "H"+row, stringOrEmpty(lease.Comment)); err != nil {
			return fmt.Errorf("set comment: %w", err)
		}
	}

	if err := f.SetPanes(leasesSheetName, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return fmt.Errorf("freeze panes: %w", err)
	}
	colWidths := []struct {
		col   string
		width float64
	}{
		{"A", 30},
		{"B", 18},
		{"C", 14},
		{"D", 14},
		{"E", 16},
		{"F", 14},
		{"G", 14},
		{"H", 40},
	}
	for _, cw := range colWidths {
		if err := f.SetColWidth(leasesSheetName, cw.col, cw.col, cw.width); err != nil {
			return fmt.Errorf("column %s width: %w", cw.col, err)
		}
	}
	return nil
}

// renderTenantsSheet fills the "Арендаторы" sheet with one row per tenant
// contact linked to at least one lease of the property.
func renderTenantsSheet(f *excelize.File, leases []ExportLeaseRow, headerStyle int) error {
	if _, err := f.NewSheet(tenantsSheetName); err != nil {
		return fmt.Errorf("new sheet: %w", err)
	}

	for i, header := range exportTenantHeaders {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return fmt.Errorf("header cell: %w", err)
		}
		if err := f.SetCellStr(tenantsSheetName, cell, header); err != nil {
			return fmt.Errorf("set header: %w", err)
		}
	}
	if err := f.SetCellStyle(tenantsSheetName, "A1", "G1", headerStyle); err != nil {
		return fmt.Errorf("style header: %w", err)
	}

	// Deduplicate contacts keeping the first occurrence: leases are sorted by
	// start_date DESC, so the first row of a contact is its most recent lease.
	type tenantEntry struct {
		row     ExportLeaseRow
		periods []string
	}
	var order []uuid.UUID
	entries := make(map[uuid.UUID]*tenantEntry)
	for _, lease := range leases {
		if lease.TenantContactID == nil {
			continue
		}
		id := *lease.TenantContactID
		entry, ok := entries[id]
		if !ok {
			entry = &tenantEntry{row: lease}
			entries[id] = entry
			order = append(order, id)
		}
		entry.periods = append(entry.periods, exportLeasePeriod(lease))
	}

	for i, id := range order {
		entry := entries[id]
		row := strconv.Itoa(i + 2)
		cells := []string{
			stringOrEmpty(entry.row.TenantSurname),
			stringOrEmpty(entry.row.TenantName),
			stringOrEmpty(entry.row.TenantPatronymic),
			stringOrEmpty(entry.row.TenantPhone),
			stringOrEmpty(entry.row.TenantEmail),
			strings.Join(entry.periods, "; "),
			stringOrEmpty(entry.row.TenantComment),
		}
		for col, value := range cells {
			cell, err := excelize.CoordinatesToCellName(col+1, i+2)
			if err != nil {
				return fmt.Errorf("tenant cell: %w", err)
			}
			if err := f.SetCellStr(tenantsSheetName, cell, value); err != nil {
				return fmt.Errorf("set tenant %s: %w", row, err)
			}
		}
	}

	if err := f.SetPanes(tenantsSheetName, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return fmt.Errorf("freeze panes: %w", err)
	}
	colWidths := []struct {
		col   string
		width float64
	}{
		{"A", 18},
		{"B", 18},
		{"C", 18},
		{"D", 18},
		{"E", 26},
		{"F", 44},
		{"G", 40},
	}
	for _, cw := range colWidths {
		if err := f.SetColWidth(tenantsSheetName, cw.col, cw.col, cw.width); err != nil {
			return fmt.Errorf("column %s width: %w", cw.col, err)
		}
	}
	return nil
}

// exportLeaseStatusLabel renders the lease status in Russian, mirroring the
// frontend canonical labels (widgets/tenant-detail/lib/get-lease-status-label.ts).
func exportLeaseStatusLabel(status domain.LeaseStatus) string {
	switch status {
	case domain.LeaseStatusAwaitingStart:
		return "Скоро начнётся"
	case domain.LeaseStatusActive:
		return "Активна"
	case domain.LeaseStatusRequiresAction:
		return "Требует действия"
	case domain.LeaseStatusCompleted:
		return "Завершена"
	case domain.LeaseStatusArchived:
		return "В архиве"
	default:
		return string(status)
	}
}

// exportLeaseTenantName renders the lease tenant as "Фамилия Имя Отчество",
// or an em dash when the lease has no tenant contact.
func exportLeaseTenantName(lease ExportLeaseRow) string {
	if lease.TenantContactID == nil {
		return "—"
	}
	return exportFIO(lease.TenantSurname, lease.TenantName, lease.TenantPatronymic)
}

// exportFIO joins the non-empty name parts with single spaces as
// "Фамилия Имя Отчество", or an em dash when every part is empty.
func exportFIO(surname, name, patronymic *string) string {
	var parts []string
	for _, part := range []*string{surname, name, patronymic} {
		if part != nil && *part != "" {
			parts = append(parts, *part)
		}
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, " ")
}

// exportLeasePeriod renders the lease period as "ДД.ММ.ГГГГ — ДД.ММ.ГГГГ";
// an open end date becomes "— настоящее время".
func exportLeasePeriod(lease ExportLeaseRow) string {
	const layout = "02.01.2006"
	start := lease.StartDate.Format(layout)
	if lease.EndDate == nil {
		return start + " — настоящее время"
	}
	return start + " — " + lease.EndDate.Format(layout)
}

// renderSummarySheet fills the "Сводка" sheet: the property title, the
// all-time totals, and the finance breakdowns by month and by category.
func renderSummarySheet(f *excelize.File, name string, summary OperationsSummary, months []FinanceReportMonthRow, categories []FinanceReportCategoryRow, headerStyle, boldStyle, moneyStyle int) error {
	setMoney := func(cell string, kopecks int64) error {
		if err := f.SetCellFloat(summarySheetName, cell, float64(kopecks)/100, 2, 64); err != nil {
			return fmt.Errorf("set amount %s: %w", cell, err)
		}
		if err := f.SetCellStyle(summarySheetName, cell, cell, moneyStyle); err != nil {
			return fmt.Errorf("style amount %s: %w", cell, err)
		}
		return nil
	}

	if err := f.SetCellStr(summarySheetName, "A1", "Объект: "+name); err != nil {
		return fmt.Errorf("set title: %w", err)
	}
	if err := f.SetCellStyle(summarySheetName, "A1", "A1", boldStyle); err != nil {
		return fmt.Errorf("style title: %w", err)
	}

	if err := f.SetCellStr(summarySheetName, "A3", "Итоги за всё время"); err != nil {
		return fmt.Errorf("set totals header: %w", err)
	}
	if err := f.SetCellStyle(summarySheetName, "A3", "B3", headerStyle); err != nil {
		return fmt.Errorf("style totals header: %w", err)
	}
	totals := []struct {
		label   string
		kopecks int64
	}{
		{"Доход", summary.AllTimeIncomeKopecks},
		{"Расход", summary.AllTimeExpenseKopecks},
		{"Прибыль", summary.AllTimeProfitKopecks},
	}
	for i, total := range totals {
		rowNum := 4 + i
		if err := f.SetCellStr(summarySheetName, "A"+strconv.Itoa(rowNum), total.label); err != nil {
			return fmt.Errorf("set totals label: %w", err)
		}
		if err := setMoney("B"+strconv.Itoa(rowNum), total.kopecks); err != nil {
			return err
		}
	}

	monthHeaderRow := 8
	monthHeaders := []string{"Месяц", "Доход", "Расход", "Прибыль"}
	for i, header := range monthHeaders {
		cell, err := excelize.CoordinatesToCellName(i+1, monthHeaderRow)
		if err != nil {
			return fmt.Errorf("month header cell: %w", err)
		}
		if err := f.SetCellStr(summarySheetName, cell, header); err != nil {
			return fmt.Errorf("set month header: %w", err)
		}
	}
	if err := f.SetCellStyle(summarySheetName, "A8", "D8", headerStyle); err != nil {
		return fmt.Errorf("style month header: %w", err)
	}
	for i, month := range months {
		rowNum := monthHeaderRow + 1 + i
		row := strconv.Itoa(rowNum)
		if err := f.SetCellStr(summarySheetName, "A"+row, exportMonthLabel(month.Month)); err != nil {
			return fmt.Errorf("set month: %w", err)
		}
		if err := setMoney("B"+row, month.IncomeKopecks); err != nil {
			return err
		}
		if err := setMoney("C"+row, month.ExpenseKopecks); err != nil {
			return err
		}
		if err := setMoney("D"+row, month.IncomeKopecks-month.ExpenseKopecks); err != nil {
			return err
		}
	}

	categoryHeaderRow := monthHeaderRow + len(months) + 2
	categoryHeaders := []string{"Категория", "Тип", "Сумма"}
	for i, header := range categoryHeaders {
		cell, err := excelize.CoordinatesToCellName(i+1, categoryHeaderRow)
		if err != nil {
			return fmt.Errorf("category header cell: %w", err)
		}
		if err := f.SetCellStr(summarySheetName, cell, header); err != nil {
			return fmt.Errorf("set category header: %w", err)
		}
	}
	categoryHeaderStart := "A" + strconv.Itoa(categoryHeaderRow)
	categoryHeaderEnd := "C" + strconv.Itoa(categoryHeaderRow)
	if err := f.SetCellStyle(summarySheetName, categoryHeaderStart, categoryHeaderEnd, headerStyle); err != nil {
		return fmt.Errorf("style category header: %w", err)
	}
	for i, category := range categories {
		row := strconv.Itoa(categoryHeaderRow + 1 + i)
		if err := f.SetCellStr(summarySheetName, "A"+row, category.CategoryName); err != nil {
			return fmt.Errorf("set category: %w", err)
		}
		opType := "Расход"
		if category.Type == domain.OperationTypeIncome {
			opType = "Доход"
		}
		if err := f.SetCellStr(summarySheetName, "B"+row, opType); err != nil {
			return fmt.Errorf("set category type: %w", err)
		}
		if err := setMoney("C"+row, category.TotalKopecks); err != nil {
			return err
		}
	}

	if err := f.SetPanes(summarySheetName, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return fmt.Errorf("freeze panes: %w", err)
	}
	colWidths := []struct {
		col   string
		width float64
	}{
		{"A", 22},
		{"B", 16},
		{"C", 16},
		{"D", 16},
	}
	for _, cw := range colWidths {
		if err := f.SetColWidth(summarySheetName, cw.col, cw.col, cw.width); err != nil {
			return fmt.Errorf("column %s width: %w", cw.col, err)
		}
	}
	return nil
}

// exportRussianMonths maps time.Month to the Russian nominative month name.
var exportRussianMonths = [...]string{
	"", "Январь", "Февраль", "Март", "Апрель", "Май", "Июнь",
	"Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь",
}

// exportMonthLabel renders a month as "Январь 2025".
func exportMonthLabel(month time.Time) string {
	return exportRussianMonths[month.Month()] + " " + strconv.Itoa(month.Year())
}

// exportTenantName renders the lease tenant as "Фамилия Имя Отчество", or an
// em dash when the operation is not linked to a lease or has no tenant data.
func exportTenantName(row ExportOperationRow) string {
	if row.LeaseID == nil {
		return "—"
	}
	return exportFIO(row.TenantSurname, row.TenantName, row.TenantPatronymic)
}

// sanitizeExportFilename makes the property name safe for a filename:
// replaces path/hostile characters and whitespace runs with underscores,
// truncates to 50 runes, and falls back to "объект" when empty.
func sanitizeExportFilename(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	lastUnderscore := false
	for _, r := range strings.TrimSpace(name) {
		switch r {
		case '/', '\\', '"', '*', ':', '?', '[', ']', ' ':
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		default:
			b.WriteRune(r)
			lastUnderscore = false
		}
	}
	runes := []rune(b.String())
	if len(runes) > exportFilenameMaxLen {
		runes = runes[:exportFilenameMaxLen]
	}
	if len(runes) == 0 {
		return "объект"
	}
	return string(runes)
}
