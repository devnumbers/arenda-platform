package application

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time { return c.now }

type fakeOperationRepo struct {
	mu  sync.Mutex
	ops []domain.Operation
}

func (r *fakeOperationRepo) Create(_ context.Context, op domain.Operation) (domain.Operation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if op.ID == uuid.Nil {
		op.ID = uuid.Must(uuid.NewV7())
	}
	r.ops = append(r.ops, op)
	return op, nil
}

func (r *fakeOperationRepo) BulkCreate(_ context.Context, ops []domain.Operation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, op := range ops {
		if op.ID == uuid.Nil {
			op.ID = uuid.Must(uuid.NewV7())
		}
		r.ops = append(r.ops, op)
	}
	return nil
}

func (r *fakeOperationRepo) ListByOwner(_ context.Context, _ uuid.UUID, _ OperationFilter) ([]domain.Operation, error) {
	return nil, nil
}

func (r *fakeOperationRepo) ListByLease(_ context.Context, leaseID uuid.UUID) ([]domain.Operation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Operation
	for _, op := range r.ops {
		if op.LeaseID == leaseID && op.DeletedAt == nil {
			out = append(out, op)
		}
	}
	return out, nil
}

func (r *fakeOperationRepo) ListByRecurringOperation(_ context.Context, recurringOperationID uuid.UUID) ([]domain.Operation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Operation
	for _, op := range r.ops {
		if op.RecurringOperationID == recurringOperationID && op.DeletedAt == nil {
			out = append(out, op)
		}
	}
	return out, nil
}

func (r *fakeOperationRepo) ListOperationDatesByLease(_ context.Context, leaseID uuid.UUID) ([]time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	dates := make(map[time.Time]struct{})
	for _, op := range r.ops {
		if op.LeaseID == leaseID && operationBlocksLeaseRentSchedule(op) && (op.DeletedAt == nil || op.IsException) {
			dates[operationSourceDate(op)] = struct{}{}
		}
	}
	out := make([]time.Time, 0, len(dates))
	for d := range dates {
		out = append(out, d)
	}
	return out, nil
}

func (r *fakeOperationRepo) ListOperationDatesByRecurringOperation(_ context.Context, recurringOperationID uuid.UUID) ([]time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	dates := make(map[time.Time]struct{})
	for _, op := range r.ops {
		if op.RecurringOperationID == recurringOperationID && (op.DeletedAt == nil || op.IsException) {
			dates[operationSourceDate(op)] = struct{}{}
		}
	}
	out := make([]time.Time, 0, len(dates))
	for d := range dates {
		out = append(out, d)
	}
	return out, nil
}

func (r *fakeOperationRepo) UpdateFutureGeneratedOperationReminderOffsets(
	_ context.Context,
	ownerID, recurringOperationID uuid.UUID,
	offsetDays *int,
	from time.Time,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, op := range r.ops {
		if op.OwnerID != ownerID ||
			op.RecurringOperationID != recurringOperationID ||
			op.IsException ||
			op.DeletedAt != nil ||
			timeutil.Date(op.OperationDate).Before(timeutil.Date(from)) {
			continue
		}
		op.ReminderOffsetDays = offsetDays
		r.ops[id] = op
	}
	return nil
}

func (r *fakeOperationRepo) ListByProperty(_ context.Context, ownerID, propertyID uuid.UUID) ([]domain.Operation, error) {
	return nil, nil
}

func (r *fakeOperationRepo) GetPropertyOperationsSummary(
	_ context.Context,
	ownerID, propertyID uuid.UUID,
	asOf time.Time,
) (OperationsSummary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	monthStart := time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, time.UTC)
	acc := newSummaryAccumulator(monthStart)
	for _, op := range r.ops {
		if op.OwnerID != ownerID || op.PropertyID != propertyID || op.DeletedAt != nil {
			continue
		}
		acc.add(op)
	}
	return acc.summary(), nil
}

