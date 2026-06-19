package main

import (
	"os"
	"testing"
	"time"
)

func TestParseSummary(t *testing.T) {
	data := []byte(`{
		"metrics": {
			"http_req_duration{expected_response:true}": {
				"values": {
					"p(95)": 123.45
				}
			},
			"http_req_failed": {
				"values": {
					"rate": 0.005
				}
			}
		}
	}`)

	p95, errorRate, err := parseSummary(data)
	if err != nil {
		t.Fatalf("parseSummary failed: %v", err)
	}
	if p95 != 123.45 {
		t.Errorf("p95 = %v, want 123.45", p95)
	}
	if errorRate != 0.005 {
		t.Errorf("errorRate = %v, want 0.005", errorRate)
	}
}

func TestParseSummaryFallback(t *testing.T) {
	data := []byte(`{
		"metrics": {
			"http_req_duration": {
				"values": {
					"p(95)": 200.0
				}
			},
			"http_req_failed": {
				"values": {
					"rate": 0.0
				}
			}
		}
	}`)

	p95, errorRate, err := parseSummary(data)
	if err != nil {
		t.Fatalf("parseSummary failed: %v", err)
	}
	if p95 != 200.0 {
		t.Errorf("p95 = %v, want 200.0", p95)
	}
	if errorRate != 0.0 {
		t.Errorf("errorRate = %v, want 0.0", errorRate)
	}
}

func TestParseSummaryMissingMetric(t *testing.T) {
	data := []byte(`{"metrics": {}}`)
	_, _, err := parseSummary(data)
	if err == nil {
		t.Fatal("expected error for missing metric")
	}
}

func TestEnvInt(t *testing.T) {
	t.Run("fallback", func(t *testing.T) {
		t.Cleanup(func() { os.Unsetenv("TEST_INT") })
		os.Unsetenv("TEST_INT")
		v, err := envInt("TEST_INT", 42)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != 42 {
			t.Errorf("envInt fallback = %d, want 42", v)
		}
	})

	t.Run("valid", func(t *testing.T) {
		t.Cleanup(func() { os.Unsetenv("TEST_INT") })
		os.Setenv("TEST_INT", "99")
		v, err := envInt("TEST_INT", 42)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != 99 {
			t.Errorf("envInt = %d, want 99", v)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Cleanup(func() { os.Unsetenv("TEST_INT") })
		os.Setenv("TEST_INT", "not-a-number")
		_, err := envInt("TEST_INT", 42)
		if err == nil {
			t.Fatal("expected error for invalid int")
		}
	})
}

func TestEnvDuration(t *testing.T) {
	t.Run("fallback", func(t *testing.T) {
		t.Cleanup(func() { os.Unsetenv("TEST_DUR") })
		os.Unsetenv("TEST_DUR")
		v, err := envDuration("TEST_DUR", 15*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != 15*time.Second {
			t.Errorf("envDuration fallback = %v, want 15s", v)
		}
	})

	t.Run("valid", func(t *testing.T) {
		t.Cleanup(func() { os.Unsetenv("TEST_DUR") })
		os.Setenv("TEST_DUR", "30s")
		v, err := envDuration("TEST_DUR", 15*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != 30*time.Second {
			t.Errorf("envDuration = %v, want 30s", v)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Cleanup(func() { os.Unsetenv("TEST_DUR") })
		os.Setenv("TEST_DUR", "invalid")
		_, err := envDuration("TEST_DUR", 15*time.Second)
		if err == nil {
			t.Fatal("expected error for invalid duration")
		}
	})
}
