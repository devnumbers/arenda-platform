package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// This file is the role-matrix cover for the T3 policy enforcement in the
// reminder write use cases Reschedule and Cancel (issue #166): the reminder is
// loaded unscoped, its data owner becomes the scope of every repository call,
// and the actor is checked against the owner-wide role. None/suspended map to
// ErrNotFound (object privacy); a viewer maps to ErrForbidden. The global
// feeds (ListByOwner, calendar) intentionally stay actor-scoped and are not
// covered here.

// reminderActorAndPolicy returns the actor and the policy fake for a role
// case. The owner acts on their own data (the fake policy maps actor == scope
// to RoleOwner); other roles act as a member with the owner-wide role seeded
// for the (member, owner) pair.
func reminderActorAndPolicy(role sharedpolicy.Role) (uuid.UUID, fakePolicy) {
	if role == sharedpolicy.RoleOwner {
		return policyOwnerID, fakePolicy{}
	}
	return policyMemberID, fakePolicy{roles: map[[2]uuid.UUID]sharedpolicy.Role{
		{policyMemberID, policyOwnerID}: role,
	}}
}

// fakeReminderRepo is an in-memory ReminderRepository that records the scope
// of every scoped write call so tests can prove repository calls use the data
// owner, not the actor.
type fakeReminderRepo struct {
	reminders map[uuid.UUID]domain.Reminder

	updateScheduledAtCalls [][2]uuid.UUID // (scope, id)
	cancelCalls            [][2]uuid.UUID // (scope, reminderID)
	cancelled              bool           // CancelByIDAndOwner result
}

func (r *fakeReminderRepo) Save(context.Context, domain.Reminder) error { return nil }

func (r *fakeReminderRepo) SaveOrReplaceOperationReminder(context.Context, domain.Reminder) error {
	return nil
}

func (r *fakeReminderRepo) UpdateScheduledAt(_ context.Context, scope, id uuid.UUID, scheduledAt time.Time) error {
	r.updateScheduledAtCalls = append(r.updateScheduledAtCalls, [2]uuid.UUID{scope, id})
	rem := r.reminders[id]
	rem.ScheduledAt = scheduledAt
	r.reminders[id] = rem
	return nil
}

func (r *fakeReminderRepo) ReschedulePendingRemindersByOwner(context.Context, uuid.UUID, string, string) error {
	return nil
}

func (r *fakeReminderRepo) GetByID(_ context.Context, id, scope uuid.UUID) (domain.Reminder, error) {
	rem, ok := r.reminders[id]
	if !ok || rem.OwnerID != scope {
		return domain.Reminder{}, ErrNotFound
	}
	return rem, nil
}

func (r *fakeReminderRepo) GetByIDUnscoped(_ context.Context, id uuid.UUID) (domain.Reminder, error) {
	rem, ok := r.reminders[id]
	if !ok {
		return domain.Reminder{}, ErrNotFound
	}
	return rem, nil
}

func (r *fakeReminderRepo) ListByOwner(context.Context, uuid.UUID, ListFilter, []uuid.UUID) ([]domain.Reminder, error) {
	return nil, nil
}

func (r *fakeReminderRepo) ListByOperation(context.Context, uuid.UUID, uuid.UUID, ListFilter) ([]domain.Reminder, error) {
	return nil, nil
}

func (r *fakeReminderRepo) ListByLease(context.Context, uuid.UUID, uuid.UUID, ListFilter) ([]domain.Reminder, error) {
	return nil, nil
}

func (r *fakeReminderRepo) ListByRecurringOperation(context.Context, uuid.UUID, uuid.UUID, ListFilter) ([]domain.Reminder, error) {
	return nil, nil
}

func (r *fakeReminderRepo) ListDue(context.Context, time.Time, int) ([]domain.Reminder, error) {
	return nil, nil
}

func (r *fakeReminderRepo) ListStaleSendingReminders(context.Context, time.Time, int) ([]domain.Reminder, error) {
	return nil, nil
}

func (r *fakeReminderRepo) ListUpcomingFreeRemindersByProperty(context.Context, uuid.UUID, uuid.UUID, time.Time, int) ([]domain.UpcomingFreeReminder, error) {
	return nil, nil
}

