package application

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	propdomain "github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
	"github.com/xuri/excelize/v2"
)

// This file is the regression cover for the refactored property xlsx export
// (export_service.go). The export builds seven sheets: "Объект", "Операции",
// "Сводка по месяцам", "По категориям", "Аренды", "Арендаторы", "Контакты".
//
// The export fakes below are dedicated to the export tests so the existing
// service_test.go / rent_service_test.go fakes stay untouched. Each fake only
// implements the slice of its interface that ExportProperty actually calls; the
// rest are zero-value stubs so the fake still satisfies the full interface.

// --- export fakes --------------------------------------------------------

// exportPropertyRepo satisfies PropertyRepository for ExportProperty. Only
// GetForExport is configurable; the rest return zero / not-found.
type exportPropertyRepo struct {
	row ExportPropertyRow
	err error
}

func (r *exportPropertyRepo) ExistsActiveByOwner(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return false, nil
}

func (r *exportPropertyRepo) ExistsByOwner(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return false, nil
}

func (r *exportPropertyRepo) GetStatusByOwner(_ context.Context, _, _ uuid.UUID) (string, error) {
	return "", nil
}

func (r *exportPropertyRepo) GetNameByOwner(_ context.Context, _, _ uuid.UUID) (string, error) {
	return "", nil
}

func (r *exportPropertyRepo) GetForExport(_ context.Context, _, _ uuid.UUID) (ExportPropertyRow, error) {
	return r.row, r.err
}

func (r *exportPropertyRepo) GetByIDAndOwnerForUpdate(_ context.Context, _, _ uuid.UUID) (string, error) {
	return "", nil
}

func (r *exportPropertyRepo) HasOpenLease(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, nil
}
func (r *exportPropertyRepo) WithTx(_ transaction.Tx) PropertyRepository { return r }

// exportOperationRepo satisfies OperationRepository for ExportProperty. Only the
// four methods ExportProperty calls are configurable; the rest are stubs.
type exportOperationRepo struct {
	completed    []ExportOperationRow
	completedErr error
	summary      OperationsSummary
	summaryErr   error
	months       []FinanceReportMonthRow
	monthsErr    error
	cats         []FinanceReportCategoryRow
	catsErr      error
}

func (r *exportOperationRepo) Create(_ context.Context, op domain.Operation) (domain.Operation, error) {
	return op, nil
}

func (r *exportOperationRepo) BulkCreate(_ context.Context, _ []domain.Operation) error { return nil }

func (r *exportOperationRepo) ListByOwner(_ context.Context, _ uuid.UUID, _ OperationFilter) ([]domain.Operation, error) {
	return nil, nil
}

func (r *exportOperationRepo) ListByLease(_ context.Context, _ uuid.UUID) ([]domain.Operation, error) {
	return nil, nil
}

func (r *exportOperationRepo) ListByRecurringOperation(_ context.Context, _ uuid.UUID) ([]domain.Operation, error) {
	return nil, nil
}

func (r *exportOperationRepo) ListOperationDatesByLease(_ context.Context, _ uuid.UUID) ([]time.Time, error) {
	return nil, nil
}

func (r *exportOperationRepo) ListOperationDatesByRecurringOperation(_ context.Context, _ uuid.UUID) ([]time.Time, error) {
	return nil, nil
}

func (r *exportOperationRepo) UpdateFutureGeneratedOperationReminderOffsets(_ context.Context, _, _ uuid.UUID, _ *int, _ time.Time) error {
	return nil
}

func (r *exportOperationRepo) ListByProperty(_ context.Context, _, _ uuid.UUID) ([]domain.Operation, error) {
	return nil, nil
}

func (r *exportOperationRepo) ListByPropertyWithStatuses(_ context.Context, _, _ uuid.UUID, _ []domain.OperationStatus) ([]domain.Operation, error) {
	return nil, nil
}

func (r *exportOperationRepo) GetByIDAndOwner(_ context.Context, _, _ uuid.UUID) (domain.Operation, error) {
	return domain.Operation{}, ErrNotFound
}

func (r *exportOperationRepo) GetByIDAndOwnerForUpdate(_ context.Context, _, _ uuid.UUID) (domain.Operation, error) {
	return domain.Operation{}, ErrNotFound
}

func (r *exportOperationRepo) Update(_ context.Context, op domain.Operation) (domain.Operation, error) {
	return op, nil
}

