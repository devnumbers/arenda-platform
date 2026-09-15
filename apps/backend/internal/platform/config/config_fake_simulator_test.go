package config

import (
	"strings"
	"testing"
)

// The fake payment simulator knobs (issue #663): WEB_ORIGIN — the frontend
// origin the fake bank returns the browser to; FAKE_AUTOCONFIRM — the
// auto/manual switch of the confirmation endpoints. Both are fake-provider
// concerns: they load and validate only when PAYMENT_PROVIDER=fake.

func TestFakeSimulatorWebOriginDefaultsToAppBaseURLOrigin(t *testing.T) { //nolint:paralleltest // fixture uses t.Setenv
	setRequiredLocalEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.WebOrigin != "http://localhost:8080" {
		t.Fatalf("WebOrigin = %q, want the APP_BASE_URL origin %q", cfg.WebOrigin, "http://localhost:8080")
	}
}

func TestFakeSimulatorWebOriginExplicitWinsAndNormalizes(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("WEB_ORIGIN", "http://localhost:3000/")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.WebOrigin != "http://localhost:3000" {
		t.Fatalf("WebOrigin = %q, want the trailing slash trimmed", cfg.WebOrigin)
	}
}

func TestFakeSimulatorWebOriginRejectsNonURL(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("WEB_ORIGIN", "localhost:3000")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "WEB_ORIGIN") {
		t.Fatalf("expected a WEB_ORIGIN validation error, got: %v", err)
	}
}

func TestFakeSimulatorWebOriginIgnoredUnderTKassa(t *testing.T) {
	setRequiredLocalEnv(t)
	requireTKassaCreds(t)
	t.Setenv("PAYMENT_PROVIDER", providerTKassa)
	t.Setenv("T_KASSA_BASE_URL", tkassaSandboxBaseURL)
	t.Setenv("WEB_ORIGIN", ":::not-a-url")

	if _, err := Load(); err != nil {
		t.Fatalf("WEB_ORIGIN must not be validated under tkassa, got: %v", err)
	}
}

func TestFakeSimulatorAutoConfirmDefaultsToAuto(t *testing.T) { //nolint:paralleltest // fixture uses t.Setenv
	setRequiredLocalEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !cfg.FakeAutoConfirm {
		t.Fatal("FakeAutoConfirm = false, want true (the auto default)")
	}
}

func TestFakeSimulatorAutoConfirmOff(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("FAKE_AUTOCONFIRM", fakeConfirmModeOff)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.FakeAutoConfirm {
		t.Fatal("FakeAutoConfirm = true, want false (the manual mode)")
	}
}

func TestFakeSimulatorAutoConfirmRejectsUnknownValue(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("FAKE_AUTOCONFIRM", "yes")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "FAKE_AUTOCONFIRM") {
		t.Fatalf("expected a FAKE_AUTOCONFIRM validation error, got: %v", err)
	}
}

func TestFakeSimulatorAutoConfirmIgnoredUnderTKassa(t *testing.T) {
	setRequiredLocalEnv(t)
	requireTKassaCreds(t)
	t.Setenv("PAYMENT_PROVIDER", providerTKassa)
	t.Setenv("T_KASSA_BASE_URL", tkassaSandboxBaseURL)
	t.Setenv("FAKE_AUTOCONFIRM", "bogus")

	if _, err := Load(); err != nil {
		t.Fatalf("FAKE_AUTOCONFIRM must not be validated under tkassa, got: %v", err)
	}
}
