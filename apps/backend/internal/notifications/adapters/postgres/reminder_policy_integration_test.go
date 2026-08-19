package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// These integration tests run against a real Postgres via TEST_DATABASE_URL
// and are skipped when it is unset (same convention as the other policy
// integration tests). They exercise the T3 policy enforcement (issue #166) in
// the reminder write use cases Reschedule and Cancel end-to-end: real
// MembershipPolicy over the real repository, with the service called as
// owner, full-access member, viewer, outsider and suspended member.

// policyTestName is the shared name/title fixture of the seeded rows.
const policyTestName = "policy test"

// reminderPolicyFixture wires the reminder service with the real repository
// bound to the test transaction and the real membership policy.
type reminderPolicyFixture struct {
	q       *genpostgres.Queries
	tx      pgx.Tx
	policy  *accessapp.MembershipPolicy
	members *accesspg.MembershipRepository
	repo    *ReminderRepository
	clock   policyTestClock
}

func newReminderPolicyFixture(tx pgx.Tx) *reminderPolicyFixture {
	return &reminderPolicyFixture{
		q:       genpostgres.New(tx),
		tx:      tx,
		policy:  accessapp.NewMembershipPolicy(accesspg.NewOwnerResolver(tx), accesspg.NewMembershipRepository(tx)),
		members: accesspg.NewMembershipRepository(tx),
		repo:    NewReminderRepository(tx),
		clock:   policyTestClock{now: time.Now()},
	}
}

func (f *reminderPolicyFixture) service() *application.ReminderService {
	return application.NewReminderService(f.repo, f.clock, policyTestTzResolver{}, f.policy)
}

// serviceWithShared mirrors the production wiring where the SharedProperties
// adapter is injected into ReminderService, so the aggregate list folds in the
// actor's shared properties (issue #157).
func (f *reminderPolicyFixture) serviceWithShared() *application.ReminderService {
	svc := application.NewReminderService(f.repo, f.clock, policyTestTzResolver{}, f.policy)
	svc.SetSharedPropertyIDs(accesspg.NewSharedProperties(f.tx))
	return svc
}

// seedReminder inserts a pending operation reminder on the owner's scope
// directly through the generated queries — the lightest seeding path, as the
// write gate under test does not depend on how the row was created. The
// exactly_one_target constraint requires a target row, so a one-off operation
// (with its category) is created first.
func (f *reminderPolicyFixture) seedReminder(t *testing.T, ctx context.Context, owner, property uuid.UUID) domain.Reminder {
	t.Helper()
	categoryID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := f.q.CreateOperationCategory(ctx, genpostgres.CreateOperationCategoryParams{
		ID: pgUUID(categoryID), OwnerID: pgUUID(owner),
		Type: "expense", Name: policyTestName,
	}); err != nil {
		t.Fatalf("seed operation category: %v", err)
	}

	operationID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := f.q.CreateOperation(ctx, genpostgres.CreateOperationParams{
		ID: pgUUID(operationID), OwnerID: pgUUID(owner),
		PropertyID: pgUUID(property), Type: "expense",
		CategoryID:          pgUUID(categoryID),
		Name:                policyTestName,
		AmountKopecks:       1000,
		OperationDate:       pgtype.Date{Time: f.clock.now.Add(72 * time.Hour), Valid: true},
		SourceOperationDate: pgtype.Date{Time: f.clock.now.Add(72 * time.Hour), Valid: true},
		Status:              "pending",
	}); err != nil {
		t.Fatalf("seed operation: %v", err)
	}

	reminderID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	rm := domain.Reminder{
		ID:           reminderID,
		OwnerID:      owner,
		TargetType:   domain.TargetOperation,
		OperationID:  &operationID,
		PropertyID:   &property,
		EventType:    domain.EventOperationDue,
		Status:       domain.ReminderPending,
		ScheduledAt:  f.clock.now.Add(24 * time.Hour),
		MessageTitle: policyTestName,
		MessageBody:  policyTestName,
		CreatedAt:    f.clock.now,
		UpdatedAt:    f.clock.now,
	}
	if err := f.repo.Save(ctx, rm); err != nil {
		t.Fatalf("seed reminder: %v", err)
	}
	return rm
}

