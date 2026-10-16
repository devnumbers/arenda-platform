package config

import (
	"strings"
	"testing"
	"time"
)

func TestDaDataDefaults(t *testing.T) {
	setRequiredLocalEnv(t)
	// This is an env test and stays serial: t.Setenv cannot follow t.Parallel.
	// The call is direct (the helper's identical set is invisible to the
	// linter) so paralleltest's Setenv exemption recognizes the class.
	t.Setenv("DADATA_API_KEY", "test-dadata-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DaDataBaseURL != "https://suggestions.dadata.ru/suggestions/api/4_1/rs/suggest/address" {
		t.Fatalf("expected default DaDataBaseURL, got %q", cfg.DaDataBaseURL)
	}
	if cfg.DaDataTimeout != 10*time.Second {
		t.Fatalf("expected default DaDataTimeout 10s, got %v", cfg.DaDataTimeout)
	}
}

func TestDaDataEnvOverrides(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("DADATA_BASE_URL", "http://localhost:8081/suggest/address")
	t.Setenv("DADATA_TIMEOUT", "5s")
	t.Setenv("DADATA_SECRET_KEY", "test-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DaDataBaseURL != "http://localhost:8081/suggest/address" {
		t.Fatalf("expected overridden DaDataBaseURL, got %q", cfg.DaDataBaseURL)
	}
	if cfg.DaDataTimeout != 5*time.Second {
		t.Fatalf("expected overridden DaDataTimeout 5s, got %v", cfg.DaDataTimeout)
	}
	if cfg.DaDataSecretKey != "test-secret" {
		t.Fatalf("expected DaDataSecretKey, got %q", cfg.DaDataSecretKey)
	}
}

func TestDaDataAPIKeyRequired(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("DADATA_API_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing DADATA_API_KEY")
	}
	if !strings.Contains(err.Error(), "DADATA_API_KEY") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDaDataBaseURLRejectsInvalidScheme(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("DADATA_BASE_URL", "ftp://example.com")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid DADATA_BASE_URL scheme")
	}
	if !strings.Contains(err.Error(), "scheme") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDaDataBaseURLRejectsInvalidURL(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("DADATA_BASE_URL", "://not-a-url")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid DADATA_BASE_URL")
	}
}

func TestDaDataTimeoutInvalid(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("DADATA_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid DADATA_TIMEOUT")
	}
}

func TestDaDataTimeoutNonPositive(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("DADATA_TIMEOUT", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for non-positive DADATA_TIMEOUT")
	}
}
