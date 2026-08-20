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
	propdomain "github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
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

// ExportContactRow is a read-model row of a property contact for the property
// xlsx export.
type ExportContactRow struct {
	Name  string
	Phone string
}

// ExportPropertyRow is a read-model row of a property for the xlsx export.
// Type and Attributes use the properties domain catalog types so the
// generated catalog helpers (CatalogKeys, FilterByType, PropertyTypeLabel,
// AttributeFieldLabel, AttributeEnumLabel) work directly.
type ExportPropertyRow struct {
	Name        string
	Type        propdomain.PropertyType
	Address     string
	Description string
	Attributes  propdomain.Attributes
}

// ExportFile is a generated export workbook with its download filename.
type ExportFile struct {
	Filename string
	Content  []byte
}

const (
	objectSheetName      = "Объект"
	exportSheetName      = "Операции"
	monthlySheetName     = "Сводка по месяцам"
	categorySheetName    = "По категориям"
	leasesSheetName      = "Аренды"
	tenantsSheetName     = "Арендаторы"
	contactsSheetName    = "Контакты"
	exportMaxRows        = 1_000_000
	exportFilenameMaxLen = 50

	// Dates across all export sheets render as DD.MM.YYYY.
	exportDateFormat = "02.01.2006"

	// The month-table header row on the monthly sheet; the all-time totals
	// block occupies the rows above it.
	monthlyTableHeaderRow = 6

	// Labels reused across sheets (headers, row labels, month-table columns).
	exportLabelIncome    = "Доход"
	exportLabelExpense   = "Расход"
	exportLabelCategory  = "Категория"
	exportLabelAddress   = "Адрес"
	exportLabelType      = "Тип"
	exportLabelComment   = "Комментарий"
	activePaneBottomLeft = "bottomLeft" // The excelize.Panes.ActivePane value for frozen-top-row sheets.
)

var exportHeaders = []string{
	"Дата", exportLabelType, exportLabelCategory, "Название",
	"Сумма", "Аренда/арендатор", exportLabelComment,
}

var exportLeaseHeaders = []string{
	"Арендатор", "Статус", "Дата начала", "Дата окончания",
	"Ставка ₽/мес", "Депозит ₽", "День платежа", exportLabelComment,
}

var exportTenantHeaders = []string{
	"Фамилия", "Имя", "Отчество", "Телефон", "Email", "Связанная аренда/период", exportLabelComment,
}

var exportContactHeaders = []string{"Имя", "Телефон"}

// ExportService builds the property xlsx export.
type ExportService struct {
	operations OperationRepository
	leases     LeaseRepository
	properties PropertyRepository
	contacts   PropertyContactRepository
	policy     sharedpolicy.Policy
	clock      clock.Clock
	logger     *slog.Logger
}

// NewExportService creates the property export use case.
func NewExportService(
	operations OperationRepository,
	leases LeaseRepository,
	properties PropertyRepository,
	contacts PropertyContactRepository,
	policy sharedpolicy.Policy,
	clk clock.Clock,
	logger *slog.Logger,
) *ExportService {
	return &ExportService{
		operations: operations, leases: leases, properties: properties,
		contacts: contacts, policy: policy, clock: clk, logger: logger,
	}
}

