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
	reminders          []domain.Reminder
	isSMSReminderSent  bool
	saveSentSMSErr     error
	saveSentEmailErr   error
	markFailedErr      error
	getByIDUnscoped    domain.Reminder
	markSendingPending error
	channelAllowed     map[uuid.UUID]bool // nil or missing entry means allowed
	sentEmailAudit     map[uuid.UUID]bool // recipientID → audit row exists
	savedEmails        []application.SaveSentEmailReminderParams
	deletedEmailAudit  []uuid.UUID
	markSentCalls      int
	markSkippedCalls   int
	markFailedCalls    int
	cancelCalls        int
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

func (r *fakeReminderRepoForWorker) ListByOwner(context.Context, uuid.UUID, application.ListFilter, []uuid.UUID) ([]domain.Reminder, error) {
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

func (r *fakeReminderRepoForWorker) ListCalendarByOwner(context.Context, uuid.UUID, time.Time, time.Time, []uuid.UUID) ([]domain.CalendarReminder, error) {
	return nil, nil
}

func (r *fakeReminderRepoForWorker) MarkReminderSending(context.Context, uuid.UUID) (domain.Reminder, error) {
	return domain.Reminder{}, nil
}

func (r *fakeReminderRepoForWorker) MarkSent(context.Context, uuid.UUID, time.Time) error {
	r.markSentCalls++
	return nil
}

func (r *fakeReminderRepoForWorker) MarkReminderSent(context.Context, uuid.UUID, time.Time) error {
	r.markSentCalls++
	return nil
}

func (r *fakeReminderRepoForWorker) MarkFailed(context.Context, uuid.UUID, *time.Time, bool) error {
	r.markFailedCalls++
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

func (r *fakeReminderRepoForWorker) SaveSentEmailReminder(_ context.Context, arg application.SaveSentEmailReminderParams) error {
	if r.saveSentEmailErr != nil {
		return r.saveSentEmailErr
	}
	if r.sentEmailAudit == nil {
		r.sentEmailAudit = make(map[uuid.UUID]bool)
	}
	if r.sentEmailAudit[arg.ScopeID] {
		return application.ErrDuplicateEmailReminder
	}
	r.sentEmailAudit[arg.ScopeID] = true
	r.savedEmails = append(r.savedEmails, arg)
	return nil
}

func (r *fakeReminderRepoForWorker) IsEmailReminderSent(_ context.Context, _, recipientID uuid.UUID) (bool, error) {
	return r.sentEmailAudit[recipientID], nil
}

func (r *fakeReminderRepoForWorker) DeleteSentEmailReminder(_ context.Context, _, recipientID uuid.UUID) error {
	delete(r.sentEmailAudit, recipientID)
	r.deletedEmailAudit = append(r.deletedEmailAudit, recipientID)
	return nil
}

func (r *fakeReminderRepoForWorker) IsPushReminderSent(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return false, nil
}

func (r *fakeReminderRepoForWorker) SaveSentPushReminder(_ context.Context, _ application.SaveSentPushReminderParams) error {
	return nil
}

func (r *fakeReminderRepoForWorker) DeleteSentPushReminder(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (r *fakeReminderRepoForWorker) ResetReminderSending(context.Context, uuid.UUID) error {
	return nil
}

func (r *fakeReminderRepoForWorker) MarkSendingReminderPending(context.Context, uuid.UUID, time.Time) error {
	return r.markSendingPending
}

func (r *fakeReminderRepoForWorker) MarkReminderSkipped(context.Context, uuid.UUID) error {
	r.markSkippedCalls++
	return nil
}

func (r *fakeReminderRepoForWorker) ListPreferences(context.Context, uuid.UUID) ([]domain.NotificationPreference, error) {
	return nil, nil
}

func (r *fakeReminderRepoForWorker) UpsertPreference(context.Context, uuid.UUID, domain.NotificationPreference) error {
	return nil
}

// IsEventAllowed is the legacy per-event-type stub. The worker dispatches via
// IsChannelAllowed now; this method is kept only to satisfy the repository
// interface during the expand phase (ADR 0030) and is not exercised by these
// tests.
func (r *fakeReminderRepoForWorker) IsEventAllowed(context.Context, uuid.UUID, domain.EventType) (bool, error) {
	return true, nil
}

func (r *fakeReminderRepoForWorker) ListChannelPreferences(context.Context, uuid.UUID) ([]domain.NotificationChannelPreference, error) {
	return nil, nil
}

func (r *fakeReminderRepoForWorker) UpsertChannelPreference(context.Context, uuid.UUID, domain.NotificationChannelPreference) error {
	return nil
}

func (r *fakeReminderRepoForWorker) IsChannelAllowed(_ context.Context, userID uuid.UUID, _ domain.EventType, _ domain.NotificationChannel) (bool, error) {
	if r.channelAllowed != nil {
		if allowed, ok := r.channelAllowed[userID]; ok {
			return allowed, nil
		}
	}
	return true, nil
}

func (r *fakeReminderRepoForWorker) CancelByIDAndOwner(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	r.cancelCalls++
	return true, nil
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
	failFor   map[uuid.UUID]bool // when set, only these recipients fail with notifyErr
	calls     []uuid.UUID
}

func (n *fakeNotifier) Notify(_ context.Context, notif application.Notification) (string, string, error) {
	n.calls = append(n.calls, notif.RecipientID)
	if n.notifyErr == nil {
		return "", "", nil
	}
	if n.failFor == nil || n.failFor[notif.RecipientID] {
		return "", "", n.notifyErr
	}
	return "", "", nil
}

type fakeContactResolver struct {
	errFor map[uuid.UUID]error
}

func (f fakeContactResolver) Resolve(_ context.Context, userID uuid.UUID) (application.Contact, error) {
	if err, ok := f.errFor[userID]; ok {
		return application.Contact{}, err
	}
	return application.Contact{Channel: application.ChannelEmail, Email: "owner@example.com"}, nil
}

type fakePropertyRecipientLister struct {
	members map[uuid.UUID][]uuid.UUID
	err     error
}

func (f fakePropertyRecipientLister) ListActiveRecipientIDs(_ context.Context, propertyID uuid.UUID) ([]uuid.UUID, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.members[propertyID], nil
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

var workerTestNow = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

func newWorkerForTest(t *testing.T, repo *fakeReminderRepoForWorker, notifier *fakeNotifier, resolver application.ContactResolver, recipients application.PropertyRecipientLister, logger *slog.Logger) *ReminderWorker {
	t.Helper()
	renderer, err := mailer.NewRenderer("../../../templates/email")
	if err != nil {
		t.Fatalf("load email templates: %v", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return NewReminderWorker(
		repo,
		renderer,
		map[application.Channel]application.Notifier{
			application.ChannelEmail: notifier,
		},
		resolver,
		recipients,
		nil, // pushSender — disabled in email-only unit tests
		nil, // pushSubRepo
		nil, // pushMetrics
		fakeBeginnerForWorker{},
		fakeClockForWorker{now: workerTestNow},
		fakeBackoff{},
		3,
		time.Hour,
		time.Minute,
		logger,
	)
}

func newPropertyReminder(reminderID, ownerID uuid.UUID, propertyID *uuid.UUID) domain.Reminder {
	return domain.Reminder{
		ID:           reminderID,
		OwnerID:      ownerID,
		PropertyID:   propertyID,
		EventType:    domain.EventOperationDue,
		Status:       domain.ReminderSending,
		MessageTitle: "title",
		MessageBody:  "body",
	}
}

func TestReminderWorker_DispatchReminder_SanitizesProviderError(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	reminderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ownerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := &fakeReminderRepoForWorker{}
	notifier := &fakeNotifier{
		notifyErr: errors.New("smtp provider error: token=secret123 card 1234-5678-9012-3456 phone +79991234567"),
	}

	w := newWorkerForTest(t, repo, notifier, fakeContactResolver{}, fakePropertyRecipientLister{}, logger)

	r := newPropertyReminder(reminderID, ownerID, nil)

	if err := w.dispatchReminder(context.Background(), r, workerTestNow); err != nil {
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

// Fan-out: a property reminder is delivered to the owner and to every active
// member, with one audit row per recipient (issue #159).
func TestReminderWorker_DispatchReminder_FansOutToActiveMembers(t *testing.T) {
	t.Parallel()

	reminderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ownerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	memberID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	propertyID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := &fakeReminderRepoForWorker{}
	notifier := &fakeNotifier{}
	recipients := fakePropertyRecipientLister{members: map[uuid.UUID][]uuid.UUID{
		propertyID: {memberID},
	}}

	w := newWorkerForTest(t, repo, notifier, fakeContactResolver{}, recipients, nil)

	r := newPropertyReminder(reminderID, ownerID, &propertyID)
	if err := w.dispatchReminder(context.Background(), r, workerTestNow); err != nil {
		t.Fatalf("dispatchReminder unexpected error: %v", err)
	}

	if len(notifier.calls) != 2 {
		t.Fatalf("notify calls = %v, want 2 recipients", notifier.calls)
	}
	if notifier.calls[0] != ownerID || notifier.calls[1] != memberID {
		t.Errorf("notify recipients = %v, want [owner member]", notifier.calls)
	}
	if len(repo.savedEmails) != 2 {
		t.Fatalf("saved email audit rows = %d, want 2", len(repo.savedEmails))
	}
	scopes := map[uuid.UUID]bool{}
	for _, saved := range repo.savedEmails {
		if saved.ReminderID != reminderID {
			t.Errorf("audit row reminder id = %v, want %v", saved.ReminderID, reminderID)
		}
		scopes[saved.ScopeID] = true
	}
	if !scopes[ownerID] || !scopes[memberID] {
		t.Errorf("audit scope ids = %v, want owner and member", scopes)
	}
	if repo.markSentCalls != 1 {
		t.Errorf("markSent calls = %d, want 1", repo.markSentCalls)
	}
	if repo.markFailedCalls != 0 || repo.markSkippedCalls != 0 || repo.cancelCalls != 0 {
		t.Errorf("unexpected finalization: failed=%d skipped=%d cancelled=%d",
			repo.markFailedCalls, repo.markSkippedCalls, repo.cancelCalls)
	}
}

// Per-recipient preferences: an opted-out member is skipped, the owner still
// receives the reminder and it is finalized as sent (issue #159).
func TestReminderWorker_DispatchReminder_MemberOptOut(t *testing.T) {
	t.Parallel()

	reminderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ownerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	memberID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	propertyID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := &fakeReminderRepoForWorker{
		channelAllowed: map[uuid.UUID]bool{memberID: false},
	}
	notifier := &fakeNotifier{}
	recipients := fakePropertyRecipientLister{members: map[uuid.UUID][]uuid.UUID{
		propertyID: {memberID},
	}}

	w := newWorkerForTest(t, repo, notifier, fakeContactResolver{}, recipients, nil)

	r := newPropertyReminder(reminderID, ownerID, &propertyID)
	if err := w.dispatchReminder(context.Background(), r, workerTestNow); err != nil {
		t.Fatalf("dispatchReminder unexpected error: %v", err)
	}

	if len(notifier.calls) != 1 || notifier.calls[0] != ownerID {
		t.Errorf("notify calls = %v, want [owner]", notifier.calls)
	}
	if repo.markSentCalls != 1 {
		t.Errorf("markSent calls = %d, want 1", repo.markSentCalls)
	}
}

// Per-recipient preferences: an opted-out owner is skipped while an active
// member who allows the event type still receives the reminder (issue #159).
func TestReminderWorker_DispatchReminder_OwnerOptOutMemberAllowed(t *testing.T) {
	t.Parallel()

	reminderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ownerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	memberID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	propertyID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := &fakeReminderRepoForWorker{
		channelAllowed: map[uuid.UUID]bool{ownerID: false},
	}
	notifier := &fakeNotifier{}
	recipients := fakePropertyRecipientLister{members: map[uuid.UUID][]uuid.UUID{
		propertyID: {memberID},
	}}

	w := newWorkerForTest(t, repo, notifier, fakeContactResolver{}, recipients, nil)

	r := newPropertyReminder(reminderID, ownerID, &propertyID)
	if err := w.dispatchReminder(context.Background(), r, workerTestNow); err != nil {
		t.Fatalf("dispatchReminder unexpected error: %v", err)
	}

	if len(notifier.calls) != 1 || notifier.calls[0] != memberID {
		t.Errorf("notify calls = %v, want [member]", notifier.calls)
	}
	if repo.markSentCalls != 1 {
		t.Errorf("markSent calls = %d, want 1", repo.markSentCalls)
	}
}

// When every recipient opted out of the event type, the reminder is finalized
// as skipped (issue #159).
func TestReminderWorker_DispatchReminder_AllRecipientsOptOut(t *testing.T) {
	t.Parallel()

	reminderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ownerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	memberID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	propertyID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := &fakeReminderRepoForWorker{
		channelAllowed: map[uuid.UUID]bool{ownerID: false, memberID: false},
	}
	notifier := &fakeNotifier{}
	recipients := fakePropertyRecipientLister{members: map[uuid.UUID][]uuid.UUID{
		propertyID: {memberID},
	}}

	w := newWorkerForTest(t, repo, notifier, fakeContactResolver{}, recipients, nil)

	r := newPropertyReminder(reminderID, ownerID, &propertyID)
	if err := w.dispatchReminder(context.Background(), r, workerTestNow); err != nil {
		t.Fatalf("dispatchReminder unexpected error: %v", err)
	}

	if len(notifier.calls) != 0 {
		t.Errorf("notify calls = %v, want none", notifier.calls)
	}
	if repo.markSkippedCalls != 1 {
		t.Errorf("markSkipped calls = %d, want 1", repo.markSkippedCalls)
	}
	if repo.markSentCalls != 0 || repo.markFailedCalls != 0 {
		t.Errorf("unexpected finalization: sent=%d failed=%d", repo.markSentCalls, repo.markFailedCalls)
	}
}

// A send failure for one recipient fails the reminder; on the retry the
// already-delivered recipient is deduplicated by the per-recipient audit row
// and only the failed recipient is re-sent (issue #159).
func TestReminderWorker_DispatchReminder_MemberSendFailureRetriesPerRecipient(t *testing.T) {
	t.Parallel()

	reminderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ownerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	memberID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	propertyID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := &fakeReminderRepoForWorker{}
	notifier := &fakeNotifier{
		notifyErr: errors.New("smtp unavailable"),
		failFor:   map[uuid.UUID]bool{memberID: true},
	}
	recipients := fakePropertyRecipientLister{members: map[uuid.UUID][]uuid.UUID{
		propertyID: {memberID},
	}}

	w := newWorkerForTest(t, repo, notifier, fakeContactResolver{}, recipients, nil)

	r := newPropertyReminder(reminderID, ownerID, &propertyID)
	if err := w.dispatchReminder(context.Background(), r, workerTestNow); err != nil {
		t.Fatalf("first dispatchReminder unexpected error: %v", err)
	}

	if repo.markFailedCalls != 1 {
		t.Errorf("markFailed calls = %d, want 1", repo.markFailedCalls)
	}
	if len(repo.deletedEmailAudit) != 1 || repo.deletedEmailAudit[0] != memberID {
		t.Errorf("deleted audit rows = %v, want [member]", repo.deletedEmailAudit)
	}

	// Retry: the member's provider recovers. The owner is deduplicated by the
	// audit row saved on the first run; only the member is re-sent.
	notifier.failFor = nil
	notifier.notifyErr = nil
	if err := w.dispatchReminder(context.Background(), r, workerTestNow); err != nil {
		t.Fatalf("second dispatchReminder unexpected error: %v", err)
	}

	if len(notifier.calls) != 3 {
		t.Fatalf("total notify calls = %v, want [owner member member]", notifier.calls)
	}
	if notifier.calls[2] != memberID {
		t.Errorf("retry notify call = %v, want member", notifier.calls[2])
	}
	if repo.markSentCalls != 1 {
		t.Errorf("markSent calls = %d, want 1 (only the retry succeeds)", repo.markSentCalls)
	}
}

// A reminder without a property is delivered to the owner only; the member
// recipient lister is not consulted (issue #159).
func TestReminderWorker_DispatchReminder_NoPropertyOwnerOnly(t *testing.T) {
	t.Parallel()

	reminderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ownerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := &fakeReminderRepoForWorker{}
	notifier := &fakeNotifier{}
	recipients := fakePropertyRecipientLister{err: errors.New("must not be called")}

	w := newWorkerForTest(t, repo, notifier, fakeContactResolver{}, recipients, nil)

	r := newPropertyReminder(reminderID, ownerID, nil)
	if err := w.dispatchReminder(context.Background(), r, workerTestNow); err != nil {
		t.Fatalf("dispatchReminder unexpected error: %v", err)
	}

	if len(notifier.calls) != 1 || notifier.calls[0] != ownerID {
		t.Errorf("notify calls = %v, want [owner]", notifier.calls)
	}
	if repo.markSentCalls != 1 {
		t.Errorf("markSent calls = %d, want 1", repo.markSentCalls)
	}
}

// A member without a resolvable contact is skipped; the owner still receives
// the reminder (issue #159).
func TestReminderWorker_DispatchReminder_MemberNoContact(t *testing.T) {
	t.Parallel()

	reminderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ownerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	memberID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	propertyID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := &fakeReminderRepoForWorker{}
	notifier := &fakeNotifier{}
	resolver := fakeContactResolver{errFor: map[uuid.UUID]error{
		memberID: application.ErrNoContact,
	}}
	recipients := fakePropertyRecipientLister{members: map[uuid.UUID][]uuid.UUID{
		propertyID: {memberID},
	}}

	w := newWorkerForTest(t, repo, notifier, resolver, recipients, nil)

	r := newPropertyReminder(reminderID, ownerID, &propertyID)
	if err := w.dispatchReminder(context.Background(), r, workerTestNow); err != nil {
		t.Fatalf("dispatchReminder unexpected error: %v", err)
	}

	if len(notifier.calls) != 1 || notifier.calls[0] != ownerID {
		t.Errorf("notify calls = %v, want [owner]", notifier.calls)
	}
	if repo.markSentCalls != 1 {
		t.Errorf("markSent calls = %d, want 1", repo.markSentCalls)
	}
	if repo.cancelCalls != 0 {
		t.Errorf("cancel calls = %d, want 0 (owner has a contact)", repo.cancelCalls)
	}
}
