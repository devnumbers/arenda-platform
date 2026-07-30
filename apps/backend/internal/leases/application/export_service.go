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

// ExportFile is a generated export workbook with its download filename.
type ExportFile struct {
	Filename string
	Content  []byte
}

const (
	exportSheetName      = "Операции"
	exportMaxRows        = 1_000_000
	exportFilenameMaxLen = 50
)

var exportHeaders = []string{"Дата", "Тип", "Категория", "Название", "Сумма", "Аренда/арендатор", "Комментарий"}

// ExportService builds the property xlsx export.
type ExportService struct {
	operations OperationRepository
	properties PropertyRepository
	clock      clock.Clock
	logger     *slog.Logger
}

// NewExportService creates the property export use case.
func NewExportService(operations OperationRepository, properties PropertyRepository, clk clock.Clock, logger *slog.Logger) *ExportService {
	return &ExportService{operations: operations, properties: properties, clock: clk, logger: logger}
}

// ExportProperty returns the xlsx workbook with the property operations.
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

	content, err := buildExportWorkbook(rows)
	if err != nil {
		return ExportFile{}, fmt.Errorf("export property: build workbook: %w", err)
	}

	filename := fmt.Sprintf("Объект_%s_экспорт_%s.xlsx", sanitizeExportFilename(name), s.clock.Now().Format("02.01.2006"))
	return ExportFile{Filename: filename, Content: content}, nil
}

// buildExportWorkbook renders the "Операции" sheet in memory.
func buildExportWorkbook(rows []ExportOperationRow) (_ []byte, err error) {
	f := excelize.NewFile()
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close workbook: %w", closeErr)
		}
	}()

	if err := f.SetSheetName("Sheet1", exportSheetName); err != nil {
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
	dateStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: new("DD.MM.YYYY")})
	if err != nil {
		return nil, fmt.Errorf("date style: %w", err)
	}
	moneyStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: new(`#,##0.00" ₽"`)})
	if err != nil {
		return nil, fmt.Errorf("money style: %w", err)
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

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write workbook: %w", err)
	}
	return buf.Bytes(), nil
}

// exportTenantName renders the lease tenant as "Фамилия Имя Отчество", or an
// em dash when the operation is not linked to a lease or has no tenant data.
func exportTenantName(row ExportOperationRow) string {
	if row.LeaseID == nil {
		return "—"
	}
	var parts []string
	for _, part := range []*string{row.TenantSurname, row.TenantName, row.TenantPatronymic} {
		if part != nil && *part != "" {
			parts = append(parts, *part)
		}
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, " ")
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
