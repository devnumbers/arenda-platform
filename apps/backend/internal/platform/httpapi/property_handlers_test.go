package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	notificationsdomain "github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	propertiesdomain "github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// fakePropertyRepoForHandlers satisfies propertiesapp.PropertyRepository.
// fakeLeasesAppPropertyRepo wraps the same data for leasesapp.PropertyRepository.
// Only GetByIDAndOwner, ExistsByOwner, CountActiveByOwner and WithTx are functional.
type fakePropertyRepoForHandlers struct {
	mu         sync.Mutex
	properties map[uuid.UUID]propertiesdomain.Property
}

func newFakePropertyRepoForHandlers(properties ...propertiesdomain.Property) *fakePropertyRepoForHandlers {
	data := make(map[uuid.UUID]propertiesdomain.Property, len(properties))
	for _, p := range properties {
		data[p.ID] = p
	}
	return &fakePropertyRepoForHandlers{properties: data}
}

func (r *fakePropertyRepoForHandlers) Create(_ context.Context, _ uuid.UUID, property propertiesdomain.Property) (propertiesdomain.Property, error) {
	return property, nil
}

func (r *fakePropertyRepoForHandlers) GetByIDAndOwner(_ context.Context, id, _ uuid.UUID) (propertiesdomain.Property, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.properties[id]
	if !ok {
		return propertiesdomain.Property{}, propertiesapp.ErrNotFound
	}
	return p, nil
}

func (r *fakePropertyRepoForHandlers) GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (propertiesdomain.Property, error) {
	return r.GetByIDAndOwner(ctx, id, ownerID)
}

func (r *fakePropertyRepoForHandlers) ListActiveByOwner(_ context.Context, _ uuid.UUID) ([]propertiesdomain.Property, error) {
	return nil, nil
}

func (r *fakePropertyRepoForHandlers) Update(_ context.Context, _ uuid.UUID, property propertiesdomain.Property) (propertiesdomain.Property, error) {
	return property, nil
}

