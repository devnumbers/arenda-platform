package notificationsjob

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pushSubsStub struct {
	subs    []domain.PushSubscription
	deleted []string
	listErr error
}

func (s *pushSubsStub) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.PushSubscription, error) {
	return s.subs, s.listErr
}

func (s *pushSubsStub) Delete(ctx context.Context, userID uuid.UUID, endpoint string) error {
	s.deleted = append(s.deleted, endpoint)
	return nil
}

func (s *pushSubsStub) Upsert(ctx context.Context, sub domain.PushSubscription) (domain.PushSubscription, error) {
	return sub, nil
}

type senderStub struct {
	sent   []domain.PushSubscription
	last   application.PushPayload
	errSeq []error // Per-call outcomes; a nil entry means success.
}

func (s *senderStub) Send(ctx context.Context, sub domain.PushSubscription, payload application.PushPayload) error {
	i := len(s.sent)
	s.sent = append(s.sent, sub)
	s.last = payload
	if i < len(s.errSeq) {
		if err := s.errSeq[i]; err != nil {
			return err
		}
	}
	return nil
}

func pushSub(endpoint string) domain.PushSubscription {
	return domain.PushSubscription{
		ID:       uuid.Must(uuid.NewV7()),
		UserID:   uuid.Must(uuid.NewV7()),
		Endpoint: endpoint,
		P256dh:   "p256dh",
		Auth:     "auth",
	}
}

func pushJob(id uuid.UUID) *river.Job[DeliverPushArgs] {
	return &river.Job[DeliverPushArgs]{
		JobRow: &rivertype.JobRow{Attempt: 1},
		Args:   DeliverPushArgs{NotificationID: id},
	}
}

func newPushWorker(
	t *testing.T,
	feed application.NotificationRepository,
	sender application.PushSender,
	subs application.PushSubscriptionRepository,
) *DeliverPushWorker {
	t.Helper()
	return NewDeliverPushWorker(feed, sender, subs, nil)
}

func TestDeliverPushWorkerFansOutOverSubscriptions(t *testing.T) {
	t.Parallel()

	n := feedNotification(t)
	sub1, sub2 := pushSub("https://push.example/1"), pushSub("https://push.example/2")
	sub2.UserID = n.UserID
	sub1.UserID = n.UserID
	subs := &pushSubsStub{subs: []domain.PushSubscription{sub1, sub2}}
	sender := &senderStub{}
	w := newPushWorker(t, &feedStub{notification: n}, sender, subs)

	require.NoError(t, w.Work(context.Background(), pushJob(n.ID)))

	require.Len(t, sender.sent, 2)
	assert.Equal(t, n.Title, sender.last.Title)
	assert.Equal(t, n.Body, sender.last.Body)
	assert.Equal(t, n.ID.String(), sender.last.Tag, "the payload tag is the notification id: retries replace, distinct notifications don't")
	assert.Equal(t, n.EventType, sender.last.EventType)
	assert.Empty(t, sender.last.URL, "no deep link mapped for the event type yet")
}

func TestDeliverPushWorkerDeletesDeadSubscriptionAndContinues(t *testing.T) {
	t.Parallel()

	n := feedNotification(t)
	dead, alive := pushSub("https://push.example/dead"), pushSub("https://push.example/alive")
	dead.UserID, alive.UserID = n.UserID, n.UserID
	subs := &pushSubsStub{subs: []domain.PushSubscription{dead, alive}}
	sender := &senderStub{errSeq: []error{application.ErrSubscriptionGone}}
	w := newPushWorker(t, &feedStub{notification: n}, sender, subs)

	require.NoError(t, w.Work(context.Background(), pushJob(n.ID)))

	assert.Equal(t, []string{dead.Endpoint}, subs.deleted, "a 404/410 subscription is deleted on the spot")
	require.Len(t, sender.sent, 2, "the fan-out continues past a dead device")
}

func TestDeliverPushWorkerSnoozesOnRateLimit(t *testing.T) {
	t.Parallel()

	n := feedNotification(t)
	sub1, sub2 := pushSub("https://push.example/1"), pushSub("https://push.example/2")
	sub1.UserID, sub2.UserID = n.UserID, n.UserID
	subs := &pushSubsStub{subs: []domain.PushSubscription{sub1, sub2}}
	sender := &senderStub{errSeq: []error{application.ErrRateLimited}}
	w := newPushWorker(t, &feedStub{notification: n}, sender, subs)

	job := pushJob(n.ID)
	job.Attempt = 2
	err := w.Work(context.Background(), job)

	require.Error(t, err)
	var snooze *rivertype.JobSnoozeError
	require.ErrorAs(t, err, &snooze)
	assert.Equal(t, 2*pushRateSnoozeStep, snooze.Duration, "the snooze grows with the attempt")
	assert.Len(t, sender.sent, 1, "the throttled fan-out stops; the retry re-runs it")
}

func TestDeliverPushWorkerLogsPerDeviceFailureAndContinues(t *testing.T) {
	t.Parallel()

	n := feedNotification(t)
	sub1, sub2 := pushSub("https://push.example/1"), pushSub("https://push.example/2")
	sub1.UserID, sub2.UserID = n.UserID, n.UserID
	subs := &pushSubsStub{subs: []domain.PushSubscription{sub1, sub2}}
	sender := &senderStub{errSeq: []error{errors.New("push service 500")}}
	w := newPushWorker(t, &feedStub{notification: n}, sender, subs)

	require.NoError(t, w.Work(context.Background(), pushJob(n.ID)), "a single device's failure never fails the job")
	require.Len(t, sender.sent, 2)
	assert.Empty(t, subs.deleted)
}

func TestDeliverPushWorkerNoSubscriptionsIsNoop(t *testing.T) {
	t.Parallel()

	n := feedNotification(t)
	sender := &senderStub{}
	w := newPushWorker(t, &feedStub{notification: n}, sender, &pushSubsStub{})

	require.NoError(t, w.Work(context.Background(), pushJob(n.ID)))
	assert.Empty(t, sender.sent)
}

func TestDeliverPushWorkerCancelsWhenFeedRowGone(t *testing.T) {
	t.Parallel()

	w := newPushWorker(t, &feedStub{err: application.ErrNotFound}, &senderStub{}, &pushSubsStub{})

	err := w.Work(context.Background(), pushJob(uuid.Must(uuid.NewV7())))
	require.Error(t, err)
	var cancel *rivertype.JobCancelError
	require.ErrorAs(t, err, &cancel)
}

func TestDeliverPushWorkerRetriesListFailure(t *testing.T) {
	t.Parallel()

	n := feedNotification(t)
	w := newPushWorker(t, &feedStub{notification: n}, &senderStub{}, &pushSubsStub{listErr: errors.New("db unavailable")})

	err := w.Work(context.Background(), pushJob(n.ID))
	require.ErrorContains(t, err, "list push subscriptions")
}
