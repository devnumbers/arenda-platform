package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type fakeReminderRepo struct {
	due           []domain.Reminder
	markSent      []markSentCall
	markFailed    []markFailedCall
	listDueErr    error
	markSentErr   error
	markFailedErr error
}

type markSentCall struct {
	id uuid.UUID
	at time.Time
}

type markFailedCall struct {
	id       uuid.UUID
	next     *time.Time
	terminal bool
}

func (r *fakeReminderRepo) Save(ctx context.Context, rm domain.Reminder) error {
	return nil
}

func (r *fakeReminderRepo) Update(ctx context.Context, rm domain.Reminder) error {
	return nil
}

func (r *fakeReminderRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Reminder, error) {
	return domain.Reminder{}, nil
}

func (r *fakeReminderRepo) ListByOwner(ctx context.Context, ownerID uuid.UUID, filter application.ListFilter) ([]domain.Reminder, error) {
	return nil, nil
}

func (r *fakeReminderRepo) ListDue(ctx context.Context, before time.Time, limit int) ([]domain.Reminder, error) {
	if r.listDueErr != nil {
		return nil, r.listDueErr
	}
	return r.due, nil
}

func (r *fakeReminderRepo) MarkSent(ctx context.Context, id uuid.UUID, at time.Time) error {
	r.markSent = append(r.markSent, markSentCall{id: id, at: at})
	return r.markSentErr
}

func (r *fakeReminderRepo) MarkFailed(ctx context.Context, id uuid.UUID, nextAttempt *time.Time, terminal bool) error {
	r.markFailed = append(r.markFailed, markFailedCall{id: id, next: nextAttempt, terminal: terminal})
	return r.markFailedErr
}

func (r *fakeReminderRepo) CancelByTarget(ctx context.Context, ownerID uuid.UUID, targetType domain.TargetType, targetID uuid.UUID, eventType domain.EventType) error {
	return nil
}

func (r *fakeReminderRepo) CancelByRecurringOperationID(ctx context.Context, ownerID, recID uuid.UUID) error {
	return nil
}

func (r *fakeReminderRepo) WithTx(tx transaction.Tx) application.ReminderRepository {
	return r
}

type fakeNotifier struct {
	notifications []application.Notification
	err           error
}

func (n *fakeNotifier) Notify(ctx context.Context, notification application.Notification) error {
	n.notifications = append(n.notifications, notification)
	return n.err
}

type fakeContactResolver struct {
	contact application.Contact
	err     error
	calls   []uuid.UUID
}

func (r *fakeContactResolver) Resolve(ctx context.Context, ownerID uuid.UUID) (application.Contact, error) {
	r.calls = append(r.calls, ownerID)
	return r.contact, r.err
}

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	return c.now
}

type fakeBackoff struct {
	d time.Duration
}

func (b *fakeBackoff) Next(attempt int) time.Duration {
	return b.d
}

func newTestReminder(id uuid.UUID, ownerID uuid.UUID, failedAttempts int) domain.Reminder {
	return domain.Reminder{
		ID:             id,
		OwnerID:        ownerID,
		TargetType:     domain.TargetOperation,
		OperationID:    &id,
		PropertyID:     &ownerID,
		EventType:      domain.EventOperationDue,
		Status:         domain.ReminderPending,
		MessageTitle:   "title",
		MessageBody:    "body",
		FailedAttempts: failedAttempts,
	}
}

func TestReminderWorker_DispatchMarksSent(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	reminderID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := &fakeReminderRepo{due: []domain.Reminder{newTestReminder(reminderID, ownerID, 0)}}
	notifier := &fakeNotifier{}
	resolver := &fakeContactResolver{contact: application.Contact{Channel: application.ChannelSMS, Address: "+7999"}}
	worker := NewReminderWorker(repo, resolver, notifier, &fakeClock{now: now}, &fakeBackoff{d: 5 * time.Minute}, 3, time.Minute, 10, nil)

	if err := worker.tick(context.Background()); err != nil {
		t.Fatalf("tick failed: %v", err)
	}

	if len(repo.markSent) != 1 {
		t.Fatalf("expected 1 MarkSent call, got %d", len(repo.markSent))
	}
	if repo.markSent[0].id != reminderID || !repo.markSent[0].at.Equal(now) {
		t.Errorf("MarkSent called with wrong args: %+v", repo.markSent[0])
	}
	if len(repo.markFailed) != 0 {
		t.Errorf("expected no MarkFailed calls, got %d", len(repo.markFailed))
	}
	if len(notifier.notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifier.notifications))
	}
	n := notifier.notifications[0]
	if n.RecipientID != ownerID || n.ReminderID != reminderID || n.EventType != domain.EventOperationDue {
		t.Errorf("unexpected notification: %+v", n)
	}
	if n.Title == "" || n.Body == "" {
		t.Error("notification title and body must not be empty")
	}
	if len(resolver.calls) != 1 || resolver.calls[0] != ownerID {
		t.Errorf("expected contact resolver to be called for owner %s, calls: %v", ownerID, resolver.calls)
	}
}

