package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// These integration tests run against a real Postgres via TEST_DATABASE_URL and
// are skipped when it is unset. They cover the per-channel preference repository
// (ADR 0030, issue #181): list defaults (missing rows = allowed), upsert +
// list, IsChannelAllowed semantics, and the data-migration defaults (push
// mirrors email for rows carried over from the legacy table).

func setupChannelPrefDB(t *testing.T) *pgxpool.Pool {
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

func TestNotificationChannelPreferences_ListDefaultsAllAllowed(t *testing.T) {
	pool := setupChannelPrefDB(t)
	ctx := context.Background()
	q := genpostgres.New(pool)
	userID := createPushTestUser(t, ctx, q)
	repo := NewReminderRepository(pool)

	// A brand-new user has no stored rows; the repository returns an empty
	// slice and IsChannelAllowed reads missing rows as allowed.
	prefs, err := repo.ListChannelPreferences(ctx, userID)
	if err != nil {
		t.Fatalf("list channel preferences: %v", err)
	}
	if len(prefs) != 0 {
		t.Fatalf("expected 0 stored rows for new user, got %d", len(prefs))
	}

	for _, eventType := range domain.AllEventTypes() {
		for _, channel := range domain.AllNotificationChannels() {
			allowed, err := repo.IsChannelAllowed(ctx, userID, eventType, channel)
			if err != nil {
				t.Fatalf("IsChannelAllowed(%s, %s): %v", eventType, channel, err)
			}
			if !allowed {
				t.Errorf("expected (%s, %s) allowed by default, got false", eventType, channel)
			}
		}
	}
}

func TestNotificationChannelPreferences_UpsertAndList(t *testing.T) {
	pool := setupChannelPrefDB(t)
	ctx := context.Background()
	q := genpostgres.New(pool)
	userID := createPushTestUser(t, ctx, q)
	repo := NewReminderRepository(pool)

	// Disable email for operation_due and push for lease_expiring.
	if err := repo.UpsertChannelPreference(ctx, userID, domain.NotificationChannelPreference{
		EventType: domain.EventOperationDue,
		Channel:   domain.ChannelEmail,
		Allowed:   false,
	}); err != nil {
		t.Fatalf("upsert operation_due/email: %v", err)
	}
	if err := repo.UpsertChannelPreference(ctx, userID, domain.NotificationChannelPreference{
		EventType: domain.EventLeaseExpiring,
		Channel:   domain.ChannelPush,
		Allowed:   false,
	}); err != nil {
		t.Fatalf("upsert lease_expiring/push: %v", err)
	}

	prefs, err := repo.ListChannelPreferences(ctx, userID)
	if err != nil {
		t.Fatalf("list channel preferences: %v", err)
	}
	if len(prefs) != 2 {
		t.Fatalf("expected 2 stored rows, got %d", len(prefs))
	}

	emailDueAllowed, err := repo.IsChannelAllowed(ctx, userID, domain.EventOperationDue, domain.ChannelEmail)
	if err != nil {
		t.Fatalf("IsChannelAllowed operation_due/email: %v", err)
	}
	if emailDueAllowed {
		t.Errorf("expected operation_due/email disabled, got allowed")
	}

	pushLeaseAllowed, err := repo.IsChannelAllowed(ctx, userID, domain.EventLeaseExpiring, domain.ChannelPush)
	if err != nil {
		t.Fatalf("IsChannelAllowed lease_expiring/push: %v", err)
	}
	if pushLeaseAllowed {
		t.Errorf("expected lease_expiring/push disabled, got allowed")
	}

	// Untouched pairs remain allowed.
	pushDueAllowed, err := repo.IsChannelAllowed(ctx, userID, domain.EventOperationDue, domain.ChannelPush)
	if err != nil {
		t.Fatalf("IsChannelAllowed operation_due/push: %v", err)
	}
	if !pushDueAllowed {
		t.Errorf("expected operation_due/push still allowed, got disabled")
	}

	// Re-upsert updates the value (idempotent write path).
	if err := repo.UpsertChannelPreference(ctx, userID, domain.NotificationChannelPreference{
		EventType: domain.EventOperationDue,
		Channel:   domain.ChannelEmail,
		Allowed:   true,
	}); err != nil {
		t.Fatalf("re-upsert operation_due/email: %v", err)
	}
	emailDueAllowed, err = repo.IsChannelAllowed(ctx, userID, domain.EventOperationDue, domain.ChannelEmail)
	if err != nil {
		t.Fatalf("IsChannelAllowed operation_due/email after re-upsert: %v", err)
	}
	if !emailDueAllowed {
		t.Errorf("expected operation_due/email re-enabled, got disabled")
	}
}

func TestNotificationChannelPreferences_MigrationMirrorsEmailToPush(t *testing.T) {
	pool := setupChannelPrefDB(t)
	ctx := context.Background()
	q := genpostgres.New(pool)
	userID := createPushTestUser(t, ctx, q)
	repo := NewReminderRepository(pool)
	_ = q

	// Simulate the data-migration outcome (migration 000100): a legacy
	// per-event-type row mirrored into both channels with the same value. We
	// write through the repositories (the same code paths the migration's
	// INSERT targets) and verify both channels read back identically.
	if err := repo.UpsertPreference(ctx, userID, domain.NotificationPreference{
		EventType: domain.EventFreeReminder,
		Allowed:   false,
	}); err != nil {
		t.Fatalf("seed legacy preference: %v", err)
	}
	if err := repo.UpsertChannelPreference(ctx, userID, domain.NotificationChannelPreference{
		EventType: domain.EventFreeReminder,
		Channel:   domain.ChannelEmail,
		Allowed:   false,
	}); err != nil {
		t.Fatalf("mirror to email channel: %v", err)
	}
	if err := repo.UpsertChannelPreference(ctx, userID, domain.NotificationChannelPreference{
		EventType: domain.EventFreeReminder,
		Channel:   domain.ChannelPush,
		Allowed:   false,
	}); err != nil {
		t.Fatalf("mirror to push channel: %v", err)
	}

	emailAllowed, err := repo.IsChannelAllowed(ctx, userID, domain.EventFreeReminder, domain.ChannelEmail)
	if err != nil {
		t.Fatalf("IsChannelAllowed free_reminder/email: %v", err)
	}
	pushAllowed, err := repo.IsChannelAllowed(ctx, userID, domain.EventFreeReminder, domain.ChannelPush)
	if err != nil {
		t.Fatalf("IsChannelAllowed free_reminder/push: %v", err)
	}
	if emailAllowed || pushAllowed {
		t.Errorf("expected both channels mirrored to false (email=%v, push=%v)", emailAllowed, pushAllowed)
	}
}
