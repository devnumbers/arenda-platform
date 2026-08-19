//go:build integration

package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// preDropFreeRemindersVersion is the last schema version before 000108: the
// free-reminders table, column and constraint branch still exist there, so
// the pre-removal data the drop contract must erase can be planted.
const preDropFreeRemindersVersion = 107

// TestFreeRemindersDropContract verifies migration 000108 — the schema
// contract of the FreeReminder removal (spec #380, decisions #268/#277).
// Like the cycle test it always starts its own PostgreSQL container: it
// plants pre-removal data at version 107 and then steps 108 up and down, so
// the shared testdb fixture (already at the latest version, where planting
// is structurally impossible) cannot serve it.
func TestFreeRemindersDropContract(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available — the drop contract test always needs its own container")
	}

	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("arenda"),
		postgres.WithUsername("arenda"),
		postgres.WithPassword("arenda"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("container connection string: %v", err)
	}

	dir := migrationsDir(t)

	// Pre-removal schema state: everything the migration must erase exists.
	migrateToVersion(t, url, dir, preDropFreeRemindersVersion)

	pool, err := NewPool(ctx, url)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	f := seedFreeRemindersFixture(t, pool)

	// Up: 000108 erases the free-reminders data and drops the schema.
	if err := MigrateUp(url, dir); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	assertFreeRemindersDropped(t, pool, f)

	// Down: the paired migration recreates the structure — never the data.
	migrateToVersion(t, url, dir, preDropFreeRemindersVersion)
	replanted := assertFreeRemindersStructureRestored(t, pool, f)

	// The second up must also work on a database that went through down and
	// carries the free rows planted by the down assertions.
	if err := MigrateUp(url, dir); err != nil {
		t.Fatalf("migrate up again: %v", err)
	}
	assertSchemaAtLatestVersion(t, url, dir)
	assertFreeRemindersReErased(t, pool, replanted)
}

// migrateToVersion applies migrations up or down until the schema sits at
// the given version (golang-migrate Migrate semantics).
func migrateToVersion(t *testing.T, databaseURL, dir string, version int) {
	t.Helper()

	m, err := newMigrate(databaseURL, dir)
	if err != nil {
		t.Fatalf("create migrate: %v", err)
	}
	defer func() {
		// Close returns two halves (source, database); Join drops nils.
		if err := errors.Join(m.Close()); err != nil {
			t.Errorf("close migrate: %v", err)
		}
	}()
	if err := m.Migrate(uint(version)); err != nil {
		t.Fatalf("migrate to version %d: %v", version, err)
	}
}

// freeRemindersFixture holds the ids of the pre-removal rows planted at
// version 107: a free template with its concrete reminder and delivery
// rows, free channel settings, and a control operation reminder proving the
// deletes touch only the free rows.
type freeRemindersFixture struct {
	ownerID           uuid.UUID
	propertyID        uuid.UUID
	templateID        uuid.UUID
	freeReminderID    uuid.UUID
	controlReminderID uuid.UUID
}

// countRows runs a scalar COUNT query and returns the number.
func countRows(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()

	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", sql, err)
	}
	return n
}

// tableExists reports whether the given table is present.
func tableExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()

	var exists bool
	if err := pool.QueryRow(context.Background(),
		`SELECT to_regclass($1) IS NOT NULL`, "public."+name).Scan(&exists); err != nil {
		t.Fatalf("to_regclass(%q): %v", name, err)
	}
	return exists
}

// constraintDef returns pg_get_constraintdef of exactly_one_target.
func constraintDef(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()

	var def string
	constraintDefQuery := `SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conrelid = 'reminders'::regclass AND conname = 'exactly_one_target'`
	if err := pool.QueryRow(context.Background(), constraintDefQuery).Scan(&def); err != nil {
		t.Fatalf("read exactly_one_target definition: %v", err)
	}
	return def
}

