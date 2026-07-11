package config

import (
	"strings"
	"testing"
)

func TestOTelEnabledRequiresEndpoint(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("OTEL_TRACES_EXPORTER", "otlp")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when OTel is enabled without OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if !strings.Contains(err.Error(), "OTEL_EXPORTER_OTLP_ENDPOINT") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOTelEnabledWithEndpoint(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("OTEL_TRACES_EXPORTER", "otlp")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !cfg.OTelEnabled {
		t.Fatal("expected OTelEnabled to be true")
	}
	if cfg.OTelOTLPEndpoint != "http://localhost:4317" {
		t.Fatalf("expected OTelOTLPEndpoint, got %q", cfg.OTelOTLPEndpoint)
	}
}

func TestOTelDisabledAllowsEmptyEndpoint(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.OTelEnabled {
		t.Fatal("expected OTelEnabled to be false")
	}
}