// summaryAccumulator folds live operations into the property summary the same
// way the SQL summary query aggregates them: all-time and current-month
// profit, overdue counts, and the next rent payment date.
type summaryAccumulator struct {
	monthStart, monthEnd time.Time
	allTimeProfit        int64
	monthlyProfit        int64
	overdueRentCount     int
	overdueTotalCount    int
	nextPaymentDate      *time.Time
}

// newSummaryAccumulator starts folding at the given month start.
func newSummaryAccumulator(monthStart time.Time) *summaryAccumulator {
	return &summaryAccumulator{
		monthStart: monthStart,
		monthEnd:   monthStart.AddDate(0, 1, 0),
	}
}

// add folds one operation into the summary by its type.
func (a *summaryAccumulator) add(op domain.Operation) {
	switch op.Type {
	case domain.OperationTypeIncome:
		a.addIncome(op)
	case domain.OperationTypeExpense:
		a.addExpense(op)
	}
}

// addIncome folds an income operation: received payments add to the profit
// totals, overdue income counts toward the overdue totals, and a pending or
// overdue rent operation can become the next payment date.
func (a *summaryAccumulator) addIncome(op domain.Operation) {
	if op.Status == domain.OperationStatusReceived {
		a.addProfit(op.OperationDate, op.AmountKopecks)
	}
	if op.Status == domain.OperationStatusOverdue {
		a.countOverdue(op.CategoryID)
	}
	if (op.Status == domain.OperationStatusPending || op.Status == domain.OperationStatusOverdue) && op.CategoryID == testRentCategoryID {
		a.trackNextPayment(op.OperationDate)
	}
}

// addExpense folds an expense operation: paid expenses subtract from the
// profit totals and overdue expenses count toward the overdue total.
func (a *summaryAccumulator) addExpense(op domain.Operation) {
	if op.Status == domain.OperationStatusPaid {
		a.addProfit(op.OperationDate, -op.AmountKopecks)
	}
	if op.Status == domain.OperationStatusOverdue {
		a.countOverdue(uuid.Nil)
	}
}

// addProfit adds a signed kopecks amount to the all-time profit and, when the
// operation falls in the current month, to the monthly profit.
func (a *summaryAccumulator) addProfit(operationDate time.Time, amountKopecks int64) {
	a.allTimeProfit += amountKopecks
	opDate := timeutil.Date(operationDate)
	if !opDate.Before(a.monthStart) && opDate.Before(a.monthEnd) {
		a.monthlyProfit += amountKopecks
	}
}

// countOverdue counts an overdue operation, additionally as rent when its
// category is the rent category.
func (a *summaryAccumulator) countOverdue(categoryID uuid.UUID) {
	a.overdueTotalCount++
	if categoryID == testRentCategoryID {
		a.overdueRentCount++
	}
}

// trackNextPayment keeps the earliest rent payment date seen.
func (a *summaryAccumulator) trackNextPayment(operationDate time.Time) {
	opDate := timeutil.Date(operationDate)
	if a.nextPaymentDate == nil || opDate.Before(*a.nextPaymentDate) {
		a.nextPaymentDate = &opDate
	}
}

// summary returns the folded summary.
func (a *summaryAccumulator) summary() OperationsSummary {
	return OperationsSummary{
		AllTimeProfitKopecks: a.allTimeProfit,
		MonthlyProfitKopecks: a.monthlyProfit,
		OverdueRentCount:     a.overdueRentCount,
		OverdueTotalCount:    a.overdueTotalCount,
		NextPaymentDate:      a.nextPaymentDate,
	}
}

func (r *fakeOperationRepo) ListOverdueRentOperations(_ context.Context, ownerID uuid.UUID, _ []uuid.UUID) ([]OverdueRentOperation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []OverdueRentOperation
	for _, op := range r.ops {
		if op.OwnerID != ownerID ||
			op.DeletedAt != nil ||
			op.LeaseID == uuid.Nil ||
			op.Status != domain.OperationStatusOverdue ||
			op.Type != domain.OperationTypeIncome ||
			op.CategoryID != testRentCategoryID {
			continue
		}
		out = append(out, OverdueRentOperation{LeaseID: op.LeaseID, OperationDate: timeutil.Date(op.OperationDate)})
	}
	return out, nil
}

