//go:build integration

package scheduler

import (
	"context"
	"fmt"
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
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
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

// seedOperationReminder creates the operation category and a one-off
// operation on the owner's property (the reminders CHECK exactly_one_target
// requires a target row), then saves a sending reminder attached to it. The
// fan-out/push tests dispatch that reminder.
func seedOperationReminder(t *testing.T, ctx context.Context, q *genpostgres.Queries, repo *notificationspg.ReminderRepository, ownerID, propertyID uuid.UUID, title, body string) domain.Reminder {
	t.Helper()

	categoryID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := q.CreateOperationCategory(ctx, genpostgres.CreateOperationCategoryParams{
		ID: pgtype.UUID{Bytes: categoryID, Valid: true}, OwnerID: pgtype.UUID{Bytes: ownerID, Valid: true},
		Type: "expense", Name: "utilities",
	}); err != nil {
		t.Fatalf("create operation category: %v", err)
	}
	operationID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := q.CreateOperation(ctx, genpostgres.CreateOperationParams{
		ID: pgtype.UUID{Bytes: operationID, Valid: true}, OwnerID: pgtype.UUID{Bytes: ownerID, Valid: true},
		PropertyID: pgtype.UUID{Bytes: propertyID, Valid: true}, Type: "expense",
		CategoryID: pgtype.UUID{Bytes: categoryID, Valid: true},
		Name:       "Utilities", AmountKopecks: 1000,
		OperationDate:       pgtype.Date{Time: workerTestNow, Valid: true},
		SourceOperationDate: pgtype.Date{Time: workerTestNow, Valid: true},
		Status:              "pending",
	}); err != nil {
		t.Fatalf("create operation: %v", err)
	}

	reminderID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	reminder := domain.Reminder{
		ID:           reminderID,
		OwnerID:      ownerID,
		TargetType:   domain.TargetOperation,
		OperationID:  &operationID,
		PropertyID:   &propertyID,
		EventType:    domain.EventOperationDue,
		Status:       domain.ReminderSending,
		ScheduledAt:  workerTestNow,
		MessageTitle: title,
		MessageBody:  body,
		CreatedAt:    workerTestNow,
	}
	if err := repo.Save(ctx, reminder); err != nil {
		t.Fatalf("save reminder: %v", err)
	}
	return reminder
}

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
	// The shared testdb harness (testcontainers PostgreSQL 18, or
	// TEST_DATABASE_URL when set) keeps these legacy per-transaction fixtures
	// running under the same CI integration job as the testdb-based tests.
	return testdb.Setup(t)
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
// sending operation reminder bound to their property.
type reminderFanoutFixture struct {
	worker   *ReminderWorker
	repo     *notificationspg.ReminderRepository
	notifier *fakeNotifier
	reminder domain.Reminder
	ownerID  uuid.UUID
	memberID uuid.UUID
}

// newReminderFanoutFixture creates the owner, the property, the member (with
// the given membership status) and a sending operation reminder, then builds a
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
	reminder := seedOperationReminder(t, ctx, q, repo, ownerID, propertyID, "title", "body")

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
		EventType: domain.EventOperationDue,
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

// The due selection carries the eternal filter against the dead 'free' target
// type (ticket #381, code contract of the FreeReminder removal): orphaned
// materialized free-reminder rows stay in the table until the drop migration,
// but the worker must never pick them up for dispatch again. The free row is
// planted with raw SQL to simulate a pre-removal orphan — no service, no
// repository — while a due operation reminder planted beside it proves the
// selection itself still works.
func TestReminderWorker_Integration_ListDueSkipsFreeTargetRows(t *testing.T) {
	pool := setupReminderWorkerDB(t)
	ctx, tx, cleanup := beginReminderWorkerTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	ownerID := createWorkerTestUser(t, ctx, q)
	propertyID := createWorkerTestProperty(t, ctx, q, ownerID)
	pgOwner := pgtype.UUID{Bytes: ownerID, Valid: true}
	pgProperty := pgtype.UUID{Bytes: propertyID, Valid: true}
	dueAt := workerTestNow.Add(-time.Hour)

	// Control: a due operation reminder created through the regular queries.
	categoryID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := q.CreateOperationCategory(ctx, genpostgres.CreateOperationCategoryParams{
		ID: pgtype.UUID{Bytes: categoryID, Valid: true}, OwnerID: pgOwner,
		Type: "expense", Name: "utilities",
	}); err != nil {
		t.Fatalf("create operation category: %v", err)
	}
	operationID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := q.CreateOperation(ctx, genpostgres.CreateOperationParams{
		ID: pgtype.UUID{Bytes: operationID, Valid: true}, OwnerID: pgOwner,
		PropertyID: pgProperty, Type: "expense",
		CategoryID: pgtype.UUID{Bytes: categoryID, Valid: true},
		Name:       "Utilities", AmountKopecks: 1000,
		OperationDate:       pgtype.Date{Time: workerTestNow, Valid: true},
		SourceOperationDate: pgtype.Date{Time: workerTestNow, Valid: true},
		Status:              "pending",
	}); err != nil {
		t.Fatalf("create operation: %v", err)
	}
	controlID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := q.CreateReminder(ctx, genpostgres.CreateReminderParams{
		ID: pgtype.UUID{Bytes: controlID, Valid: true}, OwnerID: pgOwner,
		TargetType:   genpostgres.NotificationTargetTypeOperation,
		OperationID:  pgtype.UUID{Bytes: operationID, Valid: true},
		PropertyID:   pgProperty,
		EventType:    genpostgres.NotificationEventTypeOperationDue,
		Status:       genpostgres.NotificationStatusPending,
		ScheduledAt:  pgtype.Timestamptz{Time: dueAt, Valid: true},
		MessageTitle: "Utilities due", MessageBody: "Pay utilities",
		CreatedAt: pgtype.Timestamptz{Time: dueAt, Valid: true},
	}); err != nil {
		t.Fatalf("create control reminder: %v", err)
	}

	// The orphan: a 'free' target row planted directly with SQL. The CHECK
	// exactly_one_target still accepts it, so only the template row is needed.
	templateID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO free_reminders (id, owner_id, property_id, title, trigger_at, periodicity, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, 'once', $5, $5)`,
		templateID, ownerID, propertyID, "Orphan", dueAt,
	); err != nil {
		t.Fatalf("insert free reminder template: %v", err)
	}
	freeID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO reminders (id, owner_id, target_type, property_id, free_reminder_id, event_type, status, scheduled_at, message_title, message_body, created_at)
		 VALUES ($1, $2, 'free', $3, $4, 'free_reminder', 'pending', $5, $6, $7, $5)`,
		freeID, ownerID, propertyID, templateID, dueAt, "Orphan", "Orphan body",
	); err != nil {
		t.Fatalf("insert free reminder row: %v", err)
	}

	repo := notificationspg.NewReminderRepository(tx)
	due, err := repo.ListDue(ctx, workerTestNow, 10)
	if err != nil {
		t.Fatalf("ListDue: %v", err)
	}

	if len(due) != 1 || due[0].ID != controlID {
		t.Fatalf("ListDue = %v reminders, want exactly the operation reminder %v", len(due), controlID)
	}
	// The free row must have been skipped by the filter, not by a broken seed:
	// it is still a pending row in the table.
	free, err := repo.GetByIDUnscoped(ctx, freeID)
	if err != nil {
		t.Fatalf("GetByIDUnscoped(free): %v", err)
	}
	if free.Status != domain.ReminderPending || string(free.TargetType) != "free" {
		t.Fatalf("planted row = (%s, %s), want (pending, free) — seed broken", free.TargetType, free.Status)
	}
}
