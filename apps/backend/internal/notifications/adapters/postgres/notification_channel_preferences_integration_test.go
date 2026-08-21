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
// list, and IsChannelAllowed semantics.

func setupChannelPrefDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	// Same minimal test pool as setupPolicyDB: these tests commit directly
	// against the shared test database, one statement at a time, so each
	// parallel fixture needs at most one connection.
	cfg := database.DefaultPoolConfig()
	cfg.MinConns = 0
	cfg.MaxConns = 2
	ctx := context.Background()
	pool, err := database.NewPoolWithConfig(ctx, databaseURL, cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestNotificationChannelPreferences_ListDefaultsAllAllowed(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