func (r *fakePropertyRepoForHandlers) Archive(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

func (r *fakePropertyRepoForHandlers) Unarchive(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

func (r *fakePropertyRepoForHandlers) CountActiveByOwner(_ context.Context, _ uuid.UUID) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, p := range r.properties {
		if p.Status == propertiesdomain.PropertyStatusActive {
			count++
		}
	}
	return count, nil
}

func (r *fakePropertyRepoForHandlers) ExistsActiveByOwner(_ context.Context, id, _ uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.properties[id]
	return ok && p.Status == propertiesdomain.PropertyStatusActive, nil
}

func (r *fakePropertyRepoForHandlers) ExistsByOwner(_ context.Context, id, _ uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.properties[id]
	return ok, nil
}

func (r *fakePropertyRepoForHandlers) HasOpenLease(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, nil
}

func (r *fakePropertyRepoForHandlers) WithTx(_ transaction.Tx) propertiesapp.PropertyRepository {
	return r
}

var _ propertiesapp.PropertyRepository = (*fakePropertyRepoForHandlers)(nil)

// fakeLeasesAppPropertyRepo satisfies leasesapp.PropertyRepository.
// It is backed by a fakePropertyRepoForHandlers so both views share data.
type fakeLeasesAppPropertyRepo struct {
	wrapped *fakePropertyRepoForHandlers
}

func (r *fakeLeasesAppPropertyRepo) ExistsActiveByOwner(ctx context.Context, id, ownerID uuid.UUID) (bool, error) {
	return r.wrapped.ExistsActiveByOwner(ctx, id, ownerID)
}

func (r *fakeLeasesAppPropertyRepo) ExistsByOwner(ctx context.Context, id, ownerID uuid.UUID) (bool, error) {
	return r.wrapped.ExistsByOwner(ctx, id, ownerID)
}

func (r *fakeLeasesAppPropertyRepo) HasOpenLease(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, nil
}

func (r *fakeLeasesAppPropertyRepo) WithTx(_ transaction.Tx) leasesapp.PropertyRepository {
	return r
}

var _ leasesapp.PropertyRepository = (*fakeLeasesAppPropertyRepo)(nil)

// fakeOperationRepoForHandlers satisfies leasesapp.OperationRepository.
// Create, HasDepositReturnForLease and GetPropertyOperationsSummary are
// functional; the rest are stubs.
type fakeOperationRepoForHandlers struct {
	mu  sync.Mutex
	ops []leasesdomain.Operation
}

func (r *fakeOperationRepoForHandlers) Create(_ context.Context, op leasesdomain.Operation) (leasesdomain.Operation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if op.ID == uuid.Nil {
		op.ID = uuid.New()
	}
	r.ops = append(r.ops, op)
	return op, nil
}

func (r *fakeOperationRepoForHandlers) BulkCreate(_ context.Context, _ []leasesdomain.Operation) error {
	return nil
}

func (r *fakeOperationRepoForHandlers) ListByOwner(_ context.Context, _ uuid.UUID, _ leasesapp.OperationFilter) ([]leasesdomain.Operation, error) {
	return nil, nil
}

func (r *fakeOperationRepoForHandlers) ListByLease(_ context.Context, _ uuid.UUID) ([]leasesdomain.Operation, error) {
	return nil, nil
}

func (r *fakeOperationRepoForHandlers) ListByRecurringOperation(_ context.Context, _ uuid.UUID) ([]leasesdomain.Operation, error) {
	return nil, nil
}

func (r *fakeOperationRepoForHandlers) ListOperationDatesByLease(_ context.Context, _ uuid.UUID) ([]time.Time, error) {
	return nil, nil
}

func (r *fakeOperationRepoForHandlers) ListOperationDatesByRecurringOperation(_ context.Context, _ uuid.UUID) ([]time.Time, error) {
	return nil, nil
}

func (r *fakeOperationRepoForHandlers) ListByProperty(_ context.Context, _, _ uuid.UUID) ([]leasesdomain.Operation, error) {
	return nil, nil
}

func (r *fakeOperationRepoForHandlers) ListByPropertyWithStatuses(_ context.Context, _, _ uuid.UUID, _ []leasesdomain.OperationStatus) ([]leasesdomain.Operation, error) {
	return nil, nil
}

func (r *fakeOperationRepoForHandlers) GetByIDAndOwner(_ context.Context, _ uuid.UUID, _ uuid.UUID) (leasesdomain.Operation, error) {
	return leasesdomain.Operation{}, leasesapp.ErrNotFound
}

func (r *fakeOperationRepoForHandlers) GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (leasesdomain.Operation, error) {
	return r.GetByIDAndOwner(ctx, id, ownerID)
}

func (r *fakeOperationRepoForHandlers) Update(_ context.Context, op leasesdomain.Operation) (leasesdomain.Operation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.ops {
		if r.ops[i].ID == op.ID {
			r.ops[i] = op
			return op, nil
		}
	}
	return leasesdomain.Operation{}, leasesapp.ErrNotFound
}

func (r *fakeOperationRepoForHandlers) MarkOverdue(_ context.Context, _, _ uuid.UUID, _ time.Time) (leasesdomain.Operation, bool, error) {
	return leasesdomain.Operation{}, false, nil
}

func (r *fakeOperationRepoForHandlers) SoftDeleteOperation(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

func (r *fakeOperationRepoForHandlers) DeleteUneditedFutureOperationsByLease(_ context.Context, _ uuid.UUID, _ time.Time) error {
	return nil
}

func (r *fakeOperationRepoForHandlers) DeleteUneditedFutureOperationsByRecurringOperation(_ context.Context, _ uuid.UUID, _ time.Time) error {
	return nil
}

func (r *fakeOperationRepoForHandlers) DeleteUneditedOperationsByRecurringOperation(_ context.Context, _ uuid.UUID, _ time.Time) error {
	return nil
}

func (r *fakeOperationRepoForHandlers) DeleteOperationsOutsideLeaseRange(_ context.Context, _ uuid.UUID, _ time.Time, _ *time.Time) error {
	return nil
}

func (r *fakeOperationRepoForHandlers) DeleteUneditedOperationsByLease(_ context.Context, _ uuid.UUID, _ time.Time) error {
	return nil
}

func (r *fakeOperationRepoForHandlers) ListPendingOperationsWithPastDate(_ context.Context, _ uuid.UUID, _ time.Time, _ int) ([]leasesdomain.Operation, error) {
	return nil, nil
}

func (r *fakeOperationRepoForHandlers) ListAllPendingOperationsWithPastDate(_ context.Context, _ time.Time, _ int) ([]leasesdomain.Operation, error) {
	return nil, nil
}

func (r *fakeOperationRepoForHandlers) GetPropertyOperationsSummary(_ context.Context, ownerID, propertyID uuid.UUID, asOf time.Time) (leasesapp.OperationsSummary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	monthStart := time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)
	var allTimeProfit, monthlyProfit int64
	overdueRentCount := 0
	overdueTotalCount := 0
	var nextPaymentDate *time.Time
	for _, op := range r.ops {
		if op.OwnerID != ownerID || op.PropertyID != propertyID || op.DeletedAt != nil {
			continue
		}
		opDate := timeutil.Date(op.OperationDate)
		switch op.Type {
		case leasesdomain.OperationTypeIncome:
			if op.Status == leasesdomain.OperationStatusReceived {
				allTimeProfit += op.AmountKopecks
				if !opDate.Before(monthStart) && opDate.Before(monthEnd) {
					monthlyProfit += op.AmountKopecks
				}
			}
			if op.Status == leasesdomain.OperationStatusOverdue {
				overdueTotalCount++
				if op.Category == leasesdomain.OperationCategoryRent {
					overdueRentCount++
				}
			}
			if (op.Status == leasesdomain.OperationStatusPending || op.Status == leasesdomain.OperationStatusOverdue) && op.Category == leasesdomain.OperationCategoryRent {
				if nextPaymentDate == nil || opDate.Before(*nextPaymentDate) {
					d := opDate
					nextPaymentDate = &d
				}
			}
		case leasesdomain.OperationTypeExpense:
			if op.Status == leasesdomain.OperationStatusPaid {
				allTimeProfit -= op.AmountKopecks
				if !opDate.Before(monthStart) && opDate.Before(monthEnd) {
					monthlyProfit -= op.AmountKopecks
				}
			}
			if op.Status == leasesdomain.OperationStatusOverdue {
				overdueTotalCount++
			}
		}
	}
	return leasesapp.OperationsSummary{
		AllTimeProfitKopecks: allTimeProfit,
		MonthlyProfitKopecks: monthlyProfit,
		OverdueRentCount:     overdueRentCount,
		OverdueTotalCount:    overdueTotalCount,
		NextPaymentDate:      nextPaymentDate,
	}, nil
}

func (r *fakeOperationRepoForHandlers) HasDepositReturnForLease(_ context.Context, leaseID uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, op := range r.ops {
		if op.LeaseID == leaseID && op.Type == leasesdomain.OperationTypeExpense && op.Category == leasesdomain.OperationCategoryDepositReturn && op.DeletedAt == nil {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeOperationRepoForHandlers) GetFinanceReportTotals(_ context.Context, _ uuid.UUID, _, _ time.Time) (leasesapp.FinanceReportTotals, error) {
	return leasesapp.FinanceReportTotals{}, nil
}

func (r *fakeOperationRepoForHandlers) GetFinanceReportByProperty(_ context.Context, _ uuid.UUID, _, _ time.Time) ([]leasesapp.FinanceReportPropertyRow, error) {
	return nil, nil
}

func (r *fakeOperationRepoForHandlers) GetFinanceReportByCategory(_ context.Context, _ uuid.UUID, _, _ time.Time) ([]leasesapp.FinanceReportCategoryRow, error) {
	return nil, nil
}

func (r *fakeOperationRepoForHandlers) GetFinanceReportByMonth(_ context.Context, _ uuid.UUID, _, _ time.Time) ([]leasesapp.FinanceReportMonthRow, error) {
	return nil, nil
}

func (r *fakeOperationRepoForHandlers) WithTx(_ transaction.Tx) leasesapp.OperationRepository {
	return r
}

var _ leasesapp.OperationRepository = (*fakeOperationRepoForHandlers)(nil)

// fakeRecurringOperationRepoForHandlers satisfies leasesapp.RecurringOperationRepository.
type fakeRecurringOperationRepoForHandlers struct{}

func (fakeRecurringOperationRepoForHandlers) Create(_ context.Context, rec leasesdomain.RecurringOperation) (leasesdomain.RecurringOperation, error) {
	return rec, nil
}

func (fakeRecurringOperationRepoForHandlers) GetByLeaseID(_ context.Context, _, _ uuid.UUID) (leasesdomain.RecurringOperation, error) {
	return leasesdomain.RecurringOperation{}, leasesapp.ErrNotFound
}

func (fakeRecurringOperationRepoForHandlers) GetByIDAndOwner(_ context.Context, _ uuid.UUID, _ uuid.UUID) (leasesdomain.RecurringOperation, error) {
	return leasesdomain.RecurringOperation{}, leasesapp.ErrNotFound
}

func (fakeRecurringOperationRepoForHandlers) GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (leasesdomain.RecurringOperation, error) {
	return fakeRecurringOperationRepoForHandlers{}.GetByIDAndOwner(ctx, id, ownerID)
}

func (fakeRecurringOperationRepoForHandlers) ListByOwner(_ context.Context, _ uuid.UUID) ([]leasesdomain.RecurringOperation, error) {
	return nil, nil
}

func (fakeRecurringOperationRepoForHandlers) ListByProperty(_ context.Context, _, _ uuid.UUID) ([]leasesdomain.RecurringOperation, error) {
	return nil, nil
}

func (fakeRecurringOperationRepoForHandlers) ListByPropertyID(_ context.Context, _ uuid.UUID) ([]leasesdomain.RecurringOperation, error) {
	return nil, nil
}

func (fakeRecurringOperationRepoForHandlers) Update(_ context.Context, rec leasesdomain.RecurringOperation) (leasesdomain.RecurringOperation, error) {
	return rec, nil
}

func (fakeRecurringOperationRepoForHandlers) UpdateStatus(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ string) (leasesdomain.RecurringOperation, error) {
	return leasesdomain.RecurringOperation{}, nil
}

func (fakeRecurringOperationRepoForHandlers) UpdateStatusByLeaseID(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ string) error {
	return nil
}

func (fakeRecurringOperationRepoForHandlers) SetReminderOffset(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ int) error {
	return nil
}

func (fakeRecurringOperationRepoForHandlers) DeleteByLease(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (fakeRecurringOperationRepoForHandlers) WithTx(_ transaction.Tx) leasesapp.RecurringOperationRepository {
	return fakeRecurringOperationRepoForHandlers{}
}

var _ leasesapp.RecurringOperationRepository = (*fakeRecurringOperationRepoForHandlers)(nil)

// fakeScheduler satisfies leasesapp.ReminderScheduler (notificationsapp.ReminderScheduler).
type fakeScheduler struct{}

func (fakeScheduler) ScheduleForOperation(_ context.Context, _ notificationsapp.OperationInfo, _ time.Time) error {
	return nil
}

func (fakeScheduler) ScheduleOverdueReminder(_ context.Context, _ notificationsapp.OperationInfo, _ time.Time) error {
	return nil
}

func (fakeScheduler) ScheduleForRecurringOperation(_ context.Context, _ notificationsapp.RecurringOperationInfo, _ time.Time, _ []notificationsapp.OperationInfo) error {
	return nil
}

func (fakeScheduler) ScheduleForLease(_ context.Context, _ notificationsapp.LeaseInfo) error {
	return nil
}

func (fakeScheduler) EnsureRequiresActionReminder(_ context.Context, _ notificationsapp.LeaseInfo) error {
	return nil
}

func (fakeScheduler) CancelByOperation(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (fakeScheduler) CancelOverdueReminderByOperation(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (fakeScheduler) CancelByRecurringOperation(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (fakeScheduler) CancelByLease(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (fakeScheduler) HasReminderForOperationEvent(_ context.Context, _, _ uuid.UUID, _ notificationsdomain.EventType) (bool, error) {
	return false, nil
}

func (fakeScheduler) ListByOperation(_ context.Context, _, _ uuid.UUID, _ notificationsapp.ListFilter) ([]notificationsdomain.Reminder, error) {
	return nil, nil
}

func (fakeScheduler) ListByLease(_ context.Context, _, _ uuid.UUID, _ notificationsapp.ListFilter) ([]notificationsdomain.Reminder, error) {
	return nil, nil
}

func (fakeScheduler) WithTx(_ transaction.Tx) notificationsapp.ReminderScheduler {
	return fakeScheduler{}
}

var _ leasesapp.ReminderScheduler = fakeScheduler{}

// fakePropertyPhotoRepo satisfies propertiesapp.PropertyPhotoRepository.
type fakePropertyPhotoRepo struct{}

func (fakePropertyPhotoRepo) Create(_ context.Context, _ uuid.UUID, _ string) (propertiesdomain.Photo, error) {
	return propertiesdomain.Photo{}, nil
}

func (fakePropertyPhotoRepo) GetByPropertyID(_ context.Context, _ uuid.UUID) ([]propertiesdomain.Photo, error) {
	return nil, nil
}

func (fakePropertyPhotoRepo) GetByPropertyIDs(_ context.Context, _ []uuid.UUID) (map[uuid.UUID][]propertiesdomain.Photo, error) {
	return map[uuid.UUID][]propertiesdomain.Photo{}, nil
}

func (fakePropertyPhotoRepo) CountByPropertyID(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}

func (fakePropertyPhotoRepo) GetByID(_ context.Context, _ uuid.UUID) (propertiesdomain.Photo, error) {
	return propertiesdomain.Photo{}, nil
}

func (fakePropertyPhotoRepo) GetByIDAndPropertyID(_ context.Context, _, _ uuid.UUID) (propertiesdomain.Photo, error) {
	return propertiesdomain.Photo{}, nil
}

func (fakePropertyPhotoRepo) Delete(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (fakePropertyPhotoRepo) WithTx(_ transaction.Tx) propertiesapp.PropertyPhotoRepository {
	return fakePropertyPhotoRepo{}
}

var _ propertiesapp.PropertyPhotoRepository = fakePropertyPhotoRepo{}

// fakePropertyPhotoStorage satisfies propertiesapp.PhotoStorage.
type fakePropertyPhotoStorage struct{}

func (fakePropertyPhotoStorage) Upload(_ context.Context, _, _ string, _ io.Reader) (string, error) {
	return "", nil
}

func (fakePropertyPhotoStorage) Delete(_ context.Context, _ string) error {
	return nil
}

var _ propertiesapp.PhotoStorage = fakePropertyPhotoStorage{}

// fakeOccupancyProviderForHandlers satisfies propertiesapp.OccupancyProvider.
type fakeOccupancyProviderForHandlers struct{}

func (fakeOccupancyProviderForHandlers) IsOccupied(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return false, nil
}

func (fakeOccupancyProviderForHandlers) OccupiedPropertyIDs(_ context.Context, _ uuid.UUID) (map[uuid.UUID]bool, error) {
	return map[uuid.UUID]bool{}, nil
}

func (fakeOccupancyProviderForHandlers) WithTx(_ transaction.Tx) propertiesapp.OccupancyProvider {
	return fakeOccupancyProviderForHandlers{}
}

var _ propertiesapp.OccupancyProvider = fakeOccupancyProviderForHandlers{}

// fakeSubscriptionLimiterForHandlers satisfies propertiesapp.SubscriptionLimiter.
type fakeSubscriptionLimiterForHandlers struct{}

func (fakeSubscriptionLimiterForHandlers) ActivePropertyLimit(_ context.Context, _ uuid.UUID) (int, error) {
	return 10, nil
}

func (fakeSubscriptionLimiterForHandlers) WithTx(_ transaction.Tx) propertiesapp.SubscriptionLimiter {
	return fakeSubscriptionLimiterForHandlers{}
}

var _ propertiesapp.SubscriptionLimiter = fakeSubscriptionLimiterForHandlers{}

// fakePropertyBillingLifecycleForHandlers satisfies propertiesapp.PropertyBillingLifecycle.
type fakePropertyBillingLifecycleForHandlers struct{}

func (fakePropertyBillingLifecycleForHandlers) Suspend(_ context.Context, _ uuid.UUID, _ time.Time) error {
	return nil
}

func (fakePropertyBillingLifecycleForHandlers) Resume(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ time.Time) error {
	return nil
}

func (fakePropertyBillingLifecycleForHandlers) WithTx(_ transaction.Tx) propertiesapp.PropertyBillingLifecycle {
	return fakePropertyBillingLifecycleForHandlers{}
}

var _ propertiesapp.PropertyBillingLifecycle = fakePropertyBillingLifecycleForHandlers{}

func TestPropertyHandlers_ListPropertyLeases(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	propertyID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	contactID1 := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a03")
	contactID2 := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a04")

	property := propertiesdomain.Property{
		ID:      propertyID,
		OwnerID: ownerID,
		Name:    "Test Property",
		Address: "Address",
		Type:    propertiesdomain.PropertyTypeApartment,
		Status:  propertiesdomain.PropertyStatusActive,
	}

	leases := []leasesdomain.Lease{
		{ID: uuid.New(), OwnerID: ownerID, PropertyID: propertyID, Status: leasesdomain.LeaseStatusActive, TenantContactID: &contactID1, StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), PaymentDay: 1},
		{ID: uuid.New(), OwnerID: ownerID, PropertyID: propertyID, Status: leasesdomain.LeaseStatusActive, TenantContactID: &contactID2, StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), PaymentDay: 1},
	}

	propertyRepo := newFakePropertyRepoForHandlers(property)
	leaseRepo := &fakeLeaseRepo{leases: leases}
	contactRepo := &fakeTenantContactRepo{
		contacts: []leasesdomain.TenantContact{
			{ID: contactID1, OwnerID: ownerID, Name: "Alice"},
			{ID: contactID2, OwnerID: ownerID, Name: "Bob"},
		},
	}

	propertySvc := propertiesapp.NewPropertyService(
		propertyRepo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		fakeOccupancyProviderForHandlers{},
		fakeSubscriptionLimiterForHandlers{},
		fakePropertyBillingLifecycleForHandlers{},
		leaseRepo,
		fakeLeaseBeginner{},
		fakeLeaseClock{},
		slog.Default(),
	)
	tenantContactSvc := leasesapp.NewTenantContactService(contactRepo, slog.Default())
	handlers := NewPropertyHandlers(propertySvc, nil, tenantContactSvc, nil, slog.Default())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/properties/"+propertyID.String()+"/leases", nil)
	req = req.WithContext(withOwnerID(req.Context(), ownerID))
	rr := httptest.NewRecorder()

	handlers.ListPropertyLeases(rr, req, propertyID)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if contactRepo.listByIDsCalls != 1 {
		t.Fatalf("expected 1 batch contact load, got %d", contactRepo.listByIDsCalls)
	}
	if contactRepo.getByIDCalls != 0 {
		t.Fatalf("expected 0 individual contact loads, got %d", contactRepo.getByIDCalls)
	}

	var resp openapi.PropertyLeasesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 leases, got %d", len(resp.Items))
	}
	if resp.Items[0].TenantContact == nil || resp.Items[0].TenantContact.Name != "Alice" {
		t.Fatalf("expected first tenant contact Alice, got %v", resp.Items[0].TenantContact)
	}
	if resp.Items[1].TenantContact == nil || resp.Items[1].TenantContact.Name != "Bob" {
		t.Fatalf("expected second tenant contact Bob, got %v", resp.Items[1].TenantContact)
	}
}

func TestPropertyHandlers_ListPropertyLeases_Unauthorized(t *testing.T) {
	propertyID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")

	handlers := NewPropertyHandlers(nil, nil, nil, nil, slog.Default())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/properties/"+propertyID.String()+"/leases", nil)
	rr := httptest.NewRecorder()

	handlers.ListPropertyLeases(rr, req, propertyID)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPropertyHandlers_ListPropertyLeases_NotFound(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	propertyID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")

	propertyRepo := newFakePropertyRepoForHandlers()
	propertySvc := propertiesapp.NewPropertyService(
		propertyRepo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		fakeOccupancyProviderForHandlers{},
		fakeSubscriptionLimiterForHandlers{},
		fakePropertyBillingLifecycleForHandlers{},
		&fakeLeaseRepo{},
		fakeLeaseBeginner{},
		fakeLeaseClock{},
		slog.Default(),
	)
	handlers := NewPropertyHandlers(propertySvc, nil, nil, nil, slog.Default())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/properties/"+propertyID.String()+"/leases", nil)
	req = req.WithContext(withOwnerID(req.Context(), ownerID))
	rr := httptest.NewRecorder()

	handlers.ListPropertyLeases(rr, req, propertyID)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPropertyHandlers_GetPropertyOperationsSummary(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	propertyID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
	leaseID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a03")
	now := fakeLeaseClock{}.Now()

	property := propertiesdomain.Property{
		ID:      propertyID,
		OwnerID: ownerID,
		Name:    "Test Property",
		Address: "Address",
		Type:    propertiesdomain.PropertyTypeApartment,
		Status:  propertiesdomain.PropertyStatusActive,
	}

	opsRepo := &fakeOperationRepoForHandlers{}
	_, _ = opsRepo.Create(context.Background(), leasesdomain.Operation{
		ID:            uuid.New(),
		OwnerID:       ownerID,
		PropertyID:    propertyID,
		LeaseID:       leaseID,
		Type:          leasesdomain.OperationTypeIncome,
		Category:      leasesdomain.OperationCategoryRent,
		Status:        leasesdomain.OperationStatusReceived,
		AmountKopecks: 50000,
		OperationDate: now,
	})

	propertyRepo := newFakePropertyRepoForHandlers(property)
	opSvc := leasesapp.NewOperationService(
		opsRepo,
		&fakeLeasesAppPropertyRepo{wrapped: propertyRepo},
		&fakeLeaseRepo{},
		fakeRecurringOperationRepoForHandlers{},
		fakeScheduler{},
		fakeLeaseBeginner{},
		fakeLeaseClock{},
		slog.Default(),
	)
	handlers := NewPropertyHandlers(nil, nil, nil, opSvc, slog.Default())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/properties/"+propertyID.String()+"/operations/summary", nil)
	req = req.WithContext(withOwnerID(req.Context(), ownerID))
	rr := httptest.NewRecorder()

	handlers.GetPropertyOperationsSummary(rr, req, propertyID)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp openapi.PropertyOperationsSummaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.AllTimeProfitKopecks != 50000 {
		t.Fatalf("expected all_time_profit 50000, got %d", resp.AllTimeProfitKopecks)
	}
	if resp.MonthlyProfitKopecks != 50000 {
		t.Fatalf("expected monthly_profit 50000, got %d", resp.MonthlyProfitKopecks)
	}
}

func TestPropertyHandlers_GetPropertyOperationsSummary_NotFound(t *testing.T) {
	ownerID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
	propertyID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")

	propertyRepo := newFakePropertyRepoForHandlers()
	opSvc := leasesapp.NewOperationService(
		&fakeOperationRepoForHandlers{},
		&fakeLeasesAppPropertyRepo{wrapped: propertyRepo},
		&fakeLeaseRepo{},
		fakeRecurringOperationRepoForHandlers{},
		fakeScheduler{},
		fakeLeaseBeginner{},
		fakeLeaseClock{},
		slog.Default(),
	)
	handlers := NewPropertyHandlers(nil, nil, nil, opSvc, slog.Default())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/properties/"+propertyID.String()+"/operations/summary", nil)
	req = req.WithContext(withOwnerID(req.Context(), ownerID))
	rr := httptest.NewRecorder()

	handlers.GetPropertyOperationsSummary(rr, req, propertyID)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPropertyHandlers_GetPropertyOperationsSummary_Unauthorized(t *testing.T) {
	propertyID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")

	handlers := NewPropertyHandlers(nil, nil, nil, nil, slog.Default())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/properties/"+propertyID.String()+"/operations/summary", nil)
	rr := httptest.NewRecorder()

	handlers.GetPropertyOperationsSummary(rr, req, propertyID)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rr.Code, rr.Body.String())
	}
}
