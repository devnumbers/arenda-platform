package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type fakeReminderRepoForWorker struct {
	reminders           []domain.Reminder
	isSMSReminderSent   bool
	isEmailReminderSent bool
	saveSentSMSErr      error
	saveSentEmailErr    error
	markFailedErr       error
	getByIDUnscoped     domain.Reminder
	markSendingPending  error
}

func (r *fakeReminderRepoForWorker) Save(context.Context, domain.Reminder) error { return nil }
func (r *fakeReminderRepoForWorker) SaveOrReplaceOperationReminder(context.Context, domain.Reminder) error {
	return nil
}

func (r *fakeReminderRepoForWorker) UpdateScheduledAt(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
	return nil
}

func (r *fakeReminderRepoForWorker) ReschedulePendingRemindersByOwner(context.Context, uuid.UUID, string, string) error {
	return nil
}

func (r *fakeReminderRepoForWorker) GetByID(context.Context, uuid.UUID, uuid.UUID) (domain.Reminder, error) {
	return domain.Reminder{}, nil
}

func (r *fakeReminderRepoForWorker) GetByIDUnscoped(context.Context, uuid.UUID) (domain.Reminder, error) {
	return r.getByIDUnscoped, nil
}

func (r *fakeReminderRepoForWorker) ListByOwner(context.Context, uuid.UUID, application.ListFilter) ([]domain.Reminder, error) {
	return nil, nil
}

func (r *fakeReminderRepoForWorker) ListByOperation(context.Context, uuid.UUID, uuid.UUID, application.ListFilter) ([]domain.Reminder, error) {
	return nil, nil
}

func (r *fakeReminderRepoForWorker) ListByLease(context.Context, uuid.UUID, uuid.UUID, application.ListFilter) ([]domain.Reminder, error) {
	return nil, nil
}

func (r *fakeReminderRepoForWorker) ListByRecurringOperation(context.Context, uuid.UUID, uuid.UUID, application.ListFilter) ([]domain.Reminder, error) {
	return nil, nil
}

func (r *fakeReminderRepoForWorker) ListDue(context.Context, time.Time, int) ([]domain.Reminder, error) {
	return r.reminders, nil
}

func (r *fakeReminderRepoForWorker) ListStaleSendingReminders(context.Context, time.Time, int) ([]domain.Reminder, error) {
	return nil, nil
}

func (r *fakeReminderRepoForWorker) ListUpcomingFreeRemindersByProperty(context.Context, uuid.UUID, uuid.UUID, time.Time, int) ([]domain.UpcomingFreeReminder, error) {
	return nil, nil
}

func (r *fakeReminderRepoForWorker) MarkReminderSending(context.Context, uuid.UUID) (domain.Reminder, error) {
	return domain.Reminder{}, nil
}
func (r *fakeReminderRepoForWorker) MarkSent(context.Context, uuid.UUID, time.Time) error { return nil }
func (r *fakeReminderRepoForWorker) MarkReminderSent(context.Context, uuid.UUID, time.Time) error {
	return nil
}

func (r *fakeReminderRepoForWorker) MarkFailed(context.Context, uuid.UUID, *time.Time, bool) error {
	return r.markFailedErr
}

func (r *fakeReminderRepoForWorker) SaveSentSMSReminder(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, string, time.Time) error {
	return r.saveSentSMSErr
}

func (r *fakeReminderRepoForWorker) UpdateSMSProviderResponse(context.Context, uuid.UUID, string) error {
	return nil
}

func (r *fakeReminderRepoForWorker) IsSMSReminderSent(context.Context, uuid.UUID) (bool, error) {
	return r.isSMSReminderSent, nil
}

func (r *fakeReminderRepoForWorker) SaveSentEmailReminder(context.Context, application.SaveSentEmailReminderParams) error {
	return r.saveSentEmailErr
}

func (r *fakeReminderRepoForWorker) IsEmailReminderSent(context.Context, uuid.UUID) (bool, error) {
	return r.isEmailReminderSent, nil
}

func (r *fakeReminderRepoForWorker) DeleteSentEmailReminder(context.Context, uuid.UUID) error {
	return nil
}

func (r *fakeReminderRepoForWorker) ResetReminderSending(context.Context, uuid.UUID) error {
	return nil
}

