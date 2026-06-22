package config

import (
	"strings"
	"testing"
	"time"
)

func setRequiredLocalEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "local")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	t.Setenv("MIGRATIONS_DIR", "./migrations")
}

func TestTKassaTimeoutDefault(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.TKassaTimeout != 30*time.Second {
		t.Fatalf("expected default T_KASSA_TIMEOUT 30s, got %v", cfg.TKassaTimeout)
	}
}

func TestTKassaTimeoutEnv(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")
	t.Setenv("T_KASSA_TIMEOUT", "45s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.TKassaTimeout != 45*time.Second {
		t.Fatalf("expected T_KASSA_TIMEOUT 45s, got %v", cfg.TKassaTimeout)
	}
}

func TestTKassaTimeoutInvalid(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")
	t.Setenv("T_KASSA_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid T_KASSA_TIMEOUT")
	}
}

func TestTKassaTimeoutNonPositive(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")
	t.Setenv("T_KASSA_TIMEOUT", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for non-positive T_KASSA_TIMEOUT")
	}
}

func TestTKassaTerminalKeyRequired(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_PASSWORD", "pass")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing T_KASSA_TERMINAL_KEY")
	}
	if !strings.Contains(err.Error(), "T_KASSA_TERMINAL_KEY") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTKassaPasswordRequired(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing T_KASSA_PASSWORD")
	}
	if !strings.Contains(err.Error(), "T_KASSA_PASSWORD") {
		t.Fatalf("unexpected error: %v", err)
	}
}