// ExportProperty returns the xlsx workbook with the property card, operations,
// finance summaries, leases, tenants, and contacts.
func (s *ExportService) ExportProperty(ctx context.Context, actor, propertyID uuid.UUID) (ExportFile, error) {
	scope, err := resolveReadScope(ctx, s.policy, s.properties, actor, propertyID)
	if err != nil {
		return ExportFile{}, err
	}

	propRow, err := s.properties.GetForExport(ctx, propertyID, scope)
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: get property: %w", err)
	}

	rows, err := s.operations.ListCompletedForExport(ctx, scope, propertyID)
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: list operations: %w", err)
	}
	if len(rows) > exportMaxRows {
		s.logger.WarnContext(ctx, "property export truncated", slog.String("property_id", propertyID.String()), slog.Int("rows", len(rows)))
		rows = rows[:exportMaxRows]
	}

	leases, err := s.leases.ListWithTenantForExport(ctx, scope, propertyID)
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: list leases: %w", err)
	}

	summary, err := s.operations.GetPropertyOperationsSummary(ctx, scope, propertyID, s.clock.Now())
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: get summary: %w", err)
	}

	months, err := s.operations.GetPropertyFinanceByMonth(ctx, scope, propertyID)
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: get finance by month: %w", err)
	}

	categories, err := s.operations.GetPropertyFinanceByCategory(ctx, scope, propertyID)
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: get finance by category: %w", err)
	}

	contacts, err := s.contacts.ListForExport(ctx, propertyID, scope)
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: list contacts: %w", err)
	}

	data := exportWorkbookData{
		property:   propRow,
		summary:    summary,
		months:     months,
		categories: categories,
		operations: rows,
		leases:     leases,
		contacts:   contacts,
	}
	content, err := buildExportWorkbook(data)
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: build workbook: %w", err)
	}

	filename := fmt.Sprintf("Объект_%s_экспорт_%s.xlsx", sanitizeExportFilename(propRow.Name), s.clock.Now().Format("02.01.2006"))
	return ExportFile{Filename: filename, Content: content}, nil
}

// exportWorkbookData bundles the read models rendered into one export workbook.
type exportWorkbookData struct {
	property   ExportPropertyRow
	summary    OperationsSummary
	months     []FinanceReportMonthRow
	categories []FinanceReportCategoryRow
	operations []ExportOperationRow
	leases     []ExportLeaseRow
	contacts   []ExportContactRow
}

// exportStyles bundles the cell styles shared by the export sheets.
type exportStyles struct {
	header int
	bold   int
	money  int
}

// buildExportWorkbook renders the "Объект", "Операции", "Сводка по месяцам",
// "По категориям", "Аренды", "Арендаторы", and "Контакты" sheets in memory.
// The default Sheet1 is renamed to "Объект" so it stays first in the tab order.
func buildExportWorkbook(data exportWorkbookData) (_ []byte, err error) {
	f := excelize.NewFile()
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close workbook: %w", closeErr)
		}
	}()

	if err := f.SetSheetName("Sheet1", objectSheetName); err != nil {
		return nil, fmt.Errorf("rename sheet: %w", err)
	}

	styles, err := newExportStyles(f)
	if err != nil {
		return nil, err
	}

	if err := renderWorkbookSheets(f, styles, data); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write workbook: %w", err)
	}
	return buf.Bytes(), nil
}

// newExportStyles registers the shared workbook styles: the bold header with
// fill and border, the plain bold label, and the ruble money format.
func newExportStyles(f *excelize.File) (exportStyles, error) {
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true},
		Fill:   excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"D9E1F2"}},
		Border: []excelize.Border{{Type: "bottom", Color: "000000", Style: 1}},
	})
	if err != nil {
		return exportStyles{}, fmt.Errorf("header style: %w", err)
	}
	boldStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return exportStyles{}, fmt.Errorf("bold style: %w", err)
	}
	moneyStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: new(`#,##0.00" ₽"`)})
	if err != nil {
		return exportStyles{}, fmt.Errorf("money style: %w", err)
	}
	return exportStyles{header: headerStyle, bold: boldStyle, money: moneyStyle}, nil
}

