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
	t.Setenv("APP_BASE_URL", "http://localhost:8080")
	t.Setenv("DADATA_API_KEY", "test-dadata-key")
	t.Setenv("EMAIL_SENDER", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_FROM", "test@example.com")
	t.Setenv("EMAIL_TEMPLATES_DIR", "/app/templates/email")
	t.Setenv("REGRU_S3_ENDPOINT", "https://s3.example.com")
	t.Setenv("REGRU_S3_REGION", "ru-1")
	t.Setenv("REGRU_S3_BUCKET", "test-bucket")
	t.Setenv("REGRU_S3_ACCESS_KEY", "access")
	t.Setenv("REGRU_S3_SECRET_KEY", "secret")
	t.Setenv("REGRU_S3_PUBLIC_BASE_URL", "https://cdn.example.com")
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
	if cfg.TKassaRetryMutations {
		t.Fatal("expected default T_KASSA_RETRY_MUTATIONS off — mutation 5xx retries stay" +
			" disabled until the stage smoke confirms Init idempotency by OrderId")
	}
}

func TestTKassaRetryMutationsEnv(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")
	t.Setenv("T_KASSA_RETRY_MUTATIONS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !cfg.TKassaRetryMutations {
		t.Fatal("expected T_KASSA_RETRY_MUTATIONS=true to enable mutation 5xx retries")
	}
}

func TestTKassaRetryMutationsInvalid(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")
	t.Setenv("T_KASSA_RETRY_MUTATIONS", "maybe")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid T_KASSA_RETRY_MUTATIONS")
	}
	if !strings.Contains(err.Error(), "T_KASSA_RETRY_MUTATIONS") {
		t.Fatalf("unexpected error: %v", err)
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

func TestTKassaAppBaseURLRequired(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")
	t.Setenv("APP_BASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing APP_BASE_URL")
	}
	if !strings.Contains(err.Error(), "APP_BASE_URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTKassaAppBaseURLRequiresHttpsOutsideLocalDev(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("APP_ENV", "staging")
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")
	t.Setenv("APP_BASE_URL", "http://example.com")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for http APP_BASE_URL in staging")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTKassaBaseURLEmptyAllowedInLocal(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.TKassaBaseURL != "" {
		t.Fatalf("expected empty T_KASSA_BASE_URL in local, got %q", cfg.TKassaBaseURL)
	}
}

func TestTKassaBaseURLRequiredOutsideLocalDev(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("APP_ENV", "staging")
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")
	t.Setenv("APP_BASE_URL", "https://example.com")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing T_KASSA_BASE_URL in staging")
	}
	if !strings.Contains(err.Error(), "T_KASSA_BASE_URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTKassaBaseURLAcceptsSandboxInProduction(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")
	t.Setenv("APP_BASE_URL", "https://example.com")
	t.Setenv("T_KASSA_BASE_URL", "https://rest-api-test.tinkoff.ru/v2/")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.TKassaBaseURL != "https://rest-api-test.tinkoff.ru/v2/" {
		t.Fatalf("expected sandbox T_KASSA_BASE_URL, got %q", cfg.TKassaBaseURL)
	}
}

func TestTKassaBaseURLRejectsUnknownHostInProduction(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")
	t.Setenv("APP_BASE_URL", "https://example.com")
	t.Setenv("T_KASSA_BASE_URL", "https://evil.example.com/v2/")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for unknown T_KASSA_BASE_URL host in production")
	}
	if !strings.Contains(err.Error(), "securepay.tinkoff.ru") && !strings.Contains(err.Error(), "rest-api-test.tinkoff.ru") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTKassaBaseURLAcceptsProductionURL(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")
	t.Setenv("APP_BASE_URL", "https://example.com")
	t.Setenv("T_KASSA_BASE_URL", "https://securepay.tinkoff.ru/v2/")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.TKassaBaseURL != "https://securepay.tinkoff.ru/v2/" {
		t.Fatalf("expected production T_KASSA_BASE_URL, got %q", cfg.TKassaBaseURL)
	}
}