func TestReminderPolicyIntegration_MemberReschedulesAndCancels(t *testing.T) {
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newReminderPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	member := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	addPolicyMembership(t, ctx, f.members, property, member, owner, accessdomain.RoleFullAccess)

	seeded := f.seedReminder(t, ctx, owner, property)

	// The full-access member reschedules the owner's reminder: the row changes
	// on the owner's scope.
	newDate := f.clock.now.Add(72 * time.Hour)
	updated, err := f.service().Reschedule(ctx, member, seeded.ID, newDate)
	if err != nil {
		t.Fatalf("Reschedule as member: %v", err)
	}
	if updated.OwnerID != owner {
		t.Errorf("updated OwnerID: want owner %s, got %s", owner, updated.OwnerID)
	}
	row, err := f.q.GetReminderByIDUnscoped(ctx, pgUUID(seeded.ID))
	if err != nil {
		t.Fatalf("get reminder row: %v", err)
	}
	// The service resolves the owner's timezone as UTC (test resolver) and
	// schedules at the dispatch hour (10:00) of the requested date.
	d := newDate.In(time.UTC)
	wantScheduledAt := time.Date(d.Year(), d.Month(), d.Day(), 10, 0, 0, 0, time.UTC)
	if !row.ScheduledAt.Time.Equal(wantScheduledAt) {
		t.Errorf("scheduled_at: want %s, got %s", wantScheduledAt, row.ScheduledAt.Time)
	}
	if row.OwnerID.Bytes != owner {
		t.Errorf("reminder owner_id: want %s, got %s", owner, row.OwnerID.Bytes)
	}

	// The member cancels the reminder: the row flips to cancelled.
	if err := f.service().Cancel(ctx, member, seeded.ID); err != nil {
		t.Fatalf("Cancel as member: %v", err)
	}
	row, err = f.q.GetReminderByIDUnscoped(ctx, pgUUID(seeded.ID))
	if err != nil {
		t.Fatalf("get reminder row: %v", err)
	}
	if row.Status != genpostgres.NotificationStatus(domain.ReminderCancelled) {
		t.Errorf("status: want %q, got %q", domain.ReminderCancelled, row.Status)
	}
}

func TestReminderPolicyIntegration_ViewerCannotWrite(t *testing.T) {
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newReminderPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	viewer := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	addPolicyMembership(t, ctx, f.members, property, viewer, owner, accessdomain.RoleViewer)

	seeded := f.seedReminder(t, ctx, owner, property)

	if _, err := f.service().Reschedule(ctx, viewer, seeded.ID, f.clock.now.Add(72*time.Hour)); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("Reschedule as viewer: want ErrForbidden, got %v", err)
	}
	if err := f.service().Cancel(ctx, viewer, seeded.ID); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("Cancel as viewer: want ErrForbidden, got %v", err)
	}

	// The row is untouched.
	row, err := f.q.GetReminderByIDUnscoped(ctx, pgUUID(seeded.ID))
	if err != nil {
		t.Fatalf("get reminder row: %v", err)
	}
	if row.Status != genpostgres.NotificationStatus(domain.ReminderPending) {
		t.Errorf("status: want %q, got %q", domain.ReminderPending, row.Status)
	}
}

func TestReminderPolicyIntegration_NoneAndSuspendedGetNotFound(t *testing.T) {
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newReminderPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	outsider := createPolicyTestUser(t, ctx, f.q)
	suspended := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	addSuspendedPolicyMembership(t, ctx, f.members, property, suspended, owner, accessdomain.RoleFullAccess)

	seeded := f.seedReminder(t, ctx, owner, property)

	for name, actor := range map[string]uuid.UUID{"outsider": outsider, "suspended": suspended} {
		if _, err := f.service().Reschedule(ctx, actor, seeded.ID, f.clock.now.Add(72*time.Hour)); !errors.Is(err, application.ErrNotFound) {
			t.Errorf("Reschedule as %s: want ErrNotFound, got %v", name, err)
		}
		if err := f.service().Cancel(ctx, actor, seeded.ID); !errors.Is(err, application.ErrNotFound) {
			t.Errorf("Cancel as %s: want ErrNotFound, got %v", name, err)
		}
	}
}
