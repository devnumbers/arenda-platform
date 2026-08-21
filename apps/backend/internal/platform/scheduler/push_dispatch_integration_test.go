//go:build integration

package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	notificationspg "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
)

// These integration tests exercise the push-dispatch path of the ReminderWorker
// (issue #182). They reuse the DB and transaction helpers from the email fan-out
// tests and add a fakePushSender + real push_subscription repository so the
// worker's dispatchPushReminder runs end-to-end against a real database. The
// tests are skipped when TEST_DATABASE_URL is unset.

// pushSendCall records one invocation of PushSender.Send.
type pushSendCall struct {
	Subscription domain.PushSubscription
	Payload      application.PushPayload
}

// fakePushSender is a test double for application.PushSender. It records every
// call and returns the configured error (nil by default = success).
type fakePushSender struct {
	calls     []pushSendCall
	errByUser map[uuid.UUID]error // Per-recipient error override.
	err       error               // Default error (nil = success).
}

func (s *fakePushSender) Send(_ context.Context, sub domain.PushSubscription, payload application.PushPayload) error {
	s.calls = append(s.calls, pushSendCall{Subscription: sub, Payload: payload})
	if s.errByUser != nil {
		if e, ok := s.errByUser[sub.UserID]; ok {
			return e
		}
	}
	return s.err
}

// pushDispatchFixture bundles the real worker (with push enabled) and the fake
// push sender plus the fixture data needed by push-dispatch assertions.
type pushDispatchFixture struct {
	worker       *ReminderWorker
	repo         *notificationspg.ReminderRepository
	pushSubRepo  *notificationspg.PushSubscriptionRepository
	pushSender   *fakePushSender
	emailNotifer *fakeNotifier
	reminder     domain.Reminder
	ownerID      uuid.UUID
	memberID     uuid.UUID
	propertyID   uuid.UUID
}

// newPushDispatchFixture wires a real ReminderWorker with push enabled: a real
// ReminderRepository, a real PushSubscriptionRepository, a fake PushSender, and
// the email notifiers as fakes. It seeds an owner, a property, an active member
// and a sending operation reminder attached to the property.
func newPushDispatchFixture(t *testing.T, ctx context.Context, tx pgx.Tx) pushDispatchFixture {
	t.Helper()

	q := genpostgres.New(tx)
	ownerID := createWorkerTestUser(t, ctx, q)
	memberID := createWorkerTestUser(t, ctx, q)
	propertyID := createWorkerTestProperty(t, ctx, q, ownerID)

	memberRepo := accesspg.NewMembershipRepository(tx)
	membershipID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := memberRepo.CreateWithStatus(ctx, accessdomain.Membership{
		ID: membershipID, PropertyID: propertyID, UserID: memberID,
		Role: accessdomain.RoleViewer, GrantedBy: ownerID, Status: accessdomain.MemberStatusActive,
	}); err != nil {
		t.Fatalf("create membership: %v", err)
	}

	repo := notificationspg.NewReminderRepository(tx)
	pushSubRepo := notificationspg.NewPushSubscriptionRepository(tx)

	reminder := seedOperationReminder(t, ctx, q, repo, ownerID, propertyID, "Test reminder", "Body text")

	renderer, err := mailer.NewRenderer("../../../templates/email")
	if err != nil {
		t.Fatalf("load email templates: %v", err)
	}
	emailNotifier := &fakeNotifier{}
	pushSender := &fakePushSender{}

	worker := NewReminderWorker(
		repo,
		renderer,
		map[application.Channel]application.Notifier{
			application.ChannelEmail: emailNotifier,
		},
		fakeContactResolver{},
		accesspg.NewMemberRecipientAdapter(memberRepo),
		pushSender,
		pushSubRepo,
		beginnerOverTx{tx: tx},
		fakeClockForWorker{now: workerTestNow},
		fakeBackoff{},
		3,
		time.Hour,
		time.Minute,
		nil,
	)

	return pushDispatchFixture{
		worker:       worker,
		repo:         repo,
		pushSubRepo:  pushSubRepo,
		pushSender:   pushSender,
		emailNotifer: emailNotifier,
		reminder:     reminder,
		ownerID:      ownerID,
		memberID:     memberID,
		propertyID:   propertyID,
	}
}

// addPushSubscription seeds a push subscription for a user and returns it.
func (f *pushDispatchFixture) addPushSubscription(
	t *testing.T,
	ctx context.Context,
	userID uuid.UUID,
	endpoint string,
) domain.PushSubscription {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	sub := domain.PushSubscription{
		ID:        id,
		UserID:    userID,
		Endpoint:  endpoint,
		P256dh:    "p256dh-key",
		Auth:      "auth-secret",
		CreatedAt: workerTestNow,
		UpdatedAt: workerTestNow,
	}
	stored, err := f.pushSubRepo.Upsert(ctx, sub)
	if err != nil {
		t.Fatalf("upsert push subscription: %v", err)
	}
	return stored
}