func (r *fakeReminderRepo) ListCalendarByOwner(context.Context, uuid.UUID, time.Time, time.Time, []uuid.UUID) ([]domain.CalendarReminder, error) {
	return nil, nil
}

func (r *fakeReminderRepo) MarkReminderSending(context.Context, uuid.UUID) (domain.Reminder, error) {
	return domain.Reminder{}, nil
}

func (r *fakeReminderRepo) MarkSent(context.Context, uuid.UUID, time.Time) error { return nil }

func (r *fakeReminderRepo) MarkReminderSent(context.Context, uuid.UUID, time.Time) error { return nil }

func (r *fakeReminderRepo) MarkFailed(context.Context, uuid.UUID, *time.Time, bool) error { return nil }

func (r *fakeReminderRepo) SaveSentSMSReminder(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, string, time.Time) error {
	return nil
}

func (r *fakeReminderRepo) UpdateSMSProviderResponse(context.Context, uuid.UUID, string) error {
	return nil
}

func (r *fakeReminderRepo) IsSMSReminderSent(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *fakeReminderRepo) SaveSentEmailReminder(context.Context, SaveSentEmailReminderParams) error {
	return nil
}

func (r *fakeReminderRepo) IsEmailReminderSent(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *fakeReminderRepo) DeleteSentEmailReminder(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (r *fakeReminderRepo) IsPushReminderSent(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *fakeReminderRepo) SaveSentPushReminder(context.Context, SaveSentPushReminderParams) error {
	return nil
}

func (r *fakeReminderRepo) DeleteSentPushReminder(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (r *fakeReminderRepo) ResetReminderSending(context.Context, uuid.UUID) error { return nil }

func (r *fakeReminderRepo) MarkSendingReminderPending(context.Context, uuid.UUID, time.Time) error {
	return nil
}

func (r *fakeReminderRepo) MarkReminderSkipped(context.Context, uuid.UUID) error { return nil }

func (r *fakeReminderRepo) CancelByIDAndOwner(_ context.Context, scope, reminderID uuid.UUID) (bool, error) {
	r.cancelCalls = append(r.cancelCalls, [2]uuid.UUID{scope, reminderID})
	return r.cancelled, nil
}

func (r *fakeReminderRepo) CancelByTarget(context.Context, uuid.UUID, domain.TargetType, uuid.UUID, domain.EventType) error {
	return nil
}

func (r *fakeReminderRepo) CancelByRecurringOperationID(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (r *fakeReminderRepo) HasReminderForLeaseEvent(context.Context, uuid.UUID, uuid.UUID, domain.EventType) (bool, error) {
	return false, nil
}

func (r *fakeReminderRepo) HasReminderForOperationEvent(context.Context, uuid.UUID, uuid.UUID, domain.EventType) (bool, error) {
	return false, nil
}

func (r *fakeReminderRepo) ListPreferences(context.Context, uuid.UUID) ([]domain.NotificationPreference, error) {
	return nil, nil
}

func (r *fakeReminderRepo) UpsertPreference(context.Context, uuid.UUID, domain.NotificationPreference) error {
	return nil
}

func (r *fakeReminderRepo) IsEventAllowed(context.Context, uuid.UUID, domain.EventType) (bool, error) {
	return true, nil
}

func (r *fakeReminderRepo) ListChannelPreferences(context.Context, uuid.UUID) ([]domain.NotificationChannelPreference, error) {
	return nil, nil
}

func (r *fakeReminderRepo) UpsertChannelPreference(context.Context, uuid.UUID, domain.NotificationChannelPreference) error {
	return nil
}

func (r *fakeReminderRepo) IsChannelAllowed(context.Context, uuid.UUID, domain.EventType, domain.NotificationChannel) (bool, error) {
	return true, nil
}

func (r *fakeReminderRepo) WithTx(transaction.Tx) ReminderRepository { return r }

var _ ReminderRepository = (*fakeReminderRepo)(nil)

func newPolicyReminderService(policy fakePolicy, repo *fakeReminderRepo) *ReminderService {
	return NewReminderService(repo, policyClock, fakeTzResolver{}, policy)
}

func policyTestReminder() domain.Reminder {
	return domain.Reminder{
		ID:           policyReminderID,
		OwnerID:      policyOwnerID,
		TargetType:   domain.TargetFree,
		EventType:    domain.EventFreeReminder,
		Status:       domain.ReminderPending,
		ScheduledAt:  policyClock.now.Add(24 * time.Hour),
		MessageTitle: "test",
		MessageBody:  "test",
		CreatedAt:    policyClock.now,
		UpdatedAt:    policyClock.now,
	}
}

func TestPolicyEnforcement_RescheduleReminder(t *testing.T) {
	ctx := t.Context()
	newDate := policyClock.now.Add(48 * time.Hour)
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := reminderActorAndPolicy(tc.role)
			repo := &fakeReminderRepo{reminders: map[uuid.UUID]domain.Reminder{
				policyReminderID: policyTestReminder(),
			}}
			svc := newPolicyReminderService(policy, repo)

			updated, err := svc.Reschedule(ctx, actor, policyReminderID, newDate)
			switch {
			case sharedpolicy.CanEdit(tc.role):
				if err != nil {
					t.Fatalf("Reschedule: want success, got %v", err)
				}
				wantScheduledAt := domain.ScheduledAtForDate(newDate, time.UTC, dispatchHour)
				if !updated.ScheduledAt.Equal(wantScheduledAt) {
					t.Errorf("ScheduledAt: want %s, got %s", wantScheduledAt, updated.ScheduledAt)
				}
				if len(repo.updateScheduledAtCalls) != 1 || repo.updateScheduledAtCalls[0] != [2]uuid.UUID{policyOwnerID, policyReminderID} {
					t.Errorf("UpdateScheduledAt: want one call on the owner scope, got %+v", repo.updateScheduledAtCalls)
				}
			case tc.role == sharedpolicy.RoleViewer:
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("Reschedule: want ErrForbidden, got %v", err)
				}
			default:
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("Reschedule: want ErrNotFound, got %v", err)
				}
			}
			if !sharedpolicy.CanEdit(tc.role) && len(repo.updateScheduledAtCalls) != 0 {
				t.Errorf("UpdateScheduledAt: want no calls for role %s, got %+v", tc.role, repo.updateScheduledAtCalls)
			}
		})
	}
}

