package config

import (
	"testing"
	"time"
)

func TestTariffCacheTTLDefault(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	t.Setenv("MIGRATIONS_DIR", "./migrations")
	t.Setenv("APP_BASE_URL", "http://localhost:8080")
	t.Setenv("DADATA_API_KEY", "test-dadata-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.TariffCacheTTL != 5*time.Minute {
		t.Fatalf("expected default TARIFF_CACHE_TTL 5m, got %v", cfg.TariffCacheTTL)
	}
}

func TestTariffCacheTTLEnv(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	t.Setenv("MIGRATIONS_DIR", "./migrations")
	t.Setenv("APP_BASE_URL", "http://localhost:8080")
	t.Setenv("DADATA_API_KEY", "test-dadata-key")
	t.Setenv("TARIFF_CACHE_TTL", "10m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.TariffCacheTTL != 10*time.Minute {
		t.Fatalf("expected TARIFF_CACHE_TTL 10m, got %v", cfg.TariffCacheTTL)
	}
}

func TestTariffCacheTTLInvalid(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	t.Setenv("MIGRATIONS_DIR", "./migrations")
	t.Setenv("APP_BASE_URL", "http://localhost:8080")
	t.Setenv("DADATA_API_KEY", "test-dadata-key")
	t.Setenv("TARIFF_CACHE_TTL", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid TARIFF_CACHE_TTL")
	}
}

func TestTariffCacheTTLNonPositive(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	t.Setenv("MIGRATIONS_DIR", "./migrations")
	t.Setenv("APP_BASE_URL", "http://localhost:8080")
	t.Setenv("DADATA_API_KEY", "test-dadata-key")
	t.Setenv("TARIFF_CACHE_TTL", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for non-positive TARIFF_CACHE_TTL")
	}
}