// renderWorkbookSheets renders the seven export sheets in tab order: the
// property card first (the renamed default sheet), then one sheet per data
// section. Each data renderer appends its own sheet.
func renderWorkbookSheets(f *excelize.File, styles exportStyles, data exportWorkbookData) error {
	if err := renderObjectSheet(f, data.property, styles.bold); err != nil {
		return fmt.Errorf("object sheet: %w", err)
	}
	if err := renderOperationsSheet(f, data.operations, styles.header, styles.money); err != nil {
		return fmt.Errorf("operations sheet: %w", err)
	}
	if err := renderMonthlySheet(f, data.summary, data.months, styles); err != nil {
		return fmt.Errorf("monthly sheet: %w", err)
	}
	if err := renderCategorySheet(f, data.categories, styles.header, styles.money); err != nil {
		return fmt.Errorf("category sheet: %w", err)
	}
	if err := renderLeasesSheet(f, data.leases, styles.header, styles.money); err != nil {
		return fmt.Errorf("leases sheet: %w", err)
	}
	if err := renderTenantsSheet(f, data.leases, styles.header); err != nil {
		return fmt.Errorf("tenants sheet: %w", err)
	}
	if err := renderContactsSheet(f, data.contacts, styles.header); err != nil {
		return fmt.Errorf("contacts sheet: %w", err)
	}
	return nil
}

// renderOperationsSheet fills the "Операции" sheet: the header row with filter
// and frozen panes, one row per operation (date, type, category, name, amount
// in rubles, tenant, comment), and per-column widths.
func renderOperationsSheet(f *excelize.File, rows []ExportOperationRow, headerStyle, moneyStyle int) error {
	if err := addSheet(f, exportSheetName); err != nil {
		return err
	}
	if err := writeSheetHeader(f, exportSheetName, exportHeaders, 1, headerStyle); err != nil {
		return err
	}
	if err := writeOperationRows(f, rows, moneyStyle); err != nil {
		return err
	}
	lastRow := len(rows) + 1
	if err := f.AutoFilter(exportSheetName, fmt.Sprintf("A1:G%d", lastRow), nil); err != nil {
		return fmt.Errorf("auto filter: %w", err)
	}
	if err := freezeSheetRows(f, exportSheetName, 1); err != nil {
		return err
	}
	widths := []exportColWidth{{"A", 12}, {"B", 10}, {"C", 22}, {"D", 34}, {"E", 16}, {"F", 30}, {"G", 40}}
	return setSheetColWidths(f, exportSheetName, widths)
}

// writeOperationRows writes one row per completed operation.
func writeOperationRows(f *excelize.File, rows []ExportOperationRow, moneyStyle int) error {
	for i, row := range rows {
		if err := writeOperationRow(f, row, i+2, moneyStyle); err != nil {
			return err
		}
	}
	return nil
}

// writeOperationRow writes one operation row: date, type, category, name,
// amount in rubles, tenant, comment.
func writeOperationRow(f *excelize.File, row ExportOperationRow, rowNum, moneyStyle int) error {
	r := strconv.Itoa(rowNum)
	if err := f.SetCellStr(exportSheetName, "A"+r, row.OperationDate.Format(exportDateFormat)); err != nil {
		return fmt.Errorf("set date: %w", err)
	}
	if err := f.SetCellStr(exportSheetName, "B"+r, exportOperationTypeLabel(row.Type)); err != nil {
		return fmt.Errorf("set type: %w", err)
	}
	if err := f.SetCellStr(exportSheetName, "C"+r, row.CategoryName); err != nil {
		return fmt.Errorf("set category: %w", err)
	}
	if err := f.SetCellStr(exportSheetName, "D"+r, row.Name); err != nil {
		return fmt.Errorf("set name: %w", err)
	}
	if err := setMoneyCell(f, exportSheetName, "E"+r, row.AmountKopecks, moneyStyle); err != nil {
		return err
	}
	if err := f.SetCellStr(exportSheetName, "F"+r, exportTenantName(row)); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	if err := f.SetCellStr(exportSheetName, "G"+r, stringOrEmpty(row.Comment)); err != nil {
		return fmt.Errorf("set comment: %w", err)
	}
	return nil
}

// objectSheetPair is one Параметр — Значение row of the property card.
type objectSheetPair struct {
	label string
	value string
}

