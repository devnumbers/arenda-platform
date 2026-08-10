package scheduler

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	notificationspg "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// These integration tests run against a real Postgres via TEST_DATABASE_URL
// and are skipped when it is unset (same convention as the other integration
// tests). They exercise the reminder fan-out (issue #159) end-to-end: the
// real ReminderWorker over the real notifications and access repositories
// inside a rolled-back transaction. Only the Notifier and the ContactResolver
// are fakes — they are not the integration seam under test.

// noCommitTx wraps a real pgx transaction so the worker's finalize step does
// not commit it (the test rolls the outer transaction back in cleanup). The
// embedded pgx.Tx still satisfies postgres.DBTX, so real repositories can bind
// to it via WithTx.
type noCommitTx struct{ pgx.Tx }

func (noCommitTx) Commit(context.Context) error   { return nil }
func (noCommitTx) Rollback(context.Context) error { return nil }

// beginnerOverTx is a transaction.Beginner that always returns the test's
// already-open transaction wrapped in noCommitTx.
type beginnerOverTx struct{ tx pgx.Tx }

func (b beginnerOverTx) Begin(context.Context) (transaction.Tx, error) {
	return noCommitTx{b.tx}, nil
}

func setupReminderWorkerDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func beginReminderWorkerTx(t *testing.T, pool *pgxpool.Pool) (context.Context, pgx.Tx, func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	return ctx, tx, func() { _ = tx.Rollback(ctx) }
}

func createWorkerTestUser(t *testing.T, ctx context.Context, q *genpostgres.Queries) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	_, err = q.CreateUser(ctx, genpostgres.CreateUserParams{
		ID:    pgtype.UUID{Bytes: id, Valid: true},
		Phone: fmt.Sprintf("+7999%010d", id.Time()%1e10),
		Role:  "owner",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func createWorkerTestProperty(t *testing.T, ctx context.Context, q *genpostgres.Queries, owner uuid.UUID) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	_, err = q.CreateProperty(ctx, genpostgres.CreatePropertyParams{
		ID:          pgtype.UUID{Bytes: id, Valid: true},
		OwnerID:     pgtype.UUID{Bytes: owner, Valid: true},
		Name:        "Test Property",
		Type:        "apartment",
		Address:     "",
		Description: pgtype.Text{},
		Attributes:  []byte("{}"),
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("create property: %v", err)
	}
	return id
}

// reminderFanoutFixture bundles a real reminder worker wired to the test
// transaction together with the fixture ids of an owner, one member and a
// sending free reminder bound to their property.
type reminderFanoutFixture struct {
	worker   *ReminderWorker
	repo     *notificationspg.ReminderRepository
	notifier *fakeNotifier
	reminder domain.Reminder
	ownerID  uuid.UUID
	memberID uuid.UUID
}

// newReminderFanoutFixture creates the owner, the property, the member (with
// the given membership status) and a sending free reminder, then builds a
// real ReminderWorker over the real notifications and access repositories.
func newReminderFanoutFixture(t *testing.T, ctx context.Context, tx pgx.Tx, memberStatus accessdomain.MemberStatus) reminderFanoutFixture {
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
		Role: accessdomain.RoleViewer, GrantedBy: ownerID, Status: memberStatus,
	}); err != nil {
		t.Fatalf("create membership: %v", err)
	}

	repo := notificationspg.NewReminderRepository(tx)

	// A 'free' target requires a concrete reminder linked to its template
	// (CHECK exactly_one_target), so create the template first and save the
	// materialized reminder through SaveFreeReminder.
	freeRepo := notificationspg.NewFreeReminderRepository(tx)
	templateID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := freeRepo.Create(ctx, domain.FreeReminder{
		ID: templateID, OwnerID: ownerID, PropertyID: propertyID,
		Title: "title", TriggerAt: workerTestNow, Periodicity: domain.PeriodicityOnce,
		CreatedAt: workerTestNow, UpdatedAt: workerTestNow,
	}); err != nil {
		t.Fatalf("create free reminder template: %v", err)
	}

	reminderID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	reminder := domain.Reminder{
		ID:             reminderID,
		OwnerID:        ownerID,
		TargetType:     domain.TargetFree,
		PropertyID:     &propertyID,
		FreeReminderID: &templateID,
		EventType:      domain.EventFreeReminder,
		Status:         domain.ReminderSending,
		ScheduledAt:    workerTestNow,
		MessageTitle:   "title",
		MessageBody:    "body",
		CreatedAt:      workerTestNow,
	}
	if err := freeRepo.SaveFreeReminder(ctx, reminder); err != nil {
		t.Fatalf("save reminder: %v", err)
	}

	renderer, err := mailer.NewRenderer("../../../templates/email")
	if err != nil {
		t.Fatalf("load email templates: %v", err)
	}
	notifier := &fakeNotifier{}
	worker := NewReminderWorker(
		repo,
		renderer,
		map[application.Channel]application.Notifier{
			application.ChannelEmail: notifier,
		},
		fakeContactResolver{},
		accesspg.NewMemberRecipientAdapter(memberRepo),
		nil, // pushSender — disabled in email-only fan-out tests
		nil, // pushSubRepo
		nil, // pushMetrics
		beginnerOverTx{tx: tx},
		fakeClockForWorker{now: workerTestNow},
		fakeBackoff{},
		3,
		time.Hour,
		time.Minute,
		nil,
	)

	return reminderFanoutFixture{
		worker:   worker,
		repo:     repo,
		notifier: notifier,
		reminder: reminder,
		ownerID:  ownerID,
		memberID: memberID,
	}
}