func seedFreeRemindersFixture(t *testing.T, pool *pgxpool.Pool) freeRemindersFixture {
	t.Helper()

	ctx := context.Background()
	now := time.Now().UTC()

	newID := func() uuid.UUID {
		id, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("new uuid: %v", err)
		}
		return id
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed %q: %v", sql, err)
		}
	}

	f := freeRemindersFixture{
		ownerID:           newID(),
		propertyID:        newID(),
		templateID:        newID(),
		freeReminderID:    newID(),
		controlReminderID: newID(),
	}
	categoryID := newID()
	operationID := newID()

	exec(`INSERT INTO users (id, phone, role) VALUES ($1, $2, 'owner')`,
		f.ownerID, "+79990000001")
	exec(`INSERT INTO properties (id, owner_id, name, type, address, status)
	      VALUES ($1, $2, 'Drop contract', 'apartment', '', 'active')`,
		f.propertyID, f.ownerID)

	// The control target and reminder: an operation reminder must survive
	// every delete of the drop migration.
	exec(`INSERT INTO operation_categories (id, owner_id, type, name)
	      VALUES ($1, $2, 'expense', 'drop contract')`,
		categoryID, f.ownerID)
	exec(`INSERT INTO operations (id, owner_id, property_id, type, category_id, name,
	      amount_kopecks, operation_date, source_operation_date, status)
	      VALUES ($1, $2, $3, 'expense', $4, 'Utilities', 1000, $5, $5, 'pending')`,
		operationID, f.ownerID, f.propertyID, categoryID, now)
	exec(`INSERT INTO reminders (id, owner_id, target_type, operation_id, property_id,
	      event_type, status, scheduled_at, message_title, message_body)
	      VALUES ($1, $2, 'operation', $3, $4, 'operation_due', 'pending', $5, 'Utilities', 'Pay utilities')`,
		f.controlReminderID, f.ownerID, operationID, f.propertyID, now)

	// The free-reminders aggregate the migration must erase: template,
	// concrete row, email/push delivery rows and channel settings.
	exec(`INSERT INTO free_reminders (id, owner_id, property_id, title, trigger_at, periodicity)
	      VALUES ($1, $2, $3, 'Insurance', $4, 'once')`,
		f.templateID, f.ownerID, f.propertyID, now)
	exec(`INSERT INTO reminders (id, owner_id, target_type, property_id, free_reminder_id,
	      event_type, status, scheduled_at, message_title, message_body)
	      VALUES ($1, $2, 'free', $3, $4, 'free_reminder', 'pending', $5, 'Insurance', 'Renew insurance')`,
		f.freeReminderID, f.ownerID, f.propertyID, f.templateID, now)
	exec(`INSERT INTO sent_email_reminders (id, reminder_id, owner_id, email, subject, plain_body, sent_at)
	      VALUES ($1, $2, $3, 'owner@example.com', 'Insurance', 'Renew insurance', $4)`,
		newID(), f.freeReminderID, f.ownerID, now)
	exec(`INSERT INTO sent_push_reminders (id, reminder_id, recipient_id)
	      VALUES ($1, $2, $3)`,
		newID(), f.freeReminderID, f.ownerID)
	exec(`INSERT INTO user_notification_channel_preferences (user_id, event_type, channel, allowed)
	      VALUES ($1, 'free_reminder', 'email', true), ($1, 'free_reminder', 'push', false),
	             ($1, 'operation_due', 'email', true)`,
		f.ownerID)

	return f
}