// renderObjectSheet fills the "Объект" sheet with a property card: the basic
// fields (Название, Тип, Адрес, Описание) and the catalog characteristics for
// the property type (Комнаты, Этаж, …), each as a Параметр — Значение pair.
// Empty/nil characteristics are skipped; characteristics are ordered by the
// sorted catalog keys for the type.
func renderObjectSheet(f *excelize.File, propRow ExportPropertyRow, boldStyle int) error {
	basic := []objectSheetPair{
		{"Название", propRow.Name},
		{exportLabelType, propdomain.PropertyTypeLabel(propRow.Type)},
		{exportLabelAddress, propRow.Address},
		{"Описание", propRow.Description},
	}
	row, err := writeObjectPairs(f, basic, boldStyle, 1)
	if err != nil {
		return err
	}
	if err := writeCharacteristicsSection(f, propRow, boldStyle, row); err != nil {
		return err
	}
	// Column widths: A narrow for labels, B wide for values/description.
	widths := []exportColWidth{{"A", 20}, {"B", 50}}
	return setSheetColWidths(f, objectSheetName, widths)
}

// writeObjectPairs writes Параметр — Значение pairs starting at startRow and
// returns the next free row. Labels in column A get the bold style.
func writeObjectPairs(f *excelize.File, pairs []objectSheetPair, boldStyle, startRow int) (int, error) {
	row := startRow
	for _, pair := range pairs {
		r := strconv.Itoa(row)
		if err := f.SetCellStr(objectSheetName, "A"+r, pair.label); err != nil {
			return 0, fmt.Errorf("set object label %s: %w", r, err)
		}
		if err := f.SetCellStr(objectSheetName, "B"+r, pair.value); err != nil {
			return 0, fmt.Errorf("set object value %s: %w", r, err)
		}
		if err := f.SetCellStyle(objectSheetName, "A"+r, "A"+r, boldStyle); err != nil {
			return 0, fmt.Errorf("style object label %s: %w", r, err)
		}
		row++
	}
	return row, nil
}

// writeCharacteristicsSection appends the catalog characteristics of the
// property type to the card. A blank separator row and a bold "Характеристики"
// header precede the first characteristic; nil/missing fields are skipped and
// the order follows the sorted catalog keys for the type.
func writeCharacteristicsSection(f *excelize.File, propRow ExportPropertyRow, boldStyle, startRow int) error {
	filtered := propRow.Attributes.FilterByType(propRow.Type)
	row := startRow
	wroteHeader := false
	for _, key := range propdomain.CatalogKeys(propRow.Type) {
		value, ok := filtered[key]
		if !ok || value == nil {
			continue
		}
		if !wroteHeader {
			row++ // Blank separator row before the section header.
			if err := writeCharacteristicsHeader(f, boldStyle, row); err != nil {
				return err
			}
			row++
			wroteHeader = true
		}
		pair := objectSheetPair{
			propdomain.AttributeFieldLabel(propRow.Type, key),
			formatAttributeValue(propRow.Type, key, value),
		}
		next, err := writeObjectPairs(f, []objectSheetPair{pair}, boldStyle, row)
		if err != nil {
			return err
		}
		row = next
	}
	return nil
}

// writeCharacteristicsHeader writes the bold "Характеристики" section header.
func writeCharacteristicsHeader(f *excelize.File, boldStyle, row int) error {
	r := strconv.Itoa(row)
	if err := f.SetCellStr(objectSheetName, "A"+r, "Характеристики"); err != nil {
		return fmt.Errorf("set characteristics header: %w", err)
	}
	if err := f.SetCellStyle(objectSheetName, "A"+r, "A"+r, boldStyle); err != nil {
		return fmt.Errorf("style characteristics header: %w", err)
	}
	return nil
}

// formatAttributeValue renders a catalog attribute value for display. Enum
// fields are resolved to their Russian label via AttributeEnumLabel; all other
// kinds fall back to fmt.Sprint(value).
func formatAttributeValue(propType propdomain.PropertyType, key string, value any) string {
	if enumLabel := propdomain.AttributeEnumLabel(propType, key, fmt.Sprint(value)); enumLabel != "" {
		return enumLabel
	}
	return fmt.Sprint(value)
}

