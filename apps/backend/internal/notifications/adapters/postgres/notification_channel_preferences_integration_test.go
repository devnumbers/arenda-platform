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
	repo := NewPreferenceRepository(pool)

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
	repo := NewPreferenceRepository(pool)

	// Disable email for subscription_grace.
	if err := repo.UpsertChannelPreference(ctx, userID, domain.NotificationChannelPreference{
		EventType: domain.EventSubscriptionGrace,
		Channel:   domain.ChannelEmail,
		Allowed:   false,
	}); err != nil {
		t.Fatalf("upsert subscription_grace/email: %v", err)
	}

	prefs, err := repo.ListChannelPreferences(ctx, userID)
	if err != nil {
		t.Fatalf("list channel preferences: %v", err)
	}
	if len(prefs) != 1 {
		t.Fatalf("expected 1 stored row, got %d", len(prefs))
	}

	graceEmailAllowed, err := repo.IsChannelAllowed(ctx, userID, domain.EventSubscriptionGrace, domain.ChannelEmail)
	if err != nil {
		t.Fatalf("IsChannelAllowed subscription_grace/email: %v", err)
	}
	if graceEmailAllowed {
		t.Errorf("expected subscription_grace/email disabled, got allowed")
	}

	// Untouched pairs remain allowed.
	gracePushAllowed, err := repo.IsChannelAllowed(ctx, userID, domain.EventSubscriptionGrace, domain.ChannelPush)
	if err != nil {
		t.Fatalf("IsChannelAllowed subscription_grace/push: %v", err)
	}
	if !gracePushAllowed {
		t.Errorf("expected subscription_grace/push still allowed, got disabled")
	}

	// Re-upsert updates the value (idempotent write path).
	if err := repo.UpsertChannelPreference(ctx, userID, domain.NotificationChannelPreference{
		EventType: domain.EventSubscriptionGrace,
		Channel:   domain.ChannelEmail,
		Allowed:   true,
	}); err != nil {
		t.Fatalf("re-upsert subscription_grace/email: %v", err)
	}
	graceEmailAllowed, err = repo.IsChannelAllowed(ctx, userID, domain.EventSubscriptionGrace, domain.ChannelEmail)
	if err != nil {
		t.Fatalf("IsChannelAllowed subscription_grace/email after re-upsert: %v", err)
	}
	if !graceEmailAllowed {
		t.Errorf("expected subscription_grace/email re-enabled, got disabled")
	}
}
