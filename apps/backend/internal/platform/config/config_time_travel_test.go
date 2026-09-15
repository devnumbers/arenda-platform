package config

import (
	"strings"
	"testing"
)

// The billing time-travel rig (issue #665): BILLING_TIME_TRAVEL enables the
// admin time-shift and tick endpoints of the subscription lifecycle
// acceptance. The mechanism exists only on the non-production stands — the
// same allowlist the fake providers live on — and the config validator fails
// at startup when a production process asks for it.

func TestTimeTravelDefaultsToOff(t *testing.T) { //nolint:paralleltest // fixture uses t.Setenv
	setRequiredLocalEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.BillingTimeTravel {
		t.Fatal("BillingTimeTravel = true, want the default off")
	}
}

func TestTimeTravelExplicitOnLocal(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("BILLING_TIME_TRAVEL", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !cfg.BillingTimeTravel {
		t.Fatal("BillingTimeTravel = false, want true on the local stand")
	}
}

func TestTimeTravelExplicitOnStage(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("APP_ENV", envStage)
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_BASE_URL", "https://stage.example.com")
	t.Setenv("PAYMENT_PROVIDER", providerFake)
	t.Setenv("BILLING_TIME_TRAVEL", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !cfg.BillingTimeTravel {
		t.Fatal("BillingTimeTravel = false, want true on the stage stand")
	}
}

func TestTimeTravelRejectedInProduction(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("APP_ENV", envProduction)
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_BASE_URL", "https://example.com")
	t.Setenv("PAYMENT_PROVIDER", providerTKassa)
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")
	t.Setenv("T_KASSA_BASE_URL", "https://securepay.tinkoff.ru/v2/")
	t.Setenv("BILLING_TIME_TRAVEL", "true")

	_, err := Load()
	if err == nil {
		t.Fatal("expected the production startup failure for BILLING_TIME_TRAVEL=true")
	}
	if !strings.Contains(err.Error(), "BILLING_TIME_TRAVEL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTimeTravelRejectsUnknownValue(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("BILLING_TIME_TRAVEL", "yes")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "BILLING_TIME_TRAVEL") {
		t.Fatalf("expected a BILLING_TIME_TRAVEL validation error, got: %v", err)
	}
}