func (r *fakeOperationRepo) ListNextRentPayments(
	_ context.Context,
	ownerID uuid.UUID,
	_ []uuid.UUID,
	asOf time.Time,
) ([]NextRentPayment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	minByLease := make(map[uuid.UUID]time.Time)
	for _, op := range r.ops {
		if op.OwnerID != ownerID ||
			op.DeletedAt != nil ||
			op.LeaseID == uuid.Nil ||
			op.Status != domain.OperationStatusPending ||
			op.Type != domain.OperationTypeIncome ||
			op.CategoryID != testRentCategoryID {
			continue
		}
		d := timeutil.Date(op.OperationDate)
		if d.Before(asOf) {
			continue
		}
		if cur, ok := minByLease[op.LeaseID]; !ok || d.Before(cur) {
			minByLease[op.LeaseID] = d
		}
	}
	out := make([]NextRentPayment, 0, len(minByLease))
	for leaseID, d := range minByLease {
		out = append(out, NextRentPayment{LeaseID: leaseID, NextPaymentDate: d})
	}
	return out, nil
}

func (r *fakeOperationRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Operation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, op := range r.ops {
		if op.ID == id && op.DeletedAt == nil {
			return op, nil
		}
	}
	return domain.Operation{}, ErrNotFound
}

func (r *fakeOperationRepo) MoveToProperty(
	_ context.Context,
	id,
	scope, propertyID uuid.UUID,
	updatedAt time.Time,
) (domain.Operation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.ops {
		if r.ops[i].ID == id && r.ops[i].OwnerID == scope && r.ops[i].DeletedAt == nil {
			r.ops[i].PropertyID = propertyID
			r.ops[i].UpdatedAt = updatedAt
			return r.ops[i], nil
		}
	}
	return domain.Operation{}, ErrNotFound
}

func (r *fakeOperationRepo) GetByIDAndOwner(_ context.Context, id, _ uuid.UUID) (domain.Operation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, op := range r.ops {
		if op.ID == id && op.DeletedAt == nil {
			return op, nil
		}
	}
	return domain.Operation{}, ErrNotFound
}

func (r *fakeOperationRepo) GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (domain.Operation, error) {
	return r.GetByIDAndOwner(ctx, id, ownerID)
}

func (r *fakeOperationRepo) Update(_ context.Context, op domain.Operation) (domain.Operation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.ops {
		if r.ops[i].ID == op.ID {
			r.ops[i] = op
			return op, nil
		}
	}
	return domain.Operation{}, ErrNotFound
}

func (r *fakeOperationRepo) SoftDeleteOperation(_ context.Context, id, _ uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for i := range r.ops {
		if r.ops[i].ID == id {
			r.ops[i].DeletedAt = &now
			if r.ops[i].RecurringOperationID != uuid.Nil || (r.ops[i].LeaseID != uuid.Nil && r.ops[i].CategoryID == testRentCategoryID) {
				r.ops[i].IsException = true
			}
			return nil
		}
	}
	return ErrNotFound
}

func (r *fakeOperationRepo) DeleteUneditedFutureOperationsByLease(_ context.Context, leaseID uuid.UUID, after time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	after = timeutil.Date(after)
	filtered := r.ops[:0]
	for _, op := range r.ops {
		keep := true
		if op.LeaseID == leaseID && !op.IsException && timeutil.Date(op.OperationDate).After(after) {
			keep = false
		}
		if keep {
			filtered = append(filtered, op)
		}
	}
	r.ops = filtered
	return nil
}

func (r *fakeOperationRepo) DeleteUneditedFutureOperationsByRecurringOperation(
	_ context.Context,
	recurringOperationID uuid.UUID,
	after time.Time,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	after = timeutil.Date(after)
	filtered := r.ops[:0]
	for _, op := range r.ops {
		keep := true
		if op.RecurringOperationID == recurringOperationID && !op.IsException && timeutil.Date(op.OperationDate).After(after) {
			keep = false
		}
		if keep {
			filtered = append(filtered, op)
		}
	}
	r.ops = filtered
	return nil
}

