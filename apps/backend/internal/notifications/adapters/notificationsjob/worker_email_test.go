package notificationsjob

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// feedStub returns canned notifications and errors for the worker tests.
type feedStub struct {
	notification domain.Notification
	err          error
}

func (s *feedStub) WithTx(tx transaction.Tx) (application.NotificationRepository, error) {
	return s, nil
}

func (s *feedStub) Insert(ctx context.Context, n domain.Notification) (bool, error) {
	return true, nil
}

func (s *feedStub) GetByID(ctx context.Context, id uuid.UUID) (domain.Notification, error) {
	return s.notification, s.err
}

func (s *feedStub) ListPage(
	ctx context.Context, userID uuid.UUID, unreadOnly bool,
	afterCreatedAt *time.Time, afterID uuid.UUID, limit int,
) ([]domain.Notification, error) {
	return nil, nil
}
func (s *feedStub) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) { return 0, nil }
func (s *feedStub) MarkRead(ctx context.Context, userID, id uuid.UUID) (bool, error) {
	return false, nil
}

func (s *feedStub) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	return 0, nil
}

func feedNotification(t *testing.T) domain.Notification {
	t.Helper()
	n, err := domain.NewNotification(
		uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()),
		domain.EventPaymentDue,
		"Оплатите платёж",
		"Платёж по объекту «Объект»: 2000000. Срок оплаты: 1 октября",
		"Объект",
		domain.Payload{},
		domain.DedupKey("payment_due:prop:2026-10-01"),
	)
	require.NoError(t, err)
	return *n
}

// testRecipientEmail is the resolved contact shared by the email worker tests.
const testRecipientEmail = "owner@example.ru"

type resolverStub struct {
	contact application.Contact
	err     error
}

func (r resolverStub) Resolve(ctx context.Context, scope uuid.UUID) (application.Contact, error) {
	return r.contact, r.err
}

type emailerStub struct {
	calls int
	last  struct {
		to, subject, template string
		data                  map[string]any
	}
	err error
}

func (e *emailerStub) SendDirect(ctx context.Context, to, subject, template string, data map[string]any) error {
	e.calls++
	e.last.to, e.last.subject, e.last.template, e.last.data = to, subject, template, data
	return e.err
}

type limiterStub struct{ allow bool }

func (l limiterStub) Allow(key string) bool { return l.allow }

func emailJob(id uuid.UUID) *river.Job[DeliverEmailArgs] {
	return &river.Job[DeliverEmailArgs]{
		JobRow: &rivertype.JobRow{Attempt: 1},
		Args:   DeliverEmailArgs{NotificationID: id},
	}
}

func newEmailWorker(
	t *testing.T,
	feed application.NotificationRepository,
	resolver application.ContactResolver,
	emailer application.DirectEmailSender,
	limiter providerLimiter,
) *DeliverEmailWorker {
	t.Helper()
	metrics, err := NewEmailMetrics()
	require.NoError(t, err)
	return NewDeliverEmailWorker(feed, resolver, emailer, limiter, metrics, "https://app.example", nil)
}

func TestDeliverEmailWorkerSendsRenderedNotification(t *testing.T) {
	t.Parallel()

	n := feedNotification(t)
	feed := &feedStub{notification: n}
	emailer := &emailerStub{}
	w := newEmailWorker(t, feed,
		resolverStub{contact: application.Contact{Email: testRecipientEmail}},
		emailer, limiterStub{allow: true})

	require.NoError(t, w.Work(context.Background(), emailJob(n.ID)))

	require.Equal(t, 1, emailer.calls)
	assert.Equal(t, testRecipientEmail, emailer.last.to)
	assert.Equal(t, n.Title, emailer.last.subject)
	assert.Equal(t, "notification", emailer.last.template)
	assert.Equal(t, n.Title, emailer.last.data["Title"])
	assert.Equal(t, n.Body, emailer.last.data["Body"])
	// No deep link is mapped for the event type yet: the email renders
	// without a button.
	assert.Empty(t, emailer.last.data["ActionURL"])
}

func TestDeliverEmailWorkerSkipsRecipientWithoutContact(t *testing.T) {
	t.Parallel()

	n := feedNotification(t)
	emailer := &emailerStub{}
	w := newEmailWorker(t,
		&feedStub{notification: n},
		resolverStub{err: application.ErrNoContact},
		emailer, limiterStub{allow: true})

	require.NoError(t, w.Work(context.Background(), emailJob(n.ID)))
	assert.Zero(t, emailer.calls, "a recipient without a verified contact has no email leg")
}

func TestDeliverEmailWorkerSnoozesWhenProviderBudgetSpent(t *testing.T) {
	t.Parallel()

	n := feedNotification(t)
	emailer := &emailerStub{}
	w := newEmailWorker(t,
		&feedStub{notification: n},
		resolverStub{contact: application.Contact{Email: testRecipientEmail}},
		emailer, limiterStub{allow: false})

	err := w.Work(context.Background(), emailJob(n.ID))
	require.Error(t, err)
	var snooze *rivertype.JobSnoozeError
	require.ErrorAs(t, err, &snooze, "an exhausted provider budget snoozes without burning an attempt")
	assert.Equal(t, providerSnooze, snooze.Duration)
	assert.Zero(t, emailer.calls)
}

func TestDeliverEmailWorkerRetriesSendFailure(t *testing.T) {
	t.Parallel()

	n := feedNotification(t)
	emailer := &emailerStub{err: errors.New("smtp dial timeout")}
	w := newEmailWorker(t,
		&feedStub{notification: n},
		resolverStub{contact: application.Contact{Email: testRecipientEmail}},
		emailer, limiterStub{allow: true})

	err := w.Work(context.Background(), emailJob(n.ID))
	require.ErrorContains(t, err, "send notification email")
	assert.Equal(t, 1, emailer.calls, "a failed send follows the retry ladder")
}

func TestDeliverEmailWorkerCancelsWhenFeedRowGone(t *testing.T) {
	t.Parallel()

	w := newEmailWorker(t,
		&feedStub{err: application.ErrNotFound},
		resolverStub{}, &emailerStub{}, limiterStub{allow: true})

	err := w.Work(context.Background(), emailJob(uuid.Must(uuid.NewV7())))
	require.Error(t, err)
	var cancel *rivertype.JobCancelError
	require.ErrorAs(t, err, &cancel, "a vanished feed row has nothing to retry")
}

func TestDeliverEmailWorkerRetriesFeedFailure(t *testing.T) {
	t.Parallel()

	w := newEmailWorker(t,
		&feedStub{err: errors.New("db unavailable")},
		resolverStub{}, &emailerStub{}, limiterStub{allow: true})

	err := w.Work(context.Background(), emailJob(uuid.Must(uuid.NewV7())))
	require.ErrorContains(t, err, "load notification")
}