func (f *pushDispatchFixture) dispatch(t *testing.T, ctx context.Context) {
	t.Helper()
	if err := f.worker.dispatchReminder(ctx, f.reminder, workerTestNow); err != nil {
		t.Fatalf("dispatchReminder: %v", err)
	}
}

func (f *pushDispatchFixture) disableEmail(t *testing.T, ctx context.Context, userID uuid.UUID) {
	t.Helper()
	if err := f.repo.UpsertChannelPreference(ctx, userID, domain.NotificationChannelPreference{
		EventType: domain.EventOperationDue,
		Channel:   domain.ChannelEmail,
		Allowed:   false,
	}); err != nil {
		t.Fatalf("disable email: %v", err)
	}
}

func (f *pushDispatchFixture) disablePush(t *testing.T, ctx context.Context, userID uuid.UUID) {
	t.Helper()
	if err := f.repo.UpsertChannelPreference(ctx, userID, domain.NotificationChannelPreference{
		EventType: domain.EventOperationDue,
		Channel:   domain.ChannelPush,
		Allowed:   false,
	}); err != nil {
		t.Fatalf("disable push: %v", err)
	}
}

// pushSentTo returns true when the fake push sender was called for the given
// recipient at least once.
func (f *pushDispatchFixture) pushSentTo(userID uuid.UUID) bool {
	for _, c := range f.pushSender.calls {
		if c.Subscription.UserID == userID {
			return true
		}
	}
	return false
}

// pushAuditExists checks the sent_push_reminders audit row for a recipient.
func (f *pushDispatchFixture) pushAuditExists(t *testing.T, ctx context.Context, recipientID uuid.UUID) bool {
	t.Helper()
	sent, err := f.repo.IsPushReminderSent(ctx, f.reminder.ID, recipientID)
	if err != nil {
		t.Fatalf("IsPushReminderSent: %v", err)
	}
	return sent
}

// subscriptionExists checks whether a push subscription is still stored.
func (f *pushDispatchFixture) subscriptionExists(t *testing.T, ctx context.Context, userID uuid.UUID) bool {
	t.Helper()
	subs, err := f.pushSubRepo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	return len(subs) > 0
}

// Due-reminder with push-allowed recipient and a subscription: Send is called
// with the correct payload, and the audit row is created.
func TestPushDispatch_Integration_DeliversToRecipientWithSubscription(t *testing.T) {
	t.Parallel()

	pool := setupReminderWorkerDB(t)
	ctx, tx, cleanup := beginReminderWorkerTx(t, pool)
	defer cleanup()

	f := newPushDispatchFixture(t, ctx, tx)
	f.disableEmail(t, ctx, f.ownerID) // Owner uses push only.
	f.addPushSubscription(t, ctx, f.ownerID, "https://fcm.googleapis.com/fcm/send/owner-1")
	f.dispatch(t, ctx)

	if !f.pushSentTo(f.ownerID) {
		t.Errorf("expected push to be sent to owner")
	}
	if len(f.pushSender.calls) == 0 {
		t.Fatalf("expected at least one push Send call")
	}
	call := f.pushSender.calls[0]
	if call.Payload.Title != "Test reminder" {
		t.Errorf("payload title = %q, want %q", call.Payload.Title, "Test reminder")
	}
	if call.Payload.EventType != domain.EventOperationDue {
		t.Errorf("payload eventType = %q, want %q", call.Payload.EventType, domain.EventOperationDue)
	}
	if call.Payload.URL == "" {
		t.Errorf("payload url should not be empty")
	}
	if !f.pushAuditExists(t, ctx, f.ownerID) {
		t.Errorf("expected push audit row for owner")
	}
}

// A 404/410 (ErrSubscriptionGone) from the push service deletes the dead
// subscription so future dispatches do not waste attempts on it.
func TestPushDispatch_Integration_DeadSubscriptionIsDeleted(t *testing.T) {
	t.Parallel()

	pool := setupReminderWorkerDB(t)
	ctx, tx, cleanup := beginReminderWorkerTx(t, pool)
	defer cleanup()

	f := newPushDispatchFixture(t, ctx, tx)
	f.disableEmail(t, ctx, f.ownerID)
	f.addPushSubscription(t, ctx, f.ownerID, "https://fcm.googleapis.com/fcm/send/owner-gone")
	f.pushSender.err = application.ErrSubscriptionGone

	if !f.subscriptionExists(t, ctx, f.ownerID) {
		t.Fatalf("subscription should exist before dispatch")
	}
	f.dispatch(t, ctx)

	if f.subscriptionExists(t, ctx, f.ownerID) {
		t.Errorf("dead subscription should have been deleted")
	}
}