func TestPolicyEnforcement_CancelReminder(t *testing.T) {
	ctx := t.Context()
	for _, tc := range policyRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			actor, policy := reminderActorAndPolicy(tc.role)
			repo := &fakeReminderRepo{
				reminders: map[uuid.UUID]domain.Reminder{
					policyReminderID: policyTestReminder(),
				},
				cancelled: true,
			}
			svc := newPolicyReminderService(policy, repo)

			err := svc.Cancel(ctx, actor, policyReminderID)
			switch {
			case sharedpolicy.CanEdit(tc.role):
				if err != nil {
					t.Fatalf("Cancel: want success, got %v", err)
				}
				if len(repo.cancelCalls) != 1 || repo.cancelCalls[0] != [2]uuid.UUID{policyOwnerID, policyReminderID} {
					t.Errorf("CancelByIDAndOwner: want one call on the owner scope, got %+v", repo.cancelCalls)
				}
			case tc.role == sharedpolicy.RoleViewer:
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("Cancel: want ErrForbidden, got %v", err)
				}
			default:
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("Cancel: want ErrNotFound, got %v", err)
				}
			}
			if !sharedpolicy.CanEdit(tc.role) && len(repo.cancelCalls) != 0 {
				t.Errorf("CancelByIDAndOwner: want no calls for role %s, got %+v", tc.role, repo.cancelCalls)
			}
		})
	}
}

func TestPolicyEnforcement_RescheduleCancelMissingReminder(t *testing.T) {
	ctx := t.Context()
	repo := &fakeReminderRepo{reminders: map[uuid.UUID]domain.Reminder{}}
	svc := newPolicyReminderService(fakePolicy{}, repo)

	if _, err := svc.Reschedule(ctx, policyOwnerID, policyReminderID, policyClock.now.Add(48*time.Hour)); !errors.Is(err, ErrNotFound) {
		t.Errorf("Reschedule missing reminder: want ErrNotFound, got %v", err)
	}
	if err := svc.Cancel(ctx, policyOwnerID, policyReminderID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Cancel missing reminder: want ErrNotFound, got %v", err)
	}
}
