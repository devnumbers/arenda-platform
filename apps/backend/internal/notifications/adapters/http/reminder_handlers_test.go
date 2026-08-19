package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	notificationsdomain "github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// TestListLeaseReminders_OwnerScope covers the regression-prone link between
// the lease read gate and the reminder listing (issue #166): the handler
// resolves the lease's data owner and must pass it — never the actor — as the
// scope of ListByLease. A full-access member reads the owner's lease
// reminders through the owner's scope.

var (
	handlerOwnerID     = uuid.MustParse("dddddddd-0000-0000-0000-000000000001")
	handlerMemberID    = uuid.MustParse("dddddddd-0000-0000-0000-000000000002")
	handlerLeaseID     = uuid.MustParse("eeeeeeee-0000-0000-0000-000000000001")
	handlerPropertyID  = uuid.MustParse("eeeeeeee-0000-0000-0000-000000000002")
	handlerReminderNow = time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	errBeginnerNotUsed = errors.New("transaction beginner is not used by this test")
)

// handlerFakePolicy maps (actor, property) -> role; RoleForProperty defaults
// to RoleOwner so the owner case needs no seeding. Implements
// sharedpolicy.Policy.
type handlerFakePolicy struct {
	propertyRoles map[[2]uuid.UUID]sharedpolicy.Role
}

func (f handlerFakePolicy) Role(_ context.Context, actor, scope uuid.UUID) (sharedpolicy.Role, error) {
	if actor == scope {
		return sharedpolicy.RoleOwner, nil
	}
	return sharedpolicy.RoleNone, nil
}

func (f handlerFakePolicy) RoleForProperty(_ context.Context, actor, propertyID uuid.UUID) (sharedpolicy.Role, error) {
	if r, ok := f.propertyRoles[[2]uuid.UUID{actor, propertyID}]; ok {
		return r, nil
	}
	return sharedpolicy.RoleOwner, nil
}

var _ sharedpolicy.Policy = handlerFakePolicy{}

// handlerFakeLeaseRepo returns a fixed lease from GetByID; the other methods
// are stubs to satisfy leasesapp.LeaseRepository.
type handlerFakeLeaseRepo struct {
	lease leasesdomain.Lease
}

func (r handlerFakeLeaseRepo) Create(context.Context, uuid.UUID, leasesdomain.Lease) (leasesdomain.Lease, error) {
	return leasesdomain.Lease{}, nil
}

func (r handlerFakeLeaseRepo) GetByID(context.Context, uuid.UUID) (leasesdomain.Lease, error) {
	return r.lease, nil
}

func (r handlerFakeLeaseRepo) GetByIDForUpdate(context.Context, uuid.UUID) (leasesdomain.Lease, error) {
	return r.lease, nil
}

func (r handlerFakeLeaseRepo) GetByIDAndOwner(context.Context, uuid.UUID, uuid.UUID) (leasesdomain.Lease, error) {
	return r.lease, nil
}

func (r handlerFakeLeaseRepo) GetByIDAndOwnerForUpdate(context.Context, uuid.UUID, uuid.UUID) (leasesdomain.Lease, error) {
	return r.lease, nil
}

func (r handlerFakeLeaseRepo) ListByOwner(context.Context, uuid.UUID, []uuid.UUID) ([]leasesdomain.Lease, error) {
	return nil, nil
}

func (r handlerFakeLeaseRepo) Update(_ context.Context, _ uuid.UUID, lease leasesdomain.Lease) (leasesdomain.Lease, error) {
	return lease, nil
}

func (r handlerFakeLeaseRepo) Complete(context.Context, uuid.UUID, uuid.UUID) (leasesdomain.Lease, error) {
	return r.lease, nil
}

func (r handlerFakeLeaseRepo) CountOpenLeasesByProperty(context.Context, uuid.UUID) (int, error) {
	return 0, nil
}

func (r handlerFakeLeaseRepo) GetOpenLeaseByProperty(context.Context, uuid.UUID, uuid.UUID) (leasesdomain.Lease, error) {
	return r.lease, nil
}

func (r handlerFakeLeaseRepo) ListOpenLeasesWithPastEndDate(context.Context, time.Time, int) ([]leasesdomain.Lease, error) {
	return nil, nil
}

func (r handlerFakeLeaseRepo) ListByProperty(context.Context, uuid.UUID, uuid.UUID) ([]leasesdomain.Lease, error) {
	return nil, nil
}

func (r handlerFakeLeaseRepo) ListWithTenantForExport(context.Context, uuid.UUID, uuid.UUID) ([]leasesapp.ExportLeaseRow, error) {
	return nil, nil
}

func (r handlerFakeLeaseRepo) WithTx(transaction.Tx) leasesapp.LeaseRepository { return r }

var _ leasesapp.LeaseRepository = handlerFakeLeaseRepo{}

// handlerFakeCategories stubs leasesapp.OperationCategoryRepository; the
// constructor requires a non-nil value but GetLease never calls it.
type handlerFakeCategories struct{}

func (handlerFakeCategories) Create(context.Context, uuid.UUID, leasesdomain.OperationType, string) (leasesdomain.OperationCategory, error) {
	return leasesdomain.OperationCategory{}, nil
}