// renderMonthlySheet fills the "Сводка по месяцам" sheet: all-time totals
// (Доход/Расход/Прибыль) and the month-by-month finance breakdown.
func renderMonthlySheet(f *excelize.File, summary OperationsSummary, months []FinanceReportMonthRow, styles exportStyles) error {
	if err := addSheet(f, monthlySheetName); err != nil {
		return err
	}
	if err := writeMonthlyTotals(f, summary, styles.bold, styles.money); err != nil {
		return err
	}
	if err := writeMonthTable(f, months, styles); err != nil {
		return err
	}
	if err := freezeSheetRows(f, monthlySheetName, monthlyTableHeaderRow); err != nil {
		return err
	}
	widths := []exportColWidth{{"A", 22}, {"B", 16}, {"C", 16}, {"D", 16}}
	return setSheetColWidths(f, monthlySheetName, widths)
}

// writeMonthlyTotals writes the all-time totals block (Доход/Расход/Прибыль)
// under its bold "Итоги за всё время" title.
func writeMonthlyTotals(f *excelize.File, summary OperationsSummary, boldStyle, moneyStyle int) error {
	if err := f.SetCellStr(monthlySheetName, "A1", "Итоги за всё время"); err != nil {
		return fmt.Errorf("set totals header: %w", err)
	}
	if err := f.SetCellStyle(monthlySheetName, "A1", "A1", boldStyle); err != nil {
		return fmt.Errorf("style totals header: %w", err)
	}
	totals := []struct {
		label   string
		kopecks int64
	}{
		{exportLabelIncome, summary.AllTimeIncomeKopecks},
		{exportLabelExpense, summary.AllTimeExpenseKopecks},
		{"Прибыль", summary.AllTimeProfitKopecks},
	}
	for i, total := range totals {
		rowNum := 2 + i
		if err := f.SetCellStr(monthlySheetName, "A"+strconv.Itoa(rowNum), total.label); err != nil {
			return fmt.Errorf("set totals label: %w", err)
		}
		if err := setMoneyCell(f, monthlySheetName, "B"+strconv.Itoa(rowNum), total.kopecks, moneyStyle); err != nil {
			return err
		}
	}
	return nil
}

// writeMonthTable writes the "По месяцам" section: its bold title, the styled
// header row, and one finance breakdown row per month.
func writeMonthTable(f *excelize.File, months []FinanceReportMonthRow, styles exportStyles) error {
	titleCell := "A" + strconv.Itoa(monthlyTableHeaderRow-1)
	if err := f.SetCellStr(monthlySheetName, titleCell, "По месяцам"); err != nil {
		return fmt.Errorf("set months section header: %w", err)
	}
	if err := f.SetCellStyle(monthlySheetName, titleCell, titleCell, styles.bold); err != nil {
		return fmt.Errorf("style months section header: %w", err)
	}
	monthHeaders := []string{"Месяц", exportLabelIncome, exportLabelExpense, "Прибыль"}
	if err := writeSheetHeader(f, monthlySheetName, monthHeaders, monthlyTableHeaderRow, styles.header); err != nil {
		return err
	}
	for i, month := range months {
		rowNum := monthlyTableHeaderRow + 1 + i
		row := strconv.Itoa(rowNum)
		if err := f.SetCellStr(monthlySheetName, "A"+row, exportMonthLabel(month.Month)); err != nil {
			return fmt.Errorf("set month: %w", err)
		}
		if err := setMoneyCell(f, monthlySheetName, "B"+row, month.IncomeKopecks, styles.money); err != nil {
			return err
		}
		if err := setMoneyCell(f, monthlySheetName, "C"+row, month.ExpenseKopecks, styles.money); err != nil {
			return err
		}
		if err := setMoneyCell(f, monthlySheetName, "D"+row, month.IncomeKopecks-month.ExpenseKopecks, styles.money); err != nil {
			return err
		}
	}
	return nil
}