func (r *exportOperationRepo) MarkOverdue(_ context.Context, _, _ uuid.UUID, _ time.Time) (domain.Operation, bool, error) {
	return domain.Operation{}, false, nil
}

func (r *exportOperationRepo) SoftDeleteOperation(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (r *exportOperationRepo) DeleteUneditedFutureOperationsByLease(_ context.Context, _ uuid.UUID, _ time.Time) error {
	return nil
}

func (r *exportOperationRepo) DeleteUneditedFutureOperationsByRecurringOperation(_ context.Context, _ uuid.UUID, _ time.Time) error {
	return nil
}

func (r *exportOperationRepo) DeleteFutureGeneratedOperations(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (r *exportOperationRepo) DeleteUneditedOperationsByRecurringOperation(_ context.Context, _ uuid.UUID, _ time.Time) error {
	return nil
}

func (r *exportOperationRepo) DeleteOperationsOutsideLeaseRange(_ context.Context, _ uuid.UUID, _ time.Time, _ *time.Time) error {
	return nil
}

func (r *exportOperationRepo) DeleteUneditedOperationsByLease(_ context.Context, _ uuid.UUID, _ time.Time) error {
	return nil
}

func (r *exportOperationRepo) ListPendingOperationsWithPastDate(_ context.Context, _ uuid.UUID, _ time.Time, _ int) ([]domain.Operation, error) {
	return nil, nil
}

func (r *exportOperationRepo) ListAllPendingOperationsWithPastDate(_ context.Context, _ time.Time, _ int) ([]domain.Operation, error) {
	return nil, nil
}

func (r *exportOperationRepo) GetPropertyOperationsSummary(_ context.Context, _, _ uuid.UUID, _ time.Time) (OperationsSummary, error) {
	return r.summary, r.summaryErr
}

func (r *exportOperationRepo) ListOverdueRentOperations(_ context.Context, _ uuid.UUID) ([]OverdueRentOperation, error) {
	return nil, nil
}

func (r *exportOperationRepo) ListNextRentPayments(_ context.Context, _ uuid.UUID, _ time.Time) ([]NextRentPayment, error) {
	return nil, nil
}

func (r *exportOperationRepo) GetFinanceReportTotals(_ context.Context, _ uuid.UUID, _, _ *time.Time) (FinanceReportTotals, error) {
	return FinanceReportTotals{}, nil
}

func (r *exportOperationRepo) GetFinanceReportByProperty(_ context.Context, _ uuid.UUID, _, _ *time.Time) ([]FinanceReportPropertyRow, error) {
	return nil, nil
}

func (r *exportOperationRepo) GetFinanceReportByCategory(_ context.Context, _ uuid.UUID, _, _ *time.Time) ([]FinanceReportCategoryRow, error) {
	return nil, nil
}

func (r *exportOperationRepo) GetFinanceReportByMonth(_ context.Context, _ uuid.UUID, _, _ *time.Time) ([]FinanceReportMonthRow, error) {
	return nil, nil
}

func (r *exportOperationRepo) GetPropertyFinanceByMonth(_ context.Context, _, _ uuid.UUID) ([]FinanceReportMonthRow, error) {
	return r.months, r.monthsErr
}

func (r *exportOperationRepo) GetPropertyFinanceByCategory(_ context.Context, _, _ uuid.UUID) ([]FinanceReportCategoryRow, error) {
	return r.cats, r.catsErr
}

func (r *exportOperationRepo) ListCompletedForExport(_ context.Context, _, _ uuid.UUID) ([]ExportOperationRow, error) {
	return r.completed, r.completedErr
}
func (r *exportOperationRepo) WithTx(_ transaction.Tx) OperationRepository { return r }

// exportLeaseRepo satisfies LeaseRepository for ExportProperty. Only
// ListWithTenantForExport is configurable; the rest are stubs.
type exportLeaseRepo struct {
	rows []ExportLeaseRow
	err  error
}

func (r *exportLeaseRepo) Create(_ context.Context, _ uuid.UUID, l domain.Lease) (domain.Lease, error) {
	return l, nil
}

func (r *exportLeaseRepo) GetByID(_ context.Context, _ uuid.UUID) (domain.Lease, error) {
	return domain.Lease{}, ErrNotFound
}

func (r *exportLeaseRepo) GetByIDForUpdate(_ context.Context, _ uuid.UUID) (domain.Lease, error) {
	return domain.Lease{}, ErrNotFound
}

func (r *exportLeaseRepo) GetByIDAndOwner(_ context.Context, _, _ uuid.UUID) (domain.Lease, error) {
	return domain.Lease{}, ErrNotFound
}

func (r *exportLeaseRepo) GetByIDAndOwnerForUpdate(_ context.Context, _, _ uuid.UUID) (domain.Lease, error) {
	return domain.Lease{}, ErrNotFound
}

func (r *exportLeaseRepo) ListByOwner(_ context.Context, _ uuid.UUID) ([]domain.Lease, error) {
	return nil, nil
}

func (r *exportLeaseRepo) Update(_ context.Context, _ uuid.UUID, l domain.Lease) (domain.Lease, error) {
	return l, nil
}

func (r *exportLeaseRepo) Complete(_ context.Context, _, _ uuid.UUID) (domain.Lease, error) {
	return domain.Lease{}, nil
}

func (r *exportLeaseRepo) CountOpenLeasesByProperty(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}

func (r *exportLeaseRepo) GetOpenLeaseByProperty(_ context.Context, _, _ uuid.UUID) (domain.Lease, error) {
	return domain.Lease{}, ErrNotFound
}

func (r *exportLeaseRepo) ListOpenLeasesWithPastEndDate(_ context.Context, _ time.Time, _ int) ([]domain.Lease, error) {
	return nil, nil
}

func (r *exportLeaseRepo) ListByProperty(_ context.Context, _, _ uuid.UUID) ([]domain.Lease, error) {
	return nil, nil
}

func (r *exportLeaseRepo) ListWithTenantForExport(_ context.Context, _, _ uuid.UUID) ([]ExportLeaseRow, error) {
	return r.rows, r.err
}
func (r *exportLeaseRepo) WithTx(_ transaction.Tx) LeaseRepository { return r }

// exportContactRepo satisfies PropertyContactRepository for ExportProperty.
type exportContactRepo struct {
	rows []ExportContactRow
	err  error
}

func (r *exportContactRepo) ListForExport(_ context.Context, _, _ uuid.UUID) ([]ExportContactRow, error) {
	return r.rows, r.err
}

// Compile-time interface checks for the export fakes.
var (
	_ PropertyRepository        = (*exportPropertyRepo)(nil)
	_ OperationRepository       = (*exportOperationRepo)(nil)
	_ LeaseRepository           = (*exportLeaseRepo)(nil)
	_ PropertyContactRepository = (*exportContactRepo)(nil)
)

// --- helpers -------------------------------------------------------------

var (
	exportTestOwner    = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	exportTestProperty = uuid.MustParse("22222222-2222-2222-2222-222222222222")
)

// newExportService wires an ExportService with the given fakes and a fixed clock.
func newExportService(t *testing.T, prop *exportPropertyRepo, ops *exportOperationRepo, leases *exportLeaseRepo, contacts *exportContactRepo) *ExportService {
	t.Helper()
	return NewExportService(ops, leases, prop, contacts, fakeClock{now: time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)}, slog.Default())
}

// openExport parses the generated workbook bytes for assertions.
func openExport(t *testing.T, content []byte) *excelize.File {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("excelize.OpenReader: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// cellValue reads a cell as a display string (GetCellValue), failing the test
// on error.
func cellValue(t *testing.T, f *excelize.File, sheet, cell string) string {
	t.Helper()
	v, err := f.GetCellValue(sheet, cell)
	if err != nil {
		t.Fatalf("GetCellValue(%s!%s): %v", sheet, cell, err)
	}
	return v
}

// objectSheetPairs walks the "Объект" sheet and returns every Параметр — Значение
// pair (columns A and B) for non-empty rows. Used to assert characteristics by
// label rather than by hard-coded row index.
func objectSheetPairs(t *testing.T, f *excelize.File) map[string]string {
	t.Helper()
	pairs := make(map[string]string)
	for row := 1; row <= 50; row++ {
		label := cellValue(t, f, objectSheetName, "A"+itoa(row))
		value := cellValue(t, f, objectSheetName, "B"+itoa(row))
		if label == "" && value == "" {
			continue
		}
		pairs[label] = value
	}
	return pairs
}

// itoa is a tiny strconv.Itoa alias to keep the test file import-light.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// --- tests ---------------------------------------------------------------

func TestExportProperty_SheetNames(t *testing.T) {
	prop := &exportPropertyRepo{row: ExportPropertyRow{
		Name:    "Тест",
		Type:    propdomain.PropertyTypeApartment,
		Address: "Москва, Тверская 1",
	}}
	svc := newExportService(t, prop, &exportOperationRepo{}, &exportLeaseRepo{}, &exportContactRepo{})

	out, err := svc.ExportProperty(context.Background(), exportTestOwner, exportTestProperty)
	if err != nil {
		t.Fatalf("ExportProperty: %v", err)
	}

	f := openExport(t, out.Content)
	got := f.GetSheetList()

	// Seven sheets in the documented order.
	want := []string{
		objectSheetName,
		exportSheetName,
		monthlySheetName,
		categorySheetName,
		leasesSheetName,
		tenantsSheetName,
		contactsSheetName,
	}
	if len(got) != len(want) {
		t.Fatalf("sheet count: want %d (%v), got %d (%v)", len(want), want, len(got), got)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("sheet[%d]: want %q, got %q", i, name, got[i])
		}
	}
}

func TestExportProperty_OperationDateFormat(t *testing.T) {
	july31 := time.Date(2026, 7, 31, 9, 30, 0, 0, time.UTC) // time component must be dropped
	ops := &exportOperationRepo{completed: []ExportOperationRow{
		{
			OperationDate: july31,
			Type:          domain.OperationTypeIncome,
			CategoryName:  "Аренда",
			Name:          "Платёж за июль",
			AmountKopecks: 50000,
		},
	}}
	svc := newExportService(t, &exportPropertyRepo{row: ExportPropertyRow{
		Name: "Тест", Type: propdomain.PropertyTypeApartment, Address: "Адрес",
	}}, ops, &exportLeaseRepo{}, &exportContactRepo{})

	out, err := svc.ExportProperty(context.Background(), exportTestOwner, exportTestProperty)
	if err != nil {
		t.Fatalf("ExportProperty: %v", err)
	}

	f := openExport(t, out.Content)
	dateCell := cellValue(t, f, exportSheetName, "A2")

	// Exact value, formatted DD.MM.YYYY without a time component.
	if dateCell != "31.07.2026" {
		t.Errorf("operation date A2: want %q, got %q", "31.07.2026", dateCell)
	}
	// Defensive: shape must be a pure date, no time suffix.
	if !regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}$`).MatchString(dateCell) {
		t.Errorf("operation date %q does not match ^\\d{2}\\.\\d{2}\\.\\d{4}$", dateCell)
	}
}

func TestExportProperty_ObjectSheet(t *testing.T) {
	prop := &exportPropertyRepo{row: ExportPropertyRow{
		Name:        "Тестовая квартира",
		Type:        propdomain.PropertyTypeApartment,
		Address:     "Москва, Тверская 1",
		Description: "Уютная",
		// Attributes arrive from JSON: numeric fields are float64.
		Attributes: propdomain.Attributes{
			"rooms":        float64(2),
			"floor":        float64(3),
			"floors_total": float64(5),
		},
	}}
	svc := newExportService(t, prop, &exportOperationRepo{}, &exportLeaseRepo{}, &exportContactRepo{})

	out, err := svc.ExportProperty(context.Background(), exportTestOwner, exportTestProperty)
	if err != nil {
		t.Fatalf("ExportProperty: %v", err)
	}

	f := openExport(t, out.Content)

	// Basic card fields.
	if got := cellValue(t, f, objectSheetName, "B1"); got != "Тестовая квартира" {
		t.Errorf("Название (B1): want %q, got %q", "Тестовая квартира", got)
	}
	pairs := objectSheetPairs(t, f)

	if got := pairs["Тип"]; got != "Квартира" {
		t.Errorf("Тип: want %q, got %q", "Квартира", got)
	}
	if got := pairs["Адрес"]; !strings.Contains(got, "Тверская") {
		t.Errorf("Адрес: want to contain %q, got %q", "Тверская", got)
	}
	if got := pairs["Описание"]; got != "Уютная" {
		t.Errorf("Описание: want %q, got %q", "Уютная", got)
	}

	// Catalog characteristics (exact labels from the apartment catalog).
	checks := map[string]string{
		"Комнаты":        "2",
		"Этаж":           "3",
		"Этажность дома": "5",
	}
	for label, want := range checks {
		got, ok := pairs[label]
		if !ok {
			t.Errorf("characteristic %q missing from Объект sheet (pairs=%v)", label, pairs)
			continue
		}
		if got != want {
			t.Errorf("characteristic %q: want %q, got %q", label, want, got)
		}
	}
}

func TestExportProperty_ObjectSheetSkipsMissingAttributes(t *testing.T) {
	prop := &exportPropertyRepo{row: ExportPropertyRow{
		Name:    "Тест",
		Type:    propdomain.PropertyTypeApartment,
		Address: "Адрес",
		// Only rooms is set; floor and floors_total are absent.
		Attributes: propdomain.Attributes{"rooms": float64(2)},
	}}
	svc := newExportService(t, prop, &exportOperationRepo{}, &exportLeaseRepo{}, &exportContactRepo{})

	out, err := svc.ExportProperty(context.Background(), exportTestOwner, exportTestProperty)
	if err != nil {
		t.Fatalf("ExportProperty: %v", err)
	}

	f := openExport(t, out.Content)
	pairs := objectSheetPairs(t, f)

	// rooms is present...
	if pairs["Комнаты"] != "2" {
		t.Errorf("Комнаты: want %q, got %q", "2", pairs["Комнаты"])
	}
	// ...but Этаж must be skipped entirely (no label, no value).
	for label := range pairs {
		if strings.Contains(label, "Этаж") {
			t.Errorf("expected Этаж characteristic to be skipped, but found label %q = %q", label, pairs[label])
		}
	}
}

func TestExportProperty_EmptySectionsRenderHeaders(t *testing.T) {
	// No operations, leases, or contacts — but every sheet must still render
	// with its header row and the monthly/category skeletons.
	prop := &exportPropertyRepo{row: ExportPropertyRow{
		Name: "Пустой объект", Type: propdomain.PropertyTypeApartment, Address: "Адрес",
	}}
	svc := newExportService(t, prop, &exportOperationRepo{}, &exportLeaseRepo{}, &exportContactRepo{})

	out, err := svc.ExportProperty(context.Background(), exportTestOwner, exportTestProperty)
	if err != nil {
		t.Fatalf("ExportProperty: %v", err)
	}
	f := openExport(t, out.Content)

	headerCases := []struct {
		sheet string
		cell  string
		want  string
	}{
		{exportSheetName, "A1", "Дата"},
		{leasesSheetName, "A1", exportLeaseHeaders[0]},
		{tenantsSheetName, "A1", exportTenantHeaders[0]},
		{contactsSheetName, "A1", exportContactHeaders[0]},
		{categorySheetName, "A1", "Категория"},
	}
	for _, tc := range headerCases {
		if got := cellValue(t, f, tc.sheet, tc.cell); got != tc.want {
			t.Errorf("%s!%s header: want %q, got %q", tc.sheet, tc.cell, tc.want, got)
		}
	}

	// Monthly sheet: all-time totals block header + month table header.
	if got := cellValue(t, f, monthlySheetName, "A1"); got != "Итоги за всё время" {
		t.Errorf("monthly A1: want %q, got %q", "Итоги за всё время", got)
	}
	if got := cellValue(t, f, monthlySheetName, "A6"); got != "Месяц" {
		t.Errorf("monthly month header A6: want %q, got %q", "Месяц", got)
	}

	// All seven sheets must exist even when their data is empty.
	for _, name := range []string{objectSheetName, exportSheetName, monthlySheetName, categorySheetName, leasesSheetName, tenantsSheetName, contactsSheetName} {
		if idx, _ := f.GetSheetIndex(name); idx < 0 {
			t.Errorf("expected sheet %q to exist", name)
		}
	}
}

func TestExportProperty_NotFound(t *testing.T) {
	prop := &exportPropertyRepo{err: ErrNotFound}
	svc := newExportService(t, prop, &exportOperationRepo{}, &exportLeaseRepo{}, &exportContactRepo{})

	_, err := svc.ExportProperty(context.Background(), exportTestOwner, exportTestProperty)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ExportProperty error: want ErrNotFound, got %v", err)
	}
}
