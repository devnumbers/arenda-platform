package config

import "testing"

// TestRateLimitEnvRejectsNonPositive pins the validateRateLimit contract: a
// non-positive value of any rate limit fails config load with its own "must be
// positive" error instead of reaching the wire, where a zero sends-per-hour
// limit divides time.Hour by zero and panics on startup.
func TestRateLimitEnvRejectsNonPositive(t *testing.T) {
	cases := []struct {
		env  string
		want string
	}{
		{"RATE_LIMIT_IP_RPS", "RATE_LIMIT_IP_RPS must be positive"},
		{"RATE_LIMIT_IP_BURST", "RATE_LIMIT_IP_BURST must be positive"},
		{"RATE_LIMIT_EMAIL_SEND_PER_HOUR", "RATE_LIMIT_EMAIL_SEND_PER_HOUR must be positive"},
		{"RATE_LIMIT_EMAIL_VERIFY_PER_15MIN", "RATE_LIMIT_EMAIL_VERIFY_PER_15MIN must be positive"},
		{"RATE_LIMIT_PHONE_CHANGE_SEND_PER_HOUR", "RATE_LIMIT_PHONE_CHANGE_SEND_PER_HOUR must be positive"},
		{"RATE_LIMIT_PHONE_CHANGE_VERIFY_PER_15MIN", "RATE_LIMIT_PHONE_CHANGE_VERIFY_PER_15MIN must be positive"},
		{"RATE_LIMIT_EMAIL_CHANGE_SEND_PER_HOUR", "RATE_LIMIT_EMAIL_CHANGE_SEND_PER_HOUR must be positive"},
		{"RATE_LIMIT_CODE_SEND_PER_RECIPIENT_PER_HOUR", "RATE_LIMIT_CODE_SEND_PER_RECIPIENT_PER_HOUR must be positive"},
		{"RATE_LIMIT_CODE_SEND_PER_INITIATOR_PER_HOUR", "RATE_LIMIT_CODE_SEND_PER_INITIATOR_PER_HOUR must be positive"},
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv("APP_ENV", "local")
			t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
			t.Setenv("MIGRATIONS_DIR", "./migrations")
			t.Setenv("APP_BASE_URL", "http://localhost:8080")
			t.Setenv("DADATA_API_KEY", "test-dadata-key")
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

func TestRateLimitDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	t.Setenv("MIGRATIONS_DIR", "./migrations")
	t.Setenv("APP_BASE_URL", "http://localhost:8080")
	t.Setenv("DADATA_API_KEY", "test-dadata-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.RateLimit.EmailChangeSendPerHour != 5 {
		t.Fatalf("expected default EmailChangeSendPerHour 5, got %d", cfg.RateLimit.EmailChangeSendPerHour)
	}
	// The anti-flood send limits (#1210): 5 codes per hour to one recipient
	// address, 10 per hour per authenticated initiator.
	if cfg.RateLimit.CodeSendPerRecipientPerHour != 5 {
		t.Fatalf("expected default CodeSendPerRecipientPerHour 5, got %d", cfg.RateLimit.CodeSendPerRecipientPerHour)
	}
	if cfg.RateLimit.CodeSendPerInitiatorPerHour != 10 {
		t.Fatalf("expected default CodeSendPerInitiatorPerHour 10, got %d", cfg.RateLimit.CodeSendPerInitiatorPerHour)
	}
}

// TestRateLimitCodeSendOverrides applies the anti-flood env overrides (#1210),
// keeping the built-in defaults when a value arrives unset.
func TestRateLimitCodeSendOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	t.Setenv("MIGRATIONS_DIR", "./migrations")
	t.Setenv("APP_BASE_URL", "http://localhost:8080")
	t.Setenv("DADATA_API_KEY", "test-dadata-key")
	t.Setenv("RATE_LIMIT_CODE_SEND_PER_RECIPIENT_PER_HOUR", "50")
	t.Setenv("RATE_LIMIT_CODE_SEND_PER_INITIATOR_PER_HOUR", "100")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.RateLimit.CodeSendPerRecipientPerHour != 50 {
		t.Fatalf("expected CodeSendPerRecipientPerHour 50, got %d", cfg.RateLimit.CodeSendPerRecipientPerHour)
	}
	if cfg.RateLimit.CodeSendPerInitiatorPerHour != 100 {
		t.Fatalf("expected CodeSendPerInitiatorPerHour 100, got %d", cfg.RateLimit.CodeSendPerInitiatorPerHour)
	}
}