// renderCategorySheet fills the "По категориям" sheet with the finance
// breakdown by operation category.
func renderCategorySheet(f *excelize.File, categories []FinanceReportCategoryRow, headerStyle, moneyStyle int) error {
	if err := addSheet(f, categorySheetName); err != nil {
		return err
	}
	categoryHeaders := []string{exportLabelCategory, exportLabelType, "Сумма"}
	if err := writeSheetHeader(f, categorySheetName, categoryHeaders, 1, headerStyle); err != nil {
		return err
	}
	if err := writeCategoryRows(f, categories, moneyStyle); err != nil {
		return err
	}
	if err := freezeSheetRows(f, categorySheetName, 1); err != nil {
		return err
	}
	widths := []exportColWidth{{"A", 30}, {"B", 12}, {"C", 16}}
	return setSheetColWidths(f, categorySheetName, widths)
}

// writeCategoryRows writes one finance row per operation category: name, type
// label, and total in rubles.
func writeCategoryRows(f *excelize.File, categories []FinanceReportCategoryRow, moneyStyle int) error {
	for i, category := range categories {
		row := strconv.Itoa(2 + i)
		if err := f.SetCellStr(categorySheetName, "A"+row, category.CategoryName); err != nil {
			return fmt.Errorf("set category: %w", err)
		}
		if err := f.SetCellStr(categorySheetName, "B"+row, exportOperationTypeLabel(category.Type)); err != nil {
			return fmt.Errorf("set category type: %w", err)
		}
		if err := setMoneyCell(f, categorySheetName, "C"+row, category.TotalKopecks, moneyStyle); err != nil {
			return err
		}
	}
	return nil
}

// renderLeasesSheet fills the "Аренды" sheet with one row per lease.
func renderLeasesSheet(f *excelize.File, leases []ExportLeaseRow, headerStyle, moneyStyle int) error {
	if err := addSheet(f, leasesSheetName); err != nil {
		return err
	}
	if err := writeSheetHeader(f, leasesSheetName, exportLeaseHeaders, 1, headerStyle); err != nil {
		return err
	}
	if err := writeLeaseRows(f, leases, moneyStyle); err != nil {
		return err
	}
	if err := freezeSheetRows(f, leasesSheetName, 1); err != nil {
		return err
	}
	widths := []exportColWidth{
		{"A", 30}, {"B", 18}, {"C", 14}, {"D", 14}, {"E", 16}, {"F", 14}, {"G", 14}, {"H", 40},
	}
	return setSheetColWidths(f, leasesSheetName, widths)
}

// writeLeaseRows writes one row per lease.
func writeLeaseRows(f *excelize.File, leases []ExportLeaseRow, moneyStyle int) error {
	for i, lease := range leases {
		if err := writeLeaseRow(f, lease, i+2, moneyStyle); err != nil {
			return err
		}
	}
	return nil
}

// writeLeaseRow writes one lease row: tenant, status, dates, rent and deposit
// in rubles, payment day, comment.
func writeLeaseRow(f *excelize.File, lease ExportLeaseRow, rowNum, moneyStyle int) error {
	row := strconv.Itoa(rowNum)
	if err := f.SetCellStr(leasesSheetName, "A"+row, exportLeaseTenantName(lease)); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	if err := f.SetCellStr(leasesSheetName, "B"+row, exportLeaseStatusLabel(lease.Status)); err != nil {
		return fmt.Errorf("set status: %w", err)
	}
	if err := f.SetCellStr(leasesSheetName, "C"+row, lease.StartDate.Format(exportDateFormat)); err != nil {
		return fmt.Errorf("set start date: %w", err)
	}
	if err := f.SetCellStr(leasesSheetName, "D"+row, exportLeaseEndDate(lease)); err != nil {
		return fmt.Errorf("set end date: %w", err)
	}
	if err := setMoneyCell(f, leasesSheetName, "E"+row, lease.RentAmountKopecks, moneyStyle); err != nil {
		return err
	}
	if err := setMoneyCell(f, leasesSheetName, "F"+row, lease.DepositAmountKopecks, moneyStyle); err != nil {
		return err
	}
	if err := f.SetCellInt(leasesSheetName, "G"+row, int64(lease.PaymentDay)); err != nil {
		return fmt.Errorf("set payment day: %w", err)
	}
	if err := f.SetCellStr(leasesSheetName, "H"+row, stringOrEmpty(lease.Comment)); err != nil {
		return fmt.Errorf("set comment: %w", err)
	}
	return nil
}

