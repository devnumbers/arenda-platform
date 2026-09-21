package config

import (
	"testing"
	"time"
)

// baseNotificationsQueueEnv pins the minimal environment every notifications
// queue config test loads under.
func baseNotificationsQueueEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "local")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	t.Setenv("MIGRATIONS_DIR", "./migrations")
	t.Setenv("APP_BASE_URL", "http://localhost:8080")
	t.Setenv("DADATA_API_KEY", "test-dadata-key")
}

func TestNotificationsQueueDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	t.Setenv("MIGRATIONS_DIR", "./migrations")
	t.Setenv("APP_BASE_URL", "http://localhost:8080")
	t.Setenv("DADATA_API_KEY", "test-dadata-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.NotificationsEmailMaxWorkers != 4 {
		t.Fatalf("expected default NotificationsEmailMaxWorkers 4, got %d", cfg.NotificationsEmailMaxWorkers)
	}
	if cfg.NotificationsPushMaxWorkers != 16 {
		t.Fatalf("expected default NotificationsPushMaxWorkers 16, got %d", cfg.NotificationsPushMaxWorkers)
	}
	if cfg.NotificationsEmailMaxAttempts != 8 {
		t.Fatalf("expected default NotificationsEmailMaxAttempts 8, got %d", cfg.NotificationsEmailMaxAttempts)
	}
	if cfg.NotificationsPushMaxAttempts != 8 {
		t.Fatalf("expected default NotificationsPushMaxAttempts 8, got %d", cfg.NotificationsPushMaxAttempts)
	}
	if cfg.NotificationsRiverSoftStopTimeout != 10*time.Second {
		t.Fatalf("expected default NotificationsRiverSoftStopTimeout 10s, got %s", cfg.NotificationsRiverSoftStopTimeout)
	}
	if cfg.NotificationsEmailProviderPerMinute != 60 {
		t.Fatalf("expected default NotificationsEmailProviderPerMinute 60, got %d", cfg.NotificationsEmailProviderPerMinute)
	}
}

func TestNotificationsQueueRejectsNonPositive(t *testing.T) {
	cases := []struct {
		env  string
		want string
	}{
		{"NOTIFICATIONS_EMAIL_MAX_WORKERS", "NOTIFICATIONS_EMAIL_MAX_WORKERS must be positive"},
		{"NOTIFICATIONS_PUSH_MAX_WORKERS", "NOTIFICATIONS_PUSH_MAX_WORKERS must be positive"},
		{"NOTIFICATIONS_EMAIL_MAX_ATTEMPTS", "NOTIFICATIONS_EMAIL_MAX_ATTEMPTS must be positive"},
		{"NOTIFICATIONS_PUSH_MAX_ATTEMPTS", "NOTIFICATIONS_PUSH_MAX_ATTEMPTS must be positive"},
		{"NOTIFICATIONS_EMAIL_PROVIDER_PER_MINUTE", "NOTIFICATIONS_EMAIL_PROVIDER_PER_MINUTE must be positive"},
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			baseNotificationsQueueEnv(t)
			t.Setenv(tc.env, "0")

			_, err := Load()
			if err == nil {
				t.Fatalf("Load with %s=0 must fail", tc.env)
			}
			if err.Error() != tc.want {
				t.Fatalf("expected error %q, got %q", tc.want, err.Error())
			}
		})
	}
}

func TestNotificationsQueueSoftStopOverride(t *testing.T) {
	baseNotificationsQueueEnv(t)
	t.Setenv("NOTIFICATIONS_RIVER_SOFT_STOP_TIMEOUT", "3s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.NotificationsRiverSoftStopTimeout != 3*time.Second {
		t.Fatalf("expected NotificationsRiverSoftStopTimeout 3s, got %s", cfg.NotificationsRiverSoftStopTimeout)
	}
}

func TestNotificationsQueueSoftStopRejectsNonPositive(t *testing.T) {
	baseNotificationsQueueEnv(t)
	t.Setenv("NOTIFICATIONS_RIVER_SOFT_STOP_TIMEOUT", "0")

	if _, err := Load(); err == nil || err.Error() != "NOTIFICATIONS_RIVER_SOFT_STOP_TIMEOUT must be positive" {
		t.Fatalf("expected positive-duration rejection, got %v", err)
	}
}