func (r *fakeOperationRepo) DeleteFutureGeneratedOperations(_ context.Context, recurringOperationID, ownerID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	today := timeutil.Date(time.Now())
	filtered := r.ops[:0]
	for _, op := range r.ops {
		keep := true
		if op.RecurringOperationID == recurringOperationID && op.OwnerID == ownerID && !op.IsException && op.DeletedAt == nil {
			if timeutil.Date(op.OperationDate).After(today) &&
				(op.Status == domain.OperationStatusPending || op.Status == domain.OperationStatusOverdue) {
				keep = false
			}
		}
		if keep {
			filtered = append(filtered, op)
		}
	}
	r.ops = filtered
	return nil
}

func (r *fakeOperationRepo) DeleteUneditedOperationsByRecurringOperation(
	_ context.Context,
	recurringOperationID uuid.UUID,
	from time.Time,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	from = timeutil.Date(from)
	filtered := r.ops[:0]
	for _, op := range r.ops {
		keep := true
		if op.RecurringOperationID == recurringOperationID && !op.IsException && !timeutil.Date(op.OperationDate).Before(from) {
			keep = false
		}
		if keep {
			filtered = append(filtered, op)
		}
	}
	r.ops = filtered
	return nil
}

func (r *fakeOperationRepo) DeleteOperationsOutsideLeaseRange(_ context.Context, leaseID uuid.UUID, start time.Time, end *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	start = timeutil.Date(start)
	filtered := r.ops[:0]
	for _, op := range r.ops {
		keep := true
		if op.LeaseID == leaseID {
			d := timeutil.Date(op.OperationDate)
			if d.Before(start) || (end != nil && d.After(timeutil.Date(*end))) {
				keep = false
			}
		}
		if keep {
			filtered = append(filtered, op)
		}
	}
	r.ops = filtered
	return nil
}

func (r *fakeOperationRepo) DeleteUneditedOperationsByLease(_ context.Context, leaseID uuid.UUID, from time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	from = timeutil.Date(from)
	filtered := r.ops[:0]
	for _, op := range r.ops {
		keep := true
		if op.LeaseID == leaseID && !op.IsException && !timeutil.Date(op.OperationDate).Before(from) {
			keep = false
		}
		if keep {
			filtered = append(filtered, op)
		}
	}
	r.ops = filtered
	return nil
}

func (r *fakeOperationRepo) ListByPropertyWithStatuses(
	_ context.Context,
	_, _ uuid.UUID,
	_ []domain.OperationStatus,
) ([]domain.Operation, error) {
	return nil, nil
}

func (r *fakeOperationRepo) ListPendingOperationsWithPastDate(
	_ context.Context,
	_ uuid.UUID,
	_ time.Time,
	_ int,
) ([]domain.Operation, error) {
	return nil, nil
}

func (r *fakeOperationRepo) ListAllPendingOperationsWithPastDate(_ context.Context, _ time.Time, _ int) ([]domain.Operation, error) {
	return nil, nil
}

func (r *fakeOperationRepo) MarkOverdue(_ context.Context, ownerID, id uuid.UUID, asOf time.Time) (domain.Operation, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.ops {
		if r.ops[i].ID == id && r.ops[i].OwnerID == ownerID && r.ops[i].DeletedAt == nil {
			if r.ops[i].Status == domain.OperationStatusPending && r.ops[i].OperationDate.Before(timeutil.Date(asOf)) {
				r.ops[i].Status = domain.OperationStatusOverdue
				return r.ops[i], true, nil
			}
			return r.ops[i], false, nil
		}
	}
	return domain.Operation{}, false, ErrNotFound
}

func (r *fakeOperationRepo) GetFinanceReportTotals(
	_ context.Context,
	_ uuid.UUID,
	_ []uuid.UUID,
	_, _ *time.Time,
) (FinanceReportTotals, error) {
	return FinanceReportTotals{}, nil
}