// exportLeaseEndDate renders the lease end date as DD.MM.YYYY, or an em dash
// for an open-ended lease.
func exportLeaseEndDate(lease ExportLeaseRow) string {
	if lease.EndDate == nil {
		return "—"
	}
	return lease.EndDate.Format(exportDateFormat)
}

// renderTenantsSheet fills the "Арендаторы" sheet with one row per tenant
// contact linked to at least one lease of the property.
func renderTenantsSheet(f *excelize.File, leases []ExportLeaseRow, headerStyle int) error {
	if err := addSheet(f, tenantsSheetName); err != nil {
		return err
	}
	if err := writeSheetHeader(f, tenantsSheetName, exportTenantHeaders, 1, headerStyle); err != nil {
		return err
	}
	order, entries := groupTenantEntries(leases)
	if err := writeTenantRows(f, order, entries); err != nil {
		return err
	}
	if err := freezeSheetRows(f, tenantsSheetName, 1); err != nil {
		return err
	}
	widths := []exportColWidth{
		{"A", 18}, {"B", 18}, {"C", 18}, {"D", 18}, {"E", 26}, {"F", 44}, {"G", 40},
	}
	return setSheetColWidths(f, tenantsSheetName, widths)
}

// tenantEntry is a tenant contact deduplicated across leases: the row of its
// most recent lease plus the periods of every lease it appears in.
type tenantEntry struct {
	row     ExportLeaseRow
	periods []string
}

// groupTenantEntries deduplicates tenant contacts keeping the first
// occurrence: leases are sorted by start_date DESC, so the first row of a
// contact is its most recent lease. It returns the first-seen contact order
// and the entries keyed by contact id.
func groupTenantEntries(leases []ExportLeaseRow) (order []uuid.UUID, entries map[uuid.UUID]*tenantEntry) {
	entries = make(map[uuid.UUID]*tenantEntry)
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
	return order, entries
}

// writeTenantRows writes one row per deduplicated tenant contact: FIO, phone,
// email, the semicolon-joined lease periods, and the contact comment.
func writeTenantRows(f *excelize.File, order []uuid.UUID, entries map[uuid.UUID]*tenantEntry) error {
	for i, id := range order {
		entry := entries[id]
		cells := []string{
			stringOrEmpty(entry.row.TenantSurname),
			stringOrEmpty(entry.row.TenantName),
			stringOrEmpty(entry.row.TenantPatronymic),
			stringOrEmpty(entry.row.TenantPhone),
			stringOrEmpty(entry.row.TenantEmail),
			strings.Join(entry.periods, "; "),
			stringOrEmpty(entry.row.TenantComment),
		}
		if err := writeSheetRow(f, tenantsSheetName, i+2, cells); err != nil {
			return err
		}
	}
	return nil
}

// renderContactsSheet fills the "Контакты" sheet with one row per property
// contact ordered by created_at ASC. An empty contact list renders a sheet
// with only the header row.
func renderContactsSheet(f *excelize.File, contacts []ExportContactRow, headerStyle int) error {
	if err := addSheet(f, contactsSheetName); err != nil {
		return err
	}
	if err := writeSheetHeader(f, contactsSheetName, exportContactHeaders, 1, headerStyle); err != nil {
		return err
	}
	for i, contact := range contacts {
		cells := []string{contact.Name, contact.Phone}
		if err := writeSheetRow(f, contactsSheetName, i+2, cells); err != nil {
			return err
		}
	}
	if err := freezeSheetRows(f, contactsSheetName, 1); err != nil {
		return err
	}
	widths := []exportColWidth{{"A", 28}, {"B", 18}}
	return setSheetColWidths(f, contactsSheetName, widths)
}