// assertFreeRemindersDropped checks every effect of the up migration: the
// channel settings, concrete rows and delivery rows are gone (neighbour
// rows survive), the table/column/index are dropped, the CHECK is rebuilt
// without the 'free' branch and rejects the dead target type, while the
// enum values stay in the PostgreSQL types forever (#277).
func assertFreeRemindersDropped(t *testing.T, pool *pgxpool.Pool, f freeRemindersFixture) {
	t.Helper()
	ctx := context.Background()

	if got := countRows(t, pool, `SELECT count(*) FROM user_notification_channel_preferences
	                             WHERE user_id = $1 AND event_type = 'free_reminder'`, f.ownerID); got != 0 {
		t.Errorf("free_reminder channel settings after up = %d, want 0", got)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM user_notification_channel_preferences
	                             WHERE user_id = $1 AND event_type = 'operation_due'`, f.ownerID); got != 1 {
		t.Errorf("operation_due channel settings after up = %d, want 1 (neighbour rows must survive)", got)
	}

	if got := countRows(t, pool, `SELECT count(*) FROM reminders WHERE id = $1`, f.freeReminderID); got != 0 {
		t.Errorf("free concrete row after up = %d, want 0", got)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM reminders WHERE id = $1`, f.controlReminderID); got != 1 {
		t.Errorf("control operation reminder after up = %d, want 1", got)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM sent_email_reminders WHERE reminder_id = $1`, f.freeReminderID); got != 0 {
		t.Errorf("email delivery rows after up = %d, want 0 (cascade)", got)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM sent_push_reminders WHERE reminder_id = $1`, f.freeReminderID); got != 0 {
		t.Errorf("push delivery rows after up = %d, want 0 (cascade)", got)
	}

	if tableExists(t, pool, "free_reminders") {
		t.Errorf("free_reminders table after up still exists")
	}
	if got := countRows(t, pool, `SELECT count(*) FROM information_schema.columns
	                             WHERE table_name = 'reminders' AND column_name = 'free_reminder_id'`); got != 0 {
		t.Errorf("reminders.free_reminder_id after up still exists")
	}
	if got := countRows(t, pool, `SELECT count(*) FROM pg_indexes
	                             WHERE tablename = 'reminders' AND indexname = 'idx_reminders_free_reminder'`); got != 0 {
		t.Errorf("idx_reminders_free_reminder after up still exists")
	}

	if def := constraintDef(t, pool); strings.Contains(def, `'free'`) {
		t.Errorf("exactly_one_target after up still has the 'free' branch: %s", def)
	}

	// The rebuilt CHECK rejects the dead target type (the enum value itself
	// stays in the type — asserted below), so a free row can never return.
	freeID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO reminders (id, owner_id, target_type, event_type, status,
	                         scheduled_at, message_title, message_body)
	                         VALUES ($1, $2, 'free', 'free_reminder', 'pending', $3, 'x', 'x')`,
		freeID, f.ownerID, time.Now().UTC())
	if err == nil {
		t.Errorf("insert target_type='free' after up succeeded, want exactly_one_target violation")
	} else if !strings.Contains(err.Error(), "exactly_one_target") {
		t.Errorf("insert target_type='free' after up: want exactly_one_target violation, got %v", err)
	}

	for _, v := range []struct{ typ, label string }{
		{"notification_target_type", "free"},
		{"notification_event_type", "free_reminder"},
	} {
		if got := countRows(t, pool, `SELECT count(*) FROM pg_enum e JOIN pg_type t ON t.oid = e.enumtypid
		                             WHERE t.typname = $1 AND e.enumlabel = $2`, v.typ, v.label); got != 1 {
			t.Errorf("enum value %s.%s after up = %d, want 1 (values stay forever, #277)", v.typ, v.label, got)
		}
	}
}

// assertFreeRemindersStructureRestored checks the down migration: the
// table, column, index and the 'free' branch of the constraint are back and
// accept free rows again — but the data erased by the up migration is gone
// for good (#268: no data restoration, only the pre-deploy dump). It
// returns the ids of a freshly planted template/reminder pair so the caller
// can prove the next up erases them once more.
func assertFreeRemindersStructureRestored(t *testing.T, pool *pgxpool.Pool, f freeRemindersFixture) freeRemindersFixture {
	t.Helper()
	ctx := context.Background()

	if !tableExists(t, pool, "free_reminders") {
		t.Fatalf("free_reminders table after down missing")
	}
	if got := countRows(t, pool, `SELECT count(*) FROM information_schema.columns
	                             WHERE table_name = 'reminders' AND column_name = 'free_reminder_id'`); got != 1 {
		t.Fatalf("reminders.free_reminder_id after down missing")
	}
	if got := countRows(t, pool, `SELECT count(*) FROM pg_indexes
	                             WHERE tablename = 'reminders' AND indexname = 'idx_reminders_free_reminder'`); got != 1 {
		t.Fatalf("idx_reminders_free_reminder after down missing")
	}

	if def := constraintDef(t, pool); !strings.Contains(def, `'free'`) {
		t.Errorf("exactly_one_target after down lacks the 'free' branch: %s", def)
	}

	if got := countRows(t, pool, `SELECT count(*) FROM free_reminders WHERE id = $1`, f.templateID); got != 0 {
		t.Errorf("seeded template after down = %d, want 0 (data is not restored)", got)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM reminders WHERE id = $1`, f.freeReminderID); got != 0 {
		t.Errorf("seeded free row after down = %d, want 0 (data is not restored)", got)
	}
	if got := countRows(t, pool, `SELECT count(*) FROM user_notification_channel_preferences
	                             WHERE user_id = $1 AND event_type = 'free_reminder'`, f.ownerID); got != 0 {
		t.Errorf("free_reminder channel settings after down = %d, want 0 (data is not restored)", got)
	}

	// Behavioral: the recreated structure accepts free rows again; the ids
	// flow back to the caller for the re-up erasure check.
	templateID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO free_reminders (id, owner_id, property_id, title, trigger_at, periodicity)
	                             VALUES ($1, $2, $3, 'Post-down', $4, 'once')`,
		templateID, f.ownerID, f.propertyID, time.Now().UTC()); err != nil {
		t.Fatalf("insert free template after down: %v", err)
	}
	reminderID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO reminders (id, owner_id, target_type, property_id, free_reminder_id,
	                             event_type, status, scheduled_at, message_title, message_body)
	                             VALUES ($1, $2, 'free', $3, $4, 'free_reminder', 'pending', $5, 'Post-down', 'Post-down body')`,
		reminderID, f.ownerID, f.propertyID, templateID, time.Now().UTC()); err != nil {
		t.Fatalf("insert free reminder after down: %v", err)
	}
	return freeRemindersFixture{
		ownerID:        f.ownerID,
		templateID:     templateID,
		freeReminderID: reminderID,
	}
}

// assertFreeRemindersReErased proves the second up erases the free rows the
// down assertions planted after recreating the structure.
func assertFreeRemindersReErased(t *testing.T, pool *pgxpool.Pool, replanted freeRemindersFixture) {
	t.Helper()

	if got := countRows(t, pool, `SELECT count(*) FROM reminders WHERE id = $1`, replanted.freeReminderID); got != 0 {
		t.Errorf("free row planted after down survived the second up = %d, want 0", got)
	}
	if tableExists(t, pool, "free_reminders") {
		t.Errorf("free_reminders table after the second up still exists")
	}
}