func (f reminderFanoutFixture) optOut(t *testing.T, ctx context.Context, userID uuid.UUID) {
	t.Helper()
	// The worker checks per-channel preferences (ADR 0030) on the email channel,
	// so the opt-out must write to the channel preferences table.
	if err := f.repo.UpsertChannelPreference(ctx, userID, domain.NotificationChannelPreference{
		EventType: domain.EventFreeReminder,
		Channel:   domain.ChannelEmail,
		Allowed:   false,
	}); err != nil {
		t.Fatalf("upsert channel preference: %v", err)
	}
}

func (f reminderFanoutFixture) dispatch(t *testing.T, ctx context.Context) {
	t.Helper()
	if err := f.worker.dispatchReminder(ctx, f.reminder, workerTestNow); err != nil {
		t.Fatalf("dispatchReminder: %v", err)
	}
}

func (f reminderFanoutFixture) assertDelivered(t *testing.T, ctx context.Context, want []uuid.UUID) {
	t.Helper()
	if len(f.notifier.calls) != len(want) {
		t.Fatalf("notify calls = %v, want %v", f.notifier.calls, want)
	}
	for i, id := range want {
		if f.notifier.calls[i] != id {
			t.Errorf("notify call %d = %v, want %v (calls: %v)", i, f.notifier.calls[i], id, f.notifier.calls)
		}
	}
	// The per-recipient audit row is the source of truth for delivery and dedup.
	for _, id := range []uuid.UUID{f.ownerID, f.memberID} {
		sent, err := f.repo.IsEmailReminderSent(ctx, f.reminder.ID, id)
		if err != nil {
			t.Fatalf("IsEmailReminderSent(%v): %v", id, err)
		}
		wantSent := false
		for _, w := range want {
			if w == id {
				wantSent = true
			}
		}
		if sent != wantSent {
			t.Errorf("IsEmailReminderSent(%v) = %v, want %v", id, sent, wantSent)
		}
	}
	// The reminder is finalized as sent exactly once per dispatch.
	got, err := f.repo.GetByIDUnscoped(ctx, f.reminder.ID)
	if err != nil {
		t.Fatalf("GetByIDUnscoped: %v", err)
	}
	if got.Status != domain.ReminderSent {
		t.Errorf("reminder status = %v, want sent", got.Status)
	}
}

// Owner and active member without any preference rows both receive the
// property reminder (default is allowed in the opt-out model).
func TestReminderWorker_Integration_DeliversToOwnerAndActiveMember(t *testing.T) {
	pool := setupReminderWorkerDB(t)
	ctx, tx, cleanup := beginReminderWorkerTx(t, pool)
	defer cleanup()

	f := newReminderFanoutFixture(t, ctx, tx, accessdomain.MemberStatusActive)
	f.dispatch(t, ctx)
	f.assertDelivered(t, ctx, []uuid.UUID{f.ownerID, f.memberID})
}

// An active member who opted out of the event type does not receive the
// reminder; the owner still does.
func TestReminderWorker_Integration_MemberOptOutReceivesNothing(t *testing.T) {
	pool := setupReminderWorkerDB(t)
	ctx, tx, cleanup := beginReminderWorkerTx(t, pool)
	defer cleanup()

	f := newReminderFanoutFixture(t, ctx, tx, accessdomain.MemberStatusActive)
	f.optOut(t, ctx, f.memberID)
	f.dispatch(t, ctx)
	f.assertDelivered(t, ctx, []uuid.UUID{f.ownerID})
}

// The owner's opt-out does not affect the member: preferences are enforced
// per recipient.
func TestReminderWorker_Integration_OwnerOptOutMemberStillReceives(t *testing.T) {
	pool := setupReminderWorkerDB(t)
	ctx, tx, cleanup := beginReminderWorkerTx(t, pool)
	defer cleanup()

	f := newReminderFanoutFixture(t, ctx, tx, accessdomain.MemberStatusActive)
	f.optOut(t, ctx, f.ownerID)
	f.dispatch(t, ctx)
	f.assertDelivered(t, ctx, []uuid.UUID{f.memberID})
}

// A suspended member is excluded from the fan-out at the recipient source.
func TestReminderWorker_Integration_SuspendedMemberReceivesNothing(t *testing.T) {
	pool := setupReminderWorkerDB(t)
	ctx, tx, cleanup := beginReminderWorkerTx(t, pool)
	defer cleanup()

	f := newReminderFanoutFixture(t, ctx, tx, accessdomain.MemberStatusSuspended)
	f.dispatch(t, ctx)
	f.assertDelivered(t, ctx, []uuid.UUID{f.ownerID})
}