// addSheet appends a new named sheet to the workbook.
func addSheet(f *excelize.File, name string) error {
	if _, err := f.NewSheet(name); err != nil {
		return fmt.Errorf("new sheet: %w", err)
	}
	return nil
}

// writeSheetHeader writes a header row of string cells starting at column A
// and applies the header style across it.
func writeSheetHeader(f *excelize.File, sheet string, headers []string, row, headerStyle int) error {
	for i, header := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, row)
		if err != nil {
			return fmt.Errorf("header cell: %w", err)
		}
		if err := f.SetCellStr(sheet, cell, header); err != nil {
			return fmt.Errorf("set header: %w", err)
		}
	}
	start, err := excelize.CoordinatesToCellName(1, row)
	if err != nil {
		return fmt.Errorf("header cell: %w", err)
	}
	end, err := excelize.CoordinatesToCellName(len(headers), row)
	if err != nil {
		return fmt.Errorf("header cell: %w", err)
	}
	if err := f.SetCellStyle(sheet, start, end, headerStyle); err != nil {
		return fmt.Errorf("style header: %w", err)
	}
	return nil
}

// writeSheetRow writes a row of string cells starting at column A.
func writeSheetRow(f *excelize.File, sheet string, rowNum int, cells []string) error {
	for col, value := range cells {
		cell, err := excelize.CoordinatesToCellName(col+1, rowNum)
		if err != nil {
			return fmt.Errorf("row cell: %w", err)
		}
		if err := f.SetCellStr(sheet, cell, value); err != nil {
			return fmt.Errorf("set cell: %w", err)
		}
	}
	return nil
}

// setMoneyCell writes a kopecks amount as rubles with the money format.
func setMoneyCell(f *excelize.File, sheet, cell string, kopecks int64, moneyStyle int) error {
	if err := f.SetCellFloat(sheet, cell, float64(kopecks)/100, 2, 64); err != nil {
		return fmt.Errorf("set amount %s: %w", cell, err)
	}
	if err := f.SetCellStyle(sheet, cell, cell, moneyStyle); err != nil {
		return fmt.Errorf("style amount %s: %w", cell, err)
	}
	return nil
}

// exportColWidth is a sheet column letter with its display width.
type exportColWidth struct {
	col   string
	width float64
}

// setSheetColWidths applies per-column display widths to a sheet.
func setSheetColWidths(f *excelize.File, sheet string, widths []exportColWidth) error {
	for _, cw := range widths {
		if err := f.SetColWidth(sheet, cw.col, cw.col, cw.width); err != nil {
			return fmt.Errorf("column %s width: %w", cw.col, err)
		}
	}
	return nil
}

// freezeSheetRows freezes the top ySplit rows of a sheet so its header stays
// visible while scrolling.
func freezeSheetRows(f *excelize.File, sheet string, ySplit int) error {
	topLeft, err := excelize.CoordinatesToCellName(1, ySplit+1)
	if err != nil {
		return fmt.Errorf("freeze panes cell: %w", err)
	}
	if err := f.SetPanes(sheet, &excelize.Panes{
		Freeze: true, YSplit: ySplit, TopLeftCell: topLeft, ActivePane: activePaneBottomLeft,
	}); err != nil {
		return fmt.Errorf("freeze panes: %w", err)
	}
	return nil
}

// exportOperationTypeLabel renders an operation type as the Russian income or
// expense label.
func exportOperationTypeLabel(t domain.OperationType) string {
	if t == domain.OperationTypeIncome {
		return exportLabelIncome
	}
	return exportLabelExpense
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
	start := lease.StartDate.Format(exportDateFormat)
	if lease.EndDate == nil {
		return start + " — настоящее время"
	}
	return start + " — " + lease.EndDate.Format(exportDateFormat)
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