// A recipient who disabled push in their per-channel preferences does not
// receive a push notification.
func TestPushDispatch_Integration_PushDisabledNoSend(t *testing.T) {
	t.Parallel()

	pool := setupReminderWorkerDB(t)
	ctx, tx, cleanup := beginReminderWorkerTx(t, pool)
	defer cleanup()

	f := newPushDispatchFixture(t, ctx, tx)
	f.disableEmail(t, ctx, f.ownerID)
	f.disablePush(t, ctx, f.ownerID)
	f.addPushSubscription(t, ctx, f.ownerID, "https://fcm.googleapis.com/fcm/send/owner-no-push")
	f.dispatch(t, ctx)

	if f.pushSentTo(f.ownerID) {
		t.Errorf("push should not be sent when push channel is disabled")
	}
	if f.pushAuditExists(t, ctx, f.ownerID) {
		t.Errorf("push audit row should not exist when push channel is disabled")
	}
}

// Fan-out over shared property: owner and active member both receive a push if
// they each have a subscription and push allowed.
func TestPushDispatch_Integration_FanOutToOwnerAndMember(t *testing.T) {
	t.Parallel()

	pool := setupReminderWorkerDB(t)
	ctx, tx, cleanup := beginReminderWorkerTx(t, pool)
	defer cleanup()

	f := newPushDispatchFixture(t, ctx, tx)
	f.disableEmail(t, ctx, f.ownerID)
	f.disableEmail(t, ctx, f.memberID)
	f.addPushSubscription(t, ctx, f.ownerID, "https://fcm.googleapis.com/fcm/send/owner-2")
	f.addPushSubscription(t, ctx, f.memberID, "https://fcm.googleapis.com/fcm/send/member-2")
	f.dispatch(t, ctx)

	if !f.pushSentTo(f.ownerID) {
		t.Errorf("expected push to owner")
	}
	if !f.pushSentTo(f.memberID) {
		t.Errorf("expected push to member (shared property fan-out)")
	}
	if !f.pushAuditExists(t, ctx, f.ownerID) {
		t.Errorf("expected push audit row for owner")
	}
	if !f.pushAuditExists(t, ctx, f.memberID) {
		t.Errorf("expected push audit row for member")
	}
}

// Email and push dispatch independently: a recipient with both channels enabled
// receives both, and both audit rows are created.
func TestPushDispatch_Integration_EmailAndPushBothDelivered(t *testing.T) {
	t.Parallel()

	pool := setupReminderWorkerDB(t)
	ctx, tx, cleanup := beginReminderWorkerTx(t, pool)
	defer cleanup()

	f := newPushDispatchFixture(t, ctx, tx)
	// Email is allowed by default (opt-out model); push is allowed by default.
	f.addPushSubscription(t, ctx, f.ownerID, "https://fcm.googleapis.com/fcm/send/owner-both")
	f.dispatch(t, ctx)

	if !f.pushSentTo(f.ownerID) {
		t.Errorf("expected push to owner")
	}
	if len(f.emailNotifer.calls) == 0 {
		t.Errorf("expected email to owner")
	}
	if !f.pushAuditExists(t, ctx, f.ownerID) {
		t.Errorf("expected push audit row")
	}
	emailSent, err := f.repo.IsEmailReminderSent(ctx, f.reminder.ID, f.ownerID)
	if err != nil {
		t.Fatalf("IsEmailReminderSent: %v", err)
	}
	if !emailSent {
		t.Errorf("expected email audit row")
	}
}

// Deduplication: a second dispatch of the same reminder does not call Send
// again because the audit row already marks the recipient as notified.
func TestPushDispatch_Integration_DedupOnRedispatch(t *testing.T) {
	t.Parallel()

	pool := setupReminderWorkerDB(t)
	ctx, tx, cleanup := beginReminderWorkerTx(t, pool)
	defer cleanup()

	f := newPushDispatchFixture(t, ctx, tx)
	f.disableEmail(t, ctx, f.ownerID)
	f.addPushSubscription(t, ctx, f.ownerID, "https://fcm.googleapis.com/fcm/send/owner-dedup")
	f.dispatch(t, ctx)

	firstCalls := len(f.pushSender.calls)

	// Second dispatch: the reminder is already sending (the fake worker does not
	// finalize), but the push audit row should prevent a duplicate push.
	f.dispatch(t, ctx)

	if got := len(f.pushSender.calls); got != firstCalls {
		t.Errorf("push calls after redispatch = %d, want %d (dedup via audit)", got, firstCalls)
	}
}

// Compile-time check that *fakePushSender satisfies the PushSender port.
var _ application.PushSender = (*fakePushSender)(nil)