func (handlerFakeCategories) ListByOwner(context.Context, uuid.UUID, *leasesdomain.OperationType) ([]leasesdomain.OperationCategory, error) {
	return nil, nil
}

func (handlerFakeCategories) GetByIDAndOwner(context.Context, uuid.UUID, uuid.UUID) (leasesdomain.OperationCategory, error) {
	return leasesdomain.OperationCategory{}, nil
}

func (handlerFakeCategories) GetByOwnerAndCode(context.Context, uuid.UUID, leasesdomain.OperationCategoryDefaultCode) (leasesdomain.OperationCategory, error) {
	return leasesdomain.OperationCategory{}, nil
}

func (handlerFakeCategories) CreateDefaultCategories(context.Context, uuid.UUID) error { return nil }

func (handlerFakeCategories) WithTx(transaction.Tx) leasesapp.OperationCategoryRepository {
	return handlerFakeCategories{}
}

var _ leasesapp.OperationCategoryRepository = handlerFakeCategories{}

// handlerFakeBeginner satisfies the leases txBeginner parameter; GetLease
// never begins a transaction, so Begin fails loudly if it ever is called.
type handlerFakeBeginner struct{}

func (handlerFakeBeginner) Begin(context.Context) (transaction.Tx, error) {
	return nil, errBeginnerNotUsed
}

type handlerFakeClock struct{}

func (handlerFakeClock) Now() time.Time { return handlerReminderNow }

type handlerFakeTzResolver struct{}

func (handlerFakeTzResolver) Resolve(context.Context, uuid.UUID) (*time.Location, error) {
	return time.UTC, nil
}

// handlerFakeReminderRepo records the scope and lease of ListByLease calls;
// the other methods are stubs to satisfy notificationsapp.ReminderRepository.
type handlerFakeReminderRepo struct {
	listByLeaseCall [2]uuid.UUID // (scope, leaseID)
	called          bool
}

func (r *handlerFakeReminderRepo) Save(context.Context, notificationsdomain.Reminder) error {
	return nil
}

func (r *handlerFakeReminderRepo) SaveOrReplaceOperationReminder(context.Context, notificationsdomain.Reminder) error {
	return nil
}

func (r *handlerFakeReminderRepo) UpdateScheduledAt(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
	return nil
}

func (r *handlerFakeReminderRepo) ReschedulePendingRemindersByOwner(context.Context, uuid.UUID, string, string) error {
	return nil
}