func (r *fakeReminderRepoForWorker) MarkSendingReminderPending(context.Context, uuid.UUID, time.Time) error {
	return r.markSendingPending
}

func (r *fakeReminderRepoForWorker) MarkReminderSkipped(context.Context, uuid.UUID) error {
	return nil
}

func (r *fakeReminderRepoForWorker) ListPreferences(context.Context, uuid.UUID) ([]domain.NotificationPreference, error) {
	return nil, nil
}

func (r *fakeReminderRepoForWorker) UpsertPreference(context.Context, uuid.UUID, domain.NotificationPreference) error {
	return nil
}

func (r *fakeReminderRepoForWorker) IsEventAllowed(context.Context, uuid.UUID, domain.EventType) (bool, error) {
	return true, nil
}

func (r *fakeReminderRepoForWorker) CancelByIDAndOwner(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *fakeReminderRepoForWorker) CancelByTarget(context.Context, uuid.UUID, domain.TargetType, uuid.UUID, domain.EventType) error {
	return nil
}

func (r *fakeReminderRepoForWorker) CancelByRecurringOperationID(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (r *fakeReminderRepoForWorker) HasReminderForLeaseEvent(context.Context, uuid.UUID, uuid.UUID, domain.EventType) (bool, error) {
	return false, nil
}

func (r *fakeReminderRepoForWorker) HasReminderForOperationEvent(context.Context, uuid.UUID, uuid.UUID, domain.EventType) (bool, error) {
	return false, nil
}
func (r *fakeReminderRepoForWorker) WithTx(transaction.Tx) application.ReminderRepository { return r }

type fakeNotifier struct {
	notifyErr error
}

func (n *fakeNotifier) Notify(context.Context, application.Notification) (string, string, error) {
	return "", "", n.notifyErr
}

type fakeContactResolver struct{}

func (fakeContactResolver) Resolve(context.Context, uuid.UUID) (application.Contact, error) {
	return application.Contact{Channel: application.ChannelEmail, Email: "owner@example.com"}, nil
}

type fakeTxForWorker struct{}

func (fakeTxForWorker) Commit(context.Context) error   { return nil }
func (fakeTxForWorker) Rollback(context.Context) error { return nil }

type fakeBeginnerForWorker struct{}

func (fakeBeginnerForWorker) Begin(context.Context) (transaction.Tx, error) {
	return fakeTxForWorker{}, nil
}

type fakeClockForWorker struct{ now time.Time }

func (c fakeClockForWorker) Now() time.Time { return c.now }

type fakeBackoff struct{}

func (fakeBackoff) Next(int) time.Duration { return time.Minute }

func TestReminderWorker_DispatchReminder_SanitizesProviderError(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	reminderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ownerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	repo := &fakeReminderRepoForWorker{
		isEmailReminderSent: false,
		markSendingPending:  nil,
	}
	notifier := &fakeNotifier{
		notifyErr: errors.New("smtp provider error: token=secret123 card 1234-5678-9012-3456 phone +79991234567"),
	}

	renderer, err := mailer.NewRenderer("../../../templates/email")
	if err != nil {
		t.Fatalf("load email templates: %v", err)
	}

	w := NewReminderWorker(
		repo,
		renderer,
		map[application.Channel]application.Notifier{
			application.ChannelEmail: notifier,
		},
		fakeContactResolver{},
		fakeBeginnerForWorker{},
		fakeClockForWorker{now: now},
		fakeBackoff{},
		3,
		time.Hour,
		time.Minute,
		logger,
	)

	r := domain.Reminder{
		ID:           reminderID,
		OwnerID:      ownerID,
		EventType:    domain.EventOperationDue,
		Status:       domain.ReminderSending,
		MessageTitle: "title",
		MessageBody:  "body",
	}

	if err := w.dispatchReminder(context.Background(), r, now); err != nil {
		t.Fatalf("dispatchReminder unexpected error: %v", err)
	}

	logs := logBuf.String()
	forbidden := []string{
		"token=secret123",
		"1234-5678-9012-3456",
		"owner@example.com",
	}
	for _, s := range forbidden {
		if strings.Contains(logs, s) {
			t.Errorf("log contains sensitive substring %q:\n%s", s, logs)
		}
	}
	if !strings.Contains(logs, "notify reminder failed") {
		t.Errorf("expected notify reminder failed log, got:\n%s", logs)
	}
}