func (r *fakeOperationRepo) GetFinanceReportByProperty(
	_ context.Context,
	_ uuid.UUID,
	_ []uuid.UUID,
	_, _ *time.Time,
) ([]FinanceReportPropertyRow, error) {
	return nil, nil
}

func (r *fakeOperationRepo) GetFinanceReportByCategory(
	_ context.Context,
	_ uuid.UUID,
	_ []uuid.UUID,
	_, _ *time.Time,
) ([]FinanceReportCategoryRow, error) {
	return nil, nil
}

func (r *fakeOperationRepo) GetFinanceReportByMonth(
	_ context.Context,
	_ uuid.UUID,
	_ []uuid.UUID,
	_, _ *time.Time,
) ([]FinanceReportMonthRow, error) {
	return nil, nil
}

func (r *fakeOperationRepo) GetPropertyFinanceByMonth(_ context.Context, _, _ uuid.UUID) ([]FinanceReportMonthRow, error) {
	return nil, nil
}

func (r *fakeOperationRepo) GetPropertyFinanceByCategory(_ context.Context, _, _ uuid.UUID) ([]FinanceReportCategoryRow, error) {
	return nil, nil
}

func (r *fakeOperationRepo) ListCompletedForExport(_ context.Context, _, _ uuid.UUID) ([]ExportOperationRow, error) {
	return nil, nil
}

func (r *fakeOperationRepo) WithTx(_ transaction.Tx) OperationRepository {
	return r
}

func operationSourceDate(op domain.Operation) time.Time {
	if op.SourceOperationDate != nil {
		return timeutil.Date(*op.SourceOperationDate)
	}
	return timeutil.Date(op.OperationDate)
}

func operationBlocksLeaseRentSchedule(op domain.Operation) bool {
	return op.RecurringOperationID != uuid.Nil || op.CategoryID == testRentCategoryID
}

type fakeRecurringOperationRepo struct {
	mu   sync.Mutex
	recs map[uuid.UUID]domain.RecurringOperation
}

func (r *fakeRecurringOperationRepo) Create(_ context.Context, rec domain.RecurringOperation) (domain.RecurringOperation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.recs == nil {
		r.recs = make(map[uuid.UUID]domain.RecurringOperation)
	}
	r.recs[rec.ID] = rec
	return rec, nil
}

func (r *fakeRecurringOperationRepo) GetByID(_ context.Context, id uuid.UUID) (domain.RecurringOperation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.recs[id]
	if !ok || rec.DeletedAt != nil {
		return domain.RecurringOperation{}, ErrNotFound
	}
	return rec, nil
}

func (r *fakeRecurringOperationRepo) GetByLeaseID(_ context.Context, _, leaseID uuid.UUID) (domain.RecurringOperation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.recs {
		if rec.LeaseID == leaseID {
			return rec, nil
		}
	}
	return domain.RecurringOperation{}, ErrNotFound
}

func (r *fakeRecurringOperationRepo) GetByIDAndOwner(_ context.Context, id, _ uuid.UUID) (domain.RecurringOperation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.recs[id]
	if !ok {
		return domain.RecurringOperation{}, ErrNotFound
	}
	return rec, nil
}

func (r *fakeRecurringOperationRepo) GetByIDAndOwnerForUpdate(
	ctx context.Context,
	id, ownerID uuid.UUID,
) (domain.RecurringOperation, error) {
	return r.GetByIDAndOwner(ctx, id, ownerID)
}

func (r *fakeRecurringOperationRepo) ListByOwner(_ context.Context, _ uuid.UUID, _ []uuid.UUID) ([]domain.RecurringOperation, error) {
	return nil, nil
}

func (r *fakeRecurringOperationRepo) ListByProperty(_ context.Context, _, _ uuid.UUID) ([]domain.RecurringOperation, error) {
	return nil, nil
}

func (r *fakeRecurringOperationRepo) ListByPropertyID(_ context.Context, _ uuid.UUID) ([]domain.RecurringOperation, error) {
	return nil, nil
}