func TestReminderWorker_FailureIncrementsAttemptsAndSchedulesRetry(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	reminderID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := &fakeReminderRepo{due: []domain.Reminder{newTestReminder(reminderID, ownerID, 0)}}
	notifier := &fakeNotifier{err: errors.New("send failed")}
	resolver := &fakeContactResolver{contact: application.Contact{Channel: application.ChannelSMS, Address: "+7999"}}
	worker := NewReminderWorker(repo, resolver, notifier, &fakeClock{now: now}, &fakeBackoff{d: 5 * time.Minute}, 3, time.Minute, 10, nil)

	if err := worker.tick(context.Background()); err != nil {
		t.Fatalf("tick failed: %v", err)
	}

	if len(repo.markSent) != 0 {
		t.Errorf("expected no MarkSent calls, got %d", len(repo.markSent))
	}
	if len(repo.markFailed) != 1 {
		t.Fatalf("expected 1 MarkFailed call, got %d", len(repo.markFailed))
	}
	call := repo.markFailed[0]
	if call.id != reminderID || call.terminal {
		t.Errorf("expected non-terminal failure for %s, got %+v", reminderID, call)
	}
	if call.next == nil || !call.next.Equal(now.Add(5*time.Minute)) {
		t.Errorf("expected retry at %v, got %v", now.Add(5*time.Minute), call.next)
	}
}

func TestReminderWorker_TerminalFailedAfterMaxAttempts(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	reminderID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := &fakeReminderRepo{due: []domain.Reminder{newTestReminder(reminderID, ownerID, 2)}}
	notifier := &fakeNotifier{err: errors.New("send failed")}
	resolver := &fakeContactResolver{contact: application.Contact{Channel: application.ChannelSMS, Address: "+7999"}}
	worker := NewReminderWorker(repo, resolver, notifier, &fakeClock{now: now}, &fakeBackoff{d: 5 * time.Minute}, 3, time.Minute, 10, nil)

	if err := worker.tick(context.Background()); err != nil {
		t.Fatalf("tick failed: %v", err)
	}

	if len(repo.markFailed) != 1 {
		t.Fatalf("expected 1 MarkFailed call, got %d", len(repo.markFailed))
	}
	call := repo.markFailed[0]
	if call.id != reminderID || !call.terminal || call.next != nil {
		t.Errorf("expected terminal failure with no retry, got %+v", call)
	}
}

func TestReminderWorker_ContactResolutionFailureMarksFailedAndRetries(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	reminderID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := &fakeReminderRepo{due: []domain.Reminder{newTestReminder(reminderID, ownerID, 0)}}
	notifier := &fakeNotifier{}
	resolver := &fakeContactResolver{err: errors.New("no contact")}
	worker := NewReminderWorker(repo, resolver, notifier, &fakeClock{now: now}, &fakeBackoff{d: 5 * time.Minute}, 3, time.Minute, 10, nil)

	if err := worker.tick(context.Background()); err != nil {
		t.Fatalf("tick failed: %v", err)
	}

	if len(notifier.notifications) != 0 {
		t.Errorf("expected no notifications on resolver failure, got %d", len(notifier.notifications))
	}
	if len(repo.markFailed) != 1 {
		t.Fatalf("expected 1 MarkFailed call, got %d", len(repo.markFailed))
	}
	if repo.markFailed[0].terminal {
		t.Error("expected non-terminal failure on resolver error")
	}
}

func TestReminderWorker_NoDueRemindersResultsInNoCalls(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)

	repo := &fakeReminderRepo{due: []domain.Reminder{}}
	notifier := &fakeNotifier{}
	resolver := &fakeContactResolver{}
	worker := NewReminderWorker(repo, resolver, notifier, &fakeClock{now: now}, &fakeBackoff{d: 5 * time.Minute}, 3, time.Minute, 10, nil)

	if err := worker.tick(context.Background()); err != nil {
		t.Fatalf("tick failed: %v", err)
	}

	if len(repo.markSent) != 0 || len(repo.markFailed) != 0 {
		t.Error("expected no repository mutation calls when no reminders are due")
	}
	if len(notifier.notifications) != 0 || len(resolver.calls) != 0 {
		t.Error("expected no notifier or resolver calls when no reminders are due")
	}
}