func (r *handlerFakeReminderRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (notificationsdomain.Reminder, error) {
	return notificationsdomain.Reminder{}, nil
}

func (r *handlerFakeReminderRepo) GetByIDUnscoped(context.Context, uuid.UUID) (notificationsdomain.Reminder, error) {
	return notificationsdomain.Reminder{}, nil
}

func (r *handlerFakeReminderRepo) ListByOwner(context.Context, uuid.UUID, notificationsapp.ListFilter, []uuid.UUID) ([]notificationsdomain.Reminder, error) {
	return nil, nil
}

func (r *handlerFakeReminderRepo) ListByOperation(context.Context, uuid.UUID, uuid.UUID, notificationsapp.ListFilter) ([]notificationsdomain.Reminder, error) {
	return nil, nil
}

func (r *handlerFakeReminderRepo) ListByLease(_ context.Context, scope, leaseID uuid.UUID, _ notificationsapp.ListFilter) ([]notificationsdomain.Reminder, error) {
	r.listByLeaseCall = [2]uuid.UUID{scope, leaseID}
	r.called = true
	return nil, nil
}

func (r *handlerFakeReminderRepo) ListByRecurringOperation(context.Context, uuid.UUID, uuid.UUID, notificationsapp.ListFilter) ([]notificationsdomain.Reminder, error) {
	return nil, nil
}

func (r *handlerFakeReminderRepo) ListDue(context.Context, time.Time, int) ([]notificationsdomain.Reminder, error) {
	return nil, nil
}

func (r *handlerFakeReminderRepo) ListStaleSendingReminders(context.Context, time.Time, int) ([]notificationsdomain.Reminder, error) {
	return nil, nil
}

func (r *handlerFakeReminderRepo) ListCalendarByOwner(context.Context, uuid.UUID, time.Time, time.Time, []uuid.UUID) ([]notificationsdomain.CalendarReminder, error) {
	return nil, nil
}

func (r *handlerFakeReminderRepo) MarkReminderSending(context.Context, uuid.UUID) (notificationsdomain.Reminder, error) {
	return notificationsdomain.Reminder{}, nil
}

func (r *handlerFakeReminderRepo) MarkSent(context.Context, uuid.UUID, time.Time) error { return nil }

func (r *handlerFakeReminderRepo) MarkReminderSent(context.Context, uuid.UUID, time.Time) error {
	return nil
}

func (r *handlerFakeReminderRepo) MarkFailed(context.Context, uuid.UUID, *time.Time, bool) error {
	return nil
}

func (r *handlerFakeReminderRepo) SaveSentSMSReminder(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, string, time.Time) error {
	return nil
}

func (r *handlerFakeReminderRepo) UpdateSMSProviderResponse(context.Context, uuid.UUID, string) error {
	return nil
}

func (r *handlerFakeReminderRepo) IsSMSReminderSent(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *handlerFakeReminderRepo) SaveSentEmailReminder(context.Context, notificationsapp.SaveSentEmailReminderParams) error {
	return nil
}

func (r *handlerFakeReminderRepo) IsEmailReminderSent(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *handlerFakeReminderRepo) DeleteSentEmailReminder(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (r *handlerFakeReminderRepo) IsPushReminderSent(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *handlerFakeReminderRepo) SaveSentPushReminder(context.Context, notificationsapp.SaveSentPushReminderParams) error {
	return nil
}

func (r *handlerFakeReminderRepo) DeleteSentPushReminder(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (r *handlerFakeReminderRepo) ResetReminderSending(context.Context, uuid.UUID) error { return nil }

func (r *handlerFakeReminderRepo) MarkSendingReminderPending(context.Context, uuid.UUID, time.Time) error {
	return nil
}

func (r *handlerFakeReminderRepo) MarkReminderSkipped(context.Context, uuid.UUID) error { return nil }

func (r *handlerFakeReminderRepo) CancelByIDAndOwner(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *handlerFakeReminderRepo) CancelByTarget(context.Context, uuid.UUID, notificationsdomain.TargetType, uuid.UUID, notificationsdomain.EventType) error {
	return nil
}

func (r *handlerFakeReminderRepo) CancelByRecurringOperationID(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (r *handlerFakeReminderRepo) HasReminderForLeaseEvent(context.Context, uuid.UUID, uuid.UUID, notificationsdomain.EventType) (bool, error) {
	return false, nil
}

func (r *handlerFakeReminderRepo) HasReminderForOperationEvent(context.Context, uuid.UUID, uuid.UUID, notificationsdomain.EventType) (bool, error) {
	return false, nil
}

func (r *handlerFakeReminderRepo) ListChannelPreferences(context.Context, uuid.UUID) ([]notificationsdomain.NotificationChannelPreference, error) {
	return nil, nil
}

func (r *handlerFakeReminderRepo) UpsertChannelPreference(context.Context, uuid.UUID, notificationsdomain.NotificationChannelPreference) error {
	return nil
}

func (r *handlerFakeReminderRepo) IsChannelAllowed(context.Context, uuid.UUID, notificationsdomain.EventType, notificationsdomain.NotificationChannel) (bool, error) {
	return true, nil
}

func (r *handlerFakeReminderRepo) WithTx(transaction.Tx) notificationsapp.ReminderRepository {
	return r
}

var _ notificationsapp.ReminderRepository = (*handlerFakeReminderRepo)(nil)

func newListLeaseRemindersHandler(reminderRepo *handlerFakeReminderRepo) *ReminderHandlers {
	leaseSvc := leasesapp.NewLeaseService(
		handlerFakeLeaseRepo{lease: leasesdomain.Lease{
			ID:         handlerLeaseID,
			OwnerID:    handlerOwnerID,
			PropertyID: handlerPropertyID,
			Status:     leasesdomain.LeaseStatusActive,
			StartDate:  handlerReminderNow.Add(-30 * 24 * time.Hour),
			CreatedAt:  handlerReminderNow,
			UpdatedAt:  handlerReminderNow,
		}},
		nil, // properties: unused by GetLease
		nil, // tenantContacts: unused
		nil, // recurringOps: unused
		nil, // operations: unused
		handlerFakeCategories{},
		nil, // scheduler: unused
		handlerFakeBeginner{},
		nil, // audit: unused
		handlerFakeClock{},
		handlerFakeTzResolver{},
		handlerFakePolicy{propertyRoles: map[[2]uuid.UUID]sharedpolicy.Role{
			{handlerMemberID, handlerPropertyID}: sharedpolicy.RoleFullAccess,
		}},
		nil, // logger: defaults to slog.Default()
	)
	reminderSvc := notificationsapp.NewReminderService(reminderRepo, handlerFakeClock{}, handlerFakeTzResolver{}, nil)
	return NewReminderHandlers(reminderSvc, nil, nil, nil, leaseSvc, slog.Default())
}

func TestListLeaseReminders_OwnerScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		actor uuid.UUID
	}{
		{name: "owner", actor: handlerOwnerID},
		{name: "full access member", actor: handlerMemberID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &handlerFakeReminderRepo{}
			h := newListLeaseRemindersHandler(repo)

			req := httptest.NewRequestWithContext(httpsupport.WithUserID(t.Context(), tt.actor), http.MethodGet, "/", nil)
			rr := httptest.NewRecorder()

			h.ListLeaseReminders(rr, req, handlerLeaseID)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
			}
			if !repo.called {
				t.Fatal("ListByLease was not called")
			}
			if repo.listByLeaseCall != [2]uuid.UUID{handlerOwnerID, handlerLeaseID} {
				t.Errorf("ListByLease scope: want the lease data owner %s, got %+v", handlerOwnerID, repo.listByLeaseCall)
			}
		})
	}
}