func (r *fakeRecurringOperationRepo) Update(_ context.Context, rec domain.RecurringOperation) (domain.RecurringOperation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.recs == nil {
		return domain.RecurringOperation{}, ErrNotFound
	}
	if _, ok := r.recs[rec.ID]; !ok {
		return domain.RecurringOperation{}, ErrNotFound
	}
	r.recs[rec.ID] = rec
	return rec, nil
}

func (r *fakeRecurringOperationRepo) UpdateStatus(_ context.Context, _, _ uuid.UUID, _ string) (domain.RecurringOperation, error) {
	return domain.RecurringOperation{}, nil
}

func (r *fakeRecurringOperationRepo) UpdateStatusByLeaseID(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}

func (r *fakeRecurringOperationRepo) SetReminderOffset(_ context.Context, _, _ uuid.UUID, _ *int) error {
	return nil
}

func (r *fakeRecurringOperationRepo) DeleteByLease(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (r *fakeRecurringOperationRepo) SoftDelete(_ context.Context, id, _ uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.recs[id]; !ok {
		return ErrNotFound
	}
	rec := r.recs[id]
	now := time.Now()
	rec.DeletedAt = &now
	r.recs[id] = rec
	return nil
}

func (r *fakeRecurringOperationRepo) WithTx(_ transaction.Tx) RecurringOperationRepository {
	return r
}

func TestGenerateRentOperations_BackdatedLeaseMarksPastPeriodsUnconfirmed(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	leaseID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	recID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	endDate := date(2024, 5, 31)

	lease := domain.Lease{
		ID:                leaseID,
		OwnerID:           ownerID,
		PropertyID:        propertyID,
		StartDate:         date(2024, 3, 1),
		EndDate:           &endDate,
		RentAmountKopecks: 10000,
		PaymentDay:        1,
	}

	svc := NewRentService(&fakeOperationRepo{}, &fakeRecurringOperationRepo{},
		newFakeCategoryRepoForOwner(ownerID), fakeClock{now: date(2024, 6, 30)}, fakeTzResolver{})
	ops, err := svc.GenerateRentOperations(ctx, lease, recID, ownerID, testRentCategoryID)
	if err != nil {
		t.Fatalf("generate rent operations: %v", err)
	}
	if len(ops) == 0 {
		t.Fatal("expected generated rent operations")
	}

	for _, op := range ops {
		if op.Status != domain.OperationStatusUnconfirmed {
			t.Fatalf("expected past rent operation %s to be unconfirmed, got %s", op.OperationDate.Format("2006-01-02"), op.Status)
		}
	}
}

func TestRebuildSchedule_DeletedManualLeaseOperationDoesNotBlockGeneratedRent(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	leaseID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	recID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	deletedOperationID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	deletedAt := date(2024, 4, 20)

	start := date(2024, 3, 1)
	blockedDate := date(2024, 4, 1)

	lease := domain.Lease{
		ID:                leaseID,
		OwnerID:           ownerID,
		PropertyID:        propertyID,
		StartDate:         start,
		RentAmountKopecks: 10000,
		PaymentDay:        1,
	}

	rec := domain.RecurringOperation{
		ID:            recID,
		OwnerID:       ownerID,
		PropertyID:    propertyID,
		LeaseID:       leaseID,
		Type:          domain.OperationTypeIncome,
		CategoryID:    testRentCategoryID,
		AmountKopecks: 10000,
		StartDate:     start,
		PaymentDay:    1,
		Periodicity:   domain.RecurringOperationPeriodicityMonthly,
		Status:        domain.RecurringOperationStatusActive,
	}

	opsRepo := &fakeOperationRepo{}
	recRepo := &fakeRecurringOperationRepo{recs: map[uuid.UUID]domain.RecurringOperation{recID: rec}}

	mustCreateOperation(t, opsRepo, ctx, domain.Operation{
		ID:            deletedOperationID,
		OwnerID:       ownerID,
		PropertyID:    propertyID,
		LeaseID:       leaseID,
		Type:          domain.OperationTypeIncome,
		CategoryID:    testCustomIncomeCategoryID,
		AmountKopecks: 5000,
		OperationDate: blockedDate,
		IsException:   true,
		DeletedAt:     &deletedAt,
	})

	svc := NewRentService(opsRepo, recRepo, newFakeCategoryRepoForOwner(ownerID), fakeClock{now: date(2024, 6, 1)}, fakeTzResolver{})
	if err := svc.RebuildSchedule(ctx, lease, start); err != nil {
		t.Fatalf("RebuildSchedule failed: %v", err)
	}

	finalOps, err := opsRepo.ListByLease(ctx, leaseID)
	if err != nil {
		t.Fatalf("list operations: %v", err)
	}

	var generatedRentOnBlockedDate bool
	for _, op := range finalOps {
		if op.CategoryID == testRentCategoryID && timeutil.Date(op.OperationDate).Equal(blockedDate) {
			generatedRentOnBlockedDate = true
			break
		}
	}
	if !generatedRentOnBlockedDate {
		t.Fatalf("expected deleted manual lease operation not to block generated rent on %s", blockedDate.Format("2006-01-02"))
	}
}

func TestRebuildSchedule_MovedGeneratedRentOperationBlocksOriginalScheduleDate(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	leaseID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	recID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	movedOperationID := uuid.MustParse("55555555-5555-5555-5555-555555555555")

	start := date(2024, 3, 1)
	originalScheduleDate := date(2024, 4, 1)
	movedDate := date(2024, 4, 15)
	now := date(2024, 6, 1)

	lease := domain.Lease{
		ID:                leaseID,
		OwnerID:           ownerID,
		PropertyID:        propertyID,
		StartDate:         start,
		RentAmountKopecks: 10000,
		PaymentDay:        1,
	}

	rec := domain.RecurringOperation{
		ID:            recID,
		OwnerID:       ownerID,
		PropertyID:    propertyID,
		LeaseID:       leaseID,
		Type:          domain.OperationTypeIncome,
		CategoryID:    testRentCategoryID,
		AmountKopecks: 10000,
		StartDate:     start,
		PaymentDay:    1,
		Periodicity:   domain.RecurringOperationPeriodicityMonthly,
		Status:        domain.RecurringOperationStatusActive,
	}

	opsRepo := &fakeOperationRepo{}
	recRepo := &fakeRecurringOperationRepo{recs: map[uuid.UUID]domain.RecurringOperation{recID: rec}}

	mustCreateOperation(t, opsRepo, ctx, domain.Operation{
		ID:                   movedOperationID,
		OwnerID:              ownerID,
		PropertyID:           propertyID,
		LeaseID:              leaseID,
		RecurringOperationID: recID,
		Type:                 domain.OperationTypeIncome,
		CategoryID:           testRentCategoryID,
		AmountKopecks:        10000,
		OperationDate:        movedDate,
		SourceOperationDate:  &originalScheduleDate,
		IsException:          true,
	})

	svc := NewRentService(opsRepo, recRepo, newFakeCategoryRepoForOwner(ownerID), fakeClock{now: now}, fakeTzResolver{})
	if err := svc.RebuildSchedule(ctx, lease, start); err != nil {
		t.Fatalf("RebuildSchedule failed: %v", err)
	}

	finalOps, err := opsRepo.ListByLease(ctx, leaseID)
	if err != nil {
		t.Fatalf("list operations: %v", err)
	}

	var originalDateCount int
	var movedDateCount int
	for _, op := range finalOps {
		switch timeutil.Date(op.OperationDate) {
		case originalScheduleDate:
			originalDateCount++
		case movedDate:
			movedDateCount++
		}
	}

	if originalDateCount != 0 {
		t.Fatalf("expected moved generated operation to block original date %s, got %d operation(s)",
			originalScheduleDate.Format("2006-01-02"), originalDateCount)
	}
	if movedDateCount != 1 {
		t.Fatalf("expected moved generated operation on %s to be preserved once, got %d", movedDate.Format("2006-01-02"), movedDateCount)
	}
}

func TestRebuildSchedule_EarlierStartDatePreservesPastOperations(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	leaseID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	recID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	originalStart := date(2024, 3, 1)
	newStart := date(2024, 1, 1)
	now := date(2024, 6, 1)

	lease := domain.Lease{
		ID:                leaseID,
		OwnerID:           ownerID,
		PropertyID:        propertyID,
		StartDate:         newStart,
		RentAmountKopecks: 10000,
		PaymentDay:        1,
	}

	rec := domain.RecurringOperation{
		ID:            recID,
		OwnerID:       ownerID,
		PropertyID:    propertyID,
		LeaseID:       leaseID,
		Type:          domain.OperationTypeIncome,
		CategoryID:    testRentCategoryID,
		AmountKopecks: 10000,
		StartDate:     originalStart,
		PaymentDay:    1,
		Periodicity:   domain.RecurringOperationPeriodicityMonthly,
		Status:        domain.RecurringOperationStatusActive,
	}

	opsRepo := &fakeOperationRepo{}
	recRepo := &fakeRecurringOperationRepo{recs: map[uuid.UUID]domain.RecurringOperation{recID: rec}}

	// Seed generated rent operations for the original schedule.
	for _, d := range []time.Time{date(2024, 3, 1), date(2024, 4, 1), date(2024, 5, 1)} {
		mustCreateOperation(t, opsRepo, ctx, domain.Operation{
			OwnerID:              ownerID,
			PropertyID:           propertyID,
			LeaseID:              leaseID,
			RecurringOperationID: recID,
			Type:                 domain.OperationTypeIncome,
			CategoryID:           testRentCategoryID,
			AmountKopecks:        10000,
			OperationDate:        d,
			IsException:          false,
		})
	}

	// Seed a manual exception that should survive the rebuild.
	exceptionID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	mustCreateOperation(t, opsRepo, ctx, domain.Operation{
		ID:            exceptionID,
		OwnerID:       ownerID,
		PropertyID:    propertyID,
		LeaseID:       leaseID,
		Type:          domain.OperationTypeIncome,
		CategoryID:    testRentCategoryID,
		AmountKopecks: 5000,
		OperationDate: date(2024, 4, 1),
		IsException:   true,
	})

	svc := NewRentService(opsRepo, recRepo, newFakeCategoryRepoForOwner(ownerID), fakeClock{now: now}, fakeTzResolver{})
	if err := svc.RebuildSchedule(ctx, lease, originalStart); err != nil {
		t.Fatalf("RebuildSchedule failed: %v", err)
	}

	finalOps, err := opsRepo.ListByLease(ctx, leaseID)
	if err != nil {
		t.Fatalf("list operations: %v", err)
	}

	// The exception must still exist.
	if _, err := opsRepo.GetByIDAndOwner(ctx, exceptionID, ownerID); err != nil {
		t.Fatalf("historical exception operation was deleted: %v", err)
	}

	// Historical generated dates must still be present in the rebuilt schedule.
	hasDate := func(want time.Time) bool {
		for _, op := range finalOps {
			if timeutil.Date(op.OperationDate).Equal(want) {
				return true
			}
		}
		return false
	}
	for _, d := range []time.Time{date(2024, 3, 1), date(2024, 4, 1), date(2024, 5, 1)} {
		if !hasDate(d) {
			t.Errorf("missing historical operation date %s after rebuilding schedule", d.Format("2006-01-02"))
		}
	}

	// New dates introduced by the earlier start must be present.
	for _, d := range []time.Time{date(2024, 1, 1), date(2024, 2, 1)} {
		if !hasDate(d) {
			t.Errorf("missing new operation date %s after rebuilding schedule", d.Format("2006-01-02"))
		}
	}
}

func date(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// Compile-time interface checks.
var (
	_ OperationRepository          = (*fakeOperationRepo)(nil)
	_ RecurringOperationRepository = (*fakeRecurringOperationRepo)(nil)
	_ clock.Clock                  = fakeClock{}
)
